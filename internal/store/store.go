// Package store хранит пользовательские настройки приложения.
//
// Настройки лежат на сервере, а не в браузере: Mini App открывают с разных
// устройств, и закреплённые папки должны быть везде одинаковыми.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// maxPinned — сколько папок можно закрепить.
// Больше десятка превращают выбор в прокрутку и теряют смысл.
const maxPinned = 12

// Settings — настройки одного пользователя.
type Settings struct {
	// PinnedFolders — папки быстрого выбора, в порядке показа.
	PinnedFolders []string `json:"pinned_folders"`
	// ShowRecent — подмешивать ли папки недавних задач.
	ShowRecent bool `json:"show_recent"`
	// LastUsed — куда клали в прошлый раз.
	LastUsed string `json:"last_used"`
}

// Defaults возвращает настройки для пользователя, который ничего не менял.
func Defaults() Settings {
	return Settings{ShowRecent: true}
}

// Store — настройки всех пользователей в одном файле.
type Store struct {
	path string

	mu   sync.RWMutex
	data map[string]Settings
}

// New открывает хранилище. Отсутствующий файл — не ошибка: это первый запуск.
func New(path string) (*Store, error) {
	s := &Store{path: path, data: make(map[string]Settings)}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("не прочитать настройки: %w", err)
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		// Испорченный файл не должен мешать работе: начинаем с чистых
		// настроек, прежние значения перезапишутся при первом сохранении.
		s.data = make(map[string]Settings)
	}
	return s, nil
}

// Get возвращает настройки пользователя.
func (s *Store) Get(userID int64) Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.data[key(userID)]; ok {
		return v
	}
	return Defaults()
}

// Set сохраняет настройки, приводя их в порядок: пути нормализуются,
// дубликаты и пустые значения выбрасываются, список подрезается.
func (s *Store) Set(userID int64, v Settings) (Settings, error) {
	v.PinnedFolders = cleanFolders(v.PinnedFolders)
	v.LastUsed = normalize(v.LastUsed)

	s.mu.Lock()
	s.data[key(userID)] = v
	snapshot, err := json.Marshal(s.data)
	s.mu.Unlock()
	if err != nil {
		return v, err
	}
	return v, s.write(snapshot)
}

// RememberLastUsed запоминает папку последней задачи.
func (s *Store) RememberLastUsed(userID int64, folder string) {
	folder = normalize(folder)
	if folder == "" {
		return
	}
	current := s.Get(userID)
	if current.LastUsed == folder {
		return
	}
	current.LastUsed = folder
	_, _ = s.Set(userID, current)
}

func (s *Store) write(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	// Через временный файл: прерванная запись не испортит настройки.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func cleanFolders(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, f := range in {
		f = normalize(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
		if len(out) == maxPinned {
			break
		}
	}
	return out
}

// normalize приводит путь к виду, который принимает Download Station:
// без ведущего и завершающего слеша, например "Media/Music".
func normalize(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" || strings.Contains(p, "..") {
		return ""
	}
	return p
}

func key(id int64) string { return strconv.FormatInt(id, 10) }
