package enrichment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testRefresher struct {
	mu        sync.Mutex
	calls     []string
	returnErr error
}

func (r *testRefresher) RefreshService(ctx context.Context, serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, serviceID)
	return r.returnErr
}

func (r *testRefresher) getCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]string, len(r.calls))
	copy(copied, r.calls)
	return copied
}

func TestEnrichmentEngine_MovieRip(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	moviesDir := filepath.Join(tmpDir, "movies")
	tvDir := filepath.Join(tmpDir, "tv")
	musicDir := filepath.Join(tmpDir, "music")

	// Create staged movie disc
	stagedMovie := filepath.Join(stagingDir, "THE_MATRIX_1999")
	if err := os.MkdirAll(stagedMovie, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedMovie, "disc.iso"), []byte("matrix-iso-content"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	mockClient.MockMovie = &MovieMetadata{
		Title:       "The Matrix",
		Year:        1999,
		Overview:    "A computer hacker learns from mysterious rebels about the true nature of his reality.",
		Genres:      []string{"Action", "Sci-Fi"},
		PosterBytes: []byte("matrix-poster-bytes"),
	}

	refresher := &testRefresher{}
	var broadcastEvents []string
	var bMu sync.Mutex

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MoviesDir:  moviesDir,
		TVDir:      tvDir,
		MusicDir:   musicDir,
		Client:     mockClient,
		Refresher:  refresher,
		Broadcaster: func(event string, data any) {
			bMu.Lock()
			broadcastEvents = append(broadcastEvents, event)
			bMu.Unlock()
		},
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil // 50% free headroom
		},
	})
	if err != nil {
		t.Fatalf("failed to create EnrichmentEngine: %v", err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("engine.Scan failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed disc, got %d", processed)
	}

	// Verify target movie library structure: /data/media/movies/The Matrix (1999)/
	expectedTarget := filepath.Join(moviesDir, "The Matrix (1999)")
	if _, err := os.Stat(expectedTarget); os.IsNotExist(err) {
		t.Fatalf("expected movie target folder %q does not exist", expectedTarget)
	}

	// Verify media file moved
	movedMedia := filepath.Join(expectedTarget, "disc.iso")
	content, err := os.ReadFile(movedMedia)
	if err != nil {
		t.Fatalf("expected moved disc.iso in target: %v", err)
	}
	if string(content) != "matrix-iso-content" {
		t.Errorf("unexpected content in moved disc.iso: %s", string(content))
	}

	// Verify movie.nfo generated
	nfoPath := filepath.Join(expectedTarget, "movie.nfo")
	nfoBytes, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatalf("expected movie.nfo: %v", err)
	}
	if !strings.Contains(string(nfoBytes), "<title>The Matrix</title>") ||
		!strings.Contains(string(nfoBytes), "<year>1999</year>") ||
		!strings.Contains(string(nfoBytes), "<genre>Action</genre>") {
		t.Errorf("movie.nfo content missing expected XML elements: %s", string(nfoBytes))
	}

	// Verify poster.jpg saved
	posterPath := filepath.Join(expectedTarget, "poster.jpg")
	posterBytes, err := os.ReadFile(posterPath)
	if err != nil {
		t.Fatalf("expected poster.jpg: %v", err)
	}
	if string(posterBytes) != "matrix-poster-bytes" {
		t.Errorf("unexpected poster content: %s", string(posterBytes))
	}

	// Verify staging folder cleaned up
	if _, err := os.Stat(stagedMovie); !os.IsNotExist(err) {
		t.Errorf("expected staging folder %q to be deleted, but still exists", stagedMovie)
	}

	// Verify Jellyfin refresh triggered within 5s
	time.Sleep(100 * time.Millisecond)
	calls := refresher.getCalls()
	if len(calls) == 0 || calls[0] != "jellyfin" {
		t.Errorf("expected refresh call to jellyfin, got: %v", calls)
	}
}

