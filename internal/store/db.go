package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var baselineSchema string

// DB wraps the SQL database handle and provides appliance state management.
type DB struct {
	*sql.DB
	path string
}

// Open initializes the SQLite database at the specified path, ensuring WAL mode
// and NORMAL synchronous pragma are strictly enforced for power-loss resilience.
func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path cannot be empty")
	}

	// Ensure parent directory exists with secure permissions (0755)
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %q: %w", dir, err)
	}

	// modernc.org/sqlite supports pragma query parameters in the connection string
	// E.g. _pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath, separator)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure pool for SQLite WAL durability
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Explicitly verify and enforce WAL pragma
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL;").Scan(&journalMode); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply journal_mode=WAL: %w", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		db.Close()
		return nil, fmt.Errorf("expected journal_mode WAL, got: %s", journalMode)
	}

	// Explicitly apply synchronous=NORMAL pragma
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply synchronous=NORMAL: %w", err)
	}

	// Verify synchronous pragma is 1 (NORMAL)
	var syncMode int
	if err := db.QueryRowContext(ctx, "PRAGMA synchronous;").Scan(&syncMode); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to query pragma synchronous: %w", err)
	}
	if syncMode != 1 {
		db.Close()
		return nil, fmt.Errorf("expected synchronous=1 (NORMAL), got: %d", syncMode)
	}

	storeDB := &DB{
		DB:   db,
		path: dbPath,
	}

	// Apply baseline schema migrations
	if err := storeDB.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply baseline schema: %w", err)
	}

	return storeDB, nil
}

// migrate executes the embedded baseline schema.
func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, baselineSchema); err != nil {
		return fmt.Errorf("executing baseline schema failed: %w", err)
	}
	return nil
}

// CheckIntegrity runs SQLite PRAGMA integrity_check.
func (d *DB) CheckIntegrity(ctx context.Context) (bool, string, error) {
	var result string
	row := d.QueryRowContext(ctx, "PRAGMA integrity_check;")
	if err := row.Scan(&result); err != nil {
		return false, "", fmt.Errorf("integrity check failed: %w", err)
	}
	return result == "ok", result, nil
}

// BootStatusInfo holds the boot status record.
type BootStatusInfo struct {
	Mode                 string `json:"mode"`
	CleanShutdown        bool   `json:"clean_shutdown"`
	DiagnosticsCompleted bool   `json:"diagnostics_completed"`
	Message              string `json:"message"`
}

// GetBootStatus returns the current boot status.
func (d *DB) GetBootStatus(ctx context.Context) (*BootStatusInfo, error) {
	row := d.QueryRowContext(ctx, `
		SELECT mode, clean_shutdown, diagnostics_completed, message 
		FROM boot_status 
		ORDER BY id DESC LIMIT 1
	`)

	var status BootStatusInfo
	var cleanShutdownInt, diagCompletedInt int
	if err := row.Scan(&status.Mode, &cleanShutdownInt, &diagCompletedInt, &status.Message); err != nil {
		if err == sql.ErrNoRows {
			return &BootStatusInfo{
				Mode:                 "operational",
				CleanShutdown:        true,
				DiagnosticsCompleted: true,
				Message:              "All systems operational",
			}, nil
		}
		return nil, fmt.Errorf("querying boot status failed: %w", err)
	}

	status.CleanShutdown = cleanShutdownInt != 0
	status.DiagnosticsCompleted = diagCompletedInt != 0
	return &status, nil
}

// FlushWAL executes PRAGMA wal_checkpoint(TRUNCATE) to flush WAL to the main DB file.
func (d *DB) FlushWAL(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("wal checkpoint failed: %w", err)
	}
	return nil
}

