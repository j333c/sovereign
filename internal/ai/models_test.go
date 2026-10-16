package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockSettingStore struct {
	mu       sync.RWMutex
	settings map[string]string
}

func newMockSettingStore() *mockSettingStore {
	return &mockSettingStore{
		settings: make(map[string]string),
	}
}

func (s *mockSettingStore) GetSetting(ctx context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings[key], nil
}

func (s *mockSettingStore) SetSetting(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[key] = value
	return nil
}

type mockServiceReloader struct {
	mu             sync.Mutex
	restartedCount int
	lastService    string
}

func (r *mockServiceReloader) ReloadService(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.restartedCount++
	r.lastService = id
	return nil
}

// createValidGGUFBytes creates binary content with a valid GGUF header.
func createValidGGUFBytes(version uint32, tensorCount uint64, kvCount uint64) []byte {
	buf := new(bytes.Buffer)
	buf.Write(GGUFMagic[:])
	_ = binary.Write(buf, binary.LittleEndian, version)
	_ = binary.Write(buf, binary.LittleEndian, tensorCount)
	_ = binary.Write(buf, binary.LittleEndian, kvCount)
	// pad with dummy payload
	buf.WriteString("dummy-gguf-tensor-weights-payload")
	return buf.Bytes()
}

func TestModelManager_ListModelsAndActiveModel(t *testing.T) {
	tmpDir := t.TempDir()
	store := newMockSettingStore()
	reloader := &mockServiceReloader{}

	// Write two valid models and one invalid file
	modelABytes := createValidGGUFBytes(3, 100, 20)
	modelBBytes := createValidGGUFBytes(2, 50, 10)

	if err := os.WriteFile(filepath.Join(tmpDir, "model-a.gguf"), modelABytes, 0644); err != nil {
		t.Fatalf("failed to write model A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "model-b.gguf"), modelBBytes, 0644); err != nil {
		t.Fatalf("failed to write model B: %v", err)
	}
	// invalid non-gguf file
	if err := os.WriteFile(filepath.Join(tmpDir, "corrupt.gguf"), []byte("not a gguf file"), 0644); err != nil {
		t.Fatalf("failed to write corrupt model: %v", err)
	}

	// Create symlink active.gguf -> model-a.gguf
	if err := os.Symlink("model-a.gguf", filepath.Join(tmpDir, "active.gguf")); err != nil {
		t.Fatalf("failed to create active symlink: %v", err)
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IsGPU:        true,
		HardwareMode: "gpu",
	})
	defer arb.Close()

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		Store:      store,
		Reloader:   reloader,
		Arbitrator: arb,
	})

	resp, err := mgr.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if resp.ActiveModel != "model-a.gguf" {
		t.Errorf("expected active model 'model-a.gguf', got %q", resp.ActiveModel)
	}

	// corrupt.gguf should have been filtered out by ScanModels
	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 valid models, got %d", len(resp.Models))
	}

	for _, m := range resp.Models {
		if m.Filename == "model-a.gguf" {
			if !m.IsActive {
				t.Errorf("expected model-a to be active")
			}
			if m.VRAMUsageBytes <= 0 {
				t.Errorf("expected positive VRAM usage on active GPU model, got %d", m.VRAMUsageBytes)
			}
			if m.TokensPerSec <= 0 {
				t.Errorf("expected positive TokensPerSec benchmark, got %f", m.TokensPerSec)
			}
			if !m.FlashAttention {
				t.Errorf("expected FlashAttention to be true")
			}
		} else if m.Filename == "model-b.gguf" {
			if m.IsActive {
				t.Errorf("expected model-b to be inactive")
			}
		}
	}
}