func TestEnrichmentEngine_TVRip(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	tvDir := filepath.Join(tmpDir, "tv")

	// Create staged TV disc
	stagedTV := filepath.Join(stagingDir, "BREAKING_BAD_S01D01")
	if err := os.MkdirAll(stagedTV, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedTV, "title01.mkv"), []byte("ep01-content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedTV, "title02.mkv"), []byte("ep02-content"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	mockClient.MockTV = &TVMetadata{
		Title:       "Breaking Bad",
		Season:      1,
		Overview:    "A chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing methamphetamine.",
		Genres:      []string{"Drama", "Crime"},
		PosterBytes: []byte("breaking-bad-poster"),
	}

	refresher := &testRefresher{}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		TVDir:      tvDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("engine.Scan failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed disc, got %d", processed)
	}

	// Verify target TV library structure: /data/media/tv/Breaking Bad/Season 01/
	expectedSeason := filepath.Join(tvDir, "Breaking Bad", "Season 01")
	if _, err := os.Stat(expectedSeason); os.IsNotExist(err) {
		t.Fatalf("expected TV season folder %q does not exist", expectedSeason)
	}

	// Verify episodes moved
	ep1 := filepath.Join(expectedSeason, "title01.mkv")
	ep2 := filepath.Join(expectedSeason, "title02.mkv")
	if _, err := os.Stat(ep1); err != nil {
		t.Errorf("expected ep1 moved: %v", err)
	}
	if _, err := os.Stat(ep2); err != nil {
		t.Errorf("expected ep2 moved: %v", err)
	}

	// Verify tvshow.nfo and poster in show folder
	showRoot := filepath.Join(tvDir, "Breaking Bad")
	nfoPath := filepath.Join(showRoot, "tvshow.nfo")
	if _, err := os.Stat(nfoPath); err != nil {
		t.Errorf("expected tvshow.nfo: %v", err)
	}
	posterPath := filepath.Join(showRoot, "poster.jpg")
	if _, err := os.Stat(posterPath); err != nil {
		t.Errorf("expected poster.jpg: %v", err)
	}

	// Verify staging folder cleaned up
	if _, err := os.Stat(stagedTV); !os.IsNotExist(err) {
		t.Errorf("expected staging folder %q to be deleted", stagedTV)
	}

	// Verify Jellyfin refresh triggered
	time.Sleep(100 * time.Millisecond)
	calls := refresher.getCalls()
	if len(calls) == 0 || calls[0] != "jellyfin" {
		t.Errorf("expected refresh call to jellyfin, got: %v", calls)
	}
}

func TestEnrichmentEngine_MusicRip(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	musicDir := filepath.Join(tmpDir, "music")

	// Create staged audio CD
	stagedCD := filepath.Join(stagingDir, "AUDIO_CD_8F3A")
	if err := os.MkdirAll(stagedCD, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedCD, "track01.cdda"), []byte("track01-audio"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedCD, "track02.cdda"), []byte("track02-audio"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	mockClient.MockMusic = &MusicMetadata{
		Artist:     "Pink Floyd",
		Album:      "The Dark Side of the Moon",
		Year:       1973,
		CoverBytes: []byte("dark-side-cover"),
	}

	refresher := &testRefresher{}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MusicDir:   musicDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("engine.Scan failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed disc, got %d", processed)
	}

	// Verify canonical target: /data/media/music/Pink Floyd/The Dark Side of the Moon (1973)/
	expectedAlbum := filepath.Join(musicDir, "Pink Floyd", "The Dark Side of the Moon (1973)")
	if _, err := os.Stat(expectedAlbum); os.IsNotExist(err) {
		t.Fatalf("expected album target folder %q does not exist", expectedAlbum)
	}

	// Verify audio tracks moved
	t1 := filepath.Join(expectedAlbum, "track01.cdda")
	t2 := filepath.Join(expectedAlbum, "track02.cdda")
	if _, err := os.Stat(t1); err != nil {
		t.Errorf("expected track01 moved: %v", err)
	}
	if _, err := os.Stat(t2); err != nil {
		t.Errorf("expected track02 moved: %v", err)
	}

	// Verify cover.jpg saved
	coverPath := filepath.Join(expectedAlbum, "cover.jpg")
	coverBytes, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("expected cover.jpg: %v", err)
	}
	if string(coverBytes) != "dark-side-cover" {
		t.Errorf("unexpected cover bytes: %s", string(coverBytes))
	}

	// Verify staging folder cleaned up
	if _, err := os.Stat(stagedCD); !os.IsNotExist(err) {
		t.Errorf("expected staging folder %q to be deleted", stagedCD)
	}

	// Verify Navidrome refresh triggered
	time.Sleep(100 * time.Millisecond)
	calls := refresher.getCalls()
	if len(calls) == 0 || calls[0] != "navidrome" {
		t.Errorf("expected refresh call to navidrome, got: %v", calls)
	}
}

func TestEnrichmentEngine_OfflineFallback(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	moviesDir := filepath.Join(tmpDir, "movies")

	stagedMovie := filepath.Join(stagingDir, "GLADIATOR_2000")
	if err := os.MkdirAll(stagedMovie, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedMovie, "disc.iso"), []byte("gladiator-iso"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock client simulates offline / timeout failure
	mockClient := NewMockMetadataClient()
	mockClient.MovieErr = ErrAPIUnavailable

	refresher := &testRefresher{}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MoviesDir:  moviesDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("engine.Scan failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed disc on offline fallback, got %d", processed)
	}

	// Verify fallback target: /data/media/movies/Gladiator (2000)/
	expectedTarget := filepath.Join(moviesDir, "Gladiator (2000)")
	if _, err := os.Stat(expectedTarget); os.IsNotExist(err) {
		t.Fatalf("expected fallback target %q does not exist", expectedTarget)
	}

	// Verify media file preserved and moved
	mediaPath := filepath.Join(expectedTarget, "disc.iso")
	if _, err := os.Stat(mediaPath); err != nil {
		t.Errorf("expected disc.iso in fallback target: %v", err)
	}

	// Verify fallback movie.nfo generated
	nfoPath := filepath.Join(expectedTarget, "movie.nfo")
	nfoBytes, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatalf("expected fallback movie.nfo: %v", err)
	}
	if !strings.Contains(string(nfoBytes), "<title>Gladiator</title>") ||
		!strings.Contains(string(nfoBytes), "<year>2000</year>") {
		t.Errorf("unexpected fallback nfo content: %s", string(nfoBytes))
	}

	// Verify staging folder deleted
	if _, err := os.Stat(stagedMovie); !os.IsNotExist(err) {
		t.Errorf("expected staging folder %q to be deleted", stagedMovie)
	}
}

