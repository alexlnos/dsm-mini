package store

import (
	"context"
	"fmt"
	"time"
)

// TaskStates возвращает последние известные статусы задач.
//
// Наблюдатель сверяет с ними свежий список, чтобы поймать переход в
// завершённое состояние и не повторить уведомление после перезапуска.
func (s *Store) TaskStates(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, status FROM task_state`)
	if err != nil {
		return nil, fmt.Errorf("не прочитать состояние задач: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

// SaveTaskStates заменяет состояние целиком: задачи, которых больше нет,
// удаляются, чтобы таблица не росла бесконечно.
func (s *Store) SaveTaskStates(ctx context.Context, states map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM task_state`); err != nil {
		return err
	}
	now := time.Now().Unix()
	for id, status := range states {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO task_state (task_id, status, updated_at) VALUES (?, ?, ?)`,
			id, status, now); err != nil {
			return fmt.Errorf("не сохранить состояние задачи %s: %w", id, err)
		}
	}
	return tx.Commit()
}
