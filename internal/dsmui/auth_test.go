package dsmui

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

type stubVerifier struct {
	session Session
	err     error
	calls   atomic.Int32
	// The last pair handed over, so a test can check what travelled.
	mu            sync.Mutex
	cookie, token string
}

func (s *stubVerifier) Identify(_ context.Context, cookie, token string) (Session, error) {
	s.calls.Add(1)
	s.mu.Lock()
	s.cookie, s.token = cookie, token
	s.mu.Unlock()
	return s.session, s.err
}

func (s *stubVerifier) seen() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cookie, s.token
}

func guarded(v Verifier) (http.Handler, *atomic.Int32) {
	var reached atomic.Int32
	a := NewAuth(v, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return a.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached.Add(1)
	})), &reached
}

func request(h http.Handler, cookie string) int {
	r := httptest.NewRequest(http.MethodGet, "/dsm/admin/settings", nil)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "id", Value: cookie})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// DSM serves /webman/3rdparty/ to anyone — checked against a live NAS — so a
// request with no session must get nowhere near the settings.
func TestNoCookieIsRefused(t *testing.T) {
	h, reached := guarded(&stubVerifier{session: Session{User: "root", IsAdmin: true}})
	if code := request(h, ""); code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", code)
	}
	if reached.Load() != 0 {
		t.Fatal("a request without a session reached the handler")
	}
}

func TestUnknownSessionIsRefused(t *testing.T) {
	h, reached := guarded(&stubVerifier{err: ErrNotLoggedIn})
	if code := request(h, "stale"); code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", code)
	}
	if reached.Load() != 0 {
		t.Fatal("an unknown session reached the handler")
	}
}

// Being logged in is not enough: the screen edits the bot token and the NAS
// password, so an ordinary account has no business here.
func TestNonAdminIsRefused(t *testing.T) {
	h, reached := guarded(&stubVerifier{session: Session{User: "guest", IsAdmin: false}})
	if code := request(h, "valid"); code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", code)
	}
	if reached.Load() != 0 {
		t.Fatal("a non-administrator reached the handler")
	}
}

func TestAdminGoesThrough(t *testing.T) {
	h, reached := guarded(&stubVerifier{session: Session{User: "alex", IsAdmin: true}})
	if code := request(h, "valid"); code != http.StatusOK {
		t.Fatalf("got %d, want 200", code)
	}
	if reached.Load() != 1 {
		t.Fatal("an administrator did not reach the handler")
	}
}

// Opening the screen fires several requests at once; they should cost DSM one
// round trip, not one each.
func TestVerdictIsCached(t *testing.T) {
	v := &stubVerifier{session: Session{User: "alex", IsAdmin: true}}
	h, _ := guarded(v)
	for i := 0; i < 5; i++ {
		if code := request(h, "valid"); code != http.StatusOK {
			t.Fatalf("call %d: got %d, want 200", i, code)
		}
	}
	if got := v.calls.Load(); got != 1 {
		t.Fatalf("dsm was asked %d times, want 1", got)
	}
}

// A different cookie is a different person: the cache must not answer for one
// session with the verdict given for another.
func TestCacheIsPerSession(t *testing.T) {
	v := &stubVerifier{session: Session{User: "alex", IsAdmin: true}}
	h, _ := guarded(v)
	request(h, "one")
	request(h, "two")
	if got := v.calls.Load(); got != 2 {
		t.Fatalf("dsm was asked %d times, want 2", got)
	}
}

// DSM has CSRF protection on by default, and with it a session made in a
// browser is refused unless the call carries X-SYNO-TOKEN. The screen reads
// the token from the desktop around it; this checks it gets that far.
func TestTokenReachesTheVerifier(t *testing.T) {
	v := &stubVerifier{session: Session{User: "alex", IsAdmin: true}}
	a := NewAuth(v, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := a.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	r := httptest.NewRequest(http.MethodGet, "/dsm/admin/settings", nil)
	r.AddCookie(&http.Cookie{Name: "id", Value: "cookie-value"})
	r.Header.Set(TokenHeader, "token-value")
	h.ServeHTTP(httptest.NewRecorder(), r)

	cookie, token := v.seen()
	if cookie != "cookie-value" || token != "token-value" {
		t.Fatalf("verifier saw cookie=%q token=%q", cookie, token)
	}
}

// A different token is a different request context, so the cached verdict for
// one must not answer for the other.
func TestCacheIsPerToken(t *testing.T) {
	v := &stubVerifier{session: Session{User: "alex", IsAdmin: true}}
	a := NewAuth(v, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := a.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for _, token := range []string{"one", "two"} {
		r := httptest.NewRequest(http.MethodGet, "/dsm/admin/settings", nil)
		r.AddCookie(&http.Cookie{Name: "id", Value: "same"})
		r.Header.Set(TokenHeader, token)
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	if got := v.calls.Load(); got != 2 {
		t.Fatalf("dsm was asked %d times, want 2", got)
	}
}
