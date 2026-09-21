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
		t.Fatalf("open the database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database)
}

// TestDefaultsForUnknownUser: an unknown user gets working settings rather
// than empty ones.
func TestDefaultsForUnknownUser(t *testing.T) {
	s := newStore(t)
	got, err := s.Get(context.Background(), 777)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.ShowRecent {
		t.Error("recent folders should be shown by default")
	}
	if len(got.PinnedFolders) != 0 {
		t.Errorf("there should be no pinned folders: %v", got.PinnedFolders)
	}
}

// TestPinnedOrderKept: folder order matters — the user sets it.
func TestPinnedOrderKept(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	want := []string{"Downloads", "Media/Music", "Media/TV Shows"}
	if _, err := s.Set(ctx, 1, Settings{PinnedFolders: want, ShowRecent: true}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got.PinnedFolders) != len(want) {
		t.Fatalf("%d folders, expected %d", len(got.PinnedFolders), len(want))
	}
	for i := range want {
		if got.PinnedFolders[i] != want[i] {
			t.Errorf("position %d: %q, expected %q", i, got.PinnedFolders[i], want[i])
		}
	}

	// Reordering must replace the whole list, not append to it.
	reordered := []string{"Media/Music", "Downloads"}
	if _, err := s.Set(ctx, 1, Settings{PinnedFolders: reordered}); err != nil {
		t.Fatalf("save again: %v", err)
	}
	got, _ = s.Get(ctx, 1)
	if len(got.PinnedFolders) != 2 || got.PinnedFolders[0] != "Media/Music" {
		t.Errorf("after reordering: %v", got.PinnedFolders)
	}
}

// TestPathsNormalized: paths are brought to the shape Download Station
// accepts, and attempts to walk up the tree are dropped.
func TestPathsNormalized(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	saved, err := s.Set(ctx, 2, Settings{PinnedFolders: []string{
		"/Media/Music/", "  Downloads  ", "", "Media/Music", "../../etc", "ok/path",
	}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	want := []string{"Media/Music", "Downloads", "ok/path"}
	if len(saved.PinnedFolders) != len(want) {
		t.Fatalf("got %v, expected %v", saved.PinnedFolders, want)
	}
	for i := range want {
		if saved.PinnedFolders[i] != want[i] {
			t.Errorf("position %d: %q, expected %q", i, saved.PinnedFolders[i], want[i])
		}
	}
}

// TestPinnedLimit: the list is trimmed so the choice does not become scrolling.
func TestPinnedLimit(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	many := make([]string, 0, maxPinned+5)
	for i := 0; i < maxPinned+5; i++ {
		many = append(many, "folder/"+string(rune('a'+i)))
	}
	saved, err := s.Set(ctx, 3, Settings{PinnedFolders: many})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(saved.PinnedFolders) != maxPinned {
		t.Errorf("%d folders saved, expected %d", len(saved.PinnedFolders), maxPinned)
	}
}

// TestRememberLastUsed: the folder of the latest task is remembered and does
// not wipe the other settings.
func TestRememberLastUsed(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.Set(ctx, 4, Settings{
		PinnedFolders: []string{"Media/Music"}, ShowRecent: false,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.RememberLastUsed(ctx, 4, "/Downloads/"); err != nil {
		t.Fatalf("remember: %v", err)
	}

	got, _ := s.Get(ctx, 4)
	if got.LastUsed != "Downloads" {
		t.Errorf("last folder %q, expected Downloads", got.LastUsed)
	}
	if len(got.PinnedFolders) != 1 {
		t.Errorf("the pinned folders were lost: %v", got.PinnedFolders)
	}
	if got.ShowRecent {
		t.Error("the show-recent flag was lost")
	}
}

// TestSettingsIsolatedPerUser: one person's settings are invisible to another.
func TestSettingsIsolatedPerUser(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.Set(ctx, 10, Settings{PinnedFolders: []string{"Media/Music"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	other, err := s.Get(ctx, 11)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(other.PinnedFolders) != 0 {
		t.Errorf("someone else's settings are visible: %v", other.PinnedFolders)
	}
}

// TestTaskStatesRoundTrip: task state survives a rewrite and vanished tasks
// leave the table.
func TestTaskStatesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if err := s.SaveTaskStates(ctx, map[string]string{
		"dbid_1": "downloading", "dbid_2": "finished",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.TaskStates(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got["dbid_1"] != "downloading" || got["dbid_2"] != "finished" {
		t.Errorf("the state did not match: %v", got)
	}

	if err := s.SaveTaskStates(ctx, map[string]string{"dbid_2": "seeding"}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got, _ = s.TaskStates(ctx)
	if len(got) != 1 || got["dbid_2"] != "seeding" {
		t.Errorf("after the rewrite: %v", got)
	}
}

// TestMigrationsAreIdempotent: reopening the database does not break the data.
func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "repeat.db")

	first, err := db.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := New(first).Set(ctx, 5, Settings{PinnedFolders: []string{"Media/Movies"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	first.Close()

	second, err := db.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()

	got, err := New(second).Get(ctx, 5)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got.PinnedFolders) != 1 || got.PinnedFolders[0] != "Media/Movies" {
		t.Errorf("the data did not survive reopening: %v", got.PinnedFolders)
	}
}
