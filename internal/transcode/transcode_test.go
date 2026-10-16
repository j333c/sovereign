package transcode

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, Store) {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test_transcode.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}

	store := NewSQLiteStore(db)
	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db, store
}

func createTestVideoFile(t *testing.T, dir, filename string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte("MOCK_RAW_MKV_BYTES_FOR_TESTING"), 0644); err != nil {
		t.Fatalf("failed to write test video file: %v", err)
	}
	return p
}

type mockRefresher struct {
	mu           sync.Mutex
	refreshedSvc []string
}

func (m *mockRefresher) RefreshService(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshedSvc = append(m.refreshedSvc, serviceID)
	return nil
}

func (m *mockRefresher) WasRefreshed(serviceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.refreshedSvc {
		if s == serviceID {
			return true
		}
	}
	return false
}

func TestScheduler_IsOffPeak(t *testing.T) {
	cfg := DefaultScheduleConfig() // 01:00 to 07:00
	sched, err := NewScheduler(cfg)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	tests := []struct {
		name     string
		hour     int
		minute   int
		expected bool
	}{
		{"exact start 01:00", 1, 0, true},
		{"mid night 03:30", 3, 30, true},
		{"near end 06:59", 6, 59, true},
		{"exact end 07:00", 7, 0, false},
		{"morning 09:15", 9, 15, false},
		{"noon 12:00", 12, 0, false},
		{"evening 20:45", 20, 45, false},
		{"midnight 00:00", 0, 0, false},
		{"just before start 00:59", 0, 59, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 16, tc.hour, tc.minute, 0, 0, time.Local)
			got := sched.IsOffPeak(now)
			if got != tc.expected {
				t.Errorf("at %02d:%02d: expected IsOffPeak=%v, got %v", tc.hour, tc.minute, tc.expected, got)
			}
		})
	}
}

func TestScheduler_MidnightCrossing(t *testing.T) {
	// Schedule: 23:00 to 06:00
	cfg := ScheduleConfig{
		StartHour:   23,
		StartMinute: 0,
		EndHour:     6,
		EndMinute:   0,
	}
	sched, err := NewScheduler(cfg)
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	tests := []struct {
		hour     int
		minute   int
		expected bool
	}{
		{23, 0, true},
		{23, 45, true},
		{0, 15, true},
		{3, 0, true},
		{5, 59, true},
		{6, 0, false},
		{12, 0, false},
		{22, 59, false},
	}

	for _, tc := range tests {
		now := time.Date(2026, 10, 16, tc.hour, tc.minute, 0, 0, time.Local)
		got := sched.IsOffPeak(now)
		if got != tc.expected {
			t.Errorf("at %02d:%02d: expected %v, got %v", tc.hour, tc.minute, tc.expected, got)
		}
	}
}

func TestScheduler_Validation(t *testing.T) {
	badConfigs := []ScheduleConfig{
		{StartHour: -1, StartMinute: 0, EndHour: 7, EndMinute: 0},
		{StartHour: 24, StartMinute: 0, EndHour: 7, EndMinute: 0},
		{StartHour: 1, StartMinute: -1, EndHour: 7, EndMinute: 0},
		{StartHour: 1, StartMinute: 60, EndHour: 7, EndMinute: 0},
		{StartHour: 1, StartMinute: 0, EndHour: 25, EndMinute: 0},
		{StartHour: 1, StartMinute: 0, EndHour: 7, EndMinute: 65},
	}

	for i, c := range badConfigs {
		_, err := NewScheduler(c)
		if err == nil {
			t.Errorf("case %d: expected error for invalid config %+v, got nil", i, c)
		}
	}
}

func TestScheduler_ManualOverrideAndGPUStatus(t *testing.T) {
	sched, _ := NewScheduler(DefaultScheduleConfig()) // 01:00-07:00

	daytime := time.Date(2026, 10, 16, 14, 0, 0, 0, time.Local)
	nighttime := time.Date(2026, 10, 16, 3, 0, 0, 0, time.Local)

	// 1. Daytime without override
	if sched.IsActive(daytime) {
		t.Errorf("expected IsActive=false during daytime without override")
	}
	if status := sched.GetGPUStatus(daytime); status != GPUStatusDaytime {
		t.Errorf("expected GPUStatus=%q, got %q", GPUStatusDaytime, status)
	}

	// 2. Daytime WITH manual override
	sched.SetManualOverride(true)
	if !sched.IsActive(daytime) {
		t.Errorf("expected IsActive=true when manual override is enabled")
	}
	if status := sched.GetGPUStatus(daytime); status != GPUStatusOffPeak {
		t.Errorf("expected GPUStatus=%q under override, got %q", GPUStatusOffPeak, status)
	}

	// 3. Clear manual override
	sched.SetManualOverride(false)
	if sched.IsActive(daytime) {
		t.Errorf("expected IsActive=false after clearing override")
	}

	// 4. Nighttime without override is active
	if !sched.IsActive(nighttime) {
		t.Errorf("expected IsActive=true during off-peak window")
	}
	if status := sched.GetGPUStatus(nighttime); status != GPUStatusOffPeak {
		t.Errorf("expected GPUStatus=%q at night, got %q", GPUStatusOffPeak, status)
	}
}

