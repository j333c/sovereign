package supervisor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestAudioProvisioner_ProvisionNavidrome(t *testing.T) {
	tempDir := t.TempDir()
	p := NewAudioProvisioner(tempDir, nil)

	// Pre-create user media to verify preservation
	musicDir := filepath.Join(tempDir, "media", "music")
	if err := os.MkdirAll(musicDir, 0755); err != nil {
		t.Fatalf("failed to create music dir: %v", err)
	}
	existingTrack := filepath.Join(musicDir, "artist-song.flac")
	if err := os.WriteFile(existingTrack, []byte("audio-content-sample"), 0644); err != nil {
		t.Fatalf("failed to write test music file: %v", err)
	}

	dataDir := filepath.Join(tempDir, "state", "services", "navidrome", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	existingDB := filepath.Join(dataDir, "navidrome.db")
	if err := os.WriteFile(existingDB, []byte("sqlite-database-header"), 0644); err != nil {
		t.Fatalf("failed to write test db file: %v", err)
	}

	res, err := p.Provision(context.Background(), ServiceNavidrome)
	if err != nil {
		t.Fatalf("ProvisionNavidrome failed: %v", err)
	}

	if res.ServiceID != ServiceNavidrome {
		t.Errorf("expected ServiceID %s, got %s", ServiceNavidrome, res.ServiceID)
	}

	// Verify required environment variables
	expectedEnv := map[string]string{
		"ND_MUSICFOLDER":  "/music",
		"ND_DATAFOLDER":   "/data",
		"ND_PORT":         "4533",
		"ND_LOGLEVEL":     "info",
		"ND_SCANINTERVAL": "1m",
	}
	for k, expectedVal := range expectedEnv {
		if val, ok := res.Env[k]; !ok || val != expectedVal {
			t.Errorf("expected env %s=%s, got %s (exists=%v)", k, expectedVal, val, ok)
		}
	}

	// Verify preservation of existing files
	content, err := os.ReadFile(existingTrack)
	if err != nil || string(content) != "audio-content-sample" {
		t.Errorf("existing track corrupted or altered: %v", err)
	}
	dbContent, err := os.ReadFile(existingDB)
	if err != nil || string(dbContent) != "sqlite-database-header" {
		t.Errorf("existing db corrupted or altered: %v", err)
	}
}

func TestAudioProvisioner_ProvisionAudiobookshelf(t *testing.T) {
	tempDir := t.TempDir()
	p := NewAudioProvisioner(tempDir, nil)

	// Pre-create audiobook and configuration files to verify persistence
	audiobooksDir := filepath.Join(tempDir, "media", "audiobooks")
	if err := os.MkdirAll(audiobooksDir, 0755); err != nil {
		t.Fatalf("failed to create audiobooks dir: %v", err)
	}
	existingBook := filepath.Join(audiobooksDir, "classic-novel.m4b")
	if err := os.WriteFile(existingBook, []byte("m4b-audiobook-data"), 0644); err != nil {
		t.Fatalf("failed to write test audiobook file: %v", err)
	}

	configDir := filepath.Join(tempDir, "state", "services", "audiobookshelf", "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	userDB := filepath.Join(configDir, "users.sqlite")
	if err := os.WriteFile(userDB, []byte("user-accounts-and-progress"), 0644); err != nil {
		t.Fatalf("failed to write test user db: %v", err)
	}

	metadataDir := filepath.Join(tempDir, "state", "services", "audiobookshelf", "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}
	chapterInfo := filepath.Join(metadataDir, "chapters.json")
	if err := os.WriteFile(chapterInfo, []byte(`{"chapters":[{"title":"Chapter 1","start":0}]}`), 0644); err != nil {
		t.Fatalf("failed to write test chapter metadata: %v", err)
	}

	res, err := p.Provision(context.Background(), ServiceAudiobookshelf)
	if err != nil {
		t.Fatalf("ProvisionAudiobookshelf failed: %v", err)
	}

	if res.ServiceID != ServiceAudiobookshelf {
		t.Errorf("expected ServiceID %s, got %s", ServiceAudiobookshelf, res.ServiceID)
	}

	// Verify required environment variables
	expectedEnv := map[string]string{
		"PORT":               "13378",
		"CONFIG_PATH":        "/config",
		"METADATA_PATH":      "/metadata",
		"AUDIOBOOKSHELF_UID": "1000",
		"AUDIOBOOKSHELF_GID": "1000",
	}
	for k, expectedVal := range expectedEnv {
		if val, ok := res.Env[k]; !ok || val != expectedVal {
			t.Errorf("expected env %s=%s, got %s (exists=%v)", k, expectedVal, val, ok)
		}
	}

	// Verify persistence across provisioning
	bookData, err := os.ReadFile(existingBook)
	if err != nil || string(bookData) != "m4b-audiobook-data" {
		t.Errorf("existing audiobook corrupted or altered: %v", err)
	}
	userData, err := os.ReadFile(userDB)
	if err != nil || string(userData) != "user-accounts-and-progress" {
		t.Errorf("existing user db corrupted or altered: %v", err)
	}
	metaData, err := os.ReadFile(chapterInfo)
	if err != nil || !strings.Contains(string(metaData), "Chapter 1") {
		t.Errorf("existing chapter metadata corrupted or altered: %v", err)
	}
}

func TestAudioProvisioner_NeverMountDockerSocket(t *testing.T) {
	mgr, _, _, _ := setupTestManager(t, nil)
	defer mgr.Close()

	audioServices := []string{ServiceNavidrome, ServiceAudiobookshelf}
	for _, id := range audioServices {
		def, exists := mgr.definitions[id]
		if !exists {
			t.Fatalf("service %s not found in definitions", id)
		}

		for _, tmpl := range def.VolumeTemplates {
			if strings.Contains(tmpl, "docker.sock") {
				t.Fatalf("CRITICAL SECURITY VIOLATION: service %s volume template contains docker.sock: %s", id, tmpl)
			}
		}

		resolved := resolveVolumes("/data", def.VolumeTemplates)
		for _, v := range resolved {
			if strings.Contains(v, "docker.sock") {
				t.Fatalf("CRITICAL SECURITY VIOLATION: service %s resolved volume contains docker.sock: %s", id, v)
			}
		}
	}
}

func TestAudioProvisioner_NavidromeRefresh_HTTP(t *testing.T) {
	receivedMethod := ""
	receivedPath := ""
	receivedBody := ""
	receivedContentType := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"scanning"}`))
	}))
	defer server.Close()

	p := NewAudioProvisioner(t.TempDir(), nil)
	p.SetNavidromeBaseURL(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.TriggerNavidromeRefresh(ctx); err != nil {
		t.Fatalf("TriggerNavidromeRefresh failed: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedPath != "/api/scan" {
		t.Errorf("expected path /api/scan, got %s", receivedPath)
	}
	if !strings.Contains(receivedContentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}
	if !strings.Contains(receivedBody, "fullScan") {
		t.Errorf("expected body to contain fullScan, got %s", receivedBody)
	}

	// Test 500 error propagation
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	p.SetNavidromeBaseURL(errorServer.URL)
	if err := p.TriggerNavidromeRefresh(ctx); err == nil {
		t.Errorf("expected error on HTTP 500, got nil")
	}
}

func TestAudioProvisioner_AudiobookshelfRefresh_HTTP(t *testing.T) {
	receivedMethod := ""
	receivedPath := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	p := NewAudioProvisioner(t.TempDir(), nil)
	p.SetAudiobookshelfBaseURL(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.TriggerAudiobookshelfRefresh(ctx); err != nil {
		t.Fatalf("TriggerAudiobookshelfRefresh failed: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedPath != "/api/libraries/scan" {
		t.Errorf("expected path /api/libraries/scan, got %s", receivedPath)
	}

	// Test 502 error propagation
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer errorServer.Close()

	p.SetAudiobookshelfBaseURL(errorServer.URL)
	if err := p.TriggerAudiobookshelfRefresh(ctx); err == nil {
		t.Errorf("expected error on HTTP 502, got nil")
	}
}

func TestSupervisor_Audio_Activation_And_Compose_Synthesis(t *testing.T) {
	tempDir := t.TempDir()
	mockEngine := NewMockRuntimeEngine()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})
	defer mgr.Close()

	ctx := context.Background()

	// 1. Activate Navidrome
	navSvc, err := mgr.ActivateService(ctx, ServiceNavidrome)
	if err != nil {
		t.Fatalf("failed to activate navidrome: %v", err)
	}
	if navSvc.Status != StatusRunning {
		t.Errorf("expected navidrome status running, got %s", navSvc.Status)
	}
	if navSvc.Port != 4533 {
		t.Errorf("expected navidrome port 4533, got %d", navSvc.Port)
	}
	if navSvc.Subpath != "/music" {
		t.Errorf("expected navidrome subpath /music, got %s", navSvc.Subpath)
	}

	// Verify Navidrome Compose YAML on disk
	navComposePath := filepath.Join(tempDir, "state", "compose", "navidrome.yaml")
	navYAMLBytes, err := os.ReadFile(navComposePath)
	if err != nil {
		t.Fatalf("failed to read navidrome compose yaml: %v", err)
	}
	var navCompose ComposeFile
	if err := yaml.Unmarshal(navYAMLBytes, &navCompose); err != nil {
		t.Fatalf("failed to unmarshal navidrome compose yaml: %v", err)
	}
	navSpec, ok := navCompose.Services["navidrome"]
	if !ok {
		t.Fatalf("navidrome service not found in compose file")
	}
	if navSpec.Image != ImageNavidrome {
		t.Errorf("expected image %s, got %s", ImageNavidrome, navSpec.Image)
	}
	if navSpec.Environment["ND_MUSICFOLDER"] != "/music" || navSpec.Environment["ND_PORT"] != "4533" {
		t.Errorf("navidrome compose environment missing ND keys: %v", navSpec.Environment)
	}
	// Verify volume mounts include music as :ro
	roFound := false
	for _, v := range navSpec.Volumes {
		if strings.Contains(v, "media/music:/music:ro") {
			roFound = true
			break
		}
	}
	if !roFound {
		t.Errorf("expected /music to be mounted :ro, volumes: %v", navSpec.Volumes)
	}

	// 2. Activate Audiobookshelf
	absSvc, err := mgr.ActivateService(ctx, ServiceAudiobookshelf)
	if err != nil {
		t.Fatalf("failed to activate audiobookshelf: %v", err)
	}
	if absSvc.Status != StatusRunning {
		t.Errorf("expected audiobookshelf status running, got %s", absSvc.Status)
	}
	if absSvc.Port != 13378 {
		t.Errorf("expected audiobookshelf port 13378, got %d", absSvc.Port)
	}
	if absSvc.Subpath != "/audiobooks" {
		t.Errorf("expected audiobookshelf subpath /audiobooks, got %s", absSvc.Subpath)
	}

	// Verify Audiobookshelf Compose YAML on disk
	absComposePath := filepath.Join(tempDir, "state", "compose", "audiobookshelf.yaml")
	absYAMLBytes, err := os.ReadFile(absComposePath)
	if err != nil {
		t.Fatalf("failed to read audiobookshelf compose yaml: %v", err)
	}
	var absCompose ComposeFile
	if err := yaml.Unmarshal(absYAMLBytes, &absCompose); err != nil {
		t.Fatalf("failed to unmarshal audiobookshelf compose yaml: %v", err)
	}
	absSpec, ok := absCompose.Services["audiobookshelf"]
	if !ok {
		t.Fatalf("audiobookshelf service not found in compose file")
	}
	if absSpec.Image != ImageAudiobookshelf {
		t.Errorf("expected image %s, got %s", ImageAudiobookshelf, absSpec.Image)
	}
	if absSpec.Environment["PORT"] != "13378" || absSpec.Environment["CONFIG_PATH"] != "/config" {
		t.Errorf("audiobookshelf compose environment missing PORT/CONFIG_PATH: %v", absSpec.Environment)
	}
	// Verify memory limits: 512M
	if absSpec.Deploy == nil || absSpec.Deploy.Resources.Limits.Memory != "512M" {
		t.Errorf("expected 512M memory limit in compose file, got %v", absSpec.Deploy)
	}

	// Verify Audiobookshelf volume mounts
	expectedAbsVols := []string{
		"media/audiobooks:/audiobooks",
		"state/services/audiobookshelf/config:/config",
		"state/services/audiobookshelf/metadata:/metadata",
	}
	for _, exp := range expectedAbsVols {
		found := false
		for _, v := range absSpec.Volumes {
			if strings.Contains(v, exp) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected volume containing %s, got volumes: %v", exp, absSpec.Volumes)
		}
	}
}

func TestSupervisor_Audio_Bundle_Activation(t *testing.T) {
	tempDir := t.TempDir()
	mockEngine := NewMockRuntimeEngine()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})
	defer mgr.Close()

	ctx := context.Background()

	// Bundle activation via alias "audio"
	bundleSvc, err := mgr.ActivateService(ctx, "audio")
	if err != nil {
		t.Fatalf("failed to activate audio bundle: %v", err)
	}
	if bundleSvc == nil {
		t.Fatalf("expected non-nil service status from audio bundle activation")
	}

	// Verify BOTH Navidrome and Audiobookshelf are now running
	navStatus, err := mgr.GetService(ctx, ServiceNavidrome)
	if err != nil || navStatus.Status != StatusRunning {
		t.Errorf("expected navidrome to be running after audio bundle activation, got status=%s, err=%v", navStatus.Status, err)
	}

	absStatus, err := mgr.GetService(ctx, ServiceAudiobookshelf)
	if err != nil || absStatus.Status != StatusRunning {
		t.Errorf("expected audiobookshelf to be running after audio bundle activation, got status=%s, err=%v", absStatus.Status, err)
	}

	// Verify audio bundle restart
	restartedSvc, err := mgr.RestartService(ctx, "audio")
	if err != nil {
		t.Fatalf("failed to restart audio bundle: %v", err)
	}
	if restartedSvc == nil {
		t.Fatalf("expected non-nil service status from audio bundle restart")
	}
	navStatus, err = mgr.GetService(ctx, ServiceNavidrome)
	if err != nil || navStatus.Status != StatusRunning {
		t.Errorf("expected navidrome to be running after audio bundle restart, got status=%s, err=%v", navStatus.Status, err)
	}
	absStatus, err = mgr.GetService(ctx, ServiceAudiobookshelf)
	if err != nil || absStatus.Status != StatusRunning {
		t.Errorf("expected audiobookshelf to be running after audio bundle restart, got status=%s, err=%v", absStatus.Status, err)
	}

	// Verify audio bundle deactivation
	if _, err := mgr.DeactivateService(ctx, "audio"); err != nil {
		t.Fatalf("failed to deactivate audio bundle: %v", err)
	}
	navStatus, err = mgr.GetService(ctx, ServiceNavidrome)
	if err != nil || navStatus.Status != StatusInactive {
		t.Errorf("expected navidrome to be inactive after audio bundle deactivation, got status=%s, err=%v", navStatus.Status, err)
	}
	absStatus, err = mgr.GetService(ctx, ServiceAudiobookshelf)
	if err != nil || absStatus.Status != StatusInactive {
		t.Errorf("expected audiobookshelf to be inactive after audio bundle deactivation, got status=%s, err=%v", absStatus.Status, err)
	}
}

func TestSupervisor_Audio_Bundle_Activation_FailureGraceful(t *testing.T) {
	tempDir := t.TempDir()
	mockEngine := NewMockRuntimeEngine()
	// Fail image pull for Audiobookshelf
	mockEngine.SetPullFailure(ImageAudiobookshelf, errors.New("simulated network error"))

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})
	defer mgr.Close()

	ctx := context.Background()

	_, err := mgr.ActivateService(ctx, "audio")
	if err == nil {
		t.Fatalf("expected error when audiobookshelf fails to pull in bundle, got nil")
	}
	if !strings.Contains(err.Error(), "audio bundle failed") {
		t.Errorf("expected descriptive audio bundle error, got: %v", err)
	}

	// Verify Navidrome was rolled back and is not running
	navStatus, err := mgr.GetService(ctx, ServiceNavidrome)
	if err != nil {
		t.Fatalf("failed to get navidrome status: %v", err)
	}
	if navStatus.Status == StatusRunning {
		t.Errorf("expected navidrome to be rolled back (not running) after audio bundle failure, got: %s", navStatus.Status)
	}
}

func TestSupervisor_Audio_RefreshService_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	mockEngine := NewMockRuntimeEngine()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})
	defer mgr.Close()

	ctx := context.Background()

	// 1. Refreshing while inactive returns ErrServiceInactive (503)
	errNavInactive := mgr.RefreshService(ctx, ServiceNavidrome)
	if !errors.Is(errNavInactive, ErrServiceInactive) {
		t.Errorf("expected ErrServiceInactive for inactive navidrome, got: %v", errNavInactive)
	}

	errAbsInactive := mgr.RefreshService(ctx, ServiceAudiobookshelf)
	if !errors.Is(errAbsInactive, ErrServiceInactive) {
		t.Errorf("expected ErrServiceInactive for inactive audiobookshelf, got: %v", errAbsInactive)
	}

	// 2. Activate services
	_, err := mgr.ActivateService(ctx, ServiceNavidrome)
	if err != nil {
		t.Fatalf("failed to activate navidrome: %v", err)
	}
	_, err = mgr.ActivateService(ctx, ServiceAudiobookshelf)
	if err != nil {
		t.Fatalf("failed to activate audiobookshelf: %v", err)
	}

	// 3. Register refresh hooks
	navRefreshed := false
	absRefreshed := false
	mgr.AudioProvisioner().NavidromeRefreshFunc = func(ctx context.Context) error {
		navRefreshed = true
		return nil
	}
	mgr.AudioProvisioner().AudiobookshelfRefreshFunc = func(ctx context.Context) error {
		absRefreshed = true
		return nil
	}

	// 4. Test Navidrome refresh
	if err := mgr.RefreshService(ctx, ServiceNavidrome); err != nil {
		t.Fatalf("RefreshService(navidrome) failed: %v", err)
	}
	if !navRefreshed {
		t.Errorf("expected Navidrome refresh hook to be called")
	}

	// 5. Test Audiobookshelf refresh
	if err := mgr.RefreshService(ctx, ServiceAudiobookshelf); err != nil {
		t.Fatalf("RefreshService(audiobookshelf) failed: %v", err)
	}
	if !absRefreshed {
		t.Errorf("expected Audiobookshelf refresh hook to be called")
	}

	// 6. Test Aliases
	navRefreshed = false
	if err := mgr.RefreshService(ctx, "music"); err != nil {
		t.Fatalf("RefreshService(\"music\") failed: %v", err)
	}
	if !navRefreshed {
		t.Errorf("expected alias 'music' to trigger Navidrome refresh")
	}

	absRefreshed = false
	if err := mgr.RefreshService(ctx, "audiobooks"); err != nil {
		t.Fatalf("RefreshService(\"audiobooks\") failed: %v", err)
	}
	if !absRefreshed {
		t.Errorf("expected alias 'audiobooks' to trigger Audiobookshelf refresh")
	}

	// 7. Test bundle refresh "audio"
	navRefreshed = false
	absRefreshed = false
	if err := mgr.RefreshService(ctx, "audio"); err != nil {
		t.Fatalf("RefreshService(\"audio\") bundle refresh failed: %v", err)
	}
	if !navRefreshed || !absRefreshed {
		t.Errorf("expected bundle refresh to trigger both navidrome (%v) and audiobookshelf (%v)", navRefreshed, absRefreshed)
	}

	// 8. Test partial bundle refresh when only one service is running
	if _, err := mgr.DeactivateService(ctx, ServiceNavidrome); err != nil {
		t.Fatalf("failed to deactivate navidrome: %v", err)
	}
	navRefreshed = false
	absRefreshed = false
	if err := mgr.RefreshService(ctx, "audio"); err != nil {
		t.Fatalf("expected partial bundle refresh to succeed when audiobookshelf is active, got: %v", err)
	}
	if navRefreshed {
		t.Errorf("did not expect inactive navidrome to be refreshed")
	}
	if !absRefreshed {
		t.Errorf("expected active audiobookshelf to be refreshed during partial bundle refresh")
	}

	// 9. When both are inactive, bundle refresh returns ErrServiceInactive
	if _, err := mgr.DeactivateService(ctx, ServiceAudiobookshelf); err != nil {
		t.Fatalf("failed to deactivate audiobookshelf: %v", err)
	}
	if err := mgr.RefreshService(ctx, "audio"); !errors.Is(err, ErrServiceInactive) {
		t.Errorf("expected ErrServiceInactive when both audio services are inactive, got: %v", err)
	}
}