// RecordBootStatus inserts a new boot status record into boot_status.
func (d *DB) RecordBootStatus(ctx context.Context, status *BootStatusInfo) error {
	cleanShutdownInt := 0
	if status.CleanShutdown {
		cleanShutdownInt = 1
	}
	diagCompletedInt := 0
	if status.DiagnosticsCompleted {
		diagCompletedInt = 1
	}

	_, err := d.ExecContext(ctx, `
		INSERT INTO boot_status (mode, clean_shutdown, diagnostics_completed, message)
		VALUES (?, ?, ?, ?)
	`, status.Mode, cleanShutdownInt, diagCompletedInt, status.Message)
	if err != nil {
		return fmt.Errorf("recording boot status failed: %w", err)
	}
	return nil
}

// GetSetting retrieves a configuration value by key from the settings table.
// Returns an empty string and nil error if the key does not exist.
func (d *DB) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := d.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting %q: %w", key, err)
	}
	return val, nil
}

// SetSetting inserts or updates a configuration value by key in the settings table.
func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed to set setting %q: %w", key, err)
	}
	return nil
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// CreateSnapshot creates an atomic SQLite snapshot at destPath using VACUUM INTO.
// It ensures parent directories are created and guarantees overwrite safety.
func (d *DB) CreateSnapshot(ctx context.Context, destPath string) error {
	if destPath == "" {
		return errors.New("destination snapshot path cannot be empty")
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for snapshot %q: %w", dir, err)
	}

	// SQLite VACUUM INTO fails if destination file already exists. Ensure overwrite safety.
	if _, err := os.Stat(destPath); err == nil {
		if err := os.Remove(destPath); err != nil {
			return fmt.Errorf("failed to remove existing snapshot file %q: %w", destPath, err)
		}
	}

	// Checkpoint WAL to flush pending writes
	_ = d.FlushWAL(ctx)

	// Execute VACUUM INTO with parameter or escaped fallback
	_, err := d.ExecContext(ctx, "VACUUM INTO ?", destPath)
	if err != nil {
		escaped := strings.ReplaceAll(destPath, "'", "''")
		if _, errFallback := d.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); errFallback != nil {
			return fmt.Errorf("vacuum into failed: %w", err)
		}
	}

	return nil
}

// ComposeRevision tracks declarative Compose revisions and snapshots.
type ComposeRevision struct {
	Revision           int    `json:"revision"`
	ServiceID          string `json:"service_id"`
	OldImage           string `json:"old_image"`
	NewImage           string `json:"new_image"`
	ComposeFilePath    string `json:"compose_file_path"`
	SQLiteSnapshotPath string `json:"sqlite_snapshot_path"`
	VolumeBackupPath   string `json:"volume_backup_path"`
	Status             string `json:"status"`
	CreatedAt          string `json:"created_at"`
}

// GetNextRevisionNumber returns the next sequential revision number.
func (d *DB) GetNextRevisionNumber(ctx context.Context) (int, error) {
	var nextRev int
	err := d.QueryRowContext(ctx, "SELECT COALESCE(MAX(revision), 0) + 1 FROM compose_revisions").Scan(&nextRev)
	if err != nil {
		return 0, fmt.Errorf("failed to query next revision number: %w", err)
	}
	return nextRev, nil
}

