package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/enrichment"
	"sovereign/internal/hardware"
	"sovereign/internal/ingress"
	"sovereign/internal/optical"
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"sovereign/internal/transcode"
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

func TestServer_StorageDisks(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Inject mock disks
	defaultDiskOps := srv.StorageEngine().DiskOps().(*storage.DefaultDiskOps)
	defaultDiskOps.SetMockDisks([]storage.DiskInfo{
		{
			Name:       "nvme0n1",
			Path:       "/dev/nvme0n1",
			Model:      "Samsung SSD 980 1TB",
			SizeBytes:  1000204886016,
			Type:       "nvme",
			Rotational: false,
			Mounted:    false,
			IsSystem:   false,
		},
		{
			Name:       "sda",
			Path:       "/dev/sda",
			Model:      "System Boot Disk",
			SizeBytes:  500107862016,
			Type:       "sata",
			Rotational: false,
			Mounted:    true,
			MountPoint: "/",
			IsSystem:   true,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/disks", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: body=%s", w.Code, w.Body.String())
	}

	var res struct {
		Disks []storage.DiskInfo `json:"disks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode disks response: %v", err)
	}

	if len(res.Disks) != 1 {
		t.Fatalf("expected 1 unmounted disk, got %d", len(res.Disks))
	}
	if res.Disks[0].Path != "/dev/nvme0n1" {
		t.Errorf("expected available disk /dev/nvme0n1, got %s", res.Disks[0].Path)
	}

	// Empty list case
	defaultDiskOps.SetMockDisks([]storage.DiskInfo{})
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/storage/disks", nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w2.Code)
	}
	var res2 struct {
		Disks []storage.DiskInfo `json:"disks"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &res2)
	if len(res2.Disks) != 0 {
		t.Errorf("expected empty array, got %d disks", len(res2.Disks))
	}
}

func TestServer_StorageInitialize(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	defaultDiskOps := srv.StorageEngine().DiskOps().(*storage.DefaultDiskOps)
	defaultDiskOps.SetMockDisks([]storage.DiskInfo{
		{
			Name:     "nvme0n1",
			Path:     "/dev/nvme0n1",
			Mounted:  false,
			IsSystem: false,
		},
		{
			Name:       "sda",
			Path:       "/dev/sda",
			Mounted:    true,
			MountPoint: "/",
			IsSystem:   true,
		},
	})

	// Case 1: Reject formatting system/boot disk
	bodyProtected, _ := json.Marshal(map[string]string{"device": "/dev/sda"})
	reqProtected := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewReader(bodyProtected))
	wProtected := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wProtected, reqProtected)

	if wProtected.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for system partition, got %d: body=%s", wProtected.Code, wProtected.Body.String())
	}
	var probProtected ProblemDetails
	_ = json.Unmarshal(wProtected.Body.Bytes(), &probProtected)
	if probProtected.Title != "Invalid Device" {
		t.Errorf("expected title 'Invalid Device', got %q", probProtected.Title)
	}

	// Case 2: Reject invalid/empty device path
	bodyEmpty, _ := json.Marshal(map[string]string{"device": ""})
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewReader(bodyEmpty))
	wEmpty := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty device, got %d", wEmpty.Code)
	}

	// Case 3: Successful formatting and layout creation
	bodyValid, _ := json.Marshal(map[string]string{"device": "/dev/nvme0n1"})
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewReader(bodyValid))
	wValid := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid device, got %d: body=%s", wValid.Code, wValid.Body.String())
	}
	var initRes storage.StoragePoolInfo
	if err := json.Unmarshal(wValid.Body.Bytes(), &initRes); err != nil {
		t.Fatalf("failed to parse init response: %v", err)
	}
	if initRes.Status != "initialized" || initRes.MountPoint != dataDir {
		t.Errorf("unexpected init response: %+v", initRes)
	}
}

func TestServer_StorageNASTest(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	defaultNASOps := srv.StorageEngine().NASOps().(*storage.DefaultNASOps)

	// Valid credentials test
	bodyValid, _ := json.Marshal(storage.NASTestRequest{
		Type:     "smb",
		Host:     "192.168.1.100",
		Share:    "media",
		Username: "family",
		Password: "secretpassword",
	})
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/test", bytes.NewReader(bodyValid))
	wValid := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: body=%s", wValid.Code, wValid.Body.String())
	}
	var testRes storage.NASTestResponse
	_ = json.Unmarshal(wValid.Body.Bytes(), &testRes)
	if !testRes.Success {
		t.Errorf("expected testRes.Success = true")
	}

	// Unreachable host test
	defaultNASOps.SetMockReachability(false)
	bodyUnreachable, _ := json.Marshal(storage.NASTestRequest{
		Type:  "smb",
		Host:  "unreachable.nas.local",
		Share: "media",
	})
	reqUnreachable := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/test", bytes.NewReader(bodyUnreachable))
	wUnreachable := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUnreachable, reqUnreachable)

	if wUnreachable.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", wUnreachable.Code)
	}
	var prob ProblemDetails
	_ = json.Unmarshal(wUnreachable.Body.Bytes(), &prob)
	if !strings.Contains(prob.Detail, "unreachable") {
		t.Errorf("expected unreachable detail, got %q", prob.Detail)
	}
	defaultNASOps.SetMockReachability(true)

	// Auth failure test
	defaultNASOps.SetMockAuthSuccess(false)
	bodyAuthFail, _ := json.Marshal(storage.NASTestRequest{
		Type:     "smb",
		Host:     "192.168.1.100",
		Share:    "media",
		Username: "baduser",
		Password: "badpassword",
	})
	reqAuthFail := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/test", bytes.NewReader(bodyAuthFail))
	wAuthFail := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAuthFail, reqAuthFail)

	if wAuthFail.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for auth failure, got %d", wAuthFail.Code)
	}
}

func TestServer_StorageNASMountAndStatus(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	defaultNASOps := srv.StorageEngine().NASOps().(*storage.DefaultNASOps)

	// Mount NAS
	mountBody, _ := json.Marshal(storage.NASMountRequest{
		Type:     "smb",
		Host:     "192.168.1.200",
		Share:    "multimedia",
		Username: "household",
		Password: "confidential_password",
	})
	reqMount := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/mount", bytes.NewReader(mountBody))
	wMount := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wMount, reqMount)

	if wMount.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for mount, got %d: body=%s", wMount.Code, wMount.Body.String())
	}
	var mountRes storage.NASMountResponse
	_ = json.Unmarshal(wMount.Body.Bytes(), &mountRes)
	if mountRes.Status != "mounted" || mountRes.MountPoint != filepath.Join(dataDir, "media") {
		t.Errorf("unexpected mount response: %+v", mountRes)
	}

	// GET /api/v1/storage/status - Connected state
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/storage/status", nil)
	wStatus := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", wStatus.Code)
	}
	var statusRes storage.StorageStatusResponse
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusRes); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	if statusRes.Status != "ok" || statusRes.PoolType != "nas" {
		t.Errorf("expected ok/nas status, got %+v", statusRes)
	}
	// Invariant: password redacted as '***'
	if statusRes.NAS == nil || statusRes.NAS.Password != "***" {
		t.Errorf("expected redacted password '***', got %+v", statusRes.NAS)
	}
	if statusRes.NAS.Status != "connected" {
		t.Errorf("expected NAS connected, got %s", statusRes.NAS.Status)
	}

	// Simulate NAS disconnection in runtime
	defaultNASOps.SetMockReachability(false)

	reqStatusDegraded := httptest.NewRequest(http.MethodGet, "/api/v1/storage/status", nil)
	wStatusDegraded := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatusDegraded, reqStatusDegraded)

	// Daemon must remain healthy and return HTTP 200 with degraded telemetry
	if wStatusDegraded.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for degraded telemetry, got %d", wStatusDegraded.Code)
	}
	var statusDegraded storage.StorageStatusResponse
	_ = json.Unmarshal(wStatusDegraded.Body.Bytes(), &statusDegraded)

	if statusDegraded.Status != "degraded" {
		t.Errorf("expected overall status 'degraded', got %q", statusDegraded.Status)
	}
	if statusDegraded.NAS.Status != "degraded" {
		t.Errorf("expected NAS status 'degraded', got %q", statusDegraded.NAS.Status)
	}
	if statusDegraded.NAS.Message != "Network storage temporarily unavailable" {
		t.Errorf("expected consumer message 'Network storage temporarily unavailable', got %q", statusDegraded.NAS.Message)
	}

	// Automatic recovery back to connected (Green)
	defaultNASOps.SetMockReachability(true)
	reqStatusRecovered := httptest.NewRequest(http.MethodGet, "/api/v1/storage/status", nil)
	wStatusRecovered := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatusRecovered, reqStatusRecovered)

	var statusRecovered storage.StorageStatusResponse
	_ = json.Unmarshal(wStatusRecovered.Body.Bytes(), &statusRecovered)
	if statusRecovered.Status != "ok" || statusRecovered.NAS.Status != "connected" {
		t.Errorf("expected recovered ok/connected, got %+v", statusRecovered)
	}
}

func TestServer_StorageStatus_InitialUnconfigured(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	var res storage.StorageStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Status != "not_configured" {
		t.Errorf("expected status 'not_configured', got %q", res.Status)
	}
	if res.PoolType != "none" {
		t.Errorf("expected pool_type 'none', got %q", res.PoolType)
	}
	if res.LocalPool != nil {
		t.Errorf("expected nil local_pool, got %+v", res.LocalPool)
	}
	if res.NAS == nil || res.NAS.Configured {
		t.Errorf("expected NAS configured = false, got %+v", res.NAS)
	}
	if res.NAS.Status != "disconnected" {
		t.Errorf("expected NAS status 'disconnected', got %q", res.NAS.Status)
	}
}

