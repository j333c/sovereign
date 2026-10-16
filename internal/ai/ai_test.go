package ai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func createMockGGUFBytes(version uint32, tensorCount, metadataCount uint64) []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x47, 0x47, 0x55, 0x46}) // "GGUF"
	_ = binary.Write(buf, binary.LittleEndian, version)
	_ = binary.Write(buf, binary.LittleEndian, tensorCount)
	_ = binary.Write(buf, binary.LittleEndian, metadataCount)
	return buf.Bytes()
}

func TestValidateGGUF_Success(t *testing.T) {
	data := createMockGGUFBytes(3, 128, 42)
	hdr, err := ValidateGGUF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected valid GGUF header, got error: %v", err)
	}
	if hdr.Version != 3 {
		t.Errorf("expected version 3, got %d", hdr.Version)
	}
	if hdr.TensorCount != 128 {
		t.Errorf("expected tensor count 128, got %d", hdr.TensorCount)
	}
	if hdr.MetadataKVCount != 42 {
		t.Errorf("expected metadata count 42, got %d", hdr.MetadataKVCount)
	}
}

func TestValidateGGUF_InvalidMagic(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte{0x47, 0x47}},
		{"wrong_magic", []byte{0x50, 0x4b, 0x03, 0x04, 0x01, 0x00, 0x00, 0x00}}, // PK zip
		{"text_file", []byte("This is not a GGUF file at all")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateGGUF(bytes.NewReader(tc.data))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !errors.Is(err, ErrInvalidGGUFMagic) {
				t.Errorf("expected ErrInvalidGGUFMagic, got: %v", err)
			}
		})
	}
}

func TestValidateGGUF_InvalidVersion(t *testing.T) {
	// Version 0
	buf0 := createMockGGUFBytes(0, 10, 10)
	_, err := ValidateGGUF(bytes.NewReader(buf0))
	if err == nil || !errors.Is(err, ErrInvalidGGUFVersion) {
		t.Errorf("expected ErrInvalidGGUFVersion for version 0, got: %v", err)
	}

	// Version 101 (> 100)
	buf101 := createMockGGUFBytes(101, 10, 10)
	_, err = ValidateGGUF(bytes.NewReader(buf101))
	if err == nil || !errors.Is(err, ErrInvalidGGUFVersion) {
		t.Errorf("expected ErrInvalidGGUFVersion for version 101, got: %v", err)
	}
}

