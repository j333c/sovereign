package optical

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// State represents the discrete optical ripper lifecycle phases.
const (
	StateIdle      = "idle"
	StateDetected  = "detected"
	StateRipping   = "ripping"
	StateEjecting  = "ejecting"
	StatePaused    = "paused"
	StateError     = "error"
)

const (
	StatusTextIdle         = "Idle · Ready for disc"
	StatusTextDetected     = "Disc detected"
	StatusTextAutoEjecting = "Auto-ejecting disc"
	StatusTextStorageLow   = "Storage low: Imports paused"
	StatusTextKeysRequired = "Decryption keys required"
	StatusTextReadError    = "Disc read error"

	NoticeMissingKeys = "This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."
	NoticeReadFailed  = "Disc reading failed after multiple attempts. The disc has been safely ejected."
	NoticeStorageLow  = "Transcoding paused: storage capacity is below 10%. Please free up space to continue."
)

// OpticalStatus represents the public state of the optical ripper engine.
type OpticalStatus struct {
	State       string   `json:"state"`
	StatusText  string   `json:"status_text"`
	DriveReady  bool     `json:"drive_ready"`
	DiscPresent bool     `json:"disc_present"`
	DiscType    DiscType `json:"disc_type,omitempty"`
	DiscLabel   string   `json:"disc_label,omitempty"`
	DiscID      string   `json:"disc_id,omitempty"`
	Progress    float64  `json:"progress"`
	Error       string   `json:"error,omitempty"`
	Notice      string   `json:"notice,omitempty"`
	StagingPath string   `json:"staging_path,omitempty"`
	KeysActive  bool     `json:"keys_active"`
}

// RipperConfig holds configuration options for OpticalRipper.
type RipperConfig struct {
	DataDir          string
	StagingDir       string
	Driver           OpticalDriver
	KeyMgr           *KEYDBManager
	EventBroadcaster func(event string, data any)
	HeadroomChecker  func(path string) (float64, error)
	PollingInterval  time.Duration
	RetryDelay       time.Duration
	EjectResetDelay  time.Duration
	Logger           *slog.Logger
}

// OpticalRipper coordinates hardware polling, media identification,
// headroom enforcement, retry loops, and disc auto-ejection.
type OpticalRipper struct {
	cfg               RipperConfig
	driver            OpticalDriver
	keyMgr            *KEYDBManager
	broadcaster       func(event string, data any)
	headroomChecker   func(path string) (float64, error)
	stagingDir        string
	logger            *slog.Logger
	pollingInterval   time.Duration
	retryDelay        time.Duration
	ejectResetDelay   time.Duration

	mu                sync.RWMutex
	status            OpticalStatus
	lastProcessedDisc string
	stopChan          chan struct{}
	wg                sync.WaitGroup
	closeOnce         sync.Once
	isPolling         bool
}

// NewOpticalRipper instantiates and initializes an OpticalRipper.
func NewOpticalRipper(cfg RipperConfig) *OpticalRipper {
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.StagingDir == "" {
		cfg.StagingDir = filepath.Join(cfg.DataDir, "media", "staging", "discs")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Driver == nil {
		cfg.Driver = NewLinuxOpticalDriver("/dev/sr0", cfg.Logger)
	}
	if cfg.KeyMgr == nil {
		cfg.KeyMgr = NewKEYDBManager(filepath.Join(cfg.DataDir, "state", "keys", "KEYDB.cfg"), cfg.Logger)
	}
	if cfg.HeadroomChecker == nil {
		cfg.HeadroomChecker = defaultHeadroomChecker
	}
	if cfg.PollingInterval <= 0 {
		cfg.PollingInterval = 2 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 100 * time.Millisecond
	}

	keysActive := cfg.KeyMgr != nil && cfg.KeyMgr.HasKeys()

	ripper := &OpticalRipper{
		cfg:             cfg,
		driver:          cfg.Driver,
		keyMgr:          cfg.KeyMgr,
		broadcaster:     cfg.EventBroadcaster,
		headroomChecker: cfg.HeadroomChecker,
		stagingDir:      cfg.StagingDir,
		logger:          cfg.Logger,
		pollingInterval: cfg.PollingInterval,
		retryDelay:      cfg.RetryDelay,
		ejectResetDelay: cfg.EjectResetDelay,
		stopChan:        make(chan struct{}),
		status: OpticalStatus{
			State:       StateIdle,
			StatusText:  StatusTextIdle,
			DriveReady:  true,
			DiscPresent: false,
			DiscType:    DiscTypeNone,
			Progress:    0.0,
			KeysActive:  keysActive,
		},
	}

	// Ensure staging directory exists
	_ = os.MkdirAll(ripper.stagingDir, 0755)

	return ripper
}

// SetEventBroadcaster configures or updates the SSE event broadcast callback.
func (r *OpticalRipper) SetEventBroadcaster(fn func(event string, data any)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcaster = fn
}

// GetStatus returns the current snapshot of optical ripping engine status.
func (r *OpticalRipper) GetStatus() OpticalStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	st := r.status
	if r.keyMgr != nil {
		st.KeysActive = r.keyMgr.HasKeys()
	}
	return st
}

