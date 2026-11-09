package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sovereign/internal/enrichment"
	"sovereign/internal/optical"
	"sovereign/internal/supervisor"
	"strings"
	"testing"
	"time"
)

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