func TestValidateGGUFHeader_File(t *testing.T) {
	tmpDir := t.TempDir()

	validFile := filepath.Join(tmpDir, "valid.gguf")
	if err := os.WriteFile(validFile, createMockGGUFBytes(2, 64, 16), 0644); err != nil {
		t.Fatal(err)
	}

	corruptFile := filepath.Join(tmpDir, "corrupt.gguf")
	if err := os.WriteFile(corruptFile, []byte("NOT_A_GGUF_HEADER"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Valid file
	hdr, err := ValidateGGUFHeader(validFile)
	if err != nil {
		t.Fatalf("ValidateGGUFHeader failed on valid file: %v", err)
	}
	if hdr.Version != 2 || hdr.TensorCount != 64 {
		t.Errorf("unexpected header: %+v", hdr)
	}

	// 2. Corrupt file
	_, err = ValidateGGUFHeader(corruptFile)
	if err == nil || !errors.Is(err, ErrInvalidGGUFMagic) {
		t.Fatalf("expected ErrInvalidGGUFMagic for corrupt file, got: %v", err)
	}

	// 3. Non-existent file
	_, err = ValidateGGUFHeader(filepath.Join(tmpDir, "missing.gguf"))
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestScanModels(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Empty dir
	models, err := ScanModels(tmpDir)
	if err != nil {
		t.Fatalf("ScanModels on empty dir failed: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}

	// 2. Populate models dir
	// Valid model 1
	_ = os.WriteFile(filepath.Join(tmpDir, "qwen2.5-7b.gguf"), createMockGGUFBytes(3, 200, 30), 0644)
	// Valid model 2
	_ = os.WriteFile(filepath.Join(tmpDir, "llama-3.2-3b.gguf"), createMockGGUFBytes(2, 150, 20), 0644)
	// Corrupt model (should be skipped)
	_ = os.WriteFile(filepath.Join(tmpDir, "corrupt.gguf"), []byte("corrupt data"), 0644)
	// Incomplete downloading file (should be ignored)
	_ = os.WriteFile(filepath.Join(tmpDir, "mistral.gguf.downloading"), createMockGGUFBytes(3, 100, 10), 0644)
	// Hidden file (should be ignored)
	_ = os.WriteFile(filepath.Join(tmpDir, ".hidden.gguf"), createMockGGUFBytes(3, 100, 10), 0644)
	// Non-gguf file
	_ = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)

	models, err = ScanModels(tmpDir)
	if err != nil {
		t.Fatalf("ScanModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected exactly 2 valid models, got %d", len(models))
	}

	// Deterministic sorting by filename
	if models[0].Filename != "llama-3.2-3b.gguf" {
		t.Errorf("expected first model to be llama-3.2-3b.gguf, got %s", models[0].Filename)
	}
	if models[1].Filename != "qwen2.5-7b.gguf" {
		t.Errorf("expected second model to be qwen2.5-7b.gguf, got %s", models[1].Filename)
	}
	if !models[0].Valid || models[0].Header == nil {
		t.Errorf("expected model 0 to have Valid: true and Header populated")
	}
}

func TestCheckModelStorageSpace(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	modelSize := 4 * gib // 4.0 GB

	// Test 1: Available = 10 GB, Required = 4 GB + 5 GB = 9 GB -> PASS (free >= model + 5GB)
	mockFree10GB := func(path string) (int64, error) {
		return 10 * gib, nil
	}
	err := CheckModelStorageSpaceWithFunc("/data/models", modelSize, mockFree10GB)
	if err != nil {
		t.Fatalf("expected nil error for 10 GB free, got: %v", err)
	}

	// Test 2: Available = 9 GB, Required = 4 GB + 5 GB = 9 GB -> PASS (exact match)
	mockFree9GB := func(path string) (int64, error) {
		return 9 * gib, nil
	}
	err = CheckModelStorageSpaceWithFunc("/data/models", modelSize, mockFree9GB)
	if err != nil {
		t.Fatalf("expected nil error for exact 9 GB free, got: %v", err)
	}

	// Test 3: Available = 8 GB, Required = 4 GB + 5 GB = 9 GB -> FAIL (insufficient space)
	mockFree8GB := func(path string) (int64, error) {
		return 8 * gib, nil
	}
	err = CheckModelStorageSpaceWithFunc("/data/models", modelSize, mockFree8GB)
	if err == nil {
		t.Fatal("expected ErrInsufficientStorageSpace for 8 GB free, got nil")
	}
	if !errors.Is(err, ErrInsufficientStorageSpace) {
		t.Errorf("expected errors.Is(err, ErrInsufficientStorageSpace), got: %v", err)
	}

	var spaceErr *StorageSpaceError
	if !errors.As(err, &spaceErr) {
		t.Fatalf("expected error to cast to *StorageSpaceError, got: %T", err)
	}
	if spaceErr.AvailableBytes != 8*gib {
		t.Errorf("expected AvailableBytes = %d, got %d", 8*gib, spaceErr.AvailableBytes)
	}
	if spaceErr.RequiredBytes != 9*gib {
		t.Errorf("expected RequiredBytes = %d, got %d", 9*gib, spaceErr.RequiredBytes)
	}
	if spaceErr.BufferBytes != 5*gib {
		t.Errorf("expected BufferBytes = %d, got %d", 5*gib, spaceErr.BufferBytes)
	}
}

func TestSafeStubHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()

	SafeStubHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 Service Unavailable, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", contentType)
	}

	var prob SafeStubProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}

	if prob.Status != 503 {
		t.Errorf("expected status 503, got %d", prob.Status)
	}
	if prob.Title != "No AI Model Available" {
		t.Errorf("unexpected title: %s", prob.Title)
	}
	if prob.Instance != "/v1/chat/completions" {
		t.Errorf("expected instance /v1/chat/completions, got %s", prob.Instance)
	}
	if !bytes.Contains([]byte(prob.Detail), []byte("Settings")) {
		t.Errorf("expected detail to mention Settings, got: %s", prob.Detail)
	}
}

func TestDefaultGateway_SafeStubWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	arb := NewGPUArbitrator(ArbitratorConfig{
		InitialState: ArbitratorStateReady,
	})
	defer arb.Close()

	gw := NewGateway(GatewayConfig{
		ModelsDir:  tmpDir,
		LlamaURL:   "http://127.0.0.1:8080",
		Arbitrator: arb,
	})

	available, err := gw.CheckModelAvailability(context.Background())
	if err != nil {
		t.Fatalf("CheckModelAvailability failed: %v", err)
	}
	if available {
		t.Fatal("expected model availability to be false for empty models dir")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503, got %d", rec.Code)
	}
	if gw.Arbitrator().State() != ArbitratorStateSafeStub {
		t.Fatalf("expected arbitrator state safe_stub, got %s", gw.Arbitrator().State())
	}
}