func TestModelManager_SetActiveModel(t *testing.T) {
	tmpDir := t.TempDir()
	store := newMockSettingStore()
	reloader := &mockServiceReloader{}
	arb := NewGPUArbitrator(ArbitratorConfig{
		IsGPU: true,
	})
	defer arb.Close()

	modelABytes := createValidGGUFBytes(3, 100, 20)
	modelBBytes := createValidGGUFBytes(3, 150, 25)

	_ = os.WriteFile(filepath.Join(tmpDir, "model-a.gguf"), modelABytes, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "model-b.gguf"), modelBBytes, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "invalid.gguf"), []byte("invalid content"), 0644)

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		Store:      store,
		Reloader:   reloader,
		Arbitrator: arb,
	})

	ctx := context.Background()

	// 1. Setting invalid model should fail header validation
	if err := mgr.SetActiveModel(ctx, "invalid.gguf"); err == nil {
		t.Errorf("expected error setting invalid model, got nil")
	}

	// 2. Setting non-existent model should fail
	if err := mgr.SetActiveModel(ctx, "missing.gguf"); !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got %v", err)
	}

	// 3. Switch to model-b.gguf
	if err := mgr.SetActiveModel(ctx, "model-b.gguf"); err != nil {
		t.Fatalf("SetActiveModel failed: %v", err)
	}

	// Verify symlink was created pointing to model-b.gguf
	resolved, err := filepath.EvalSymlinks(filepath.Join(tmpDir, "active.gguf"))
	if err != nil {
		t.Fatalf("failed to resolve active.gguf: %v", err)
	}
	if filepath.Base(resolved) != "model-b.gguf" {
		t.Errorf("expected active.gguf to resolve to model-b.gguf, got %s", resolved)
	}

	// Verify store was updated
	settingVal, _ := store.GetSetting(ctx, "ai_active_model")
	if settingVal != "model-b.gguf" {
		t.Errorf("expected store setting to be 'model-b.gguf', got %q", settingVal)
	}

	// Verify reloader was called
	reloader.mu.Lock()
	count := reloader.restartedCount
	lastSvc := reloader.lastService
	reloader.mu.Unlock()
	if count != 1 || lastSvc != "llama" {
		t.Errorf("expected reloader to restart 'llama' once, got count=%d service=%s", count, lastSvc)
	}

	// Verify arbitrator state is ready
	if arb.State() != ArbitratorStateReady {
		t.Errorf("expected arbitrator state 'ready', got %s", arb.State())
	}
}

func TestModelManager_DeleteModel(t *testing.T) {
	tmpDir := t.TempDir()
	store := newMockSettingStore()

	modelABytes := createValidGGUFBytes(3, 100, 20)
	modelBBytes := createValidGGUFBytes(3, 150, 25)

	_ = os.WriteFile(filepath.Join(tmpDir, "active-model.gguf"), modelABytes, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "unused-model.gguf"), modelBBytes, 0644)
	_ = os.Symlink("active-model.gguf", filepath.Join(tmpDir, "active.gguf"))

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir: tmpDir,
		Store:     store,
	})

	ctx := context.Background()

	// 1. Deleting active model must fail with ErrActiveModelDeletion
	err := mgr.DeleteModel(ctx, "active-model.gguf")
	if !errors.Is(err, ErrActiveModelDeletion) {
		t.Errorf("expected ErrActiveModelDeletion, got %v", err)
	}

	// 2. Deleting unused model should succeed
	if err := mgr.DeleteModel(ctx, "unused-model.gguf"); err != nil {
		t.Fatalf("DeleteModel failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "unused-model.gguf")); !os.IsNotExist(err) {
		t.Errorf("expected unused-model.gguf to be deleted")
	}

	// 3. Deleting non-existent model returns ErrModelNotFound
	if err := mgr.DeleteModel(ctx, "missing.gguf"); !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got %v", err)
	}
}

