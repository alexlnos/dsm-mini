// Package db — хранилище на SQLite.
//
// Драйвер взят на чистом Go (modernc.org/sqlite), а не привычный
// mattn/go-sqlite3: последний требует CGO, а с ним пропадает статическая
// сборка, на которой держится образ из distroless.
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

// migrations применяются по порядку; номер последней записывается в
// user_version. Менять уже применённую миграцию нельзя — только добавлять
// новую, иначе базы у разных людей разойдутся.
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

	-- Последний известный статус задачи: по нему наблюдатель понимает, что
	-- задача завершилась, и не повторяет уведомление после перезапуска.
	CREATE TABLE task_state (
		task_id    TEXT    PRIMARY KEY,
		status     TEXT    NOT NULL,
		updated_at INTEGER NOT NULL
	);
	`,
}

// Open открывает базу и доводит её схему до актуальной.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("не создать каталог базы: %w", err)
		}
	}

	// WAL — чтобы чтение не блокировалось записью: HTTP-запросы и наблюдатель
	// работают с базой одновременно. busy_timeout убирает случайные
	// «database is locked» под нагрузкой.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("не открыть базу: %w", err)
	}

	// Одно соединение на запись: SQLite не любит параллельную запись, а
	// нагрузка тут — десятки запросов в минуту.
	database.SetMaxOpenConns(1)
	database.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("база не отвечает: %w", err)
	}
	if err := migrate(ctx, database); err != nil {
		database.Close()
		return nil, err
	}

	// SQLite создаёт файлы по umask, то есть обычно доступными на чтение
	// всем. В базе лежат пользовательские настройки, поэтому права режем
	// явно — вместе со спутниками WAL-режима.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Chmod(path+suffix, 0o600); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("не выставить права на %s: %w", path+suffix, err)
		}
	}
	return database, nil
}

func migrate(ctx context.Context, database *sql.DB) error {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("не прочитать версию схемы: %w", err)
	}
	if version > len(migrations) {
		return fmt.Errorf("база новее приложения: схема версии %d, известно %d — "+
			"обновите dsm-mini", version, len(migrations))
	}

	for i := version; i < len(migrations); i++ {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("миграция %d не применилась: %w", i+1, err)
		}
		// PRAGMA не принимает параметры, поэтому номер подставляется в текст;
		// значение — индекс из кода, а не пользовательский ввод.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("не записать версию схемы: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
