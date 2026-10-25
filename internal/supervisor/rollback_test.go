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

func TestRollback_CircuitBreaker(t *testing.T) {
	tempDir := t.TempDir()
	db, err := store.Open(filepath.Join(tempDir, "state", "sovereign.db"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()
	ctx := context.Background()

	var eventsMu sync.Mutex
	type eventRecord struct {
		Event string
		Data  any
	}
	var events []eventRecord
	broadcaster := func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, eventRecord{Event: event, Data: data})
	}

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:          tempDir,
		Engine:           engine,
		RevisionStore:    db,
		EventBroadcaster: broadcaster,
	})
	defer mgr.Close()

	// 1. Initial activation (Rev 1)
	_, err = mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	// 2. Upgrade service (Rev 2)
	rev2, err := mgr.UpgradeService(ctx, "jellyfin", "jellyfin/jellyfin:12.3.0")
	if err != nil {
		t.Fatalf("UpgradeService failed: %v", err)
	}

	// First automated rollback attempt (attempt 1 <= 2)
	st1, err := mgr.RollbackService(ctx, "jellyfin", rev2.Revision)
	if err != nil {
		t.Fatalf("RollbackService attempt 1 failed: %v", err)
	}
	if st1.Status != StatusDegraded {
		t.Errorf("expected status 'degraded' after attempt 1, got %q", st1.Status)
	}
	if attempts := mgr.GetRollbackAttempts("jellyfin", rev2.Revision); attempts != 1 {
		t.Errorf("expected 1 rollback attempt recorded, got %d", attempts)
	}

	// Second automated rollback attempt (attempt 2 <= 2)
	st2, err := mgr.RollbackService(ctx, "jellyfin", rev2.Revision)
	if err != nil {
		t.Fatalf("RollbackService attempt 2 failed: %v", err)
	}
	if st2.Status != StatusDegraded {
		t.Errorf("expected status 'degraded' after attempt 2, got %q", st2.Status)
	}
	if attempts := mgr.GetRollbackAttempts("jellyfin", rev2.Revision); attempts != 2 {
		t.Errorf("expected 2 rollback attempts recorded, got %d", attempts)
	}

	// Third automated rollback attempt: circuit breaker trips (attempts >= 2)
	st3, err := mgr.RollbackService(ctx, "jellyfin", rev2.Revision)
	if err != nil {
		t.Fatalf("RollbackService attempt 3 returned unexpected error: %v", err)
	}

	if st3.Status != StatusSafeRecovery {
		t.Errorf("expected status 'safe_recovery' when circuit breaker trips, got %q", st3.Status)
	}
	if st3.Health != HealthDegraded {
		t.Errorf("expected health 'degraded', got %q", st3.Health)
	}
	if st3.ReassuranceMessage != ReassuranceSafeRecovery {
		t.Errorf("expected reassurance message %q, got %q", ReassuranceSafeRecovery, st3.ReassuranceMessage)
	}

	// Verify container is kept stopped
	info, err := engine.InspectContainer(ctx, "jellyfin")
	if err == nil && info != nil && info.Running {
		t.Errorf("expected container to be stopped in safe recovery mode, got running")
	}

	// Verify emergency.rollback_loop SSE event was broadcast
	eventsMu.Lock()
	foundEmergencyEvent := false
	for _, e := range events {
		if e.Event == "emergency.rollback_loop" {
			foundEmergencyEvent = true
			if payload, ok := e.Data.(map[string]any); ok {
				if payload["service_id"] != "jellyfin" {
					t.Errorf("expected emergency event service_id 'jellyfin', got %v", payload["service_id"])
				}
				if payload["status"] != StatusSafeRecovery {
					t.Errorf("expected emergency event status 'safe_recovery', got %v", payload["status"])
				}
			}
		}
	}
	eventsMu.Unlock()

	if !foundEmergencyEvent {
		t.Errorf("expected emergency.rollback_loop SSE event to be broadcast")
	}

	// 4. Test manual rollback resets attempt counter
	_, err = mgr.ManualRollbackService(ctx, "jellyfin", rev2.Revision)
	if err != nil {
		t.Fatalf("ManualRollbackService failed: %v", err)
	}
	if attempts := mgr.GetRollbackAttempts("jellyfin", rev2.Revision); attempts != 0 {
		t.Errorf("expected manual rollback to reset attempts counter to 0, got %d", attempts)
	}
}

