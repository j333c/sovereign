package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Boot modes matching OpenAPI and architectural spine.
const (
	BootModeOperational = "operational"
	BootModeDiagnostics = "diagnostics"
	BootModeRecovery    = "recovery"
	BootModeFailed      = "failed"
)

// ReassuranceMessage is the exact consumer microcopy per UX-DR7 & UX-DR11.
const ReassuranceMessage = "Checking system safety after power restoration... your files are secure."

// BootStateFile represents the persistent state serialized in boot_state.json.
type BootStateFile struct {
	CleanShutdown bool      `json:"clean_shutdown"`
	Timestamp     time.Time `json:"timestamp"`
}

// BootManager manages the appliance boot lifecycle, dirty shutdown detection,
// atomic boot state persistence, and storage & SQLite integrity verification.
type BootManager struct {
	statePath       string
	db              *DB
	logger          *slog.Logger
	mu              sync.RWMutex
	currentStatus   *BootStatusInfo
	storageDegraded bool
	closed          bool
}

// NewBootManager creates a new BootManager instance.
func NewBootManager(statePath string, db *DB, logger *slog.Logger) *BootManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &BootManager{
		statePath: statePath,
		db:        db,
		logger:    logger,
		currentStatus: &BootStatusInfo{
			Mode:                 BootModeOperational,
			CleanShutdown:        true,
			DiagnosticsCompleted: true,
			Message:              "All systems operational",
		},
	}
}

// StatePath returns the file path to boot_state.json.
func (bm *BootManager) StatePath() string {
	return bm.statePath
}

// RecordBoot reads the persistent boot state from boot_state.json, determines whether
// the system experienced an unclean shutdown, enters DiagnosticsMode if unclean or missing,
// and immediately writes clean_shutdown = false to boot_state.json via atomic temp-file, fsync, and rename.
func (bm *BootManager) RecordBoot(ctx context.Context) (*BootStatusInfo, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.closed {
		return nil, errors.New("boot manager is closed")
	}

	cleanShutdownFound := false
	raw, err := os.ReadFile(bm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			bm.logger.Info("boot state file not found, initializing in diagnostics mode", "path", bm.statePath)
		} else {
			bm.logger.Warn("failed to read boot state file, assuming dirty shutdown", "path", bm.statePath, "error", err)
		}
	} else {
		var state BootStateFile
		if err := json.Unmarshal(raw, &state); err != nil {
			bm.logger.Warn("invalid boot state JSON, assuming dirty shutdown", "path", bm.statePath, "error", err)
		} else if state.CleanShutdown {
			cleanShutdownFound = true
		}
	}

	if cleanShutdownFound {
		bm.currentStatus = &BootStatusInfo{
			Mode:                 BootModeOperational,
			CleanShutdown:        true,
			DiagnosticsCompleted: true,
			Message:              "All systems operational",
		}
		bm.logger.Info("clean shutdown detected, entering operational mode")
	} else {
		bm.currentStatus = &BootStatusInfo{
			Mode:                 BootModeDiagnostics,
			CleanShutdown:        false,
			DiagnosticsCompleted: false,
			Message:              ReassuranceMessage,
		}
		bm.logger.Info("unclean shutdown detected, entering diagnostics mode")
	}

	// Atomically write clean_shutdown = false to boot_state.json
	// This ensures an ungraceful power loss while running will be detected on subsequent boot.
	uncleanMarker := BootStateFile{
		CleanShutdown: false,
		Timestamp:     time.Now().UTC(),
	}
	markerBytes, err := json.MarshalIndent(uncleanMarker, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal boot state: %w", err)
	}

	if err := atomicWriteFile(bm.statePath, markerBytes, 0644); err != nil {
		bm.logger.Error("failed to write dirty boot marker (storage may be read-only)", "path", bm.statePath, "error", err)
		bm.storageDegraded = true
		return bm.currentStatus, fmt.Errorf("failed to write boot state marker: %w", err)
	}

	if bm.db != nil {
		_ = bm.db.RecordBootStatus(ctx, bm.currentStatus)
	}

	return bm.currentStatus, nil
}

