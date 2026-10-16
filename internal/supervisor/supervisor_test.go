package supervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setupTestManager(t *testing.T, customTime *time.Time) (*ServiceManager, *MockRuntimeEngine, string, *[]string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "sovereign-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	engine := NewMockRuntimeEngine()
	var events []string
	var mu sync.Mutex

	broadcaster := func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	timeFunc := time.Now
	if customTime != nil {
		timeFunc = func() time.Time { return *customTime }
	}

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  engine,
		WatchdogConfig: WatchdogConfig{
			MinFailures: 3,
			MinWindow:   60 * time.Second,
			Interval:    100 * time.Millisecond,
			TimeFunc:    timeFunc,
		},
		EventBroadcaster: broadcaster,
	})

	return mgr, engine, tempDir, &events
}

func TestSupervisor_InitialListServices_InactiveAndNotDownloaded(t *testing.T) {
	mgr, _, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()
	services, err := mgr.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}

	if len(services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(services))
	}

	expectedIDs := []string{ServiceJellyfin, ServiceNavidrome, ServiceAudiobookshelf, ServicePaperless}
	for i, id := range expectedIDs {
		svc := services[i]
		if svc.ID != id {
			t.Errorf("expected service[%d] to be %s, got %s", i, id, svc.ID)
		}
		if svc.Status != StatusInactive {
			t.Errorf("expected %s status to be inactive, got %s", id, svc.Status)
		}
		if svc.ImageDownloaded {
			t.Errorf("expected %s image_downloaded to be false initially", id)
		}
		if svc.Health != HealthUnknown {
			t.Errorf("expected %s health to be unknown initially, got %s", id, svc.Health)
		}
	}
}

func TestSupervisor_EnforceMemoryBounds(t *testing.T) {
	mgr, engine, tempDir, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()

	tests := []struct {
		id              string
		expectedLimit   int64
		expectedRes     int64
		expectedImage   string
		expectedPort    int
		expectedSubpath string
	}{
		{
			id:              ServiceJellyfin,
			expectedLimit:   MemLimitJellyfin,       // 2048 MB
			expectedRes:     MemReservationJellyfin, // 1024 MB
			expectedImage:   ImageJellyfin,
			expectedPort:    8096,
			expectedSubpath: "/media",
		},
		{
			id:              ServiceNavidrome,
			expectedLimit:   MemLimitNavidrome,       // 512 MB
			expectedRes:     MemReservationNavidrome, // 256 MB
			expectedImage:   ImageNavidrome,
			expectedPort:    4533,
			expectedSubpath: "/music",
		},
		{
			id:              ServiceAudiobookshelf,
			expectedLimit:   MemLimitAudiobookshelf,       // 512 MB
			expectedRes:     MemReservationAudiobookshelf, // 256 MB
			expectedImage:   ImageAudiobookshelf,
			expectedPort:    13378,
			expectedSubpath: "/audiobooks",
		},
		{
			id:              ServicePaperless,
			expectedLimit:   MemLimitPaperless,       // 1024 MB
			expectedRes:     MemReservationPaperless, // 512 MB
			expectedImage:   ImagePaperless,
			expectedPort:    8000,
			expectedSubpath: "/docs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			svc, err := mgr.ActivateService(ctx, tc.id)
			if err != nil {
				t.Fatalf("failed to activate service %s: %v", tc.id, err)
			}

			if svc.MemLimitBytes != tc.expectedLimit {
				t.Errorf("expected %s MemLimitBytes=%d, got %d", tc.id, tc.expectedLimit, svc.MemLimitBytes)
			}
			if svc.MemReservationBytes != tc.expectedRes {
				t.Errorf("expected %s MemReservationBytes=%d, got %d", tc.id, tc.expectedRes, svc.MemReservationBytes)
			}
			if svc.Image != tc.expectedImage {
				t.Errorf("expected %s image=%s, got %s", tc.id, tc.expectedImage, svc.Image)
			}
			if svc.Port != tc.expectedPort {
				t.Errorf("expected %s port=%d, got %d", tc.id, tc.expectedPort, svc.Port)
			}
			if svc.Subpath != tc.expectedSubpath {
				t.Errorf("expected %s subpath=%s, got %s", tc.id, tc.expectedSubpath, svc.Subpath)
			}

			// Verify container spec received by engine
			containerInfo, err := engine.InspectContainer(ctx, tc.id)
			if err != nil {
				t.Fatalf("engine missing container %s: %v", tc.id, err)
			}
			if containerInfo.Spec.MemoryLimitBytes != tc.expectedLimit {
				t.Errorf("container spec limit mismatch: expected %d, got %d", tc.expectedLimit, containerInfo.Spec.MemoryLimitBytes)
			}
			if containerInfo.Spec.MemoryReservationBytes != tc.expectedRes {
				t.Errorf("container spec reservation mismatch: expected %d, got %d", tc.expectedRes, containerInfo.Spec.MemoryReservationBytes)
			}
			if tc.id == ServiceJellyfin {
				if containerInfo.Spec.UDPPorts[7359] != 7359 || containerInfo.Spec.UDPPorts[1900] != 1900 {
					t.Errorf("container spec missing UDP ports for Jellyfin: %v", containerInfo.Spec.UDPPorts)
				}
			}

			// Verify synthesized Compose YAML on disk
			composeFile := filepath.Join(tempDir, "state", "compose", tc.id+".yaml")
			content, err := os.ReadFile(composeFile)
			if err != nil {
				t.Fatalf("failed to read synthesized compose file %s: %v", composeFile, err)
			}
			if len(content) == 0 {
				t.Errorf("compose file %s is empty", composeFile)
			}
		})
	}
}

