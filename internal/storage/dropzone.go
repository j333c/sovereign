package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sovereign/internal/fsutil"
	"sovereign/internal/supervisor"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Supported media category constants.
const (
	CategoryMovies     = "movies"
	CategoryTV         = "tv"
	CategoryMusic      = "music"
	CategoryAudiobooks = "audiobooks"
	CategoryDocuments  = "documents"
)

// ValidCategories defines the recognized media destination categories.
var ValidCategories = []string{
	CategoryMovies,
	CategoryTV,
	CategoryMusic,
	CategoryAudiobooks,
	CategoryDocuments,
}

// Safety buffer required for ingestion operations (1.0 GB).
const DropzoneSafetyBufferBytes int64 = 1 * 1024 * 1024 * 1024

// Sentinel dropzone errors.
var (
	ErrInvalidCategory     = errors.New("invalid media category")
	ErrInvalidMedia        = errors.New("only media files are accepted")
	ErrCategoryMismatch    = errors.New("uploaded media format does not match selected category")
	ErrEmptyFile           = errors.New("file is empty")
	ErrStagingAborted      = errors.New("upload stream was aborted")
	ErrInsufficientStorage = errors.New("insufficient storage space")
)

// CleanOrphanedStagingFiles removes any lingering in-flight staging files (.uploading-*)
// in /data/drop/incoming/:category from previous crashes or ungraceful daemon stops.
func CleanOrphanedStagingFiles(dataDir string) error {
	if dataDir == "" {
		dataDir = "/data"
	}
	for _, cat := range ValidCategories {
		incDir := filepath.Join(dataDir, "drop", "incoming", cat)
		entries, err := os.ReadDir(incDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".uploading-") {
				_ = os.Remove(filepath.Join(incDir, entry.Name()))
			}
		}
	}
	return nil
}

// EnsureDropzoneDirectories auto-provisions staging directories (/data/drop/incoming/:category)
// and media targets (/data/media/:category) with 0755 permissions and cleans any orphaned staging files.
func EnsureDropzoneDirectories(dataDir string) error {
	if dataDir == "" {
		dataDir = "/data"
	}

	for _, cat := range ValidCategories {
		incomingDir := filepath.Join(dataDir, "drop", "incoming", cat)
		if err := os.MkdirAll(incomingDir, 0755); err != nil {
			return fmt.Errorf("failed to create incoming staging directory %q: %w", incomingDir, err)
		}
		var targetDir string
		if cat == CategoryDocuments {
			targetDir = filepath.Join(dataDir, "documents", "consume")
		} else {
			targetDir = filepath.Join(dataDir, "media", cat)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %q: %w", targetDir, err)
		}
	}

	_ = CleanOrphanedStagingFiles(dataDir)
	return nil
}

// ResolveTargetFilename resolves destination filename collisions deterministically.
// If "song.mp3" exists, checks "song (1).mp3", "song (2).mp3", etc.
func ResolveTargetFilename(targetDir, desiredFilename string) string {
	cleanName := filepath.Base(filepath.Clean(desiredFilename))
	if cleanName == "." || cleanName == "/" || cleanName == "" {
		cleanName = "unnamed_media"
	}

	ext := filepath.Ext(cleanName)
	base := strings.TrimSuffix(cleanName, ext)

	candidate := cleanName
	destPath := filepath.Join(targetDir, candidate)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; ; i++ {
		candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
		destPath = filepath.Join(targetDir, candidate)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			return candidate
		}
	}
}

// atomicMoveOrCopy moves a file via os.Rename, falling back to a full file copy + fsync + remove
// if the source and destination are across different mount devices (syscall.EXDEV).
func atomicMoveOrCopy(srcPath, destPath string) error {
	return fsutil.AtomicMoveOrCopy(srcPath, destPath)
}

// DropHistoryItem records metadata for an ingested media file.
type DropHistoryItem struct {
	ID               string `json:"id"`
	Filename         string `json:"filename"`
	OriginalFilename string `json:"original_filename"`
	Category         string `json:"category"`
	Size             int64  `json:"size"`
	ContentType      string `json:"content_type"`
	Path             string `json:"path"`
	Status           string `json:"status"`
	UploadedAt       string `json:"uploaded_at"`
}

// LibraryRefresher triggers media library scans upon promotion.
type LibraryRefresher interface {
	RefreshService(ctx context.Context, serviceID string) error
}

