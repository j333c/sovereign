package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootManager_DirtyBoot_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state", "boot_state.json")
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)

	ctx := context.Background()
	status, err := bm.RecordBoot(ctx)
	if err != nil {
		t.Fatalf("unexpected error from RecordBoot: %v", err)
	}

	if status.Mode != BootModeDiagnostics {
		t.Errorf("expected mode %q, got %q", BootModeDiagnostics, status.Mode)
	}
	if status.CleanShutdown {
		t.Errorf("expected clean_shutdown = false on missing state file")
	}
	if status.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = false")
	}
	if status.Message != ReassuranceMessage {
		t.Errorf("expected reassurance copy %q, got %q", ReassuranceMessage, status.Message)
	}

	// Verify boot_state.json was written with clean_shutdown = false
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("expected boot_state.json to be created: %v", err)
	}

	var state BootStateFile
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("failed to parse boot_state.json: %v", err)
	}
	if state.CleanShutdown {
		t.Errorf("expected persisted clean_shutdown = false")
	}
}

func TestBootManager_DirtyBoot_CleanShutdownFalse(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	statePath := filepath.Join(stateDir, "boot_state.json")
	dbPath := filepath.Join(stateDir, "sovereign.db")

	// Pre-create boot_state.json with clean_shutdown: false
	initial := `{"clean_shutdown": false, "timestamp": "2026-10-15T12:00:00Z"}`
	if err := os.WriteFile(statePath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial state file: %v", err)
	}

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)
	status, err := bm.RecordBoot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error from RecordBoot: %v", err)
	}

	if status.Mode != BootModeDiagnostics {
		t.Errorf("expected mode %q, got %q", BootModeDiagnostics, status.Mode)
	}
	if status.CleanShutdown {
		t.Errorf("expected clean_shutdown = false")
	}
	if !bm.IsDiagnosticsMode() {
		t.Errorf("expected IsDiagnosticsMode() = true")
	}
}

func TestBootManager_CleanBoot(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	statePath := filepath.Join(stateDir, "boot_state.json")
	dbPath := filepath.Join(stateDir, "sovereign.db")

	// Pre-create boot_state.json with clean_shutdown: true
	initial := `{"clean_shutdown": true, "timestamp": "2026-10-15T12:00:00Z"}`
	if err := os.WriteFile(statePath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial state file: %v", err)
	}

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)
	status, err := bm.RecordBoot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error from RecordBoot: %v", err)
	}

	if status.Mode != BootModeOperational {
		t.Errorf("expected mode %q, got %q", BootModeOperational, status.Mode)
	}
	if !status.CleanShutdown {
		t.Errorf("expected clean_shutdown = true")
	}
	if !status.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = true")
	}
	if bm.IsDiagnosticsMode() {
		t.Errorf("expected IsDiagnosticsMode() = false")
	}

	// Verify boot_state.json was updated to clean_shutdown = false for the active running session
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read boot_state.json: %v", err)
	}
	var state BootStateFile
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("failed to parse boot_state.json: %v", err)
	}
	if state.CleanShutdown {
		t.Errorf("expected clean_shutdown to be reset to false for active session")
	}
}

func TestBootManager_RecordCleanShutdown_AtomicPersistence(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state", "boot_state.json")
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)
	_, _ = bm.RecordBoot(context.Background())
	// Integrity check passes, transitioning to operational mode
	if err := bm.RunIntegrityCheck(context.Background()); err != nil {
		t.Fatalf("RunIntegrityCheck failed: %v", err)
	}

	// Perform graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := bm.RecordCleanShutdown(shutdownCtx); err != nil {
		t.Fatalf("RecordCleanShutdown failed: %v", err)
	}

	// Check status
	status, degraded, err := bm.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}
	if degraded {
		t.Errorf("expected degraded = false")
	}
	if !status.CleanShutdown {
		t.Errorf("expected status.CleanShutdown = true")
	}

	// Verify on-disk file has clean_shutdown: true and recent timestamp
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read boot_state.json: %v", err)
	}

	var state BootStateFile
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("failed to parse boot_state.json: %v", err)
	}
	if !state.CleanShutdown {
		t.Errorf("expected state.CleanShutdown = true")
	}
	if time.Since(state.Timestamp) > 10*time.Second {
		t.Errorf("expected recent timestamp, got %v", state.Timestamp)
	}
}