func TestSQLiteStore_CRUD(t *testing.T) {
	_, store := setupTestDB(t)
	ctx := context.Background()

	// 1. Enqueue job
	job := &Job{
		SourcePath: "/data/media/staging/avatar.mkv",
		TargetPath: "/data/media/movies/Avatar/avatar.mp4",
		Profile:    "universal",
	}
	if err := store.Enqueue(ctx, job); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if job.ID == "" {
		t.Fatalf("expected job.ID to be generated, got empty")
	}
	if job.Status != StatusQueued {
		t.Fatalf("expected status %q, got %q", StatusQueued, job.Status)
	}

	// 2. GetJob
	fetched, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if fetched.SourcePath != job.SourcePath {
		t.Errorf("expected source %q, got %q", job.SourcePath, fetched.SourcePath)
	}

	// 3. GetNextQueuedJob
	next, err := store.GetNextQueuedJob(ctx)
	if err != nil {
		t.Fatalf("GetNextQueuedJob failed: %v", err)
	}
	if next == nil || next.ID != job.ID {
		t.Fatalf("expected next job ID %q, got %+v", job.ID, next)
	}

	// 4. UpdateProgress
	if err := store.UpdateProgress(ctx, job.ID, 45.5); err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}
	fetched, _ = store.GetJob(ctx, job.ID)
	if fetched.Progress != 45.5 {
		t.Errorf("expected progress 45.5, got %f", fetched.Progress)
	}

	// 5. MarkStarted
	if err := store.MarkStarted(ctx, job.ID, "qsv"); err != nil {
		t.Fatalf("MarkStarted failed: %v", err)
	}
	fetched, _ = store.GetJob(ctx, job.ID)
	if fetched.Status != StatusProcessing {
		t.Errorf("expected status %q, got %q", StatusProcessing, fetched.Status)
	}
	if fetched.EncoderUsed != "qsv" {
		t.Errorf("expected encoder 'qsv', got %q", fetched.EncoderUsed)
	}
	if fetched.StartedAt == nil {
		t.Errorf("expected started_at timestamp to be recorded")
	}

	// 6. ResetProcessingJobs
	if err := store.ResetProcessingJobs(ctx); err != nil {
		t.Fatalf("ResetProcessingJobs failed: %v", err)
	}
	fetched, _ = store.GetJob(ctx, job.ID)
	if fetched.Status != StatusQueued {
		t.Errorf("expected status reset to %q, got %q", StatusQueued, fetched.Status)
	}

	// 7. MarkCompleted
	if err := store.MarkCompleted(ctx, job.ID, job.TargetPath, "qsv"); err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}
	fetched, _ = store.GetJob(ctx, job.ID)
	if fetched.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, fetched.Status)
	}
	if fetched.Progress != 100.0 {
		t.Errorf("expected progress 100, got %f", fetched.Progress)
	}
	if fetched.CompletedAt == nil {
		t.Errorf("expected completed_at timestamp")
	}

	// 8. MarkFailed
	job2 := &Job{
		SourcePath: "/data/media/staging/bad.mkv",
		TargetPath: "/data/media/movies/bad.mp4",
	}
	_ = store.Enqueue(ctx, job2)
	if err := store.MarkFailed(ctx, job2.ID, "codec unsupported"); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	fetched2, _ := store.GetJob(ctx, job2.ID)
	if fetched2.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, fetched2.Status)
	}
	if fetched2.ErrorMessage != "codec unsupported" {
		t.Errorf("expected error message 'codec unsupported', got %q", fetched2.ErrorMessage)
	}

	// 9. ListJobs
	list, err := store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(list))
	}
}

