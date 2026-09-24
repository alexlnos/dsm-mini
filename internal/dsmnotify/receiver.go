package dsmnotify

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Notifier sends one message to one chat.
type Notifier interface {
	Notify(ctx context.Context, chatID int64, text string) error
}

// Modes reports what the installation agreed to receive.
type Modes interface {
	NotifyMode(ctx context.Context) (string, error)
}

// Receiver is the endpoint DSM calls. It is created before the bot exists —
// the HTTP server starts while the bot is still waiting for Telegram — so the
// notifier arrives later through SetNotifier.
type Receiver struct {
	secret string
	modes  Modes
	chats  []int64
	log    *slog.Logger

	mu       sync.RWMutex
	notifier Notifier
}

// NewReceiver builds the endpoint. chats is the allow list: the same people
// the Mini App lets in are the ones the NAS talks to.
func NewReceiver(secret string, modes Modes, chats []int64, log *slog.Logger) *Receiver {
	return &Receiver{secret: secret, modes: modes, chats: chats, log: log}
}

// SetNotifier hands over the bot once Telegram has answered.
func (r *Receiver) SetNotifier(n Notifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifier = n
}

func (r *Receiver) current() Notifier {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.notifier
}

// The body is DSM's, and DSM is not a hostile party — but it is also not the
// only thing that can reach this path, so the read is capped.
const maxBody = 64 << 10

// ServeHTTP takes one notification from DSM and passes it on.
//
// The reply says nothing about what happened: the caller is a machine that
// cannot act on the difference, and a body that distinguishes "wrong secret"
// from "nobody is listening" would be a hint to whoever guessed the path.
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	got := req.Header.Get(SecretHeader)
	if subtle.ConstantTimeCompare([]byte(got), []byte(r.secret)) != 1 {
		r.log.Warn("dsm notification refused: the secret does not match",
			"remote", req.RemoteAddr)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(req.Body, maxBody)).Decode(&body); err != nil {
		r.log.Warn("dsm notification is not readable", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// The answer goes out before Telegram is involved: DSM is waiting on this
	// connection, and whether a message reached a phone is not its business.
	w.WriteHeader(http.StatusNoContent)
	go r.deliver(text)
}

func (r *Receiver) deliver(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mode, err := r.modes.NotifyMode(ctx)
	if err != nil {
		r.log.Error("cannot read the notification mode", "err", err)
		return
	}
	if mode != "all" {
		// Info, not debug: "DSM called and nothing arrived" has to be
		// answerable from the log without changing the log level first.
		r.log.Info("dsm notification not forwarded: the mode says otherwise", "mode", mode)
		return
	}

	n := r.current()
	if n == nil {
		r.log.Warn("dsm notification dropped: the bot is not connected yet")
		return
	}
	sent := 0
	for _, chat := range r.chats {
		if err := n.Notify(ctx, chat, text); err != nil {
			r.log.Error("cannot forward a dsm notification", "chat", chat, "err", err)
			continue
		}
		sent++
	}
	// The text is not logged: it is DSM's, and may name users, addresses and
	// files. How many people got it is enough to tell delivery from silence.
	r.log.Info("dsm notification forwarded", "chats", sent, "of", len(r.chats))
}
