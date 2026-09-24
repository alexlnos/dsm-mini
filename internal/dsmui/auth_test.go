package dsmui

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type stubVerifier struct {
	session Session
	err     error
	calls   atomic.Int32
}

func (s *stubVerifier) Identify(context.Context, string) (Session, error) {
	s.calls.Add(1)
	return s.session, s.err
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
