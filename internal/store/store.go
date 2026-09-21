// Package store хранит пользовательские настройки в SQLite.
//
// Настройки лежат на стороне сервиса, а не в браузере: Mini App открывают с
// разных устройств, и закреплённые папки должны быть везде одинаковыми, а
// хранилище webview Telegram чистится без предупреждения.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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

// Defaults возвращает настройки пользователя, который ничего не менял.
func Defaults() Settings {
	return Settings{ShowRecent: true}
}

// Store — доступ к настройкам.
type Store struct {
	db *sql.DB
}

// New оборачивает уже открытую базу.
func New(database *sql.DB) *Store { return &Store{db: database} }

// Get возвращает настройки пользователя; для незнакомого — значения по умолчанию.
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
		return out, fmt.Errorf("не прочитать настройки: %w", err)
	}
	out.ShowRecent = showRecent != 0

	rows, err := s.db.QueryContext(ctx,
		`SELECT path FROM pinned_folders WHERE user_id = ? ORDER BY position`, userID)
	if err != nil {
		return out, fmt.Errorf("не прочитать закреплённые папки: %w", err)
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

// Set сохраняет настройки, приводя их в порядок: пути нормализуются,
// дубликаты и пустые значения выбрасываются, список подрезается.
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
		return v, fmt.Errorf("не сохранить настройки: %w", err)
	}

	// Порядок папок значим, поэтому список переписывается целиком: так
	// позиции всегда плотные и совпадают с тем, что видит пользователь.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pinned_folders WHERE user_id = ?`, userID); err != nil {
		return v, err
	}
	for i, folder := range v.PinnedFolders {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pinned_folders (user_id, position, path) VALUES (?, ?, ?)`,
			userID, i, folder); err != nil {
			return v, fmt.Errorf("не сохранить папку %q: %w", folder, err)
		}
	}
	return v, tx.Commit()
}

// RememberLastUsed запоминает папку последней задачи и пополняет историю.
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

// RecentFolders возвращает папки, куда пользователь складывал загрузки,
// начиная с самой свежей.
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
		return nil, fmt.Errorf("не прочитать историю папок: %w", err)
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