func TestSupervisor_OnDemandActivation_ProgressAndFailure(t *testing.T) {
	mgr, engine, _, events := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()

	// 1. Success path with progress streaming
	svc, err := mgr.ActivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	if svc.Status != StatusRunning {
		t.Errorf("expected status running, got %s", svc.Status)
	}
	if !svc.ImageDownloaded {
		t.Errorf("expected image_downloaded true after activation")
	}

	// Verify events were emitted
	hasProgress := false
	hasHealthChanged := false
	for _, evt := range *events {
		if evt == "service.pull_progress" {
			hasProgress = true
		}
		if evt == "service.health_changed" {
			hasHealthChanged = true
		}
	}
	if !hasProgress {
		t.Errorf("expected service.pull_progress event to be emitted during activation")
	}
	if !hasHealthChanged {
		t.Errorf("expected service.health_changed event to be emitted after activation")
	}

	// 2. Failure path: simulate network unreachable error
	engine.SetImagePresent(ImageNavidrome, false)
	engine.SetPullFailure(ImageNavidrome, errors.New("network unreachable: connection timeout"))

	failSvc, err := mgr.ActivateService(ctx, ServiceNavidrome)
	if err == nil {
		t.Fatalf("expected error on pull failure, got nil")
	}
	if !errors.Is(err, ErrPullFailed) {
		t.Errorf("expected ErrPullFailed, got %v", err)
	}
	if failSvc != nil {
		t.Errorf("expected nil service on failure")
	}

	// Verify service state reverted to inactive
	current, err := mgr.GetService(ctx, ServiceNavidrome)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if current.Status != StatusInactive {
		t.Errorf("expected status to revert to inactive on pull failure, got %s", current.Status)
	}
	if current.ImageDownloaded {
		t.Errorf("expected image_downloaded to remain false on pull failure")
	}

	hasErrorEvt := false
	for _, evt := range *events {
		if evt == "service.error" {
			hasErrorEvt = true
			break
		}
	}
	if !hasErrorEvt {
		t.Errorf("expected service.error event to be emitted on pull failure")
	}
}

