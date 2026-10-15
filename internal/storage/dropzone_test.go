package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/supervisor"
)

type mockRefresher struct {
	mu        sync.Mutex
	refreshed []string
	err       error
}

func (m *mockRefresher) RefreshService(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshed = append(m.refreshed, serviceID)
	return m.err
}

func (m *mockRefresher) getRefreshed() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]string, len(m.refreshed))
	copy(res, m.refreshed)
	return res
}

func TestDropzone_DetectMedia_ValidFormats(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		wantFormat  string
		wantType    string
		expectMedia bool
	}{
		{
			name:        "FLAC audio",
			data:        []byte("fLaC\x00\x00\x00\x22"),
			wantFormat:  "flac",
			wantType:    "audio/flac",
			expectMedia: true,
		},
		{
			name:        "OGG Vorbis",
			data:        append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"), []byte("\x01vorbis")...),
			wantFormat:  "ogg",
			wantType:    "audio/ogg",
			expectMedia: true,
		},
		{
			name:        "OGG Opus",
			data:        append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"), []byte("OpusHead")...),
			wantFormat:  "opus",
			wantType:    "audio/opus",
			expectMedia: true,
		},
		{
			name:        "MKV Matroska",
			data:        []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\xF7\x81\x01\x42\xF2\x81\x04\x42\xF3\x81\x08\x42\x82\x88matroska"),
			wantFormat:  "mkv",
			wantType:    "video/x-matroska",
			expectMedia: true,
		},
		{
			name:        "WebM Matroska",
			data:        []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\xF7\x81\x01\x42\xF2\x81\x04\x42\xF3\x81\x08\x42\x82\x84webm"),
			wantFormat:  "webm",
			wantType:    "video/webm",
			expectMedia: true,
		},
		{
			name:        "AVI Video",
			data:        []byte("RIFF\x24\x00\x00\x00AVI LIST\x00\x00\x00\x00"),
			wantFormat:  "avi",
			wantType:    "video/x-msvideo",
			expectMedia: true,
		},
		{
			name:        "WAV Audio",
			data:        []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00"),
			wantFormat:  "wav",
			wantType:    "audio/wav",
			expectMedia: true,
		},
		{
			name:        "MP4 Video (isom)",
			data:        []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41"),
			wantFormat:  "mp4",
			wantType:    "video/mp4",
			expectMedia: true,
		},
		{
			name:        "M4A Audio",
			data:        []byte("\x00\x00\x00\x20ftypM4A \x00\x00\x00\x00M4A mp42isom"),
			wantFormat:  "m4a",
			wantType:    "audio/mp4",
			expectMedia: true,
		},
		{
			name:        "M4B Audiobook",
			data:        []byte("\x00\x00\x00\x20ftypM4B \x00\x00\x00\x00M4B mp42isom"),
			wantFormat:  "m4b",
			wantType:    "audio/mp4",
			expectMedia: true,
		},
		{
			name:        "MP3 with ID3v2 tag",
			data:        []byte("ID3\x04\x00\x00\x00\x00\x02\x00"),
			wantFormat:  "mp3",
			wantType:    "audio/mpeg",
			expectMedia: true,
		},
		{
			name:        "MP3 raw frame sync (Layer III)",
			data:        []byte("\xFF\xFB\x90\x44"),
			wantFormat:  "mp3",
			wantType:    "audio/mpeg",
			expectMedia: true,
		},
		{
			name:        "AAC ADTS frame",
			data:        []byte("\xFF\xF1\x50\x80"),
			wantFormat:  "aac",
			wantType:    "audio/aac",
			expectMedia: true,
		},
		{
			name: "MPEG-TS Video",
			data: func() []byte {
				buf := make([]byte, 376)
				buf[0] = 0x47
				buf[188] = 0x47
				return buf
			}(),
			wantFormat:  "ts",
			wantType:    "video/mp2t",
			expectMedia: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cType, fmt, ok := DetectMedia(tc.data)
			if ok != tc.expectMedia {
				t.Fatalf("DetectMedia(%s) ok = %v, want %v", tc.name, ok, tc.expectMedia)
			}
			if tc.expectMedia {
				if fmt != tc.wantFormat {
					t.Errorf("DetectMedia(%s) format = %q, want %q", tc.name, fmt, tc.wantFormat)
				}
				if cType != tc.wantType {
					t.Errorf("DetectMedia(%s) contentType = %q, want %q", tc.name, cType, tc.wantType)
				}
			}
		})
	}
}