// RecordRevision inserts a revision record into compose_revisions and updates active_compose_revision.
func (d *DB) RecordRevision(ctx context.Context, rev *ComposeRevision) error {
	if rev == nil {
		return errors.New("compose revision cannot be nil")
	}
	if rev.Status == "" {
		rev.Status = "active"
	}

	if rev.Revision > 0 {
		_, err := d.ExecContext(ctx, `
			INSERT INTO compose_revisions (revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, rev.Revision, rev.ServiceID, rev.OldImage, rev.NewImage, rev.ComposeFilePath, rev.SQLiteSnapshotPath, rev.VolumeBackupPath, rev.Status)
		if err != nil {
			return fmt.Errorf("failed to insert compose revision: %w", err)
		}
	} else {
		res, err := d.ExecContext(ctx, `
			INSERT INTO compose_revisions (service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, rev.ServiceID, rev.OldImage, rev.NewImage, rev.ComposeFilePath, rev.SQLiteSnapshotPath, rev.VolumeBackupPath, rev.Status)
		if err != nil {
			return fmt.Errorf("failed to insert compose revision: %w", err)
		}
		id, err := res.LastInsertId()
		if err == nil {
			rev.Revision = int(id)
		}
	}

	if rev.Status == "active" {
		// Supersede previous active revisions
		_, err := d.ExecContext(ctx, `
			UPDATE compose_revisions
			SET status = 'superseded'
			WHERE status = 'active' AND revision != ?
		`, rev.Revision)
		if err != nil {
			return fmt.Errorf("failed to supersede prior active revisions: %w", err)
		}

		if err := d.SetActiveRevisionNumber(ctx, rev.Revision); err != nil {
			return fmt.Errorf("failed to set active revision in system state: %w", err)
		}
	}

	// Fetch recorded timestamp
	_ = d.QueryRowContext(ctx, "SELECT created_at FROM compose_revisions WHERE revision = ?", rev.Revision).Scan(&rev.CreatedAt)
	return nil
}

// GetRevision fetches a compose revision by its revision number.
func (d *DB) GetRevision(ctx context.Context, revision int) (*ComposeRevision, error) {
	row := d.QueryRowContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		WHERE revision = ?
	`, revision)

	var rev ComposeRevision
	if err := row.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query compose revision %d: %w", revision, err)
	}
	return &rev, nil
}

// GetActiveRevision returns the currently active compose revision.
func (d *DB) GetActiveRevision(ctx context.Context) (*ComposeRevision, error) {
	activeRevNum, err := d.GetActiveRevisionNumber(ctx)
	if err == nil && activeRevNum > 0 {
		rev, err := d.GetRevision(ctx, activeRevNum)
		if err == nil {
			return rev, nil
		}
	}

	row := d.QueryRowContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		WHERE status = 'active'
		ORDER BY revision DESC
		LIMIT 1
	`)

	var rev ComposeRevision
	if err := row.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query active compose revision: %w", err)
	}
	return &rev, nil
}

// ListRevisions returns all compose revisions ordered by revision descending.
func (d *DB) ListRevisions(ctx context.Context) ([]ComposeRevision, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		ORDER BY revision DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list compose revisions: %w", err)
	}
	defer rows.Close()

	revisions := make([]ComposeRevision, 0)
	for rows.Next() {
		var rev ComposeRevision
		if err := rows.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan compose revision: %w", err)
		}
		revisions = append(revisions, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating compose revisions: %w", err)
	}
	return revisions, nil
}

// UpdateRevisionStatus updates the status of a specific revision.
func (d *DB) UpdateRevisionStatus(ctx context.Context, revision int, status string) error {
	res, err := d.ExecContext(ctx, `
		UPDATE compose_revisions
		SET status = ?
		WHERE revision = ?
	`, status, revision)
	if err != nil {
		return fmt.Errorf("failed to update revision %d status: %w", revision, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if status == "active" {
		_, _ = d.ExecContext(ctx, `
			UPDATE compose_revisions
			SET status = 'superseded'
			WHERE status = 'active' AND revision != ?
		`, revision)
		_ = d.SetActiveRevisionNumber(ctx, revision)
	}
	return nil
}

// SetActiveRevisionNumber records the active revision number in system_state.
func (d *DB) SetActiveRevisionNumber(ctx context.Context, revision int) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO system_state (key, value, updated_at)
		VALUES ('active_compose_revision', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, strconv.Itoa(revision))
	if err != nil {
		return fmt.Errorf("failed to set active_compose_revision in system_state: %w", err)
	}
	return nil
}

// GetActiveRevisionNumber reads the active revision number from system_state.
func (d *DB) GetActiveRevisionNumber(ctx context.Context) (int, error) {
	var val string
	err := d.QueryRowContext(ctx, "SELECT value FROM system_state WHERE key = 'active_compose_revision'").Scan(&val)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query active_compose_revision: %w", err)
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return 0, nil
	}
	return num, nil
}

