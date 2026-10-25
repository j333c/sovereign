package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
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
			return fmt.Errorf("vacuum into failed: %w", errFallback)
		}
	}

	return nil
}

// RestoreSnapshot safely replaces the active SQLite database with the pre-upgrade snapshot
// at snapshotPath, executing PRAGMA wal_checkpoint(TRUNCATE), cleanly closing the active database
// connection pool, replacing the database file on disk, reopening the database, and running an integrity check.
func (d *DB) RestoreSnapshot(ctx context.Context, snapshotPath string) error {
	if snapshotPath == "" {
		return errors.New("snapshot path cannot be empty")
	}

	fi, err := os.Stat(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("snapshot file does not exist: %s", snapshotPath)
		}
		return fmt.Errorf("failed to stat snapshot file %s: %w", snapshotPath, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("snapshot path is a directory, expected file: %s", snapshotPath)
	}

	// Ensure snapshot does not resolve to the same destination file
	absSnapshot, err1 := filepath.Abs(snapshotPath)
	absDest, err2 := filepath.Abs(d.path)
	if err1 == nil && err2 == nil && absSnapshot == absDest {
		return errors.New("snapshot path cannot be the same as the database file")
	}
	if destFi, err := os.Stat(d.path); err == nil && os.SameFile(fi, destFi) {
		return errors.New("snapshot path cannot be the same as the database file")
	}

	// 1. Flush active WAL to truncate log before closing connection
	_ = d.FlushWAL(ctx)

	// 2. Cleanly close active database connection pool before touching disk files
	if d.DB != nil {
		if err := d.DB.Close(); err != nil {
			return fmt.Errorf("failed to close active database connection pool: %w", err)
		}
	}

	// 3. Remove existing WAL and SHM files to prevent desynchronization with restored file
	_ = os.Remove(d.path + "-wal")
	_ = os.Remove(d.path + "-shm")

	// 4. Stage copy of snapshot file to temporary file with Sync(), then atomically rename
	tmpPath := d.path + ".restore.tmp"
	defer os.Remove(tmpPath)

	srcFile, err := os.Open(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file %s: %w", snapshotPath, err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open staging file %s: %w", tmpPath, err)
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close()
		return fmt.Errorf("failed to copy snapshot content to staging file: %w", err)
	}
	if err := destFile.Sync(); err != nil {
		destFile.Close()
		return fmt.Errorf("failed to sync staging file: %w", err)
	}
	if err := destFile.Close(); err != nil {
		return fmt.Errorf("failed to close staging file: %w", err)
	}
	_ = srcFile.Close()

	if err := os.Rename(tmpPath, d.path); err != nil {
		return fmt.Errorf("failed to replace database file %s with snapshot: %w", d.path, err)
	}

	// 5. Reopen SQLite database with WAL mode and synchronous=NORMAL
	separator := "?"
	if strings.Contains(d.path, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", d.path, separator)

	newDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to reopen sqlite database: %w", err)
	}

	newDB.SetMaxOpenConns(1)
	newDB.SetMaxIdleConns(1)
	newDB.SetConnMaxLifetime(0)

	var journalMode string
	if err := newDB.QueryRowContext(ctx, "PRAGMA journal_mode = WAL;").Scan(&journalMode); err != nil {
		newDB.Close()
		return fmt.Errorf("failed to enforce journal_mode=WAL on reopened database: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(journalMode)) != "wal" {
		newDB.Close()
		return fmt.Errorf("expected journal_mode WAL on reopened database, got: %s", journalMode)
	}

	if _, err := newDB.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		newDB.Close()
		return fmt.Errorf("failed to apply synchronous=NORMAL on reopened database: %w", err)
	}

	d.DB = newDB

	// 6. Verify integrity of restored database
	ok, msg, err := d.CheckIntegrity(ctx)
	if err != nil {
		newDB.Close()
		d.DB = nil
		return fmt.Errorf("integrity check failed after restore: %w", err)
	}
	if !ok || msg != "ok" {
		newDB.Close()
		d.DB = nil
		return fmt.Errorf("database corrupted after restore: %s", msg)
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

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin revision transaction: %w", err)
	}
	defer tx.Rollback()

	if rev.Revision > 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO compose_revisions (revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, rev.Revision, rev.ServiceID, rev.OldImage, rev.NewImage, rev.ComposeFilePath, rev.SQLiteSnapshotPath, rev.VolumeBackupPath, rev.Status)
		if err != nil {
			return fmt.Errorf("failed to insert compose revision: %w", err)
		}
	} else {
		res, err := tx.ExecContext(ctx, `
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
		_, err := tx.ExecContext(ctx, `
			UPDATE compose_revisions
			SET status = 'superseded'
			WHERE status = 'active' AND revision != ?
		`, rev.Revision)
		if err != nil {
			return fmt.Errorf("failed to supersede prior active revisions: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO system_state (key, value, updated_at)
			VALUES ('active_compose_revision', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
		`, strconv.Itoa(rev.Revision))
		if err != nil {
			return fmt.Errorf("failed to set active revision in system state: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit revision transaction: %w", err)
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
		if err == nil && rev.Status == "active" {
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
	} else {
		activeRevNum, err := d.GetActiveRevisionNumber(ctx)
		if err == nil && activeRevNum == revision {
			var latestActive int
			err := d.QueryRowContext(ctx, "SELECT revision FROM compose_revisions WHERE status = 'active' ORDER BY revision DESC LIMIT 1").Scan(&latestActive)
			if err == nil && latestActive > 0 {
				_ = d.SetActiveRevisionNumber(ctx, latestActive)
			} else {
				_, _ = d.ExecContext(ctx, "DELETE FROM system_state WHERE key = 'active_compose_revision'")
			}
		}
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

