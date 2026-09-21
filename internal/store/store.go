// Package store keeps user settings in SQLite.
//
// Settings live on the service side rather than in the browser: the Mini App
// is opened from different devices and pinned folders must look the same
// everywhere, while Telegram's webview storage is wiped without warning.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// maxPinned is how many folders may be pinned.
// More than a dozen turns the choice into scrolling and loses its point.
const maxPinned = 12

// Settings are the settings of a single user.
type Settings struct {
	// PinnedFolders are the quick-pick folders, in display order.
	PinnedFolders []string `json:"pinned_folders"`
	// ShowRecent tells whether to mix in folders from recent tasks.
	ShowRecent bool `json:"show_recent"`
	// LastUsed is where things went the previous time.
	LastUsed string `json:"last_used"`
}

// Defaults returns the settings of a user who changed nothing.
func Defaults() Settings {
	return Settings{ShowRecent: true}
}

// Store gives access to the settings.
type Store struct {
	db *sql.DB
}

// New wraps an already open database.
func New(database *sql.DB) *Store { return &Store{db: database} }

// Get returns a user's settings; for an unknown user, the defaults.
func (s *Store) Get(ctx context.Context, userID int64) (Settings, error) {
	out := Defaults()

	var showRecent int
	err := s.db.QueryRowContext(ctx,
		`SELECT show_recent, last_used FROM user_settings WHERE user_id = ?`,
		userID).Scan(&showRecent, &out.LastUsed)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return out, nil
	case err != nil:
		return out, fmt.Errorf("cannot read the settings: %w", err)
	}
	out.ShowRecent = showRecent != 0

	rows, err := s.db.QueryContext(ctx,
		`SELECT path FROM pinned_folders WHERE user_id = ? ORDER BY position`, userID)
	if err != nil {
		return out, fmt.Errorf("cannot read the pinned folders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return out, err
		}
		out.PinnedFolders = append(out.PinnedFolders, path)
	}
	return out, rows.Err()
}

// Set saves the settings after tidying them up: paths are normalised,
// duplicates and empty values are dropped, the list is trimmed.
func (s *Store) Set(ctx context.Context, userID int64, v Settings) (Settings, error) {
	v.PinnedFolders = cleanFolders(v.PinnedFolders)
	v.LastUsed = normalize(v.LastUsed)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()

	showRecent := 0
	if v.ShowRecent {
		showRecent = 1
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, show_recent, last_used)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			show_recent = excluded.show_recent,
			last_used   = excluded.last_used`,
		userID, showRecent, v.LastUsed); err != nil {
		return v, fmt.Errorf("cannot save the settings: %w", err)
	}

	// Folder order matters, so the list is rewritten whole: that keeps the
	// positions dense and matching what the user sees.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pinned_folders WHERE user_id = ?`, userID); err != nil {
		return v, err
	}
	for i, folder := range v.PinnedFolders {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pinned_folders (user_id, position, path) VALUES (?, ?, ?)`,
			userID, i, folder); err != nil {
			return v, fmt.Errorf("cannot save the folder %q: %w", folder, err)
		}
	}
	return v, tx.Commit()
}

// RememberLastUsed records the folder of the latest task and adds to history.
func (s *Store) RememberLastUsed(ctx context.Context, userID int64, folder string) error {
	folder = normalize(folder)
	if folder == "" {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, show_recent, last_used)
		VALUES (?, 1, ?)
		ON CONFLICT(user_id) DO UPDATE SET last_used = excluded.last_used`,
		userID, folder); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recent_folders (user_id, path, used_at)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, path) DO UPDATE SET used_at = excluded.used_at`,
		userID, folder, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

// RecentFolders returns the folders the user put downloads into, newest
// first.
func (s *Store) RecentFolders(ctx context.Context, userID int64, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 6
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT path FROM recent_folders
		WHERE user_id = ?
		ORDER BY used_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("cannot read the folder history: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
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

// normalize brings a path to the shape Download Station accepts:
// no leading and no trailing slash, for example "Media/Music".
func normalize(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "" || strings.Contains(p, "..") {
		return ""
	}
	return p
}

// RememberLanguage records the language Telegram speaks to this person in.
//
// Notifications need it: they are sent on our own initiative, with no
// incoming message, and there is nobody to ask at that moment. An empty code
// is ignored — it would wipe a known language and switch the person to English.
func (s *Store) RememberLanguage(ctx context.Context, userID int64, code string) error {
	if code == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, language) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET language = excluded.language`,
		userID, code)
	if err != nil {
		return fmt.Errorf("cannot remember the language: %w", err)
	}
	return nil
}

// Language returns the remembered language; empty for an unknown person.
func (s *Store) Language(ctx context.Context, userID int64) (string, error) {
	var code string
	err := s.db.QueryRowContext(ctx,
		`SELECT language FROM user_settings WHERE user_id = ?`, userID).Scan(&code)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("cannot read the language: %w", err)
	}
	return code, nil
}