func TestServer_StorageInitialize_DeviceInUse(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	defaultOps := srv.StorageEngine().DiskOps().(*storage.DefaultDiskOps)
	defaultOps.SetMockDisks([]storage.DiskInfo{
		{
			Name:       "sdc",
			Path:       "/dev/sdc",
			Mounted:    true,
			MountPoint: "/mnt/external",
			IsSystem:   false, // Non-system device, but mounted
		},
	})

	body, _ := json.Marshal(map[string]string{"device": "/dev/sdc"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for mounted device, got %d: body=%s", w.Code, w.Body.String())
	}
	var prob ProblemDetails
	_ = json.Unmarshal(w.Body.Bytes(), &prob)
	if !strings.Contains(prob.Detail, "currently in use or mounted") {
		t.Errorf("expected detail containing 'currently in use or mounted', got %q", prob.Detail)
	}
}

func TestServer_StorageNASMount_Errors(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	defaultNASOps := srv.StorageEngine().NASOps().(*storage.DefaultNASOps)

	// Case 1: Unreachable host
	defaultNASOps.SetMockReachability(false)
	bodyUnreachable, _ := json.Marshal(storage.NASMountRequest{
		Type:  "smb",
		Host:  "unreachable.host",
		Share: "media",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/mount", bytes.NewReader(bodyUnreachable))
	w1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1, req1)

	if w1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for unreachable host, got %d: body=%s", w1.Code, w1.Body.String())
	}
	defaultNASOps.SetMockReachability(true)

	// Case 2: Auth failure
	defaultNASOps.SetMockAuthSuccess(false)
	bodyAuthFail, _ := json.Marshal(storage.NASMountRequest{
		Type:     "smb",
		Host:     "192.168.1.100",
		Share:    "media",
		Username: "baduser",
		Password: "badpassword",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/mount", bytes.NewReader(bodyAuthFail))
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for auth failure, got %d: body=%s", w2.Code, w2.Body.String())
	}
	defaultNASOps.SetMockAuthSuccess(true)

	// Case 3: Path traversal attempt
	bodyTraversal, _ := json.Marshal(storage.NASMountRequest{
		Type:       "smb",
		Host:       "192.168.1.100",
		Share:      "media",
		TargetPath: "../../etc",
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/mount", bytes.NewReader(bodyTraversal))
	w3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for path traversal, got %d", w3.Code)
	}
}

func TestServer_StoragePersistenceRestorationOnRestart(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	// Server 1
	srv1, err := NewServer(Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("failed to create srv1: %v", err)
	}
	defaultDiskOps1 := srv1.StorageEngine().DiskOps().(*storage.DefaultDiskOps)
	defaultDiskOps1.SetMockDisks([]storage.DiskInfo{
		{Name: "nvme0n1", Path: "/dev/nvme0n1", Mounted: false, IsSystem: false},
	})

	// Initialize local storage pool
	initBody, _ := json.Marshal(map[string]string{"device": "/dev/nvme0n1"})
	reqInit := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewReader(initBody))
	wInit := httptest.NewRecorder()
	srv1.Handler().ServeHTTP(wInit, reqInit)
	if wInit.Code != http.StatusOK {
		t.Fatalf("Initialize failed on srv1: %s", wInit.Body.String())
	}

	// Mount NAS
	mountBody, _ := json.Marshal(storage.NASMountRequest{
		Type:     "smb",
		Host:     "backup.nas",
		Share:    "videos",
		Username: "user",
		Password: "secretpassword",
	})
	reqMount := httptest.NewRequest(http.MethodPost, "/api/v1/storage/nas/mount", bytes.NewReader(mountBody))
	wMount := httptest.NewRecorder()
	srv1.Handler().ServeHTTP(wMount, reqMount)
	if wMount.Code != http.StatusOK {
		t.Fatalf("Mount failed on srv1: %s", wMount.Body.String())
	}

	// Close Server 1
	_ = srv1.Close()

	// Server 2 (restart with same data directory)
	srv2, err := NewServer(Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("failed to create srv2: %v", err)
	}
	defer srv2.Close()

	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/storage/status", nil)
	wStatus := httptest.NewRecorder()
	srv2.Handler().ServeHTTP(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("Status failed on srv2: %d", wStatus.Code)
	}
	var status storage.StorageStatusResponse
	_ = json.Unmarshal(wStatus.Body.Bytes(), &status)

	if status.PoolType != "hybrid" {
		t.Errorf("expected pool_type 'hybrid', got %q", status.PoolType)
	}
	if status.LocalPool == nil || status.LocalPool.Device != "/dev/nvme0n1" {
		t.Errorf("expected restored local pool, got %+v", status.LocalPool)
	}
	if status.NAS == nil || !status.NAS.Configured {
		t.Fatalf("expected restored NAS config, got %+v", status.NAS)
	}
	if status.NAS.Host != "backup.nas" || status.NAS.Share != "videos" {
		t.Errorf("expected restored NAS host/share, got %+v", status.NAS)
	}
	if status.NAS.Password != "***" {
		t.Errorf("expected redacted password '***', got %q", status.NAS.Password)
	}
}

func TestServer_AuthAndSetupWizardFlow(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Initial boot: verify setup-status reports is_configured: false
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-status", nil)
	wStatus := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 from setup-status, got: %d", wStatus.Code)
	}

	var statusMap map[string]any
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusMap); err != nil {
		t.Fatalf("failed to decode setup status response: %v", err)
	}
	if isConfigured, ok := statusMap["is_configured"].(bool); !ok || isConfigured {
		t.Fatalf("expected is_configured: false, got: %v", statusMap["is_configured"])
	}

	// 2. Setup validation: password < 8 characters
	bodyShortPass := bytes.NewBufferString(`{"password": "short", "email": "admin@family.net"}`)
	reqShortPass := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bodyShortPass)
	wShortPass := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wShortPass, reqShortPass)
	if wShortPass.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got: %d", wShortPass.Code)
	}

	// 3. Setup validation: invalid email
	bodyBadEmail := bytes.NewBufferString(`{"password": "validPassword123!", "email": "not-an-email"}`)
	reqBadEmail := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bodyBadEmail)
	wBadEmail := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBadEmail, reqBadEmail)
	if wBadEmail.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got: %d", wBadEmail.Code)
	}

	// 4. Valid setup submission (Step 5)
	setupPayload := `{
		"username": "admin",
		"password": "HouseholdMasterKey2026!",
		"email": "alex@family.net",
		"language": "de",
		"features": ["video", "audio", "dms", "ai"],
		"hostname": "sovereign.local"
	}`
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(setupPayload))
	wSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSetup, reqSetup)
	if wSetup.Code != http.StatusOK {
		t.Fatalf("expected 200 for setup completion, got: %d: %s", wSetup.Code, wSetup.Body.String())
	}

	// Check cookie was set
	cookieHeader := wSetup.Header().Get("Set-Cookie")
	if !strings.Contains(cookieHeader, "sovereign_session=") ||
		!strings.Contains(cookieHeader, "HttpOnly") ||
		!strings.Contains(cookieHeader, "SameSite=Strict") {
		t.Fatalf("expected HttpOnly SameSite=Strict cookie in Set-Cookie, got: %s", cookieHeader)
	}

	var setupRes map[string]any
	if err := json.Unmarshal(wSetup.Body.Bytes(), &setupRes); err != nil {
		t.Fatalf("failed to decode setup response: %v", err)
	}
	if configured, ok := setupRes["is_configured"].(bool); !ok || !configured {
		t.Fatalf("expected is_configured: true in setup response, got: %v", setupRes)
	}

	// Extract session token from cookie
	cookies := wSetup.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "sovereign_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected sovereign_session cookie to be returned")
	}

	// 5. Subsequent setup-status should now return is_configured: true
	reqStatus2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-status", nil)
	wStatus2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatus2, reqStatus2)
	if wStatus2.Code != http.StatusOK {
		t.Fatalf("expected 200 from setup-status, got: %d", wStatus2.Code)
	}
	var statusMap2 map[string]any
	_ = json.Unmarshal(wStatus2.Body.Bytes(), &statusMap2)
	if isConfigured, ok := statusMap2["is_configured"].(bool); !ok || !isConfigured {
		t.Fatalf("expected is_configured: true post-setup, got: %v", statusMap2)
	}

	// 6. Duplicate setup attempt must be rejected with 409 Conflict Problem Details
	reqDupSetup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(setupPayload))
	wDupSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDupSetup, reqDupSetup)
	if wDupSetup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate setup, got: %d", wDupSetup.Code)
	}

	// 7. Session inspection: unauthenticated vs authenticated
	reqNoSess := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	wNoSess := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wNoSess, reqNoSess)
	if wNoSess.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated session check, got: %d", wNoSess.Code)
	}

	reqWithSess := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	reqWithSess.AddCookie(sessionCookie)
	wWithSess := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wWithSess, reqWithSess)
	if wWithSess.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated session check, got: %d: %s", wWithSess.Code, wWithSess.Body.String())
	}

	// 8. Protected Settings endpoint: unauthenticated vs authenticated
	reqSettingsNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	wSettingsNoAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSettingsNoAuth, reqSettingsNoAuth)
	if wSettingsNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /api/v1/settings without auth, got: %d", wSettingsNoAuth.Code)
	}

	reqSettingsAuth := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	reqSettingsAuth.AddCookie(sessionCookie)
	wSettingsAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSettingsAuth, reqSettingsAuth)
	if wSettingsAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/settings with session cookie, got: %d", wSettingsAuth.Code)
	}
	var settingsRes map[string]any
	_ = json.Unmarshal(wSettingsAuth.Body.Bytes(), &settingsRes)
	if settingsRes["language"] != "de" || settingsRes["admin_email"] != "alex@family.net" {
		t.Errorf("unexpected settings content: %+v", settingsRes)
	}
	if settingsRes["features"] != "video,audio,dms,ai" {
		t.Errorf("expected features 'video,audio,dms,ai', got: %v", settingsRes["features"])
	}
	if settingsRes["hostname"] != "sovereign.local" {
		t.Errorf("expected hostname 'sovereign.local', got: %v", settingsRes["hostname"])
	}

	// 9. Update Settings via POST /api/v1/settings: unauthenticated vs authenticated
	reqUpdateNoAuth := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewBufferString(`{"language": "en"}`))
	wUpdateNoAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUpdateNoAuth, reqUpdateNoAuth)
	if wUpdateNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on POST /api/v1/settings without auth, got: %d", wUpdateNoAuth.Code)
	}

	reqUpdateAuth := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewBufferString(`{"language": "en"}`))
	reqUpdateAuth.AddCookie(sessionCookie)
	wUpdateAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUpdateAuth, reqUpdateAuth)
	if wUpdateAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST /api/v1/settings with auth, got: %d", wUpdateAuth.Code)
	}

	// Verify setting updated
	reqCheckUpdated := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	reqCheckUpdated.AddCookie(sessionCookie)
	wCheckUpdated := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wCheckUpdated, reqCheckUpdated)
	var updatedSettings map[string]any
	_ = json.Unmarshal(wCheckUpdated.Body.Bytes(), &updatedSettings)
	if updatedSettings["language"] != "en" {
		t.Fatalf("expected updated language 'en', got: %v", updatedSettings["language"])
	}

	// 10. Post-setup storage endpoint protection: unauthenticated call must return 401
	reqPostSetupStorage := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewBufferString(`{"device": "/dev/sda"}`))
	wPostSetupStorage := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wPostSetupStorage, reqPostSetupStorage)
	if wPostSetupStorage.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /api/v1/storage/initialize post-setup without auth, got: %d", wPostSetupStorage.Code)
	}

	// 11. Login flow: invalid password vs valid password
	reqBadLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username": "admin", "password": "WrongPassword!"}`))
	wBadLogin := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBadLogin, reqBadLogin)
	if wBadLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad login, got: %d", wBadLogin.Code)
	}

	reqGoodLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username": "admin", "password": "HouseholdMasterKey2026!"}`))
	wGoodLogin := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wGoodLogin, reqGoodLogin)
	if wGoodLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid login, got: %d", wGoodLogin.Code)
	}
	loginCookieHeader := wGoodLogin.Header().Get("Set-Cookie")
	if !strings.Contains(loginCookieHeader, "sovereign_session=") {
		t.Fatalf("expected Set-Cookie on login, got: %s", loginCookieHeader)
	}

	// 12. Logout: revokes session and clears cookie
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	wLogout := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wLogout, reqLogout)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got: %d", wLogout.Code)
	}

	// Verify session is revoked
	reqRevokedCheck := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	reqRevokedCheck.AddCookie(sessionCookie)
	wRevokedCheck := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRevokedCheck, reqRevokedCheck)
	if wRevokedCheck.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout session revocation, got: %d", wRevokedCheck.Code)
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
			MemoryUsedBytes:    4294967296, // 4 GB
			MemoryTotalBytes:   17179869184, // 16 GB
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

	splitLines := strings.Split(fullText, "\n")
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

