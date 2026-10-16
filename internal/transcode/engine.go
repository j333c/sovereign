package transcode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrStorageHeadroomLow = errors.New("storage capacity below 10% safety threshold")
	ErrDaytimeRestricted  = errors.New("daytime hours active: batch video transcoding paused to preserve GPU VRAM for AI inference")
)

// LibraryRefresher defines the interface to trigger Jellyfin/Navidrome library refresh webhooks.
type LibraryRefresher interface {
	RefreshService(ctx context.Context, serviceID string) error
}

// EnqueueRequest defines arguments for enqueuing a new transcode job.
type EnqueueRequest struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path,omitempty"`
	Profile    string `json:"profile,omitempty"`
}

// EngineStatus describes the live state of the transcoding engine.
type EngineStatus struct {
	State          string    `json:"state"` // "idle", "processing", "paused", "storage_low"
	StatusText     string    `json:"status_text"`
	ActiveJobID    string    `json:"active_job_id,omitempty"`
	ActiveSource   string    `json:"active_source,omitempty"`
	ActiveProgress float64   `json:"active_progress"`
	GPUStatus      string    `json:"gpu_status"`
	IsOffPeak      bool      `json:"is_off_peak"`
	ManualOverride bool      `json:"manual_override"`
	QueueLength    int       `json:"queue_length"`
	LastRunAt      time.Time `json:"last_run_at,omitempty"`
}

// EngineConfig holds dependencies for the TranscodeEngine.
type EngineConfig struct {
	DataDir         string
	Store           Store
	Scheduler       *Scheduler
	Transcoder      Transcoder
	Refresher       LibraryRefresher
	Broadcaster     func(event string, data any)
	HeadroomChecker func(dirPath string) (float64, error)
	PollInterval    time.Duration
	Logger          *slog.Logger
}

// TranscodeEngine coordinates the off-peak video transcoding queue, enforces the 10%
// storage headroom floor, handles GPU arbitration (day/night window & manual override),
// executes transcodes with hardware acceleration and software fallback, and dispatches
// Jellyfin library refresh webhooks upon completion.
type TranscodeEngine struct {
	dataDir         string
	store           Store
	scheduler       *Scheduler
	transcoder      Transcoder
	refresher       LibraryRefresher
	broadcaster     func(event string, data any)
	headroomChecker func(dirPath string) (float64, error)
	pollInterval    time.Duration
	logger          *slog.Logger

	mu             sync.RWMutex
	activeJobID    string
	activeSource   string
	activeProgress float64
	activeCancel   context.CancelFunc
	state          string
	statusText     string
	wakeCh         chan struct{}
	stopCh         chan struct{}
	wg             sync.WaitGroup
	closeOnce      sync.Once
}

// NewTranscodeEngine creates and initializes a TranscodeEngine.
func NewTranscodeEngine(cfg EngineConfig) (*TranscodeEngine, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.Scheduler == nil {
		var err error
		cfg.Scheduler, err = NewScheduler(DefaultScheduleConfig())
		if err != nil {
			return nil, fmt.Errorf("failed to initialize default scheduler: %w", err)
		}
	}
	if cfg.Transcoder == nil {
		cfg.Transcoder = NewFFmpegTranscoder(cfg.Logger, DefaultHardwareDetector)
	}
	if cfg.HeadroomChecker == nil {
		cfg.HeadroomChecker = DefaultHeadroomChecker
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}

	engine := &TranscodeEngine{
		dataDir:         cfg.DataDir,
		store:           cfg.Store,
		scheduler:       cfg.Scheduler,
		transcoder:      cfg.Transcoder,
		refresher:       cfg.Refresher,
		broadcaster:     cfg.Broadcaster,
		headroomChecker: cfg.HeadroomChecker,
		pollInterval:    cfg.PollInterval,
		logger:          cfg.Logger,
		state:           "idle",
		statusText:      "Idle",
		wakeCh:          make(chan struct{}, 1),
		stopCh:          make(chan struct{}),
	}

	// Reset any dangling processing jobs from prior daemon runs
	if cfg.Store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = cfg.Store.ResetProcessingJobs(ctx)
		cancel()
	}

	return engine, nil
}

// Start launches the background transcoding orchestrator goroutine.
func (e *TranscodeEngine) Start(ctx context.Context) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopCh:
				return
			case <-e.wakeCh:
				e.runLoopPass(ctx)
			case <-ticker.C:
				e.runLoopPass(ctx)
			}
		}
	}()
}

func (e *TranscodeEngine) triggerWake() {
	select {
	case e.wakeCh <- struct{}{}:
	default:
	}
}

// Close stops background routines and cancels any currently executing encode.
func (e *TranscodeEngine) Close() error {
	e.closeOnce.Do(func() {
		e.cancelActiveJob()
		close(e.stopCh)
	})
	e.wg.Wait()
	return nil
}

