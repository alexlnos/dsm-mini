package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// Keys of service_meta. The table holds what belongs to the installation
// rather than to any one person.
const (
	keyNotifications = "notifications"
	keyWebhookSecret = "webhook_secret"
)

// meta reads one value; a missing key is not an error, it is the empty string.
func (s *Store) meta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM service_meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", key, err)
	}
	return v, nil
}

func (s *Store) setMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO service_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("cannot save %q: %w", key, err)
	}
	return nil
}

// NotifyMode says what the service may send without being asked.
func (s *Store) NotifyMode(ctx context.Context) (string, error) {
	v, err := s.meta(ctx, keyNotifications)
	if err != nil {
		return NotifyDownloads, err
	}
	return cleanNotifications(v), nil
}

// SetNotifyMode records the choice. An unknown value is not rejected but
// corrected: the caller gets back what was actually stored.
func (s *Store) SetNotifyMode(ctx context.Context, mode string) (string, error) {
	mode = cleanNotifications(mode)
	return mode, s.setMeta(ctx, keyNotifications, mode)
}

// WebhookSecret returns the secret DSM sends in a header when it calls the
// notification endpoint, making one on first use.
//
// It is generated rather than configured because nobody should have to invent
// it, and it never leaves the NAS: DSM reads it from its own provider entry
// and the service from here, and the two talk over loopback.
func (s *Store) WebhookSecret(ctx context.Context) (string, error) {
	v, err := s.meta(ctx, keyWebhookSecret)
	if err != nil {
		return "", err
	}
	if v != "" {
		return v, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cannot generate the webhook secret: %w", err)
	}
	v = hex.EncodeToString(buf)
	if err := s.setMeta(ctx, keyWebhookSecret, v); err != nil {
		return "", err
	}
	return v, nil
}
