package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// TestServesMiniApp: static files are served without authorisation — the
// Telegram signature arrives from an already loaded page, so there is nothing
// to lock the page itself with, and no reason to. The data is locked, not the shell.
func TestServesMiniApp(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>Downloads</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := New(Options{
		BotToken: testToken,
		Static:   static,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, tc := range []struct{ path, want string }{
		{"/", "Downloads"},
		{"/assets/app.js", "console.log"},
		// An unknown path serves the shell: navigation inside the Mini App
		// happens without touching the server.
		{"/files", "Downloads"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("%s: code %d", tc.path, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s: the response lacks %q", tc.path, tc.want)
		}
	}
}

// TestNoStaticStillServesAPI: without a built frontend the service stays a
// working bot instead of crashing.
func TestNoStaticStillServesAPI(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("without static files the home page returned %d, expected 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not built") {
		t.Errorf("the response does not explain why: %s", w.Body.String())
	}
}

// TestSecurityHeaders: headers suited to a page inside a webview.
func TestSecurityHeaders(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
}

// TestCacheHeaders: index.html is revalidated every time, assets with a hash
// in the name are cached forever.
//
// Without this Telegram keeps opening the old build after an update: it holds
// index.html in cache, and that one points at the previous files.
func TestCacheHeaders(t *testing.T) {
	static := fstest.MapFS{
		"index.html":             {Data: []byte("<!doctype html>")},
		"assets/index-abc123.js": {Data: []byte("console.log(1)")},
	}
	s := New(Options{
		BotToken: testToken,
		Static:   static,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, tc := range []struct{ path, want string }{
		{"/", "no-cache"},
		{"/files", "no-cache"},
		{"/assets/index-abc123.js", "public, max-age=31536000, immutable"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)

		if got := w.Header().Get("Cache-Control"); got != tc.want {
			t.Errorf("%s: Cache-Control = %q, expected %q", tc.path, got, tc.want)
		}
	}
}
