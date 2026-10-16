package enrichment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	StateIdle      = "idle"
	StateEnriching = "enriching"
	StatePaused    = "paused"
	StateError     = "error"

	StatusTextIdle      = "Idle"
	StatusTextMatching  = "Matching metadata"
	StatusTextPausedLow = "Storage low: Enrichment paused"

	StorageHeadroomFloor = 0.10 // 10% disk capacity threshold
)

var (
	ErrStorageLow = errors.New("insufficient storage: free capacity is below 10% safety threshold")
)

// LibraryRefresher defines the interface for triggering media library refresh webhooks.
type LibraryRefresher interface {
	RefreshService(ctx context.Context, serviceID string) error
}

// EngineStatus describes the live state of the metadata enrichment engine.
type EngineStatus struct {
	State          string    `json:"state"`
	StatusText     string    `json:"status_text"`
	CurrentDisc    string    `json:"current_disc,omitempty"`
	Progress       float64   `json:"progress"`
	LastScannedAt  time.Time `json:"last_scanned_at"`
	ItemsProcessed int       `json:"items_processed"`
	LastError      string    `json:"last_error,omitempty"`
}

// EngineConfig holds configuration parameters for EnrichmentEngine.
type EngineConfig struct {
	DataDir         string
	StagingDir      string
	MoviesDir       string
	TVDir           string
	MusicDir        string
	Client          MetadataClient
	Refresher       LibraryRefresher
	Broadcaster     func(event string, data any)
	HeadroomChecker func(path string) (float64, error)
	PollInterval    time.Duration
	Logger          *slog.Logger
}

// EnrichmentEngine monitors disc staging, enriches metadata, downloads artwork,
// promotes media into canonical library structures, and dispatches refresh triggers.
type EnrichmentEngine struct {
	stagingDir      string
	moviesDir       string
	tvDir           string
	musicDir        string
	client          MetadataClient
	refresher       LibraryRefresher
	broadcaster     func(event string, data any)
	headroomChecker func(path string) (float64, error)
	pollInterval    time.Duration
	logger          *slog.Logger

	mu        sync.RWMutex
	scanMu    sync.Mutex
	status    EngineStatus
	stopChan  chan struct{}
	closeOnce sync.Once
}

// NewEnrichmentEngine initializes and returns an EnrichmentEngine.
func NewEnrichmentEngine(cfg EngineConfig) (*EnrichmentEngine, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "/data"
	}

	stagingDir := cfg.StagingDir
	if stagingDir == "" {
		stagingDir = filepath.Join(dataDir, "media", "staging", "discs")
	}

	moviesDir := cfg.MoviesDir
	if moviesDir == "" {
		moviesDir = filepath.Join(dataDir, "media", "movies")
	}

	tvDir := cfg.TVDir
	if tvDir == "" {
		tvDir = filepath.Join(dataDir, "media", "tv")
	}

	musicDir := cfg.MusicDir
	if musicDir == "" {
		musicDir = filepath.Join(dataDir, "media", "music")
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}

	client := cfg.Client
	if client == nil {
		client = NewHTTPMetadataClient(HTTPMetadataConfig{})
	}

	headroomChecker := cfg.HeadroomChecker
	if headroomChecker == nil {
		headroomChecker = defaultHeadroomChecker
	}

	// Ensure directories exist
	_ = os.MkdirAll(stagingDir, 0755)
	_ = os.MkdirAll(moviesDir, 0755)
	_ = os.MkdirAll(tvDir, 0755)
	_ = os.MkdirAll(musicDir, 0755)

	e := &EnrichmentEngine{
		stagingDir:      stagingDir,
		moviesDir:       moviesDir,
		tvDir:           tvDir,
		musicDir:        musicDir,
		client:          client,
		refresher:       cfg.Refresher,
		broadcaster:     cfg.Broadcaster,
		headroomChecker: headroomChecker,
		pollInterval:    pollInterval,
		logger:          cfg.Logger,
		stopChan:        make(chan struct{}),
		status: EngineStatus{
			State:      StateIdle,
			StatusText: StatusTextIdle,
		},
	}

	return e, nil
}

// SetRefresher updates the LibraryRefresher implementation dynamically.
func (e *EnrichmentEngine) SetRefresher(refresher LibraryRefresher) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refresher = refresher
}

// SetClient updates the MetadataClient implementation dynamically.
func (e *EnrichmentEngine) SetClient(client MetadataClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.client = client
}

