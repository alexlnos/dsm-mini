package bot

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// pendingTTL — сколько ждём выбора папки, прежде чем забыть запрос.
const pendingTTL = 30 * time.Minute

// pending — то, что пользователь прислал и что ждёт выбора папки.
//
// Хранить приходится на сервере: в callback_data инлайн-кнопки помещается
// 64 байта, а magnet-ссылка и тем более файл туда не влезают.
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

// put кладёт запрос и возвращает короткий ключ для кнопки.
func (s *pendingStore) put(p pending) string {
	p.Created = time.Now()
	key := randomKey()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	s.items[key] = p
	return key
}

// take забирает запрос по ключу: повторное нажатие кнопки задачу не задвоит.
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
		// Ключ нужен только как метка в памяти процесса; при отказе
		// генератора берём временную метку, чтобы бот не падал.
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
