package transcode

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrJobNotFound = errors.New("transcode job not found")
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusPaused     JobStatus = "paused"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

// Job represents a single transcoding task.
type Job struct {
	ID           string     `json:"id"`
	SourcePath   string     `json:"source_path"`
	TargetPath   string     `json:"target_path"`
	Profile      string     `json:"profile"`
	Status       JobStatus  `json:"status"`
	Progress     float64    `json:"progress"`
	EncoderUsed  string     `json:"encoder_used,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Store defines persistent storage operations for transcoding queue jobs.
type Store interface {
	InitSchema(ctx context.Context) error
	Enqueue(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id string) (*Job, error)
	ListJobs(ctx context.Context) ([]Job, error)
	GetNextQueuedJob(ctx context.Context) (*Job, error)
	UpdateProgress(ctx context.Context, id string, progress float64) error
	UpdateStatus(ctx context.Context, id string, status JobStatus, errMsg string) error
	MarkStarted(ctx context.Context, id string, encoderUsed string) error
	MarkCompleted(ctx context.Context, id string, targetPath, encoderUsed string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	ResetProcessingJobs(ctx context.Context) error
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore instance.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// InitSchema ensures the transcode_queue table and indexes exist.
func (s *SQLiteStore) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS transcode_queue (
		id TEXT PRIMARY KEY,
		source_path TEXT NOT NULL,
		target_path TEXT NOT NULL,
		profile TEXT NOT NULL DEFAULT 'universal',
		status TEXT NOT NULL DEFAULT 'queued',
		progress REAL NOT NULL DEFAULT 0.0,
		encoder_used TEXT DEFAULT '',
		error_message TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL
	);
	CREATE INDEX IF NOT EXISTS idx_transcode_queue_status ON transcode_queue(status, created_at);
	`
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to initialize transcode_queue schema: %w", err)
	}
	return nil
}

// Enqueue inserts a new transcode job into the database.
func (s *SQLiteStore) Enqueue(ctx context.Context, job *Job) error {
	if job.ID == "" {
		job.ID = generateJobID()
	}
	if job.Status == "" {
		job.Status = StatusQueued
	}
	if job.Profile == "" {
		job.Profile = "universal"
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}

	query := `
	INSERT INTO transcode_queue (id, source_path, target_path, profile, status, progress, encoder_used, error_message, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		job.SourcePath,
		job.TargetPath,
		job.Profile,
		string(job.Status),
		job.Progress,
		job.EncoderUsed,
		job.ErrorMessage,
		job.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue transcode job: %w", err)
	}
	return nil
}

// GetJob retrieves a transcode job by ID.
func (s *SQLiteStore) GetJob(ctx context.Context, id string) (*Job, error) {
	query := `
	SELECT id, source_path, target_path, profile, status, progress, encoder_used, error_message, created_at, started_at, completed_at
	FROM transcode_queue
	WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to query transcode job %q: %w", id, err)
	}
	return job, nil
}

// ListJobs returns all transcode jobs ordered by created_at DESC.
func (s *SQLiteStore) ListJobs(ctx context.Context) ([]Job, error) {
	query := `
	SELECT id, source_path, target_path, profile, status, progress, encoder_used, error_message, created_at, started_at, completed_at
	FROM transcode_queue
	ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query transcode jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcode job: %w", err)
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading transcode jobs: %w", err)
	}
	return jobs, nil
}

// GetNextQueuedJob retrieves the oldest queued or paused job.
func (s *SQLiteStore) GetNextQueuedJob(ctx context.Context) (*Job, error) {
	query := `
	SELECT id, source_path, target_path, profile, status, progress, encoder_used, error_message, created_at, started_at, completed_at
	FROM transcode_queue
	WHERE status IN ('queued', 'paused')
	ORDER BY created_at ASC
	LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No jobs waiting
		}
		return nil, fmt.Errorf("failed to query next queued job: %w", err)
	}
	return job, nil
}

// UpdateProgress updates the progress percentage of an active job.
func (s *SQLiteStore) UpdateProgress(ctx context.Context, id string, progress float64) error {
	query := `UPDATE transcode_queue SET progress = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, progress, id)
	if err != nil {
		return fmt.Errorf("failed to update progress for job %q: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJobNotFound
	}
	return nil
}

// UpdateStatus updates the job's status and error message.
func (s *SQLiteStore) UpdateStatus(ctx context.Context, id string, status JobStatus, errMsg string) error {
	query := `UPDATE transcode_queue SET status = ?, error_message = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, string(status), errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to update status for job %q: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJobNotFound
	}
	return nil
}

// MarkStarted transitions a job to processing status and records started_at.
func (s *SQLiteStore) MarkStarted(ctx context.Context, id string, encoderUsed string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `
	UPDATE transcode_queue
	SET status = 'processing', encoder_used = ?, started_at = COALESCE(started_at, ?), error_message = ''
	WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, encoderUsed, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark job %q started: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJobNotFound
	}
	return nil
}

// MarkCompleted marks a job as completed, records target path, encoder used, progress 100%, and completed_at.
func (s *SQLiteStore) MarkCompleted(ctx context.Context, id string, targetPath, encoderUsed string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `
	UPDATE transcode_queue
	SET status = 'completed', target_path = ?, encoder_used = ?, progress = 100.0, completed_at = ?, error_message = ''
	WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, targetPath, encoderUsed, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark job %q completed: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJobNotFound
	}
	return nil
}

// MarkFailed marks a job as failed, records error message and completed_at.
func (s *SQLiteStore) MarkFailed(ctx context.Context, id string, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `
	UPDATE transcode_queue
	SET status = 'failed', error_message = ?, completed_at = ?
	WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark job %q failed: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrJobNotFound
	}
	return nil
}

// ResetProcessingJobs resets any jobs stuck in 'processing' status back to 'queued'
// (for example after an unexpected supervisor daemon restart).
func (s *SQLiteStore) ResetProcessingJobs(ctx context.Context) error {
	query := `UPDATE transcode_queue SET status = 'queued' WHERE status = 'processing'`
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to reset processing jobs: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner rowScanner) (*Job, error) {
	var (
		job          Job
		statusStr    string
		createdAtStr string
		startedAtStr sql.NullString
		completedStr sql.NullString
	)

	err := scanner.Scan(
		&job.ID,
		&job.SourcePath,
		&job.TargetPath,
		&job.Profile,
		&statusStr,
		&job.Progress,
		&job.EncoderUsed,
		&job.ErrorMessage,
		&createdAtStr,
		&startedAtStr,
		&completedStr,
	)
	if err != nil {
		return nil, err
	}

	job.Status = JobStatus(statusStr)

	if t, err := parseTimestamp(createdAtStr); err == nil {
		job.CreatedAt = t
	}

	if startedAtStr.Valid && startedAtStr.String != "" {
		if t, err := parseTimestamp(startedAtStr.String); err == nil {
			job.StartedAt = &t
		}
	}

	if completedStr.Valid && completedStr.String != "" {
		if t, err := parseTimestamp(completedStr.String); err == nil {
			job.CompletedAt = &t
		}
	}

	return &job, nil
}

func parseTimestamp(val string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, val); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %q", val)
}

func generateJobID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("job-%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
