package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/hardware"
	"sovereign/internal/ingress"
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
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

	srv, err := NewServer(Config{
		DataDir:    dataDir,
		ServiceMgr: svcMgr,
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
	status    map[string]any
	err       error
	syncCalls int
	mu        sync.Mutex
}

func (m *mockIngressManager) SyncRoutes(ctx context.Context, routes []ingress.RouteConfig) error {
	m.mu.Lock()
	m.syncCalls++
	m.mu.Unlock()
	return m.err
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

	if len(res.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(res.Services))
	}

	expectedIDs := []string{"jellyfin", "navidrome", "audiobookshelf"}
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