func TestSupervisor_VolumePreservation_DeactivationAndRestart(t *testing.T) {
	mgr, _, tempDir, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()

	// 1. Activate Jellyfin
	_, err := mgr.ActivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}

	// 2. Create simulated user media and configuration files under /data
	mediaFile := filepath.Join(tempDir, "media", "movies", "BigBuckBunny.mp4")
	configFile := filepath.Join(tempDir, "state", "services", "jellyfin", "config", "system.xml")

	if err := os.WriteFile(mediaFile, []byte("fake-video-payload-1080p"), 0644); err != nil {
		t.Fatalf("failed to write media file: %v", err)
	}
	if err := os.WriteFile(configFile, []byte("<ServerConfiguration><Name>Sovereign</Name></ServerConfiguration>"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// 3. Deactivate Jellyfin
	deactSvc, err := mgr.DeactivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("DeactivateService failed: %v", err)
	}
	if deactSvc.Status != StatusInactive {
		t.Errorf("expected status inactive after deactivation, got %s", deactSvc.Status)
	}

	// 4. Assert all files under /data/media and /data/state are strictly preserved!
	if _, err := os.Stat(mediaFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: media file %s was deleted during deactivation!", mediaFile)
	}
	mediaContent, _ := os.ReadFile(mediaFile)
	if string(mediaContent) != "fake-video-payload-1080p" {
		t.Errorf("media file content modified: %s", string(mediaContent))
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: config file %s was deleted during deactivation!", configFile)
	}
	configContent, _ := os.ReadFile(configFile)
	if string(configContent) != "<ServerConfiguration><Name>Sovereign</Name></ServerConfiguration>" {
		t.Errorf("config file content modified: %s", string(configContent))
	}

	// 5. Reactivate Jellyfin and assert volumes still intact
	reactivateSvc, err := mgr.ActivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}
	if reactivateSvc.Status != StatusRunning {
		t.Errorf("expected status running after reactivate, got %s", reactivateSvc.Status)
	}

	// 6. Restart running Jellyfin and assert volumes still intact
	restartSvc, err := mgr.RestartService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("RestartService failed: %v", err)
	}
	if restartSvc.Status != StatusRunning {
		t.Errorf("expected status running after restart, got %s", restartSvc.Status)
	}

	if _, err := os.Stat(mediaFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: media file %s was deleted during restart!", mediaFile)
	}
}

func TestSupervisor_WatchdogDebouncing_TransientBlipAndSustainedFailure(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	debouncer := NewWatchdogDebouncer(WatchdogConfig{
		MinFailures: 3,
		MinWindow:   60 * time.Second,
		Interval:    10 * time.Second,
		TimeFunc:    func() time.Time { return now },
	})

	serviceID := ServiceJellyfin

	// Case 1: First failure (T = 0s) -> transient blip, no state change, no restart
	changed, health, status, shouldRestart := debouncer.RecordProbe(serviceID, false)
	if changed {
		t.Errorf("expected no health change on 1st probe failure")
	}
	if health != HealthHealthy || status != StatusRunning {
		t.Errorf("expected health=healthy, status=running on transient blip, got health=%s, status=%s", health, status)
	}
	if shouldRestart {
		t.Errorf("expected shouldRestart=false on 1st failure")
	}

	// Case 2: Second failure (T = 15s) -> still transient blip, no state change
	now = now.Add(15 * time.Second)
	changed, health, status, shouldRestart = debouncer.RecordProbe(serviceID, false)
	if changed {
		t.Errorf("expected no health change on 2nd probe failure")
	}
	if health != HealthHealthy || status != StatusRunning {
		t.Errorf("expected health=healthy, status=running on 2nd blip, got health=%s, status=%s", health, status)
	}
	if shouldRestart {
		t.Errorf("expected shouldRestart=false on 2nd failure")
	}

	// Case 3: Third failure but elapsed time is only 30s (< 60s min window) -> no state change yet!
	now = now.Add(15 * time.Second) // total elapsed = 30s
	changed, health, status, shouldRestart = debouncer.RecordProbe(serviceID, false)
	if changed {
		t.Errorf("expected no health change on 3rd failure because window < 60s")
	}
	if health != HealthHealthy || status != StatusRunning {
		t.Errorf("expected health=healthy before 60s elapsed, got health=%s, status=%s", health, status)
	}
	if shouldRestart {
		t.Errorf("expected shouldRestart=false because 60s window not satisfied")
	}

	// Case 4: Fourth failure with elapsed time = 65s (count >= 3 AND elapsed >= 60s)
	now = now.Add(35 * time.Second) // total elapsed = 65s
	changed, health, status, shouldRestart = debouncer.RecordProbe(serviceID, false)
	if !changed {
		t.Errorf("expected health change on sustained failure (>=3 failures AND >=60s)")
	}
	if health != HealthDegraded || status != StatusDegraded {
		t.Errorf("expected health=degraded, status=degraded on sustained failure, got health=%s, status=%s", health, status)
	}
	if !shouldRestart {
		t.Errorf("expected shouldRestart=true on sustained failure to trigger recovery")
	}

	// Case 5: Probe recovers to healthy!
	now = now.Add(10 * time.Second)
	changed, health, status, shouldRestart = debouncer.RecordProbe(serviceID, true)
	if !changed {
		t.Errorf("expected health change when recovering to healthy")
	}
	if health != HealthHealthy || status != StatusRunning {
		t.Errorf("expected health=healthy, status=running on recovery, got health=%s, status=%s", health, status)
	}
	if shouldRestart {
		t.Errorf("expected shouldRestart=false on healthy recovery")
	}

	// Verify state reset
	st := debouncer.GetState(serviceID)
	if st.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive failures reset to 0, got %d", st.ConsecutiveFailures)
	}
}

