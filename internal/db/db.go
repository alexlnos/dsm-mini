// Package db is the SQLite storage.
//
// The driver is the pure Go one (modernc.org/sqlite) rather than the usual
// mattn/go-sqlite3: the latter needs CGO, and with CGO the static build the
// distroless image relies on is gone.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// migrations are applied in order; the number of the last one is written to
// user_version. An already applied migration must never be edited — only a
// new one added, otherwise databases drift apart between installs.
var migrations = []string{
	`
	CREATE TABLE user_settings (
		user_id     INTEGER PRIMARY KEY,
		show_recent INTEGER NOT NULL DEFAULT 1,
		last_used   TEXT    NOT NULL DEFAULT ''
	);

	CREATE TABLE pinned_folders (
		user_id  INTEGER NOT NULL,
		position INTEGER NOT NULL,
		path     TEXT    NOT NULL,
		PRIMARY KEY (user_id, position)
	);

	-- Last known status of a task: the watcher uses it to tell that a task
	-- finished, and not to repeat a notification after a restart.
	CREATE TABLE task_state (
		task_id    TEXT    PRIMARY KEY,
		status     TEXT    NOT NULL,
		updated_at INTEGER NOT NULL
	);
	`,
	`
	-- Where the user put downloads themselves. "Recent" folders used to come
	-- from Download Station tasks, that is from other people's torrents and
	-- old downloads; here it is their own history only.
	CREATE TABLE recent_folders (
		user_id INTEGER NOT NULL,
		path    TEXT    NOT NULL,
		used_at INTEGER NOT NULL,
		PRIMARY KEY (user_id, path)
	);

	CREATE INDEX idx_recent_folders_user_time
		ON recent_folders (user_id, used_at DESC);
	`,
	`
	-- The language Telegram reported on the last request. Notifications about
	-- finished tasks are sent on our own initiative, with no incoming message,
	-- and there is nobody to ask at that moment — hence it is stored.
	ALTER TABLE user_settings ADD COLUMN language TEXT NOT NULL DEFAULT '';
	`,
	`
	-- Values that belong to the installation rather than to any one person.
	--
	-- 'notifications' is what the service may send unprompted — 'off',
	-- 'downloads' or 'all'. It lives here rather than in user_settings because
	-- the settings screen inside DSM has no Telegram user to attach it to, and
	-- because a NAS tool with a handful of allowed accounts does not need a
	-- separate feed per person. Absent means 'downloads', which is what the
	-- service did before the choice existed: an upgrade must not start sending
	-- people things they never asked for.
	--
	-- 'webhook_secret' is what DSM puts in a header when it calls the
	-- notification webhook. The endpoint sits on the same server as everything
	-- else and is therefore reachable through the reverse proxy like any other
	-- path, so that header is the only thing separating DSM's call from
	-- anyone else's.
	CREATE TABLE service_meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`,
}

// Open opens the database and brings its schema up to date.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("cannot create the database directory: %w", err)
		}
	}

	// WAL so that reads are not blocked by writes: HTTP requests and the
	// watcher work with the database at the same time. busy_timeout removes
	// random "database is locked" under load.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open the database: %w", err)
	}

	// A single writer connection: SQLite dislikes concurrent writes, and the
	// load here is tens of requests per minute.
	database.SetMaxOpenConns(1)
	database.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("the database does not answer: %w", err)
	}
	if err := migrate(ctx, database); err != nil {
		database.Close()
		return nil, err
	}

	// SQLite creates files according to umask, that is usually readable by
	// everyone. The database holds user settings, so permissions are set
	// explicitly — together with the WAL mode companions.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Chmod(path+suffix, 0o600); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot set permissions on %s: %w", path+suffix, err)
		}
	}
	return database, nil
}

func migrate(ctx context.Context, database *sql.DB) error {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("cannot read the schema version: %w", err)
	}
	if version > len(migrations) {
		return fmt.Errorf("the database is newer than the app: schema version %d, known %d — "+
			"update dsm-mini", version, len(migrations))
	}

	for i := version; i < len(migrations); i++ {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d did not apply: %w", i+1, err)
		}
		// PRAGMA takes no parameters, so the number is interpolated into the
		// text; the value is an index from code, not user input.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("cannot write the schema version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
