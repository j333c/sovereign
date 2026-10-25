package store

import (
	"context"
	"os"
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

func TestDB_Settings(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Missing key returns empty string
	val, err := db.GetSetting(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSetting failed for nonexistent key: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}

	// Insert setting
	if err := db.SetSetting(ctx, "test_key", "test_value"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err = db.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "test_value" {
		t.Fatalf("expected 'test_value', got %q", val)
	}

	// Upsert setting
	if err := db.SetSetting(ctx, "test_key", "updated_value"); err != nil {
		t.Fatalf("SetSetting update failed: %v", err)
	}

	val, err = db.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting failed after update: %v", err)
	}
	if val != "updated_value" {
		t.Fatalf("expected 'updated_value', got %q", val)
	}
}

func TestDB_CreateSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	_ = db.SetSetting(ctx, "sample_key", "sample_val")

	// 1. Snapshot with directory creation
	snapPath := filepath.Join(tempDir, "backups", "subfolder", "pre-upgrade-1.db")
	if err := db.CreateSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Verify snapshot file exists and is a valid SQLite DB
	snapDB, err := Open(snapPath)
	if err != nil {
		t.Fatalf("failed to open snapshot DB: %v", err)
	}
	defer snapDB.Close()

	ok, msg, err := snapDB.CheckIntegrity(ctx)
	if err != nil || !ok || msg != "ok" {
		t.Fatalf("snapshot DB integrity check failed: ok=%v, msg=%s, err=%v", ok, msg, err)
	}

	val, err := snapDB.GetSetting(ctx, "sample_key")
	if err != nil || val != "sample_val" {
		t.Fatalf("snapshot DB missing expected setting: val=%q, err=%v", val, err)
	}

	// 2. Overwrite safety: running CreateSnapshot again on existing file must not fail
	if err := db.CreateSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("CreateSnapshot overwrite failed: %v", err)
	}

	// 3. Empty destPath must fail
	if err := db.CreateSnapshot(ctx, ""); err == nil {
		t.Fatalf("expected error for empty snapshot destination, got nil")
	}
}

func TestDB_RestoreSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initial state before snapshot
	if err := db.SetSetting(ctx, "cluster_name", "alpha"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	if err := db.SetSetting(ctx, "app_version", "1.0.0"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	snapPath := filepath.Join(tempDir, "backups", "pre-upgrade-1.db")
	if err := db.CreateSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Mutate state after snapshot (simulating post-upgrade modifications)
	if err := db.SetSetting(ctx, "app_version", "2.0.0"); err != nil {
		t.Fatalf("SetSetting update failed: %v", err)
	}
	if err := db.SetSetting(ctx, "newly_added_key", "unwanted_data"); err != nil {
		t.Fatalf("SetSetting insert failed: %v", err)
	}

	val, err := db.GetSetting(ctx, "app_version")
	if err != nil || val != "2.0.0" {
		t.Fatalf("expected post-upgrade version '2.0.0', got %q", val)
	}

	// Restore snapshot
	if err := db.RestoreSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// Verify restored state matches pre-upgrade snapshot
	val, err = db.GetSetting(ctx, "app_version")
	if err != nil || val != "1.0.0" {
		t.Fatalf("expected restored version '1.0.0', got %q (err=%v)", val, err)
	}

	val, err = db.GetSetting(ctx, "cluster_name")
	if err != nil || val != "alpha" {
		t.Fatalf("expected restored cluster_name 'alpha', got %q (err=%v)", val, err)
	}

	val, err = db.GetSetting(ctx, "newly_added_key")
	if err != nil || val != "" {
		t.Fatalf("expected newly_added_key to be gone after restore, got %q", val)
	}

	// Verify WAL mode is preserved and functional
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("expected journal_mode 'wal' after restore, got %q", journalMode)
	}

	ok, msg, err := db.CheckIntegrity(ctx)
	if err != nil || !ok || msg != "ok" {
		t.Fatalf("CheckIntegrity failed after restore: ok=%v, msg=%s, err=%v", ok, msg, err)
	}

	// Error handling tests:
	// Empty path
	if err := db.RestoreSnapshot(ctx, ""); err == nil {
		t.Fatalf("expected error for empty snapshot path, got nil")
	}

	// Non-existent path
	if err := db.RestoreSnapshot(ctx, filepath.Join(tempDir, "does-not-exist.db")); err == nil {
		t.Fatalf("expected error for non-existent snapshot path, got nil")
	}

	// Directory path
	if err := db.RestoreSnapshot(ctx, tempDir); err == nil {
		t.Fatalf("expected error for directory path, got nil")
	}
}