// SetHeadroomChecker overrides the storage headroom check function dynamically.
func (e *EnrichmentEngine) SetHeadroomChecker(checker func(path string) (float64, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.headroomChecker = checker
}

// SetBroadcaster updates the SSE event broadcast callback dynamically.
func (e *EnrichmentEngine) SetBroadcaster(fn func(event string, data any)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.broadcaster = fn
}

// GetStatus returns a snapshot of current enrichment engine state.
func (e *EnrichmentEngine) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// IsEnriching returns true if the engine is actively processing or enriching media.
func (e *EnrichmentEngine) IsEnriching() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status.State == StateEnriching
}

func (e *EnrichmentEngine) setStatus(state, statusText, currentDisc string, progress float64, lastErr string) {
	e.mu.Lock()
	e.status.State = state
	e.status.StatusText = statusText
	e.status.CurrentDisc = currentDisc
	e.status.Progress = progress
	if lastErr != "" {
		e.status.LastError = lastErr
	}
	e.mu.Unlock()

	e.broadcast("enrichment.status", map[string]any{
		"state":        state,
		"status_text":  statusText,
		"current_disc": currentDisc,
		"progress":     progress,
		"last_error":   lastErr,
	})

	if statusText == StatusTextMatching {
		e.broadcast("optical.status", map[string]any{
			"state":       state,
			"status_text": StatusTextMatching,
		})
	}
}

func (e *EnrichmentEngine) broadcast(event string, data any) {
	e.mu.RLock()
	fn := e.broadcaster
	e.mu.RUnlock()
	if fn != nil {
		fn(event, data)
	}
}

// Start launches the background directory polling loop.
func (e *EnrichmentEngine) Start(ctx context.Context) {
	go e.runLoop(ctx)
}

// Close gracefully terminates background workers.
func (e *EnrichmentEngine) Close() error {
	e.closeOnce.Do(func() {
		close(e.stopChan)
	})
	return nil
}

func (e *EnrichmentEngine) runLoop(ctx context.Context) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	// Initial scan on boot
	_, _ = e.Scan(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			_, _ = e.Scan(ctx)
		}
	}
}

// Scan inspects the staging directory and enriches all completed disc rips.
func (e *EnrichmentEngine) Scan(ctx context.Context) (int, error) {
	e.scanMu.Lock()
	defer e.scanMu.Unlock()

	e.mu.Lock()
	e.status.LastScannedAt = time.Now().UTC()
	e.mu.Unlock()

	entries, err := os.ReadDir(e.stagingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read staging directory %q: %w", e.stagingDir, err)
	}

	processedCount := 0
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return processedCount, ctx.Err()
		case <-e.stopChan:
			return processedCount, nil
		default:
		}

		if !entry.IsDir() {
			continue
		}

		discDirPath := filepath.Join(e.stagingDir, entry.Name())
		if err := e.enrichDisc(ctx, discDirPath); err != nil {
			e.logger.Error("failed to enrich staged disc", "dir", discDirPath, "error", err)
			e.mu.Lock()
			e.status.LastError = err.Error()
			e.mu.Unlock()
			continue
		}
		processedCount++
	}

	return processedCount, nil
}

