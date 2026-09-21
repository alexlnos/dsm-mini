package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const testToken = "123456:TEST-TOKEN-NOT-REAL"

// stubStation подставляется вместо NAS: тесты авторизации не должны от него зависеть.
type stubStation struct{ called bool }

func (s *stubStation) List(context.Context) ([]downloadstation.Task, error) {
	s.called = true
	return []downloadstation.Task{{ID: "dbid_1", Title: "тест"}}, nil
}
func (s *stubStation) Pause(context.Context, []string) error        { s.called = true; return nil }
func (s *stubStation) Resume(context.Context, []string) error       { s.called = true; return nil }
func (s *stubStation) Delete(context.Context, []string, bool) error { s.called = true; return nil }
func (s *stubStation) Create(context.Context, downloadstation.CreateRequest) error {
	s.called = true
	return nil
}
func (s *stubStation) Stats(context.Context) (downloadstation.Stats, error) {
	return downloadstation.Stats{}, nil
}
func (s *stubStation) Volumes(context.Context) ([]downloadstation.Volume, error) { return nil, nil }
func (s *stubStation) DefaultDestination(context.Context) (string, error)        { return "Downloads", nil }
func (s *stubStation) Generation() string                                        { return "stub" }

func newTestServer(t *testing.T, allowed []int64) (*Server, *stubStation) {
	t.Helper()
	stub := &stubStation{}
	s := New(Options{
		BotToken:       testToken,
		AllowedUserIDs: allowed,
		Downloads:      stub,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return s, stub
}

// signedInitData собирает подлинную строку initData, как её выдал бы Telegram.
func signedInitData(t *testing.T, userID int64, issued time.Time) string {
	t.Helper()
	u, err := json.Marshal(map[string]any{
		"id":            userID,
		"first_name":    "Тест",
		"username":      "tester",
		"language_code": "ru",
	})
	if err != nil {
		t.Fatalf("не собрать пользователя: %v", err)
	}
	values := url.Values{
		"auth_date": {strconv.FormatInt(issued.Unix(), 10)},
		"query_id":  {"AAH-test"},
		"user":      {string(u)},
	}
	hash, err := initdata.SignQueryString(values.Encode(), testToken, issued)
	if err != nil {
		t.Fatalf("не подписать initData: %v", err)
	}
	values.Set("hash", hash)
	return values.Encode()
}

func request(t *testing.T, s *Server, auth string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	if auth != "" {
		r.Header.Set("Authorization", "tma "+auth)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

// TestRejectsMissingInitData: без подписи внутрь не пускают.
func TestRejectsMissingInitData(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	if got := request(t, s, "").Code; got != http.StatusUnauthorized {
		t.Errorf("без initData код %d, ожидался 401", got)
	}
	if stub.called {
		t.Error("запрос без авторизации дошёл до NAS")
	}
}

// TestRejectsForgedSignature: подпись чужим ключом не проходит.
//
// Это главная защита: адрес Mini App публичен, и открыть его может кто угодно.
func TestRejectsForgedSignature(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})

	issued := time.Now()
	values := url.Values{
		"auth_date": {strconv.FormatInt(issued.Unix(), 10)},
		"user":      {`{"id":42,"first_name":"Злоумышленник"}`},
	}
	hash, err := initdata.SignQueryString(values.Encode(), "999999:WRONG-TOKEN", issued)
	if err != nil {
		t.Fatalf("не подписать: %v", err)
	}
	values.Set("hash", hash)

	if got := request(t, s, values.Encode()).Code; got != http.StatusUnauthorized {
		t.Errorf("подделанная подпись дала код %d, ожидался 401", got)
	}
	if stub.called {
		t.Error("запрос с подделанной подписью дошёл до NAS")
	}
}

// TestRejectsTamperedUser: подменить id пользователя в подписанной строке нельзя.
func TestRejectsTamperedUser(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})

	raw := signedInitData(t, 999, time.Now())
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("разбор: %v", err)
	}
	// Подпись оставляем прежней, id меняем на разрешённый.
	values.Set("user", `{"id":42,"first_name":"Подмена"}`)

	if got := request(t, s, values.Encode()).Code; got != http.StatusUnauthorized {
		t.Errorf("подмена пользователя дала код %d, ожидался 401", got)
	}
}

// TestRejectsExpired: просроченная строка не принимается.
func TestRejectsExpired(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})
	old := time.Now().Add(-authTTL - time.Hour)
	if got := request(t, s, signedInitData(t, 42, old)).Code; got != http.StatusUnauthorized {
		t.Errorf("просроченная initData дала код %d, ожидался 401", got)
	}
}

// TestRejectsUserOutsideAllowlist: подлинная подпись не заменяет разрешения.
//
// Бота может открыть любой человек и получить настоящую подписанную строку —
// поэтому список разрешённых проверяется отдельно.
func TestRejectsUserOutsideAllowlist(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	w := request(t, s, signedInitData(t, 777, time.Now()))
	if w.Code != http.StatusForbidden {
		t.Errorf("посторонний пользователь получил код %d, ожидался 403", w.Code)
	}
	if stub.called {
		t.Error("посторонний запрос дошёл до NAS")
	}
}

// TestEmptyAllowlistBlocksEveryone: пустой список закрывает доступ, а не открывает.
func TestEmptyAllowlistBlocksEveryone(t *testing.T) {
	s, _ := newTestServer(t, nil)
	if got := request(t, s, signedInitData(t, 42, time.Now())).Code; got != http.StatusForbidden {
		t.Errorf("при пустом списке код %d, ожидался 403", got)
	}
}

// TestAllowsValidUser: разрешённый пользователь с подлинной подписью проходит.
func TestAllowsValidUser(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	w := request(t, s, signedInitData(t, 42, time.Now()))
	if w.Code != http.StatusOK {
		t.Fatalf("разрешённый пользователь получил код %d: %s", w.Code, w.Body.String())
	}
	if !stub.called {
		t.Error("запрос не дошёл до NAS")
	}

	var body struct {
		Tasks []taskView `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не разобрался: %v", err)
	}
	if len(body.Tasks) != 1 || body.Tasks[0].ID != "dbid_1" {
		t.Errorf("неожиданный ответ: %s", w.Body.String())
	}
}

// TestHealthNeedsNoAuth: проверка живости доступна без подписи и молчит о NAS.
func TestHealthNeedsNoAuth(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("healthz вернул %d", w.Code)
	}
	if stub.called {
		t.Error("healthz обращался к NAS")
	}
}
