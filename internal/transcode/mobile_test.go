package transcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMobileProfile_Definitions verifies standardized mobile profiles match requirements.
func TestMobileProfile_Definitions(t *testing.T) {
	// 1. mobile-720p
	p720, ok := GetMobileProfile("mobile-720p")
	if !ok {
		t.Fatalf("expected mobile-720p to be defined")
	}
	if p720.ScaleFilter != "scale=-2:'min(720,ih)'" {
		t.Errorf("expected scale filter \"scale=-2:'min(720,ih)'\", got %q", p720.ScaleFilter)
	}
	if p720.VAAPIScaleFilter != "scale_vaapi=w=-2:h=720" {
		t.Errorf("expected VAAPI filter 'scale_vaapi=w=-2:h=720', got %q", p720.VAAPIScaleFilter)
	}
	if p720.MaxRate != "3000k" || p720.BufSize != "5000k" {
		t.Errorf("expected maxrate=3000k, bufsize=5000k, got maxrate=%s, bufsize=%s", p720.MaxRate, p720.BufSize)
	}
	if p720.AudioCodec != "aac" || p720.AudioBitrate != "128k" || p720.AudioChannels != 2 {
		t.Errorf("expected aac 128k stereo, got %s %s %d channels", p720.AudioCodec, p720.AudioBitrate, p720.AudioChannels)
	}
	if !p720.FastStart || p720.Container != "mp4" {
		t.Errorf("expected faststart mp4 container")
	}

	// 2. mobile-1080p
	p1080, ok := GetMobileProfile("mobile-1080p")
	if !ok {
		t.Fatalf("expected mobile-1080p to be defined")
	}
	if p1080.ScaleFilter != "scale=-2:'min(1080,ih)'" {
		t.Errorf("expected scale filter \"scale=-2:'min(1080,ih)'\", got %q", p1080.ScaleFilter)
	}
	if p1080.VAAPIScaleFilter != "scale_vaapi=w=-2:h=1080" {
		t.Errorf("expected VAAPI filter 'scale_vaapi=w=-2:h=1080', got %q", p1080.VAAPIScaleFilter)
	}
	if p1080.MaxRate != "5500k" || p1080.BufSize != "9000k" {
		t.Errorf("expected maxrate=5500k, bufsize=9000k, got maxrate=%s, bufsize=%s", p1080.MaxRate, p1080.BufSize)
	}
	if p1080.AudioCodec != "aac" || p1080.AudioBitrate != "192k" || p1080.AudioChannels != 2 {
		t.Errorf("expected aac 192k stereo, got %s %s %d channels", p1080.AudioCodec, p1080.AudioBitrate, p1080.AudioChannels)
	}
	if !p1080.FastStart || p1080.Container != "mp4" {
		t.Errorf("expected faststart mp4 container")
	}

	// 3. Unknown profile
	if IsMobileProfile("4k") || IsMobileProfile("mobile-4k") || IsMobileProfile("universal") {
		t.Errorf("expected non-mobile profiles to return false for IsMobileProfile")
	}
}

