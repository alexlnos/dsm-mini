// Package bot — телеграм-бот: быстрый путь без открытия Mini App.
//
// Самый частый сценарий на практике: пользователь пересылает magnet-ссылку
// или файл .torrent прямо в чат, бот предлагает папку кнопками и ставит
// задачу. Mini App нужен, когда хочется посмотреть на ход загрузки.
package bot

import (
	"context"
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

// maxTorrentSize — предел размера .torrent, который примем из чата.
// Настоящие торрент-файлы на порядки меньше.
const maxTorrentSize = 10 << 20

// Bot обслуживает чат.
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

// Options — зависимости бота.
type Options struct {
	Token          string
	AllowedUserIDs []int64
	Downloads      downloadstation.Station
	// Settings — закреплённые папки пользователя и история.
	Settings *store.Store
	// PublicURL — адрес Mini App для кнопки «Открыть».
	PublicURL string
	Logger    *slog.Logger
}

// New создаёт бота.
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
		// Библиотека проверяет токен запросом getMe прямо при создании,
		// поэтому неверный токен виден сразу при запуске, а не при первом
		// сообщении. Подсказываем, где его взять.
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, fmt.Errorf("Telegram отклонил токен бота — проверьте TELEGRAM_BOT_TOKEN, его выдаёт @BotFather: %w", err)
		}
		return nil, fmt.Errorf("не создать бота: %w", err)
	}
	b.api = api
	return b, nil
}

// Start запускает получение обновлений и работает, пока жив контекст.
//
// Используется long polling, а не webhook: DSM перезапускает nginx при каждом
// продлении сертификата, и на webhook это означало бы потерянные сообщения.
func (b *Bot) Start(ctx context.Context) {
	b.log.Info("бот запущен")
	b.api.Start(ctx)
}

// API даёт доступ к клиенту Telegram для отправки уведомлений.
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
		b.log.Warn("сообщение от постороннего", "user", from, "username", msg.From.Username)
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

// rememberLanguage запоминает язык собеседника: уведомления о завершённых
// задачах уходят сами, и спросить язык в тот момент не у кого.
func (b *Bot) rememberLanguage(ctx context.Context, userID int64, code string) {
	if b.settings == nil {
		return
	}
	if err := b.settings.RememberLanguage(ctx, userID, code); err != nil {
		b.log.Warn("не запомнить язык", "user", userID, "err", err)
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
		b.log.Error("не отправить приветствие", "err", err)
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
		b.log.Error("не скачать файл из Telegram", "err", err)
		b.reply(ctx, msg.Chat.ID, i18n.T(lang, "bot.fetchFailed"))
		return
	}
	b.askDestination(ctx, msg.Chat.ID, msg.From.ID, lang,
		pending{File: data, FileName: name, Title: name})
}

// download забирает файл, присланный в чат, с серверов Telegram.
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
		return nil, fmt.Errorf("Telegram ответил HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxTorrentSize+1))
}

// askDestination предлагает папки кнопками.
func (b *Bot) askDestination(ctx context.Context, chatID, userID int64, lang i18n.Lang, p pending) {
	key := b.pending.put(p)

	folders := b.folders(ctx, userID)
	rows := make([][]models.InlineKeyboardButton, 0, len(folders))
	for i, f := range folders {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text: f,
			// В callback_data влезает 64 байта, поэтому здесь только ключ
			// запроса и номер папки, а не сам путь.
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
		b.log.Error("не отправить выбор папки", "err", err)
	}
}

// folders собирает список папок для кнопок: папка по умолчанию и те, куда
// недавно уже качали.
// folders собирает папки для кнопок: закреплённые пользователем, затем его
// недавние и папка по умолчанию.
//
// Папки существующих задач сюда не идут: это чужие раздачи и давние
// закачки, к новой загрузке отношения не имеющие.
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
			b.log.Warn("не прочитать настройки для папок", "user", userID, "err", err)
		} else {
			for _, folder := range settings.PinnedFolders {
				add(folder)
			}
			if settings.ShowRecent {
				recent, err := b.settings.RecentFolders(ctx, userID, 5)
				if err != nil {
					b.log.Warn("не прочитать историю папок", "err", err)
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
		b.log.Error("не поставить задачу из чата", "user", q.From.ID, "err", err)
		b.answer(ctx, q.ID, i18n.T(lang, "bot.failed"))
		b.reply(ctx, chatOf(q), i18n.T(lang, "bot.nasRefused", i18n.P{"error": err.Error()}))
		return
	}

	if b.settings != nil {
		if err := b.settings.RememberLastUsed(ctx, q.From.ID, dest); err != nil {
			b.log.Warn("не запомнить папку", "err", err)
		}
	}
	b.log.Info("задача из чата поставлена", "user", q.From.ID, "dest", dest)
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
		b.log.Error("не ответить на нажатие", "err", err)
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if _, err := b.api.SendMessage(ctx, &tg.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		b.log.Error("не отправить сообщение", "err", err)
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

// titleFromLink достаёт из magnet-ссылки человекочитаемое имя, если оно там есть.
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