func TestDefaultGateway_ProxyWhenModelPresent(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "model.gguf"), createMockGGUFBytes(3, 100, 10), 0644)

	// Mock upstream llama-server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "model.gguf"}]}`))
	}))
	defer upstream.Close()

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  upstream.URL,
	})

	available, err := gw.CheckModelAvailability(context.Background())
	if err != nil {
		t.Fatalf("CheckModelAvailability failed: %v", err)
	}
	if !available {
		t.Fatal("expected model availability to be true when model exists")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPresets_CatalogueAndList(t *testing.T) {
	presets := ListPresets()
	if len(presets) != 3 {
		t.Fatalf("expected 3 presets, got %d", len(presets))
	}

	expected := map[string]struct {
		name string
		temp float64
		topP float64
	}{
		"precise":  {"Precise & Factual", 0.1, 0.9},
		"balanced": {"Balanced Assistant", 0.7, 0.95},
		"creative": {"Creative Drafting", 0.85, 0.98},
	}

	for id, exp := range expected {
		p, ok := GetPreset(id)
		if !ok {
			t.Errorf("expected preset %s to exist", id)
			continue
		}
		if p.Name != exp.name {
			t.Errorf("[%s] expected name %s, got %s", id, exp.name, p.Name)
		}
		if p.Temperature != exp.temp {
			t.Errorf("[%s] expected temp %f, got %f", id, exp.temp, p.Temperature)
		}
		if p.TopP != exp.topP {
			t.Errorf("[%s] expected top_p %f, got %f", id, exp.topP, p.TopP)
		}
		if len(p.RecommendedUseCases) == 0 {
			t.Errorf("[%s] expected non-empty recommended use cases", id)
		}
	}
}

func TestNormalizeInferencePayload(t *testing.T) {
	tests := []struct {
		name          string
		inputJSON     string
		defaultPreset string
		expectTemp    float64
		expectTopP    float64
	}{
		{
			name:       "creative preset in json",
			inputJSON:  `{"preset": "creative", "messages": [{"role": "user", "content": "hello"}]}`,
			expectTemp: 0.85,
			expectTopP: 0.98,
		},
		{
			name:       "precise preset in json",
			inputJSON:  `{"preset": "precise", "messages": []}`,
			expectTemp: 0.1,
			expectTopP: 0.9,
		},
		{
			name:       "intent field alternative in json",
			inputJSON:  `{"intent": "creative", "messages": []}`,
			expectTemp: 0.85,
			expectTopP: 0.98,
		},
		{
			name:          "fallback to defaultPreset header",
			inputJSON:     `{"messages": []}`,
			defaultPreset: "precise",
			expectTemp:    0.1,
			expectTopP:    0.9,
		},
		{
			name:       "invalid preset falls back to balanced",
			inputJSON:  `{"preset": "unknown_preset", "messages": []}`,
			expectTemp: 0.7,
			expectTopP: 0.95,
		},
		{
			name:       "power user explicit temperature override preserved",
			inputJSON:  `{"preset": "precise", "temperature": 0.2}`,
			expectTemp: 0.2,
			expectTopP: 0.9, // calibrated top_p still injected
		},
		{
			name:       "power user explicit top_p override preserved",
			inputJSON:  `{"preset": "creative", "top_p": 0.5}`,
			expectTemp: 0.85, // calibrated temp still injected
			expectTopP: 0.5,
		},
		{
			name:       "power user explicit zero temperature preserved",
			inputJSON:  `{"preset": "creative", "temperature": 0.0}`,
			expectTemp: 0.0,
			expectTopP: 0.98,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalized, err := NormalizeInferencePayload([]byte(tc.inputJSON), tc.defaultPreset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result map[string]any
			if err := json.Unmarshal(normalized, &result); err != nil {
				t.Fatalf("failed to parse normalized output: %v", err)
			}

			tempVal, ok := result["temperature"].(float64)
			if !ok || tempVal != tc.expectTemp {
				t.Errorf("expected temperature %v, got %v", tc.expectTemp, result["temperature"])
			}

			topPVal, ok := result["top_p"].(float64)
			if !ok || topPVal != tc.expectTopP {
				t.Errorf("expected top_p %v, got %v", tc.expectTopP, result["top_p"])
			}
		})
	}

	// Empty body returns empty
	emptyRes, err := NormalizeInferencePayload([]byte(""), "balanced")
	if err != nil || len(emptyRes) != 0 {
		t.Fatalf("expected empty response for empty body, got %v, err: %v", emptyRes, err)
	}

	// Invalid JSON returns error
	_, err = NormalizeInferencePayload([]byte("{not json"), "balanced")
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
}

func TestClassifyPriority(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		value    string
		query    string
		expected InferencePriority
	}{
		{"X-Inference-Priority interactive", "X-Inference-Priority", "interactive", "", PriorityInteractive},
		{"X-Inference-Priority background", "X-Inference-Priority", "background", "", PriorityBackground},
		{"X-Inference-Priority case insensitive", "X-Inference-Priority", "BACKGROUND", "", PriorityBackground},
		{"X-Priority background", "X-Priority", "background", "", PriorityBackground},
		{"X-Lane batch", "X-Lane", "batch", "", PriorityBackground},
		{"Query param background", "", "", "priority=background", PriorityBackground},
		{"Default to interactive", "", "", "", PriorityInteractive},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?"+tc.query, nil)
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			p := ClassifyPriority(req)
			if p != tc.expected {
				t.Errorf("expected priority %v, got %v", tc.expected, p)
			}
		})
	}
}

func TestRequestGovernor_BasicAcquireRelease(t *testing.T) {
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
	})

	// 1. Interactive request acquires slot immediately
	relInteractive, err := gov.Acquire(context.Background(), PriorityInteractive)
	if err != nil {
		t.Fatalf("Acquire interactive failed: %v", err)
	}

	st := gov.Status()
	if st.ActiveInteractive != 1 || st.ActiveBackground != 0 {
		t.Errorf("unexpected status: %+v", st)
	}

	// 2. Release slot
	relInteractive()

	st = gov.Status()
	if st.ActiveInteractive != 0 || st.TotalServed != 1 {
		t.Errorf("unexpected status after release: %+v", st)
	}

	// 3. Background request acquires slot when idle
	relBackground, err := gov.Acquire(context.Background(), PriorityBackground)
	if err != nil {
		t.Fatalf("Acquire background failed: %v", err)
	}

	st = gov.Status()
	if st.ActiveBackground != 1 {
		t.Errorf("expected 1 active background, got: %+v", st)
	}

	relBackground()
	st = gov.Status()
	if st.ActiveBackground != 0 || st.TotalServed != 2 {
		t.Errorf("expected 2 total served, got: %+v", st)
	}
}

