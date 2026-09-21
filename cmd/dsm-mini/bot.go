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
	"github.com/alexlnos/dsm-mini/internal/store"
)

// botRetry — пауза между попытками достучаться до Telegram.
const botRetry = 30 * time.Second

// runBot поднимает бота и всё, что от него зависит: оформление, приём
// сообщений и наблюдатель за завершёнными задачами.
//
// Недоступность Telegram не роняет сервис. Mini App открывает клиент на
// телефоне, а не NAS, поэтому приложение работает даже тогда, когда сам NAS
// до api.telegram.org не достаёт — так бывает, когда его трафик идёт мимо
// VPN. Бот подключится сам, как только связь появится.
func runBot(ctx context.Context, cfg *config.Config, ds downloadstation.Station,
	settings *store.Store, log *slog.Logger) {

	tgBot := waitForBot(ctx, cfg, ds, settings, log)
	if tgBot == nil {
		return
	}

	var wg sync.WaitGroup

	// Вид бота задаётся из кода на каждом языке словаря: имя, описания и
	// команды не живут только в @BotFather. Значения читаются перед записью,
	// поэтому перезапуск не тратит лимиты Telegram. Аватар Bot API менять не
	// умеет — он ставится вручную.
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

// waitForBot создаёт бота, повторяя попытки, пока Telegram недоступен.
//
// Возвращает nil, только если ждать бессмысленно: сервис остановлен или
// токен отвергнут.
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
				log.Info("связь с Telegram появилась")
				notifyDSM("dsm-mini", "Связь с Telegram восстановлена, бот работает.")
			}
			return tgBot
		}

		// Неверный токен повтором не лечится.
		if errors.Is(err, bot.ErrBadToken) {
			log.Error("бот не запустится", "err", err)
			notifyDSM("dsm-mini", "Telegram отклонил токен бота. Проверьте настройки пакета.")
			return nil
		}

		// О недоступности сообщаем один раз: повторять её каждые полминуты
		// в Центре уведомлений — значит сделать его бесполезным.
		if !warned {
			warned = true
			log.Warn("Telegram недоступен, продолжаю без бота", "err", err, "повтор", botRetry)
			notifyDSM("dsm-mini",
				"Нет связи с api.telegram.org. Mini App работает, бот подключится сам, когда связь появится.")
		} else {
			log.Debug("Telegram всё ещё недоступен", "err", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(botRetry):
		}
	}
}

// notifyDSM пишет в Центр уведомлений DSM.
//
// Работает, только когда сервис запущен пакетом на самом NAS: в контейнере и
// при локальном запуске команды просто нет, и это не ошибка. Права на неё
// тоже может не быть — тогда молчим, уведомление не стоит остановки службы.
func notifyDSM(title, message string) {
	const tool = "/usr/syno/bin/synodsmnotify"
	if _, err := os.Stat(tool); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, tool, "@administrators", title, message).Run()
}