func TestTranscodeEngine_DaytimeRestriction(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	sched, _ := NewScheduler(DefaultScheduleConfig()) // 01:00-07:00
	sched.SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 14, 0, 0, 0, time.Local) // 14:00 daytime
	})

	mockTC := NewMockTranscoder()
	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
	})
	if err != nil {
		t.Fatalf("NewTranscodeEngine failed: %v", err)
	}
	defer engine.Close()

	srcFile := createTestVideoFile(t, tempDir, "daytime_video.mkv")
	job, err := engine.Enqueue(context.Background(), EnqueueRequest{
		SourcePath: srcFile,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// ProcessOnce during daytime should return ErrDaytimeRestricted and NOT invoke transcoder
	err = engine.ProcessOnce(context.Background())
	if !errors.Is(err, ErrDaytimeRestricted) {
		t.Fatalf("expected ErrDaytimeRestricted, got: %v", err)
	}

	if mockTC.TranscodeCalls != 0 {
		t.Errorf("transcoder was invoked %d times during daytime without override", mockTC.TranscodeCalls)
	}

	fetched, _ := store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusQueued {
		t.Errorf("expected job to remain queued, got %q", fetched.Status)
	}

	// Now activate manual override: job should process immediately!
	engine.SetManualOverride(true)
	err = engine.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("expected ProcessOnce to succeed with manual override, got: %v", err)
	}

	if mockTC.TranscodeCalls != 1 {
		t.Errorf("expected 1 transcode call after override, got %d", mockTC.TranscodeCalls)
	}

	fetched, _ = store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusCompleted {
		t.Errorf("expected job completed, got %q", fetched.Status)
	}
}

func TestTranscodeEngine_NightWindowArrival(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	sched, _ := NewScheduler(DefaultScheduleConfig())
	sched.SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 2, 30, 0, 0, time.Local) // 02:30 night window
	})

	var eventsMu sync.Mutex
	var events []string
	broadcaster := func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}

	refresher := &mockRefresher{}
	mockTC := NewMockTranscoder()

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:     tempDir,
		Store:       store,
		Scheduler:   sched,
		Transcoder:  mockTC,
		Refresher:   refresher,
		Broadcaster: broadcaster,
		HeadroomChecker: func(path string) (float64, error) {
			return 50.0, nil // 50% free headroom
		},
	})
	if err != nil {
		t.Fatalf("NewTranscodeEngine failed: %v", err)
	}
	defer engine.Close()

	srcFile := createTestVideoFile(t, tempDir, "night_movie.mkv")
	targetFile := filepath.Join(tempDir, "night_movie.mp4")

	job, err := engine.Enqueue(context.Background(), EnqueueRequest{
		SourcePath: srcFile,
		TargetPath: targetFile,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	err = engine.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce failed at night: %v", err)
	}

	// Check output exists
	if _, err := os.Stat(targetFile); err != nil {
		t.Fatalf("expected target output file %q to exist: %v", targetFile, err)
	}

	// Check job status in store
	fetched, _ := store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, fetched.Status)
	}

	// Check library refresh dispatched to Jellyfin
	time.Sleep(50 * time.Millisecond) // Allow goroutine refresh to run
	if !refresher.WasRefreshed("jellyfin") {
		t.Errorf("expected Jellyfin library refresh to be dispatched upon transcode completion")
	}

	// Verify events emitted
	eventsMu.Lock()
	hasStarted := false
	hasProgress := false
	hasCompleted := false
	for _, ev := range events {
		if ev == "transcode.job_started" {
			hasStarted = true
		} else if ev == "transcode.job_progress" {
			hasProgress = true
		} else if ev == "transcode.job_completed" {
			hasCompleted = true
		}
	}
	eventsMu.Unlock()

	if !hasStarted || !hasProgress || !hasCompleted {
		t.Errorf("expected started, progress, completed events; got: %v", events)
	}
}

func TestTranscodeEngine_StorageHeadroomFloor(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	sched, _ := NewScheduler(DefaultScheduleConfig())
	sched.SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 2, 0, 0, 0, time.Local) // Night window
	})

	var diskAlertEmitted bool
	var eventsMu sync.Mutex
	broadcaster := func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		if event == "storage.disk_alert" {
			diskAlertEmitted = true
		}
	}

	mockTC := NewMockTranscoder()
	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:     tempDir,
		Store:       store,
		Scheduler:   sched,
		Transcoder:  mockTC,
		Broadcaster: broadcaster,
		HeadroomChecker: func(path string) (float64, error) {
			return 8.5, nil // Below 10% threshold!
		},
	})
	if err != nil {
		t.Fatalf("NewTranscodeEngine failed: %v", err)
	}
	defer engine.Close()

	srcFile := createTestVideoFile(t, tempDir, "headroom_test.mkv")
	job, err := engine.Enqueue(context.Background(), EnqueueRequest{
		SourcePath: srcFile,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	err = engine.ProcessOnce(context.Background())
	if !errors.Is(err, ErrStorageHeadroomLow) {
		t.Fatalf("expected ErrStorageHeadroomLow, got %v", err)
	}

	if mockTC.TranscodeCalls != 0 {
		t.Errorf("expected 0 transcode calls when storage is low, got %d", mockTC.TranscodeCalls)
	}

	eventsMu.Lock()
	alert := diskAlertEmitted
	eventsMu.Unlock()

	if !alert {
		t.Errorf("expected storage.disk_alert event to be emitted when headroom < 10%%")
	}

	// Status should be storage_low
	st := engine.GetStatus()
	if st.State != "storage_low" {
		t.Errorf("expected engine state 'storage_low', got %q", st.State)
	}

	// Now restore storage to 25% and retry
	engine.SetHeadroomChecker(func(path string) (float64, error) {
		return 25.0, nil
	})

	err = engine.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("expected ProcessOnce to succeed after storage restored, got %v", err)
	}

	fetched, _ := store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusCompleted {
		t.Errorf("expected job completed after storage restored, got %q", fetched.Status)
	}
}