// DiskFreeFunc returns available bytes for a filesystem path.
type DiskFreeFunc = fsutil.DiskFreeFunc

func defaultDiskFree(path string) (int64, error) {
	return fsutil.DiskFree(path)
}

// DropzoneConfig configures the DropzoneEngine.
type DropzoneConfig struct {
	DataDir         string
	Logger          *slog.Logger
	Refresher       LibraryRefresher
	ScanInterval    time.Duration
	StabilityWindow time.Duration
	DiskFree        DiskFreeFunc
}

type fileStability struct {
	lastSize    int64
	lastModTime time.Time
	firstSeen   time.Time
}

// DropzoneEngine coordinates atomic staging writes, magic bytes validation,
// deterministic collision resolution, background SMB drop folder scanning,
// and automated media library refresh triggers.
type DropzoneEngine struct {
	dataDir          string
	logger           *slog.Logger
	refresher        LibraryRefresher
	scanInterval     time.Duration
	stabilityWindow  time.Duration
	historyMu        sync.RWMutex
	history          []DropHistoryItem
	scannerMu        sync.Mutex
	promotionMu      sync.Mutex
	stabilityTracker map[string]fileStability
	diskFreeMu       sync.RWMutex
	diskFree         DiskFreeFunc
	stopChan         chan struct{}
	wg               sync.WaitGroup
	closeOnce        sync.Once
}

// NewDropzoneEngine initializes the dropzone engine and provisions directories.
func NewDropzoneEngine(cfg DropzoneConfig) (*DropzoneEngine, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = 2 * time.Second
	}
	if cfg.StabilityWindow <= 0 {
		cfg.StabilityWindow = 1 * time.Second
	}

	if err := EnsureDropzoneDirectories(cfg.DataDir); err != nil {
		cfg.Logger.Error("failed to initialize dropzone directory hierarchy", "error", err)
		return nil, fmt.Errorf("dropzone directories setup failed: %w", err)
	}

	_ = CleanOrphanedStagingFiles(cfg.DataDir)

	diskFree := cfg.DiskFree
	if diskFree == nil {
		diskFree = defaultDiskFree
	}

	engine := &DropzoneEngine{
		dataDir:          cfg.DataDir,
		logger:           cfg.Logger,
		refresher:        cfg.Refresher,
		scanInterval:     cfg.ScanInterval,
		stabilityWindow:  cfg.StabilityWindow,
		history:          make([]DropHistoryItem, 0, 100),
		stabilityTracker: make(map[string]fileStability),
		diskFree:         diskFree,
		stopChan:         make(chan struct{}),
	}

	return engine, nil
}

// SetRefresher sets or updates the LibraryRefresher.
func (e *DropzoneEngine) SetRefresher(r LibraryRefresher) {
	e.refresher = r
}

// SetDiskFree configures or overrides the free disk space inspection function.
func (e *DropzoneEngine) SetDiskFree(fn DiskFreeFunc) {
	e.diskFreeMu.Lock()
	defer e.diskFreeMu.Unlock()
	e.diskFree = fn
}

// CheckCapacity verifies that target storage has at least requiredBytes + DropzoneSafetyBufferBytes available.
// If available space is insufficient, returns ErrInsufficientStorage.
func (e *DropzoneEngine) CheckCapacity(requiredBytes int64) error {
	if requiredBytes < 0 || requiredBytes > math.MaxInt64-DropzoneSafetyBufferBytes {
		return ErrInsufficientStorage
	}

	e.diskFreeMu.RLock()
	freeFn := e.diskFree
	e.diskFreeMu.RUnlock()

	if freeFn == nil {
		freeFn = defaultDiskFree
	}

	avail, err := freeFn(e.dataDir)
	if err != nil {
		return fmt.Errorf("failed to check available storage: %w", err)
	}

	if avail < requiredBytes+DropzoneSafetyBufferBytes {
		return ErrInsufficientStorage
	}

	return nil
}

func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr contextReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

