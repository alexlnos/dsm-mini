package dsm

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeNAS answers SYNO.API.Info and one ordinary call, and remembers what it
// was asked and with which cookie and token.
type fakeNAS struct {
	mu      sync.Mutex
	apis    []string
	cookies []string
	tokens  []string
	login   string // the whole answer to a login
}

func (f *fakeNAS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	api := r.Form.Get("api")
	f.mu.Lock()
	f.apis = append(f.apis, api)
	if c, err := r.Cookie("id"); err == nil {
		f.cookies = append(f.cookies, c.Value)
	}
	f.tokens = append(f.tokens, r.Header.Get("X-SYNO-TOKEN"))
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch api {
	case "SYNO.API.Auth":
		_, _ = io.WriteString(w, f.login)
	case "SYNO.API.Info":
		_, _ = io.WriteString(w, `{"success":true,"data":{"SYNO.Core.AppPortal.ReverseProxy":{"path":"entry.cgi","minVersion":1,"maxVersion":1,"requestFormat":"JSON"}}}`)
	default:
		_, _ = io.WriteString(w, `{"success":true,"data":{"entries":[]}}`)
	}
}

// The window has to say which refusal it was — a wrong password and a demand
// for a two-factor code need different advice — so the code has to survive,
// and code that only asks "was it a refusal" must keep working.
func TestLoginRefusalCarriesTheCode(t *testing.T) {
	nas := &fakeNAS{login: `{"success":false,"error":{"code":403}}`}
	srv := httptest.NewServer(nas)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, User: "u", Password: "p", Logger: quiet()})
	err := c.Login(context.Background())

	var refused *AuthError
	if !errors.As(err, &refused) || refused.Code != 403 {
		t.Fatalf("got %v, want an AuthError with code 403", err)
	}
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("%v does not match ErrAuth", err)
	}
}

// A client borrowed from a browser session goes in with the browser's cookie
// and DSM's CSRF token, and never signs in on its own: it has no password,
// and a login attempt with an empty one is a failed login in DSM's log.
func TestBorrowedSessionNeverSignsIn(t *testing.T) {
	nas := &fakeNAS{login: `{"success":false,"error":{"code":400}}`}
	srv := httptest.NewServer(nas)
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Cookie: "c00kie", SynoToken: "t0ken", Logger: quiet()})
	if err := c.Call(context.Background(), "SYNO.Core.AppPortal.ReverseProxy", "list", 1, nil, nil); err != nil {
		t.Fatal(err)
	}

	for _, api := range nas.apis {
		if api == "SYNO.API.Auth" {
			t.Fatalf("the client signed in by itself: %v", nas.apis)
		}
	}
	if len(nas.cookies) != len(nas.apis) {
		t.Fatalf("%d requests, %d with the cookie", len(nas.apis), len(nas.cookies))
	}
	for i := range nas.apis {
		if nas.cookies[i] != "c00kie" || nas.tokens[i] != "t0ken" {
			t.Fatalf("request %d went with cookie %q and token %q", i, nas.cookies[i], nas.tokens[i])
		}
	}
}
