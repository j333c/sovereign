package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"strings"
	"testing"
	"time"
)

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

	if len(res.Services) != 6 {
		t.Fatalf("expected 6 services, got %d", len(res.Services))
	}

	expectedIDs := []string{"jellyfin", "navidrome", "audiobookshelf", "paperless", "llama", "openwebui"}
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
		DataDir: dataDir2,
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

func TestServer_Revisions_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. GET /api/v1/system/revisions
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/revisions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ComposeRevisionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 0 || len(resp.Revisions) != 0 {
		t.Fatalf("expected 0 revisions, got total=%d, len=%d", resp.Total, len(resp.Revisions))
	}

	// 2. GET /api/v1/system/revisions/active -> 404
	reqActive := httptest.NewRequest(http.MethodGet, "/api/v1/system/revisions/active", nil)
	rrActive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrActive, reqActive)

	if rrActive.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for no active revision, got %d", rrActive.Code)
	}
	var prob map[string]any
	if err := json.Unmarshal(rrActive.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob["title"] != "Not Found" {
		t.Errorf("expected title 'Not Found', got %v", prob["title"])
	}
}

func TestServer_ServiceUpgrade_Success(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Seed dummy configuration in /data/state/services/jellyfin
	jellyfinCfgDir := filepath.Join(dataDir, "state", "services", "jellyfin", "config")
	_ = os.MkdirAll(jellyfinCfgDir, 0755)
	_ = os.WriteFile(filepath.Join(jellyfinCfgDir, "config.json"), []byte(`{"theme": "dark"}`), 0644)

	// 2. POST /api/v1/services/jellyfin/upgrade with new image_tag
	body := bytes.NewBufferString(`{"image_tag": "12.3.0"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/upgrade", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var rev store.ComposeRevision
	if err := json.Unmarshal(rr.Body.Bytes(), &rev); err != nil {
		t.Fatalf("failed to parse revision response: %v", err)
	}

	if rev.Revision != 1 {
		t.Errorf("expected revision 1, got %d", rev.Revision)
	}
	if rev.ServiceID != "jellyfin" {
		t.Errorf("expected service jellyfin, got %q", rev.ServiceID)
	}
	if rev.NewImage != "jellyfin/jellyfin:12.3.0" {
		t.Errorf("expected new image jellyfin/jellyfin:12.3.0, got %q", rev.NewImage)
	}
	if rev.Status != "active" {
		t.Errorf("expected status active, got %q", rev.Status)
	}

	// Verify filesystem artifacts
	if _, err := os.Stat(rev.ComposeFilePath); err != nil {
		t.Fatalf("compose file not found at %s: %v", rev.ComposeFilePath, err)
	}
	if _, err := os.Stat(rev.SQLiteSnapshotPath); err != nil {
		t.Fatalf("sqlite snapshot file not found at %s: %v", rev.SQLiteSnapshotPath, err)
	}
	if _, err := os.Stat(rev.VolumeBackupPath); err != nil {
		t.Fatalf("volume backup path not found at %s: %v", rev.VolumeBackupPath, err)
	}

	// 3. GET /api/v1/system/revisions/active
	reqActive := httptest.NewRequest(http.MethodGet, "/api/v1/system/revisions/active", nil)
	rrActive := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrActive, reqActive)

	if rrActive.Code != http.StatusOK {
		t.Fatalf("expected status 200 for active revision, got %d: %s", rrActive.Code, rrActive.Body.String())
	}
	var activeRev store.ComposeRevision
	if err := json.Unmarshal(rrActive.Body.Bytes(), &activeRev); err != nil {
		t.Fatalf("failed to decode active revision: %v", err)
	}
	if activeRev.Revision != 1 || activeRev.ServiceID != "jellyfin" {
		t.Fatalf("unexpected active revision: %+v", activeRev)
	}

	// 4. GET /api/v1/system/revisions
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/system/revisions", nil)
	rrList := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("expected status 200 for list revisions, got %d: %s", rrList.Code, rrList.Body.String())
	}
	var listResp ComposeRevisionsResponse
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Revisions) != 1 {
		t.Fatalf("expected 1 revision in list, got %+v", listResp)
	}
	if listResp.ActiveRevision == nil || *listResp.ActiveRevision != 1 {
		t.Fatalf("expected active_revision 1, got %v", listResp.ActiveRevision)
	}
}

func TestServer_ServiceUpgrade_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/unknown-spoke/upgrade", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unknown service upgrade, got %d: %s", rr.Code, rr.Body.String())
	}
	var prob map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob["title"] != "Service Not Found" {
		t.Errorf("expected title 'Service Not Found', got %v", prob["title"])
	}
}

func TestServer_ServiceUpgrade_EmptyBody(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/upgrade", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for empty body upgrade, got %d: %s", rr.Code, rr.Body.String())
	}

	var rev store.ComposeRevision
	if err := json.Unmarshal(rr.Body.Bytes(), &rev); err != nil {
		t.Fatalf("failed to decode revision: %v", err)
	}
	if rev.Revision != 1 || rev.ServiceID != "jellyfin" {
		t.Fatalf("unexpected revision: %+v", rev)
	}
}

func TestServer_ServiceUpgrade_MalformedJSON(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/upgrade", bytes.NewBufferString(`{malformed json}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for malformed json upgrade, got %d: %s", rr.Code, rr.Body.String())
	}
	var prob map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob["title"] != "Invalid Request" {
		t.Errorf("expected title 'Invalid Request', got %v", prob["title"])
	}
}

func TestServer_ServiceRollback_Success(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Upgrade jellyfin first
	reqUp := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/upgrade", bytes.NewBufferString(`{"image_tag":"jellyfin/jellyfin:12.3.0"}`))
	reqUp.Header.Set("Content-Type", "application/json")
	rrUp := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrUp, reqUp)
	if rrUp.Code != http.StatusOK {
		t.Fatalf("expected status 200 for upgrade, got %d: %s", rrUp.Code, rrUp.Body.String())
	}

	// 2. Perform rollback
	reqRoll := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/rollback", bytes.NewBufferString(`{"revision":1}`))
	reqRoll.Header.Set("Content-Type", "application/json")
	rrRoll := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrRoll, reqRoll)

	if rrRoll.Code != http.StatusOK {
		t.Fatalf("expected status 200 for rollback, got %d: %s", rrRoll.Code, rrRoll.Body.String())
	}

	var status supervisor.ServiceStatus
	if err := json.Unmarshal(rrRoll.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode service status: %v", err)
	}
	if status.Status != supervisor.StatusRunning {
		t.Errorf("expected status running, got %q", status.Status)
	}
	if status.Health != supervisor.HealthHealthy {
		t.Errorf("expected health healthy, got %q", status.Health)
	}
}

func TestServer_ServiceRollback_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/nonexistent/rollback", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for nonexistent service, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServer_ServiceRollback_MalformedJSON(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/rollback", bytes.NewBufferString(`{malformed`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for malformed json, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServer_ServiceRollback_NoPriorRevision(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Jellyfin has not been upgraded yet, so no revisions exist
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/jellyfin/rollback", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 when no prior revisions exist, got %d: %s", rr.Code, rr.Body.String())
	}
}