func TestRequestGovernor_InteractivePriorityOverBackground(t *testing.T) {
	// TotalSlots = 2, ReservedInteractive = 1
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
		MaxBackgroundQueue:  10,
		MaxInteractiveQueue: 10,
	})

	// 1. Background acquires slot 1 (allowed because 0 < 2-1 = 1)
	relBg1, err := gov.Acquire(context.Background(), PriorityBackground)
	if err != nil {
		t.Fatalf("failed to acquire initial background slot: %v", err)
	}

	// 2. Interactive arrives and acquires slot 2 immediately (allowed because 1 active < 2 total)
	relInt1, err := gov.Acquire(context.Background(), PriorityInteractive)
	if err != nil {
		t.Fatalf("interactive request failed to acquire immediately: %v", err)
	}

	st := gov.Status()
	if st.ActiveInteractive != 1 || st.ActiveBackground != 1 {
		t.Fatalf("expected 1 interactive and 1 background active, got: %+v", st)
	}

	// 3. Queue 2 background tasks in the background lane
	bgQueuedDone := make(chan string, 2)
	for i := 1; i <= 2; i++ {
		idx := i
		go func() {
			rel, err := gov.Acquire(context.Background(), PriorityBackground)
			if err != nil {
				return
			}
			bgQueuedDone <- fmt.Sprintf("bg-%d", idx)
			rel()
		}()
	}

	// Wait briefly for background tasks to enter queue
	time.Sleep(20 * time.Millisecond)

	st = gov.Status()
	if st.QueuedBackground != 2 {
		t.Fatalf("expected 2 queued background tasks, got %d", st.QueuedBackground)
	}

	// 4. Now an interactive task arrives and must wait because both slots are full (1 bg + 1 int)
	intQueuedDone := make(chan string, 1)
	go func() {
		rel, err := gov.Acquire(context.Background(), PriorityInteractive)
		if err != nil {
			return
		}
		intQueuedDone <- "interactive-2"
		rel()
	}()

	time.Sleep(20 * time.Millisecond)

	st = gov.Status()
	if st.QueuedInteractive != 1 {
		t.Fatalf("expected 1 queued interactive task, got %d", st.QueuedInteractive)
	}

	// 5. Release the first background task slot.
	// CRITICAL TEST: The waiting interactive request MUST be scheduled ahead of the waiting background tasks!
	relBg1()

	select {
	case tag := <-intQueuedDone:
		if tag != "interactive-2" {
			t.Fatalf("expected interactive-2, got %s", tag)
		}
	case tag := <-bgQueuedDone:
		t.Fatalf("VIOLATION: background task %s was scheduled ahead of waiting interactive task!", tag)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for interactive task to complete")
	}

	// Clean up first interactive task to allow background tasks to drain
	relInt1()

	// Drain remaining background tasks
	for i := 0; i < 2; i++ {
		select {
		case <-bgQueuedDone:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for background queue to drain")
		}
	}
}

func TestRequestGovernor_QueueFull_429(t *testing.T) {
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
		MaxBackgroundQueue:  2,
	})

	// Fill background running slot
	relBg, err := gov.Acquire(context.Background(), PriorityBackground)
	if err != nil {
		t.Fatal(err)
	}
	defer relBg()

	// Fill background queue to capacity (2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < 2; i++ {
		go func() {
			_, _ = gov.Acquire(ctx, PriorityBackground)
		}()
	}
	time.Sleep(20 * time.Millisecond)

	st := gov.Status()
	if st.QueuedBackground != 2 {
		t.Fatalf("expected 2 queued background, got %d", st.QueuedBackground)
	}

	// 3rd background request must fail with ErrQueueFull
	_, err = gov.Acquire(context.Background(), PriorityBackground)
	if err == nil || !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got: %v", err)
	}

	st = gov.Status()
	if st.TotalRejected != 1 {
		t.Errorf("expected 1 total rejected, got %d", st.TotalRejected)
	}
}

func TestRequestGovernor_QueueTimeout_504(t *testing.T) {
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 0,
		QueueTimeout:        50 * time.Millisecond,
	})

	rel, err := gov.Acquire(context.Background(), PriorityInteractive)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()

	// Acquire with short context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = gov.Acquire(ctx, PriorityInteractive)
	if err == nil || !errors.Is(err, ErrQueueTimeout) {
		t.Fatalf("expected ErrQueueTimeout, got: %v", err)
	}

	st := gov.Status()
	if st.TotalTimedOut != 1 {
		t.Errorf("expected 1 total timed out, got %d", st.TotalTimedOut)
	}
}

func TestRequestGovernor_ClientCancellation(t *testing.T) {
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 0,
	})

	// Occupy sole slot
	rel, err := gov.Acquire(context.Background(), PriorityInteractive)
	if err != nil {
		t.Fatal(err)
	}

	// Queued request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		_, err := gov.Acquire(ctx, PriorityInteractive)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	st := gov.Status()
	if st.QueuedInteractive != 1 {
		t.Fatalf("expected 1 queued, got %d", st.QueuedInteractive)
	}

	// Cancel context while waiting
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for acquire cancellation")
	}

	st = gov.Status()
	if st.QueuedInteractive != 0 {
		t.Errorf("expected 0 queued interactive after cancellation, got %d", st.QueuedInteractive)
	}

	rel()
}

