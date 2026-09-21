package main

import (
	"log/slog"

	"github.com/alexlnos/dsm-mini/internal/bot"
	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/store"
	"github.com/alexlnos/dsm-mini/internal/watcher"
)

// newWatcher wires up the task watcher.
//
// Notifications go to everyone on the allow list: it is short and consists of
// the NAS owners, and there is no way to tie a task back to whoever created
// it — Download Station does not record which chat it came from.
func newWatcher(cfg *config.Config, ds downloadstation.Station, b *bot.Bot,
	st *store.Store, log *slog.Logger) *watcher.Watcher {
	return watcher.New(watcher.Options{
		Downloads: ds,
		Notifier:  b,
		Store:     st,
		// Recipient language: a notification is sent on our own initiative,
		// so there is nobody to ask at that moment.
		Langs:    st,
		ChatIDs:  cfg.AllowedUserIDs,
		Interval: cfg.WatchInterval,
		Logger:   log,
	})
}