// KeyManager returns the active KEYDBManager instance.
func (r *OpticalRipper) KeyManager() *KEYDBManager {
	return r.keyMgr
}

// Start launches the continuous background hardware polling loop.
func (r *OpticalRipper) Start(ctx context.Context) {
	r.mu.Lock()
	if r.isPolling {
		r.mu.Unlock()
		return
	}
	r.isPolling = true
	r.wg.Add(1)
	r.mu.Unlock()

	go r.pollLoop(ctx)
}

// Close gracefully terminates the background supervisor loop.
func (r *OpticalRipper) Close() error {
	r.closeOnce.Do(func() {
		close(r.stopChan)
	})
	r.wg.Wait()
	return nil
}

// Eject issues an immediate manual tray ejection and resets state to idle.
func (r *OpticalRipper) Eject(ctx context.Context) error {
	r.mu.Lock()
	r.lastProcessedDisc = ""
	r.mu.Unlock()

	err := r.driver.Eject(ctx)
	r.setStatus(StateIdle, StatusTextIdle, DiscInfo{Inserted: false, DiscType: DiscTypeNone}, 0, "", "")
	r.broadcast("optical.disc_ejected", map[string]any{"reason": "manual_eject"})
	return err
}

// SaveKeys validates and updates KEYDB.cfg atomically.
func (r *OpticalRipper) SaveKeys(content []byte) (int, error) {
	if r.keyMgr == nil {
		return 0, fmt.Errorf("KEYDBManager is not initialized")
	}

	count, err := r.keyMgr.SaveKeys(content)
	if err != nil {
		return 0, err
	}

	r.mu.Lock()
	r.status.KeysActive = true
	r.mu.Unlock()

	r.broadcast("optical.keys_updated", map[string]any{
		"keys_count": count,
		"active":     true,
	})

	return count, nil
}

// pollLoop continuously checks the optical hardware every pollingInterval.
func (r *OpticalRipper) pollLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.pollingInterval)
	defer ticker.Stop()

	// Perform initial check immediately
	r.PollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.PollOnce(ctx)
		}
	}
}

