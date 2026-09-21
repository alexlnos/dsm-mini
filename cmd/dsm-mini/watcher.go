package main

import (
	"log/slog"

	"github.com/alexlnos/dsm-mini/internal/bot"
	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/store"
	"github.com/alexlnos/dsm-mini/internal/watcher"
)

// newWatcher настраивает наблюдателя за задачами.
//
// Уведомления идут всем, кому разрешён доступ: список короткий и состоит из
// владельцев NAS, а привязывать задачу к тому, кто её создал, невозможно —
// Download Station не хранит, из какого чата она пришла.
func newWatcher(cfg *config.Config, ds downloadstation.Station, b *bot.Bot,
	st *store.Store, log *slog.Logger) *watcher.Watcher {
	return watcher.New(watcher.Options{
		Downloads: ds,
		Notifier:  b,
		Store:     st,
		// Язык получателя: уведомление уходит само, спросить некого.
		Langs:    st,
		ChatIDs:  cfg.AllowedUserIDs,
		Interval: cfg.WatchInterval,
		Logger:   log,
	})
}
