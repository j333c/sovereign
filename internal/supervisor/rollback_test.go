package supervisor

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"sovereign/internal/store"
)

type mockAlertNotifier struct {
	mu            sync.Mutex
	alerts        []string
	rollbackCalls []struct {
		ServiceID   string
		ServiceName string
		FailedImage string
		Restored    string
		Reason      string
	}
}

func (m *mockAlertNotifier) SendRollbackAlert(ctx context.Context, serviceID, serviceName, failedVersion, restoredVersion, failureReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, serviceID)
	m.rollbackCalls = append(m.rollbackCalls, struct {
		ServiceID   string
		ServiceName string
		FailedImage string
		Restored    string
		Reason      string
	}{
		ServiceID:   serviceID,
		ServiceName: serviceName,
		FailedImage: failedVersion,
		Restored:    restoredVersion,
		Reason:      failureReason,
	})
	return nil
}

func TestVerificationEngine_PassesWhenHealthy(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		Engine:        engine,
		RevisionStore: db,
	})
	defer mgr.Close()

	ctx := context.Background()
	_, err = mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	cfg := VerificationConfig{
		WindowDuration:      50 * time.Millisecond,
		ProbeInterval:       10 * time.Millisecond,
		ConsecutiveFailures: 3,
		TimeFunc:            func() time.Time { return time.Now() },
	}
	verifier := NewVerificationEngine(mgr, cfg, nil)
	mgr.SetVerificationEngine(verifier)

	sess := verifier.StartSession(ctx, "jellyfin", 1)
	if sess == nil {
		t.Fatalf("expected non-nil session")
	}

	select {
	case <-sess.Done:
		if !sess.Passed {
			t.Errorf("expected session to pass when healthy, got passed=%v, rolledBack=%v", sess.Passed, sess.RolledBack)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("verification session timed out")
	}
}

func TestVerificationEngine_TriggersAutomatedRollbackOnConsecutiveFailures(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()
	notifier := &mockAlertNotifier{}
	ctx := context.Background()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		Engine:        engine,
		RevisionStore: db,
		AlertNotifier: notifier,
	})
	defer mgr.Close()

	// 1. Initial service activation (revision 1)
	_, err = mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	// 2. Upgrade service to revision 2 with a new image
	rev2, err := mgr.UpgradeService(ctx, "jellyfin", "jellyfin/jellyfin:12.3.0")
	if err != nil {
		t.Fatalf("UpgradeService failed: %v", err)
	}
	if rev2.Revision != 1 {
		t.Logf("rev2 revision number: %d", rev2.Revision)
	}

	// 3. Configure verification engine with fast probe interval for testing
	cfg := VerificationConfig{
		WindowDuration:      2 * time.Second,
		ProbeInterval:       15 * time.Millisecond,
		ConsecutiveFailures: 3,
		TimeFunc:            func() time.Time { return time.Now() },
	}
	verifier := NewVerificationEngine(mgr, cfg, nil)
	mgr.SetVerificationEngine(verifier)

	// Simulate health check failures on the new container
	engine.OnProbeHealth = func(ctx context.Context, name string) (bool, error) {
		return false, errors.New("connection refused")
	}

	sess := verifier.StartSession(ctx, "jellyfin", rev2.Revision)

	select {
	case <-sess.Done:
		if !sess.RolledBack {
			t.Errorf("expected session to trigger rollback, got rolledBack=%v, passed=%v", sess.RolledBack, sess.Passed)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for verification rollback")
	}

	// 4. Verify service status degraded with reassurance microcopy
	st, err := mgr.GetService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("GetService failed: %v", err)
	}
	if st.Status != StatusDegraded {
		t.Errorf("expected status 'degraded', got %q", st.Status)
	}
	if st.Health != HealthDegraded {
		t.Errorf("expected health 'degraded', got %q", st.Health)
	}
	if st.ReassuranceMessage != ReassuranceRollbackRestored {
		t.Errorf("expected reassurance copy %q, got %q", ReassuranceRollbackRestored, st.ReassuranceMessage)
	}
	if st.Image != ImageJellyfin {
		t.Errorf("expected restored image %q, got %q", ImageJellyfin, st.Image)
	}

	// 5. Verify alert email notification was dispatched
	notifier.mu.Lock()
	if len(notifier.rollbackCalls) == 0 {
		t.Errorf("expected rollback alert email to be dispatched")
	} else {
		call := notifier.rollbackCalls[0]
		if call.ServiceID != "jellyfin" {
			t.Errorf("expected alert for jellyfin, got %q", call.ServiceID)
		}
	}
	notifier.mu.Unlock()

	// 6. Verify revision status in store
	revRecord, err := db.GetRevision(ctx, rev2.Revision)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if revRecord.Status != "rolled_back" {
		t.Errorf("expected revision status 'rolled_back', got %q", revRecord.Status)
	}
}

