package dsmui

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexlnos/dsm-mini/internal/config"
)

// screen is the settings screen over a temporary package var, with an
// administrator already let in and a count of how often it asked the service
// to start again.
type screen struct {
	dir      string
	h        http.Handler
	restarts *atomic.Int32
	nas      *fakeNAS
}

func newScreen(t *testing.T) *screen {
	t.Helper()
	sc := &screen{dir: t.TempDir(), restarts: new(atomic.Int32), nas: &fakeNAS{}}
	srv := New(Options{
		StateDir: sc.dir,
		Auth:     NewAuth(&stubVerifier{session: Session{User: "admin", IsAdmin: true}}, quietLog()),
		Restart:  func() { sc.restarts.Add(1) },
		Logger:   quietLog(),
	})
	srv.session = func(*http.Request) Caller { return sc.nas }
	sc.h = srv.Handler()
	return sc
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func (sc *screen) do(t *testing.T, method, path, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.AddCookie(&http.Cookie{Name: "id", Value: "c00kie"})
	r.Header.Set(TokenHeader, "t0ken")
	w := httptest.NewRecorder()
	sc.h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (sc *screen) file(t *testing.T) map[string]string {
	t.Helper()
	values, err := config.ReadFile(SettingsFile(sc.dir))
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// A fresh package has no settings file, and the screen is how it gets one.
func TestFirstSaveCreatesTheFileAndApplies(t *testing.T) {
	sc := newScreen(t)
	code, got := sc.do(t, http.MethodGet, "/dsm/admin/settings", "")
	if code != http.StatusOK {
		t.Fatalf("a package with no settings has to open the screen, got %d", code)
	}
	if got["defaults"].(map[string]any)["LISTEN_ADDR"] != "127.0.0.1:58080" {
		t.Fatalf("defaults %v", got["defaults"])
	}

	code, got = sc.do(t, http.MethodPut, "/dsm/admin/settings",
		`{"values":{"DSM_USER":"dsm-mini","DSM_PASSWORD":"a b$c","TELEGRAM_BOT_TOKEN":"1234567890:AAExampleTokenReplaceThisWithYours0"}}`)
	if code != http.StatusOK || got["applying"] != true {
		t.Fatalf("got %d %v", code, got)
	}
	if sc.restarts.Load() != 1 {
		t.Fatalf("restarts %d: saved settings have to take effect without restarting the package", sc.restarts.Load())
	}
	if v := sc.file(t); v["DSM_PASSWORD"] != "a b$c" || v["DSM_USER"] != "dsm-mini" {
		t.Fatalf("file %v", v)
	}
	// Written, but never sent back.
	if _, leaked := got["values"].(map[string]any)["DSM_PASSWORD"]; leaked {
		t.Fatal("the password went back to the browser")
	}
	if got["set"].(map[string]any)["DSM_PASSWORD"] != true {
		t.Fatalf("set %v", got["set"])
	}
}

// Starting again costs the bot its connection and the Mini App a second of
// silence, so a save that changes nothing must not do it.
func TestUnchangedSaveDoesNotRestart(t *testing.T) {
	sc := newScreen(t)
	if err := config.WriteFile(SettingsFile(sc.dir), map[string]string{"DSM_USER": "u", "DSM_PASSWORD": "p"}); err != nil {
		t.Fatal(err)
	}
	// The same user, and an empty password field — which means "keep it".
	code, got := sc.do(t, http.MethodPut, "/dsm/admin/settings", `{"values":{"DSM_USER":"u","DSM_PASSWORD":""}}`)
	if code != http.StatusOK || got["applying"] == true || sc.restarts.Load() != 0 {
		t.Fatalf("got %d %v, restarts %d", code, got, sc.restarts.Load())
	}
	if sc.file(t)["DSM_PASSWORD"] != "p" {
		t.Fatal("an empty secret field erased the password")
	}
}

func TestBadValueIsRefusedWithACode(t *testing.T) {
	sc := newScreen(t)
	code, got := sc.do(t, http.MethodPut, "/dsm/admin/settings", `{"values":{"PUBLIC_URL":"http://nas.example.com"}}`)
	if code != http.StatusBadRequest || got["field"] != "PUBLIC_URL" || got["code"] != config.NotHTTPS {
		t.Fatalf("got %d %v", code, got)
	}
	if sc.restarts.Load() != 0 {
		t.Fatal("a refused save restarted the service")
	}
}

// The state found on a live NAS after a build that ran as another user was
// replaced: config.env is there and cannot be opened. The screen has to open,
// with empty fields, and saving has to replace the file.
func TestUnreadableFileIsReplaced(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	sc := newScreen(t)
	path := SettingsFile(sc.dir)
	if err := os.WriteFile(path, []byte("DSM_USER='old'\n"), 0); err != nil {
		t.Fatal(err)
	}
	code, got := sc.do(t, http.MethodGet, "/dsm/admin/settings", "")
	if code != http.StatusOK {
		t.Fatalf("got %d: the screen is where this gets explained, so it has to open", code)
	}
	if got["values"].(map[string]any)["DSM_USER"] != "" {
		t.Fatalf("values %v", got["values"])
	}
	code, _ = sc.do(t, http.MethodPut, "/dsm/admin/settings", `{"values":{"DSM_USER":"new"}}`)
	if code != http.StatusOK || sc.file(t)["DSM_USER"] != "new" || sc.restarts.Load() != 1 {
		t.Fatalf("got %d, file %v, restarts %d", code, sc.file(t), sc.restarts.Load())
	}
}

func TestFindUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	dir := t.TempDir()
	if got := FindUnreadable(dir, "config.env", "dsm-mini.db"); got != nil {
		t.Fatalf("nothing there, got %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "dsm-mini.db"), nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.env"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := FindUnreadable(dir, "config.env", "dsm-mini.db")
	if got == nil || len(got.Files) != 1 || got.Files[0] != "dsm-mini.db" {
		t.Fatalf("got %+v", got)
	}
	if !strings.HasPrefix(got.Command, "sudo chown -R ") {
		t.Fatalf("command %q", got.Command)
	}
}

// fakeNAS stands for DSM as the administrator's own session sees it.
type fakeNAS struct {
	calls []string
	entry map[string]any
}

func (f *fakeNAS) Call(_ context.Context, api, method string, _ int, params map[string]any, out any) error {
	f.calls = append(f.calls, api+"."+method)
	if method == "create" {
		f.entry = params["entry"].(map[string]any)
	}
	if out == nil {
		return nil
	}
	switch api {
	case "SYNO.Core.AppPortal.ReverseProxy":
		return json.Unmarshal([]byte(`{"entries":[{"description":"other","frontend":{"fqdn":"nas.example.com","port":443,"protocol":1},"backend":{"fqdn":"localhost","port":58080}}]}`), out)
	case "SYNO.Core.DDNS.ExtIP":
		return json.Unmarshal([]byte(`[{"ip":"203.0.113.7","type":"IPv4"}]`), out)
	default:
		return json.Unmarshal([]byte(`{"records":[]}`), out)
	}
}

// Reading the network setup and creating a rule are the administrator's
// doing, through their own session: the service's account is not an
// administrator, and before the first save it does not exist at all.
func TestAddressWorksThroughTheAdministratorsSession(t *testing.T) {
	sc := newScreen(t)
	code, got := sc.do(t, http.MethodGet, "/dsm/admin/address", "")
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	if got["external_ip"] != "203.0.113.7" || got["matching"] == nil {
		t.Fatalf("got %v", got)
	}

	code, got = sc.do(t, http.MethodPost, "/dsm/admin/address/proxy", `{"fqdn":"Mini.Example.com"}`)
	if code != http.StatusOK || got["public"] != "https://mini.example.com" || got["applying"] != true {
		t.Fatalf("got %d %v", code, got)
	}
	backend := sc.nas.entry["backend"].(map[string]any)
	if backend["port"] != 58080 || backend["fqdn"] != "localhost" {
		t.Fatalf("the rule points at %v", backend)
	}
	if sc.file(t)["PUBLIC_URL"] != "https://mini.example.com" || sc.restarts.Load() != 1 {
		t.Fatalf("file %v, restarts %d", sc.file(t), sc.restarts.Load())
	}
}

func TestStatusSumsUp(t *testing.T) {
	s := NewStatus()
	if got := s.View().State; got != "starting" {
		t.Fatalf("nothing known yet: %s", got)
	}
	s.SetDSM(Link{State: LinkOK})
	s.SetBot(Link{State: LinkWaiting, Reason: ReasonUnreachable})
	if got := s.View().State; got != "starting" {
		t.Fatalf("telegram still being reached: %s", got)
	}
	s.SetBot(Link{State: LinkOK, Name: "nas_bot"})
	if got := s.View().State; got != "running" {
		t.Fatalf("both answered: %s", got)
	}
	s.SetDSM(Link{State: LinkFailed, Reason: ReasonAuth, Code: 400})
	if got := s.View().State; got != "failed" {
		t.Fatalf("a refused password: %s", got)
	}
	s.SetProblems([]config.Problem{{Key: "DSM_USER", Code: config.Missing}})
	if got := s.View().State; got != "setup" {
		t.Fatalf("settings missing: %s", got)
	}
}

func TestListenPortFallsBack(t *testing.T) {
	cases := []struct {
		file, running string
		want          int
	}{
		{"127.0.0.1:9090", "127.0.0.1:8080", 9090}, // the file wins: may not be applied yet
		{"", "127.0.0.1:8080", 8080},
		{"nonsense", "", config.DefaultPort},
	}
	for _, c := range cases {
		if got := listenPort(c.file, c.running); got != c.want {
			t.Fatalf("listenPort(%q, %q) = %d, want %d", c.file, c.running, got, c.want)
		}
	}
}