// TestMobileProfile_MultiVersionNaming verifies Jellyfin multi-version naming format and stacked tag stripping.
func TestMobileProfile_MultiVersionNaming(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		profile    string
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "movie 720p",
			sourcePath: "/data/media/movies/Inception (2010)/Inception (2010).mkv",
			profile:    "mobile-720p",
			wantTarget: "/data/media/movies/Inception (2010)/Inception (2010) - 720p.mp4",
		},
		{
			name:       "movie 1080p",
			sourcePath: "/data/media/movies/Inception (2010)/Inception (2010).mkv",
			profile:    "mobile-1080p",
			wantTarget: "/data/media/movies/Inception (2010)/Inception (2010) - 1080p.mp4",
		},
		{
			name:       "tv episode 720p",
			sourcePath: "/data/media/tv/Breaking Bad/Season 01/Breaking Bad - S01E01.mkv",
			profile:    "mobile-720p",
			wantTarget: "/data/media/tv/Breaking Bad/Season 01/Breaking Bad - S01E01 - 720p.mp4",
		},
		{
			name:       "strip previous 1080p tag when generating 720p",
			sourcePath: "/data/media/movies/Gladiator (2000)/Gladiator (2000) - 1080p.mkv",
			profile:    "mobile-720p",
			wantTarget: "/data/media/movies/Gladiator (2000)/Gladiator (2000) - 720p.mp4",
		},
		{
			name:       "strip stacked quality tags iteratively",
			sourcePath: "/data/media/movies/Inception (2010)/Inception (2010) - 1080p - 720p.mkv",
			profile:    "mobile-720p",
			wantTarget: "/data/media/movies/Inception (2010)/Inception (2010) - 720p.mp4",
		},
		{
			name:       "strip case-insensitive direct-play and 1080P tags",
			sourcePath: "/data/media/movies/Avatar (2009)/Avatar (2009) - DIRECT-PLAY - 1080P.mkv",
			profile:    "mobile-1080p",
			wantTarget: "/data/media/movies/Avatar (2009)/Avatar (2009) - 1080p.mp4",
		},
		{
			name:       "strip 4k tag when generating 1080p",
			sourcePath: "/data/media/movies/Dune (2021)/Dune (2021) - 4k.mkv",
			profile:    "mobile-1080p",
			wantTarget: "/data/media/movies/Dune (2021)/Dune (2021) - 1080p.mp4",
		},
		{
			name:       "unknown profile",
			sourcePath: "/data/media/movies/Test/Test.mkv",
			profile:    "mobile-4k",
			wantErr:    true,
		},
		{
			name:       "empty source path",
			sourcePath: "",
			profile:    "mobile-720p",
			wantErr:    true,
		},
		{
			name:       "identical source and target path error",
			sourcePath: "/data/media/movies/Test/Test - 720p.mp4",
			profile:    "mobile-720p",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveMultiVersionPath(tt.sourcePath, tt.profile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveMultiVersionPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.wantTarget {
				t.Errorf("ResolveMultiVersionPath() = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}

// TestMobileProfile_FFmpegArgs verifies argument builder applies mobile resolution scaling and bounded bitrates across hardware encoders.
func TestMobileProfile_FFmpegArgs(t *testing.T) {
	transcoder := NewFFmpegTranscoder(nil, nil)

	// 1. mobile-720p with software CPU encoder (no conflicting -crf flag)
	args720, enc720 := transcoder.BuildFFmpegArgs("/media/master.mkv", "/media/master - 720p.mp4", "mobile-720p", HardwareSoftware)
	if enc720 != "cpu-x264" {
		t.Errorf("expected encoder cpu-x264, got %s", enc720)
	}

	argStr720 := strings.Join(args720, " ")
	expectedFlags720 := []string{
		"-vf scale=-2:'min(720,ih)'",
		"-c:v libx264",
		"-b:v 2500k",
		"-maxrate 3000k",
		"-bufsize 5000k",
		"-c:a aac",
		"-b:a 128k",
		"-ac 2",
		"-movflags +faststart",
		"-f mp4",
	}
	for _, flag := range expectedFlags720 {
		if !strings.Contains(argStr720, flag) {
			t.Errorf("missing expected mobile-720p flag %q in args: %s", flag, argStr720)
		}
	}
	if strings.Contains(argStr720, "-crf") {
		t.Errorf("software mobile transcode must not set conflicting -crf flag: %s", argStr720)
	}

	// 2. mobile-1080p with QSV encoder
	args1080, enc1080 := transcoder.BuildFFmpegArgs("/media/master.mkv", "/media/master - 1080p.mp4", "mobile-1080p", HardwareQSV)
	if enc1080 != "qsv" {
		t.Errorf("expected encoder qsv, got %s", enc1080)
	}

	argStr1080 := strings.Join(args1080, " ")
	expectedFlags1080 := []string{
		"-hwaccel qsv",
		"-vf scale=-2:'min(1080,ih)'",
		"-c:v h264_qsv",
		"-b:v 4500k",
		"-maxrate 5500k",
		"-bufsize 9000k",
		"-c:a aac",
		"-b:a 192k",
		"-ac 2",
		"-movflags +faststart",
		"-f mp4",
	}
	for _, flag := range expectedFlags1080 {
		if !strings.Contains(argStr1080, flag) {
			t.Errorf("missing expected mobile-1080p flag %q in args: %s", flag, argStr1080)
		}
	}

	// 3. mobile-1080p with NVENC encoder
	argsNVENC, encNVENC := transcoder.BuildFFmpegArgs("/media/master.mkv", "/media/master - 1080p.mp4", "mobile-1080p", HardwareNVENC)
	if encNVENC != "nvenc" {
		t.Errorf("expected encoder nvenc, got %s", encNVENC)
	}
	argStrNVENC := strings.Join(argsNVENC, " ")
	expectedFlagsNVENC := []string{
		"-hwaccel cuda",
		"-vf scale=-2:'min(1080,ih)'",
		"-c:v h264_nvenc",
		"-b:v 4500k",
		"-maxrate 5500k",
		"-bufsize 9000k",
	}
	for _, flag := range expectedFlagsNVENC {
		if !strings.Contains(argStrNVENC, flag) {
			t.Errorf("missing expected NVENC flag %q in args: %s", flag, argStrNVENC)
		}
	}

	// 4. mobile-720p with VAAPI encoder (hardware surface scaling)
	argsVAAPI, encVAAPI := transcoder.BuildFFmpegArgs("/media/master.mkv", "/media/master - 720p.mp4", "mobile-720p", HardwareVAAPI)
	if encVAAPI != "vaapi" {
		t.Errorf("expected encoder vaapi, got %s", encVAAPI)
	}
	argStrVAAPI := strings.Join(argsVAAPI, " ")
	expectedFlagsVAAPI := []string{
		"-hwaccel vaapi",
		"-vf scale_vaapi=w=-2:h=720",
		"-c:v h264_vaapi",
		"-b:v 2500k",
		"-maxrate 3000k",
		"-bufsize 5000k",
	}
	for _, flag := range expectedFlagsVAAPI {
		if !strings.Contains(argStrVAAPI, flag) {
			t.Errorf("missing expected VAAPI flag %q in args: %s", flag, argStrVAAPI)
		}
	}

	// 5. Mobile profiles MUST NEVER use hevc/h265 even if hardware supports it (direct-play invariant)
	if strings.Contains(argStr720, "hevc") || strings.Contains(argStr720, "h265") {
		t.Errorf("mobile-720p must never use hevc/h265")
	}
	if strings.Contains(argStr1080, "hevc") || strings.Contains(argStr1080, "h265") {
		t.Errorf("mobile-1080p must never use hevc/h265")
	}
}

// TestMobileProfile_MasterPreservedIntact verifies master video is never modified or deleted.
func TestMobileProfile_MasterPreservedIntact(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "Inception (2010).mkv")
	initialContent := []byte("ORIGINAL_UNTOUCHED_MASTER_VIDEO_BITSTREAM_20GB")
	if err := os.WriteFile(masterPath, initialContent, 0644); err != nil {
		t.Fatalf("failed to write master file: %v", err)
	}
	origFi, err := os.Stat(masterPath)
	if err != nil {
		t.Fatalf("failed to stat master file: %v", err)
	}

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true}) // always off-peak
	mockTC := NewMockTranscoder()
	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	job, isExisting, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil {
		t.Fatalf("EnqueueMobile failed: %v", err)
	}
	if isExisting {
		t.Errorf("expected isExisting=false for new job")
	}

	// Process the job
	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("ProcessOnce failed: %v", err)
	}

	// Verify master file is still intact and unmodified
	currentFi, err := os.Stat(masterPath)
	if err != nil {
		t.Fatalf("master file was deleted or lost: %v", err)
	}
	if currentFi.Size() != origFi.Size() {
		t.Errorf("master file size changed: before=%d, after=%d", origFi.Size(), currentFi.Size())
	}
	currentContent, err := os.ReadFile(masterPath)
	if err != nil {
		t.Fatalf("failed to read master file: %v", err)
	}
	if string(currentContent) != string(initialContent) {
		t.Errorf("master file content was altered during mobile transcode")
	}

	// Verify mobile profile was generated alongside master
	targetFi, err := os.Stat(job.TargetPath)
	if err != nil {
		t.Fatalf("mobile target file was not created: %v", err)
	}
	if targetFi.Size() == 0 {
		t.Errorf("mobile target file is empty")
	}
	if !strings.HasSuffix(job.TargetPath, " - 720p.mp4") {
		t.Errorf("target path unexpected suffix: %s", job.TargetPath)
	}
}

