package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// authTTL is how long initData is considered fresh.
//
// Telegram signs it once when the Mini App opens, and the window lives for
// hours. A day is a compromise between convenience and not letting an
// intercepted string work forever.
const authTTL = 24 * time.Hour

type ctxKey int

const userKey ctxKey = iota

// user is whoever opened the Mini App.
type user struct {
	ID        int64
	Username  string
	FirstName string
	Language  string
}

// userFrom takes the user that authMiddleware put in place.
func userFrom(ctx context.Context) (user, bool) {
	u, ok := ctx.Value(userKey).(user)
	return u, ok
}

// authMiddleware lets through only requests carrying genuine initData from an
// allowed user.
//
// Two things are checked, and both are mandatory:
//
//  1. The initData signature, keyed off the bot token. It proves the string
//     came from Telegram rather than from anyone who opened the address.
//  2. The user id against the allow list. The signature alone only confirms
//     that the person opened the bot — and anyone can open the bot.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := initDataFrom(r)
		if raw == "" {
			s.deny(w, r, http.StatusUnauthorized, "no Telegram authorisation data", "initData is missing")
			return
		}

		if err := initdata.Validate(raw, s.botToken, authTTL); err != nil {
			// The reason is not sent out: it would hint at how to forge a signature.
			s.deny(w, r, http.StatusUnauthorized, "the authorisation data is invalid", "initData signature failed: "+err.Error())
			return
		}

		parsed, err := initdata.Parse(raw)
		if err != nil {
			s.deny(w, r, http.StatusUnauthorized, "the authorisation data is invalid", "cannot parse initData: "+err.Error())
			return
		}
		if parsed.User.ID == 0 {
			s.deny(w, r, http.StatusUnauthorized, "the authorisation data is invalid", "initData carries no user")
			return
		}

		if !s.isAllowed(parsed.User.ID) {
			s.deny(w, r, http.StatusForbidden, "access denied",
				"the user is not on the allow list")
			return
		}

		u := user{
			ID:        parsed.User.ID,
			Username:  parsed.User.Username,
			FirstName: parsed.User.FirstName,
			Language:  parsed.User.LanguageCode,
		}
		// Remember the language: notifications about finished tasks are sent on
		// our own initiative, with nobody to ask by then.
		if s.settings != nil {
			if err := s.settings.RememberLanguage(r.Context(), u.ID, u.Language); err != nil {
				s.log.Warn("cannot remember the language", "user", u.ID, "err", err)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

// initDataFrom reads initData from the header, falling back to the query
// string.
//
// The main way is the `Authorization: tma <initData>` header: that keeps the
// string out of proxy logs and browser history.
func initDataFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := strings.CutPrefix(h, "tma "); ok {
			return strings.TrimSpace(rest)
		}
	}
	if h := r.Header.Get("X-Telegram-Init-Data"); h != "" {
		return h
	}
	return r.URL.Query().Get("_auth")
}

// deny answers with a refusal: details to the log, a general phrase outward.
func (s *Server) deny(w http.ResponseWriter, r *http.Request, status int, public, detail string) {
	s.log.Warn("request refused",
		"path", r.URL.Path,
		"status", status,
		"reason", detail,
		"remote", remoteAddr(r))
	writeJSON(w, status, map[string]string{"error": public})
}

func remoteAddr(r *http.Request) string {
	// Behind the DSM reverse proxy the real address arrives in a header.
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("cannot send the response", "err", err)
	}
}
