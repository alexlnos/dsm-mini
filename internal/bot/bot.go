// Package bot is the Telegram bot: the quick path without opening the Mini App.
//
// The most common scenario in practice: the user forwards a magnet link or a
// .torrent file straight into the chat, the bot offers folders as buttons and
// queues the task. The Mini App is for watching the download progress.
package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/i18n"
	"github.com/alexlnos/dsm-mini/internal/store"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// maxTorrentSize is the size limit for a .torrent accepted from the chat.
// Real torrent files are orders of magnitude smaller.
const maxTorrentSize = 10 << 20

// Bot serves the chat.
type Bot struct {
	api       *tg.Bot
	ds        downloadstation.Station
	settings  *store.Store
	allowed   []int64
	publicURL string
	token     string
	pending   *pendingStore
	log       *slog.Logger
}

// Options are the bot dependencies.
type Options struct {
	Token          string
	AllowedUserIDs []int64
	Downloads      downloadstation.Station
	// Settings holds the user's pinned folders and history.
	Settings *store.Store
	// PublicURL is the Mini App address for the "Open" button.
	PublicURL string
	Logger    *slog.Logger
}

// New creates a bot.
func New(o Options) (*Bot, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	b := &Bot{
		ds:        o.Downloads,
		settings:  o.Settings,
		allowed:   o.AllowedUserIDs,
		publicURL: o.PublicURL,
		token:     o.Token,
		pending:   newPendingStore(),
		log:       o.Logger,
	}

	api, err := tg.New(o.Token,
		tg.WithDefaultHandler(b.handleMessage),
		tg.WithCallbackQueryDataHandler("dest:", tg.MatchTypePrefix, b.handleDestination),
	)
	if err != nil {
		// The library checks the token with a getMe request at creation time,
		// so a wrong token shows up at startup rather than on the first
		// message. We point at where to get one.
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, fmt.Errorf("%w: %w", ErrBadToken, err)
		}
		return nil, fmt.Errorf("cannot create the bot: %w", err)
	}
	b.api = api
	return b, nil
}

// ErrBadToken means Telegram did not accept the token. Retrying is pointless:
// this is a configuration error, not a connectivity one.
var ErrBadToken = errors.New("Telegram rejected the bot token — check TELEGRAM_BOT_TOKEN, @BotFather issues it")

// Start begins receiving updates and runs for as long as the context lives.
//
// Long polling is used rather than a webhook: DSM restarts nginx on every
// certificate renewal, and with a webhook that would mean lost messages.
func (b *Bot) Start(ctx context.Context) {
	b.log.Info("bot started")
	b.api.Start(ctx)
}

// API exposes the Telegram client for sending notifications.
func (b *Bot) API() *tg.Bot { return b.api }

func (b *Bot) isAllowed(id int64) bool {
	for _, a := range b.allowed {
		if a == id {
			return true
		}
	}
	return false
}

func (b *Bot) handleMessage(ctx context.Context, api *tg.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	msg := update.Message
	from := msg.From.ID
	lang := i18n.Match(msg.From.LanguageCode)

	if !b.isAllowed(from) {
		b.log.Warn("message from an outsider", "user", from, "username", msg.From.Username)
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.denied"))
		return
	}
	b.rememberLanguage(ctx, from, msg.From.LanguageCode)

	switch {
	case strings.HasPrefix(msg.Text, "/start"):
		b.sendWelcome(ctx, msg.Chat.ID, lang)
	case strings.HasPrefix(msg.Text, "/status"):
		b.sendStatus(ctx, msg.Chat.ID, lang)
	case msg.Document != nil:
		b.handleDocument(ctx, msg, lang)
	case msg.Text != "":
		b.handleText(ctx, msg, lang)
	}
}

// rememberLanguage records the language of whoever is talking to us: task
// notifications are sent on our own initiative, with nobody to ask by then.
func (b *Bot) rememberLanguage(ctx context.Context, userID int64, code string) {
	if b.settings == nil {
		return
	}
	if err := b.settings.RememberLanguage(ctx, userID, code); err != nil {
		b.log.Warn("cannot remember the language", "user", userID, "err", err)
	}
}

func (b *Bot) sendWelcome(ctx context.Context, chatID int64, lang i18n.Lang) {
	params := &tg.SendMessageParams{ChatID: chatID, Text: i18n.T(lang, "bot.welcome")}
	if b.publicURL != "" {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: i18n.T(lang, "bot.openApp"), WebApp: &models.WebAppInfo{URL: b.publicURL}},
			}},
		}
	}
	if _, err := b.api.SendMessage(ctx, params); err != nil {
		b.log.Error("cannot send the greeting", "err", err)
	}
}