func TestDropzone_DetectMedia_RejectedNonMedia(t *testing.T) {
	rejected := []struct {
		name string
		data []byte
	}{
		{
			name: "Windows EXE (MZ)",
			data: []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xFF\xFF"),
		},
		{
			name: "ASCII Text document",
			data: []byte("General report: This is a plain text file without media magic bytes."),
		},
		{
			name: "ZIP archive",
			data: []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"),
		},
		{
			name: "Linux ELF executable",
			data: []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
		},
		{
			name: "PDF document",
			data: []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"),
		},
		{
			name: "Empty bytes",
			data: []byte{},
		},
		{
			name: "Arbitrary short noise",
			data: []byte{0x00, 0x11, 0x22},
		},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := DetectMedia(tc.data)
			if ok {
				t.Fatalf("expected non-media payload %q to be rejected, but got ok=true", tc.name)
			}
		})
	}
}

func TestDropzone_ResolveTargetFilename(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Initial file does not exist
	f1 := ResolveTargetFilename(tmpDir, "movie.mp4")
	if f1 != "movie.mp4" {
		t.Fatalf("expected movie.mp4, got %s", f1)
	}

	// Create movie.mp4
	if err := os.WriteFile(filepath.Join(tmpDir, "movie.mp4"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Collision: should become movie (1).mp4
	f2 := ResolveTargetFilename(tmpDir, "movie.mp4")
	if f2 != "movie (1).mp4" {
		t.Fatalf("expected 'movie (1).mp4', got %s", f2)
	}

	// Create movie (1).mp4
	if err := os.WriteFile(filepath.Join(tmpDir, "movie (1).mp4"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Next collision: should become movie (2).mp4
	f3 := ResolveTargetFilename(tmpDir, "movie.mp4")
	if f3 != "movie (2).mp4" {
		t.Fatalf("expected 'movie (2).mp4', got %s", f3)
	}
}

func TestDropzone_HandleUploadStream_Success(t *testing.T) {
	tmpDir := t.TempDir()
	refresher := &mockRefresher{}

	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:   tmpDir,
		Refresher: refresher,
	})
	if err != nil {
		t.Fatalf("NewDropzoneEngine failed: %v", err)
	}
	defer engine.Close()

	// Valid MP3 payload with ID3v2 tag
	mp3Payload := append([]byte("ID3\x04\x00\x00\x00\x00\x02\x00"), bytes.Repeat([]byte("\xFF\xFB\x90\x44"), 50)...)

	ctx := context.Background()
	item, err := engine.HandleUploadStream(ctx, CategoryMusic, "track.mp3", bytes.NewReader(mp3Payload))
	if err != nil {
		t.Fatalf("HandleUploadStream failed: %v", err)
	}

	if item.Filename != "track.mp3" {
		t.Errorf("expected filename track.mp3, got %s", item.Filename)
	}
	if item.Category != CategoryMusic {
		t.Errorf("expected category music, got %s", item.Category)
	}
	if item.Status != "promoted" {
		t.Errorf("expected status promoted, got %s", item.Status)
	}

	// Verify file promoted to /data/media/music/track.mp3
	destPath := filepath.Join(tmpDir, "media", "music", "track.mp3")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("promoted file does not exist at %s: %v", destPath, err)
	}

	// Verify no staging files left in /data/drop/incoming/music
	stagingDir := filepath.Join(tmpDir, "drop", "incoming", "music")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("unexpected file in staging directory: %s", entry.Name())
	}

	// Verify library refresh triggered for navidrome within 5s
	time.Sleep(100 * time.Millisecond)
	refreshed := refresher.getRefreshed()
	if len(refreshed) != 1 || refreshed[0] != supervisor.ServiceNavidrome {
		t.Errorf("expected refresh dispatched for navidrome, got: %v", refreshed)
	}

	// Verify history item
	hist := engine.GetHistory()
	if len(hist) != 1 || hist[0].Filename != "track.mp3" {
		t.Errorf("unexpected history: %+v", hist)
	}
}

