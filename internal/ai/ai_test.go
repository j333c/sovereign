package ai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

	gw := NewGateway(GatewayConfig{
		ModelsDir: tmpDir,
		LlamaURL:  "http://127.0.0.1:8080",
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
