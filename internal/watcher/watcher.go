// Package watcher follows tasks and writes to the chat when they finish.
package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/i18n"
)

// Notifier sends ready-made text to a chat.
type Notifier interface {
	Notify(ctx context.Context, chatID int64, text string) error
}

// LangSource tells which language to write to a particular chat in.
//
// A notification is sent on our own initiative, with no incoming message, so
// the language comes from what was remembered on the person's last request.
type LangSource interface {
	Language(ctx context.Context, userID int64) (string, error)
}

// StateStore keeps the last known task statuses between runs.
type StateStore interface {
	TaskStates(ctx context.Context) (map[string]string, error)
	SaveTaskStates(ctx context.Context, states map[string]string) error
}

// Watcher polls Download Station and reports finished tasks.
type Watcher struct {
	ds       downloadstation.Station
	notifier Notifier
	store    StateStore
	langs    LangSource
	chatIDs  []int64
	interval time.Duration
	log      *slog.Logger

	// seen holds the last known status of every task.
	seen map[string]downloadstation.Status
}

// Options are the watcher settings.
type Options struct {
	Downloads downloadstation.Station
	Notifier  Notifier
	// ChatIDs is who to write to. Usually the same as the allow list.
	ChatIDs  []int64
	Interval time.Duration
	// Store keeps state between runs. Without it the service would report
	// tasks that finished before a restart all over again.
	Store StateStore
	// Langs hints at the recipient's language. Without it we write in English.
	Langs  LangSource
	Logger *slog.Logger
}

// New creates a watcher.
func New(o Options) *Watcher {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Interval <= 0 {
		o.Interval = 30 * time.Second
	}
	w := &Watcher{
		ds:       o.Downloads,
		notifier: o.Notifier,
		store:    o.Store,
		langs:    o.Langs,
		chatIDs:  o.ChatIDs,
		interval: o.Interval,
		log:      o.Logger,
		seen:     make(map[string]downloadstation.Status),
	}
	w.load(context.Background())
	return w
}

// Run polls the NAS for as long as the context lives.
func (w *Watcher) Run(ctx context.Context) {
	// The first pass is a quiet one: it only records the current state of
	// affairs, otherwise a start would flood the chat with everything that
	// ever finished downloading.
	if len(w.seen) == 0 {
		w.snapshot(ctx)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("task watching started", "interval", w.interval)
	for {
		select {
		case <-ctx.Done():
			w.save(context.Background())
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *Watcher) snapshot(ctx context.Context) {
	tasks, err := w.ds.List(ctx)
	if err != nil {
		w.log.Warn("the first poll failed", "err", err)
		return
	}
	for _, t := range tasks {
		w.seen[t.ID] = t.Status
	}
	w.save(ctx)
	w.log.Info("initial state recorded", "tasks", len(tasks))
}

func (w *Watcher) check(ctx context.Context) {
	tasks, err := w.ds.List(ctx)
	if err != nil {
		w.log.Warn("polling tasks failed", "err", err)
		return
	}

	alive := make(map[string]bool, len(tasks))
	var changed bool

	for _, t := range tasks {
		alive[t.ID] = true
		prev, known := w.seen[t.ID]
		w.seen[t.ID] = t.Status
		if prev == t.Status {
			continue
		}
		changed = true

		// Report only the transition into a final state, and only if the task
		// was running before: otherwise a NAS restart or a finished task
		// showing up for the first time would raise a false notification.
		if known && t.Status.Terminal() && !prev.Terminal() {
			w.announce(ctx, t)
		}
	}

	// Forget deleted tasks so the map does not grow forever.
	for id := range w.seen {
		if !alive[id] {
			delete(w.seen, id)
			changed = true
		}
	}
	if changed {
		w.save(ctx)
	}
}

func (w *Watcher) announce(ctx context.Context, t downloadstation.Task) {
	for _, chat := range w.chatIDs {
		text := formatTask(w.langOf(ctx, chat), t)
		if err := w.notifier.Notify(ctx, chat, text); err != nil {
			w.log.Error("cannot send the notification", "chat", chat, "err", err)
		}
	}
	w.log.Info("notification sent", "task", t.ID, "status", t.Status)
}

// langOf is the recipient's language; an unknown language gives English.
func (w *Watcher) langOf(ctx context.Context, chatID int64) i18n.Lang {
	if w.langs == nil {
		return i18n.Fallback
	}
	code, err := w.langs.Language(ctx, chatID)
	if err != nil {
		w.log.Warn("cannot read the recipient language", "chat", chatID, "err", err)
		return i18n.Fallback
	}
	return i18n.Match(code)
}

func formatTask(lang i18n.Lang, t downloadstation.Task) string {
	title := t.Title
	if len(title) > 200 {
		title = title[:200] + "…"
	}

	var sb strings.Builder
	if t.Status == downloadstation.StatusError {
		sb.WriteString(i18n.T(lang, "notify.failed"))
		// Download Station knows why, and "failed" alone leaves the person
		// guessing between a full disk and a dead tracker.
		if t.FailReason != downloadstation.ReasonNone {
			fmt.Fprintf(&sb, ": %s", i18n.T(lang, "fail."+string(t.FailReason)))
		}
	} else {
		sb.WriteString(i18n.T(lang, "notify.done"))
	}
	sb.WriteString("\n\n")
	sb.WriteString(title)

	if t.Size > 0 {
		fmt.Fprintf(&sb, "\n%s", humanSize(lang, t.Size))
	}
	if t.Destination != "" {
		fmt.Fprintf(&sb, " → %s", t.Destination)
	}
	if t.Status == downloadstation.StatusFinished && !t.CreatedAt.IsZero() && !t.CompletedAt.IsZero() {
		if d := t.CompletedAt.Sub(t.CreatedAt); d > 0 {
			fmt.Fprintf(&sb, "\n%s", i18n.T(lang, "notify.took",
				i18n.P{"duration": humanDuration(lang, d)}))
		}
	}
	return sb.String()
}

func humanSize(lang i18n.Lang, b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d %s", b, i18n.T(lang, "unit.bytes"))
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	names := [...]string{"unit.kb", "unit.mb", "unit.gb", "unit.tb"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), i18n.T(lang, names[exp]))
}

func humanDuration(lang i18n.Lang, d time.Duration) string {
	switch {
	case d >= time.Hour:
		return i18n.T(lang, "unit.hours", i18n.P{"value": fmt.Sprint(int(d.Hours()))}) + " " +
			i18n.T(lang, "unit.minutes", i18n.P{"value": fmt.Sprint(int(d.Minutes()) % 60)})
	case d >= time.Minute:
		return i18n.T(lang, "unit.minutes", i18n.P{"value": fmt.Sprint(int(d.Minutes()))})
	default:
		return i18n.T(lang, "unit.seconds", i18n.P{"value": fmt.Sprint(int(d.Seconds()))})
	}
}

func (w *Watcher) load(ctx context.Context) {
	if w.store == nil {
		return
	}
	states, err := w.store.TaskStates(ctx)
	if err != nil {
		w.log.Warn("cannot read the saved state", "err", err)
		return
	}
	for id, status := range states {
		w.seen[id] = downloadstation.Status(status)
	}
	if len(w.seen) > 0 {
		w.log.Info("state restored", "tasks", len(w.seen))
	}
}

func (w *Watcher) save(ctx context.Context) {
	if w.store == nil {
		return
	}
	states := make(map[string]string, len(w.seen))
	for id, status := range w.seen {
		states[id] = string(status)
	}
	if err := w.store.SaveTaskStates(ctx, states); err != nil {
		w.log.Warn("cannot save the state", "err", err)
	}
}