func TestDropzone_HandleUploadStream_RejectedNonMediaCleaned(t *testing.T) {
	tmpDir := t.TempDir()
	refresher := &mockRefresher{}

	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:   tmpDir,
		Refresher: refresher,
	})
	if err != nil {
		t.Fatalf("NewDropzoneEngine failed: %v", err)
	}
	defer engine.Close()

	// Text file payload
	txtPayload := []byte("This is plain text and definitely not a video or audio stream.")

	ctx := context.Background()
	item, err := engine.HandleUploadStream(ctx, CategoryMovies, "script.txt", bytes.NewReader(txtPayload))
	if !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("expected ErrInvalidMedia, got item=%v, err=%v", item, err)
	}

	// Verify file was NOT promoted to /data/media/movies
	destPath := filepath.Join(tmpDir, "media", "movies", "script.txt")
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("rejected file should not exist at %s", destPath)
	}

	// Verify staging file was cleanly unlinked
	stagingDir := filepath.Join(tmpDir, "drop", "incoming", "movies")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("staging file was not cleaned up: %s", entry.Name())
	}

	// Verify refresher was not invoked
	if len(refresher.getRefreshed()) > 0 {
		t.Errorf("refresher should not be called on rejected upload")
	}
}

func TestDropzone_HandleUploadStream_CollisionPreservesBoth(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	flacPayload := append([]byte("fLaC\x00\x00\x00\x22"), bytes.Repeat([]byte{0x01}, 100)...)

	ctx := context.Background()
	// First upload
	item1, err := engine.HandleUploadStream(ctx, CategoryMusic, "song.flac", bytes.NewReader(flacPayload))
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	if item1.Filename != "song.flac" {
		t.Fatalf("expected song.flac, got %s", item1.Filename)
	}

	// Second upload with same name
	item2, err := engine.HandleUploadStream(ctx, CategoryMusic, "song.flac", bytes.NewReader(flacPayload))
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}
	if item2.Filename != "song (1).flac" {
		t.Fatalf("expected song (1).flac, got %s", item2.Filename)
	}

	// Both files must exist on disk
	path1 := filepath.Join(tmpDir, "media", "music", "song.flac")
	path2 := filepath.Join(tmpDir, "media", "music", "song (1).flac")
	if _, err := os.Stat(path1); err != nil {
		t.Errorf("file 1 missing: %v", err)
	}
	if _, err := os.Stat(path2); err != nil {
		t.Errorf("file 2 missing: %v", err)
	}
}