func TestEnrichmentEngine_TargetDirectoryCollision(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	moviesDir := filepath.Join(tmpDir, "movies")

	// Pre-create target movie directory with an existing disc.iso
	targetDir := filepath.Join(moviesDir, "The Matrix (1999)")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(targetDir, "disc.iso")
	if err := os.WriteFile(existingFile, []byte("original-matrix-disc1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create staged disc with duplicate disc.iso
	stagedMovie := filepath.Join(stagingDir, "THE_MATRIX_1999-disc2")
	if err := os.MkdirAll(stagedMovie, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedMovie, "disc.iso"), []byte("second-matrix-disc2"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	mockClient.MockMovie = &MovieMetadata{
		Title: "The Matrix",
		Year:  1999,
	}

	refresher := &testRefresher{}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MoviesDir:  moviesDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("engine.Scan failed: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed disc, got %d", processed)
	}

	// Verify original file is untouched
	origBytes, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}
	if string(origBytes) != "original-matrix-disc1" {
		t.Errorf("original file was overwritten: %s", string(origBytes))
	}

	// Verify second file was disambiguated without collision (e.g. disc (1).iso)
	disambiguatedFile := filepath.Join(targetDir, "disc (1).iso")
	secondBytes, err := os.ReadFile(disambiguatedFile)
	if err != nil {
		t.Fatalf("expected disambiguated file %q to exist: %v", disambiguatedFile, err)
	}
	if string(secondBytes) != "second-matrix-disc2" {
		t.Errorf("disambiguated file has unexpected content: %s", string(secondBytes))
	}
}

func TestEnrichmentEngine_StorageHeadroomFloor(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	moviesDir := filepath.Join(tmpDir, "movies")

	stagedMovie := filepath.Join(stagingDir, "THE_MATRIX_1999")
	if err := os.MkdirAll(stagedMovie, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedMovie, "disc.iso"), []byte("matrix-iso"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	refresher := &testRefresher{}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MoviesDir:  moviesDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.05, nil // 5% free headroom (< 10% floor)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("expected Scan not to crash: %v", err)
	}
	if processed != 0 {
		t.Errorf("expected 0 processed items when headroom is below floor, got %d", processed)
	}

	// Verify engine status is paused
	status := engine.GetStatus()
	if status.State != StatePaused {
		t.Errorf("expected StatePaused, got %s", status.State)
	}

	// Verify file was NOT moved and staging was preserved
	if _, err := os.Stat(filepath.Join(stagedMovie, "disc.iso")); err != nil {
		t.Errorf("expected staged file to remain untouched: %v", err)
	}
}

func TestEnrichmentEngine_RefresherInactiveGraceful(t *testing.T) {
	tmpDir := t.TempDir()
	stagingDir := filepath.Join(tmpDir, "staging")
	moviesDir := filepath.Join(tmpDir, "movies")

	stagedMovie := filepath.Join(stagingDir, "THE_MATRIX_1999")
	if err := os.MkdirAll(stagedMovie, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedMovie, "disc.iso"), []byte("matrix-iso"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := NewMockMetadataClient()
	mockClient.MockMovie = &MovieMetadata{Title: "The Matrix", Year: 1999}

	// Refresher returns error (e.g. service inactive)
	refresher := &testRefresher{returnErr: errors.New("service inactive")}

	engine, err := NewEnrichmentEngine(EngineConfig{
		DataDir:    tmpDir,
		StagingDir: stagingDir,
		MoviesDir:  moviesDir,
		Client:     mockClient,
		Refresher:  refresher,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should not fail promotion even if refresher fails
	processed, err := engine.Scan(context.Background())
	if err != nil {
		t.Fatalf("expected promotion to succeed despite inactive refresher: %v", err)
	}
	if processed != 1 {
		t.Errorf("expected 1 processed item, got %d", processed)
	}

	// Wait briefly for goroutine
	time.Sleep(50 * time.Millisecond)
	calls := refresher.getCalls()
	if len(calls) == 0 {
		t.Errorf("expected refresh attempt")
	}
}