func (e *TranscodeEngine) cancelActiveJob() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeCancel != nil {
		e.activeCancel()
		e.activeCancel = nil
	}
}

// Enqueue adds a media file to the transcoding queue.
func (e *TranscodeEngine) Enqueue(ctx context.Context, req EnqueueRequest) (*Job, error) {
	if strings.TrimSpace(req.SourcePath) == "" {
		return nil, errors.New("source_path cannot be empty")
	}

	cleanSource := filepath.Clean(req.SourcePath)
	fi, err := os.Stat(cleanSource)
	if err != nil {
		return nil, fmt.Errorf("source file %q does not exist or is inaccessible: %w", cleanSource, err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("source path %q is a directory, not a video file", cleanSource)
	}

	targetPath := strings.TrimSpace(req.TargetPath)
	if targetPath == "" {
		ext := filepath.Ext(cleanSource)
		baseWithoutExt := strings.TrimSuffix(cleanSource, ext)
		if strings.EqualFold(ext, ".mp4") {
			targetPath = baseWithoutExt + ".direct-play.mp4"
		} else {
			targetPath = baseWithoutExt + ".mp4"
		}
	} else {
		targetPath = filepath.Clean(targetPath)
	}

	profile := strings.TrimSpace(req.Profile)
	if profile == "" {
		profile = "universal"
	}

	job := &Job{
		SourcePath: cleanSource,
		TargetPath: targetPath,
		Profile:    profile,
		Status:     StatusQueued,
		Progress:   0.0,
		CreatedAt:  time.Now().UTC(),
	}

	if e.store != nil {
		if err := e.store.Enqueue(ctx, job); err != nil {
			return nil, fmt.Errorf("failed to persist transcode job: %w", err)
		}
	}

	e.logger.Info("enqueued transcode job", "id", job.ID, "source", job.SourcePath, "target", job.TargetPath)

	// Wake up background worker if active
	if e.scheduler.IsActiveNow() {
		e.triggerWake()
	}

	return job, nil
}

// GetQueue returns all jobs in the transcoding queue.
func (e *TranscodeEngine) GetQueue(ctx context.Context) ([]Job, error) {
	if e.store == nil {
		return nil, nil
	}
	return e.store.ListJobs(ctx)
}

// GetJob retrieves a specific job by ID.
func (e *TranscodeEngine) GetJob(ctx context.Context, id string) (*Job, error) {
	if e.store == nil {
		return nil, ErrJobNotFound
	}
	return e.store.GetJob(ctx, id)
}

// Scheduler returns the active Scheduler instance.
func (e *TranscodeEngine) Scheduler() *Scheduler {
	return e.scheduler
}

// SetManualOverride toggles the manual override bypassing daytime restrictions.
func (e *TranscodeEngine) SetManualOverride(force bool) {
	e.scheduler.SetManualOverride(force)
	e.logger.Info("transcode schedule manual override updated", "force", force)
	if e.broadcaster != nil {
		e.broadcaster("transcode.override_changed", map[string]any{
			"manual_override": force,
			"gpu_status":      e.scheduler.GetGPUStatusNow(),
		})
	}
	if force {
		e.triggerWake()
	} else if !e.scheduler.IsOffPeakNow() {
		// Daytime without override: cancel running job to free GPU VRAM
		e.cancelActiveJob()
	}
}

// GetGPUStatus returns the formatted status string for SSE telemetry.
func (e *TranscodeEngine) GetGPUStatus() string {
	return e.scheduler.GetGPUStatusNow()
}

// SetRefresher updates the LibraryRefresher dynamically.
func (e *TranscodeEngine) SetRefresher(r LibraryRefresher) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refresher = r
}

// SetBroadcaster updates the SSE event broadcaster dynamically.
func (e *TranscodeEngine) SetBroadcaster(b func(event string, data any)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.broadcaster = b
}

// SetHeadroomChecker updates the HeadroomChecker dynamically.
func (e *TranscodeEngine) SetHeadroomChecker(fn func(dirPath string) (float64, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.headroomChecker = fn
}

// SetTranscoder updates the Transcoder implementation dynamically.
func (e *TranscodeEngine) SetTranscoder(t Transcoder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.transcoder = t
}

// GetStatus returns the current live state of the engine.
func (e *TranscodeEngine) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return EngineStatus{
		State:          e.state,
		StatusText:     e.statusText,
		ActiveJobID:    e.activeJobID,
		ActiveSource:   e.activeSource,
		ActiveProgress: e.activeProgress,
		GPUStatus:      e.scheduler.GetGPUStatusNow(),
		IsOffPeak:      e.scheduler.IsOffPeakNow(),
		ManualOverride: e.scheduler.ManualOverride(),
	}
}

