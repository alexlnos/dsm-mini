package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alexlnos/dsm-mini/internal/db"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("открыть базу: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database)
}

// TestDefaultsForUnknownUser: незнакомый пользователь получает рабочие
// настройки, а не пустые.
func TestDefaultsForUnknownUser(t *testing.T) {
	s := newStore(t)
	got, err := s.Get(context.Background(), 777)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if !got.ShowRecent {
		t.Error("по умолчанию недавние папки должны показываться")
	}
	if len(got.PinnedFolders) != 0 {
		t.Errorf("закреплённых быть не должно: %v", got.PinnedFolders)
	}
}

// TestPinnedOrderKept: порядок папок значим — его задаёт пользователь.
func TestPinnedOrderKept(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	want := []string{"Downloads", "Media/Music", "Media/TV Shows"}
	if _, err := s.Set(ctx, 1, Settings{PinnedFolders: want, ShowRecent: true}); err != nil {
		t.Fatalf("сохранение: %v", err)
	}

	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if len(got.PinnedFolders) != len(want) {
		t.Fatalf("папок %d, ожидалось %d", len(got.PinnedFolders), len(want))
	}
	for i := range want {
		if got.PinnedFolders[i] != want[i] {
			t.Errorf("позиция %d: %q, ожидалось %q", i, got.PinnedFolders[i], want[i])
		}
	}

	// Перестановка должна заменять список целиком, а не дополнять его.
	reordered := []string{"Media/Music", "Downloads"}
	if _, err := s.Set(ctx, 1, Settings{PinnedFolders: reordered}); err != nil {
		t.Fatalf("повторное сохранение: %v", err)
	}
	got, _ = s.Get(ctx, 1)
	if len(got.PinnedFolders) != 2 || got.PinnedFolders[0] != "Media/Music" {
		t.Errorf("после перестановки: %v", got.PinnedFolders)
	}
}

// TestPathsNormalized: пути приводятся к виду, который принимает
// Download Station, а попытки выйти вверх по дереву отбрасываются.
func TestPathsNormalized(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	saved, err := s.Set(ctx, 2, Settings{PinnedFolders: []string{
		"/Media/Music/", "  Downloads  ", "", "Media/Music", "../../etc", "ok/path",
	}})
	if err != nil {
		t.Fatalf("сохранение: %v", err)
	}

	want := []string{"Media/Music", "Downloads", "ok/path"}
	if len(saved.PinnedFolders) != len(want) {
		t.Fatalf("получилось %v, ожидалось %v", saved.PinnedFolders, want)
	}
	for i := range want {
		if saved.PinnedFolders[i] != want[i] {
			t.Errorf("позиция %d: %q, ожидалось %q", i, saved.PinnedFolders[i], want[i])
		}
	}
}

// TestPinnedLimit: список подрезается, чтобы выбор не превращался в прокрутку.
func TestPinnedLimit(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	many := make([]string, 0, maxPinned+5)
	for i := 0; i < maxPinned+5; i++ {
		many = append(many, "folder/"+string(rune('a'+i)))
	}
	saved, err := s.Set(ctx, 3, Settings{PinnedFolders: many})
	if err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	if len(saved.PinnedFolders) != maxPinned {
		t.Errorf("сохранено %d папок, ожидалось %d", len(saved.PinnedFolders), maxPinned)
	}
}

// TestRememberLastUsed: папка последней задачи запоминается и не затирает
// остальные настройки.
func TestRememberLastUsed(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.Set(ctx, 4, Settings{
		PinnedFolders: []string{"Media/Music"}, ShowRecent: false,
	}); err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	if err := s.RememberLastUsed(ctx, 4, "/Downloads/"); err != nil {
		t.Fatalf("запоминание: %v", err)
	}

	got, _ := s.Get(ctx, 4)
	if got.LastUsed != "Downloads" {
		t.Errorf("последняя папка %q, ожидалась Downloads", got.LastUsed)
	}
	if len(got.PinnedFolders) != 1 {
		t.Errorf("закреплённые потерялись: %v", got.PinnedFolders)
	}
	if got.ShowRecent {
		t.Error("флаг показа недавних потерялся")
	}
}

// TestSettingsIsolatedPerUser: настройки одного не видны другому.
func TestSettingsIsolatedPerUser(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.Set(ctx, 10, Settings{PinnedFolders: []string{"Media/Music"}}); err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	other, err := s.Get(ctx, 11)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if len(other.PinnedFolders) != 0 {
		t.Errorf("чужие настройки видны: %v", other.PinnedFolders)
	}
}

// TestTaskStatesRoundTrip: состояние задач переживает перезапись и
// исчезнувшие задачи из таблицы уходят.
func TestTaskStatesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if err := s.SaveTaskStates(ctx, map[string]string{
		"dbid_1": "downloading", "dbid_2": "finished",
	}); err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	got, err := s.TaskStates(ctx)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if got["dbid_1"] != "downloading" || got["dbid_2"] != "finished" {
		t.Errorf("состояние не совпало: %v", got)
	}

	if err := s.SaveTaskStates(ctx, map[string]string{"dbid_2": "seeding"}); err != nil {
		t.Fatalf("перезапись: %v", err)
	}
	got, _ = s.TaskStates(ctx)
	if len(got) != 1 || got["dbid_2"] != "seeding" {
		t.Errorf("после перезаписи: %v", got)
	}
}

// TestMigrationsAreIdempotent: повторное открытие базы не ломает данные.
func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "repeat.db")

	first, err := db.Open(path)
	if err != nil {
		t.Fatalf("первое открытие: %v", err)
	}
	if _, err := New(first).Set(ctx, 5, Settings{PinnedFolders: []string{"Media/Movies"}}); err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	first.Close()

	second, err := db.Open(path)
	if err != nil {
		t.Fatalf("повторное открытие: %v", err)
	}
	defer second.Close()

	got, err := New(second).Get(ctx, 5)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if len(got.PinnedFolders) != 1 || got.PinnedFolders[0] != "Media/Movies" {
		t.Errorf("данные не пережили переоткрытие: %v", got.PinnedFolders)
	}
}
