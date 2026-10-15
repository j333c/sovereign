package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sovereign/internal/ingress"
	"sovereign/internal/storage"
	"sovereign/internal/store"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	// Ensure clean shutdown state for default test server
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

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