// enrichDisc executes the full metadata lookup, artwork retrieval, atomic promotion,
// and refresh dispatch for a single staged disc directory.
func (e *EnrichmentEngine) enrichDisc(ctx context.Context, discDirPath string) error {
	discName := filepath.Base(discDirPath)

	// Step 1: Storage Headroom Check
	freePct, err := e.headroomChecker(e.stagingDir)
	if err != nil {
		e.logger.Warn("storage headroom check failed, assuming sufficient space", "error", err)
	} else if freePct < StorageHeadroomFloor {
		e.setStatus(StatePaused, StatusTextPausedLow, discName, 0.0, ErrStorageLow.Error())
		e.logger.Warn("storage capacity below 10%, pausing enrichment", "free_percent", freePct*100)
		return ErrStorageLow
	}

	// Verify staged folder has content
	files, err := os.ReadDir(discDirPath)
	if err != nil {
		return fmt.Errorf("read disc dir: %w", err)
	}
	if len(files) == 0 {
		// Empty folder, wait or clean up
		return nil
	}

	// Step 2: Live SSE Telemetry - Matching metadata
	e.setStatus(StateEnriching, StatusTextMatching, discName, 10.0, "")

	// Step 3: Parse volume label and file structure
	parsed, err := ParseStagedDisc(discDirPath, "")
	if err != nil {
		e.logger.Warn("error parsing staged disc, using raw label fallback", "error", err)
		parsed = ParseVolumeLabel(discName)
	}

	// Step 4: Metadata Query with bounded context
	metaCtx, metaCancel := context.WithTimeout(ctx, 5*time.Second)
	defer metaCancel()

	var movieMeta *MovieMetadata
	var tvMeta *TVMetadata
	var musicMeta *MusicMetadata
	var artworkBytes []byte

	switch parsed.MediaType {
	case MediaTypeTV:
		tvMeta, err = e.client.FetchTVMetadata(metaCtx, parsed.Title, parsed.Season)
		if err != nil {
			e.logger.Warn("TV metadata query failed; using offline label fallback", "error", err, "title", parsed.Title)
			tvMeta = &TVMetadata{
				Title:    parsed.Title,
				Season:   parsed.Season,
				Overview: fmt.Sprintf("Imported from disc: %s", parsed.RawLabel),
				Genres:   []string{"Unknown"},
			}
		}
		if tvMeta != nil && len(tvMeta.PosterBytes) > 0 {
			artworkBytes = tvMeta.PosterBytes
		} else if tvMeta != nil && tvMeta.PosterURL != "" {
			artworkBytes, _ = e.client.DownloadArtwork(metaCtx, tvMeta.PosterURL)
		}

	case MediaTypeMusic:
		musicMeta, err = e.client.FetchMusicMetadata(metaCtx, parsed.Title, "")
		if err != nil {
			e.logger.Warn("music metadata query failed; using offline label fallback", "error", err, "title", parsed.Title)
			artist := parsed.Artist
			if artist == "" {
				artist = "Unknown Artist"
			}
			musicMeta = &MusicMetadata{
				Artist: artist,
				Album:  parsed.Title,
				Year:   parsed.Year,
				Genres: []string{"Unknown"},
			}
		}
		if musicMeta != nil && len(musicMeta.CoverBytes) > 0 {
			artworkBytes = musicMeta.CoverBytes
		} else if musicMeta != nil && musicMeta.CoverURL != "" {
			artworkBytes, _ = e.client.DownloadArtwork(metaCtx, musicMeta.CoverURL)
		}

	default: // Movie
		movieMeta, err = e.client.FetchMovieMetadata(metaCtx, parsed.Title, parsed.Year)
		if err != nil {
			e.logger.Warn("movie metadata query failed; using offline label fallback", "error", err, "title", parsed.Title)
			movieMeta = &MovieMetadata{
				Title:    parsed.Title,
				Year:     parsed.Year,
				Overview: fmt.Sprintf("Imported from disc: %s", parsed.RawLabel),
				Genres:   []string{"Unknown"},
			}
		}
		if movieMeta != nil && len(movieMeta.PosterBytes) > 0 {
			artworkBytes = movieMeta.PosterBytes
		} else if movieMeta != nil && movieMeta.PosterURL != "" {
			artworkBytes, _ = e.client.DownloadArtwork(metaCtx, movieMeta.PosterURL)
		}
	}

	e.setStatus(StateEnriching, StatusTextMatching, discName, 50.0, "")

	// Step 5: Compute Canonical Target Directory
	var targetDir string
	var serviceToRefresh string

	switch parsed.MediaType {
	case MediaTypeTV:
		showDir := tvMeta.Title
		if showDir == "" {
			showDir = parsed.Title
		}
		seasonFolder := fmt.Sprintf("Season %02d", tvMeta.Season)
		if tvMeta.Season == 0 {
			seasonFolder = "Season 01"
		}
		targetDir = filepath.Join(e.tvDir, showDir, seasonFolder)
		serviceToRefresh = "jellyfin"

	case MediaTypeMusic:
		artist := musicMeta.Artist
		if artist == "" {
			artist = "Unknown Artist"
		}
		albumFolder := musicMeta.Album
		if albumFolder == "" {
			albumFolder = parsed.Title
		}
		if musicMeta.Year > 0 {
			albumFolder = fmt.Sprintf("%s (%d)", albumFolder, musicMeta.Year)
		}
		targetDir = filepath.Join(e.musicDir, artist, albumFolder)
		serviceToRefresh = "navidrome"

	default: // Movie
		movieFolder := movieMeta.Title
		if movieFolder == "" {
			movieFolder = parsed.Title
		}
		if movieMeta.Year > 0 {
			movieFolder = fmt.Sprintf("%s (%d)", movieFolder, movieMeta.Year)
		}
		targetDir = filepath.Join(e.moviesDir, movieFolder)
		serviceToRefresh = "jellyfin"
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target library directory %q: %w", targetDir, err)
	}

	// Step 6: Write Metadata Files (NFO & Artwork)
	if parsed.MediaType == MediaTypeTV {
		nfoContent := formatTVShowNFO(tvMeta)
		showRoot := filepath.Dir(targetDir)
		_ = os.WriteFile(filepath.Join(showRoot, "tvshow.nfo"), []byte(nfoContent), 0644)
		if len(artworkBytes) > 0 {
			_ = os.WriteFile(filepath.Join(targetDir, "poster.jpg"), artworkBytes, 0644)
			_ = os.WriteFile(filepath.Join(showRoot, "poster.jpg"), artworkBytes, 0644)
		}
	} else if parsed.MediaType == MediaTypeMusic {
		if len(artworkBytes) > 0 {
			_ = os.WriteFile(filepath.Join(targetDir, "cover.jpg"), artworkBytes, 0644)
		}
	} else {
		// Movie
		nfoContent := formatMovieNFO(movieMeta)
		_ = os.WriteFile(filepath.Join(targetDir, "movie.nfo"), []byte(nfoContent), 0644)
		if len(artworkBytes) > 0 {
			_ = os.WriteFile(filepath.Join(targetDir, "poster.jpg"), artworkBytes, 0644)
		}
	}

	// Step 7: Atomically Move Media Files with Collision Safeguards
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		srcFile := filepath.Join(discDirPath, f.Name())
		destFile := filepath.Join(targetDir, f.Name())

		// Collision disambiguation: never overwrite existing files in destination
		destFile = disambiguateDestination(destFile)

		if err := moveFile(srcFile, destFile); err != nil {
			return fmt.Errorf("failed to move media file %q to %q: %w", srcFile, destFile, err)
		}
	}

	// Step 8: Clean up staging disc directory
	_ = os.RemoveAll(discDirPath)

	// Step 9: Update completed status & counts
	e.mu.Lock()
	e.status.ItemsProcessed++
	e.mu.Unlock()

	e.setStatus(StateIdle, StatusTextIdle, "", 100.0, "")
	e.broadcast("enrichment.completed", map[string]any{
		"disc_name":  discName,
		"target_dir": targetDir,
		"category":   string(parsed.MediaType),
	})

	// Step 10: Dispatch Library Refresh Webhook within 5 seconds
	go e.dispatchRefresh(serviceToRefresh)

	return nil
}