// ProcessOnce processes a single queued job if schedule and storage permits.
// Returns nil if no job is waiting or execution completed successfully.
func (e *TranscodeEngine) ProcessOnce(ctx context.Context) error {
	return e.processNextJob(ctx)
}

func (e *TranscodeEngine) runLoopPass(ctx context.Context) {
	if err := e.processNextJob(ctx); err != nil && !errors.Is(err, context.Canceled) {
		e.logger.Debug("transcode loop pass notice", "error", err)
	}
}

func (e *TranscodeEngine) processNextJob(ctx context.Context) error {
	// Step 1: Check Schedule Window & Manual Override
	if !e.scheduler.IsActiveNow() {
		e.mu.Lock()
		if e.activeJobID != "" {
			e.logger.Info("daytime window transition: pausing active batch transcode to preserve GPU VRAM for AI inference")
			if e.activeCancel != nil {
				e.activeCancel()
				e.activeCancel = nil
			}
			if e.store != nil && e.activeJobID != "" {
				_ = e.store.UpdateStatus(ctx, e.activeJobID, StatusPaused, "Paused: daytime hours active")
			}
			if e.broadcaster != nil {
				e.broadcaster("transcode.job_paused", map[string]any{
					"job_id": e.activeJobID,
					"reason": "daytime_transition",
				})
			}
			e.activeJobID = ""
			e.activeSource = ""
			e.activeProgress = 0.0
		}
		e.state = "paused"
		e.statusText = "Daytime (AI Priority) · Transcoding Paused"
		e.mu.Unlock()
		return ErrDaytimeRestricted
	}

	// Step 2: Enforce 10% Storage Headroom Floor
	e.mu.RLock()
	checker := e.headroomChecker
	e.mu.RUnlock()

	if checker != nil {
		freePct, err := checker(e.dataDir)
		if err == nil && freePct < 10.0 {
			e.logger.Warn("storage capacity below 10% safety threshold; pausing batch video transcoding",
				"free_percent", freePct,
			)
			e.mu.Lock()
			if e.activeJobID != "" {
				if e.activeCancel != nil {
					e.activeCancel()
					e.activeCancel = nil
				}
				if e.store != nil {
					_ = e.store.UpdateStatus(ctx, e.activeJobID, StatusPaused, "Paused: storage capacity below 10% safety floor")
				}
				if e.broadcaster != nil {
					e.broadcaster("transcode.job_paused", map[string]any{
						"job_id":       e.activeJobID,
						"reason":       "storage_low",
						"free_percent": freePct,
					})
				}
				e.activeJobID = ""
				e.activeSource = ""
				e.activeProgress = 0.0
			}
			e.state = "storage_low"
			e.statusText = "Storage low (<10%) · Transcoding Paused"
			e.mu.Unlock()

			if e.broadcaster != nil {
				e.broadcaster("storage.disk_alert", map[string]any{
					"reason":       "storage_low",
					"free_percent": freePct,
					"message":      "Transcoding paused: storage capacity is below 10%",
				})
			}
			return ErrStorageHeadroomLow
		}
	}

	// Step 3: Fetch Next Queued/Paused Job
	if e.store == nil {
		return nil
	}

	job, err := e.store.GetNextQueuedJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch next job: %w", err)
	}
	if job == nil {
		e.mu.Lock()
		if e.state != "idle" {
			e.state = "idle"
			e.statusText = "Idle"
		}
		e.mu.Unlock()
		return nil
	}

	// Step 4: Execute Job Transcoding
	return e.executeJob(ctx, job)
}