func TestServer_Services_ListInitial(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Services []supervisor.ServiceStatus `json:"services"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}

	if len(res.Services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(res.Services))
	}

	expectedIDs := []string{"jellyfin", "navidrome", "audiobookshelf", "paperless"}
	for i, id := range expectedIDs {
		svc := res.Services[i]
		if svc.ID != id {
			t.Errorf("expected service[%d] to be %s, got %s", i, id, svc.ID)
		}
		if svc.Status != "inactive" {
			t.Errorf("expected %s status to be inactive initially, got %s", id, svc.Status)
		}
		if svc.ImageDownloaded {
			t.Errorf("expected %s image_downloaded to be false initially", id)
		}
		if svc.MemLimitBytes <= 0 || svc.MemReservationBytes <= 0 {
			t.Errorf("expected %s to have positive memory bounds", id)
		}
	}
}

func TestServer_Services_Activation_Success_And_SSEProgress(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect SSE listener
	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("failed to connect to SSE stream: %v", err)
	}
	defer sseResp.Body.Close()

	receivedEvents := make(chan string, 32)
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				eventName := strings.TrimPrefix(line, "event: ")
				receivedEvents <- eventName
			}
		}
	}()

	// Wait briefly for SSE handshake to complete
	time.Sleep(50 * time.Millisecond)

	// Activate jellyfin
	actReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/services/jellyfin/activate", nil)
	if err != nil {
		t.Fatalf("failed to create activate request: %v", err)
	}
	actResp, err := http.DefaultClient.Do(actReq)
	if err != nil {
		t.Fatalf("failed to execute activate request: %v", err)
	}
	defer actResp.Body.Close()

	if actResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from activate, got %d", actResp.StatusCode)
	}

	var svc supervisor.ServiceStatus
	if err := json.NewDecoder(actResp.Body).Decode(&svc); err != nil {
		t.Fatalf("failed to decode activate response: %v", err)
	}

	if svc.ID != "jellyfin" || svc.Status != "running" || !svc.ImageDownloaded {
		t.Errorf("unexpected activated service state: %+v", svc)
	}

	// Verify SSE events were captured
	hasTelemetry := false
	hasPullProgress := false
	hasHealthChanged := false

	timeout := time.After(2 * time.Second)
	for !hasPullProgress || !hasHealthChanged {
		select {
		case evt := <-receivedEvents:
			if evt == "telemetry.update" {
				hasTelemetry = true
			}
			if evt == "service.pull_progress" {
				hasPullProgress = true
			}
			if evt == "service.health_changed" {
				hasHealthChanged = true
			}
		case <-timeout:
			break
		}
	}

	if !hasTelemetry {
		t.Errorf("expected telemetry.update event over SSE")
	}
	if !hasPullProgress {
		t.Errorf("expected service.pull_progress event over SSE")
	}
	if !hasHealthChanged {
		t.Errorf("expected service.health_changed event over SSE")
	}
}

func TestServer_Services_Activation_PullFailure(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	mockEngine := srv.ServiceManager().Engine().(*supervisor.MockRuntimeEngine)
	mockEngine.SetImagePresent(supervisor.ImageNavidrome, false)
	mockEngine.SetPullFailure(supervisor.ImageNavidrome, errors.New("network unreachable"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/activate", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway on pull failure, got: %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("expected application/problem+json, got: %s", ct)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}
	if problem.Status != 502 || problem.Title != "Image Pull Failed" {
		t.Errorf("unexpected problem details: %+v", problem)
	}

	// Verify service state remains inactive
	checkReq := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	wCheck := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wCheck, checkReq)

	var listRes struct {
		Services []supervisor.ServiceStatus `json:"services"`
	}
	_ = json.Unmarshal(wCheck.Body.Bytes(), &listRes)
	for _, s := range listRes.Services {
		if s.ID == "navidrome" {
			if s.Status != "inactive" {
				t.Errorf("expected navidrome to remain inactive, got: %s", s.Status)
			}
			if s.ImageDownloaded {
				t.Errorf("expected image_downloaded false on pull failure")
			}
		}
	}
}

func TestServer_Services_Deactivation_PreservesVolumes(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Activate Jellyfin
	actReq := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/activate", nil)
	wAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAct, actReq)
	if wAct.Code != http.StatusOK {
		t.Fatalf("failed to activate jellyfin: %d", wAct.Code)
	}

	// 2. Put simulated user media and configuration in volumes
	testMedia := filepath.Join(dataDir, "media", "movies", "TestMovie.mkv")
	testConfig := filepath.Join(dataDir, "state", "services", "jellyfin", "config", "prefs.json")

	if err := os.WriteFile(testMedia, []byte("movie-bytes-1234"), 0644); err != nil {
		t.Fatalf("failed to write media file: %v", err)
	}
	if err := os.WriteFile(testConfig, []byte(`{"configured": true}`), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// 3. Deactivate Jellyfin
	deactReq := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/deactivate", nil)
	wDeact := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDeact, deactReq)
	if wDeact.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from deactivate, got: %d", wDeact.Code)
	}

	var deactRes supervisor.ServiceStatus
	_ = json.Unmarshal(wDeact.Body.Bytes(), &deactRes)
	if deactRes.Status != "inactive" {
		t.Errorf("expected status inactive, got: %s", deactRes.Status)
	}

	// 4. Assert files are strictly preserved
	if _, err := os.Stat(testMedia); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: media file %s was deleted during deactivation!", testMedia)
	}
	if _, err := os.Stat(testConfig); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: config file %s was deleted during deactivation!", testConfig)
	}
}

func TestServer_Services_Restart_AuthGating(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Activate Jellyfin first so it is running
	actReq := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/activate", nil)
	wAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAct, actReq)
	if wAct.Code != http.StatusOK {
		t.Fatalf("failed to activate jellyfin: %d", wAct.Code)
	}

	// Before setup is completed: restart is permitted
	reqBeforeSetup := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/restart", nil)
	wBeforeSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBeforeSetup, reqBeforeSetup)
	if wBeforeSetup.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on restart before setup, got: %d", wBeforeSetup.Code)
	}

	// Complete first-run setup wizard to provision admin credentials
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

	cookies := wSetup.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "sovereign_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("session cookie missing from setup response")
	}

	// Post-setup: Unauthenticated restart must be rejected with 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/restart", nil)
	wUnauth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated restart post-setup, got: %d", wUnauth.Code)
	}

	// Post-setup: Authenticated restart with session cookie must succeed with 200 OK
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/restart", nil)
	reqAuth.AddCookie(sessionCookie)
	wAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authenticated restart post-setup, got: %d: %s", wAuth.Code, wAuth.Body.String())
	}

	var res supervisor.ServiceStatus
	_ = json.Unmarshal(wAuth.Body.Bytes(), &res)
	if res.ID != "jellyfin" || res.Status != "running" {
		t.Errorf("unexpected restart response: %+v", res)
	}
}

func TestServer_Services_Restart_InactiveRejected(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Navidrome is inactive initially
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/restart", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request restarting inactive service, got: %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("expected application/problem+json, got: %s", ct)
	}
}

func TestServer_Services_GetService(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Get existing service
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/jellyfin", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", w.Code)
	}

	var svc supervisor.ServiceStatus
	if err := json.Unmarshal(w.Body.Bytes(), &svc); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if svc.ID != "jellyfin" || svc.Status != "inactive" {
		t.Errorf("unexpected service data: %+v", svc)
	}

	// 2. Get nonexistent service
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/services/nonexistent", nil)
	w404 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w404, req404)

	if w404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got: %d", w404.Code)
	}
	ct := w404.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("expected application/problem+json, got: %s", ct)
	}
}

func TestServer_Services_UnknownService(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	endpoints := []string{
		"/api/v1/services/nonexistent/activate",
		"/api/v1/services/nonexistent/deactivate",
		"/api/v1/services/nonexistent/restart",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodPost, ep, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for %s, got: %d", ep, w.Code)
		}

		ct := w.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/problem+json") {
			t.Errorf("expected application/problem+json for %s, got: %s", ep, ct)
		}
	}
}

func TestServer_Jellyfin_ActivationAndRefresh(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngine := supervisor.NewMockRuntimeEngine()
	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})

	srv, err := NewServer(Config{
		DataDir:    dataDir,
		IngressMgr: mockIngress,
		ServiceMgr: svcMgr,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// 1. Refresh on inactive service returns 503 Service Unavailable
	reqRefreshInactive := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/refresh", nil)
	wRefreshInactive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefreshInactive, reqRefreshInactive)

	if wRefreshInactive.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable for refresh when inactive, got: %d: %s", wRefreshInactive.Code, wRefreshInactive.Body.String())
	}
	var probInactive ProblemDetails
	_ = json.Unmarshal(wRefreshInactive.Body.Bytes(), &probInactive)
	if probInactive.Title != "Service Inactive" {
		t.Errorf("expected title 'Service Inactive', got: %s", probInactive.Title)
	}

	// 2. Refresh on nonexistent service returns 404 Not Found
	reqRefresh404 := httptest.NewRequest(http.MethodPost, "/api/v1/services/nonexistent/refresh", nil)
	wRefresh404 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefresh404, reqRefresh404)
	if wRefresh404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for nonexistent refresh, got: %d", wRefresh404.Code)
	}

	// 3. Initial ingress check: Jellyfin is inactive, so only default routes present
	reqIngressInit := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngressInit := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngressInit, reqIngressInit)
	var ingressInit IngressStatusResponse
	_ = json.Unmarshal(wIngressInit.Body.Bytes(), &ingressInit)
	if len(ingressInit.Routes) != 3 {
		t.Fatalf("expected 3 default routes initially, got: %d", len(ingressInit.Routes))
	}

	// 4. Activate Jellyfin
	actReq := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/activate", nil)
	wAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAct, actReq)

	if wAct.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating Jellyfin, got: %d: %s", wAct.Code, wAct.Body.String())
	}

	var actRes supervisor.ServiceStatus
	_ = json.Unmarshal(wAct.Body.Bytes(), &actRes)
	if actRes.ID != "jellyfin" || actRes.Status != "running" || !actRes.ImageDownloaded {
		t.Errorf("unexpected activation status: %+v", actRes)
	}

	// 5. Verify library paths and direct-play configuration exist
	moviesDir := filepath.Join(dataDir, "media", "movies")
	tvDir := filepath.Join(dataDir, "media", "tv")
	configDir := filepath.Join(dataDir, "state", "services", "jellyfin", "config")
	cacheDir := filepath.Join(dataDir, "state", "services", "jellyfin", "cache")

	for _, d := range []string{moviesDir, tvDir, configDir, cacheDir} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("expected directory %s to be created", d)
		}
	}

	encData, err := os.ReadFile(filepath.Join(configDir, "encoding.xml"))
	if err != nil {
		t.Fatalf("encoding.xml not generated: %v", err)
	}
	if !strings.Contains(string(encData), "<EnableDirectPlay>true</EnableDirectPlay>") {
		t.Errorf("encoding.xml missing EnableDirectPlay: %s", string(encData))
	}
	if !strings.Contains(string(encData), "<EnableAudioVbr>true</EnableAudioVbr>") {
		t.Errorf("encoding.xml missing EnableAudioVbr: %s", string(encData))
	}

	sysData, err := os.ReadFile(filepath.Join(configDir, "system.xml"))
	if err != nil {
		t.Fatalf("system.xml not generated: %v", err)
	}
	if !strings.Contains(string(sysData), "<Port>8096</Port>") {
		t.Errorf("system.xml missing Port 8096: %s", string(sysData))
	}

	// 6. Verify Caddy ingress route was dynamically synchronized
	if mockIngress.SyncCalls() == 0 {
		t.Errorf("expected Caddy route sync to be called on Jellyfin activation")
	}

	reqIngressActive := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngressActive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngressActive, reqIngressActive)
	var ingressActive IngressStatusResponse
	_ = json.Unmarshal(wIngressActive.Body.Bytes(), &ingressActive)

	hasJellyfinRoute := false
	for _, r := range ingressActive.Routes {
		if r.Service == "jellyfin" && r.Subpath == "/media" && r.Upstream == "sovereign-jellyfin:8096" {
			hasJellyfinRoute = true
			break
		}
	}
	if !hasJellyfinRoute {
		t.Errorf("expected jellyfin ingress route in /api/v1/system/ingress, found: %+v", ingressActive.Routes)
	}

	// 7. Refresh library when Jellyfin is active
	refreshInvoked := false
	srv.ServiceManager().JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		refreshInvoked = true
		return nil
	}

	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/refresh", nil)
	wRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefresh, reqRefresh)

	if wRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for refresh on active service, got: %d: %s", wRefresh.Code, wRefresh.Body.String())
	}
	if !refreshInvoked {
		t.Errorf("expected refresh webhook to be dispatched to Jellyfin")
	}

	var refreshRes struct {
		Status    string `json:"status"`
		Service   string `json:"service"`
		Refreshed bool   `json:"refreshed"`
	}
	_ = json.Unmarshal(wRefresh.Body.Bytes(), &refreshRes)
	if refreshRes.Status != "ok" || refreshRes.Service != "jellyfin" || !refreshRes.Refreshed {
		t.Errorf("unexpected refresh response payload: %+v", refreshRes)
	}

	// Test alias 'video'
	aliasInvoked := false
	srv.ServiceManager().JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		aliasInvoked = true
		return nil
	}
	reqAlias := httptest.NewRequest(http.MethodPost, "/api/v1/services/video/refresh", nil)
	wAlias := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAlias, reqAlias)
	if wAlias.Code != http.StatusOK || !aliasInvoked {
		t.Errorf("expected alias /video/refresh to succeed and invoke refresh, got status: %d", wAlias.Code)
	}

	// 7b. Refresh unsupported active service returns 400 Bad Request
	srv.ServiceManager().RegisterServiceDef(supervisor.ServiceDef{
		ID:   "custom",
		Name: "Custom Unsupported",
	})
	srv.ServiceManager().SetServiceStatusForTest("custom", supervisor.StatusRunning)

	reqCustomRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/custom/refresh", nil)
	wCustomRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wCustomRefresh, reqCustomRefresh)
	if wCustomRefresh.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when refreshing unsupported service custom, got: %d", wCustomRefresh.Code)
	}
	var customProblem ProblemDetails
	if err := json.Unmarshal(wCustomRefresh.Body.Bytes(), &customProblem); err != nil {
		t.Errorf("failed to parse problem details for custom refresh: %v", err)
	} else if customProblem.Status != http.StatusBadRequest || !strings.Contains(customProblem.Detail, "does not support library refresh") {
		t.Errorf("unexpected problem details for unsupported service refresh: %+v", customProblem)
	}

	// 8. Place media file in movies directory and deactivate Jellyfin
	testVideo := filepath.Join(moviesDir, "Gladiator.mkv")
	_ = os.WriteFile(testVideo, []byte("gladiator-video-bytes"), 0644)

	deactReq := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/deactivate", nil)
	wDeact := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDeact, deactReq)
	if wDeact.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deactivating, got: %d", wDeact.Code)
	}

	// Verify video file is strictly preserved!
	if _, err := os.Stat(testVideo); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: media file %s was deleted during deactivation!", testVideo)
	}

	// Subsequent refresh returns 503 Service Unavailable
	reqRefreshPostDeact := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/refresh", nil)
	wRefreshPostDeact := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefreshPostDeact, reqRefreshPostDeact)
	if wRefreshPostDeact.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 after deactivation, got: %d", wRefreshPostDeact.Code)
	}
}

func TestServer_AudioServices_ActivationAndRefresh(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngine := supervisor.NewMockRuntimeEngine()
	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})

	srv, err := NewServer(Config{
		DataDir:    dataDir,
		IngressMgr: mockIngress,
		ServiceMgr: svcMgr,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// 1. Initial Inactive State checks
	reqNavInactive := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/refresh", nil)
	wNavInactive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wNavInactive, reqNavInactive)
	if wNavInactive.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for inactive navidrome refresh, got: %d", wNavInactive.Code)
	}

	reqAbsInactive := httptest.NewRequest(http.MethodPost, "/api/v1/services/audiobookshelf/refresh", nil)
	wAbsInactive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAbsInactive, reqAbsInactive)
	if wAbsInactive.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for inactive audiobookshelf refresh, got: %d", wAbsInactive.Code)
	}

	// 2. Pre-create user media to verify preservation
	musicDir := filepath.Join(dataDir, "media", "music")
	_ = os.MkdirAll(musicDir, 0755)
	existingMusic := filepath.Join(musicDir, "song.flac")
	_ = os.WriteFile(existingMusic, []byte("flac-music-data"), 0644)

	audiobooksDir := filepath.Join(dataDir, "media", "audiobooks")
	_ = os.MkdirAll(audiobooksDir, 0755)
	existingBook := filepath.Join(audiobooksDir, "novel.m4b")
	_ = os.WriteFile(existingBook, []byte("m4b-book-data"), 0644)

	absConfigDir := filepath.Join(dataDir, "state", "services", "audiobookshelf", "config")
	_ = os.MkdirAll(absConfigDir, 0755)
	existingUserDB := filepath.Join(absConfigDir, "users.sqlite")
	_ = os.WriteFile(existingUserDB, []byte("user-sqlite-db"), 0644)

	// 3. Activate Navidrome
	reqNavAct := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/activate", nil)
	wNavAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wNavAct, reqNavAct)
	if wNavAct.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating Navidrome, got: %d: %s", wNavAct.Code, wNavAct.Body.String())
	}
	var navStatus supervisor.ServiceStatus
	_ = json.Unmarshal(wNavAct.Body.Bytes(), &navStatus)
	if navStatus.ID != "navidrome" || navStatus.Status != "running" || navStatus.Port != 4533 || navStatus.Subpath != "/music" {
		t.Errorf("unexpected Navidrome status: %+v", navStatus)
	}

	// Verify Navidrome directories and file preservation
	navDataDir := filepath.Join(dataDir, "state", "services", "navidrome", "data")
	if fi, err := os.Stat(navDataDir); err != nil || !fi.IsDir() {
		t.Errorf("expected navidrome state dir to exist: %v", err)
	}
	if musicContent, err := os.ReadFile(existingMusic); err != nil || string(musicContent) != "flac-music-data" {
		t.Errorf("existing music file modified or missing: %v", err)
	}

	// 4. Activate Audiobookshelf
	reqAbsAct := httptest.NewRequest(http.MethodPost, "/api/v1/services/audiobookshelf/activate", nil)
	wAbsAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAbsAct, reqAbsAct)
	if wAbsAct.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating Audiobookshelf, got: %d: %s", wAbsAct.Code, wAbsAct.Body.String())
	}
	var absStatus supervisor.ServiceStatus
	_ = json.Unmarshal(wAbsAct.Body.Bytes(), &absStatus)
	if absStatus.ID != "audiobookshelf" || absStatus.Status != "running" || absStatus.Port != 13378 || absStatus.Subpath != "/audiobooks" {
		t.Errorf("unexpected Audiobookshelf status: %+v", absStatus)
	}

	// Verify Audiobookshelf directories and persistence preservation
	absMetaDir := filepath.Join(dataDir, "state", "services", "audiobookshelf", "metadata")
	if fi, err := os.Stat(absMetaDir); err != nil || !fi.IsDir() {
		t.Errorf("expected audiobookshelf metadata dir to exist: %v", err)
	}
	if bookContent, err := os.ReadFile(existingBook); err != nil || string(bookContent) != "m4b-book-data" {
		t.Errorf("existing audiobook file modified or missing: %v", err)
	}
	if dbContent, err := os.ReadFile(existingUserDB); err != nil || string(dbContent) != "user-sqlite-db" {
		t.Errorf("existing user db modified or missing: %v", err)
	}

	// 5. Ingress Status includes both spoke routes
	reqIngress := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngress := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngress, reqIngress)
	var ingressRes IngressStatusResponse
	_ = json.Unmarshal(wIngress.Body.Bytes(), &ingressRes)

	hasNavRoute := false
	hasAbsRoute := false
	for _, r := range ingressRes.Routes {
		if r.Service == "navidrome" && r.Subpath == "/music" && r.Upstream == "sovereign-navidrome:4533" {
			hasNavRoute = true
		}
		if r.Service == "audiobookshelf" && r.Subpath == "/audiobooks" && r.Upstream == "sovereign-audiobookshelf:13378" {
			hasAbsRoute = true
		}
	}
	if !hasNavRoute {
		t.Errorf("expected navidrome route in ingress status: %+v", ingressRes.Routes)
	}
	if !hasAbsRoute {
		t.Errorf("expected audiobookshelf route in ingress status: %+v", ingressRes.Routes)
	}

	// 6. Test Refresh endpoints when active
	navRefreshed := false
	srv.ServiceManager().AudioProvisioner().NavidromeRefreshFunc = func(ctx context.Context) error {
		navRefreshed = true
		return nil
	}
	reqNavRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/refresh", nil)
	wNavRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wNavRefresh, reqNavRefresh)
	if wNavRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 OK refreshing navidrome, got: %d", wNavRefresh.Code)
	}
	if !navRefreshed {
		t.Errorf("expected navidrome refresh webhook to be dispatched")
	}

	// Test alias 'music'
	navRefreshed = false
	reqMusicRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/music/refresh", nil)
	wMusicRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wMusicRefresh, reqMusicRefresh)
	if wMusicRefresh.Code != http.StatusOK || !navRefreshed {
		t.Errorf("expected alias /music/refresh to trigger navidrome refresh")
	}

	absRefreshed := false
	srv.ServiceManager().AudioProvisioner().AudiobookshelfRefreshFunc = func(ctx context.Context) error {
		absRefreshed = true
		return nil
	}
	reqAbsRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/audiobookshelf/refresh", nil)
	wAbsRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAbsRefresh, reqAbsRefresh)
	if wAbsRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 OK refreshing audiobookshelf, got: %d", wAbsRefresh.Code)
	}
	if !absRefreshed {
		t.Errorf("expected audiobookshelf refresh webhook to be dispatched")
	}

	// Test alias 'audiobooks'
	absRefreshed = false
	reqBooksRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/audiobooks/refresh", nil)
	wBooksRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBooksRefresh, reqBooksRefresh)
	if wBooksRefresh.Code != http.StatusOK || !absRefreshed {
		t.Errorf("expected alias /audiobooks/refresh to trigger audiobookshelf refresh")
	}

	// 7. Deactivate and verify volume preservation
	reqDeactNav := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/deactivate", nil)
	wDeactNav := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDeactNav, reqDeactNav)
	if wDeactNav.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deactivating navidrome, got: %d", wDeactNav.Code)
	}
	if _, err := os.Stat(existingMusic); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: music file was deleted during navidrome deactivation!")
	}

	reqDeactAbs := httptest.NewRequest(http.MethodPost, "/api/v1/services/audiobookshelf/deactivate", nil)
	wDeactAbs := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDeactAbs, reqDeactAbs)
	if wDeactAbs.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deactivating audiobookshelf, got: %d", wDeactAbs.Code)
	}
	if _, err := os.Stat(existingBook); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: audiobook file was deleted during audiobookshelf deactivation!")
	}
	if _, err := os.Stat(existingUserDB); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: user database was deleted during audiobookshelf deactivation!")
	}

	// Subsequent refresh post-deactivation returns 503
	reqRefreshPostNav := httptest.NewRequest(http.MethodPost, "/api/v1/services/navidrome/refresh", nil)
	wRefreshPostNav := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefreshPostNav, reqRefreshPostNav)
	if wRefreshPostNav.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for navidrome refresh after deactivation, got: %d", wRefreshPostNav.Code)
	}
}

func TestServer_AudioBundle_ActivationAndRefresh(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngine := supervisor.NewMockRuntimeEngine()
	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})

	srv, err := NewServer(Config{
		DataDir:    dataDir,
		IngressMgr: mockIngress,
		ServiceMgr: svcMgr,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// 1. Activate audio bundle via POST /api/v1/services/audio/activate
	reqBundle := httptest.NewRequest(http.MethodPost, "/api/v1/services/audio/activate", nil)
	wBundle := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBundle, reqBundle)

	if wBundle.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating audio bundle, got: %d: %s", wBundle.Code, wBundle.Body.String())
	}

	// Verify both Navidrome and Audiobookshelf are running
	navSvc, err := svcMgr.GetService(context.Background(), supervisor.ServiceNavidrome)
	if err != nil || navSvc.Status != supervisor.StatusRunning {
		t.Errorf("expected navidrome to be running, got: %+v, err: %v", navSvc, err)
	}
	absSvc, err := svcMgr.GetService(context.Background(), supervisor.ServiceAudiobookshelf)
	if err != nil || absSvc.Status != supervisor.StatusRunning {
		t.Errorf("expected audiobookshelf to be running, got: %+v, err: %v", absSvc, err)
	}

	// 2. Refresh bundle via POST /api/v1/services/audio/refresh
	navRefreshed := false
	absRefreshed := false
	svcMgr.AudioProvisioner().NavidromeRefreshFunc = func(ctx context.Context) error {
		navRefreshed = true
		return nil
	}
	svcMgr.AudioProvisioner().AudiobookshelfRefreshFunc = func(ctx context.Context) error {
		absRefreshed = true
		return nil
	}

	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/services/audio/refresh", nil)
	wRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRefresh, reqRefresh)

	if wRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 OK refreshing audio bundle, got: %d: %s", wRefresh.Code, wRefresh.Body.String())
	}
	if !navRefreshed || !absRefreshed {
		t.Errorf("expected both navidrome (%v) and audiobookshelf (%v) to be refreshed", navRefreshed, absRefreshed)
	}

	// 3. Test Pull Failure in audio bundle returns 502 Bad Gateway
	tempDir2 := t.TempDir()
	dataDir2 := filepath.Join(tempDir2, "data")
	stateDir2 := filepath.Join(dataDir2, "state")
	_ = os.MkdirAll(stateDir2, 0755)
	_ = os.WriteFile(filepath.Join(stateDir2, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngineFail := supervisor.NewMockRuntimeEngine()
	mockEngineFail.SetPullFailure(supervisor.ImageNavidrome, errors.New("pull failure for navidrome"))

	srvFail, err := NewServer(Config{
		DataDir:    dataDir2,
		ServiceMgr: supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
			DataDir: dataDir2,
			Engine:  mockEngineFail,
		}),
	})
	if err != nil {
		t.Fatalf("failed to create fail server: %v", err)
	}
	defer srvFail.Close()

	reqFail := httptest.NewRequest(http.MethodPost, "/api/v1/services/audio/activate", nil)
	wFail := httptest.NewRecorder()
	srvFail.Handler().ServeHTTP(wFail, reqFail)

	if wFail.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway on pull failure in audio bundle, got: %d: %s", wFail.Code, wFail.Body.String())
	}
	var prob ProblemDetails
	_ = json.Unmarshal(wFail.Body.Bytes(), &prob)
	if prob.Title != "Image Pull Failed" {
		t.Errorf("expected title 'Image Pull Failed', got: %s", prob.Title)
	}
}

func TestServer_Paperless_Activation_LifecycleAndIngressRouting(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	mockEngine := supervisor.NewMockRuntimeEngine()
	mockIngress := &mockIngressManager{
		status: map[string]any{"connected": true, "status": "ok"},
	}

	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
		PaperlessProvisioner: supervisor.NewPaperlessProvisioner(dataDir, nil, supervisor.PaperlessProvisionerOptions{
			NumCPU: func() int { return 4 },
		}),
	})

	srv, err := NewServer(Config{
		DataDir:    dataDir,
		IngressMgr: mockIngress,
		ServiceMgr: svcMgr,
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// 1. Verify alias resolution in inactive state
	aliases := []string{"paperless", "dms", "documents"}
	for _, alias := range aliases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/services/"+alias, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for GET /services/%s, got: %d", alias, w.Code)
		}
		var st supervisor.ServiceStatus
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatalf("failed to parse service status for %s: %v", alias, err)
		}
		if st.ID != supervisor.ServicePaperless {
			t.Errorf("expected ID %s for alias %s, got %s", supervisor.ServicePaperless, alias, st.ID)
		}
		if st.Status != supervisor.StatusInactive {
			t.Errorf("expected status inactive for %s, got %s", alias, st.Status)
		}
		if st.Port != 8000 {
			t.Errorf("expected port 8000, got %d", st.Port)
		}
		if st.Subpath != "/docs" {
			t.Errorf("expected subpath /docs, got %s", st.Subpath)
		}
	}

	// 2. Initial Ingress status has 3 default routes (no paperless route)
	reqIngressInit := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngressInit := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngressInit, reqIngressInit)
	var ingressInit IngressStatusResponse
	_ = json.Unmarshal(wIngressInit.Body.Bytes(), &ingressInit)
	for _, r := range ingressInit.Routes {
		if r.Service == "paperless" {
			t.Fatalf("unexpected paperless route in initial ingress status: %+v", r)
		}
	}

	// 3. Activate Paperless via alias "dms"
	reqAct := httptest.NewRequest(http.MethodPost, "/api/v1/services/dms/activate", nil)
	wAct := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAct, reqAct)

	if wAct.Code != http.StatusOK {
		t.Fatalf("expected 200 OK activating dms, got: %d: %s", wAct.Code, wAct.Body.String())
	}

	var actRes supervisor.ServiceStatus
	_ = json.Unmarshal(wAct.Body.Bytes(), &actRes)
	if actRes.ID != supervisor.ServicePaperless || actRes.Status != supervisor.StatusRunning {
		t.Errorf("unexpected activation response: %+v", actRes)
	}
	if actRes.MemLimitBytes != supervisor.MemLimitPaperless {
		t.Errorf("expected memory limit %d, got %d", supervisor.MemLimitPaperless, actRes.MemLimitBytes)
	}
	if actRes.MemReservationBytes != supervisor.MemReservationPaperless {
		t.Errorf("expected memory reservation %d, got %d", supervisor.MemReservationPaperless, actRes.MemReservationBytes)
	}

	// 4. Verify directory provisioning and permissions
	expectedDirs := []string{
		filepath.Join(dataDir, "documents"),
		filepath.Join(dataDir, "documents", "media"),
		filepath.Join(dataDir, "documents", "data"),
		filepath.Join(dataDir, "documents", "consume"),
		filepath.Join(dataDir, "documents", "export"),
		filepath.Join(dataDir, "state", "services", "paperless"),
	}
	for _, d := range expectedDirs {
		fi, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
		} else if !fi.IsDir() {
			t.Errorf("expected %s to be directory", d)
		}
	}

	// 5. Verify SQLite WAL mode at /data/documents/data/db.sqlite3
	dbPath := filepath.Join(dataDir, "documents", "data", "db.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Errorf("expected SQLite journal mode 'wal', got %q", journalMode)
	}

	// Insert test data to verify preservation across restart and deactivation
	_, err = db.Exec("CREATE TABLE docs (id INT PRIMARY KEY, title TEXT);")
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	_, err = db.Exec("INSERT INTO docs VALUES (1, 'Medical Insurance');")
	if err != nil {
		t.Fatalf("failed to insert test record: %v", err)
	}
	db.Close()

	// Write document file into media
	mediaDoc := filepath.Join(dataDir, "documents", "media", "policy-01.pdf")
	if err := os.WriteFile(mediaDoc, []byte("%PDF-1.4 sample policy"), 0644); err != nil {
		t.Fatalf("failed to write test media file: %v", err)
	}

	// 6. Verify Caddy Ingress was synchronized with route /docs and dedicated port 8000
	if mockIngress.SyncCalls() == 0 {
		t.Errorf("expected Caddy route sync calls > 0 on Paperless activation")
	}

	foundSyncRoute := false
	for _, r := range mockIngress.LastRoutes() {
		if r.Service == "paperless" && r.Port == 8000 && r.Subpath == "/docs" {
			foundSyncRoute = true
			break
		}
	}
	if !foundSyncRoute {
		t.Errorf("expected SyncRoutes to receive route for paperless port 8000, got: %+v", mockIngress.LastRoutes())
	}

	reqIngressActive := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngressActive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngressActive, reqIngressActive)
	var ingressActive IngressStatusResponse
	_ = json.Unmarshal(wIngressActive.Body.Bytes(), &ingressActive)

	hasPaperlessRoute := false
	for _, r := range ingressActive.Routes {
		if r.Service == "paperless" && r.Subpath == "/docs" && r.Upstream == "sovereign-paperless:8000" {
			hasPaperlessRoute = true
			break
		}
	}
	if !hasPaperlessRoute {
		t.Errorf("expected paperless ingress route (/docs -> sovereign-paperless:8000), found: %+v", ingressActive.Routes)
	}

	// 7. Restart Paperless via alias "paperless"
	reqRestart := httptest.NewRequest(http.MethodPost, "/api/v1/services/paperless/restart", nil)
	wRestart := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRestart, reqRestart)
	if wRestart.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on restart, got: %d: %s", wRestart.Code, wRestart.Body.String())
	}

	// Verify volume data is preserved across restart
	if _, err := os.Stat(mediaDoc); err != nil {
		t.Fatalf("document file was lost on restart: %v", err)
	}

	// 8. Deactivate Paperless via alias "documents"
	reqDeact := httptest.NewRequest(http.MethodPost, "/api/v1/services/documents/deactivate", nil)
	wDeact := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDeact, reqDeact)

	if wDeact.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on deactivation, got: %d: %s", wDeact.Code, wDeact.Body.String())
	}
	var deactRes supervisor.ServiceStatus
	_ = json.Unmarshal(wDeact.Body.Bytes(), &deactRes)
	if deactRes.Status != supervisor.StatusInactive {
		t.Errorf("expected inactive status, got %s", deactRes.Status)
	}

	// Verify document file and database strictly preserved after deactivation
	if _, err := os.Stat(mediaDoc); err != nil {
		t.Fatalf("document file in /data/documents was pruned or deleted on deactivation: %v", err)
	}
	dbPreserved, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open preserved sqlite database: %v", err)
	}
	defer dbPreserved.Close()
	var docTitle string
	if err := dbPreserved.QueryRow("SELECT title FROM docs WHERE id = 1;").Scan(&docTitle); err != nil {
		t.Fatalf("failed to read record from preserved sqlite db: %v", err)
	}
	if docTitle != "Medical Insurance" {
		t.Errorf("expected preserved record 'Medical Insurance', got %q", docTitle)
	}

	// 9. Ingress routes no longer contain paperless after deactivation
	reqIngressPostDeact := httptest.NewRequest(http.MethodGet, "/api/v1/system/ingress", nil)
	wIngressPostDeact := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wIngressPostDeact, reqIngressPostDeact)
	var ingressPostDeact IngressStatusResponse
	_ = json.Unmarshal(wIngressPostDeact.Body.Bytes(), &ingressPostDeact)
	for _, r := range ingressPostDeact.Routes {
		if r.Service == "paperless" {
			t.Errorf("expected paperless route to be removed from ingress after deactivation, found: %+v", r)
		}
	}

	// 10. Image pull failure returns 502 Bad Gateway with ProblemDetails
	mockEngine.SetPullFailure(supervisor.ImagePaperless, errors.New("simulated network timeout pulling paperless"))
	mockEngine.SetImagePresent(supervisor.ImagePaperless, false)

	reqFail := httptest.NewRequest(http.MethodPost, "/api/v1/services/paperless/activate", nil)
	wFail := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wFail, reqFail)

	if wFail.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway on pull failure, got: %d: %s", wFail.Code, wFail.Body.String())
	}
	var prob ProblemDetails
	_ = json.Unmarshal(wFail.Body.Bytes(), &prob)
	if prob.Title != "Image Pull Failed" {
		t.Errorf("expected title 'Image Pull Failed', got: %s", prob.Title)
	}
}

func createMultipartUpload(category, filename string, content []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if category != "" {
		if err := writer.WriteField("category", category); err != nil {
			return nil, "", err
		}
	}

	if filename != "" || content != nil {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, "", err
		}
		if len(content) > 0 {
			if _, err := part.Write(content); err != nil {
				return nil, "", err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

func TestServer_Dropzone_UploadValidMedia_PromotesAndRefreshes(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// MP4 payload with valid ftyp box
	mp4Payload := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41" + strings.Repeat("A", 100))
	body, contentType, err := createMultipartUpload("movies", "sample.mp4", mp4Payload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var res storage.DropHistoryItem
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Filename != "sample.mp4" {
		t.Errorf("expected sample.mp4, got %s", res.Filename)
	}
	if res.Category != "movies" {
		t.Errorf("expected movies, got %s", res.Category)
	}
	if res.Status != "promoted" {
		t.Errorf("expected promoted, got %s", res.Status)
	}

	// Verify file promoted to /data/media/movies/sample.mp4
	targetPath := filepath.Join(dataDir, "media", "movies", "sample.mp4")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("file not found in target media directory: %v", err)
	}

	// Verify staging directory is clean
	stagingDir := filepath.Join(dataDir, "drop", "incoming", "movies")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("unexpected staging file: %s", entry.Name())
	}

	// Verify history endpoint returns the uploaded file
	reqHist := httptest.NewRequest(http.MethodGet, "/api/v1/drop/history", nil)
	wHist := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wHist, reqHist)

	if wHist.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for history, got %d", wHist.Code)
	}

	var histRes struct {
		Items []storage.DropHistoryItem `json:"items"`
	}
	if err := json.Unmarshal(wHist.Body.Bytes(), &histRes); err != nil {
		t.Fatal(err)
	}
	if len(histRes.Items) != 1 || histRes.Items[0].Filename != "sample.mp4" {
		t.Errorf("unexpected history response: %+v", histRes)
	}
}

func TestServer_Dropzone_UploadNonMedia_Rejects422AndCleans(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// Text payload without magic bytes
	txtPayload := []byte("This is plain text and definitely not media.")
	body, contentType, err := createMultipartUpload("tv", "notes.txt", txtPayload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity, got %d: %s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if prob.Title != "Invalid Media Format" {
		t.Errorf("expected title 'Invalid Media Format', got %q", prob.Title)
	}
	if !strings.Contains(prob.Detail, "Only media files are accepted") {
		t.Errorf("expected detail to contain reassurance microcopy, got %q", prob.Detail)
	}

	// Verify file was NOT promoted to /data/media/tv
	targetPath := filepath.Join(dataDir, "media", "tv", "notes.txt")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("rejected file should not exist at %s", targetPath)
	}

	// Verify staging directory is clean
	stagingDir := filepath.Join(dataDir, "drop", "incoming", "tv")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("staging file not cleaned up: %s", entry.Name())
	}
}

func TestServer_Dropzone_UploadValidationErrors(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Missing category
	body1, ct1, _ := createMultipartUpload("", "song.mp3", []byte("ID3\x04\x00\x00\x00\x00\x02\x00"))
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body1)
	req1.Header.Set("Content-Type", ct1)
	w1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing category, got %d", w1.Code)
	}

	// 2. Invalid category
	body2, ct2, _ := createMultipartUpload("games", "song.mp3", []byte("ID3\x04\x00\x00\x00\x00\x02\x00"))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body2)
	req2.Header.Set("Content-Type", ct2)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid category, got %d", w2.Code)
	}

	// 3. Missing file
	body3 := &bytes.Buffer{}
	writer3 := multipart.NewWriter(body3)
	_ = writer3.WriteField("category", "music")
	_ = writer3.Close()
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body3)
	req3.Header.Set("Content-Type", writer3.FormDataContentType())
	w3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing file, got %d", w3.Code)
	}

	// 4. Empty file
	body4, ct4, _ := createMultipartUpload("music", "empty.mp3", []byte{})
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body4)
	req4.Header.Set("Content-Type", ct4)
	w4 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w4, req4)
	if w4.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 Unprocessable Entity on empty file, got %d", w4.Code)
	}
}

func TestServer_Dropzone_CollisionHandling(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	wavPayload := []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00" + strings.Repeat("\x00", 50))

	// First upload
	b1, ct1, _ := createMultipartUpload("music", "track.wav", wavPayload)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", b1)
	req1.Header.Set("Content-Type", ct1)
	w1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload failed: %d", w1.Code)
	}

	var res1 storage.DropHistoryItem
	_ = json.Unmarshal(w1.Body.Bytes(), &res1)
	if res1.Filename != "track.wav" {
		t.Errorf("expected track.wav, got %s", res1.Filename)
	}

	// Second upload with same name
	b2, ct2, _ := createMultipartUpload("music", "track.wav", wavPayload)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", b2)
	req2.Header.Set("Content-Type", ct2)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second upload failed: %d", w2.Code)
	}

	var res2 storage.DropHistoryItem
	_ = json.Unmarshal(w2.Body.Bytes(), &res2)
	if res2.Filename != "track (1).wav" {
		t.Errorf("expected track (1).wav, got %s", res2.Filename)
	}

	// Both files exist on disk
	f1 := filepath.Join(dataDir, "media", "music", "track.wav")
	f2 := filepath.Join(dataDir, "media", "music", "track (1).wav")
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("track.wav missing: %v", err)
	}
	if _, err := os.Stat(f2); err != nil {
		t.Errorf("track (1).wav missing: %v", err)
	}
}

func TestServer_Dropzone_UploadValidMedia_CallsRefreshService(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()
	// Activate Jellyfin service
	if _, err := srv.serviceMgr.ActivateService(ctx, supervisor.ServiceJellyfin); err != nil {
		t.Fatalf("failed to activate jellyfin: %v", err)
	}

	refreshCalled := make(chan string, 1)
	srv.serviceMgr.JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		select {
		case refreshCalled <- supervisor.ServiceJellyfin:
		default:
		}
		return nil
	}

	// Upload movie
	mp4Payload := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41" + strings.Repeat("A", 100))
	body, contentType, err := createMultipartUpload("movies", "feature.mp4", mp4Payload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// Verify RefreshFunc was called within 1 second
	select {
	case svc := <-refreshCalled:
		if svc != supervisor.ServiceJellyfin {
			t.Errorf("expected refresh on %s, got %s", supervisor.ServiceJellyfin, svc)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for jellyfin library refresh webhook")
	}
}

func TestServer_Dropzone_UploadCategoryMismatch_Rejects422AndCleans(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// MP3 audio payload uploaded to "movies" category
	mp3Payload := append([]byte("ID3\x04\x00\x00\x00\x00\x02\x00"), bytes.Repeat([]byte("\xFF\xFB\x90\x44"), 20)...)
	body, contentType, err := createMultipartUpload("movies", "song.mp3", mp3Payload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on category mismatch, got %d: %s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if prob.Title != "Category Mismatch" {
		t.Errorf("expected title 'Category Mismatch', got %q", prob.Title)
	}

	// Verify file was NOT placed in media/movies
	destPath := filepath.Join(dataDir, "media", "movies", "song.mp3")
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("mismatched file should not exist at %s", destPath)
	}

	// Verify staging directory is clean
	stagingDir := filepath.Join(dataDir, "drop", "incoming", "movies")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("staging file not cleaned up after category mismatch: %s", entry.Name())
	}
}

type serverAbortingReader struct {
	prefix []byte
	cancel context.CancelFunc
	pos    int
}

func (r *serverAbortingReader) Read(p []byte) (int, error) {
	if r.pos < len(r.prefix) {
		n := copy(p, r.prefix[r.pos:])
		r.pos += n
		return n, nil
	}
	if r.cancel != nil {
		r.cancel()
	}
	return 0, context.Canceled
}

func TestServer_Dropzone_UploadAborted_Returns499AndCleansStaging(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	mp4Payload := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41" + strings.Repeat("A", 10000))
	fullBody, contentType, err := createMultipartUpload("movies", "aborted.mp4", mp4Payload)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Read only the first 350 bytes (form fields and beginning of file) then abort
	rawBytes := fullBody.Bytes()
	cutoff := 350
	if cutoff > len(rawBytes) {
		cutoff = len(rawBytes) / 2
	}

	reader := &serverAbortingReader{
		prefix: rawBytes[:cutoff],
		cancel: cancel,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", reader).WithContext(ctx)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != 499 {
		t.Fatalf("expected HTTP 499 Upload Aborted, got %d: %s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if prob.Title != "Upload Aborted" {
		t.Errorf("expected title 'Upload Aborted', got %q", prob.Title)
	}

	// Verify staging directory is completely clean
	stagingDir := filepath.Join(dataDir, "drop", "incoming", "movies")
	entries, _ := os.ReadDir(stagingDir)
	for _, entry := range entries {
		t.Errorf("orphaned staging file after aborted upload: %s", entry.Name())
	}

	// Verify not promoted
	destPath := filepath.Join(dataDir, "media", "movies", "aborted.mp4")
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("aborted file should not exist at %s", destPath)
	}
}

func TestServer_Optical_Status_Default(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/optical/status", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var status optical.OpticalStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode optical status: %v", err)
	}

	if status.State != "idle" {
		t.Errorf("expected state 'idle', got %q", status.State)
	}
	if status.StatusText != "Idle · Ready for disc" {
		t.Errorf("expected status_text 'Idle · Ready for disc', got %q", status.StatusText)
	}
	if !status.DriveReady {
		t.Errorf("expected drive_ready true")
	}
	if status.DiscPresent {
		t.Errorf("expected disc_present false")
	}
	if status.KeysActive {
		t.Errorf("expected keys_active false initially")
	}
}

func TestServer_Optical_Eject(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/optical/eject", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res OpticalEjectResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode eject response: %v", err)
	}

	if res.Status != "ejected" {
		t.Errorf("expected status 'ejected', got %q", res.Status)
	}
}

func TestServer_Optical_Keys_Upload_Multipart(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	keyContent := `
; Doom9 AACS test keys
PK 0x0123456789ABCDEF0123456789ABCDEF
0x11223344556677889900AABBCCDDEEFF00000000 = Inception (2010) | V | 0xAABBCCDDEEFF00112233445566778899
`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "KEYDB.cfg")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte(keyContent)); err != nil {
		t.Fatalf("failed to write part: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res OpticalKeysUploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	if res.Status != "active" {
		t.Errorf("expected status 'active', got %q", res.Status)
	}
	if res.KeysCount != 2 {
		t.Errorf("expected keys_count 2, got %d", res.KeysCount)
	}

	// Verify status endpoint now reflects active keys
	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/optical/status", nil)
	statusW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusW, statusReq)

	var st optical.OpticalStatus
	if err := json.Unmarshal(statusW.Body.Bytes(), &st); err != nil {
		t.Fatalf("failed to decode status: %v", err)
	}
	if !st.KeysActive {
		t.Errorf("expected keys_active true after successful upload")
	}
}

func TestServer_Optical_Keys_Upload_RawText(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	keyContent := `
; Doom9 AACS test keys
PK 0x0123456789ABCDEF0123456789ABCDEF
`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", strings.NewReader(keyContent))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res OpticalKeysUploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.KeysCount != 1 {
		t.Errorf("expected keys_count 1, got %d", res.KeysCount)
	}
}

func TestServer_Optical_Keys_Upload_ValidationErrors(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Empty body
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", strings.NewReader("   \n\t  "))
	emptyReq.Header.Set("Content-Type", "text/plain")
	emptyW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(emptyW, emptyReq)

	if emptyW.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty key file, got %d", emptyW.Code)
	}

	// 2. Binary body
	binReq := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", bytes.NewReader([]byte{0x00, 0x01, 0x02}))
	binReq.Header.Set("Content-Type", "application/octet-stream")
	binW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(binW, binReq)

	if binW.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for binary key file, got %d", binW.Code)
	}

	// 3. Invalid text without key entries
	invReq := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", strings.NewReader("Random text without any keys"))
	invReq.Header.Set("Content-Type", "text/plain")
	invW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(invW, invReq)

	if invW.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid text without keys, got %d", invW.Code)
	}
}

func TestServer_Optical_SSE_TelemetryStream(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
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
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/system/events", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to open SSE connection: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read initial telemetry.update
	var initialOpticalStatus string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading SSE stream: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var update TelemetryUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil && update.OpticalStatus != "" {
				initialOpticalStatus = update.OpticalStatus
				break
			}
		}
	}

	if initialOpticalStatus != "Idle · Ready for disc" {
		t.Errorf("expected initial optical status 'Idle · Ready for disc', got %q", initialOpticalStatus)
	}

	// Insert DVD into mock optical driver and trigger ripping
	disc := optical.DiscInfo{
		DiscType:  optical.DiscTypeDVD,
		Label:     "VACATION_VIDEO",
		ID:        "vacation_video",
		Encrypted: false,
	}
	mockOpticalDriver.InsertDisc(disc)

	// Trigger a poll cycle
	srv.OpticalRipper().PollOnce(ctx)

	// Stream should emit optical events (optical.disc_detected, optical.rip_progress, optical.disc_ejected)
	var detectedEmitted bool
	var progressEmitted bool
	var ejectedEmitted bool

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event: optical.disc_detected") {
			detectedEmitted = true
		} else if strings.HasPrefix(line, "event: optical.rip_progress") {
			progressEmitted = true
		} else if strings.HasPrefix(line, "event: optical.disc_ejected") {
			ejectedEmitted = true
			break // All required events received!
		}
	}

	if !detectedEmitted {
		t.Errorf("expected optical.disc_detected event over SSE stream")
	}
	if !progressEmitted {
		t.Errorf("expected optical.rip_progress event over SSE stream")
	}
	if !ejectedEmitted {
		t.Errorf("expected optical.disc_ejected event over SSE stream")
	}
}

func TestServer_Optical_Keys_Upload_PayloadTooLarge(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Raw text body exceeding 10MB limit
	const largeSize = 10<<20 + 1024 // 10MB + 1KB
	largePayload := make([]byte, largeSize)
	for i := range largePayload {
		largePayload[i] = 'A'
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", bytes.NewReader(largePayload))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %q", contentType)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}
	if prob.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("expected problem status 413, got %d", prob.Status)
	}
	if prob.Title != "Payload Too Large" {
		t.Errorf("expected title 'Payload Too Large', got %q", prob.Title)
	}

	// 2. Explicit Content-Length > 10MB header rejection before reading body
	hdrReq := httptest.NewRequest(http.MethodPost, "/api/v1/optical/keys", strings.NewReader("dummy"))
	hdrReq.ContentLength = 15 * 1024 * 1024 // 15MB
	hdrReq.Header.Set("Content-Type", "text/plain")
	hdrW := httptest.NewRecorder()

	srv.Handler().ServeHTTP(hdrW, hdrReq)

	if hdrW.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413 for oversized Content-Length, got %d", hdrW.Code)
	}
}

func TestServer_Optical_SSE_ActiveRippingTelemetry(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
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
		DataDir:           dataDir,
		ServiceMgr:        svcMgr,
		OpticalDriver:     mockOpticalDriver,
		TelemetryInterval: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	srv.OpticalRipper().SetEjectResetDelay(0)

	// Use channels to coordinate and hold rip in active state and eject in active state
	ripStartedCh := make(chan struct{})
	continueRipCh := make(chan struct{})
	ejectStartedCh := make(chan struct{})
	continueEjectCh := make(chan struct{})

	mockOpticalDriver.SetOnRip(func(ctx context.Context, disc optical.DiscInfo, destDir string, progressCb func(percent float64)) error {
		if progressCb != nil {
			progressCb(42.0)
		}
		close(ripStartedCh)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-continueRipCh:
		}
		if progressCb != nil {
			progressCb(100.0)
		}
		_ = os.MkdirAll(destDir, 0755)
		return os.WriteFile(filepath.Join(destDir, "disc.iso"), []byte("data"), 0644)
	})

	mockOpticalDriver.SetOnEject(func(ctx context.Context) error {
		select {
		case ejectStartedCh <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-continueEjectCh:
		}
		return nil
	})

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/system/events", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to open SSE stream: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Insert DVD into mock driver and trigger PollOnce in background
	disc := optical.DiscInfo{
		DiscType:  optical.DiscTypeDVD,
		Label:     "TEST_ACTIVE_RIP",
		ID:        "test_active_rip",
		Encrypted: false,
	}
	mockOpticalDriver.InsertDisc(disc)

	go srv.OpticalRipper().PollOnce(ctx)

	// Wait until rip begins and reports 42%
	select {
	case <-ripStartedCh:
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for rip start")
	}

	// Read SSE telemetry.update events looking for "Ripping (42%)"
	var sawRippingTelemetry bool
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var update TelemetryUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil {
				if update.OpticalStatus == "Ripping (42%)" {
					sawRippingTelemetry = true
					break
				}
			}
		}
	}

	if !sawRippingTelemetry {
		t.Errorf("expected telemetry.update SSE to report %q", "Ripping (42%)")
	}

	// Allow ripping to finish, moving to auto-eject
	close(continueRipCh)

	select {
	case <-ejectStartedCh:
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for eject start")
	}

	// Read SSE telemetry.update events looking for "Auto-ejecting disc"
	var sawAutoEjectingTelemetry bool
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var update TelemetryUpdate
			if err := json.Unmarshal([]byte(data), &update); err == nil {
				if update.OpticalStatus == "Auto-ejecting disc" {
					sawAutoEjectingTelemetry = true
					break
				}
			}
		}
	}

	if !sawAutoEjectingTelemetry {
		t.Errorf("expected telemetry.update SSE to report 'Auto-ejecting disc'")
	}

	// Allow eject to complete
	close(continueEjectCh)
}

func TestServer_Enrichment_Status_Endpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/enrichment/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var status enrichment.EngineStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}

	if status.State != enrichment.StateIdle {
		t.Errorf("expected state %q, got %q", enrichment.StateIdle, status.State)
	}
	if status.StatusText != enrichment.StatusTextIdle {
		t.Errorf("expected status_text %q, got %q", enrichment.StatusTextIdle, status.StatusText)
	}
}

func TestServer_Enrichment_Scan_PromotesMovie_TriggersJellyfinRefresh(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()

	// Activate Jellyfin service
	if _, err := srv.serviceMgr.ActivateService(ctx, supervisor.ServiceJellyfin); err != nil {
		t.Fatalf("failed to activate jellyfin: %v", err)
	}

	refreshCalled := make(chan string, 1)
	srv.serviceMgr.JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		select {
		case refreshCalled <- supervisor.ServiceJellyfin:
		default:
		}
		return nil
	}

	// Mock metadata client
	mockClient := enrichment.NewMockMetadataClient()
	mockClient.MockMovie = &enrichment.MovieMetadata{
		Title:       "The Matrix",
		Year:        1999,
		Overview:    "A computer hacker learns from mysterious rebels about the true nature of his reality.",
		Genres:      []string{"Action", "Sci-Fi"},
		PosterBytes: []byte("matrix-poster-image"),
	}
	srv.EnrichmentEngine().SetClient(mockClient)

	// Stage disc rip
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs", "THE_MATRIX_1999")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "disc.iso"), []byte("the-matrix-movie-payload"), 0644); err != nil {
		t.Fatal(err)
	}

	// Trigger manual scan
	req := httptest.NewRequest(http.MethodPost, "/api/v1/enrichment/scan", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var scanResp EnrichmentScanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &scanResp); err != nil {
		t.Fatalf("failed to decode scan response: %v", err)
	}
	if scanResp.ItemsProcessed != 1 {
		t.Errorf("expected 1 item processed, got %d", scanResp.ItemsProcessed)
	}

	// Verify target movie library
	movieTarget := filepath.Join(dataDir, "media", "movies", "The Matrix (1999)")
	mediaFile := filepath.Join(movieTarget, "disc.iso")
	content, err := os.ReadFile(mediaFile)
	if err != nil {
		t.Fatalf("expected moved disc.iso: %v", err)
	}
	if string(content) != "the-matrix-movie-payload" {
		t.Errorf("unexpected content: %s", string(content))
	}

	// Verify movie.nfo and poster.jpg
	nfoPath := filepath.Join(movieTarget, "movie.nfo")
	if _, err := os.Stat(nfoPath); err != nil {
		t.Errorf("expected movie.nfo in target: %v", err)
	}
	posterPath := filepath.Join(movieTarget, "poster.jpg")
	if _, err := os.Stat(posterPath); err != nil {
		t.Errorf("expected poster.jpg in target: %v", err)
	}

	// Verify staging folder was cleaned up
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Errorf("expected staging dir to be removed: %v", err)
	}

	// Verify Jellyfin refresh triggered within 5 seconds
	select {
	case svc := <-refreshCalled:
		if svc != supervisor.ServiceJellyfin {
			t.Errorf("expected refresh on %s, got %s", supervisor.ServiceJellyfin, svc)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("timed out waiting for Jellyfin refresh webhook")
	}
}

func TestServer_Enrichment_Scan_PromotesMusic_TriggersNavidromeRefresh(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	ctx := context.Background()

	// Activate Navidrome service
	if _, err := srv.serviceMgr.ActivateService(ctx, supervisor.ServiceNavidrome); err != nil {
		t.Fatalf("failed to activate navidrome: %v", err)
	}

	refreshCalled := make(chan string, 1)
	srv.serviceMgr.AudioProvisioner().NavidromeRefreshFunc = func(ctx context.Context) error {
		select {
		case refreshCalled <- supervisor.ServiceNavidrome:
		default:
		}
		return nil
	}

	// Mock metadata client
	mockClient := enrichment.NewMockMetadataClient()
	mockClient.MockMusic = &enrichment.MusicMetadata{
		Artist:     "Pink Floyd",
		Album:      "The Dark Side of the Moon",
		Year:       1973,
		CoverBytes: []byte("pink-floyd-cover"),
	}
	srv.EnrichmentEngine().SetClient(mockClient)

	// Stage audio CD rip
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs", "AUDIO_CD_8F3A")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "track01.cdda"), []byte("track01-audio-payload"), 0644); err != nil {
		t.Fatal(err)
	}

	// Trigger manual scan
	req := httptest.NewRequest(http.MethodPost, "/api/v1/enrichment/scan", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var scanResp EnrichmentScanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &scanResp); err != nil {
		t.Fatalf("failed to decode scan response: %v", err)
	}
	if scanResp.ItemsProcessed != 1 {
		t.Errorf("expected 1 item processed, got %d", scanResp.ItemsProcessed)
	}

	// Verify target music library
	musicTarget := filepath.Join(dataDir, "media", "music", "Pink Floyd", "The Dark Side of the Moon (1973)")
	trackFile := filepath.Join(musicTarget, "track01.cdda")
	content, err := os.ReadFile(trackFile)
	if err != nil {
		t.Fatalf("expected moved track01.cdda: %v", err)
	}
	if string(content) != "track01-audio-payload" {
		t.Errorf("unexpected content: %s", string(content))
	}

	// Verify cover.jpg
	coverPath := filepath.Join(musicTarget, "cover.jpg")
	if _, err := os.Stat(coverPath); err != nil {
		t.Errorf("expected cover.jpg in target: %v", err)
	}

	// Verify staging folder was cleaned up
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Errorf("expected staging dir to be removed: %v", err)
	}

	// Verify Navidrome refresh triggered within 5 seconds
	select {
	case svc := <-refreshCalled:
		if svc != supervisor.ServiceNavidrome {
			t.Errorf("expected refresh on %s, got %s", supervisor.ServiceNavidrome, svc)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("timed out waiting for Navidrome refresh webhook")
	}
}

func TestServer_Enrichment_TelemetryUpdate_MatchingMetadata(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// Stage a movie disc
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs", "THE_MATRIX_1999")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "disc.iso"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchStarted := make(chan struct{})
	continueFetch := make(chan struct{})

	mockClient := enrichment.NewMockMetadataClient()
	mockClient.MovieFunc = func(ctx context.Context, query string, year int) (*enrichment.MovieMetadata, error) {
		close(fetchStarted)
		<-continueFetch
		return &enrichment.MovieMetadata{
			Title: "The Matrix",
			Year:  1999,
		}, nil
	}
	srv.EnrichmentEngine().SetClient(mockClient)

	// Run scan in background
	scanDone := make(chan struct{})
	go func() {
		_, _ = srv.EnrichmentEngine().Scan(context.Background())
		close(scanDone)
	}()

	// Wait until metadata fetch starts (enrichment is active)
	<-fetchStarted

	// Check telemetry update while enrichment is active
	update := srv.getTelemetryUpdate(context.Background())
	if update.OpticalStatus != "Matching metadata" {
		t.Errorf("expected OpticalStatus to be 'Matching metadata' while enriching, got %q", update.OpticalStatus)
	}

	// Allow fetch to finish
	close(continueFetch)
	<-scanDone

	// After completion, telemetry should return to idle
	updateAfter := srv.getTelemetryUpdate(context.Background())
	if updateAfter.OpticalStatus != "Idle · Ready for disc" {
		t.Errorf("expected OpticalStatus to return to 'Idle · Ready for disc', got %q", updateAfter.OpticalStatus)
	}
}

func TestServer_OpticalRipper_TriggersEnrichmentScanOnComplete(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// Stage a movie disc
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs", "THE_MATRIX_1999")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "disc.iso"), []byte("disc-payload"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClient := enrichment.NewMockMetadataClient()
	mockClient.MockMovie = &enrichment.MovieMetadata{Title: "The Matrix", Year: 1999}
	srv.EnrichmentEngine().SetClient(mockClient)

	// Broadcast optical.disc_ejected with reason: complete
	srv.BroadcastEvent("optical.disc_ejected", map[string]any{
		"disc_id":      "THE_MATRIX_1999",
		"staging_path": stagingDir,
		"reason":       "complete",
	})

	// Wait for background scan to process staged disc
	deadline := time.Now().Add(5 * time.Second)
	movieTarget := filepath.Join(dataDir, "media", "movies", "The Matrix (1999)", "disc.iso")
	var success bool
	for time.Now().Before(deadline) {
		if _, err := os.Stat(movieTarget); err == nil {
			success = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !success {
		t.Fatalf("expected optical.disc_ejected event to trigger enrichment scan and promote movie to %q", movieTarget)
	}
}

func TestServer_Transcode_QueueLifecycle(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Initial queue should be empty
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
	getW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getW.Code)
	}

	var queueResp TranscodeQueueResponse
	if err := json.Unmarshal(getW.Body.Bytes(), &queueResp); err != nil {
		t.Fatalf("failed to decode queue response: %v", err)
	}
	if queueResp.Total != 0 || len(queueResp.Jobs) != 0 {
		t.Errorf("expected empty queue, got total=%d", queueResp.Total)
	}

	// 2. Validation error: empty source_path
	emptyBody := bytes.NewBufferString(`{"source_path": ""}`)
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/queue", emptyBody)
	emptyReq.Header.Set("Content-Type", "application/json")
	emptyW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(emptyW, emptyReq)

	if emptyW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty source_path, got %d", emptyW.Code)
	}

	// 3. Validation error: non-existent file
	missingBody := bytes.NewBufferString(`{"source_path": "/data/media/staging/does_not_exist.mkv"}`)
	missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/queue", missingBody)
	missingReq.Header.Set("Content-Type", "application/json")
	missingW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(missingW, missingReq)

	if missingW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-existent file, got %d", missingW.Code)
	}

	// 4. Create a valid test file and enqueue
	testVideo := filepath.Join(dataDir, "media", "staging", "sample_rip.mkv")
	_ = os.MkdirAll(filepath.Dir(testVideo), 0755)
	if err := os.WriteFile(testVideo, []byte("SAMPLE_RAW_VIDEO_STREAM"), 0644); err != nil {
		t.Fatalf("failed to create test video file: %v", err)
	}

	enqueuePayload := map[string]any{
		"source_path": testVideo,
		"profile":     "universal",
	}
	payloadBytes, _ := json.Marshal(enqueuePayload)
	enqReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/queue", bytes.NewReader(payloadBytes))
	enqReq.Header.Set("Content-Type", "application/json")
	enqW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enqW, enqReq)

	if enqW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for enqueue, got %d: %s", enqW.Code, enqW.Body.String())
	}

	var jobResp TranscodeJobResponse
	if err := json.Unmarshal(enqW.Body.Bytes(), &jobResp); err != nil {
		t.Fatalf("failed to parse job response: %v", err)
	}
	if jobResp.ID == "" {
		t.Errorf("expected generated job ID")
	}
	if jobResp.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", jobResp.Status)
	}
	if !strings.HasSuffix(jobResp.TargetPath, ".mp4") {
		t.Errorf("expected target_path ending in .mp4, got %q", jobResp.TargetPath)
	}

	// 5. Query queue again: should show 1 job
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
	getW2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getW2, getReq2)

	if getW2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getW2.Code)
	}
	var queueResp2 TranscodeQueueResponse
	_ = json.Unmarshal(getW2.Body.Bytes(), &queueResp2)
	if queueResp2.Total != 1 || len(queueResp2.Jobs) != 1 {
		t.Errorf("expected queue total=1, got %d", queueResp2.Total)
	}
	if queueResp2.Jobs[0].ID != jobResp.ID {
		t.Errorf("expected job ID %q, got %q", jobResp.ID, queueResp2.Jobs[0].ID)
	}
}

func TestServer_Transcode_Schedule(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. GET schedule (defaults to 01:00 - 07:00)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/schedule", nil)
	getW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getW.Code)
	}

	var schedResp TranscodeScheduleResponse
	if err := json.Unmarshal(getW.Body.Bytes(), &schedResp); err != nil {
		t.Fatalf("failed to parse schedule response: %v", err)
	}
	if schedResp.StartHour != 1 || schedResp.EndHour != 7 {
		t.Errorf("expected default 01:00-07:00, got %02d:%02d - %02d:%02d",
			schedResp.StartHour, schedResp.StartMinute, schedResp.EndHour, schedResp.EndMinute)
	}

	// 2. POST invalid schedule (hour 25)
	badBody := bytes.NewBufferString(`{"start_hour": 25, "end_hour": 7}`)
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/schedule", badBody)
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for hour 25, got %d", badW.Code)
	}

	// 3. POST valid schedule update (02:30 to 08:30)
	updBody := bytes.NewBufferString(`{"start_hour": 2, "start_minute": 30, "end_hour": 8, "end_minute": 30}`)
	updReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/schedule", updBody)
	updReq.Header.Set("Content-Type", "application/json")
	updW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(updW, updReq)

	if updW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for schedule update, got %d: %s", updW.Code, updW.Body.String())
	}

	var updatedResp TranscodeScheduleResponse
	if err := json.Unmarshal(updW.Body.Bytes(), &updatedResp); err != nil {
		t.Fatalf("failed to decode updated schedule: %v", err)
	}
	if updatedResp.StartHour != 2 || updatedResp.StartMinute != 30 || updatedResp.EndHour != 8 || updatedResp.EndMinute != 30 {
		t.Errorf("expected updated 02:30-08:30, got %02d:%02d - %02d:%02d",
			updatedResp.StartHour, updatedResp.StartMinute, updatedResp.EndHour, updatedResp.EndMinute)
	}
}

func TestServer_Transcode_Override(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Enable manual override
	onBody := bytes.NewBufferString(`{"force": true}`)
	onReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/override", onBody)
	onReq.Header.Set("Content-Type", "application/json")
	onW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(onW, onReq)

	if onW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", onW.Code, onW.Body.String())
	}

	var onResp TranscodeOverrideResponse
	if err := json.Unmarshal(onW.Body.Bytes(), &onResp); err != nil {
		t.Fatalf("failed to parse override response: %v", err)
	}
	if onResp.Status != "active" || !onResp.ManualOverride {
		t.Errorf("expected status 'active' and manual_override=true, got %+v", onResp)
	}

	// Verify schedule endpoint reflects manual override
	schedReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/schedule", nil)
	schedW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(schedW, schedReq)

	var schedResp TranscodeScheduleResponse
	_ = json.Unmarshal(schedW.Body.Bytes(), &schedResp)
	if !schedResp.ManualOverride {
		t.Errorf("expected schedule endpoint to report manual_override=true")
	}

	// 2. Disable manual override
	offBody := bytes.NewBufferString(`{"force": false}`)
	offReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/override", offBody)
	offReq.Header.Set("Content-Type", "application/json")
	offW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(offW, offReq)

	if offW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", offW.Code, offW.Body.String())
	}

	var offResp TranscodeOverrideResponse
	_ = json.Unmarshal(offW.Body.Bytes(), &offResp)
	if offResp.Status != "inactive" || offResp.ManualOverride {
		t.Errorf("expected status 'inactive' and manual_override=false, got %+v", offResp)
	}
}

func TestServer_Transcode_TelemetryGPUStatus(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Fix time to daytime (14:00)
	srv.TranscodeEngine().Scheduler().SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 14, 0, 0, 0, time.Local)
	})

	// 1. Daytime telemetry without override
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/events", nil)
	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	srv.Handler().ServeHTTP(w, req)

	// Verify initial SSE push contains gpu_status
	body := w.Body.String()
	if !strings.Contains(body, `"gpu_status":"Daytime (AI Priority)"`) {
		t.Errorf("expected SSE telemetry to include 'Daytime (AI Priority)', got: %s", body)
	}

	// 2. Enable manual override: GPU status transitions to Off-Peak
	srv.TranscodeEngine().SetManualOverride(true)

	w2 := httptest.NewRecorder()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/system/events", nil).WithContext(ctx2)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel2()
	}()
	srv.Handler().ServeHTTP(w2, req2)

	body2 := w2.Body.String()
	if !strings.Contains(body2, `"gpu_status":"Off-Peak (Transcoding Active)"`) {
		t.Errorf("expected SSE telemetry to include 'Off-Peak (Transcoding Active)' under manual override, got: %s", body2)
	}
}

func TestServer_Transcode_EndToEndBatchExecution(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// Configure night window
	srv.TranscodeEngine().Scheduler().SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 2, 0, 0, 0, time.Local) // 02:00
	})

	// Prepare raw rip in staging
	rawRip := filepath.Join(dataDir, "media", "staging", "movie_raw.mkv")
	_ = os.MkdirAll(filepath.Dir(rawRip), 0755)
	_ = os.WriteFile(rawRip, []byte("RAW_DISC_STREAM"), 0644)

	// Enqueue job via HTTP API
	targetMP4 := filepath.Join(dataDir, "media", "movies", "Test Movie (2024)", "movie.mp4")
	enqPayload := map[string]any{
		"source_path": rawRip,
		"target_path": targetMP4,
		"profile":     "universal",
	}
	body, _ := json.Marshal(enqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/queue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("enqueue failed with status %d: %s", w.Code, w.Body.String())
	}

	var job TranscodeJobResponse
	_ = json.Unmarshal(w.Body.Bytes(), &job)

	// Process job via engine
	err := srv.TranscodeEngine().ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce failed: %v", err)
	}

	// Verify target MP4 exists and is verified
	fi, err := os.Stat(targetMP4)
	if err != nil {
		t.Fatalf("expected target MP4 %q to exist: %v", targetMP4, err)
	}
	if fi.Size() == 0 {
		t.Fatalf("target MP4 is empty")
	}

	// Verify queue status is completed
	qReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
	qW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(qW, qReq)

	var qResp TranscodeQueueResponse
	_ = json.Unmarshal(qW.Body.Bytes(), &qResp)
	if len(qResp.Jobs) != 1 || qResp.Jobs[0].Status != "completed" {
		t.Errorf("expected 1 completed job in queue, got: %+v", qResp)
	}
}

func TestServer_Transcode_BackgroundProcessing(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configure schedule to be active during night
	srv.TranscodeEngine().Scheduler().SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 3, 0, 0, 0, time.Local)
	})

	// Start background orchestrator routines
	srv.StartBackgroundServices(ctx)

	// Prepare raw video in staging
	rawFile := filepath.Join(dataDir, "media", "staging", "auto_movie.mkv")
	_ = os.MkdirAll(filepath.Dir(rawFile), 0755)
	_ = os.WriteFile(rawFile, []byte("RAW_STREAM_FOR_AUTO_TRANSCODE"), 0644)

	targetMP4 := filepath.Join(dataDir, "media", "movies", "Auto Movie (2024)", "movie.mp4")
	enqPayload := map[string]any{
		"source_path": rawFile,
		"target_path": targetMP4,
		"profile":     "universal",
	}
	body, _ := json.Marshal(enqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/queue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("enqueue failed with status %d: %s", w.Code, w.Body.String())
	}

	// Poll until completed without calling ProcessOnce manually
	var completed bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		qReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
		qW := httptest.NewRecorder()
		srv.Handler().ServeHTTP(qW, qReq)

		var qResp TranscodeQueueResponse
		_ = json.Unmarshal(qW.Body.Bytes(), &qResp)
		if len(qResp.Jobs) > 0 && qResp.Jobs[0].Status == "completed" {
			completed = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !completed {
		t.Fatalf("transcode job was not processed by background service within timeout")
	}

	// Verify target MP4 exists and is non-empty
	fi, err := os.Stat(targetMP4)
	if err != nil {
		t.Fatalf("expected target MP4 to exist: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("target MP4 is empty")
	}
}

func TestServer_Transcode_Mobile_Endpoint(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Validation error: empty source_path
	emptyBody := bytes.NewBufferString(`{"source_path": "", "target_profile": "mobile-720p"}`)
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", emptyBody)
	emptyReq.Header.Set("Content-Type", "application/json")
	emptyW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(emptyW, emptyReq)
	if emptyW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty source_path, got %d", emptyW.Code)
	}

	// 2. Validation error: non-existent file
	missingBody := bytes.NewBufferString(`{"source_path": "/data/media/movies/NonExistent (2024)/movie.mkv", "target_profile": "mobile-720p"}`)
	missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", missingBody)
	missingReq.Header.Set("Content-Type", "application/json")
	missingW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(missingW, missingReq)
	if missingW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-existent file, got %d", missingW.Code)
	}

	// 3. Validation error: missing target_profile
	noProfBody := bytes.NewBufferString(`{"source_path": "/data/media/staging/test.mkv"}`)
	noProfReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", noProfBody)
	noProfReq.Header.Set("Content-Type", "application/json")
	noProfW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(noProfW, noProfReq)
	if noProfW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing target_profile, got %d", noProfW.Code)
	}

	// 4. Validation error: unknown profile (e.g. mobile-4k)
	badProfBody := bytes.NewBufferString(`{"source_path": "/data/media/staging/test.mkv", "target_profile": "mobile-4k"}`)
	badProfReq := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", badProfBody)
	badProfReq.Header.Set("Content-Type", "application/json")
	badProfW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(badProfW, badProfReq)
	if badProfW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unsupported mobile-4k profile, got %d", badProfW.Code)
	}

	// Create test movie master
	movieDir := filepath.Join(dataDir, "media", "movies", "Inception (2010)")
	_ = os.MkdirAll(movieDir, 0755)
	masterFile := filepath.Join(movieDir, "Inception (2010).mkv")
	_ = os.WriteFile(masterFile, []byte("MASTER_INCEPTION_RAW_STREAM"), 0644)

	// 5. Valid mobile-720p enqueue
	enq720Payload := map[string]string{
		"source_path":    masterFile,
		"target_profile": "mobile-720p",
	}
	body720, _ := json.Marshal(enq720Payload)
	req720 := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(body720))
	req720.Header.Set("Content-Type", "application/json")
	w720 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w720, req720)

	if w720.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for mobile-720p, got %d: %s", w720.Code, w720.Body.String())
	}

	var job720 TranscodeJobResponse
	if err := json.Unmarshal(w720.Body.Bytes(), &job720); err != nil {
		t.Fatalf("failed to decode job response: %v", err)
	}
	expectedTarget720 := filepath.Join(movieDir, "Inception (2010) - 720p.mp4")
	if job720.TargetPath != expectedTarget720 {
		t.Errorf("expected target_path %q, got %q", expectedTarget720, job720.TargetPath)
	}
	if job720.Profile != "mobile-720p" {
		t.Errorf("expected profile mobile-720p, got %q", job720.Profile)
	}
	if job720.Status != "queued" {
		t.Errorf("expected status queued, got %q", job720.Status)
	}

	// 5b. Duplicate mobile-720p enqueue while queued: returns 200 OK, identical job ID, single queue entry
	req720Dup := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(body720))
	req720Dup.Header.Set("Content-Type", "application/json")
	w720Dup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w720Dup, req720Dup)

	if w720Dup.Code != http.StatusCreated && w720Dup.Code != http.StatusOK {
		t.Fatalf("expected 201 Created or 200 OK for duplicate pending enqueue, got %d: %s", w720Dup.Code, w720Dup.Body.String())
	}
	var job720Dup TranscodeJobResponse
	if err := json.Unmarshal(w720Dup.Body.Bytes(), &job720Dup); err != nil {
		t.Fatalf("failed to decode duplicate job response: %v", err)
	}
	if job720Dup.ID != job720.ID {
		t.Errorf("expected identical job ID %q on duplicate enqueue, got %q", job720.ID, job720Dup.ID)
	}

	qReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
	qW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(qW, qReq)
	if qW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from queue endpoint, got %d", qW.Code)
	}
	var qResp TranscodeQueueResponse
	_ = json.Unmarshal(qW.Body.Bytes(), &qResp)
	matchingJobs := 0
	for _, j := range qResp.Jobs {
		if j.TargetPath == expectedTarget720 {
			matchingJobs++
		}
	}
	if matchingJobs != 1 {
		t.Errorf("expected exactly 1 queue entry for target %q, got %d", expectedTarget720, matchingJobs)
	}

	// 6. Valid mobile-1080p enqueue
	enq1080Payload := map[string]string{
		"source_path":    masterFile,
		"target_profile": "mobile-1080p",
	}
	body1080, _ := json.Marshal(enq1080Payload)
	req1080 := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(body1080))
	req1080.Header.Set("Content-Type", "application/json")
	w1080 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w1080, req1080)

	if w1080.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for mobile-1080p, got %d: %s", w1080.Code, w1080.Body.String())
	}

	var job1080 TranscodeJobResponse
	if err := json.Unmarshal(w1080.Body.Bytes(), &job1080); err != nil {
		t.Fatalf("failed to decode job response: %v", err)
	}
	expectedTarget1080 := filepath.Join(movieDir, "Inception (2010) - 1080p.mp4")
	if job1080.TargetPath != expectedTarget1080 {
		t.Errorf("expected target_path %q, got %q", expectedTarget1080, job1080.TargetPath)
	}

	// 7. Pre-existing mobile profile: returns 200 OK and skips redundant re-encode
	avatarDir := filepath.Join(dataDir, "media", "movies", "Avatar (2009)")
	_ = os.MkdirAll(avatarDir, 0755)
	avatarMaster := filepath.Join(avatarDir, "Avatar (2009).mkv")
	_ = os.WriteFile(avatarMaster, []byte("AVATAR_MASTER_STREAM"), 0644)
	avatarPreExisting := filepath.Join(avatarDir, "Avatar (2009) - 720p.mp4")
	_ = os.WriteFile(avatarPreExisting, []byte("PRE_ENCODED_720P_AVATAR"), 0644)

	avatarPayload := map[string]string{
		"source_path":    avatarMaster,
		"target_profile": "mobile-720p",
	}
	bodyAvatar, _ := json.Marshal(avatarPayload)
	reqAvatar := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(bodyAvatar))
	reqAvatar.Header.Set("Content-Type", "application/json")
	wAvatar := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAvatar, reqAvatar)

	if wAvatar.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for pre-existing mobile profile, got %d: %s", wAvatar.Code, wAvatar.Body.String())
	}
	var jobAvatar TranscodeJobResponse
	_ = json.Unmarshal(wAvatar.Body.Bytes(), &jobAvatar)
	if jobAvatar.Status != "completed" {
		t.Errorf("expected status 'completed' for pre-existing profile, got %q", jobAvatar.Status)
	}
	if jobAvatar.Progress != 100.0 {
		t.Errorf("expected progress 100.0, got %f", jobAvatar.Progress)
	}

	// 8. Auth gating: when admin setup is complete, unauthenticated calls must return 401
	setupPayload := `{
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

	var sessionCookie *http.Cookie
	for _, cookie := range wSetup.Result().Cookies() {
		if cookie.Name == "sovereign_session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("session cookie missing from setup response")
	}

	// Unauthenticated request post-setup
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(body720))
	reqUnauth.Header.Set("Content-Type", "application/json")
	wUnauth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated request after setup, got %d", wUnauth.Code)
	}

	// Authenticated request post-setup
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(bodyAvatar))
	reqAuth.Header.Set("Content-Type", "application/json")
	reqAuth.AddCookie(sessionCookie)
	wAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK for authenticated session, got %d: %s", wAuth.Code, wAuth.Body.String())
	}
}