// dispatchRefresh invokes the configured library refresher within a 5-second timeout window.
func (e *EnrichmentEngine) dispatchRefresh(serviceID string) {
	e.mu.RLock()
	refresher := e.refresher
	e.mu.RUnlock()

	if refresher == nil {
		e.logger.Info("no library refresher configured; skipping webhook trigger", "service", serviceID)
		return
	}

	refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := refresher.RefreshService(refreshCtx, serviceID); err != nil {
		e.logger.Info("library refresh dispatched (service may be inactive or unavailable)", "service", serviceID, "error", err)
	} else {
		e.logger.Info("library refresh dispatched successfully", "service", serviceID)
	}
}

// disambiguateDestination appends numerical counter suffixes if a file already exists at targetPath.
func disambiguateDestination(targetPath string) string {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath
	}

	ext := filepath.Ext(targetPath)
	base := strings.TrimSuffix(targetPath, ext)
	counter := 1
	for {
		candidate := fmt.Sprintf("%s (%d)%s", base, counter, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		counter++
	}
}

// moveFile attempts atomic rename, falling back to copy+sync+delete for cross-device links.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %q: %w", src, err)
	}
	defer in.Close()

	tmpDst := dst + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	out, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create tmpDst %q: %w", tmpDst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmpDst)
		return fmt.Errorf("copy bytes: %w", err)
	}

	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(tmpDst)
		return fmt.Errorf("sync dst: %w", err)
	}
	out.Close()
	in.Close()

	if err := os.Rename(tmpDst, dst); err != nil {
		_ = os.Remove(tmpDst)
		return fmt.Errorf("rename tmpDst to dst: %w", err)
	}

	return os.Remove(src)
}

func formatMovieNFO(m *MovieMetadata) string {
	var genresXML strings.Builder
	for _, g := range m.Genres {
		genresXML.WriteString(fmt.Sprintf("  <genre>%s</genre>\n", escapeXML(g)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
  <title>%s</title>
  <year>%d</year>
  <plot>%s</plot>
%s</movie>
`, escapeXML(m.Title), m.Year, escapeXML(m.Overview), genresXML.String())
}

func formatTVShowNFO(t *TVMetadata) string {
	var genresXML strings.Builder
	for _, g := range t.Genres {
		genresXML.WriteString(fmt.Sprintf("  <genre>%s</genre>\n", escapeXML(g)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<tvshow>
  <title>%s</title>
  <season>%d</season>
  <plot>%s</plot>
%s</tvshow>
`, escapeXML(t.Title), t.Season, escapeXML(t.Overview), genresXML.String())
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func defaultHeadroomChecker(path string) (float64, error) {
	var stat syscall.Statfs_t
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
