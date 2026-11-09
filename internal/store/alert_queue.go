package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAlertQueueItemNotFound = errors.New("alert queue item not found")
)

const (
	AlertQueueStatusPending   = "pending"
	AlertQueueStatusDelivered = "delivered"
	AlertQueueStatusFailed    = "failed"

	DefaultMaxRetries = 5
)

// AlertQueueItem represents a persistent notification delivery job.
type AlertQueueItem struct {
	ID            string    `json:"id"`
	Channel       string    `json:"channel"`
	Recipient     string    `json:"recipient"`
	Payload       string    `json:"payload"`
	Status        string    `json:"status"` // pending, delivered, failed
	RetryCount    int       `json:"retry_count"`
	MaxRetries    int       `json:"max_retries"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	LastError     string    `json:"last_error"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AlertQueueStore manages persistence and retry scheduling for failed alerts.
type AlertQueueStore struct {
	db *DB
}

// NewAlertQueueStore initializes an AlertQueueStore backed by SQLite.
func NewAlertQueueStore(db *DB) (*AlertQueueStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	return &AlertQueueStore{db: db}, nil
}

// CalculateBackoff computes exponential backoff: delay = min(10s * (2^retryCount), 1 hour).
func CalculateBackoff(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount >= 9 {
		return time.Hour
	}
	delay := time.Duration(10*(int64(1)<<retryCount)) * time.Second
	if delay > time.Hour {
		delay = time.Hour
	}
	return delay
}

// Enqueue stores an alert delivery job in the persistent queue.
func (s *AlertQueueStore) Enqueue(ctx context.Context, item *AlertQueueItem) error {
	if item == nil {
		return errors.New("alert queue item cannot be nil")
	}
	if strings.TrimSpace(item.Channel) == "" {
		return errors.New("channel cannot be empty")
	}
	if strings.TrimSpace(item.Payload) == "" {
		return errors.New("payload cannot be empty")
	}
	if strings.TrimSpace(item.ID) == "" {
		item.ID = fmt.Sprintf("alert-%d", time.Now().UnixNano())
	}
	if item.Status == "" {
		item.Status = AlertQueueStatusPending
	}
	if item.MaxRetries <= 0 {
		item.MaxRetries = DefaultMaxRetries
	}
	if item.NextAttemptAt.IsZero() {
		item.NextAttemptAt = time.Now()
	}

	nextAttemptStr := item.NextAttemptAt.UTC().Format("2006-01-02 15:04:05")

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_dispatch_queue (
			id, channel, recipient, payload, status, retry_count, max_retries, next_attempt_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			retry_count = 0,
			next_attempt_at = excluded.next_attempt_at,
			payload = excluded.payload,
			last_error = excluded.last_error,
			updated_at = CURRENT_TIMESTAMP
	`, item.ID, item.Channel, item.Recipient, item.Payload, item.Status, item.RetryCount, item.MaxRetries, nextAttemptStr, item.LastError)
	if err != nil {
		return fmt.Errorf("failed to enqueue alert: %w", err)
	}
	return nil
}

// GetPending returns all items currently pending delivery where next_attempt_at <= asOf.
func (s *AlertQueueStore) GetPending(ctx context.Context, asOf time.Time) ([]*AlertQueueItem, error) {
	if asOf.IsZero() {
		asOf = time.Now()
	}
	asOfStr := asOf.UTC().Format("2006-01-02 15:04:05")

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel, recipient, payload, status, retry_count, max_retries, next_attempt_at, last_error, created_at, updated_at
		FROM alert_dispatch_queue
		WHERE status = 'pending' AND next_attempt_at <= ?
		ORDER BY next_attempt_at ASC, created_at ASC
	`, asOfStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending alert queue items: %w", err)
	}
	defer rows.Close()

	var items []*AlertQueueItem
	for rows.Next() {
		item, err := scanAlertQueueItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkDelivered marks an alert item as successfully delivered.
func (s *AlertQueueStore) MarkDelivered(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE alert_dispatch_queue
		SET status = 'delivered', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to mark alert %q delivered: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrAlertQueueItemNotFound
	}
	return nil
}

// RecordFailure records a delivery failure, increments retry_count, and sets the next attempt timestamp.
// If retry_count reaches max_retries, status transitions to 'failed'.
func (s *AlertQueueStore) RecordFailure(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error {
	var retryCount, maxRetries int
	err := s.db.QueryRowContext(ctx, `
		SELECT retry_count, max_retries
		FROM alert_dispatch_queue
		WHERE id = ?
	`, id).Scan(&retryCount, &maxRetries)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlertQueueItemNotFound
		}
		return fmt.Errorf("failed to query alert item %q: %w", id, err)
	}

	retryCount++
	newStatus := AlertQueueStatusPending
	if retryCount >= maxRetries {
		newStatus = AlertQueueStatusFailed
	}

	if nextAttemptAt.IsZero() {
		nextAttemptAt = time.Now().Add(CalculateBackoff(retryCount))
	}
	nextAttemptStr := nextAttemptAt.UTC().Format("2006-01-02 15:04:05")

	_, err = s.db.ExecContext(ctx, `
		UPDATE alert_dispatch_queue
		SET retry_count = ?, status = ?, next_attempt_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, retryCount, newStatus, nextAttemptStr, lastError, id)
	if err != nil {
		return fmt.Errorf("failed to record failure for alert %q: %w", id, err)
	}
	return nil
}

// List returns queue items, optionally filtered by status.
func (s *AlertQueueStore) List(ctx context.Context, status string) ([]*AlertQueueItem, error) {
	var query string
	var args []any
	if status != "" && status != "all" {
		query = `
			SELECT id, channel, recipient, payload, status, retry_count, max_retries, next_attempt_at, last_error, created_at, updated_at
			FROM alert_dispatch_queue
			WHERE status = ?
			ORDER BY created_at DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, channel, recipient, payload, status, retry_count, max_retries, next_attempt_at, last_error, created_at, updated_at
			FROM alert_dispatch_queue
			ORDER BY created_at DESC
		`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert queue items: %w", err)
	}
	defer rows.Close()

	var items []*AlertQueueItem
	for rows.Next() {
		item, err := scanAlertQueueItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Get retrieves a queue item by ID.
func (s *AlertQueueStore) Get(ctx context.Context, id string) (*AlertQueueItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, channel, recipient, payload, status, retry_count, max_retries, next_attempt_at, last_error, created_at, updated_at
		FROM alert_dispatch_queue
		WHERE id = ?
	`, id)
	item, err := scanAlertQueueItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlertQueueItemNotFound
		}
		return nil, fmt.Errorf("failed to get alert queue item %q: %w", id, err)
	}
	return item, nil
}

// Delete removes an alert queue item by ID.
func (s *AlertQueueStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM alert_dispatch_queue WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete alert queue item: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrAlertQueueItemNotFound
	}
	return nil
}

func scanAlertQueueItem(scanner rowScanner) (*AlertQueueItem, error) {
	var item AlertQueueItem
	var nextAttemptStr, createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&item.ID,
		&item.Channel,
		&item.Recipient,
		&item.Payload,
		&item.Status,
		&item.RetryCount,
		&item.MaxRetries,
		&nextAttemptStr,
		&item.LastError,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	item.NextAttemptAt = parseAlertTime(nextAttemptStr)
	item.CreatedAt = parseAlertTime(createdAtStr)
	item.UpdatedAt = parseAlertTime(updatedAtStr)

	return &item, nil
}

func parseAlertTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