// PollOnce inspects the drive and executes automated ingestion if a new disc is present.
func (r *OpticalRipper) PollOnce(ctx context.Context) {
	driveStatus, err := r.driver.GetDriveStatus(ctx)
	if err != nil {
		r.logger.Warn("failed to poll optical drive status", "error", err)
		return
	}

	r.mu.Lock()
	r.status.DriveReady = driveStatus.DriveReady
	keysActive := r.keyMgr != nil && r.keyMgr.HasKeys()
	r.status.KeysActive = keysActive

	if !driveStatus.Disc.Inserted || driveStatus.TrayOpen {
		// Drive is empty or tray is open; reset last processed disc
		r.status.DiscPresent = false
		r.status.DiscType = DiscTypeNone
		r.status.DiscLabel = ""
		r.status.DiscID = ""
		if r.status.State != StateRipping && r.status.State != StateEjecting {
			r.status.State = StateIdle
			r.status.StatusText = StatusTextIdle
			r.status.Progress = 0.0
		}
		r.lastProcessedDisc = ""
		r.mu.Unlock()
		return
	}

	disc := driveStatus.Disc
	discID := disc.ID
	if discID == "" {
		discID = disc.Label
	}

	// Avoid re-triggering an already handled disc while tray remains closed
	if r.lastProcessedDisc == discID && discID != "" {
		r.mu.Unlock()
		return
	}

	if r.status.State == StateRipping || r.status.State == StateEjecting {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	// New disc detected! Handle rip cycle.
	r.handleDiscInsertion(ctx, disc)
}

// handleDiscInsertion coordinates disc verification, headroom check, ripping, retries, and auto-ejection.
func (r *OpticalRipper) handleDiscInsertion(ctx context.Context, disc DiscInfo) {
	discID := disc.ID
	if discID == "" {
		discID = disc.Label
	}
	if discID == "" {
		discID = fmt.Sprintf("disc_%d", time.Now().Unix())
		disc.ID = discID
	}

	r.logger.Info("optical disc inserted", "disc_type", disc.DiscType, "label", disc.Label, "id", discID)

	// Step 1: Transition to Disc detected
	r.setStatus(StateDetected, StatusTextDetected, disc, 0.0, "", "")
	r.broadcast("optical.disc_detected", map[string]any{
		"disc_type": disc.DiscType,
		"label":     disc.Label,
		"id":        discID,
	})

	// Step 2: Enforce 10% storage capacity headroom floor
	freePct, err := r.headroomChecker(r.stagingDir)
	if err != nil {
		r.logger.Warn("failed to check storage headroom, continuing cautiously", "error", err)
	} else if freePct < 0.10 {
		r.logger.Warn("storage headroom below 10% safety floor, pausing optical import", "free_percent", freePct*100)
		r.setStatus(StatePaused, StatusTextStorageLow, disc, 0.0, "", NoticeStorageLow)
		r.broadcast("optical.alert", map[string]any{
			"type":    "storage_low",
			"notice":  NoticeStorageLow,
			"free_pct": freePct,
		})
		_ = r.driver.Eject(ctx)
		r.broadcast("optical.disc_ejected", map[string]any{
			"disc_id": discID,
			"reason":  "storage_low",
		})
		r.mu.Lock()
		r.lastProcessedDisc = discID
		r.mu.Unlock()
		return
	}

	// Step 3: Commercial encrypted Blu-ray AACS key validation
	if disc.DiscType == DiscTypeBluray && disc.Encrypted {
		keysAvailable := r.keyMgr != nil && (r.keyMgr.HasKeyForDisc(discID) || r.keyMgr.HasKeys())
		if !keysAvailable {
			r.logger.Info("commercial encrypted Blu-ray inserted without decryption keys, auto-ejecting", "disc_id", discID)
			r.setStatus(StateEjecting, StatusTextKeysRequired, disc, 0.0, "", NoticeMissingKeys)
			r.broadcast("optical.alert", map[string]any{
				"type":    "missing_keys",
				"notice":  NoticeMissingKeys,
				"disc_id": discID,
			})

			_ = r.driver.Eject(ctx)
			r.broadcast("optical.disc_ejected", map[string]any{
				"disc_id": discID,
				"reason":  "missing_keys",
			})

			r.mu.Lock()
			r.lastProcessedDisc = discID
			r.mu.Unlock()

			if r.ejectResetDelay > 0 {
				time.Sleep(r.ejectResetDelay)
			}
			r.setStatus(StateIdle, StatusTextIdle, DiscInfo{Inserted: false, DiscType: DiscTypeNone}, 0.0, "", NoticeMissingKeys)
			return
		}
	}

	// Step 4: Prepare isolated staging directory under /data/media/staging/discs/:disc_id/
	// Unique hashing: Never overwrite existing ripped staging archives without unique ID hashing
	stagingTarget := filepath.Join(r.stagingDir, discID)
	if _, err := os.Stat(stagingTarget); err == nil {
		uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
		stagingTarget = filepath.Join(r.stagingDir, fmt.Sprintf("%s-%s", discID, uniqueSuffix[:8]))
	}

	if err := os.MkdirAll(stagingTarget, 0755); err != nil {
		r.logger.Error("failed to create staging target directory", "path", stagingTarget, "error", err)
		r.setStatus(StateError, "Staging error", disc, 0.0, err.Error(), "")
		return
	}

	// Step 5: Execute ripping with maximum 3 retries on read/scratch errors
	r.setStatus(StateRipping, "Ripping (0%)", disc, 0.0, "", "")
	r.broadcast("optical.rip_progress", map[string]any{
		"disc_id":  discID,
		"progress": 0.0,
	})

	var ripErr error
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ripErr = r.driver.RipDisc(ctx, disc, stagingTarget, func(pct float64) {
			r.mu.Lock()
			r.status.Progress = pct
			r.status.StatusText = fmt.Sprintf("Ripping (%.0f%%)", pct)
			r.mu.Unlock()
			r.broadcast("optical.rip_progress", map[string]any{
				"disc_id":  discID,
				"progress": pct,
			})
		})

		if ripErr == nil {
			break
		}

		r.logger.Warn("read error during disc ripping, retrying", "attempt", attempt, "max", 3, "error", ripErr)
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.retryDelay):
			}
		}
	}

	// Step 6: Handle read failure or rip completion
	if ripErr != nil {
		r.logger.Error("disc read failed after 3 attempts, auto-ejecting tray", "disc_id", discID, "error", ripErr)
		r.setStatus(StateError, StatusTextReadError, disc, 0.0, ripErr.Error(), NoticeReadFailed)
		r.broadcast("optical.read_failed", map[string]any{
			"disc_id": discID,
			"error":   ripErr.Error(),
			"notice":  NoticeReadFailed,
		})
		r.broadcast("optical.alert", map[string]any{
			"type":    "read_failed",
			"error":   ripErr.Error(),
			"notice":  NoticeReadFailed,
			"disc_id": discID,
		})

		_ = r.driver.Eject(ctx)
		r.broadcast("optical.disc_ejected", map[string]any{
			"disc_id": discID,
			"reason":  "read_failed",
		})

		r.mu.Lock()
		r.lastProcessedDisc = discID
		r.mu.Unlock()

		if r.ejectResetDelay > 0 {
			time.Sleep(r.ejectResetDelay)
		}
		r.setStatus(StateIdle, StatusTextIdle, DiscInfo{Inserted: false, DiscType: DiscTypeNone}, 0.0, "", NoticeReadFailed)
		return
	}

	// Step 7: Rip completed successfully: update to 100% and auto-eject
	r.logger.Info("disc rip completed successfully", "disc_id", discID, "staging_path", stagingTarget)
	r.setStatus(StateRipping, "Ripping (100%)", disc, 100.0, "", "")
	r.broadcast("optical.rip_progress", map[string]any{
		"disc_id":  discID,
		"progress": 100.0,
	})

	r.mu.Lock()
	r.status.StagingPath = stagingTarget
	r.mu.Unlock()

	// Transition to auto-ejecting
	r.setStatus(StateEjecting, StatusTextAutoEjecting, disc, 100.0, "", "")
	_ = r.driver.Eject(ctx)
	r.broadcast("optical.disc_ejected", map[string]any{
		"disc_id":      discID,
		"staging_path": stagingTarget,
		"reason":       "complete",
	})

	r.mu.Lock()
	r.lastProcessedDisc = discID
	r.mu.Unlock()

	if r.ejectResetDelay > 0 {
		time.Sleep(r.ejectResetDelay)
	}
	r.setStatus(StateIdle, StatusTextIdle, DiscInfo{Inserted: false, DiscType: DiscTypeNone}, 0.0, "", "")
}