func TestTranscodeEngine_HardwareAccelerationFallback(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	sched, _ := NewScheduler(DefaultScheduleConfig())
	sched.SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 3, 0, 0, 0, time.Local)
	})

	mockTC := NewMockTranscoder()
	mockTC.SetDetectedHardware(HardwareQSV)
	mockTC.FailHardwareOnce = true // First hardware transcode call fails with ErrHardwareUnavailable

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
		HeadroomChecker: func(path string) (float64, error) {
			return 50.0, nil
		},
	})
	if err != nil {
		t.Fatalf("NewTranscodeEngine failed: %v", err)
	}
	defer engine.Close()

	srcFile := createTestVideoFile(t, tempDir, "hw_fallback.mkv")
	job, err := engine.Enqueue(context.Background(), EnqueueRequest{
		SourcePath: srcFile,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	err = engine.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("expected transcode to succeed via software fallback, got: %v", err)
	}

	// Should have been called twice: 1st hardware attempt (failed), 2nd software fallback (succeeded)
	if mockTC.TranscodeCalls != 2 {
		t.Errorf("expected 2 transcode calls (hw + fallback), got %d", mockTC.TranscodeCalls)
	}

	fetched, _ := store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusCompleted {
		t.Errorf("expected job status completed, got %q", fetched.Status)
	}
	if fetched.EncoderUsed != "cpu-x264" {
		t.Errorf("expected encoder_used 'cpu-x264', got %q", fetched.EncoderUsed)
	}
}

func TestTranscodeEngine_DefinitiveFailure(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	sched, _ := NewScheduler(DefaultScheduleConfig())
	sched.SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 3, 0, 0, 0, time.Local)
	})

	mockTC := NewMockTranscoder()
	mockTC.AlwaysFail = true
	mockTC.FailErr = errors.New("corrupt container bitstream")

	var failedEmitted bool
	broadcaster := func(event string, data any) {
		if event == "transcode.job_failed" {
			failedEmitted = true
		}
	}

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:     tempDir,
		Store:       store,
		Scheduler:   sched,
		Transcoder:  mockTC,
		Broadcaster: broadcaster,
		HeadroomChecker: func(path string) (float64, error) {
			return 50.0, nil
		},
	})
	if err != nil {
		t.Fatalf("NewTranscodeEngine failed: %v", err)
	}
	defer engine.Close()

	srcFile := createTestVideoFile(t, tempDir, "corrupt.mkv")
	job, _ := engine.Enqueue(context.Background(), EnqueueRequest{
		SourcePath: srcFile,
	})

	err = engine.ProcessOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error from corrupt file, got nil")
	}

	fetched, _ := store.GetJob(context.Background(), job.ID)
	if fetched.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, fetched.Status)
	}
	if !failedEmitted {
		t.Errorf("expected transcode.job_failed event to be emitted")
	}
}

func TestTranscodeEngine_EnqueueValidation(t *testing.T) {
	_, store := setupTestDB(t)
	tempDir := t.TempDir()

	engine, _ := NewTranscodeEngine(EngineConfig{
		DataDir: tempDir,
		Store:   store,
	})
	defer engine.Close()

	// 1. Empty source path
	_, err := engine.Enqueue(context.Background(), EnqueueRequest{SourcePath: ""})
	if err == nil {
		t.Errorf("expected error for empty source path, got nil")
	}

	// 2. Non-existent source path
	_, err = engine.Enqueue(context.Background(), EnqueueRequest{SourcePath: filepath.Join(tempDir, "nonexistent.mkv")})
	if err == nil {
		t.Errorf("expected error for nonexistent source path, got nil")
	}

	// 3. Source path is directory
	_, err = engine.Enqueue(context.Background(), EnqueueRequest{SourcePath: tempDir})
	if err == nil {
		t.Errorf("expected error when source path is a directory, got nil")
	}
}
