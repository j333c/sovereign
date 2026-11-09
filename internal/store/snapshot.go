package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
)

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