func TestDefaultGateway_ServeHTTP_GovernorAndPresets(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "model.gguf"), createMockGGUFBytes(3, 100, 10), 0644)

	var receivedBody []byte
	var receivedHeaderPriority string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaderPriority = r.Header.Get("X-Inference-Priority")
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "ok"}}]}`))
	}))
	defer upstream.Close()

	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
	})

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  upstream.URL,
		Governor:  gov,
	})

	// Test 1: Payload with preset: creative injects temp 0.85 and top_p 0.98
	reqBody := `{"preset": "creative", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var upstreamJSON map[string]any
	if err := json.Unmarshal(receivedBody, &upstreamJSON); err != nil {
		t.Fatalf("failed to parse upstream body: %v", err)
	}

	if upstreamJSON["temperature"] != 0.85 {
		t.Errorf("expected temperature 0.85, got %v", upstreamJSON["temperature"])
	}
	if upstreamJSON["top_p"] != 0.98 {
		t.Errorf("expected top_p 0.98, got %v", upstreamJSON["top_p"])
	}

	// Test 2: Payload with header X-Generation-Intent: precise and X-Inference-Priority: background
	reqBody = `{"messages": [{"role": "user", "content": "analyze invoice"}]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Generation-Intent", "precise")
	req.Header.Set("X-Inference-Priority", "background")
	rec = httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if receivedHeaderPriority != "background" {
		t.Errorf("expected upstream X-Inference-Priority background, got %s", receivedHeaderPriority)
	}

	if err := json.Unmarshal(receivedBody, &upstreamJSON); err != nil {
		t.Fatalf("failed to parse upstream body: %v", err)
	}
	if upstreamJSON["temperature"] != 0.1 {
		t.Errorf("expected temperature 0.1, got %v", upstreamJSON["temperature"])
	}
	if upstreamJSON["top_p"] != 0.9 {
		t.Errorf("expected top_p 0.9, got %v", upstreamJSON["top_p"])
	}

	// Check Governor status
	st := gw.Governor().Status()
	if st.TotalServed != 2 {
		t.Errorf("expected 2 total served in governor, got %d", st.TotalServed)
	}
}

func TestDefaultGateway_QueueSaturation_429(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "model.gguf"), createMockGGUFBytes(3, 100, 10), 0644)

	// Upstream that blocks until instructed
	blockCh := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blockCh
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	defer close(blockCh)

	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 0,
		MaxInteractiveQueue: 1,
	})

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  upstream.URL,
		Governor:  gov,
	})

	// Request 1: Takes slot and blocks in upstream
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
	}()
	time.Sleep(20 * time.Millisecond)

	// Request 2: Takes queue slot (capacity 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
	}()
	time.Sleep(20 * time.Millisecond)

	// Request 3: Exceeds queue -> HTTP 429 Problem Details
	req3 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec3 := httptest.NewRecorder()
	gw.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected HTTP 429 Too Many Requests, got %d: %s", rec3.Code, rec3.Body.String())
	}
	if ct := rec3.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", ct)
	}

	var prob map[string]any
	_ = json.Unmarshal(rec3.Body.Bytes(), &prob)
	if prob["title"] != "Too Many Requests" {
		t.Errorf("unexpected problem title: %v", prob["title"])
	}
}

func TestDefaultGateway_ClientCancellationReleasesSlot(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "model.gguf"), createMockGGUFBytes(3, 100, 10), 0644)

	started := make(chan struct{})
	stopUpstream := make(chan struct{})
	var closeOnce sync.Once
	cleanup := func() { closeOnce.Do(func() { close(stopUpstream) }) }
	defer cleanup()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-stopUpstream:
		case <-r.Context().Done():
		}
	}))
	defer upstream.CloseClientConnections()
	defer upstream.Close()

	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 0,
	})

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  upstream.URL,
		Governor:  gov,
		Client: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		gw.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait until upstream is entered
	<-started
	st := gov.Status()
	if st.ActiveInteractive != 1 {
		t.Fatalf("expected 1 active interactive, got: %+v", st)
	}

	// Cancel client connection
	cancel()
	cleanup()

	<-done

	st = gov.Status()
	if st.ActiveInteractive != 0 {
		t.Errorf("expected 0 active slots after client cancellation, got %d", st.ActiveInteractive)
	}
	if st.TotalCanceled != 1 {
		t.Errorf("expected 1 total canceled, got %d", st.TotalCanceled)
	}
}

func TestDefaultGateway_QueueTimeout_504(t *testing.T) {
	tmpDir := t.TempDir()
	validGGUF := createMockGGUFBytes(3, 128, 42)
	if err := os.WriteFile(filepath.Join(tmpDir, "model.gguf"), validGGUF, 0o644); err != nil {
		t.Fatalf("failed to write dummy model: %v", err)
	}

	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 1,
		QueueTimeout:        20 * time.Millisecond,
	})

	// Manually hold the only slot
	ctx := context.Background()
	release, err := gov.Acquire(ctx, PriorityInteractive)
	if err != nil {
		t.Fatalf("failed to acquire first slot: %v", err)
	}
	defer release()

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  "http://127.0.0.1:9", // unreachable, shouldn't be reached anyway
		Governor:  gov,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected HTTP 504 Gateway Timeout, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("expected application/problem+json content type, got: %s", rec.Header().Get("Content-Type"))
	}
}

func TestDefaultGateway_ReadonlyModelsBypassesSaturatedGovernor(t *testing.T) {
	tmpDir := t.TempDir()
	validGGUF := createMockGGUFBytes(3, 128, 42)
	if err := os.WriteFile(filepath.Join(tmpDir, "qwen2.5-coder-7b.gguf"), validGGUF, 0o644); err != nil {
		t.Fatalf("failed to write dummy model: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "qwen2.5-coder-7b.gguf"}]}`))
	}))
	defer upstream.Close()

	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          1,
		ReservedInteractive: 1,
	})

	// Saturate the only slot
	release, err := gov.Acquire(context.Background(), PriorityInteractive)
	if err != nil {
		t.Fatalf("failed to acquire slot: %v", err)
	}
	defer release()

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  upstream.URL,
		Governor:  gov,
	})

	// GET /v1/models should bypass governor and respond immediately 200 OK
	reqModels := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	recModels := httptest.NewRecorder()
	gw.ServeHTTP(recModels, reqModels)

	if recModels.Code != http.StatusOK {
		t.Fatalf("expected GET /v1/models to bypass governor with HTTP 200, got %d", recModels.Code)
	}

	// OPTIONS /v1/chat/completions should also bypass governor and respond immediately without blocking
	reqOptions := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	recOptions := httptest.NewRecorder()
	gw.ServeHTTP(recOptions, reqOptions)

	if recOptions.Code != http.StatusOK && recOptions.Code != http.StatusNoContent {
		t.Fatalf("expected OPTIONS to bypass governor with HTTP 200 or 204, got %d", recOptions.Code)
	}
}

