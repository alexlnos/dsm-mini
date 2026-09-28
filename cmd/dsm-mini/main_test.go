package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/db"
	"github.com/alexlnos/dsm-mini/internal/store"
)

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// A package nobody has set up yet: no settings at all, only the port so the
// test does not collide with anything. It has to stay up, serve the settings
// screen's side, tell the window's CGI where it is, and stop cleanly.
func TestFreshPackageWaitsToBeSetUp(t *testing.T) {
	state, ui := t.TempDir(), t.TempDir()
	addr := freePort(t)
	if err := os.WriteFile(filepath.Join(state, "config.env"), []byte("LISTEN_ADDR='"+addr+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STATE_DIR", state)
	t.Setenv("UI_DIR", ui)
	for _, k := range []string{"DSM_USER", "DSM_PASSWORD", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "PUBLIC_URL", "DSM_URL"} {
		t.Setenv(k, "")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	level := new(slog.LevelVar)
	go func() { done <- run(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), level) }()

	var health string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			health = string(b)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(health, `"stopped"`) {
		t.Fatalf("healthz said %q, want the stopped state", health)
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the Mini App answered %d before the package was set up", resp.StatusCode)
	}

	conf, err := os.ReadFile(filepath.Join(ui, "backend.conf"))
	_, port, _ := net.SplitHostPort(addr)
	if err != nil || string(conf) != "PORT="+port+"\n" {
		t.Fatalf("backend.conf %q, err %v", conf, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a stop asked for by the system returned %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the service did not stop")
	}
}

// The first start of the version with the Start button settles whether the
// app runs: an installation with its settings was running and keeps running
// through the upgrade, a fresh one waits for Start. After that the choice is
// the window's, whatever the settings say.
func TestFirstStartSettlesTheSwitch(t *testing.T) {
	ctx := context.Background()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	open := func(t *testing.T) *store.Store {
		database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		return store.New(database)
	}

	upgraded := open(t)
	if !switchedOn(ctx, upgraded, true, quiet) {
		t.Fatal("an installation that was set up stopped on the upgrade")
	}
	fresh := open(t)
	if switchedOn(ctx, fresh, false, quiet) {
		t.Fatal("a fresh installation started on its own")
	}
	// Set up afterwards, it still waits for Start.
	if switchedOn(ctx, fresh, true, quiet) {
		t.Fatal("saving the settings started the app without Start")
	}
	if err := fresh.SetAppEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !switchedOn(ctx, fresh, true, quiet) {
		t.Fatal("Start did not stick")
	}
	if switchedOn(ctx, nil, true, quiet) {
		t.Fatal("no database, and yet switched on")
	}
}
