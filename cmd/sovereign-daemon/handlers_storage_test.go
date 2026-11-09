package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/storage"
	"sovereign/internal/supervisor"
	"strings"
	"testing"
	"time"
)

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

func TestDropUpload_InsufficientStorage(t *testing.T) {
	srv, dataDir := setupTestServer(t)
	defer srv.Close()

	// 1. Pre-flight check via Content-Length: free space 5GB, Content-Length 10GB (+1GB buffer = 11GB needed)
	srv.dropzoneEngine.SetDiskFree(func(path string) (int64, error) {
		return 5 * 1024 * 1024 * 1024, nil // 5 GB available
	})

	mp4Payload := []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2mp41" + strings.Repeat("A", 100))
	body, contentType, err := createMultipartUpload("movies", "sample.mp4", mp4Payload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = 10 * 1024 * 1024 * 1024 // 10 GB
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected HTTP 507 Insufficient Storage on pre-flight capacity check, got %d: %s", w.Code, w.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed decoding Problem Details: %v", err)
	}
	if prob.Type != "https://sovereign.local/errors/insufficient-storage" {
		t.Errorf("expected type 'https://sovereign.local/errors/insufficient-storage', got %q", prob.Type)
	}
	if prob.Status != 507 {
		t.Errorf("expected status 507 in problem details, got %d", prob.Status)
	}

	// 2. Runtime free space check: free space < payload + 1GB safety buffer
	srv.dropzoneEngine.SetDiskFree(func(path string) (int64, error) {
		return 500 * 1024 * 1024, nil // 500 MB (< 1GB safety buffer)
	})

	body2, contentType2, err := createMultipartUpload("movies", "sample.mp4", mp4Payload)
	if err != nil {
		t.Fatal(err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/drop/upload", body2)
	req2.Header.Set("Content-Type", contentType2)
	req2.ContentLength = -1 // Streaming upload without Content-Length: tests HandleUploadStream runtime capacity check
	w2 := httptest.NewRecorder()
	srv.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected HTTP 507 Insufficient Storage on runtime capacity check, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify no files were promoted to destination
	destFile := filepath.Join(dataDir, "media", "movies", "sample.mp4")
	if _, err := os.Stat(destFile); !os.IsNotExist(err) {
		t.Errorf("expected destination file %q not to exist after 507 rejection", destFile)
	}
}