func TestNormalizeInferencePayload_NilAndStripKeys(t *testing.T) {
	// Nil payload should return nil without error or panic
	emptyNorm, err := NormalizeInferencePayload(nil, "balanced")
	if err != nil {
		t.Errorf("unexpected error for nil payload: %v", err)
	}
	if len(emptyNorm) != 0 {
		t.Errorf("expected empty payload for nil input, got %s", string(emptyNorm))
	}

	// "null" JSON payload should error
	_, err = NormalizeInferencePayload([]byte("null"), "balanced")
	if err == nil {
		t.Error("expected error for 'null' payload")
	}

	// Preset and intent keys should be stripped from raw payload before sending upstream
	input := `{"preset": "creative", "intent": "creative", "model": "test-model", "messages": []}`
	norm, err := NormalizeInferencePayload([]byte(input), "")
	if err != nil {
		t.Fatalf("failed to normalize: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(norm, &m); err != nil {
		t.Fatalf("failed to unmarshal normalized payload: %v", err)
	}

	if _, exists := m["preset"]; exists {
		t.Error("expected 'preset' field to be stripped from payload")
	}
	if _, exists := m["intent"]; exists {
		t.Error("expected 'intent' field to be stripped from payload")
	}
	if m["temperature"] != 0.85 {
		t.Errorf("expected temperature 0.85, got %v", m["temperature"])
	}
}

func TestRequestGovernor_CancellationTracksTotalCanceled(t *testing.T) {
	gov := NewRequestGovernor(GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
	})

	ctx1 := context.Background()
	release1, err := gov.Acquire(ctx1, PriorityInteractive)
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	release1() // completed successfully

	ctx2, cancel2 := context.WithCancel(context.Background())
	release2, err := gov.Acquire(ctx2, PriorityBackground)
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	cancel2()  // cancel context before release
	release2() // release after cancellation

	st := gov.Status()
	if st.TotalServed != 1 {
		t.Errorf("expected totalServed=1, got %d", st.TotalServed)
	}
	if st.TotalCanceled != 1 {
		t.Errorf("expected totalCanceled=1, got %d", st.TotalCanceled)
	}
}

func TestGPUArbitrator_IdleSurrender_300s(t *testing.T) {
	currTime := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	getTime := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return currTime
	}
	advanceTime := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		currTime = currTime.Add(d)
	}

	unloaded := false
	unloadMu := sync.Mutex{}
	unloadFunc := func(ctx context.Context) error {
		unloadMu.Lock()
		defer unloadMu.Unlock()
		unloaded = true
		return nil
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		TickInterval: 10 * time.Millisecond,
		NowFunc:      getTime,
		UnloadFunc:   unloadFunc,
		IsGPU:        true,
	})
	defer arb.Close()

	if arb.State() != ArbitratorStateReady {
		t.Fatalf("expected initial state ready, got %s", arb.State())
	}

	// Advance time by 200s (below 300s threshold)
	advanceTime(200 * time.Second)
	time.Sleep(50 * time.Millisecond)

	if arb.State() != ArbitratorStateReady {
		t.Fatalf("expected state ready after 200s, got %s", arb.State())
	}
	unloadMu.Lock()
	if unloaded {
		t.Fatalf("expected model not unloaded before 300s")
	}
	unloadMu.Unlock()

	// Advance time past 300s threshold (another 101s -> total 301s)
	advanceTime(101 * time.Second)
	time.Sleep(50 * time.Millisecond)

	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Fatalf("expected state idle_surrendered after 301s, got %s", arb.State())
	}
	unloadMu.Lock()
	if !unloaded {
		t.Fatalf("expected unloadFunc to be called after 300s inactivity")
	}
	unloadMu.Unlock()

	// Calling SurrenderVRAM again should be resilient and idempotent
	err := arb.SurrenderVRAM(context.Background())
	if err != nil {
		t.Fatalf("expected SurrenderVRAM to be resilient when already surrendered, got: %v", err)
	}
}

