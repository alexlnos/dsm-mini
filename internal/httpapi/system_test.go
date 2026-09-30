package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/system"
)

// refusingNAS answers every call the way DSM answered a tester's ordinary,
// non-administrator account: the details with 1006, everything else with
// 105, "the account lacks permission".
type refusingNAS struct{}

func (refusingNAS) Call(_ context.Context, api, method string, _ int, _ map[string]any, _ any) error {
	code := 105
	if api == "SYNO.Core.System" {
		code = 1006
	}
	return &dsm.APIError{Code: code, API: api, Method: method}
}

// The home screen has to stay a home screen for such an account: "no
// connection", zeros for a refused load and "not installed" for every app
// were what it showed instead.
func TestHomeSurvivesAnOrdinaryAccount(t *testing.T) {
	stub := &stubStation{}
	s := New(Options{
		BotToken:       testToken,
		AllowedUserIDs: []int64{42},
		Downloads:      stub,
		System:         system.New(refusingNAS{}),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	r := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	r.Header.Set("Authorization", "tma "+signedInitData(t, 42, time.Now()))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: the parts that failed must not fail the answer", w.Code)
	}
	var body struct {
		Info       any            `json:"info"`
		InfoError  string         `json:"info_error"`
		Usage      any            `json:"usage"`
		UsageError string         `json:"usage_error"`
		Packages   map[string]any `json:"packages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Info != nil || body.Usage != nil {
		t.Fatalf("refused parts came back as values: info %v, usage %v", body.Info, body.Usage)
	}
	if body.InfoError == "" || body.UsageError == "" {
		t.Fatalf("no reason given: %q / %q", body.InfoError, body.UsageError)
	}
	// 105 says why; the signed-in user speaks Russian.
	if !strings.Contains(body.UsageError, "нет доступа") {
		t.Fatalf("usage error %q does not say the account has no access", body.UsageError)
	}
	if len(body.Packages) != 0 {
		t.Fatalf("packages %v: an unread list must not mark the apps as missing", body.Packages)
	}
}

// silentNAS is a DSM that does not answer at all: the NAS rebooting, the
// network down.
type silentNAS struct{}

func (silentNAS) Call(context.Context, string, string, int, map[string]any, any) error {
	return errors.New("dial tcp 127.0.0.1:5001: connect: connection refused")
}

// Only DSM's refusals are told part by part. A DSM that does not answer is
// still "no connection" on the home screen — answering 200 would have put
// "online" next to it.
func TestHomeSaysNoConnectionWhenDSMIsSilent(t *testing.T) {
	s := New(Options{
		BotToken:       testToken,
		AllowedUserIDs: []int64{42},
		Downloads:      &stubStation{},
		System:         system.New(silentNAS{}),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	r := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	r.Header.Set("Authorization", "tma "+signedInitData(t, 42, time.Now()))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502 for a DSM that does not answer", w.Code)
	}
}