func TestRestoreSnapshot_ConnectionCyclingAndFileReplacement(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Setup baseline state
	if err := db.SetSetting(ctx, "baseline_key", "baseline_val"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	snapPath := filepath.Join(tempDir, "backups", "test-pre-upgrade.db")
	if err := db.CreateSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Mutate active DB post-snapshot
	if err := db.SetSetting(ctx, "post_upgrade_key", "unwanted"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Capture the active database connection pool handle before restore
	oldDBPool := db.DB

	// Perform snapshot restoration
	if err := db.RestoreSnapshot(ctx, snapPath); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// 1. Verify that the underlying connection pool was replaced
	if db.DB == oldDBPool {
		t.Fatalf("expected database connection pool handle to be replaced upon restore")
	}

	// 2. Verify that the previous connection pool was cleanly closed
	if err := oldDBPool.Ping(); err == nil {
		t.Fatalf("expected old database connection pool to be closed, but ping succeeded")
	}

	// 3. Verify that the new database connection pool is active and operational
	if err := db.Ping(); err != nil {
		t.Fatalf("expected new database connection pool to be responsive, ping failed: %v", err)
	}

	// 4. Verify disk file content reflects the snapshot state
	val, err := db.GetSetting(ctx, "baseline_key")
	if err != nil || val != "baseline_val" {
		t.Fatalf("expected baseline_val, got %q (err=%v)", val, err)
	}
	val, err = db.GetSetting(ctx, "post_upgrade_key")
	if err != nil || val != "" {
		t.Fatalf("expected post_upgrade_key to be wiped, got %q", val)
	}

	// 5. Verify corrupted snapshot is rejected with an error
	corruptSnapPath := filepath.Join(tempDir, "backups", "corrupted.db")
	if err := os.WriteFile(corruptSnapPath, []byte("NOT_A_VALID_SQLITE_DATABASE_HEADER"), 0644); err != nil {
		t.Fatalf("failed to write corrupted snapshot file: %v", err)
	}

	if err := db.RestoreSnapshot(ctx, corruptSnapPath); err == nil {
		t.Fatalf("expected RestoreSnapshot to fail on corrupted snapshot file, got nil")
	}
}

func TestDB_ComposeRevisions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initial next revision is 1
	nextRev, err := db.GetNextRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("GetNextRevisionNumber failed: %v", err)
	}
	if nextRev != 1 {
		t.Fatalf("expected initial next revision 1, got %d", nextRev)
	}

	// No active revision initially
	_, err = db.GetActiveRevision(ctx)
	if err == nil {
		t.Fatalf("expected error for initial GetActiveRevision, got nil")
	}

	// Record Revision 1
	rev1 := &ComposeRevision{
		Revision:           1,
		ServiceID:          "jellyfin",
		OldImage:           "jellyfin/jellyfin:12.2.0",
		NewImage:           "jellyfin/jellyfin:12.3.0",
		ComposeFilePath:    "/data/state/compose/rev-1.yaml",
		SQLiteSnapshotPath: "/data/state/backups/pre-upgrade-1.db",
		VolumeBackupPath:   "/data/state/backups/volumes-rev-1/jellyfin",
		Status:             "active",
	}
	if err := db.RecordRevision(ctx, rev1); err != nil {
		t.Fatalf("RecordRevision failed: %v", err)
	}

	// Verify Active Revision
	active, err := db.GetActiveRevision(ctx)
	if err != nil {
		t.Fatalf("GetActiveRevision failed: %v", err)
	}
	if active.Revision != 1 || active.ServiceID != "jellyfin" || active.Status != "active" {
		t.Fatalf("unexpected active revision: %+v", active)
	}

	activeNum, err := db.GetActiveRevisionNumber(ctx)
	if err != nil || activeNum != 1 {
		t.Fatalf("expected active revision number 1, got %d (err=%v)", activeNum, err)
	}

	// Next revision is now 2
	nextRev, err = db.GetNextRevisionNumber(ctx)
	if err != nil || nextRev != 2 {
		t.Fatalf("expected next revision 2, got %d", nextRev)
	}

	// Record Revision 2
	rev2 := &ComposeRevision{
		Revision:           2,
		ServiceID:          "navidrome",
		OldImage:           "deluan/navidrome:0.54.5",
		NewImage:           "deluan/navidrome:0.55.0",
		ComposeFilePath:    "/data/state/compose/rev-2.yaml",
		SQLiteSnapshotPath: "/data/state/backups/pre-upgrade-2.db",
		VolumeBackupPath:   "/data/state/backups/volumes-rev-2/navidrome",
		Status:             "active",
	}
	if err := db.RecordRevision(ctx, rev2); err != nil {
		t.Fatalf("RecordRevision failed for rev 2: %v", err)
	}

	// Rev 1 should now be superseded, Rev 2 is active
	active, err = db.GetActiveRevision(ctx)
	if err != nil || active.Revision != 2 {
		t.Fatalf("expected active revision 2, got %+v (err=%v)", active, err)
	}

	oldRev1, err := db.GetRevision(ctx, 1)
	if err != nil {
		t.Fatalf("GetRevision(1) failed: %v", err)
	}
	if oldRev1.Status != "superseded" {
		t.Fatalf("expected rev 1 to be superseded, got %q", oldRev1.Status)
	}

	// List revisions
	revs, err := db.ListRevisions(ctx)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revs))
	}
	if revs[0].Revision != 2 || revs[1].Revision != 1 {
		t.Fatalf("expected descending order [2, 1], got [%d, %d]", revs[0].Revision, revs[1].Revision)
	}

	// Update status
	if err := db.UpdateRevisionStatus(ctx, 2, "rolled_back"); err != nil {
		t.Fatalf("UpdateRevisionStatus failed: %v", err)
	}
	updatedRev2, err := db.GetRevision(ctx, 2)
	if err != nil || updatedRev2.Status != "rolled_back" {
		t.Fatalf("expected status rolled_back, got %+v (err=%v)", updatedRev2, err)
	}

	// Active revision should not return rolled_back revision
	_, err = db.GetActiveRevision(ctx)
	if err == nil {
		t.Fatalf("expected error querying active revision when active revision was rolled back, got nil")
	}

	// Updating nonexistent revision returns error
	if err := db.UpdateRevisionStatus(ctx, 999, "active"); err == nil {
		t.Fatalf("expected error updating nonexistent revision, got nil")
	}
}


