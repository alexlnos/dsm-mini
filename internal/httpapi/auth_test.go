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

// stubStation stands in for the NAS: authorisation tests must not depend on it.
type stubStation struct{ called bool }

func (s *stubStation) List(context.Context) ([]downloadstation.Task, error) {
	s.called = true
	return []downloadstation.Task{{ID: "dbid_1", Title: "test"}}, nil
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
func (s *stubStation) DefaultDestination(context.Context) (string, error)        { return "Download", nil }
func (s *stubStation) Generation() string                                        { return "stub" }

func (s *stubStation) Files(context.Context, string) ([]downloadstation.File, error) {
	s.called = true
	return nil, nil
}
func (s *stubStation) SetFile(context.Context, string, []int, downloadstation.FilePriority, *bool) error {
	s.called = true
	return nil
}
func (s *stubStation) Trackers(context.Context, string) ([]downloadstation.Tracker, error) {
	s.called = true
	return nil, nil
}
func (s *stubStation) SetDestination(context.Context, []string, string) error {
	s.called = true
	return nil
}
func (s *stubStation) SetPriority(context.Context, []string, downloadstation.FilePriority) error {
	s.called = true
	return nil
}

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

// signedInitData builds a genuine initData string, as Telegram would issue it.
func signedInitData(t *testing.T, userID int64, issued time.Time) string {
	t.Helper()
	u, err := json.Marshal(map[string]any{
		"id":            userID,
		"first_name":    "Test",
		"username":      "tester",
		"language_code": "ru",
	})
	if err != nil {
		t.Fatalf("cannot build the user: %v", err)
	}
	values := url.Values{
		"auth_date": {strconv.FormatInt(issued.Unix(), 10)},
		"query_id":  {"AAH-test"},
		"user":      {string(u)},
	}
	hash, err := initdata.SignQueryString(values.Encode(), testToken, issued)
	if err != nil {
		t.Fatalf("cannot sign initData: %v", err)
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

// TestRejectsMissingInitData: without a signature nobody gets in.
func TestRejectsMissingInitData(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	if got := request(t, s, "").Code; got != http.StatusUnauthorized {
		t.Errorf("without initData code %d, expected 401", got)
	}
	if stub.called {
		t.Error("an unauthorised request reached the NAS")
	}
}

// TestRejectsForgedSignature: a signature made with someone else's key fails.
//
// This is the main defence: the Mini App address is public and anyone can open it.
func TestRejectsForgedSignature(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})

	issued := time.Now()
	values := url.Values{
		"auth_date": {strconv.FormatInt(issued.Unix(), 10)},
		"user":      {`{"id":42,"first_name":"Attacker"}`},
	}
	hash, err := initdata.SignQueryString(values.Encode(), "999999:WRONG-TOKEN", issued)
	if err != nil {
		t.Fatalf("cannot sign: %v", err)
	}
	values.Set("hash", hash)

	if got := request(t, s, values.Encode()).Code; got != http.StatusUnauthorized {
		t.Errorf("a forged signature gave code %d, expected 401", got)
	}
	if stub.called {
		t.Error("a request with a forged signature reached the NAS")
	}
}

// TestRejectsTamperedUser: the user id inside a signed string cannot be swapped.
func TestRejectsTamperedUser(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})

	raw := signedInitData(t, 999, time.Now())
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The signature stays as it was, the id is changed to an allowed one.
	values.Set("user", `{"id":42,"first_name":"Swapped"}`)

	if got := request(t, s, values.Encode()).Code; got != http.StatusUnauthorized {
		t.Errorf("swapping the user gave code %d, expected 401", got)
	}
}

// TestRejectsExpired: an expired string is not accepted.
func TestRejectsExpired(t *testing.T) {
	s, _ := newTestServer(t, []int64{42})
	old := time.Now().Add(-authTTL - time.Hour)
	if got := request(t, s, signedInitData(t, 42, old)).Code; got != http.StatusUnauthorized {
		t.Errorf("expired initData gave code %d, expected 401", got)
	}
}

// TestRejectsUserOutsideAllowlist: a genuine signature is no substitute for permission.
//
// Anyone can open the bot and get a real signed string — which is why the
// allow list is checked separately.
func TestRejectsUserOutsideAllowlist(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	w := request(t, s, signedInitData(t, 777, time.Now()))
	if w.Code != http.StatusForbidden {
		t.Errorf("an outsider got code %d, expected 403", w.Code)
	}
	if stub.called {
		t.Error("an outsider request reached the NAS")
	}
}

// TestEmptyAllowlistBlocksEveryone: an empty list closes access, it does not open it.
func TestEmptyAllowlistBlocksEveryone(t *testing.T) {
	s, _ := newTestServer(t, nil)
	if got := request(t, s, signedInitData(t, 42, time.Now())).Code; got != http.StatusForbidden {
		t.Errorf("with an empty list code %d, expected 403", got)
	}
}

// TestAllowsValidUser: an allowed user with a genuine signature gets through.
func TestAllowsValidUser(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	w := request(t, s, signedInitData(t, 42, time.Now()))
	if w.Code != http.StatusOK {
		t.Fatalf("an allowed user got code %d: %s", w.Code, w.Body.String())
	}
	if !stub.called {
		t.Error("the request did not reach the NAS")
	}

	var body struct {
		Tasks []taskView `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("the response did not parse: %v", err)
	}
	if len(body.Tasks) != 1 || body.Tasks[0].ID != "dbid_1" {
		t.Errorf("unexpected response: %s", w.Body.String())
	}
}

// TestHealthNeedsNoAuth: the liveness check needs no signature and stays quiet about the NAS.
func TestHealthNeedsNoAuth(t *testing.T) {
	s, stub := newTestServer(t, []int64{42})
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("healthz returned %d", w.Code)
	}
	if stub.called {
		t.Error("healthz reached out to the NAS")
	}
}
