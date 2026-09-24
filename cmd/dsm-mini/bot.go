package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/alexlnos/dsm-mini/internal/bot"
	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsmnotify"
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
	settings *store.Store, receiver *dsmnotify.Receiver, log *slog.Logger) {

	tgBot := waitForBot(ctx, cfg, ds, settings, log)
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
// reach.
//
// Returns nil only when waiting is pointless: the service is shutting down or
// the token was rejected.
func waitForBot(ctx context.Context, cfg *config.Config, ds downloadstation.Station,
	settings *store.Store, log *slog.Logger) *bot.Bot {

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
				notifyDSM("dsm-mini", "Telegram is reachable again, the bot is running.")
			}
			return tgBot
		}

		// A rejected token is not cured by retrying.
		if errors.Is(err, bot.ErrBadToken) {
			log.Error("the bot will not start", "err", err)
			notifyDSM("dsm-mini", "Telegram rejected the bot token. Check the package settings.")
			return nil
		}

		// Report the outage once: repeating it every half a minute would turn
		// the notification centre into noise.
		if !warned {
			warned = true
			log.Warn("telegram unreachable, carrying on without the bot", "err", err, "retry", botRetry)
			notifyDSM("dsm-mini",
				"Cannot reach api.telegram.org. The Mini App works; the bot will connect once the link is back.")
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

// notifyDSM writes to the DSM notification centre.
//
// It only works when the service runs as a package on the NAS itself: during a
// local run the tool simply is not there, and that is not an error. Permission
// may also be missing — then we stay quiet, as a notification is not worth
// stopping the service over.
func notifyDSM(title, message string) {
	const tool = "/usr/syno/bin/synodsmnotify"
	if _, err := os.Stat(tool); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, tool, "@administrators", title, message).Run()
}
