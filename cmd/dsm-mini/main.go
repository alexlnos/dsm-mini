// Command dsm-mini is a Telegram bot with a Mini App for managing a Synology NAS.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
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
	"github.com/alexlnos/dsm-mini/internal/dsmnotify"
	"github.com/alexlnos/dsm-mini/internal/dsmui"
	"github.com/alexlnos/dsm-mini/internal/httpapi"
	"github.com/alexlnos/dsm-mini/internal/store"
	"github.com/alexlnos/dsm-mini/internal/web"
)

// version is injected at build time: -ldflags "-X main.version=1.2.3".
// Support needs it — the first question about someone else's install is
// "which version".
var version = "dev"

const (
	// dsmRetry is how long to wait before reaching DSM again when it did not
	// answer at all — during boot it comes up after the packages do.
	dsmRetry = 30 * time.Second
	// databaseName is the database in the package var.
	databaseName = "dsm-mini.db"
)

func main() {
	// Called by the package's uninstall script, not by people: the one thing
	// that has to happen when the package goes away and that nothing else
	// would do. See internal/dsmnotify.Remove.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "unregister-webhook":
			if err := unregisterWebhook(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		default:
			// Anything unknown is refused rather than ignored. An older binary
			// ignored its arguments, so a script asking it for a one-off job
			// would have started the whole service instead — during an
			// uninstall, that is an uninstall that never finishes.
			fmt.Fprintf(os.Stderr, "unknown command %q; run without arguments to start the service\n", os.Args[1])
			os.Exit(2)
		}
	}

	if err := runService(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// errRestart ends one run of the service so that the next one starts with the
// settings just saved in the DSM window.
var errRestart = errors.New("the settings changed")

// runService keeps the service up whatever state its settings are in.
//
// Nothing is asked at installation. A fresh package starts with no settings,
// serves the settings window in the DSM main menu and waits. Saving there
// starts the service's insides again with the new settings, inside this
// process: Package Center is not involved, and nobody has to find its
// Restart button. A refused password or token does not stop the process
// either — it is shown in the window, which is where it gets fixed. A package
// that exits on a bad setting is shown by Package Center as stopped, with a
// Repair button that repairs nothing, and the reason only in a log.
func runService() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	level := new(slog.LevelVar)
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)
	log.Info("dsm-mini starting", "version", version)

	for {
		err := run(ctx, log, level)
		if !errors.Is(err, errRestart) || ctx.Err() != nil {
			return err
		}
		log.Info("starting again with the settings saved in the dsm window")
	}
}

// run is one life of the service with one set of settings. It ends with nil
// when the process is told to stop, and with errRestart when the settings
// window saved new settings.
func run(parent context.Context, log *slog.Logger, level *slog.LevelVar) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

	dir := stateDir()
	status := dsmui.NewStatus()

	cfg, problems, err := config.Load(dsmui.SettingsFile(dir))
	if err != nil && !os.IsPermission(err) {
		return err
	}
	level.Set(logLevel(cfg.LogLevel))
	if len(problems) > 0 {
		status.SetProblems(problems)
		log.Info("not set up yet: waiting for the settings window in the DSM main menu",
			"missing", config.Describe(problems))
	}

	// Files an earlier installation left under another package user. The
	// settings file waits for the window, where it is either handed back
	// over SSH or typed in again; see openDatabase for the database.
	unreadable := dsmui.FindUnreadable(dir, "config.env", databaseName)
	ready := len(problems) == 0 && err == nil
	if unreadable != nil {
		log.Warn("files of an earlier installation cannot be read",
			"files", unreadable.Files, "owner", unreadable.Owner, "fix", unreadable.Command)
		if !ready {
			status.SetUnreadable(unreadable)
		}
	}

	database, err := openDatabase(dir, ready, log)
	if err != nil {
		return err
	}
	var settings *store.Store
	if database != nil {
		defer database.Close()
		settings = store.New(database)
	}

	// The settings screen inside DSM, served in every state: it is how the
	// service gets set up in the first place. It authorises itself against
	// DSM's own session cookie — /webman/3rdparty/ is served to anyone,
	// checked against a live NAS, so the path guards nothing.
	admin := dsmui.New(dsmui.Options{
		StateDir:    dir,
		Store:       settings,
		DSMURL:      cfg.DSMURL,
		InsecureTLS: cfg.DSMInsecure,
		ListenAddr:  cfg.ListenAddr,
		Status:      status,
		Restart:     func() { cancel(errRestart) },
		Auth:        dsmui.NewAuth(dsmui.NewVerifier(cfg.DSMURL, cfg.DSMInsecure), log),
		Logger:      log,
	})

	// The Mini App and its API take over once DSM has answered; until then
	// everything but the settings screen gets a short "not yet".
	var app atomic.Value
	root := http.NewServeMux()
	root.Handle("/dsm/admin/", admin.Handler())
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := app.Load().(http.Handler); ok {
			h.ServeHTTP(w, r)
			return
		}
		notReady(w, r, status)
	}))

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		// Nothing can be served, the settings screen included, so this one
		// does stop the process — with the reason in Package Center's log.
		return fmt.Errorf("cannot listen on %s: %w", cfg.ListenAddr, err)
	}
	writeBackendConf(ln.Addr(), log)

	srv := &http.Server{
		Handler:           httpapi.SecurityHeaders(root),
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
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "err", err)
			cancel(err)
		}
	}()

	if ready {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runApp(ctx, cfg, settings, status, &app, log)
		}()
	}

	<-ctx.Done()
	if errors.Is(context.Cause(ctx), errRestart) {
		log.Info("stopping to apply the new settings")
	} else {
		log.Info("shutting down")
	}

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Warn("server closed with an error", "err", err)
	}
	wg.Wait()

	switch cause := context.Cause(ctx); {
	case errors.Is(cause, errRestart):
		return errRestart
	case parent.Err() != nil:
		return nil
	default:
		return cause
	}
}