func (b *Bot) sendStatus(ctx context.Context, chatID int64, lang i18n.Lang) {
	tasks, err := b.ds.List(ctx)
	if err != nil {
		b.reply(ctx, chatID, i18n.T(lang, "bot.nasFailed", i18n.P{"error": err.Error()}))
		return
	}

	var active []downloadstation.Task
	for _, t := range tasks {
		if t.Status.Active() {
			active = append(active, t)
		}
	}
	if len(active) == 0 {
		b.reply(ctx, chatID, i18n.T(lang, "bot.nothingActive",
			i18n.P{"total": strconv.Itoa(len(tasks))}))
		return
	}

	var sb strings.Builder
	var down int64
	sb.WriteString(i18n.T(lang, "bot.activeCount", i18n.P{"count": strconv.Itoa(len(active))}))
	sb.WriteString("\n")
	for i, t := range active {
		if i == 10 {
			fmt.Fprintf(&sb, "\n%s", i18n.T(lang, "bot.andMore",
				i18n.P{"count": strconv.Itoa(len(active) - 10)}))
			break
		}
		down += t.SpeedDown
		fmt.Fprintf(&sb, "\n%s\n  %.0f%% · %s", t.Title, t.Progress()*100, speed(lang, t.SpeedDown))
		if eta, ok := t.ETA(); ok {
			sb.WriteString(i18n.T(lang, "bot.etaLeft", i18n.P{"eta": duration(lang, eta)}))
		}
	}
	fmt.Fprintf(&sb, "\n\n%s", i18n.T(lang, "bot.totalSpeed", i18n.P{"speed": speed(lang, down)}))
	b.reply(ctx, chatID, sb.String())
}

func (b *Bot) handleText(ctx context.Context, msg *models.Message, lang i18n.Lang) {
	link := strings.TrimSpace(msg.Text)
	if !isDownloadLink(link) {
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.notALink"))
		return
	}
	b.askDestination(ctx, msg.Chat.ID, msg.From.ID, lang,
		pending{URL: link, Title: titleFromLink(link)})
}

func (b *Bot) handleDocument(ctx context.Context, msg *models.Message, lang i18n.Lang) {
	doc := msg.Document
	name := doc.FileName
	if !strings.HasSuffix(strings.ToLower(name), ".torrent") {
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.needTorrent"))
		return
	}
	if doc.FileSize > maxTorrentSize {
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.tooBig"))
		return
	}

	data, err := b.download(ctx, doc.FileID)
	if err != nil {
		b.log.Error("cannot download the file from Telegram", "err", err)
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.fetchFailed"))
		return
	}
	b.askDestination(ctx, msg.Chat.ID, msg.From.ID, lang,
		pending{File: data, FileName: name, Title: name})
}

// download fetches a file sent to the chat from Telegram's servers.
func (b *Bot) download(ctx context.Context, fileID string) ([]byte, error) {
	f, err := b.api.GetFile(ctx, &tg.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}
	url := b.api.FileDownloadLink(f)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Telegram answered HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxTorrentSize+1))
}

// askDestination offers folders as buttons.
func (b *Bot) askDestination(ctx context.Context, chatID, userID int64, lang i18n.Lang, p pending) {
	key := b.pending.put(p)

	folders := b.folders(ctx, userID)
	rows := make([][]models.InlineKeyboardButton, 0, len(folders))
	for i, f := range folders {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text: f,
			// callback_data holds 64 bytes, so only the request key and the
			// folder number go here, not the path itself.
			CallbackData: fmt.Sprintf("dest:%s:%d", key, i),
		}})
	}

	text := i18n.T(lang, "bot.whereTo")
	if p.Title != "" {
		text = p.Title + "\n\n" + text
	}
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
	if err != nil {
		b.log.Error("cannot send the folder choice", "err", err)
	}
}

// folders collects the folders for the buttons: the ones pinned by the user,
// then their recent ones, then the default folder.
//
// Folders of existing tasks do not go here: those are other people's torrents
// and long-past downloads, unrelated to a new one.
func (b *Bot) folders(ctx context.Context, userID int64) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 6)
	add := func(folder string) {
		if folder == "" || seen[folder] {
			return
		}
		seen[folder] = true
		out = append(out, folder)
	}

	if b.settings != nil {
		settings, err := b.settings.Get(ctx, userID)
		if err != nil {
			b.log.Warn("cannot read the settings for folders", "user", userID, "err", err)
		} else {
			for _, folder := range settings.PinnedFolders {
				add(folder)
			}
			if settings.ShowRecent {
				recent, err := b.settings.RecentFolders(ctx, userID, 5)
				if err != nil {
					b.log.Warn("cannot read the folder history", "err", err)
				}
				for _, folder := range recent {
					add(folder)
				}
			}
		}
	}
	if def, err := b.ds.DefaultDestination(ctx); err == nil {
		add(def)
	}
	return out
}