// TestMobileProfile_DuplicateAvoidance verifies pre-existing mobile profiles are not re-encoded.
func TestMobileProfile_DuplicateAvoidance(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "Interstellar (2014).mkv")
	_ = os.WriteFile(masterPath, []byte("MASTER_VIDEO"), 0644)

	// Pre-create the mobile profile on disk
	mobileTarget := filepath.Join(tempDir, "Interstellar (2014) - 720p.mp4")
	_ = os.WriteFile(mobileTarget, []byte("PRE_EXISTING_720P_PROFILE"), 0644)

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()
	refresher := &mockRefresher{}

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
		Refresher:  refresher,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. EnqueueMobile when profile already exists on disk
	job, isExisting, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil {
		t.Fatalf("EnqueueMobile failed: %v", err)
	}
	if !isExisting {
		t.Errorf("expected isExisting=true when target profile exists on disk")
	}
	if job.Status != StatusCompleted {
		t.Errorf("expected status 'completed', got %s", job.Status)
	}
	if job.Progress != 100.0 {
		t.Errorf("expected progress 100.0, got %f", job.Progress)
	}

	// Wait briefly for asynchronous Jellyfin refresh
	time.Sleep(30 * time.Millisecond)
	refresher.mu.Lock()
	var foundJellyfin bool
	for _, s := range refresher.refreshedSvc {
		if s == "jellyfin" {
			foundJellyfin = true
			break
		}
	}
	refresher.mu.Unlock()
	if !foundJellyfin {
		t.Errorf("expected Jellyfin refresh to be triggered on pre-existing profile, got %v", refresher.refreshedSvc)
	}

	// 2. ProcessOnce should have nothing to do (TranscoderCalls must be 0)
	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("ProcessOnce failed: %v", err)
	}
	if mockTC.TranscodeCalls != 0 {
		t.Errorf("expected 0 transcoder calls for duplicate, got %d", mockTC.TranscodeCalls)
	}
}