func TestGPUArbitrator_ActivityReset(t *testing.T) {
	currTime := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	getTime := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return currTime
	}
	advanceTime := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		currTime = currTime.Add(d)
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		TickInterval: 10 * time.Millisecond,
		NowFunc:      getTime,
	})
	defer arb.Close()

	advanceTime(200 * time.Second)
	arb.RecordActivity() // reset countdown

	advanceTime(200 * time.Second) // total 400s elapsed, but only 200s since last activity
	time.Sleep(50 * time.Millisecond)

	if arb.State() != ArbitratorStateReady {
		t.Fatalf("expected state ready after activity reset, got %s", arb.State())
	}

	advanceTime(101 * time.Second) // 301s since last activity
	time.Sleep(50 * time.Millisecond)

	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Fatalf("expected state idle_surrendered after 301s since activity reset, got %s", arb.State())
	}
}

func TestGPUArbitrator_DayNightSchedule(t *testing.T) {
	tests := []struct {
		name      string
		time      time.Time
		isDaytime bool
	}{
		{"07:00 daytime start", time.Date(2026, 10, 16, 7, 0, 0, 0, time.UTC), true},
		{"12:00 noon daytime", time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), true},
		{"23:59 evening daytime", time.Date(2026, 10, 16, 23, 59, 0, 0, time.UTC), true},
		{"00:30 night-owl daytime", time.Date(2026, 10, 16, 0, 30, 0, 0, time.UTC), true},
		{"01:00 off-peak night start", time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC), false},
		{"03:00 deep off-peak night", time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC), false},
		{"06:59 night end", time.Date(2026, 10, 16, 6, 59, 59, 0, time.UTC), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsDaytime(tc.time)
			if got != tc.isDaytime {
				t.Errorf("IsDaytime(%v) = %v; expected %v", tc.time, got, tc.isDaytime)
			}
		})
	}
}

func TestGPUArbitrator_DaytimeWakeup_BufferAndMicrocopy(t *testing.T) {
	currTime := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC) // 10:00 AM (Daytime)
	var stateHistory []string
	var historyMu sync.Mutex

	wakeCalled := false
	wakeFunc := func(ctx context.Context) error {
		wakeCalled = true
		// Verify timeout buffer is at least 60s
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Errorf("expected deadline on wake context")
		} else {
			remaining := time.Until(deadline)
			if remaining < 50*time.Second {
				t.Errorf("expected wake timeout buffer >= 60s, got remaining %v", remaining)
			}
		}
		time.Sleep(20 * time.Millisecond)
		return nil
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		WakeTimeout:  90 * time.Second,
		TickInterval: 10 * time.Millisecond,
		NowFunc: func() time.Time {
			return currTime
		},
		InitialState: ArbitratorStateIdleSurrendered,
		WakeFunc:     wakeFunc,
		OnStateChange: func(oldState, newState string) {
			historyMu.Lock()
			stateHistory = append(stateHistory, newState)
			historyMu.Unlock()
		},
	})
	defer arb.Close()

	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Fatalf("expected initial state idle_surrendered")
	}

	// Trigger wake-up via EnsureReady
	err := arb.EnsureReady(context.Background())
	if err != nil {
		t.Fatalf("EnsureReady failed: %v", err)
	}

	if !wakeCalled {
		t.Fatal("expected wakeFunc to have been called")
	}
	if arb.State() != ArbitratorStateReady {
		t.Fatalf("expected state ready after successful wake, got %s", arb.State())
	}

	historyMu.Lock()
	defer historyMu.Unlock()
	if len(stateHistory) < 2 {
		t.Fatalf("expected transitions to warming_up and ready, got: %v", stateHistory)
	}
	if stateHistory[0] != ArbitratorStateWarmingUp || stateHistory[1] != ArbitratorStateReady {
		t.Errorf("unexpected state transition sequence: %v", stateHistory)
	}
}

func TestGPUArbitrator_ImmediateRequestWhileWarmingUp(t *testing.T) {
	wakeBlock := make(chan struct{})
	wakeCount := 0
	var wakeMu sync.Mutex

	wakeFunc := func(ctx context.Context) error {
		wakeMu.Lock()
		wakeCount++
		wakeMu.Unlock()
		<-wakeBlock
		return nil
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		WakeTimeout:  90 * time.Second,
		TickInterval: 10 * time.Millisecond,
		InitialState: ArbitratorStateIdleSurrendered,
		WakeFunc:     wakeFunc,
	})
	defer arb.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	var err1, err2 error

	// Request 1 arrives
	go func() {
		defer wg.Done()
		err1 = arb.EnsureReady(context.Background())
	}()

	// Wait until state transitions to warming_up
	for i := 0; i < 50; i++ {
		if arb.State() == ArbitratorStateWarmingUp {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if arb.State() != ArbitratorStateWarmingUp {
		t.Fatalf("expected state warming_up")
	}

	// Microcopy reassurance check during warm-up
	st := arb.Status()
	if st.ReassuranceMessage != ReassuranceWarmingUp {
		t.Errorf("expected reassurance microcopy %q during warm-up, got %q", ReassuranceWarmingUp, st.ReassuranceMessage)
	}

	// Request 2 arrives while warming_up
	go func() {
		defer wg.Done()
		err2 = arb.EnsureReady(context.Background())
	}()

	time.Sleep(30 * time.Millisecond)
	close(wakeBlock) // complete wake
	wg.Wait()

	if err1 != nil {
		t.Errorf("request 1 error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("request 2 error: %v", err2)
	}

	wakeMu.Lock()
	if wakeCount != 1 {
		t.Errorf("expected exactly 1 wakeFunc invocation for coalesced requests, got %d", wakeCount)
	}
	wakeMu.Unlock()

	if arb.State() != ArbitratorStateReady {
		t.Errorf("expected state ready after completion, got %s", arb.State())
	}
}

func TestGPUArbitrator_WakeTimeout_504(t *testing.T) {
	hangingWake := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		WakeTimeout:  50 * time.Millisecond,
		TickInterval: 10 * time.Millisecond,
		InitialState: ArbitratorStateIdleSurrendered,
		WakeFunc:     hangingWake,
	})
	defer arb.Close()

	// Wait for wake timeout buffer to expire
	err := arb.EnsureReady(context.Background())
	if err == nil {
		t.Fatal("expected error on timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrWakeTimeout) {
		t.Errorf("expected context.DeadlineExceeded or ErrWakeTimeout, got: %v", err)
	}
	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Errorf("expected state to revert to idle_surrendered, got %s", arb.State())
	}
}