func (e *TranscodeEngine) executeJob(ctx context.Context, job *Job) error {
	e.mu.RLock()
	tc := e.transcoder
	e.mu.RUnlock()

	if tc == nil {
		return errors.New("transcoder not configured")
	}

	jobCtx, jobCancel := context.WithCancel(ctx)
	defer jobCancel()

	e.mu.Lock()
	e.activeJobID = job.ID
	e.activeSource = job.SourcePath
	e.activeProgress = job.Progress
	e.activeCancel = jobCancel
	e.state = "processing"
	e.statusText = fmt.Sprintf("Transcoding %s", filepath.Base(job.SourcePath))
	e.mu.Unlock()

	detectedHW := tc.DetectHardware(jobCtx)
	_ = e.store.MarkStarted(ctx, job.ID, string(detectedHW))

	if e.broadcaster != nil {
		e.broadcaster("transcode.job_started", map[string]any{
			"job_id":      job.ID,
			"source_path": job.SourcePath,
			"target_path": job.TargetPath,
			"encoder":     string(detectedHW),
		})
	}

	progressCb := func(pct float64) {
		e.mu.Lock()
		if e.activeJobID == job.ID {
			e.activeProgress = pct
		}
		e.mu.Unlock()

		_ = e.store.UpdateProgress(ctx, job.ID, pct)

		if e.broadcaster != nil {
			e.broadcaster("transcode.job_progress", map[string]any{
				"job_id":   job.ID,
				"progress": pct,
			})
		}

		// Check headroom and schedule mid-encode
		if checker := e.headroomChecker; checker != nil {
			if freePct, err := checker(e.dataDir); err == nil && freePct < 10.0 {
				e.logger.Warn("headroom floor breached mid-encode; aborting active job", "free_percent", freePct)
				jobCancel()
			}
		}
		if !e.scheduler.IsActiveNow() {
			e.logger.Info("daytime arrived mid-encode; aborting active job to release VRAM")
			jobCancel()
		}
	}

	// Attempt 1: Transcode using detected hardware acceleration
	opts := TranscodeOptions{
		SourcePath:  job.SourcePath,
		TargetPath:  job.TargetPath,
		Profile:     job.Profile,
		HardwareAcc: detectedHW,
		ProgressCb:  progressCb,
	}

	result, err := tc.Transcode(jobCtx, opts)
	if err != nil {
		// If cancelled (daytime transition, low storage, or shutdown)
		if jobCtx.Err() != nil || errors.Is(err, ErrTranscodeCancelled) {
			e.logger.Info("transcode job interrupted/paused", "job_id", job.ID)
			_ = e.store.UpdateStatus(ctx, job.ID, StatusPaused, "Transcoding interrupted")
			if e.broadcaster != nil {
				e.broadcaster("transcode.job_paused", map[string]any{
					"job_id": job.ID,
					"reason": "interrupted",
				})
			}
			e.resetActiveState()
			return nil
		}

		// Hardware encoder failed mid-encode: fallback to software encoder
		isHW := detectedHW == HardwareQSV || detectedHW == HardwareNVENC || detectedHW == HardwareVAAPI
		if isHW {
			e.logger.Warn("hardware encoder failed; attempting graceful fallback to software encoder (CPU)",
				"job_id", job.ID,
				"failed_encoder", string(detectedHW),
				"error", err,
			)

			fallbackOpts := opts
			fallbackOpts.HardwareAcc = HardwareSoftware
			result, err = tc.Transcode(jobCtx, fallbackOpts)
		}

		if err != nil {
			if jobCtx.Err() != nil || errors.Is(err, ErrTranscodeCancelled) {
				_ = e.store.UpdateStatus(ctx, job.ID, StatusPaused, "Transcoding interrupted")
				e.resetActiveState()
				return nil
			}

			e.logger.Error("transcoding job failed definitively", "job_id", job.ID, "error", err)
			_ = e.store.MarkFailed(ctx, job.ID, err.Error())
			if e.broadcaster != nil {
				e.broadcaster("transcode.job_failed", map[string]any{
					"job_id": job.ID,
					"error":  err.Error(),
				})
			}
			e.resetActiveState()
			return err
		}
	}

	// Job Transcode Succeeded
	encoderUsed := result.EncoderUsed
	if encoderUsed == "" {
		encoderUsed = string(detectedHW)
	}

	_ = e.store.MarkCompleted(ctx, job.ID, result.TargetPath, encoderUsed)
	e.logger.Info("transcoding completed successfully",
		"job_id", job.ID,
		"target", result.TargetPath,
		"encoder", encoderUsed,
		"duration", result.Duration.String(),
	)

	if e.broadcaster != nil {
		e.broadcaster("transcode.job_completed", map[string]any{
			"job_id":           job.ID,
			"target_path":      result.TargetPath,
			"encoder_used":     encoderUsed,
			"duration_seconds": result.Duration.Seconds(),
		})
	}

	e.resetActiveState()

	// Trigger immediate Jellyfin library refresh
	go e.dispatchRefresh("jellyfin")

	return nil
}

func (e *TranscodeEngine) resetActiveState() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.activeJobID = ""
	e.activeSource = ""
	e.activeProgress = 0.0
	e.activeCancel = nil
	e.state = "idle"
	e.statusText = "Idle"
}

func (e *TranscodeEngine) dispatchRefresh(serviceID string) {
	e.mu.RLock()
	refresher := e.refresher
	e.mu.RUnlock()

	if refresher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := refresher.RefreshService(ctx, serviceID); err != nil {
		e.logger.Info("library refresh dispatched (service may be inactive or unavailable)", "service", serviceID, "error", err)
	} else {
		e.logger.Info("library refresh dispatched successfully", "service", serviceID)
	}
}

// DefaultHeadroomChecker calculates free capacity percentage on Linux filesystems.
func DefaultHeadroomChecker(dirPath string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dirPath, &stat); err != nil {
		return 0, err
	}
	if stat.Blocks == 0 {
		return 0, nil
	}
	freePercent := (float64(stat.Bavail) / float64(stat.Blocks)) * 100.0
	return freePercent, nil
}
