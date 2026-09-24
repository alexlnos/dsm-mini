package dsmnotify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type sentMessage struct {
	chat int64
	text string
}

type spyNotifier struct{ sent chan sentMessage }

func (s *spyNotifier) Notify(_ context.Context, chat int64, text string) error {
	s.sent <- sentMessage{chat, text}
	return nil
}

type fixedMode string

func (m fixedMode) NotifyMode(context.Context) (string, error) { return string(m), nil }

func newTestReceiver(mode string) (*Receiver, *spyNotifier) {
	spy := &spyNotifier{sent: make(chan sentMessage, 4)}
	r := NewReceiver("s3cret", fixedMode(mode), []int64{7, 8},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.SetNotifier(spy)
	return r, spy
}

func post(r *Receiver, secret, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/dsm/notify", strings.NewReader(body))
	if secret != "" {
		req.Header.Set(SecretHeader, secret)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// waitSent gives the delivery goroutine a moment; nothing arriving is an
// answer too, so the caller says which it expects.
func waitSent(t *testing.T, spy *spyNotifier, want int) []sentMessage {
	t.Helper()
	var got []sentMessage
	deadline := time.After(2 * time.Second)
	for len(got) < want {
		select {
		case m := <-spy.sent:
			got = append(got, m)
		case <-deadline:
			t.Fatalf("expected %d messages, got %d", want, len(got))
		}
	}
	// Nothing extra should follow.
	select {
	case m := <-spy.sent:
		t.Fatalf("one message too many: %+v", m)
	case <-time.After(150 * time.Millisecond):
	}
	return got
}

func TestWrongSecretIsRefused(t *testing.T) {
	r, spy := newTestReceiver("all")

	for _, secret := range []string{"", "wrong", "s3cre", "s3crett"} {
		w := post(r, secret, `{"text":"anything"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("secret %q: got %d, want 403", secret, w.Code)
		}
	}
	select {
	case m := <-spy.sent:
		t.Fatalf("a refused call still delivered %+v", m)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestDeliveredToEveryChat(t *testing.T) {
	r, spy := newTestReceiver("all")

	if w := post(r, "s3cret", `{"text":"Packages are out of date"}`); w.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", w.Code)
	}
	got := waitSent(t, spy, 2)
	for _, m := range got {
		if m.text != "Packages are out of date" {
			t.Fatalf("text changed on the way: %q", m.text)
		}
		if m.chat != 7 && m.chat != 8 {
			t.Fatalf("message sent to an outsider: %d", m.chat)
		}
	}
}

// "downloads" is the default, and it must not let DSM's own announcements
// through: that is the whole point of the three-way choice.
func TestOtherModesDropIt(t *testing.T) {
	for _, mode := range []string{"downloads", "off"} {
		r, spy := newTestReceiver(mode)
		if w := post(r, "s3cret", `{"text":"Packages are out of date"}`); w.Code != http.StatusNoContent {
			t.Fatalf("mode %q: got %d, want 204", mode, w.Code)
		}
		select {
		case m := <-spy.sent:
			t.Fatalf("mode %q still delivered %+v", mode, m)
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func TestEmptyAndMalformed(t *testing.T) {
	r, spy := newTestReceiver("all")

	if w := post(r, "s3cret", `{"text":"   "}`); w.Code != http.StatusNoContent {
		t.Fatalf("blank text: got %d, want 204", w.Code)
	}
	if w := post(r, "s3cret", `not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: got %d, want 400", w.Code)
	}
	select {
	case m := <-spy.sent:
		t.Fatalf("nothing should have been sent, got %+v", m)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestOnlyPost(t *testing.T) {
	r, _ := newTestReceiver("all")
	req := httptest.NewRequest(http.MethodGet, "/dsm/notify", nil)
	req.Header.Set(SecretHeader, "s3cret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", w.Code)
	}
}

// Without a bot the call is still accepted: DSM is not the party that can fix
// it, and failing the request would only make it retry into the same gap.
func TestNoBotYet(t *testing.T) {
	r := NewReceiver("s3cret", fixedMode("all"), []int64{7},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if w := post(r, "s3cret", `{"text":"hello"}`); w.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", w.Code)
	}
}