// RecordCleanShutdown gracefully records a clean shutdown marker to boot_state.json
// using atomic write and fsync, and flushes the SQLite DB WAL.
func (bm *BootManager) RecordCleanShutdown(ctx context.Context) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.closed {
		return errors.New("boot manager is closed")
	}

	bm.logger.Info("recording clean shutdown marker", "path", bm.statePath)

	// Flush SQLite WAL if database is open
	if bm.db != nil {
		if err := bm.db.FlushWAL(ctx); err != nil {
			bm.logger.Warn("failed to flush WAL during clean shutdown", "error", err)
		}
	}

	cleanMarker := BootStateFile{
		CleanShutdown: true,
		Timestamp:     time.Now().UTC(),
	}
	markerBytes, err := json.MarshalIndent(cleanMarker, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal clean shutdown state: %w", err)
	}

	if err := atomicWriteFile(bm.statePath, markerBytes, 0644); err != nil {
		bm.logger.Error("failed to write clean shutdown marker", "path", bm.statePath, "error", err)
		return fmt.Errorf("failed to write clean shutdown state: %w", err)
	}

	bm.currentStatus.CleanShutdown = true
	if bm.currentStatus.Mode == BootModeOperational {
		bm.currentStatus.DiagnosticsCompleted = true
	}

	if bm.db != nil {
		_ = bm.db.RecordBootStatus(ctx, bm.currentStatus)
	}

	return nil
}

// RunIntegrityCheck verifies storage mount writeability and runs SQLite PRAGMA integrity_check
// within 15 seconds. If successful, transitions mode to operational.
func (bm *BootManager) RunIntegrityCheck(ctx context.Context) error {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	// 1. Verify storage mount writeability
	if err := bm.checkStorageWriteable(); err != nil {
		bm.mu.Lock()
		bm.storageDegraded = true
		bm.currentStatus = &BootStatusInfo{
			Mode:                 BootModeRecovery,
			CleanShutdown:        bm.currentStatus.CleanShutdown,
			DiagnosticsCompleted: false,
			Message:              "Storage filesystem is read-only or unwritable",
		}
		bm.mu.Unlock()
		bm.logger.Error("storage writeability check failed", "error", err)
		return fmt.Errorf("storage writeability check failed: %w", err)
	}

	// 2. Run PRAGMA integrity_check on SQLite database
	if bm.db != nil {
		ok, result, err := bm.db.CheckIntegrity(ctx)
		if err != nil || !ok {
			bm.mu.Lock()
			bm.currentStatus = &BootStatusInfo{
				Mode:                 BootModeFailed,
				CleanShutdown:        false,
				DiagnosticsCompleted: false,
				Message:              fmt.Sprintf("System integrity check failed: database corrupted (%s). Automated snapshot restore recommended.", result),
			}
			bm.mu.Unlock()
			bm.logger.Error("database integrity check failed", "result", result, "error", err)
			return fmt.Errorf("database integrity check failed: %s (err: %w)", result, err)
		}
	}

	// 3. Mark operational
	bm.mu.Lock()
	bm.storageDegraded = false
	bm.currentStatus = &BootStatusInfo{
		Mode:                 BootModeOperational,
		CleanShutdown:        bm.currentStatus.CleanShutdown,
		DiagnosticsCompleted: true,
		Message:              "All systems operational",
	}
	bm.mu.Unlock()

	bm.logger.Info("boot diagnostics integrity check passed successfully")

	if bm.db != nil {
		_ = bm.db.RecordBootStatus(ctx, bm.currentStatus)
	}

	return nil
}

func (bm *BootManager) checkStorageWriteable() error {
	dir := filepath.Dir(bm.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to ensure state directory: %w", err)
	}

	probePath := filepath.Join(dir, fmt.Sprintf(".write_probe_%d.tmp", time.Now().UnixNano()))
	f, err := os.OpenFile(probePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("storage directory unwritable: %w", err)
	}
	if _, err := f.Write([]byte("probe")); err != nil {
		_ = f.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("failed to write probe: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("failed to fsync probe: %w", err)
	}
	_ = f.Close()
	_ = os.Remove(probePath)
	return nil
}

// GetStatus returns the current boot status, whether storage is degraded, and any error.
func (bm *BootManager) GetStatus() (*BootStatusInfo, bool, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if bm.closed {
		return nil, false, errors.New("boot manager is closed")
	}

	statusCopy := *bm.currentStatus
	return &statusCopy, bm.storageDegraded, nil
}

// IsDiagnosticsMode returns true if the system is currently in diagnostics mode.
func (bm *BootManager) IsDiagnosticsMode() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.currentStatus != nil && bm.currentStatus.Mode == BootModeDiagnostics
}

// IsStorageDegraded returns true if the storage mount is read-only or unwritable.
func (bm *BootManager) IsStorageDegraded() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.storageDegraded
}

// SetDB updates the database handle.
func (bm *BootManager) SetDB(db *DB) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.db = db
}

// Close closes the BootManager.
func (bm *BootManager) Close() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.closed = true
	return nil
}

// atomicWriteFile writes data to a temporary file in the same directory as destPath,
// calls fsync(), closes it, and atomically renames it to destPath.
func atomicWriteFile(destPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".boot_state-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to fsync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("failed to atomically rename %q to %q: %w", tmpName, destPath, err)
	}

	cleanedUp = true

	// Best-effort directory sync to guarantee directory entry persistence
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}
