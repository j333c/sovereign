package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sovereign/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	srv, err := NewServer(Config{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	return srv, dataDir
}

func TestServer_StorageLayoutInitialization(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// Verify all required subdirectories exist
	for _, sub := range storage.RequiredDataSubdirectories {
		p := filepath.Join(dataDir, sub)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", p, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", p)
		}
	}

	// Verify sovereign.db exists in state
	dbFile := filepath.Join(dataDir, "state", "sovereign.db")
	if _, err := os.Stat(dbFile); err != nil {
		t.Errorf("expected database %q to exist: %v", dbFile, err)
	}
}

func TestServer_HealthProbe_Success(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: body=%s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var res HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}

	if res.Status != "ok" || res.Version != "0.1.0" || res.Database != "connected" {
		t.Errorf("unexpected health response payload: %+v", res)
	}
}

func TestServer_HealthProbe_Failure(t *testing.T) {
	srv, _ := setupTestServer(t)
	// Intentionally close the database to simulate DB failure
	_ = srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %q", contentType)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}

	if prob.Status != http.StatusServiceUnavailable {
		t.Errorf("expected problem status 503, got %d", prob.Status)
	}
}

func TestServer_BootStatus(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: body=%s", w.Code, w.Body.String())
	}

	var res struct {
		Mode                 string `json:"mode"`
		CleanShutdown        bool   `json:"clean_shutdown"`
		DiagnosticsCompleted bool   `json:"diagnostics_completed"`
		Message              string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse boot status response: %v", err)
	}

	if res.Mode != "operational" {
		t.Errorf("expected mode 'operational', got %q", res.Mode)
	}
	if !res.CleanShutdown {
		t.Errorf("expected clean_shutdown = true")
	}
	if !res.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = true")
	}
	if res.Message != "All systems operational" {
		t.Errorf("expected message 'All systems operational', got %q", res.Message)
	}
}

func TestServer_BootStatus_Failure(t *testing.T) {
	srv, _ := setupTestServer(t)
	// Close database to simulate failure
	_ = srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %q", contentType)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}

	if prob.Status != http.StatusInternalServerError {
		t.Errorf("expected problem status 500, got %d", prob.Status)
	}
}

func TestServer_CORS(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS preflight, got %d", w.Code)
	}

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %q", origin)
	}
}

func TestServer_UnwritableDirectory(t *testing.T) {
	_, err := NewServer(Config{
		DataDir: "/proc/nonexistent-data-dir",
	})
	if err == nil {
		t.Fatalf("expected error for unwritable data dir, got nil")
	}
}
