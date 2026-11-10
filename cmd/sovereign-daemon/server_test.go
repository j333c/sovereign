package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/hardware"
	"sovereign/internal/ingress"
	"sovereign/internal/optical"
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"sovereign/internal/transcode"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	// Ensure clean shutdown state for default test server
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngine := supervisor.NewMockRuntimeEngine()
	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})

	mockOpticalDriver := optical.NewMockOpticalDriver()

	srv, err := NewServer(Config{
		DataDir:       dataDir,
		ServiceMgr:    svcMgr,
		OpticalDriver: mockOpticalDriver,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	if srv.OpticalRipper() != nil {
		srv.OpticalRipper().SetEjectResetDelay(0)
	}

	if srv.EnrichmentEngine() != nil {
		srv.EnrichmentEngine().SetHeadroomChecker(func(path string) (float64, error) {
			return 0.50, nil
		})
	}

	if srv.TranscodeEngine() != nil {
		srv.TranscodeEngine().SetTranscoder(transcode.NewMockTranscoder())
		srv.TranscodeEngine().SetHeadroomChecker(func(path string) (float64, error) {
			return 50.0, nil
		})
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

func TestServer_BootStatus_DiagnosticsMode(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	// Cold boot with no boot_state.json -> DiagnosticsMode

	srv, err := NewServer(Config{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
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

	if res.Mode != "diagnostics" {
		t.Errorf("expected mode 'diagnostics', got %q", res.Mode)
	}
	if res.CleanShutdown {
		t.Errorf("expected clean_shutdown = false")
	}
	if res.DiagnosticsCompleted {
		t.Errorf("expected diagnostics_completed = false")
	}
	if res.Message != store.ReassuranceMessage {
		t.Errorf("expected reassurance copy %q, got %q", store.ReassuranceMessage, res.Message)
	}

	// Verify running integrity check transitions to operational
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.BootManager().RunIntegrityCheck(ctx); err != nil {
		t.Fatalf("RunIntegrityCheck failed: %v", err)
	}

	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req)
	var res2 struct {
		Mode                 string `json:"mode"`
		DiagnosticsCompleted bool   `json:"diagnostics_completed"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &res2)
	if res2.Mode != "operational" || !res2.DiagnosticsCompleted {
		t.Errorf("expected operational mode after integrity check passes, got %+v", res2)
	}
}

func TestServer_BootStatus_StorageDegraded(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	srv, err := NewServer(Config{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	// Make state directory read-only
	stateDir := filepath.Join(dataDir, "state")
	_ = os.Chmod(stateDir, 0555)
	defer func() { _ = os.Chmod(stateDir, 0755) }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.BootManager().RunIntegrityCheck(ctx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 for degraded storage, got %d: body=%s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Status != http.StatusServiceUnavailable {
		t.Errorf("expected problem status 503, got %d", prob.Status)
	}
}

func TestServer_CORS(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Without Origin header -> wildcard *
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS preflight, got %d", w.Code)
	}
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %q", origin)
	}

	// 2. With trusted Origin header -> reflected origin and credentials: true
	reqOrigin := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	reqOrigin.Header.Set("Origin", "http://sovereign.local")
	wOrigin := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wOrigin, reqOrigin)

	if origin := wOrigin.Header().Get("Access-Control-Allow-Origin"); origin != "http://sovereign.local" {
		t.Errorf("expected Access-Control-Allow-Origin http://sovereign.local, got %q", origin)
	}
	if creds := wOrigin.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials true, got %q", creds)
	}

	// 3. With untrusted Origin header -> fallback to wildcard without credentials
	reqUntrusted := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	reqUntrusted.Header.Set("Origin", "https://untrusted-external-site.com")
	wUntrusted := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUntrusted, reqUntrusted)

	if origin := wUntrusted.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %q", origin)
	}
	if creds := wUntrusted.Header().Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected empty Access-Control-Allow-Credentials for untrusted origin, got %q", creds)
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

type mockIngressManager struct {
	status     map[string]any
	err        error
	syncCalls  int
	lastRoutes []ingress.RouteConfig
	mu         sync.Mutex
}

func (m *mockIngressManager) SyncRoutes(ctx context.Context, routes []ingress.RouteConfig) error {
	m.mu.Lock()
	m.syncCalls++
	m.lastRoutes = make([]ingress.RouteConfig, len(routes))
	copy(m.lastRoutes, routes)
	m.mu.Unlock()
	return m.err
}

func (m *mockIngressManager) LastRoutes() []ingress.RouteConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	routes := make([]ingress.RouteConfig, len(m.lastRoutes))
	copy(routes, m.lastRoutes)
	return routes
}

func (m *mockIngressManager) SyncCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncCalls
}

func (m *mockIngressManager) GetStatus(ctx context.Context) (map[string]any, error) {
	return m.status, m.err
}

func TestServer_IngressStatus_Success(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	cfg := ingress.DefaultMDNSConfig(nil)
	cfg.IPs = []string{"192.168.1.100"}
	mdnsSvc := ingress.NewMDNSService(cfg)
	_ = mdnsSvc.Start()

	srv, err := NewServer(Config{
		DataDir:     dataDir,
		IngressMgr:  mockIngress,
		MDNSService: mdnsSvc,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: body=%s", w.Code, w.Body.String())
	}

	var res IngressStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse ingress status response: %v", err)
	}

	if res.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", res.Status)
	}
	if !res.MDNSActive {
		t.Errorf("expected MDNSActive = true")
	}
	if !res.CaddyConnected {
		t.Errorf("expected CaddyConnected = true")
	}
	if res.Hostname != "sovereign.local" {
		t.Errorf("expected hostname sovereign.local, got %s", res.Hostname)
	}
	if len(res.Routes) != 3 {
		t.Errorf("expected 3 default routes, got %d", len(res.Routes))
	}
}

func TestServer_IngressStatus_DegradedAndError(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	// 1. Caddy disconnected -> status = "error"
	mockIngressOffline := &mockIngressManager{
		status: map[string]any{"connected": false, "status": "error"},
	}
	mdnsOffline := ingress.NewMDNSService(ingress.DefaultMDNSConfig(nil))

	srvErr, err := NewServer(Config{
		DataDir:     dataDir,
		IngressMgr:  mockIngressOffline,
		MDNSService: mdnsOffline,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srvErr.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	w := httptest.NewRecorder()
	srvErr.Handler().ServeHTTP(w, req)

	var resErr IngressStatusResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resErr)
	if resErr.Status != "error" || resErr.CaddyConnected != false {
		t.Errorf("expected status 'error' when caddy disconnected, got %+v", resErr)
	}

	// 2. Caddy connected, but mDNS offline -> status = "degraded"
	mockIngressOnline := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}
	tempDir2 := t.TempDir()
	dataDir2 := filepath.Join(tempDir2, "data")
	srvDegraded, err := NewServer(Config{
		DataDir:     dataDir2,
		IngressMgr:  mockIngressOnline,
		MDNSService: mdnsOffline, // inactive
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srvDegraded.Close()

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	w2 := httptest.NewRecorder()
	srvDegraded.Handler().ServeHTTP(w2, req2)

	var resDegraded IngressStatusResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &resDegraded)
	if resDegraded.Status != "degraded" || !resDegraded.CaddyConnected || resDegraded.MDNSActive {
		t.Errorf("expected status 'degraded' when mdns inactive, got %+v", resDegraded)
	}
}

func TestServer_StartBackgroundServices(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	cfg := ingress.DefaultMDNSConfig(nil)
	cfg.IPs = []string{"127.0.0.1"}
	mdnsSvc := ingress.NewMDNSService(cfg)

	srv, err := NewServer(Config{
		DataDir:     dataDir,
		IngressMgr:  mockIngress,
		MDNSService: mdnsSvc,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.StartBackgroundServices(ctx)

	// Wait up to 1 second for background routines to execute
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if mockIngress.SyncCalls() > 0 && mdnsSvc.IsActive() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if calls := mockIngress.SyncCalls(); calls == 0 {
		t.Errorf("expected Caddy SyncRoutes to be called by background routine, got %d calls", calls)
	}
	if !mdnsSvc.IsActive() {
		t.Errorf("expected mDNS responder to be active after StartBackgroundServices")
	}
	if srv.EnrichmentEngine() == nil {
		t.Errorf("expected EnrichmentEngine to be initialized")
	}
	if srv.OpticalRipper() == nil {
		t.Errorf("expected OpticalRipper to be initialized")
	}
}

func TestServer_StartBackgroundServices_DiagnosticsAutoIntegrityCheck(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	// No boot_state.json -> Server initializes in DiagnosticsMode

	srv, err := NewServer(Config{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	if !srv.BootManager().IsDiagnosticsMode() {
		t.Fatalf("expected server to start in diagnostics mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.StartBackgroundServices(ctx)

	// Wait up to 2 seconds for background integrity check routine to complete
	deadline := time.Now().Add(2 * time.Second)
	completed := false
	for time.Now().Before(deadline) {
		status, _, err := srv.BootManager().GetStatus()
		if err == nil && status.Mode == "operational" && status.DiagnosticsCompleted {
			completed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !completed {
		t.Fatalf("expected background services to automatically run integrity check and transition to operational")
	}
}

func TestServer_SSE_TelemetryStreaming(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockHw := &hardware.MockCollector{
		Snapshot: &hardware.TelemetrySnapshot{
			CPUUsagePercent:    18.5,
			MemoryUsedBytes:    4294967296,    // 4 GB
			MemoryTotalBytes:   17179869184,   // 16 GB
			StorageUsedBytes:   1099511627776, // 1 TB
			StorageTotalBytes:  2199023255552, // 2 TB
			TemperatureCelsius: 41.2,
		},
	}

	srv, err := NewServer(Config{
		DataDir:           dataDir,
		HardwareCollector: mockHw,
		TelemetryInterval: 50 * time.Millisecond, // Fast ticker for testing
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Unauthenticated client can stream /api/v1/system/events
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/system/events", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got: %s", ct)
	}

	// Read initial SSE message
	buf := make([]byte, 1024)

	var fullText string
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			fullText += string(buf[:n])
			if strings.Contains(fullText, "\n\n") {
				break
			}
		}
		if err != nil {
			t.Fatalf("error reading SSE stream: %v", err)
		}
	}

	firstMsg := fullText
	if idx := strings.Index(fullText, "\n\n"); idx != -1 {
		firstMsg = fullText[:idx]
	}

	splitLines := strings.Split(firstMsg, "\n")
	var eventName, dataJSON string
	for _, l := range splitLines {
		if strings.HasPrefix(l, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(l, "event:"))
		} else if strings.HasPrefix(l, "data:") {
			dataJSON = strings.TrimSpace(strings.TrimPrefix(l, "data:"))
		}
	}

	if eventName != "telemetry.update" {
		t.Fatalf("expected event: telemetry.update, got: %s", eventName)
	}

	var update TelemetryUpdate
	if err := json.Unmarshal([]byte(dataJSON), &update); err != nil {
		t.Fatalf("failed to unmarshal telemetry update JSON: %v, raw=%s", err, dataJSON)
	}

	if update.CPUPercent != 18.5 {
		t.Errorf("expected CPU 18.5, got %f", update.CPUPercent)
	}
	if update.MemoryUsedBytes != 4294967296 {
		t.Errorf("expected MemoryUsedBytes 4294967296, got %d", update.MemoryUsedBytes)
	}
	if update.MemoryTotalBytes != 17179869184 {
		t.Errorf("expected MemoryTotalBytes 17179869184, got %d", update.MemoryTotalBytes)
	}
	if update.StorageUsedBytes == 0 {
		t.Errorf("expected non-zero StorageUsedBytes, got 0")
	}
	if update.StorageTotalBytes == 0 {
		t.Errorf("expected non-zero StorageTotalBytes, got 0")
	}
	if update.StoragePercent < 0 || update.StoragePercent > 100 {
		t.Errorf("expected StoragePercent between 0 and 100, got %f", update.StoragePercent)
	}
	if update.NASStatus != "not_configured" {
		t.Errorf("expected NASStatus not_configured, got %s", update.NASStatus)
	}
	if update.OverallStatus != "operational" {
		t.Errorf("expected OverallStatus operational, got %s", update.OverallStatus)
	}
	if update.Timestamp == "" {
		t.Errorf("expected non-empty timestamp")
	}
	if update.OpticalStatus == "" {
		t.Errorf("expected non-empty optical status")
	}
	if update.GPUStatus == "" {
		t.Errorf("expected non-empty GPU status")
	}
}

func TestServer_SSE_EventsAlias(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/events alias: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/events alias, got: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got: %s", ct)
	}
}

func loginTestAdmin(t *testing.T, srv *Server) *http.Cookie {
	t.Helper()
	setupPayload := `{
		"username": "admin",
		"password": "HouseholdMasterKey2026!",
		"email": "alex@family.net",
		"language": "de",
		"features": ["video", "audio", "dms", "ai"]
	}`
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(setupPayload))
	wSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSetup, reqSetup)
	if wSetup.Code != http.StatusOK {
		t.Fatalf("failed to complete setup: %d", wSetup.Code)
	}

	for _, c := range wSetup.Result().Cookies() {
		if c.Name == "sovereign_session" {
			return c
		}
	}
	t.Fatalf("session cookie not found in setup response")
	return nil
}

func TestServer_SystemReboot(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	cookie := loginTestAdmin(t, srv)

	// Unauthenticated request should fail with 401
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/reboot", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	// Authenticated request should succeed with 200
	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/reboot", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp["status"] != "rebooting" {
		t.Errorf("expected status 'rebooting', got %v", resp["status"])
	}
}

func TestServer_FactoryReset(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	cookie := loginTestAdmin(t, srv)

	// Unauthenticated request should fail with 401
	body := strings.NewReader(`{"confirmation":"RESET"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/factory-reset", body)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	// Authenticated with invalid confirmation should fail with 400
	body = strings.NewReader(`{"confirmation":"WRONG"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/factory-reset", body)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid confirmation, got %d: %s", rr.Code, rr.Body.String())
	}

	// Authenticated with valid confirmation "RESET" should succeed with 200
	body = strings.NewReader(`{"confirmation":"RESET"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/factory-reset", body)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp["status"] != "resetting" {
		t.Errorf("expected status 'resetting', got %v", resp["status"])
	}

	// Verify database persistence of setup_completed flag
	val, err := srv.db.GetSetting(context.Background(), "setup_completed")
	if err != nil {
		t.Fatalf("failed to read setup_completed setting after reset: %v", err)
	}
	if val != "false" {
		t.Errorf("expected setup_completed to be 'false', got %q", val)
	}

	// Verify GET /api/v1/auth/setup-status reports is_configured: false
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-status", nil)
	rrStatus := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrStatus, reqStatus)
	if rrStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from setup-status, got %d", rrStatus.Code)
	}
	var statusMap map[string]any
	if err := json.Unmarshal(rrStatus.Body.Bytes(), &statusMap); err != nil {
		t.Fatalf("failed to decode setup-status JSON: %v", err)
	}
	if isConfigured, ok := statusMap["is_configured"].(bool); !ok || isConfigured {
		t.Errorf("expected is_configured to be false after factory reset, got %v", statusMap["is_configured"])
	}
}

func TestDockerSocket_MissingWarning(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	missingSockPath := filepath.Join(tempDir, "nonexistent-docker.sock")
	mockOpticalDriver := optical.NewMockOpticalDriver()

	srv, err := NewServer(Config{
		DataDir:          dataDir,
		Logger:           logger,
		DockerSocketPath: missingSockPath,
		OpticalDriver:    mockOpticalDriver,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	// 1. Verify warning log output surfaces reassuring consumer copy
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Docker socket unavailable") {
		t.Errorf("expected warning log for missing Docker socket, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Container engine unavailable — background tasks paused") {
		t.Errorf("expected reassuring microcopy in daemon logs, got: %s", logOutput)
	}

	// 2. Verify getTelemetryUpdate marks docker_available: false, docker_status: unavailable, overall_status: degraded
	ctx := context.Background()
	telemetry := srv.getTelemetryUpdate(ctx)
	if telemetry.DockerAvailable {
		t.Errorf("expected docker_available = false when socket missing, got true")
	}
	if telemetry.DockerStatus != "degraded" {
		t.Errorf("expected docker_status = 'degraded', got %q", telemetry.DockerStatus)
	}
	if telemetry.DockerMessage != "Container engine unavailable — background tasks paused" {
		t.Errorf("expected reassuring docker message, got %q", telemetry.DockerMessage)
	}

	// 3. Verify SSE stream includes telemetry.update with docker fields and system.docker_unavailable event
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()

	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	req = req.WithContext(sseCtx)

	doneCh := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(w, req)
		close(doneCh)
	}()

	// Wait briefly for initial events to flush, then cancel request
	time.Sleep(50 * time.Millisecond)
	sseCancel()
	<-doneCh

	body := w.Body.String()
	if !strings.Contains(body, "event: telemetry.update") {
		t.Fatalf("expected SSE stream to contain telemetry.update event, got: %s", body)
	}
	if !strings.Contains(body, `"docker_available":false`) {
		t.Errorf("expected SSE telemetry.update data to contain docker_available: false, got: %s", body)
	}
	if !strings.Contains(body, "event: system.docker_unavailable") {
		t.Errorf("expected SSE stream to emit system.docker_unavailable event, got: %s", body)
	}

	// 4. Verify non-container features (health, boot status) continue without error or panics
	healthReq := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	healthRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Errorf("expected /api/v1/health to return 200 OK without docker socket, got %d", healthRec.Code)
	}

	bootReq := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	bootRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(bootRec, bootReq)
	if bootRec.Code != http.StatusOK {
		t.Errorf("expected /api/v1/system/boot-status to return 200 OK without docker socket, got %d", bootRec.Code)
	}
}

func TestDockerSocket_PresentHealthy(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	// Create genuine unix domain socket
	sockPath := filepath.Join(tempDir, "docker.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix domain socket listener: %v", err)
	}
	defer l.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	mockOpticalDriver := optical.NewMockOpticalDriver()

	srv, err := NewServer(Config{
		DataDir:          dataDir,
		Logger:           logger,
		DockerSocketPath: sockPath,
		OpticalDriver:    mockOpticalDriver,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	// 1. Verify getTelemetryUpdate marks DockerAvailable: true, DockerStatus: "connected"
	ctx := context.Background()
	telemetry := srv.getTelemetryUpdate(ctx)
	if !telemetry.DockerAvailable {
		t.Errorf("expected docker_available = true when socket present, got false")
	}
	if telemetry.DockerStatus != "connected" {
		t.Errorf("expected docker_status = 'connected', got %q", telemetry.DockerStatus)
	}
	if telemetry.DockerMessage != "Container engine operational" {
		t.Errorf("expected operational docker message, got %q", telemetry.DockerMessage)
	}

	// 2. Verify SSE stream includes telemetry.update with docker fields and NO system.docker_unavailable event
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()

	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	req = req.WithContext(sseCtx)

	doneCh := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(w, req)
		close(doneCh)
	}()

	time.Sleep(50 * time.Millisecond)
	sseCancel()
	<-doneCh

	body := w.Body.String()
	if !strings.Contains(body, "event: telemetry.update") {
		t.Fatalf("expected SSE stream to contain telemetry.update event, got: %s", body)
	}
	if !strings.Contains(body, `"docker_available":true`) {
		t.Errorf("expected SSE telemetry.update to contain docker_available: true, got: %s", body)
	}
	if !strings.Contains(body, `"docker_status":"connected"`) {
		t.Errorf("expected SSE telemetry.update to contain docker_status: connected, got: %s", body)
	}
	if strings.Contains(body, "event: system.docker_unavailable") {
		t.Errorf("expected SSE stream NOT to emit system.docker_unavailable event when socket present, got: %s", body)
	}
}

func TestServer_BootStatusOvernightUpdateAndDismiss(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()

	// 1. Initial boot-status has no overnight update status
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w1.Code)
	}
	var res1 map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &res1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 2. Set overnight update status to updated
	if err := srv.db.SetOvernightUpdateStatus(ctx, "updated", "v1.1.0", "Sovereign updated to v1.1.0 overnight"); err != nil {
		t.Fatalf("failed to set overnight update status: %v", err)
	}

	// 3. GET /api/v1/system/boot-status reflects updated outcome
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w2.Code)
	}
	var res2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &res2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res2["last_update_status"] != "updated" {
		t.Errorf("expected last_update_status 'updated', got %v", res2["last_update_status"])
	}
	if res2["last_update_version"] != "v1.1.0" {
		t.Errorf("expected last_update_version 'v1.1.0', got %v", res2["last_update_version"])
	}
	if res2["last_update_dismissed"] != false {
		t.Errorf("expected last_update_dismissed false, got %v", res2["last_update_dismissed"])
	}

	// 4. POST /api/v1/system/update-status/dismiss
	reqDismiss := httptest.NewRequest(http.MethodPost, "/api/v1/system/update-status/dismiss", nil)
	wDismiss := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDismiss, reqDismiss)
	if wDismiss.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dismiss, got %d", wDismiss.Code)
	}

	// 5. Verify boot-status is now dismissed
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/system/boot-status", nil)
	w3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w3, req3)
	var res3 map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &res3); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res3["last_update_dismissed"] != true {
		t.Errorf("expected last_update_dismissed true after dismiss, got %v", res3["last_update_dismissed"])
	}
}