func TestDropzone_ScanOnce_SMBFileDrop(t *testing.T) {
	tmpDir := t.TempDir()
	refresher := &mockRefresher{}

	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:         tmpDir,
		Refresher:       refresher,
		StabilityWindow: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Simulate user copying track.flac to /data/drop/incoming/music/track.flac
	incomingDir := filepath.Join(tmpDir, "drop", "incoming", "music")
	_ = os.MkdirAll(incomingDir, 0755)
	dropFile := filepath.Join(incomingDir, "track.flac")

	flacPayload := append([]byte("fLaC\x00\x00\x00\x22"), bytes.Repeat([]byte{0x42}, 100)...)
	if err := os.WriteFile(dropFile, flacPayload, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Pass 1: detects file, records stability baseline
	promoted, err := engine.ScanOnce(ctx)
	if err != nil {
		t.Fatalf("ScanOnce pass 1 failed: %v", err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected 0 promoted on first pass before stability window, got %d", len(promoted))
	}

	// Wait stability window
	time.Sleep(20 * time.Millisecond)

	// Pass 2: stable file detected, validated, promoted
	promoted, err = engine.ScanOnce(ctx)
	if err != nil {
		t.Fatalf("ScanOnce pass 2 failed: %v", err)
	}
	if len(promoted) != 1 {
		t.Fatalf("expected 1 promoted item, got %d", len(promoted))
	}
	if promoted[0].Filename != "track.flac" {
		t.Errorf("expected track.flac, got %s", promoted[0].Filename)
	}

	// Verify moved to /data/media/music/track.flac
	targetPath := filepath.Join(tmpDir, "media", "music", "track.flac")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("target file not found: %v", err)
	}

	// Verify drop directory is clean
	if _, err := os.Stat(dropFile); !os.IsNotExist(err) {
		t.Fatalf("drop file was not moved from %s", dropFile)
	}

	// Verify library refresh triggered
	time.Sleep(100 * time.Millisecond)
	if len(refresher.getRefreshed()) != 1 || refresher.getRefreshed()[0] != supervisor.ServiceNavidrome {
		t.Errorf("expected refresh on navidrome, got: %v", refresher.getRefreshed())
	}
}

func TestDropzone_ScanOnce_RejectsNonMediaAndCleans(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:         tmpDir,
		StabilityWindow: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// User drops a text file into /data/drop/incoming/movies/notes.txt
	dropPath := filepath.Join(tmpDir, "drop", "incoming", "movies", "notes.txt")
	if err := os.WriteFile(dropPath, []byte("Some notes that are not video"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, _ = engine.ScanOnce(ctx)
	time.Sleep(20 * time.Millisecond)
	promoted, err := engine.ScanOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected 0 promoted items for non-media, got %d", len(promoted))
	}

	// Verify file was unlinked from drop directory to keep it clean
	if _, err := os.Stat(dropPath); !os.IsNotExist(err) {
		t.Fatalf("invalid drop file was not unlinked from %s", dropPath)
	}
}

func TestDropzone_ServiceForCategory(t *testing.T) {
	tests := []struct {
		cat      string
		expected string
	}{
		{CategoryMovies, supervisor.ServiceJellyfin},
		{CategoryTV, supervisor.ServiceJellyfin},
		{CategoryMusic, supervisor.ServiceNavidrome},
		{CategoryAudiobooks, supervisor.ServiceAudiobookshelf},
		{"unknown", ""},
		{"", ""},
	}

	for _, tc := range tests {
		got := ServiceForCategory(tc.cat)
		if got != tc.expected {
			t.Errorf("ServiceForCategory(%q) = %q, want %q", tc.cat, got, tc.expected)
		}
	}
}

func TestDropzone_CleanOrphanedStagingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := EnsureDropzoneDirectories(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create orphaned staging files
	orphanMovies := filepath.Join(tmpDir, "drop", "incoming", CategoryMovies, ".uploading-1234-movie.mp4")
	orphanMusic := filepath.Join(tmpDir, "drop", "incoming", CategoryMusic, ".uploading-5678-song.mp3")
	legitTv := filepath.Join(tmpDir, "drop", "incoming", CategoryTV, "legit-show.mkv")

	_ = os.WriteFile(orphanMovies, []byte("partial"), 0644)
	_ = os.WriteFile(orphanMusic, []byte("partial"), 0644)
	_ = os.WriteFile(legitTv, []byte("full"), 0644)

	if err := CleanOrphanedStagingFiles(tmpDir); err != nil {
		t.Fatalf("CleanOrphanedStagingFiles failed: %v", err)
	}

	if _, err := os.Stat(orphanMovies); !os.IsNotExist(err) {
		t.Errorf("orphaned staging file %s was not cleaned up", orphanMovies)
	}
	if _, err := os.Stat(orphanMusic); !os.IsNotExist(err) {
		t.Errorf("orphaned staging file %s was not cleaned up", orphanMusic)
	}
	if _, err := os.Stat(legitTv); err != nil {
		t.Errorf("legitimate file %s was mistakenly cleaned up", legitTv)
	}
}

func TestDropzone_ScanOnce_ZeroByteRejection(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:         tmpDir,
		StabilityWindow: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	zeroFile := filepath.Join(tmpDir, "drop", "incoming", CategoryMovies, "empty.mp4")
	if err := os.WriteFile(zeroFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// Pass 1: seen and recorded
	promoted, err := engine.ScanOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected 0 promoted on first pass, got %d", len(promoted))
	}

	time.Sleep(20 * time.Millisecond)

	// Pass 2: stable at 0 bytes -> unlinked and rejected
	promoted, err = engine.ScanOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected 0 promoted for zero-byte file, got %d", len(promoted))
	}

	if _, err := os.Stat(zeroFile); !os.IsNotExist(err) {
		t.Errorf("expected zero-byte file to be unlinked, but still exists: %s", zeroFile)
	}

	history := engine.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Status != "rejected" || history[0].Filename != "empty.mp4" {
		t.Errorf("unexpected history entry: %+v", history[0])
	}
}

func TestDropzone_HandleUploadStream_CategoryMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. Audio uploaded to Movies category
	mp3Payload := append([]byte("ID3\x04\x00\x00\x00\x00\x02\x00"), bytes.Repeat([]byte("\xFF\xFB\x90\x44"), 20)...)
	_, err = engine.HandleUploadStream(ctx, CategoryMovies, "audio_in_movies.mp3", bytes.NewReader(mp3Payload))
	if !errors.Is(err, ErrCategoryMismatch) {
		t.Errorf("expected ErrCategoryMismatch, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "media", CategoryMovies, "audio_in_movies.mp3")); !os.IsNotExist(statErr) {
		t.Errorf("category mismatch file should not exist in media directory")
	}

	// 2. Video uploaded to Music category
	mp4Payload := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41" + strings.Repeat("A", 50))
	_, err = engine.HandleUploadStream(ctx, CategoryMusic, "video_in_music.mp4", bytes.NewReader(mp4Payload))
	if !errors.Is(err, ErrCategoryMismatch) {
		t.Errorf("expected ErrCategoryMismatch, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "media", CategoryMusic, "video_in_music.mp4")); !os.IsNotExist(statErr) {
		t.Errorf("category mismatch file should not exist in media directory")
	}
}

func TestDropzone_ScanOnce_CategoryMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:         tmpDir,
		StabilityWindow: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Drop audio file into incoming movies directory
	mp3Payload := append([]byte("ID3\x04\x00\x00\x00\x00\x02\x00"), bytes.Repeat([]byte("\xFF\xFB\x90\x44"), 20)...)
	dropPath := filepath.Join(tmpDir, "drop", "incoming", CategoryMovies, "misplaced_song.mp3")
	if err := os.WriteFile(dropPath, mp3Payload, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, _ = engine.ScanOnce(ctx)
	time.Sleep(20 * time.Millisecond)

	promoted, err := engine.ScanOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected 0 promoted items for category mismatch, got %d", len(promoted))
	}

	// Verify unlinked from drop folder
	if _, err := os.Stat(dropPath); !os.IsNotExist(err) {
		t.Fatalf("category mismatch drop file was not unlinked from %s", dropPath)
	}

	// Verify recorded in history as rejected
	hist := engine.GetHistory()
	if len(hist) != 1 || hist[0].Status != "rejected" {
		t.Errorf("expected rejected history entry, got: %+v", hist)
	}
}

type cancelingReader struct {
	data   []byte
	cancel context.CancelFunc
	pos    int
}

func (cr *cancelingReader) Read(p []byte) (n int, err error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	n = copy(p, cr.data[cr.pos:])
	cr.pos += n
	// Cancel context after first read
	if cr.cancel != nil {
		cr.cancel()
	}
	return n, nil
}

func TestDropzone_HandleUploadStream_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data := bytes.Repeat([]byte("some data bytes for upload"), 100)
	reader := &cancelingReader{
		data:   data,
		cancel: cancel,
	}

	_, err = engine.HandleUploadStream(ctx, CategoryMovies, "canceled.mp4", reader)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	// Verify no staging files left
	stagingDir := filepath.Join(tmpDir, "drop", "incoming", CategoryMovies)
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("staging file left behind after cancellation: %s", entry.Name())
	}
}

