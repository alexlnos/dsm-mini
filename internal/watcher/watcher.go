// Package watcher следит за задачами и пишет в чат, когда они завершаются.
package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
)

// Notifier отправляет готовый текст в чат.
type Notifier interface {
	Notify(ctx context.Context, chatID int64, text string) error
}

// StateStore хранит последние известные статусы задач между запусками.
type StateStore interface {
	TaskStates(ctx context.Context) (map[string]string, error)
	SaveTaskStates(ctx context.Context, states map[string]string) error
}

// Watcher опрашивает Download Station и сообщает о завершившихся задачах.
type Watcher struct {
	ds       downloadstation.Station
	notifier Notifier
	store    StateStore
	chatIDs  []int64
	interval time.Duration
	log      *slog.Logger

	// seen хранит последний известный статус каждой задачи.
	seen map[string]downloadstation.Status
}

// Options — настройки наблюдателя.
type Options struct {
	Downloads downloadstation.Station
	Notifier  Notifier
	// ChatIDs — кому слать. Обычно совпадает со списком разрешённых.
	ChatIDs  []int64
	Interval time.Duration
	// Store хранит состояние между запусками. Без него после перезапуска
	// сервис повторно сообщит о задачах, завершившихся ещё до него.
	Store  StateStore
	Logger *slog.Logger
}

// New создаёт наблюдателя.
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
		chatIDs:  o.ChatIDs,
		interval: o.Interval,
		log:      o.Logger,
		seen:     make(map[string]downloadstation.Status),
	}
	w.load(context.Background())
	return w
}

// Run опрашивает NAS, пока жив контекст.
func (w *Watcher) Run(ctx context.Context) {
	// Первый проход — «тихий»: он лишь запоминает текущее положение дел,
	// иначе после запуска в чат посыпались бы сообщения обо всём, что
	// докачалось когда-то раньше.
	if len(w.seen) == 0 {
		w.snapshot(ctx)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("наблюдение за задачами запущено", "interval", w.interval)
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
		w.log.Warn("первый опрос не удался", "err", err)
		return
	}
	for _, t := range tasks {
		w.seen[t.ID] = t.Status
	}
	w.save(ctx)
	w.log.Info("исходное состояние записано", "tasks", len(tasks))
}

func (w *Watcher) check(ctx context.Context) {
	tasks, err := w.ds.List(ctx)
	if err != nil {
		w.log.Warn("опрос задач не удался", "err", err)
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

		// Сообщаем только о переходе в конечное состояние и только если
		// раньше задача была в работе: иначе перезапуск NAS или первое
		// появление готовой задачи породили бы ложное уведомление.
		if known && t.Status.Terminal() && !prev.Terminal() {
			w.announce(ctx, t)
		}
	}

	// Удалённые задачи забываем, чтобы карта не росла бесконечно.
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
	text := formatTask(t)
	for _, chat := range w.chatIDs {
		if err := w.notifier.Notify(ctx, chat, text); err != nil {
			w.log.Error("не отправить уведомление", "chat", chat, "err", err)
		}
	}
	w.log.Info("уведомление отправлено", "task", t.ID, "status", t.Status)
}

func formatTask(t downloadstation.Task) string {
	title := t.Title
	if len(title) > 200 {
		title = title[:200] + "…"
	}

	var sb strings.Builder
	if t.Status == downloadstation.StatusError {
		sb.WriteString("Задача упала\n\n")
	} else {
		sb.WriteString("Загрузка завершена\n\n")
	}
	sb.WriteString(title)

	if t.Size > 0 {
		fmt.Fprintf(&sb, "\n%s", humanSize(t.Size))
	}
	if t.Destination != "" {
		fmt.Fprintf(&sb, " → %s", t.Destination)
	}
	if t.Status == downloadstation.StatusFinished && !t.CreatedAt.IsZero() && !t.CompletedAt.IsZero() {
		if d := t.CompletedAt.Sub(t.CreatedAt); d > 0 {
			fmt.Fprintf(&sb, "\nЗаняло %s", humanDuration(d))
		}
	}
	return sb.String()
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d Б", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), [...]string{"КБ", "МБ", "ГБ", "ТБ"}[exp])
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%d ч %d мин", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%d мин", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d с", int(d.Seconds()))
	}
}

func (w *Watcher) load(ctx context.Context) {
	if w.store == nil {
		return
	}
	states, err := w.store.TaskStates(ctx)
	if err != nil {
		w.log.Warn("не прочитать сохранённое состояние", "err", err)
		return
	}
	for id, status := range states {
		w.seen[id] = downloadstation.Status(status)
	}
	if len(w.seen) > 0 {
		w.log.Info("состояние восстановлено", "tasks", len(w.seen))
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
		w.log.Warn("не сохранить состояние", "err", err)
	}
}