// runApp connects to DSM and Telegram and serves the Mini App, for as long as
// ctx lives. Every failure it meets ends up in the status the settings window
// shows, not in the process exit code.
func runApp(ctx context.Context, cfg *config.Config, settings *store.Store,
	status *dsmui.Status, app *atomic.Value, log *slog.Logger) {

	client := dsm.New(dsm.Options{
		BaseURL:     cfg.DSMURL,
		User:        cfg.DSMUser,
		Password:    cfg.DSMPassword,
		OTP:         cfg.DSMOTP,
		InsecureTLS: cfg.DSMInsecure,
		Logger:      log,
	})
	defer func() {
		logout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Logout(logout)
	}()

	if !signIn(ctx, client, cfg, status, log) {
		return
	}
	ds := waitForDownloadStation(ctx, client, status, log)
	if ds == nil {
		return
	}
	status.SetDSM(dsmui.Link{State: dsmui.LinkOK})
	log.Info("download station connected", "api", ds.Generation())

	files := filestation.New(client)
	sys := system.New(client)
	store2 := storage.New(client)
	machines := vmm.New(client)
	boxes := containers.New(client)

	// What DSM itself announces goes through a webhook it calls on this
	// machine. The secret is made on first start and never typed by anyone.
	secret, err := settings.WebhookSecret(ctx)
	if err != nil {
		log.Error("cannot read the webhook secret", "err", err)
		return
	}
	receiver := dsmnotify.NewReceiver(secret, settings, cfg.AllowedUserIDs, log)

	var wg sync.WaitGroup

	// Registering it is not worth failing over: the service is still a
	// working bot and Mini App without DSM's notifications, and the next
	// start tries again.
	wg.Add(1)
	go func() {
		defer wg.Done()
		regCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := dsmnotify.Ensure(regCtx, client, webhookURL(cfg.ListenAddr), secret, log); err != nil {
			log.Warn("dsm notifications will not arrive", "err", err)
		}
	}()

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
		DSMNotify:      receiver,
		Logger:         log,
	})
	app.Store(apiServer.Handler())

	// The bot lives its own life: with no link to Telegram it waits for one,
	// and the Mini App keeps working meanwhile — it is opened by the client
	// on the phone.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runBot(ctx, cfg, ds, settings, receiver, status, log)
	}()

	// Keep the NAS state warm: otherwise the first person to open the app
	// waits out a full DSM round trip, over a second.
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiServer.KeepWarm(ctx, 20*time.Second)
	}()

	wg.Wait()
}

// signIn signs in to DSM, retrying for as long as DSM does not answer.
//
// A refusal is not retried. Repeating a wrong password only adds failed
// sign-ins to DSM's log and to the count its auto block goes by, and the
// person who can fix it will do so in the settings window — which starts
// everything again anyway.
func signIn(ctx context.Context, client *dsm.Client, cfg *config.Config,
	status *dsmui.Status, log *slog.Logger) bool {

	for {
		status.SetDSM(dsmui.Link{State: dsmui.LinkWaiting})
		loginCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := client.Login(loginCtx)
		cancel()
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}

		var refused *dsm.AuthError
		if errors.As(err, &refused) {
			status.SetDSM(dsmui.Link{State: dsmui.LinkFailed, Reason: dsmui.ReasonAuth, Code: refused.Code})
			log.Error("dsm refused the sign-in; waiting for new settings in the dsm window",
				"user", cfg.DSMUser, "err", err)
			<-ctx.Done()
			return false
		}

		status.SetDSM(dsmui.Link{State: dsmui.LinkWaiting, Reason: dsmui.ReasonUnreachable})
		log.Warn("cannot reach dsm, trying again", "url", cfg.DSMURL, "err", err, "retry", dsmRetry)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(dsmRetry):
		}
	}
}

