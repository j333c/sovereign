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
	ErrDeadlineNotFound = errors.New("document deadline not found")
)

const (
	DeadlineStatusPending   = "pending"
	DeadlineStatusFuture    = "future"
	DeadlineStatusDismissed = "dismissed"
	DeadlineStatusNotified  = "notified"
)

// DocumentDeadline represents an extracted payment or contractual deadline.
type DocumentDeadline struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	DueDate    string    `json:"due_date"` // Normalized ISO date YYYY-MM-DD
	Amount     *float64  `json:"amount,omitempty"`
	Currency   string    `json:"currency,omitempty"`
	Snippet    string    `json:"snippet,omitempty"`
	Status     string    `json:"status"` // pending, future, dismissed, notified
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// DeadlineStore manages persistence and deduplication of document deadlines.
type DeadlineStore struct {
	db *DB
}

// NewDeadlineStore initializes a DeadlineStore repository with WAL durability.
func NewDeadlineStore(db *DB) (*DeadlineStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	return &DeadlineStore{db: db}, nil
}

// Save inserts or updates a deadline, strictly deduplicating by (document_id, due_date).
// If a record already exists, its ID and creation timestamp are preserved, and its details updated.
func (s *DeadlineStore) Save(ctx context.Context, d *DocumentDeadline) error {
	if d == nil {
		return fmt.Errorf("deadline cannot be nil")
	}
	if strings.TrimSpace(d.DocumentID) == "" {
		return fmt.Errorf("document_id cannot be empty")
	}
	if strings.TrimSpace(d.Title) == "" {
		d.Title = "Upcoming Deadline"
	}
	if strings.TrimSpace(d.DueDate) == "" {
		return fmt.Errorf("due_date cannot be empty")
	}
	if strings.TrimSpace(d.Status) == "" {
		d.Status = DeadlineStatusPending
	}
	if d.ID == "" {
		d.ID = generateUUIDv4()
	}

	query := `
		INSERT INTO document_deadlines (
			id, document_id, title, due_date, amount, currency, snippet, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(document_id, due_date) DO UPDATE SET
			title = excluded.title,
			amount = COALESCE(excluded.amount, document_deadlines.amount),
			currency = CASE WHEN excluded.currency != '' THEN excluded.currency ELSE document_deadlines.currency END,
			snippet = CASE WHEN excluded.snippet != '' THEN excluded.snippet ELSE document_deadlines.snippet END,
			status = CASE 
				WHEN document_deadlines.status = 'dismissed' THEN 'dismissed' 
				WHEN document_deadlines.status = 'notified' THEN 'notified'
				ELSE excluded.status 
			END,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query,
		d.ID,
		d.DocumentID,
		d.Title,
		d.DueDate,
		d.Amount,
		d.Currency,
		d.Snippet,
		d.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to save deadline: %w", err)
	}

	// Retrieve the actual stored record to reflect any deduplicated ID or preserved status
	saved, err := s.GetByDocAndDueDate(ctx, d.DocumentID, d.DueDate)
	if err == nil && saved != nil {
		d.ID = saved.ID
		d.Status = saved.Status
		d.CreatedAt = saved.CreatedAt
		d.UpdatedAt = saved.UpdatedAt
		d.Amount = saved.Amount
		d.Currency = saved.Currency
		d.Snippet = saved.Snippet
		d.Title = saved.Title
	}

	return nil
}

// Get retrieves a deadline by ID.
func (s *DeadlineStore) Get(ctx context.Context, id string) (*DocumentDeadline, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, document_id, title, due_date, amount, currency, snippet, status, created_at, updated_at
		FROM document_deadlines
		WHERE id = ?
	`, id)

	d, err := s.scanDeadline(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeadlineNotFound
		}
		return nil, fmt.Errorf("failed to get deadline %q: %w", id, err)
	}
	return d, nil
}

// GetByDocAndDueDate retrieves a deadline by (document_id, due_date).
func (s *DeadlineStore) GetByDocAndDueDate(ctx context.Context, docID, dueDate string) (*DocumentDeadline, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, document_id, title, due_date, amount, currency, snippet, status, created_at, updated_at
		FROM document_deadlines
		WHERE document_id = ? AND due_date = ?
	`, docID, dueDate)

	d, err := s.scanDeadline(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeadlineNotFound
		}
		return nil, fmt.Errorf("failed to get deadline for doc %q due %q: %w", docID, dueDate, err)
	}
	return d, nil
}

// List returns detected deadlines filtered by status and optional time window.
// withinDays: if > 0, restricts to due_date <= now + withinDays days, ignoring past dates older than 30 days.
// If status is empty or "pending", defaults to pending. If "all", no status filter is applied.
func (s *DeadlineStore) List(ctx context.Context, status string, withinDays int, now time.Time) ([]*DocumentDeadline, error) {
	if now.IsZero() {
		now = time.Now()
	}

	var conditions []string
	var args []any

	if status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	if withinDays > 0 {
		maxDate := now.AddDate(0, 0, withinDays).Format("2006-01-02")
		minDate := now.AddDate(0, 0, -30).Format("2006-01-02")
		conditions = append(conditions, "due_date <= ? AND due_date >= ?")
		args = append(args, maxDate, minDate)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, document_id, title, due_date, amount, currency, snippet, status, created_at, updated_at
		FROM document_deadlines
		%s
		ORDER BY due_date ASC, created_at ASC
	`, whereClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list document deadlines: %w", err)
	}
	defer rows.Close()

	var result []*DocumentDeadline
	for rows.Next() {
		d, err := s.scanDeadline(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading deadline rows: %w", err)
	}

	return result, nil
}

// Dismiss updates a deadline's status to 'dismissed'.
func (s *DeadlineStore) Dismiss(ctx context.Context, id string) (*DocumentDeadline, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE document_deadlines
		SET status = 'dismissed', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to dismiss deadline: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrDeadlineNotFound
	}

	return s.Get(ctx, id)
}

// MarkNotified updates a deadline's status to 'notified'.
func (s *DeadlineStore) MarkNotified(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE document_deadlines
		SET status = 'notified', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to mark deadline as notified: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrDeadlineNotFound
	}
	return nil
}

// Delete removes a deadline by ID.
func (s *DeadlineStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM document_deadlines WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete deadline: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrDeadlineNotFound
	}
	return nil
}

func (s *DeadlineStore) scanDeadline(scanner rowScanner) (*DocumentDeadline, error) {
	var d DocumentDeadline
	var amountVal sql.NullFloat64
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&d.ID,
		&d.DocumentID,
		&d.Title,
		&d.DueDate,
		&amountVal,
		&d.Currency,
		&d.Snippet,
		&d.Status,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	if amountVal.Valid {
		amt := amountVal.Float64
		d.Amount = &amt
	}

	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		d.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
		d.CreatedAt = t
	}

	if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
		d.UpdatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
		d.UpdatedAt = t
	}

	return &d, nil
}