// HandleUploadStream streams incoming payload into an isolated staging file:
// /data/drop/incoming/:category/.uploading-:uuid-:filename.
// Upon stream closure and fsync(), validates magic bytes, atomically moves
// file to /data/media/:category, and dispatches library refresh.
func (e *DropzoneEngine) HandleUploadStream(ctx context.Context, category, filename string, r io.Reader) (*DropHistoryItem, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if !IsValidCategory(category) {
		return nil, ErrInvalidCategory
	}

	cleanFilename := filepath.Base(filepath.Clean(filename))
	if cleanFilename == "." || cleanFilename == "/" || cleanFilename == "" {
		cleanFilename = "uploaded_media"
	}

	// Pre-flight capacity check: ensure target storage has at least the safety buffer available
	if err := e.CheckCapacity(0); err != nil {
		return nil, err
	}

	uploadUUID := generateUUID()
	stagingFilename := fmt.Sprintf(".uploading-%s-%s", uploadUUID, cleanFilename)
	stagingDir := filepath.Join(e.dataDir, "drop", "incoming", category)
	stagingPath := filepath.Join(stagingDir, stagingFilename)

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure staging directory: %w", err)
	}

	stagingFile, err := os.OpenFile(stagingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create staging file: %w", err)
	}

	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = stagingFile.Close()
			_ = os.Remove(stagingPath)
		}
	}()

	// Stream upload payload directly into staging file respecting context cancellation
	written, copyErr := io.Copy(stagingFile, contextReader{ctx: ctx, r: r})
	if copyErr != nil {
		if errors.Is(copyErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(copyErr, syscall.ENOSPC) {
			return nil, ErrInsufficientStorage
		}
		return nil, fmt.Errorf("%w: %v", ErrStagingAborted, copyErr)
	}
	if written == 0 {
		return nil, ErrEmptyFile
	}

	// Perform fsync() to ensure data durability on disk
	if syncErr := stagingFile.Sync(); syncErr != nil {
		return nil, fmt.Errorf("failed to fsync staging file: %w", syncErr)
	}

	if closeErr := stagingFile.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close staging file: %w", closeErr)
	}

	// Validate magic bytes
	headerBytes, readErr := readHeaderBytes(stagingPath, 512)
	if readErr != nil {
		return nil, fmt.Errorf("failed to inspect staging file header: %w", readErr)
	}

	var contentType, format string
	if category == CategoryDocuments {
		docType, isDoc := DetectDocument(headerBytes)
		if !isDoc {
			e.logger.Warn("rejected non-document file upload", "category", category, "filename", cleanFilename, "staging_path", stagingPath)
			return nil, ErrInvalidMedia
		}
		contentType = docType
		format = strings.TrimPrefix(filepath.Ext(cleanFilename), ".")
	} else {
		ct, fmtSlug, isMedia := DetectMedia(headerBytes)
		if !isMedia {
			e.logger.Warn("rejected non-media file upload", "category", category, "filename", cleanFilename, "staging_path", stagingPath)
			return nil, ErrInvalidMedia
		}
		if !IsCategoryCompatible(category, ct) {
			e.logger.Warn("rejected category mismatch upload", "category", category, "contentType", ct, "filename", cleanFilename)
			return nil, ErrCategoryMismatch
		}
		contentType = ct
		format = fmtSlug
	}

	// Resolve destination file path with mutex protection against concurrent collisions
	var destDir string
	if category == CategoryDocuments {
		destDir = filepath.Join(e.dataDir, "documents", "consume")
	} else {
		destDir = filepath.Join(e.dataDir, "media", category)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure destination directory: %w", err)
	}

	e.promotionMu.Lock()
	finalFilename := ResolveTargetFilename(destDir, cleanFilename)
	destPath := filepath.Join(destDir, finalFilename)

	// Atomically move (or fallback copy) verified media file into destination
	renameErr := atomicMoveOrCopy(stagingPath, destPath)
	e.promotionMu.Unlock()

	if renameErr != nil {
		return nil, fmt.Errorf("failed to promote staging file to destination: %w", renameErr)
	}

	_ = os.Chmod(destPath, 0644)

	// Successfully promoted; staging file moved so no cleanup needed
	cleanupNeeded = false

	item := DropHistoryItem{
		ID:               uploadUUID,
		Filename:         finalFilename,
		OriginalFilename: cleanFilename,
		Category:         category,
		Size:             written,
		ContentType:      contentType,
		Path:             destPath,
		Status:           "promoted",
		UploadedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	e.addHistory(item)
	e.logger.Info("media file successfully promoted", "category", category, "filename", finalFilename, "format", format, "size", written)

	// Trigger library refresh within 5s
	e.triggerLibraryRefresh(category)

	return &item, nil
}

func readHeaderBytes(filePath string, maxLen int) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, maxLen)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf[:n], nil
}

func (e *DropzoneEngine) addHistory(item DropHistoryItem) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	// Prepend item and keep at most 100 entries
	e.history = append([]DropHistoryItem{item}, e.history...)
	if len(e.history) > 100 {
		e.history = e.history[:100]
	}
}