// TestMobileProfile_ExecuteJob_TargetExistsOnDisk verifies that if a job was queued but the file was created on disk,
// executeJob marks it completed with "existing" without running the transcode binary.
func TestMobileProfile_ExecuteJob_TargetExistsOnDisk(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "The Dark Knight (2008).mkv")
	_ = os.WriteFile(masterPath, []byte("DARK_KNIGHT_MASTER"), 0644)

	targetMP4 := filepath.Join(tempDir, "The Dark Knight (2008) - 720p.mp4")

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()
	refresher := &mockRefresher{}

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
		Refresher:  refresher,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Enqueue job before target file exists
	job, _, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil {
		t.Fatalf("EnqueueMobile failed: %v", err)
	}

	// Now place the target file on disk before ProcessOnce runs
	_ = os.WriteFile(targetMP4, []byte("PLACED_TARGET_FILE_BEFORE_ENCODE"), 0644)

	// Process the job
	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("ProcessOnce failed: %v", err)
	}

	// Verify transcoder was NOT invoked
	if mockTC.TranscodeCalls != 0 {
		t.Errorf("expected 0 transcoder calls, got %d", mockTC.TranscodeCalls)
	}

	// Verify job in store is completed with encoder "existing"
	jobInStore, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to get job from store: %v", err)
	}
	if jobInStore.Status != StatusCompleted {
		t.Errorf("expected status 'completed', got %s", jobInStore.Status)
	}
	if jobInStore.EncoderUsed != "existing" {
		t.Errorf("expected encoder 'existing', got %s", jobInStore.EncoderUsed)
	}

	// Verify Jellyfin refresh was triggered
	time.Sleep(30 * time.Millisecond)
	refresher.mu.Lock()
	var foundJellyfin bool
	for _, s := range refresher.refreshedSvc {
		if s == "jellyfin" {
			foundJellyfin = true
			break
		}
	}
	refresher.mu.Unlock()
	if !foundJellyfin {
		t.Errorf("expected Jellyfin refresh upon target detection, got %v", refresher.refreshedSvc)
	}
}