func TestModelManager_DownloadResumptionAndAtomicRename(t *testing.T) {
	tmpDir := t.TempDir()
	modelData := createValidGGUFBytes(3, 200, 40)
	totalLen := len(modelData)

	hasher := sha256.New()
	hasher.Write(modelData)
	expectedSHA := hex.EncodeToString(hasher.Sum(nil))

	rangeRequestsCount := 0
	// Setup test HTTP server with Range header support
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(totalLen))
			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			rangeRequestsCount++
			// Expect bytes=N-
			var start int64
			_, _ = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if start >= int64(totalLen) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, totalLen-1, totalLen))
			w.Header().Set("Content-Length", strconv.Itoa(totalLen-int(start)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(modelData[start:])
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(totalLen))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(modelData)
	}))
	defer ts.Close()

	// Pre-create a partial .downloading file with first 10 bytes to test HTTP Range resumption
	partialBytes := modelData[:10]
	tempPath := filepath.Join(tmpDir, "qwen2.5.gguf.downloading")
	if err := os.WriteFile(tempPath, partialBytes, 0644); err != nil {
		t.Fatalf("failed to write partial temp file: %v", err)
	}

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		HTTPClient: ts.Client(),
		DiskFree: func(path string) (int64, error) {
			// Report 50 GB free (plenty of room over 5.0 GB safety buffer)
			return 50 * 1024 * 1024 * 1024, nil
		},
	})

	progress, err := mgr.StartDownload(context.Background(), AIModelDownloadRequest{
		URL:      ts.URL + "/qwen2.5.gguf",
		Filename: "qwen2.5.gguf",
		SHA256:   expectedSHA,
	})
	if err != nil {
		t.Fatalf("StartDownload failed: %v", err)
	}

	if progress.ID == "" {
		t.Errorf("expected non-empty download ID")
	}

	// Poll until completed (or timeout)
	timeout := time.After(5 * time.Second)
	completed := false
	for !completed {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for download completion")
		case <-time.After(50 * time.Millisecond):
			downloads := mgr.ListDownloads()
			for _, d := range downloads {
				if d.ID == progress.ID {
					if d.Status == "completed" {
						completed = true
						break
					}
					if d.Status == "failed" {
						t.Fatalf("download failed with error: %s", d.Error)
					}
				}
			}
		}
	}

	// 1. Verify Range header was used for resumption
	if rangeRequestsCount == 0 {
		t.Errorf("expected at least one Range HTTP request for resumption")
	}

	// 2. Verify temp file was atomically renamed to final .gguf file
	finalPath := filepath.Join(tmpDir, "qwen2.5.gguf")
	if _, err := os.Stat(finalPath); err != nil {
		t.Errorf("final model file was not found: %v", err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temporary .downloading file should have been renamed away")
	}

	// 3. Verify content matches exactly
	downloadedData, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if !bytes.Equal(downloadedData, modelData) {
		t.Errorf("downloaded content mismatch")
	}
}

func TestModelManager_DownloadStorageFloorRejection(t *testing.T) {
	tmpDir := t.TempDir()
	totalLen := 500 * 1024 * 1024 // 500 MB model

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(totalLen))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		HTTPClient: ts.Client(),
		DiskFree: func(path string) (int64, error) {
			// Free space is 5.2 GB, but 500 MB model requires 500 MB + 5.0 GB = 5.5 GB -> Insufficient!
			return (52 * 1024 * 1024 * 1024) / 10, nil
		},
	})

	_, err := mgr.StartDownload(context.Background(), AIModelDownloadRequest{
		URL:      ts.URL + "/huge-model.gguf",
		Filename: "huge-model.gguf",
	})

	if !errors.Is(err, ErrInsufficientStorageSpace) {
		t.Fatalf("expected ErrInsufficientStorageSpace, got %v", err)
	}
}

