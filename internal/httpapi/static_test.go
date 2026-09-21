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

// TestServesMiniApp: статика отдаётся без авторизации — подпись Telegram
// приходит уже из загруженной страницы, поэтому саму страницу закрывать
// нечем и незачем. Закрыты данные, а не оболочка.
func TestServesMiniApp(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>Загрузки</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := New(Options{
		BotToken: testToken,
		Static:   static,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, tc := range []struct{ path, want string }{
		{"/", "Загрузки"},
		{"/assets/app.js", "console.log"},
		// Неизвестный путь отдаёт оболочку: навигация внутри Mini App
		// происходит без обращения к серверу.
		{"/files", "Загрузки"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("%s: код %d", tc.path, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s: в ответе нет %q", tc.path, tc.want)
		}
	}
}

// TestNoStaticStillServesAPI: без собранного фронтенда сервис остаётся
// рабочим ботом, а не падает.
func TestNoStaticStillServesAPI(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("без статики главная вернула %d, ожидался 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "не собрано") {
		t.Errorf("ответ не объясняет причину: %s", w.Body.String())
	}
}

// TestSecurityHeaders: заголовки, уместные для страницы внутри webview.
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
