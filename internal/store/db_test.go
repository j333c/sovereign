package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen_WALModeAndSynchronousPragma(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Verify PRAGMA journal_mode is 'wal'
	var journalMode string
	err = db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query PRAGMA journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", journalMode)
	}

	// Verify PRAGMA synchronous is 1 (NORMAL)
	var synchronous int
	err = db.QueryRowContext(ctx, "PRAGMA synchronous;").Scan(&synchronous)
	if err != nil {
		t.Fatalf("failed to query PRAGMA synchronous: %v", err)
	}
	if synchronous != 1 {
		t.Errorf("expected PRAGMA synchronous = 1 (NORMAL), got %d", synchronous)
	}

	// Verify baseline schema tables were created
	var count int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('system_state', 'settings', 'boot_status');").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 baseline tables, found %d", count)
	}

	// Verify initial boot status
	status, err := db.GetBootStatus(ctx)
	if err != nil {
		t.Fatalf("GetBootStatus failed: %v", err)
	}
	if status.Mode != "operational" {
		t.Errorf("expected boot status mode 'operational', got %q", status.Mode)
	}
	if !status.CleanShutdown {
		t.Errorf("expected clean_shutdown = true, got %v", status.CleanShutdown)
	}
	if !status.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = true, got %v", status.DiagnosticsCompleted)
	}

	// Verify integrity check passes
	ok, msg, err := db.CheckIntegrity(ctx)
	if err != nil {
		t.Fatalf("CheckIntegrity failed: %v", err)
	}
	if !ok || msg != "ok" {
		t.Errorf("expected integrity 'ok', got %q (ok=%v)", msg, ok)
	}
}

func TestOpen_UnwritableDirectory(t *testing.T) {
	// Attempt to open in an unwritable path
	dbPath := "/proc/nonexistent/sovereign.db"
	_, err := Open(dbPath)
	if err == nil {
		t.Fatalf("expected error when opening in unwritable path, got nil")
	}
}
