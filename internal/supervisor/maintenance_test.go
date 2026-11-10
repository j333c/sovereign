package supervisor

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"sovereign/internal/store"
)

func TestMaintenanceScheduler_ScheduleAndWindow(t *testing.T) {
	cfg := DefaultMaintenanceConfig()
	if cfg.Hour != 3 || cfg.Minute != 0 {
		t.Fatalf("expected 3:00 AM default, got %02d:%02d", cfg.Hour, cfg.Minute)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sched := NewMaintenanceScheduler(nil, cfg, logger)

	t1 := time.Date(2026, 11, 10, 3, 0, 15, 0, time.UTC)
	if !sched.IsMaintenanceWindow(t1) {
		t.Errorf("expected 03:00:15 to match maintenance window")
	}

	t2 := time.Date(2026, 11, 10, 3, 1, 0, 0, time.UTC)
	if sched.IsMaintenanceWindow(t2) {
		t.Errorf("expected 03:01:00 to not match maintenance window")
	}
}

func TestMaintenanceScheduler_ExecuteMaintenanceWindow_WALSnapshotAndStatus(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_maintenance.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 1. Initial idle execution without updates
	sched := NewMaintenanceScheduler(nil, DefaultMaintenanceConfig(), logger)
	sched.SetStore(db)

	_, err = sched.ExecuteMaintenanceWindow(ctx, nil)
	// ServiceManager is nil, but status should be recorded
	if err == nil {
		t.Fatalf("expected error due to nil service manager, got nil")
	}

	bootStatus, err := db.GetBootStatus(ctx)
	if err != nil {
		t.Fatalf("failed to get boot status: %v", err)
	}
	if bootStatus.LastUpdateStatus != "idle" {
		t.Errorf("expected last_update_status 'idle', got %q", bootStatus.LastUpdateStatus)
	}

	// 2. Test SetOvernightUpdateStatus and DismissOvernightUpdateStatus
	err = db.SetOvernightUpdateStatus(ctx, "updated", "v1.2.0", "Sovereign updated to v1.2.0 overnight")
	if err != nil {
		t.Fatalf("SetOvernightUpdateStatus failed: %v", err)
	}

	bootStatus, err = db.GetBootStatus(ctx)
	if err != nil {
		t.Fatalf("GetBootStatus failed: %v", err)
	}
	if bootStatus.LastUpdateStatus != "updated" {
		t.Errorf("expected status 'updated', got %q", bootStatus.LastUpdateStatus)
	}
	if bootStatus.LastUpdateVersion != "v1.2.0" {
		t.Errorf("expected version 'v1.2.0', got %q", bootStatus.LastUpdateVersion)
	}
	if bootStatus.LastUpdateDismissed {
		t.Errorf("expected last_update_dismissed false, got true")
	}

	err = db.DismissOvernightUpdateStatus(ctx)
	if err != nil {
		t.Fatalf("DismissOvernightUpdateStatus failed: %v", err)
	}

	bootStatus, err = db.GetBootStatus(ctx)
	if err != nil {
		t.Fatalf("GetBootStatus failed after dismiss: %v", err)
	}
	if !bootStatus.LastUpdateDismissed {
		t.Errorf("expected last_update_dismissed true, got false")
	}

	// 3. Test Postponed Status
	err = db.SetOvernightUpdateStatus(ctx, "postponed", "v1.3.0", "Update postponed: automatic safety check prevented an issue. All data remains secure.")
	if err != nil {
		t.Fatalf("SetOvernightUpdateStatus postponed failed: %v", err)
	}

	bootStatus, err = db.GetBootStatus(ctx)
	if err != nil {
		t.Fatalf("GetBootStatus failed: %v", err)
	}
	if bootStatus.LastUpdateStatus != "postponed" {
		t.Errorf("expected status 'postponed', got %q", bootStatus.LastUpdateStatus)
	}
	if bootStatus.LastUpdateDismissed {
		t.Errorf("expected last_update_dismissed false after new status, got true")
	}
}

func TestMaintenanceScheduler_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultMaintenanceConfig()
	cfg.CheckInterval = 10 * time.Millisecond

	sched := NewMaintenanceScheduler(nil, cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	// Calling Start again should be a no-op
	sched.Start(ctx)

	time.Sleep(25 * time.Millisecond)
	sched.Stop()
	// Calling Stop again should be a no-op
	sched.Stop()
}