// GetHistory returns the recent upload and drop history items.
func (e *DropzoneEngine) GetHistory() []DropHistoryItem {
	e.historyMu.RLock()
	defer e.historyMu.RUnlock()

	res := make([]DropHistoryItem, len(e.history))
	copy(res, e.history)
	return res
}

// triggerLibraryRefresh initiates an asynchronous webhook refresh to the active
// media service within 5 seconds. If the service is inactive, it logs info and succeeds.
func (e *DropzoneEngine) triggerLibraryRefresh(category string) {
	if e.refresher == nil {
		return
	}

	serviceID := ServiceForCategory(category)
	if serviceID == "" || serviceID == supervisor.ServicePaperless {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.refresher.RefreshService(ctx, serviceID); err != nil {
			if errors.Is(err, supervisor.ErrServiceInactive) || errors.Is(err, supervisor.ErrInactiveService) || errors.Is(err, supervisor.ErrServiceNotFound) {
				e.logger.Info("media service is inactive; skipping library scan webhook", "service", serviceID, "category", category)
			} else {
				e.logger.Warn("media service library refresh warning", "service", serviceID, "category", category, "error", err)
			}
		} else {
			e.logger.Info("dispatched library refresh webhook", "service", serviceID, "category", category)
		}
	}()
}

// ScanOnce performs a single scan across all /data/drop/incoming/:category directories.
// Promotes stable verified files to /data/media/:category, unlinks invalid files,
// handles zero-byte files, and returns promoted items.
func (e *DropzoneEngine) ScanOnce(ctx context.Context) ([]DropHistoryItem, error) {
	e.scannerMu.Lock()
	defer e.scannerMu.Unlock()

	var promotedItems []DropHistoryItem
	now := time.Now()
	activePaths := make(map[string]bool)

	for _, category := range ValidCategories {
		incomingDir := filepath.Join(e.dataDir, "drop", "incoming", category)
		entries, err := os.ReadDir(incomingDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			e.logger.Warn("failed to read incoming drop directory", "category", category, "dir", incomingDir, "error", err)
			continue
		}

		var targetDir string
		if category == CategoryDocuments {
			targetDir = filepath.Join(e.dataDir, "documents", "consume")
		} else {
			targetDir = filepath.Join(e.dataDir, "media", category)
		}
		_ = os.MkdirAll(targetDir, 0755)

		for _, entry := range entries {
			if ctx.Err() != nil {
				return promotedItems, ctx.Err()
			}

			// Ignore in-flight uploads (.uploading-*) and hidden files
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if entry.IsDir() {
				continue
			}

			filePath := filepath.Join(incomingDir, name)
			activePaths[filePath] = true

			info, statErr := entry.Info()
			if statErr != nil {
				continue
			}

			size := info.Size()
			modTime := info.ModTime()

			tracker, exists := e.stabilityTracker[filePath]
			if !exists {
				// First time seen: record initial stability snapshot
				e.stabilityTracker[filePath] = fileStability{
					lastSize:    size,
					lastModTime: modTime,
					firstSeen:   now,
				}
				continue
			}

			isWindowElapsed := e.stabilityWindow <= 0 || now.Sub(tracker.firstSeen) >= e.stabilityWindow

			// Handle zero-byte files: unlink after stability window to avoid infinite tracking
			if size == 0 {
				if isWindowElapsed && tracker.lastSize == 0 {
					e.logger.Warn("rejected zero-byte file in drop folder; unlinking to keep folder clean", "path", filePath, "category", category)
					_ = os.Remove(filePath)
					delete(e.stabilityTracker, filePath)

					rejectedItem := DropHistoryItem{
						ID:               generateUUID(),
						Filename:         name,
						OriginalFilename: name,
						Category:         category,
						Size:             0,
						ContentType:      "application/octet-stream",
						Path:             filePath,
						Status:           "rejected",
						UploadedAt:       time.Now().UTC().Format(time.RFC3339),
					}
					e.addHistory(rejectedItem)
				}
				continue
			}

			// Check if file size and modtime have stabilized
			isSizeStable := size > 0 && size == tracker.lastSize
			isTimeStable := modTime.Equal(tracker.lastModTime)

			if !isSizeStable || !isTimeStable {
				// Write still in progress: update tracker
				e.stabilityTracker[filePath] = fileStability{
					lastSize:    size,
					lastModTime: modTime,
					firstSeen:   now,
				}
				continue
			}

			if !isWindowElapsed {
				// Stable but waiting for required window
				continue
			}

			// File is stable: inspect magic bytes
			headerBytes, err := readHeaderBytes(filePath, 512)
			if err != nil {
				e.logger.Warn("failed to read dropped file header", "path", filePath, "error", err)
				continue
			}

			var contentType, format string
			if category == CategoryDocuments {
				docType, isDoc := DetectDocument(headerBytes)
				if !isDoc {
					e.logger.Warn("rejected non-document in drop folder; unlinking to keep folder clean", "path", filePath, "category", category)
					_ = os.Remove(filePath)
					delete(e.stabilityTracker, filePath)

					rejectedItem := DropHistoryItem{
						ID:               generateUUID(),
						Filename:         name,
						OriginalFilename: name,
						Category:         category,
						Size:             size,
						ContentType:      "application/octet-stream",
						Path:             filePath,
						Status:           "rejected",
						UploadedAt:       time.Now().UTC().Format(time.RFC3339),
					}
					e.addHistory(rejectedItem)
					continue
				}
				contentType = docType
				format = strings.TrimPrefix(filepath.Ext(name), ".")
			} else {
				ct, fmtSlug, isMedia := DetectMedia(headerBytes)
				if !isMedia || !IsCategoryCompatible(category, ct) {
					reason := "non-media file"
					if isMedia && !IsCategoryCompatible(category, ct) {
						reason = "category mismatch"
					}
					e.logger.Warn("rejected file in drop folder; unlinking to keep folder clean", "path", filePath, "category", category, "reason", reason)
					_ = os.Remove(filePath)
					delete(e.stabilityTracker, filePath)

					rejectedItem := DropHistoryItem{
						ID:               generateUUID(),
						Filename:         name,
						OriginalFilename: name,
						Category:         category,
						Size:             size,
						ContentType:      ct,
						Path:             filePath,
						Status:           "rejected",
						UploadedAt:       time.Now().UTC().Format(time.RFC3339),
					}
					e.addHistory(rejectedItem)
					continue
				}
				contentType = ct
				format = fmtSlug
			}

			// Pre-flight capacity check: verify target storage has size + 1.0 GB safety buffer
			if capErr := e.CheckCapacity(size); capErr != nil {
				e.logger.Warn("insufficient storage to promote dropped file; preserving file in incoming drop until space is freed",
					"path", filePath, "category", category, "size", size)
				continue
			}

			// Valid media: promote atomically with collision resolution & mutex
			e.promotionMu.Lock()
			finalFilename := ResolveTargetFilename(targetDir, name)
			destPath := filepath.Join(targetDir, finalFilename)

			renameErr := atomicMoveOrCopy(filePath, destPath)
			e.promotionMu.Unlock()

			if renameErr != nil {
				e.logger.Error("failed to move dropped file to destination directory", "from", filePath, "to", destPath, "error", renameErr)
				continue
			}

			_ = os.Chmod(destPath, 0644)

			delete(e.stabilityTracker, filePath)

			item := DropHistoryItem{
				ID:               generateUUID(),
				Filename:         finalFilename,
				OriginalFilename: name,
				Category:         category,
				Size:             size,
				ContentType:      contentType,
				Path:             destPath,
				Status:           "promoted",
				UploadedAt:       time.Now().UTC().Format(time.RFC3339),
			}
			e.addHistory(item)
			promotedItems = append(promotedItems, item)
			e.logger.Info("SMB dropped file successfully promoted", "category", category, "filename", finalFilename, "format", format, "size", size)

			// Trigger library refresh
			e.triggerLibraryRefresh(category)
		}
	}

	// Clean up stale entries for deleted files
	for p := range e.stabilityTracker {
		if !activePaths[p] {
			delete(e.stabilityTracker, p)
		}
	}

	return promotedItems, nil
}

// Start launches the background folder scanner routine.
func (e *DropzoneEngine) Start(ctx context.Context) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.scanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopChan:
				return
			case <-ticker.C:
				if _, err := e.ScanOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
					e.logger.Debug("dropzone scan pass completed with notice", "error", err)
				}
			}
		}
	}()
}

// Close gracefully terminates the dropzone engine and background scanner.
func (e *DropzoneEngine) Close() error {
	e.closeOnce.Do(func() {
		close(e.stopChan)
	})
	e.wg.Wait()
	return nil
}