func (r *OpticalRipper) setStatus(state, statusText string, disc DiscInfo, progress float64, errText, notice string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.status.State = state
	r.status.StatusText = statusText
	r.status.DiscPresent = disc.Inserted
	r.status.DiscType = disc.DiscType
	r.status.DiscLabel = disc.Label
	r.status.DiscID = disc.ID
	r.status.Progress = progress
	r.status.Error = errText
	r.status.Notice = notice
	if r.keyMgr != nil {
		r.status.KeysActive = r.keyMgr.HasKeys()
	}
}

func (r *OpticalRipper) broadcast(event string, data any) {
	r.mu.RLock()
	fn := r.broadcaster
	r.mu.RUnlock()

	if fn != nil {
		fn(event, data)
	}
}

// defaultHeadroomChecker uses statfs to check the ratio of free blocks to total blocks.
func defaultHeadroomChecker(path string) (float64, error) {
	var stat syscall.Statfs_t
	// Stat parent if path doesn't exist yet
	checkPath := path
	for checkPath != "/" && checkPath != "." {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		checkPath = filepath.Dir(checkPath)
	}

	if err := syscall.Statfs(checkPath, &stat); err != nil {
		return 0, fmt.Errorf("statfs failed for %q: %w", checkPath, err)
	}

	if stat.Blocks == 0 {
		return 1.0, nil
	}

	return float64(stat.Bavail) / float64(stat.Blocks), nil
}