func TestGPUArbitrator_CPUOptimizedStatus(t *testing.T) {
	arb := NewGPUArbitrator(ArbitratorConfig{
		CPUOptimized: true,
		IsGPU:        false,
		HardwareMode: "cpu",
	})
	defer arb.Close()

	st := arb.Status()
	if !st.CPUOptimized {
		t.Error("expected CPUOptimized to be true")
	}
	if st.HardwareMode != "cpu" {
		t.Errorf("expected HardwareMode 'cpu', got %s", st.HardwareMode)
	}
	if st.ReassuranceMessage != ReassuranceCPUOptimized {
		t.Errorf("expected reassurance microcopy %q, got %q", ReassuranceCPUOptimized, st.ReassuranceMessage)
	}
}

func TestGateway_IdleSurrenderAndWakeIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	validGGUF := createMockGGUFBytes(3, 128, 42)
	if err := os.WriteFile(filepath.Join(tmpDir, "qwen2.5-coder-7b.gguf"), validGGUF, 0o644); err != nil {
		t.Fatalf("failed to write dummy model: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "Woken up successfully"}}]}`))
	}))
	defer upstream.Close()

	wakeCalled := false
	unloadCalled := false

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		WakeTimeout:  90 * time.Second,
		TickInterval: 10 * time.Millisecond,
		InitialState: ArbitratorStateReady,
		UnloadFunc: func(ctx context.Context) error {
			unloadCalled = true
			return nil
		},
		WakeFunc: func(ctx context.Context) error {
			wakeCalled = true
			return nil
		},
	})
	defer arb.Close()

	gw := NewGateway(GatewayConfig{
		ModelsDir:  tmpDir,
		LlamaURL:   upstream.URL,
		Arbitrator: arb,
	})

	// 1. Manually surrender VRAM
	if err := gw.SurrenderIdleVRAM(context.Background()); err != nil {
		t.Fatalf("SurrenderIdleVRAM failed: %v", err)
	}
	if !unloadCalled {
		t.Fatal("expected unloadFunc to be called on SurrenderIdleVRAM")
	}
	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Fatalf("expected state idle_surrendered, got %s", arb.State())
	}

	// 1b. Read-only requests (GET /v1/models and OPTIONS) must bypass wake and remain idle_surrendered
	reqModels := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	recModels := httptest.NewRecorder()
	gw.ServeHTTP(recModels, reqModels)
	if recModels.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for GET /v1/models during idle surrender, got %d", recModels.Code)
	}
	if wakeCalled {
		t.Fatal("expected read-only GET /v1/models NOT to trigger wakeFunc during idle surrender")
	}
	if arb.State() != ArbitratorStateIdleSurrendered {
		t.Fatalf("expected state to remain idle_surrendered after read-only request, got %s", arb.State())
	}

	// 2. Inference request should wake model automatically
	body := `{"model": "qwen2.5-coder-7b.gguf", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 after waking, got %d: %s", rec.Code, rec.Body.String())
	}
	if !wakeCalled {
		t.Fatal("expected wakeFunc to have been triggered by incoming inference request")
	}
	if arb.State() != ArbitratorStateReady {
		t.Fatalf("expected arbitrator to transition back to ready, got %s", arb.State())
	}
}

func TestGateway_WarmupTimeout_504(t *testing.T) {
	tmpDir := t.TempDir()
	validGGUF := createMockGGUFBytes(3, 128, 42)
	if err := os.WriteFile(filepath.Join(tmpDir, "qwen2.5-coder-7b.gguf"), validGGUF, 0o644); err != nil {
		t.Fatalf("failed to write dummy model: %v", err)
	}

	hangingWake := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	arb := NewGPUArbitrator(ArbitratorConfig{
		IdleTimeout:  300 * time.Second,
		WakeTimeout:  40 * time.Millisecond,
		TickInterval: 10 * time.Millisecond,
		InitialState: ArbitratorStateIdleSurrendered,
		WakeFunc:     hangingWake,
	})
	defer arb.Close()

	gw := NewGateway(GatewayConfig{
		ModelsDir:  tmpDir,
		LlamaURL:   "http://127.0.0.1:9",
		Arbitrator: arb,
	})

	body := `{"messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()

	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected HTTP 504 Gateway Timeout, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("expected application/problem+json, got: %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "warm-up timeout expired") {
		t.Errorf("expected warm-up timeout detail, got: %s", rec.Body.String())
	}
}