func TestServer_Transcode_Mobile_E2E_ExecutionAndJellyfinRefresh(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Activate Jellyfin service and track library refresh
	if _, err := srv.ServiceManager().ActivateService(ctx, supervisor.ServiceJellyfin); err != nil {
		t.Fatalf("failed to activate jellyfin: %v", err)
	}

	refreshCalled := make(chan string, 1)
	srv.ServiceManager().JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		select {
		case refreshCalled <- supervisor.ServiceJellyfin:
		default:
		}
		return nil
	}

	// 2. Configure schedule to be active during night (03:00)
	srv.TranscodeEngine().Scheduler().SetNowFunc(func() time.Time {
		return time.Date(2026, 10, 16, 3, 0, 0, 0, time.Local)
	})

	// 3. Start background services
	srv.StartBackgroundServices(ctx)

	// 4. Create master movie file
	movieDir := filepath.Join(dataDir, "media", "movies", "Gladiator (2000)")
	_ = os.MkdirAll(movieDir, 0755)
	masterFile := filepath.Join(movieDir, "Gladiator (2000).mkv")
	masterInitialBytes := []byte("GLADIATOR_UNTOUCHED_ORIGINAL_MASTER_20GB")
	_ = os.WriteFile(masterFile, masterInitialBytes, 0644)
	masterFiBefore, err := os.Stat(masterFile)
	if err != nil {
		t.Fatalf("failed to stat master file: %v", err)
	}

	// 5. Enqueue mobile-720p transcode via REST API
	enqPayload := map[string]string{
		"source_path":    masterFile,
		"target_profile": "mobile-720p",
	}
	body, _ := json.Marshal(enqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transcode/mobile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to enqueue mobile transcode: %d - %s", w.Code, w.Body.String())
	}

	var jobResp TranscodeJobResponse
	_ = json.Unmarshal(w.Body.Bytes(), &jobResp)
	targetMP4 := filepath.Join(movieDir, "Gladiator (2000) - 720p.mp4")
	if jobResp.TargetPath != targetMP4 {
		t.Errorf("expected target path %q, got %q", targetMP4, jobResp.TargetPath)
	}

	// 6. Poll until background worker completes transcode
	var completed bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		qReq := httptest.NewRequest(http.MethodGet, "/api/v1/transcode/queue", nil)
		qW := httptest.NewRecorder()
		srv.Handler().ServeHTTP(qW, qReq)

		var qResp TranscodeQueueResponse
		_ = json.Unmarshal(qW.Body.Bytes(), &qResp)
		for _, j := range qResp.Jobs {
			if j.ID == jobResp.ID && j.Status == "completed" {
				completed = true
				break
			}
		}
		if completed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !completed {
		t.Fatalf("mobile transcode job did not complete within timeout")
	}

	// 7. Verify mobile profile was generated alongside master and is non-empty
	fiTarget, err := os.Stat(targetMP4)
	if err != nil {
		t.Fatalf("target mobile profile does not exist: %v", err)
	}
	if fiTarget.Size() == 0 {
		t.Errorf("target mobile profile is empty")
	}

	// 8. Verify master video file is completely untouched
	fiMasterAfter, err := os.Stat(masterFile)
	if err != nil {
		t.Fatalf("master file was deleted or lost: %v", err)
	}
	if fiMasterAfter.Size() != masterFiBefore.Size() {
		t.Errorf("master file size changed: before=%d, after=%d", masterFiBefore.Size(), fiMasterAfter.Size())
	}
	masterCurrentBytes, err := os.ReadFile(masterFile)
	if err != nil {
		t.Fatalf("failed to read master file: %v", err)
	}
	if string(masterCurrentBytes) != string(masterInitialBytes) {
		t.Errorf("master file contents altered")
	}

	// 9. Verify immediate Jellyfin library refresh was dispatched
	select {
	case svc := <-refreshCalled:
		if svc != supervisor.ServiceJellyfin {
			t.Errorf("expected refresh on %s, got %s", supervisor.ServiceJellyfin, svc)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for Jellyfin refresh webhook upon mobile transcode completion")
	}
}


