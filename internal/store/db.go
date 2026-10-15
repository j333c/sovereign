package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
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

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}
