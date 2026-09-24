// Package dsmui serves the settings screen that lives inside DSM.
//
// The screen is a page under /webman/3rdparty/dsm-mini/, and DSM serves that
// path to anyone: a request without a session gets 200, checked against a live
// NAS. So the path proves nothing, and every call is authorised here instead —
// by taking the DSM session cookie the browser already sends and asking DSM
// whose it is.
package dsmui

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Session is what DSM says about the cookie it was shown.
type Session struct {
	User    string
	IsAdmin bool
}

// Verifier asks DSM to identify a session cookie.
type Verifier interface {
	Identify(ctx context.Context, cookie, token string) (Session, error)
}

// TokenHeader is where the page puts DSM's CSRF token. It reads it from the
// desktop around it — the screen is an iframe of the same origin — and
// api.cgi carries it here.
const TokenHeader = "X-Syno-Token"

// Auth guards the endpoints the screen calls.
//
// Answers are cached for a few seconds: opening the screen fires a handful of
// requests at once, and each one would otherwise cost a round trip to DSM.
// The window is short on purpose — an account that loses its administrator
// rights should not keep them for long.
type Auth struct {
	verifier Verifier
	log      *slog.Logger

	mu     sync.Mutex
	recent map[string]cached
}

type cached struct {
	session Session
	until   time.Time
}

const cacheFor = 10 * time.Second

// NewAuth builds the guard.
func NewAuth(v Verifier, log *slog.Logger) *Auth {
	return &Auth{verifier: v, log: log, recent: map[string]cached{}}
}

type contextKey struct{}

// SessionFrom returns who the request belongs to.
func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(contextKey{}).(Session)
	return s, ok
}

// Middleware lets through administrators of this NAS and nobody else.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := dsmCookie(r)
		if cookie == "" {
			a.refuse(w, r, "no dsm session cookie")
			return
		}
		session, err := a.identify(r.Context(), cookie, r.Header.Get(TokenHeader))
		if err != nil {
			a.refuse(w, r, "dsm did not recognise the session: "+err.Error())
			return
		}
		if !session.IsAdmin {
			a.refuse(w, r, "not an administrator: "+session.User)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, session)))
	})
}

// refuse says the same thing to everyone. What exactly did not match goes to
// the log: spelling it out in the answer would help whoever is guessing.
func (a *Auth) refuse(w http.ResponseWriter, r *http.Request, reason string) {
	a.log.Warn("dsm settings request refused", "reason", reason, "path", r.URL.Path)
	http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
}

func (a *Auth) identify(ctx context.Context, cookie, token string) (Session, error) {
	now := time.Now()
	// The token is part of the key: a cookie that stops being accepted when
	// the token changes must not be answered from what the old pair earned.
	key := cookie + "\x00" + token

	a.mu.Lock()
	if c, ok := a.recent[key]; ok && now.Before(c.until) {
		a.mu.Unlock()
		return c.session, nil
	}
	a.mu.Unlock()

	session, err := a.verifier.Identify(ctx, cookie, token)
	if err != nil {
		return Session{}, err
	}

	a.mu.Lock()
	// The map only ever holds live sessions, and a NAS has a handful of
	// people: expired entries are swept here rather than by a goroutine.
	for k, c := range a.recent {
		if now.After(c.until) {
			delete(a.recent, k)
		}
	}
	a.recent[key] = cached{session: session, until: now.Add(cacheFor)}
	a.mu.Unlock()
	return session, nil
}

// dsmCookie picks DSM's session cookie out of the request.
//
// The name is "id"; the CGI in the package passes the browser's Cookie header
// along untouched, so whatever DSM set is what arrives here.
func dsmCookie(r *http.Request) string {
	if c, err := r.Cookie("id"); err == nil && c.Value != "" {
		return c.Value
	}
	// Some DSM versions name it differently behind a reverse proxy; take the
	// first cookie that looks like a session rather than failing outright.
	for _, c := range r.Cookies() {
		if strings.EqualFold(c.Name, "smid") && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

// ErrNotLoggedIn is what a verifier returns for a cookie DSM does not know.
var ErrNotLoggedIn = fmt.Errorf("dsm: the session is not valid")
