package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/alexlnos/dsm-mini/internal/bot"
	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsmnotify"
	"github.com/alexlnos/dsm-mini/internal/dsmui"
	"github.com/alexlnos/dsm-mini/internal/store"
)

// botRetry is how long to wait between attempts to reach Telegram.
const botRetry = 30 * time.Second

// runBot starts the bot and everything that depends on it: appearance,
// incoming messages and the watcher for finished tasks.
//
// An unreachable Telegram must not take the service down. The Mini App is
// opened by the client on the phone, not by the NAS, so the app keeps working
// even when the NAS itself cannot reach api.telegram.org — which happens when
// its traffic bypasses the VPN. The bot connects on its own once the link is
// back.
func runBot(ctx context.Context, cfg *config.Config, ds downloadstation.Station,
	settings *store.Store, receiver *dsmnotify.Receiver, status *dsmui.Status, log *slog.Logger) {

	tgBot := waitForBot(ctx, cfg, ds, settings, status, log)
	if tgBot == nil {
		return
	}

	// The HTTP server has been up since before Telegram answered, so the
	// endpoint DSM calls had nowhere to send anything. It does now.
	receiver.SetNotifier(tgBot)

	var wg sync.WaitGroup

	// The bot's appearance is declared in code for every language in the
	// dictionary: name, descriptions and commands do not live in @BotFather
	// alone. Values are read before they are written, so a restart does not
	// burn Telegram's rate limits. The Bot API cannot change the avatar — it
	// is set by hand.
	wg.Add(1)
	go func() {
		defer wg.Done()
		setupCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		tgBot.ConfigureAll(setupCtx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		tgBot.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		newWatcher(cfg, ds, tgBot, settings, log).Run(ctx)
	}()

	wg.Wait()
}

// waitForBot creates the bot, retrying for as long as Telegram is out of
// reach. How it is going is shown in the settings window inside DSM.
//
// Returns nil only when waiting is pointless: the service is shutting down or
// the token was rejected.
func waitForBot(ctx context.Context, cfg *config.Config, ds downloadstation.Station,
	settings *store.Store, status *dsmui.Status, log *slog.Logger) *bot.Bot {

	status.SetBot(dsmui.Link{State: dsmui.LinkWaiting})
	warned := false
	for {
		tgBot, err := bot.New(bot.Options{
			Token:          cfg.BotToken,
			AllowedUserIDs: cfg.AllowedUserIDs,
			Downloads:      ds,
			Settings:       settings,
			PublicURL:      cfg.PublicURL,
			Logger:         log,
		})
		if err == nil {
			if warned {
				log.Info("telegram is reachable again")
			}
			nameCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			status.SetBot(dsmui.Link{State: dsmui.LinkOK, Name: tgBot.Username(nameCtx)})
			cancel()
			return tgBot
		}

		// A rejected token is not cured by retrying.
		if errors.Is(err, bot.ErrBadToken) {
			status.SetBot(dsmui.Link{State: dsmui.LinkFailed, Reason: dsmui.ReasonToken})
			log.Error("the bot will not start", "err", err)
			return nil
		}

		status.SetBot(dsmui.Link{State: dsmui.LinkWaiting, Reason: dsmui.ReasonUnreachable})
		// Logged once: repeating it every half a minute would bury
		// everything else in the log.
		if !warned {
			warned = true
			log.Warn("telegram unreachable, carrying on without the bot", "err", err, "retry", botRetry)
		} else {
			log.Debug("telegram still unreachable", "err", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(botRetry):
		}
	}
}