func TestSupervisor_UnknownServiceErrors(t *testing.T) {
	mgr, _, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()

	if _, err := mgr.GetService(ctx, "nonexistent"); !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound for GetService, got %v", err)
	}
	if _, err := mgr.ActivateService(ctx, "nonexistent"); !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound for ActivateService, got %v", err)
	}
	if _, err := mgr.DeactivateService(ctx, "nonexistent"); !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound for DeactivateService, got %v", err)
	}
	if _, err := mgr.RestartService(ctx, "nonexistent"); !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound for RestartService, got %v", err)
	}
}

func TestSupervisor_RestartService_InactiveServiceRejected(t *testing.T) {
	mgr, _, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()
	_, err := mgr.RestartService(ctx, ServiceNavidrome)
	if !errors.Is(err, ErrInactiveService) {
		t.Fatalf("expected ErrInactiveService, got %v", err)
	}
}

func TestSupervisor_ActivateService_InFlightDuplicateGuard(t *testing.T) {
	mgr, engine, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	ctx := context.Background()
	slowPullStarted := make(chan struct{})
	pullRelease := make(chan struct{})

	engine.OnPullImage = func(ctx context.Context, image string, progressCb func(percent float64, status string)) error {
		close(slowPullStarted)
		<-pullRelease
		return nil
	}

	go func() {
		_, _ = mgr.ActivateService(ctx, ServiceNavidrome)
	}()

	<-slowPullStarted

	// Attempt second concurrent activation
	_, err := mgr.ActivateService(ctx, ServiceNavidrome)
	if err == nil {
		t.Fatalf("expected error on concurrent activation, got nil")
	}

	close(pullRelease)
}

