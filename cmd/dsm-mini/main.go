// Command dsm-mini is a Telegram bot with a Mini App for managing a Synology NAS.
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

// version is injected at build time: -ldflags "-X main.version=1.2.3".
// Support needs it — the first question about someone else's install is
// "which version".
var version = "dev"

func main() {
	// The container checks itself with this very binary: distroless has
	// neither a shell nor curl, and pulling them in for a health check means
	// pulling in everything that comes with them.
	if len(os.Args) > 1 && (os.Args[1] == "-healthcheck" || os.Args[1] == "--healthcheck") {
		os.Exit(healthcheck())
	}

	if err := run(); err != nil {
		// Printed directly rather than through the logger: a configuration
		// error is multi-line, and squeezed into a single line with \n it
		// becomes unreadable.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// healthcheck knocks on our own /healthz and returns an exit code:
// 0 — the service answers, 1 — it does not.
func healthcheck() int {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	// ":8080" means "listen everywhere"; we have to knock on a real address.
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
		fmt.Fprintf(os.Stderr, "healthz answered %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		// Configuration errors are printed as they are: someone is bringing
		// the service up for the first time and must see straight away what
		// is missing.
		return err
	}

	log := newLogger(cfg.LogLevel)
	log.Info("dsm-mini starting", "version", version)
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

	// Log in right away so that wrong credentials surface at startup instead
	// of on the first user request.
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
	log.Info("download station connected", "api", ds.Generation())
	files := filestation.New(client)
	sys := system.New(client)
	store2 := storage.New(client)
	machines := vmm.New(client)
	boxes := containers.New(client)

	// Database with settings and watcher state. It lives in a volume that
	// survives recreating the container.
	database, err := db.Open(databaseFile())
	if err != nil {
		return err
	}
	defer database.Close()
	log.Info("database opened", "path", databaseFile())
	settings := store.New(database)

	static, err := web.Assets()
	if err != nil {
		log.Warn("mini app is not embedded in the binary — only the bot will be available", "err", err)
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
		// Uploads to the NAS go through this server, so the write timeout is
		// deliberately generous.
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("http listening", "addr", cfg.ListenAddr, "public", cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "err", err)
			stop()
		}
	}()

	// The bot lives its own life: with no link to Telegram it waits for one,
	// and the Mini App keeps working meanwhile — it is opened by the client
	// on the phone.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runBot(ctx, cfg, ds, settings, log)
	}()

	// Keep the NAS state warm: otherwise the first person to open the app
	// waits out a full DSM round trip, over a second.
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiServer.KeepWarm(ctx, 20*time.Second)
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Warn("server closed with an error", "err", err)
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