func TestVerificationEngine_CrashLoopTriggersImmediateRollback(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()
	notifier := &mockAlertNotifier{}
	ctx := context.Background()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		Engine:        engine,
		RevisionStore: db,
		AlertNotifier: notifier,
	})
	defer mgr.Close()

	_, err = mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	rev, err := mgr.UpgradeService(ctx, "jellyfin", "12.3.0")
	if err != nil {
		t.Fatalf("UpgradeService failed: %v", err)
	}

	cfg := VerificationConfig{
		WindowDuration:      2 * time.Second,
		ProbeInterval:       15 * time.Millisecond,
		ConsecutiveFailures: 3,
		TimeFunc:            func() time.Time { return time.Now() },
	}
	verifier := NewVerificationEngine(mgr, cfg, nil)
	mgr.SetVerificationEngine(verifier)

	// Mark container as stopped/crashed in status
	mgr.mu.Lock()
	if st, exists := mgr.statuses["jellyfin"]; exists {
		st.Status = StatusStopped
	}
	mgr.mu.Unlock()

	sess := verifier.StartSession(ctx, "jellyfin", rev.Revision)

	select {
	case <-sess.Done:
		if !sess.RolledBack {
			t.Errorf("expected session to trigger rollback on crash loop, got %v", sess.RolledBack)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for crash loop rollback")
	}

	st, err := mgr.GetService(ctx, "jellyfin")
	if err != nil || st.ReassuranceMessage != ReassuranceRollbackRestored {
		t.Errorf("expected reassurance message on crash loop rollback, got %+v", st)
	}
}

func TestMaintenanceScheduler_3AMWindow(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  engine,
	})
	defer mgr.Close()

	cfg := MaintenanceConfig{
		Hour:          3,
		Minute:        0,
		Enabled:       true,
		CheckInterval: 50 * time.Millisecond,
	}

	sched := NewMaintenanceScheduler(mgr, cfg, nil)
	if !sched.IsMaintenanceWindow(time.Date(2026, 10, 17, 3, 0, 15, 0, time.UTC)) {
		t.Errorf("expected true for 3:00 AM maintenance window")
	}
	if sched.IsMaintenanceWindow(time.Date(2026, 10, 17, 3, 1, 0, 0, time.UTC)) {
		t.Errorf("expected false for 3:01 AM")
	}
	if sched.IsMaintenanceWindow(time.Date(2026, 10, 17, 4, 0, 0, 0, time.UTC)) {
		t.Errorf("expected false for 4:00 AM")
	}
}

func TestUpgradeService_StartsVerificationSession(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		Engine:        engine,
		RevisionStore: db,
	})
	defer mgr.Close()

	ctx := context.Background()
	_, err = mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	rev, err := mgr.UpgradeService(ctx, "jellyfin", "12.3.0")
	if err != nil {
		t.Fatalf("UpgradeService failed: %v", err)
	}

	ve := mgr.VerificationEngine()
	if ve == nil {
		t.Fatalf("expected non-nil VerificationEngine")
	}
	sess := ve.GetSession("jellyfin")
	if sess == nil {
		t.Fatalf("expected active verification session after UpgradeService")
	}
	if sess.Revision != rev.Revision {
		t.Errorf("expected session revision %d, got %d", rev.Revision, sess.Revision)
	}
	ve.StopSession("jellyfin")
}

func TestRollbackService_NoPriorRevision(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		RevisionStore: db,
	})
	defer mgr.Close()

	_, err = mgr.RollbackService(context.Background(), "jellyfin", 0)
	if !errors.Is(err, ErrNoPriorRevision) {
		t.Errorf("expected ErrNoPriorRevision, got: %v", err)
	}
}