func TestBootManager_RunIntegrityCheck_Success(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state", "boot_state.json")
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)
	// Start with dirty boot
	_, _ = bm.RecordBoot(context.Background())
	if !bm.IsDiagnosticsMode() {
		t.Fatalf("expected DiagnosticsMode before check")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := bm.RunIntegrityCheck(ctx); err != nil {
		t.Fatalf("RunIntegrityCheck failed: %v", err)
	}

	if bm.IsDiagnosticsMode() {
		t.Errorf("expected IsDiagnosticsMode() = false after successful check")
	}

	status, degraded, err := bm.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if degraded {
		t.Errorf("expected degraded = false")
	}
	if status.Mode != BootModeOperational {
		t.Errorf("expected mode %q, got %q", BootModeOperational, status.Mode)
	}
	if !status.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = true")
	}
}

func TestBootManager_RecordCleanShutdown_SkippedWhenNotOperational(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state", "boot_state.json")
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	bm := NewBootManager(statePath, db, nil)
	// Boot enters diagnostics mode
	_, _ = bm.RecordBoot(context.Background())

	// Attempt clean shutdown while still in diagnostics mode
	err = bm.RecordCleanShutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error from RecordCleanShutdown: %v", err)
	}

	// Verify boot_state.json still has clean_shutdown = false
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read boot_state.json: %v", err)
	}
	var state BootStateFile
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("failed to parse state: %v", err)
	}
	if state.CleanShutdown {
		t.Errorf("clean_shutdown should NOT be recorded as true when mode != operational")
	}
}

func TestBootManager_RunIntegrityCheck_NilDB(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state", "boot_state.json")

	bm := NewBootManager(statePath, nil, nil)
	ctx := context.Background()

	err := bm.RunIntegrityCheck(ctx)
	if err == nil {
		t.Fatalf("expected error when DB handle is nil, got nil")
	}

	status, _, _ := bm.GetStatus()
	if status.Mode != BootModeFailed {
		t.Errorf("expected mode failed on nil DB handle, got %q", status.Mode)
	}
}

func TestBootManager_RunIntegrityCheck_CorruptedDB(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	statePath := filepath.Join(stateDir, "boot_state.json")
	dbPath := filepath.Join(stateDir, "sovereign.db")

	// Create valid db and populate rows so additional b-tree pages are allocated
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	for i := 0; i < 150; i++ {
		_, _ = db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", fmt.Sprintf("key_%d", i), strings.Repeat("A", 200))
	}
	_ = db.FlushWAL(context.Background())
	_ = db.Close()

	// Corrupt database page at offset 4096 (preserving database file header)
	f, err := os.OpenFile(dbPath, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open db for corruption: %v", err)
	}
	_, _ = f.WriteAt([]byte("CORRUPTED_PAGE_CONTENT_GARBAGE"), 4096)
	_ = f.Close()

	// Reopen corrupted db
	corruptDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open corrupted db: %v", err)
	}
	defer corruptDB.Close()

	bm := NewBootManager(statePath, corruptDB, nil)
	_, _ = bm.RecordBoot(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = bm.RunIntegrityCheck(ctx)
	if err == nil {
		t.Fatalf("expected error from RunIntegrityCheck on corrupted db, got nil")
	}

	// Verify error does NOT contain %!w(<nil>)
	if strings.Contains(err.Error(), "%!w") {
		t.Errorf("error contains bad formatting string: %v", err)
	}

	status, _, _ := bm.GetStatus()
	if status.Mode != BootModeFailed {
		t.Errorf("expected mode %q on corrupted db, got %q", BootModeFailed, status.Mode)
	}
	if status.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = false on failure")
	}
	if !strings.Contains(status.Message, "Integrity check failed") && !strings.Contains(status.Message, "corrupted") {
		t.Errorf("expected descriptive failure message, got %q", status.Message)
	}
}

func TestBootManager_StorageUnwritable(t *testing.T) {
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly", "state")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	statePath := filepath.Join(readOnlyDir, "boot_state.json")

	bm := NewBootManager(statePath, nil, nil)

	// Make directory read-only
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := bm.RunIntegrityCheck(ctx)
	if err == nil {
		t.Fatalf("expected error on read-only directory, got nil")
	}

	if !bm.IsStorageDegraded() {
		t.Errorf("expected IsStorageDegraded() = true")
	}

	status, degraded, _ := bm.GetStatus()
	if !degraded {
		t.Errorf("expected degraded = true in GetStatus")
	}
	if status.Mode != BootModeRecovery {
		t.Errorf("expected mode %q, got %q", BootModeRecovery, status.Mode)
	}
}

func TestAtomicWriteFile_Guarantees(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "state", "test_atomic.json")

	data := []byte(`{"hello": "world"}`)
	if err := atomicWriteFile(targetPath, data, 0644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("expected content %q, got %q", string(data), string(content))
	}

	// Verify no stray temp files left behind in the directory
	dirEntries, err := os.ReadDir(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(dirEntries) != 1 {
		t.Errorf("expected exactly 1 file in directory, found %d", len(dirEntries))
	}
}
