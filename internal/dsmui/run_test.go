package dsmui

import (
	"context"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/db"
	"github.com/alexlnos/dsm-mini/internal/store"
)

// runScreen is the screen with a database and a status of its own, for the
// Start and Stop buttons.
func runScreen(t *testing.T) (*screen, *Status, *store.Store) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	st, status := store.New(database), NewStatus()
	sc := &screen{dir: t.TempDir(), restarts: new(atomic.Int32), nas: &fakeNAS{}}
	srv := New(Options{
		StateDir: sc.dir,
		Store:    st,
		Status:   status,
		Auth:     NewAuth(&stubVerifier{session: Session{User: "admin", IsAdmin: true}}, quietLog()),
		Restart:  func() { sc.restarts.Add(1) },
		Logger:   quietLog(),
	})
	srv.session = func(*http.Request) Caller { return sc.nas }
	sc.h = srv.Handler()
	return sc, status, st
}

func TestStartAndStop(t *testing.T) {
	sc, _, st := runScreen(t)
	ctx := context.Background()

	code, got := sc.do(t, http.MethodPost, "/dsm/admin/run", `{"enabled":true}`)
	if code != http.StatusOK || got["enabled"] != true {
		t.Fatalf("start: %d %v", code, got)
	}
	if on, set, _ := st.AppEnabled(ctx); !on || !set || sc.restarts.Load() != 1 {
		t.Fatalf("after start: on %v, set %v, restarts %d", on, set, sc.restarts.Load())
	}

	code, got = sc.do(t, http.MethodPost, "/dsm/admin/run", `{"enabled":false}`)
	if code != http.StatusOK || got["enabled"] != false {
		t.Fatalf("stop: %d %v", code, got)
	}
	if on, _, _ := st.AppEnabled(ctx); on || sc.restarts.Load() != 2 {
		t.Fatalf("after stop: on %v, restarts %d", on, sc.restarts.Load())
	}
}

// "Switched on" must never mean "switched on and doing nothing": Start is
// refused while the settings are not enough to start on. Stop always works.
func TestStartWaitsForTheSettings(t *testing.T) {
	sc, status, st := runScreen(t)
	status.SetProblems([]config.Problem{{Key: "DSM_USER", Code: config.Missing}})

	code, got := sc.do(t, http.MethodPost, "/dsm/admin/run", `{"enabled":true}`)
	if code != http.StatusBadRequest || got["code"] != config.Missing {
		t.Fatalf("got %d %v", code, got)
	}
	if _, set, _ := st.AppEnabled(context.Background()); set || sc.restarts.Load() != 0 {
		t.Fatalf("a refused start was recorded: set %v, restarts %d", set, sc.restarts.Load())
	}

	if code, got := sc.do(t, http.MethodPost, "/dsm/admin/run", `{"enabled":false}`); code != http.StatusOK {
		t.Fatalf("stop: %d %v", code, got)
	}
}

func TestRunNeedsAnAnswer(t *testing.T) {
	sc, _, _ := runScreen(t)
	if code, _ := sc.do(t, http.MethodPost, "/dsm/admin/run", `{}`); code != http.StatusBadRequest {
		t.Fatalf("a body without enabled got %d", code)
	}
}

// The way the Mini App is reached is kept, and "the bot only" takes the
// address away whatever the field held: the choice a person reads and the
// address the service goes by must not disagree.
func TestAccessIsKeptAndBotOnlyDropsTheAddress(t *testing.T) {
	sc, _, st := runScreen(t)
	ctx := context.Background()

	code, got := sc.do(t, http.MethodPut, "/dsm/admin/settings",
		`{"values":{"PUBLIC_URL":"https://mini.example.com"},"access":"domain"}`)
	if code != http.StatusOK || got["access"] != "domain" {
		t.Fatalf("got %d %v", code, got)
	}
	if mode, _ := st.Access(ctx); mode != "domain" || sc.file(t)["PUBLIC_URL"] != "https://mini.example.com" {
		t.Fatalf("access %q, file %v", mode, sc.file(t))
	}

	code, got = sc.do(t, http.MethodPut, "/dsm/admin/settings",
		`{"values":{"PUBLIC_URL":"https://mini.example.com"},"access":"bot"}`)
	if code != http.StatusOK || got["access"] != "bot" {
		t.Fatalf("got %d %v", code, got)
	}
	if sc.file(t)["PUBLIC_URL"] != "" {
		t.Fatalf("the bot only kept the address: %v", sc.file(t))
	}

	if code, _ := sc.do(t, http.MethodPut, "/dsm/admin/settings", `{"access":"everything"}`); code != http.StatusBadRequest {
		t.Fatalf("an unknown way got %d", code)
	}
}