func TestDropzone_Start_BackgroundScanner(t *testing.T) {
	tmpDir := t.TempDir()
	refresher := &mockRefresher{}
	engine, err := NewDropzoneEngine(DropzoneConfig{
		DataDir:         tmpDir,
		Refresher:       refresher,
		ScanInterval:    10 * time.Millisecond,
		StabilityWindow: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.Start(ctx)

	// Drop a valid FLAC audio file into music drop folder
	flacPayload := append([]byte("fLaC\x00\x00\x00\x22"), bytes.Repeat([]byte{0x55}, 100)...)
	dropPath := filepath.Join(tmpDir, "drop", "incoming", CategoryMusic, "ambient.flac")
	if err := os.WriteFile(dropPath, flacPayload, 0644); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(tmpDir, "media", CategoryMusic, "ambient.flac")
	promoted := false

	// Wait up to 1 second for background scanner to process and promote
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if _, err := os.Stat(targetPath); err == nil {
			promoted = true
			break
		}
	}

	if !promoted {
		t.Fatalf("file was not promoted by background scanner to %s within timeout", targetPath)
	}

	// Verify incoming file was removed
	if _, err := os.Stat(dropPath); !os.IsNotExist(err) {
		t.Errorf("dropped file still exists at %s", dropPath)
	}

	// Verify refresher called
	time.Sleep(50 * time.Millisecond)
	if len(refresher.getRefreshed()) != 1 || refresher.getRefreshed()[0] != supervisor.ServiceNavidrome {
		t.Errorf("expected refresh on navidrome, got: %v", refresher.getRefreshed())
	}
}