// waitForDownloadStation connects to Download Station, which may be stopped
// or not installed yet: the service waits for it rather than giving up.
func waitForDownloadStation(ctx context.Context, client *dsm.Client,
	status *dsmui.Status, log *slog.Logger) downloadstation.Station {

	warned := false
	for {
		ds, err := downloadstation.New(ctx, client)
		if err == nil {
			return ds
		}
		if ctx.Err() != nil {
			return nil
		}
		status.SetDSM(dsmui.Link{State: dsmui.LinkWaiting, Reason: dsmui.ReasonDownloadStation})
		if !warned {
			warned = true
			log.Warn("download station is not available, waiting for it", "err", err, "retry", 2*dsmRetry)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * dsmRetry):
		}
	}
}

// openDatabase opens the database in the package var — which is what
// survives an upgrade, unlike target.
//
// A database that belongs to an earlier installation's user is left alone
// while the service waits to be set up: the owner may yet hand the files back
// over SSH and keep everything. Once the service has settings to run on, it
// is set aside instead, kept under another name: what it holds is people's
// preferences and pinned folders, and a service that stays down for them is
// worse than one that starts afresh. Nil with no error means "not opened".
func openDatabase(dir string, ready bool, log *slog.Logger) (*sql.DB, error) {
	path := filepath.Join(dir, databaseName)
	if dsmui.FindUnreadable(dir, databaseName) != nil {
		if !ready {
			return nil, nil
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			from := path + suffix
			if _, err := os.Lstat(from); err != nil {
				continue
			}
			if err := os.Rename(from, from+".unreadable"); err != nil {
				return nil, fmt.Errorf("cannot set aside the unreadable database: %w", err)
			}
		}
		log.Warn("the database of an earlier installation cannot be read: set aside, starting afresh",
			"kept_as", path+".unreadable")
	}

	database, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	log.Info("database opened", "path", path)
	return database, nil
}

// notReady answers everything but the settings screen until the Mini App is
// up. The one to see it is somebody opening the public address before the
// package is set up, or while DSM is not letting the service in.
func notReady(w http.ResponseWriter, r *http.Request, status *dsmui.Status) {
	state := status.View().State
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "{\"status\":%q}\n", state)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Retry-After", "10")
	w.WriteHeader(http.StatusServiceUnavailable)
	if state == "setup" {
		fmt.Fprintln(w, "DSM mini is not set up yet: open it from the DSM main menu.")
		return
	}
	fmt.Fprintln(w, "DSM mini is starting; its window in the DSM main menu says how it is going.")
}

// writeBackendConf tells the settings window which port to reach the service
// on. The window's CGI runs as DSM's web server and cannot read config.env,
// which is 0600, so the port — and only the port — goes into a file of its own
// next to it. The service writes it rather than the start script because the
// window can move the port without the package being restarted.
func writeBackendConf(addr net.Addr, log *slog.Logger) {
	dir := os.Getenv("UI_DIR")
	tcp, ok := addr.(*net.TCPAddr)
	if dir == "" || !ok {
		return
	}
	path := filepath.Join(dir, "backend.conf")
	tmp := path + ".new"
	body := "PORT=" + strconv.Itoa(tcp.Port) + "\n"
	err := os.WriteFile(tmp, []byte(body), 0o644)
	if err == nil {
		// World-readable on purpose, whatever the umask: the reader is
		// another user, and the file holds a port number and nothing else.
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		log.Warn("the settings window may not find the service", "file", path, "err", err)
	}
}

func logLevel(name string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(name)); err != nil {
		return slog.LevelInfo
	}
	return l
}

// unregisterWebhook removes the DSM notification webhook this service
// registered. It signs in with the same settings the service uses and does
// nothing else.
func unregisterWebhook() error {
	cfg, _, err := config.Load(dsmui.SettingsFile(stateDir()))
	if err != nil {
		return err
	}
	if cfg.DSMUser == "" || cfg.DSMPassword == "" {
		// Never set up, so never registered: nothing to take away.
		return nil
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	client := dsm.New(dsm.Options{
		BaseURL:     cfg.DSMURL,
		User:        cfg.DSMUser,
		Password:    cfg.DSMPassword,
		OTP:         cfg.DSMOTP,
		InsecureTLS: cfg.DSMInsecure,
		Logger:      log,
	})
	if err := client.Login(ctx); err != nil {
		return err
	}
	defer func() { _ = client.Logout(context.Background()) }()

	return dsmnotify.Remove(ctx, client, log)
}

// webhookURL is where DSM should call. It is built from the address the
// service listens on rather than configured: DSM runs on the same machine, so
// the call goes over loopback whatever interface the service was bound to.
func webhookURL(listenAddr string) string {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil || port == "" {
		port = strconv.Itoa(config.DefaultPort)
	}
	return "http://127.0.0.1:" + port + "/dsm/notify"
}

func stateDir() string {
	dir := os.Getenv("STATE_DIR")
	if dir == "" {
		dir = "/data"
	}
	return dir
}