// TestMobileProfile_EnqueueMobile_Deduplication verifies EnqueueMobile avoids duplicates when queued or paused.
func TestMobileProfile_EnqueueMobile_Deduplication(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "Oppenheimer (2023).mkv")
	_ = os.WriteFile(masterPath, []byte("OPPENHEIMER_MASTER"), 0644)

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. Initial enqueue
	job1, isExisting, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil || isExisting {
		t.Fatalf("initial enqueue failed: err=%v, isExisting=%v", err, isExisting)
	}

	// 2. Second enqueue while StatusQueued: must return job1
	job2, isExisting2, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil || isExisting2 {
		t.Fatalf("second enqueue failed: err=%v, isExisting=%v", err, isExisting2)
	}
	if job2.ID != job1.ID {
		t.Errorf("expected identical job ID %s, got %s", job1.ID, job2.ID)
	}

	// 3. Mark job as paused, then enqueue again: must return job1
	_ = store.UpdateStatus(ctx, job1.ID, StatusPaused, "Paused for AI priority")
	job3, isExisting3, err := engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-720p",
	})
	if err != nil || isExisting3 {
		t.Fatalf("third enqueue failed: err=%v, isExisting=%v", err, isExisting3)
	}
	if job3.ID != job1.ID {
		t.Errorf("expected identical job ID %s when paused, got %s", job1.ID, job3.ID)
	}

	// 4. Verify queue length remains 1
	jobs, err := store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected exactly 1 job in queue, got %d", len(jobs))
	}
}

// TestMobileProfile_EngineEnqueue_DefaultTarget verifies engine.Enqueue automatically computes
// Jellyfin multi-version path when TargetPath is empty and profile is mobile-720p.
func TestMobileProfile_EngineEnqueue_DefaultTarget(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "Dune Part Two (2024).mkv")
	_ = os.WriteFile(masterPath, []byte("DUNE_TWO_MASTER"), 0644)

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	job, err := engine.Enqueue(ctx, EnqueueRequest{
		SourcePath: masterPath,
		Profile:    "mobile-720p",
	})
	if err != nil {
		t.Fatalf("engine.Enqueue failed: %v", err)
	}

	expectedTarget := filepath.Join(tempDir, "Dune Part Two (2024) - 720p.mp4")
	if job.TargetPath != expectedTarget {
		t.Errorf("expected target %q, got %q", expectedTarget, job.TargetPath)
	}
}

// TestMobileProfile_InputValidation verifies unsupported extensions and directory escape prevention.
func TestMobileProfile_InputValidation(t *testing.T) {
	tempDir := t.TempDir()
	txtFile := filepath.Join(tempDir, "document.txt")
	_ = os.WriteFile(txtFile, []byte("not a video"), 0644)

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. Text file rejected
	_, _, err = engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    txtFile,
		TargetProfile: "mobile-720p",
	})
	if !errors.Is(err, ErrUnsupportedVideoFormat) {
		t.Errorf("expected ErrUnsupportedVideoFormat, got %v", err)
	}

	// 2. File outside data directory rejected
	outsideDir := t.TempDir()
	outsideVideo := filepath.Join(outsideDir, "outside.mkv")
	_ = os.WriteFile(outsideVideo, []byte("outside video"), 0644)

	_, _, err = engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    outsideVideo,
		TargetProfile: "mobile-720p",
	})
	if !errors.Is(err, ErrSourceOutsideDataDir) {
		t.Errorf("expected ErrSourceOutsideDataDir, got %v", err)
	}
}

// TestMobileProfile_HeadroomSafetyFloor verifies mobile encodes respect the 10% storage floor.
func TestMobileProfile_HeadroomSafetyFloor(t *testing.T) {
	tempDir := t.TempDir()
	masterPath := filepath.Join(tempDir, "The Matrix (1999).mkv")
	_ = os.WriteFile(masterPath, []byte("MASTER_VIDEO"), 0644)

	_, store := setupTestDB(t)
	sched, _ := NewScheduler(ScheduleConfig{StartHour: 1, EndHour: 7, ManualOverride: true})
	mockTC := NewMockTranscoder()

	engine, err := NewTranscodeEngine(EngineConfig{
		DataDir:    tempDir,
		Store:      store,
		Scheduler:  sched,
		Transcoder: mockTC,
		HeadroomChecker: func(dirPath string) (float64, error) {
			return 8.2, nil // Below 10%
		},
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	_, _, err = engine.EnqueueMobile(ctx, MobileEnqueueRequest{
		SourcePath:    masterPath,
		TargetProfile: "mobile-1080p",
	})
	if err != nil {
		t.Fatalf("EnqueueMobile failed: %v", err)
	}

	// ProcessOnce must fail with ErrStorageHeadroomLow
	err = engine.ProcessOnce(ctx)
	if err != ErrStorageHeadroomLow {
		t.Fatalf("expected ErrStorageHeadroomLow, got %v", err)
	}
	if mockTC.TranscodeCalls != 0 {
		t.Errorf("expected 0 transcoder calls under low headroom, got %d", mockTC.TranscodeCalls)
	}
}