func TestSupervisor_ServiceManager_WatchdogProbingAndRecovery(t *testing.T) {
	currTime := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	var timeMu sync.Mutex
	timeFunc := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return currTime
	}
	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		defer timeMu.Unlock()
		currTime = currTime.Add(d)
	}

	tempDir := t.TempDir()
	engine := NewMockRuntimeEngine()
	var events []string
	var eventsMu sync.Mutex
	broadcaster := func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  engine,
		WatchdogConfig: WatchdogConfig{
			MinFailures: 3,
			MinWindow:   60 * time.Second,
			Interval:    10 * time.Millisecond,
			TimeFunc:    timeFunc,
		},
		EventBroadcaster: broadcaster,
	})
	defer mgr.Close()

	ctx := context.Background()

	// 1. Activate Jellyfin
	_, err := mgr.ActivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("failed to activate service: %v", err)
	}

	// 2. Simulate container becoming unhealthy
	engine.SetContainerHealth(ServiceJellyfin, false)

	// Probe 1 (T = 0s)
	mgr.probeActiveServices(ctx)
	svc, _ := mgr.GetService(ctx, ServiceJellyfin)
	if svc.Health != HealthHealthy {
		t.Errorf("expected health healthy on 1st probe failure, got %s", svc.Health)
	}

	// Probe 2 (T = 20s)
	advanceTime(20 * time.Second)
	mgr.probeActiveServices(ctx)
	svc, _ = mgr.GetService(ctx, ServiceJellyfin)
	if svc.Health != HealthHealthy {
		t.Errorf("expected health healthy on 2nd probe failure, got %s", svc.Health)
	}

	// Probe 3 (T = 40s) -> count=3 but elapsed=40s (<60s)
	advanceTime(20 * time.Second)
	mgr.probeActiveServices(ctx)
	svc, _ = mgr.GetService(ctx, ServiceJellyfin)
	if svc.Health != HealthHealthy {
		t.Errorf("expected health healthy on 3rd failure because elapsed < 60s, got %s", svc.Health)
	}

	// Probe 4 (T = 65s) -> count >= 3 AND elapsed >= 60s -> transitions to degraded and triggers restart!
	advanceTime(25 * time.Second) // total elapsed = 65s
	mgr.probeActiveServices(ctx)

	svc, _ = mgr.GetService(ctx, ServiceJellyfin)
	if svc.Health != HealthDegraded {
		t.Errorf("expected health degraded after sustained failure, got %s", svc.Health)
	}
	if svc.Status != StatusDegraded {
		t.Errorf("expected status degraded after sustained failure, got %s", svc.Status)
	}

	// Wait briefly for the goroutine restart to complete
	time.Sleep(50 * time.Millisecond)

	// Verify restart was called on mock engine
	engine.mu.RLock()
	restartedHistory := engine.restartedHistory
	engine.mu.RUnlock()

	restarted := false
	for _, name := range restartedHistory {
		if name == ServiceJellyfin || name == "sovereign-"+ServiceJellyfin {
			restarted = true
			break
		}
	}
	if !restarted {
		t.Errorf("expected recovery restart to be called on engine")
	}

	// Verify debouncer was reset on recovery restart
	st := mgr.debouncer.GetState(ServiceJellyfin)
	if st.ConsecutiveFailures != 0 {
		t.Errorf("expected debouncer failures to be reset to 0 after recovery restart, got %d", st.ConsecutiveFailures)
	}

	// Verify service.health_changed event was emitted
	eventsMu.Lock()
	healthChangedEmitted := false
	for _, e := range events {
		if e == "service.health_changed" {
			healthChangedEmitted = true
			break
		}
	}
	eventsMu.Unlock()
	if !healthChangedEmitted {
		t.Errorf("expected service.health_changed event to be emitted")
	}
}

func TestSupervisor_ResolveID(t *testing.T) {
	mgr, _, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	if resolved := mgr.resolveID("video"); resolved != ServiceJellyfin {
		t.Errorf("expected resolveID(\"video\") == %s, got %s", ServiceJellyfin, resolved)
	}
	if resolved := mgr.resolveID("audio"); resolved != ServiceNavidrome {
		t.Errorf("expected resolveID(\"audio\") == %s, got %s", ServiceNavidrome, resolved)
	}
	if resolved := mgr.resolveID("jellyfin"); resolved != ServiceJellyfin {
		t.Errorf("expected resolveID(\"jellyfin\") == %s, got %s", ServiceJellyfin, resolved)
	}
	if resolved := mgr.resolveID("custom"); resolved != "custom" {
		t.Errorf("expected resolveID(\"custom\") == \"custom\", got %s", resolved)
	}

	// Verify GetService accepts alias "video"
	ctx := context.Background()
	svc, err := mgr.GetService(ctx, "video")
	if err != nil {
		t.Fatalf("GetService(\"video\") failed: %v", err)
	}
	if svc.ID != ServiceJellyfin {
		t.Errorf("expected service ID %s, got %s", ServiceJellyfin, svc.ID)
	}
}