func (b *Bot) handleDestination(ctx context.Context, api *tg.Bot, update *models.Update) {
	q := update.CallbackQuery
	if q == nil || q.From.ID == 0 {
		return
	}
	lang := i18n.Match(q.From.LanguageCode)
	if !b.isAllowed(q.From.ID) {
		b.answer(ctx, q.ID, i18n.T(lang, "bot.accessClosed"))
		return
	}
	b.rememberLanguage(ctx, q.From.ID, q.From.LanguageCode)

	parts := strings.SplitN(strings.TrimPrefix(q.Data, "dest:"), ":", 2)
	if len(parts) != 2 {
		b.answer(ctx, q.ID, i18n.T(lang, "bot.badChoice"))
		return
	}

	p, ok := b.pending.take(parts[0])
	if !ok {
		b.answer(ctx, q.ID, i18n.T(lang, "bot.expired"))
		return
	}

	var index int
	if _, err := fmt.Sscanf(parts[1], "%d", &index); err != nil {
		b.answer(ctx, q.ID, i18n.T(lang, "bot.badChoice"))
		return
	}
	folders := b.folders(ctx, q.From.ID)
	if index < 0 || index >= len(folders) {
		b.answer(ctx, q.ID, i18n.T(lang, "bot.folderGone"))
		return
	}
	dest := folders[index]

	req := downloadstation.CreateRequest{Destination: dest}
	if len(p.File) > 0 {
		req.TorrentFile = p.File
		req.FileName = p.FileName
	} else {
		req.URLs = []string{p.URL}
	}

	if err := b.ds.Create(ctx, req); err != nil {
		b.log.Error("cannot queue the task from the chat", "user", q.From.ID, "err", err)
		b.answer(ctx, q.ID, i18n.T(lang, "bot.failed"))
		b.reply(ctx, chatOf(q), i18n.T(lang, "bot.nasRefused", i18n.P{"error": err.Error()}))
		return
	}

	if b.settings != nil {
		if err := b.settings.RememberLastUsed(ctx, q.From.ID, dest); err != nil {
			b.log.Warn("cannot remember the folder", "err", err)
		}
	}
	b.log.Info("task from the chat queued", "user", q.From.ID, "dest", dest)
	b.answer(ctx, q.ID, i18n.T(lang, "bot.queued"))
	b.reply(ctx, chatOf(q), i18n.T(lang, "bot.queuedTo", i18n.P{"folder": dest}))
}

func chatOf(q *models.CallbackQuery) int64 {
	if q.Message.Message != nil {
		return q.Message.Message.Chat.ID
	}
	return q.From.ID
}

func (b *Bot) answer(ctx context.Context, id, text string) {
	if _, err := b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
		CallbackQueryID: id, Text: text,
	}); err != nil {
		b.log.Error("cannot answer the button press", "err", err)
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if _, err := b.api.SendMessage(ctx, &tg.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		b.log.Error("cannot send the message", "err", err)
	}
}

func isDownloadLink(s string) bool {
	lower := strings.ToLower(s)
	for _, p := range []string{"magnet:", "http://", "https://", "ftp://", "ftps://", "ed2k://"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// titleFromLink pulls a human-readable name out of a magnet link, if any.
func titleFromLink(link string) string {
	if i := strings.Index(link, "dn="); i >= 0 {
		name := link[i+3:]
		if j := strings.IndexByte(name, '&'); j >= 0 {
			name = name[:j]
		}
		if decoded, err := decodeQuery(name); err == nil && decoded != "" {
			return decoded
		}
	}
	if len(link) > 80 {
		return link[:80] + "…"
	}
	return link
}

func speed(lang i18n.Lang, bps int64) string {
	switch {
	case bps >= 1<<20:
		return fmt.Sprintf("%.1f %s/s", float64(bps)/(1<<20), i18n.T(lang, "unit.mb"))
	case bps >= 1<<10:
		return fmt.Sprintf("%.0f %s/s", float64(bps)/(1<<10), i18n.T(lang, "unit.kb"))
	default:
		return fmt.Sprintf("%d %s/s", bps, i18n.T(lang, "unit.bytes"))
	}
}

func duration(lang i18n.Lang, d time.Duration) string {
	switch {
	case d >= time.Hour:
		return i18n.T(lang, "unit.hours", i18n.P{"value": strconv.Itoa(int(d.Hours()))}) + " " +
			i18n.T(lang, "unit.minutes", i18n.P{"value": strconv.Itoa(int(d.Minutes()) % 60)})
	case d >= time.Minute:
		return i18n.T(lang, "unit.minutes", i18n.P{"value": strconv.Itoa(int(d.Minutes()))})
	default:
		return i18n.T(lang, "unit.seconds", i18n.P{"value": strconv.Itoa(int(d.Seconds()))})
	}
}
