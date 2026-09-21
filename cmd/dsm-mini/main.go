// Команда dsm-mini — телеграм-бот с Mini App для управления Synology NAS.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexlnos/dsm-mini/internal/bot"
	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/db"
	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/containers"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
	"github.com/alexlnos/dsm-mini/internal/dsm/storage"
	"github.com/alexlnos/dsm-mini/internal/dsm/system"
	"github.com/alexlnos/dsm-mini/internal/dsm/vmm"
	"github.com/alexlnos/dsm-mini/internal/httpapi"
	"github.com/alexlnos/dsm-mini/internal/store"
	"github.com/alexlnos/dsm-mini/internal/web"
)

// version подставляется при сборке: `-ldflags "-X main.version=1.2.3"`.
// Нужна в поддержке — первый вопрос к чужой установке «какая версия».
var version = "dev"

func main() {
	// Контейнер проверяет себя этим же бинарником: в distroless нет ни
	// shell, ни curl, а тянуть их ради healthcheck — значит тянуть и всё,
	// что к ним прилагается.
	if len(os.Args) > 1 && (os.Args[1] == "-healthcheck" || os.Args[1] == "--healthcheck") {
		os.Exit(healthcheck())
	}

	if err := run(); err != nil {
		// Печатаем напрямую, а не через журнал: ошибка конфигурации
		// многострочная, и в виде одной строки с \n она нечитаема.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// healthcheck стучится в собственный /healthz и возвращает код выхода:
// 0 — сервис отвечает, 1 — нет.
func healthcheck() int {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	// «:8080» — это «слушать везде»; стучаться надо в конкретный адрес.
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthz ответил %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		// Ошибки конфигурации печатаем как есть: человек поднимает сервис
		// впервые и должен сразу увидеть, чего не хватает.
		return err
	}

	log := newLogger(cfg.LogLevel)
	log.Info("dsm-mini запускается", "версия", version)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := dsm.New(dsm.Options{
		BaseURL:     cfg.DSMURL,
		User:        cfg.DSMUser,
		Password:    cfg.DSMPassword,
		OTP:         cfg.DSMOTP,
		InsecureTLS: cfg.DSMInsecure,
		Logger:      log,
	})

	// Входим сразу, чтобы неверные учётные данные всплыли при запуске, а не
	// при первом обращении пользователя.
	loginCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = client.Login(loginCtx)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Logout(shutdown)
	}()

	ds, err := downloadstation.New(ctx, client)
	if err != nil {
		return err
	}
	log.Info("Download Station подключён", "api", ds.Generation())
	files := filestation.New(client)
	sys := system.New(client)
	store2 := storage.New(client)
	machines := vmm.New(client)
	boxes := containers.New(client)

	// База с настройками и состоянием наблюдателя. Лежит в томе, который
	// переживает пересоздание контейнера.
	database, err := db.Open(databaseFile())
	if err != nil {
		return err
	}
	defer database.Close()
	log.Info("база открыта", "path", databaseFile())
	settings := store.New(database)

	tgBot, err := bot.New(bot.Options{
		Token:          cfg.BotToken,
		AllowedUserIDs: cfg.AllowedUserIDs,
		Downloads:      ds,
		Settings:       settings,
		PublicURL:      cfg.PublicURL,
		Logger:         log,
	})
	if err != nil {
		return err
	}

	// Вид бота задаётся из кода на каждом языке словаря: имя, описания и
	// команды не живут только в @BotFather. Значения читаются перед записью,
	// поэтому перезапуск не тратит лимиты Telegram. Аватар Bot API менять не
	// умеет — он ставится вручную.
	//
	// Делается в фоне: на десяти языках это два десятка обращений к Telegram,
	// а до их окончания приложение уже должно отвечать.
	go func() {
		setupCtx, cancelSetup := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelSetup()
		tgBot.ConfigureAll(setupCtx)
	}()

	static, err := web.Assets()
	if err != nil {
		log.Warn("Mini App не встроено в бинарник — будет доступен только бот", "err", err)
	}

	apiServer := httpapi.New(httpapi.Options{
		BotToken:       cfg.BotToken,
		AllowedUserIDs: cfg.AllowedUserIDs,
		Downloads:      ds,
		Files:          files,
		System:         sys,
		Storage:        store2,
		VMs:            machines,
		Containers:     boxes,
		Settings:       settings,
		Static:         static,
		Logger:         log,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Загрузка файла на NAS идёт через этот сервер, поэтому запись
		// не ограничиваем жёстко.
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("HTTP слушает", "addr", cfg.ListenAddr, "public", cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP сервер остановился", "err", err)
			stop()
		}
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

	// Состояние NAS держим наготове: иначе первый, кто откроет приложение,
	// ждёт полный обход DSM больше секунды.
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiServer.KeepWarm(ctx, 20*time.Second)
	}()

	<-ctx.Done()
	log.Info("останавливаемся")

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Warn("сервер закрылся с ошибкой", "err", err)
	}

	wg.Wait()
	return nil
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func databaseFile() string {
	return filepath.Join(stateDir(), "dsm-mini.db")
}

func stateDir() string {
	dir := os.Getenv("STATE_DIR")
	if dir == "" {
		dir = "/data"
	}
	return dir
}
