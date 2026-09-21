package bot

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// pendingTTL is how long we wait for a folder choice before forgetting the request.
const pendingTTL = 30 * time.Minute

// pending is what the user sent and what is waiting for a folder choice.
//
// It has to be kept on the server: an inline button's callback_data holds
// 64 bytes, and a magnet link — let alone a file — does not fit.
type pending struct {
	URL      string
	File     []byte
	FileName string
	Title    string
	Created  time.Time
}

type pendingStore struct {
	mu    sync.Mutex
	items map[string]pending
}

func newPendingStore() *pendingStore {
	return &pendingStore{items: make(map[string]pending)}
}

// put stores a request and returns a short key for the button.
func (s *pendingStore) put(p pending) string {
	p.Created = time.Now()
	key := randomKey()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	s.items[key] = p
	return key
}

// take fetches a request by key: pressing the button twice will not duplicate the task.
func (s *pendingStore) take(key string) (pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.items[key]
	if !ok {
		return pending{}, false
	}
	delete(s.items, key)
	if time.Since(p.Created) > pendingTTL {
		return pending{}, false
	}
	return p, true
}

func (s *pendingStore) evictLocked() {
	for k, v := range s.items {
		if time.Since(v.Created) > pendingTTL {
			delete(s.items, k)
		}
	}
}

func randomKey() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// The key is only a label inside the process memory; if the generator
		// fails we fall back to a timestamp so the bot does not crash.
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
