package store

import (
	"context"
	"fmt"
	"time"
)

// TaskStates returns the last known statuses of the tasks.
//
// The watcher compares a fresh list against them to catch the transition to
// a finished state and not repeat a notification after a restart.
func (s *Store) TaskStates(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, status FROM task_state`)
	if err != nil {
		return nil, fmt.Errorf("cannot read the task state: %w", err)
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

// SaveTaskStates replaces the state as a whole: tasks that are gone are
// removed so the table does not grow forever.
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
			return fmt.Errorf("cannot save the state of task %s: %w", id, err)
		}
	}
	return tx.Commit()
}