func TestModelManager_DownloadChecksumMismatchRejection(t *testing.T) {
	tmpDir := t.TempDir()
	modelData := createValidGGUFBytes(3, 10, 5)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(modelData)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(modelData)
	}))
	defer ts.Close()

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		HTTPClient: ts.Client(),
		DiskFree: func(path string) (int64, error) {
			return 20 * 1024 * 1024 * 1024, nil
		},
	})

	progress, err := mgr.StartDownload(context.Background(), AIModelDownloadRequest{
		URL:      ts.URL + "/model.gguf",
		Filename: "model.gguf",
		SHA256:   "0000000000000000000000000000000000000000000000000000000000000000", // Invalid SHA
	})
	if err != nil {
		t.Fatalf("StartDownload failed: %v", err)
	}

	timeout := time.After(3 * time.Second)
	failed := false
	for !failed {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for checksum verification failure")
		case <-time.After(50 * time.Millisecond):
			for _, d := range mgr.ListDownloads() {
				if d.ID == progress.ID && d.Status == "failed" {
					if !strings.Contains(d.Error, "sha256") {
						t.Errorf("expected error to mention sha256, got %s", d.Error)
					}
					failed = true
					break
				}
			}
		}
	}

	// Final .gguf file must NOT have been created
	if _, err := os.Stat(filepath.Join(tmpDir, "model.gguf")); !os.IsNotExist(err) {
		t.Errorf("model.gguf should not exist after checksum failure")
	}
}

func TestModelManager_SettingsTuningAndArbitratorSync(t *testing.T) {
	tmpDir := t.TempDir()
	store := newMockSettingStore()
	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		HardwareMode: "gpu",
		IsGPU:        true,
	})
	defer arb.Close()

	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir:  tmpDir,
		Store:      store,
		Arbitrator: arb,
	})

	ctx := context.Background()

	// Initial settings
	settings, err := mgr.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if settings.IdleTimeoutSeconds != 300 {
		t.Errorf("expected default idle timeout 300, got %d", settings.IdleTimeoutSeconds)
	}
	if settings.ContextLength != 4096 {
		t.Errorf("expected default context length 4096, got %d", settings.ContextLength)
	}

	// Update settings
	newLayers := 45
	newCtx := 8192
	newIdle := 600
	updated, err := mgr.UpdateSettings(ctx, AISettingsUpdate{
		GPULayers:          &newLayers,
		ContextLength:      &newCtx,
		IdleTimeoutSeconds: &newIdle,
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if updated.GPULayers != 45 {
		t.Errorf("expected GPULayers 45, got %d", updated.GPULayers)
	}
	if updated.ContextLength != 8192 {
		t.Errorf("expected ContextLength 8192, got %d", updated.ContextLength)
	}
	if updated.IdleTimeoutSeconds != 600 {
		t.Errorf("expected IdleTimeoutSeconds 600, got %d", updated.IdleTimeoutSeconds)
	}

	// Verify arbitrator dynamic idle timeout was updated
	arbStatus := arb.Status()
	if arbStatus.IdleTimeoutSeconds != 600 {
		t.Errorf("expected arbitrator idle timeout 600, got %d", arbStatus.IdleTimeoutSeconds)
	}
}

func TestModelManager_ScrubbedInferenceLogs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewModelManager(ModelManagerConfig{
		ModelsDir: tmpDir,
	})

	recorder := mgr.LogRecorder()
	now := time.Now().UTC()

	// Add an operational message with prompt text
	recorder.Add(now, "INFO", "inference", "Inference request processed: prompt: What is my secret bank password? (tokens=42)")

	logs, err := mgr.GetScrubbedLogs(context.Background())
	if err != nil {
		t.Fatalf("GetScrubbedLogs failed: %v", err)
	}

	foundScrubbed := false
	for _, l := range logs {
		if strings.Contains(l.Message, "secret bank password") {
			t.Errorf("detected unscrubbed sensitive prompt text in log: %s", l.Message)
		}
		if strings.Contains(l.Message, "[PROMPT_SCRUBBED]") {
			foundScrubbed = true
		}
	}

	if !foundScrubbed {
		t.Errorf("expected scrubbed prompt indicator in logs")
	}
}
