package supervisor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJellyfin_DirectoryProvisioning(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	prov := NewJellyfinProvisioner(dataDir, nil, HardwareProbeOptions{})

	ctx := context.Background()
	res, err := prov.Provision(ctx)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	if res == nil {
		t.Fatalf("expected non-nil JellyfinProvisionResult")
	}

	expectedDirs := []string{
		filepath.Join(dataDir, "media", "movies"),
		filepath.Join(dataDir, "media", "tv"),
		filepath.Join(dataDir, "state", "services", "jellyfin", "config"),
		filepath.Join(dataDir, "state", "services", "jellyfin", "cache"),
	}

	for _, d := range expectedDirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	// Invariant: Never delete or alter existing media files during provisioning
	testMovie := filepath.Join(dataDir, "media", "movies", "ExistingMovie.mkv")
	if err := os.WriteFile(testMovie, []byte("existing-movie-data"), 0644); err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	// Re-run provisioning
	_, err = prov.Provision(ctx)
	if err != nil {
		t.Fatalf("subsequent provision failed: %v", err)
	}

	data, err := os.ReadFile(testMovie)
	if err != nil {
		t.Fatalf("existing movie file was removed: %v", err)
	}
	if string(data) != "existing-movie-data" {
		t.Errorf("existing movie data altered: %s", string(data))
	}
}

func TestJellyfin_DirectPlayConfigGeneration_Default(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	prov := NewJellyfinProvisioner(dataDir, nil, HardwareProbeOptions{
		DevRoot:   filepath.Join(tempDir, "nonexistent-dev"),
		SysfsRoot: filepath.Join(tempDir, "nonexistent-sys"),
		ProcRoot:  filepath.Join(tempDir, "nonexistent-proc"),
	})

	ctx := context.Background()
	_, err := prov.Provision(ctx)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	configDir := filepath.Join(dataDir, "state", "services", "jellyfin", "config")

	// 1. Verify system.xml
	sysFile := filepath.Join(configDir, "system.xml")
	sysBytes, err := os.ReadFile(sysFile)
	if err != nil {
		t.Fatalf("failed to read system.xml: %v", err)
	}
	sysContent := string(sysBytes)

	if !strings.Contains(sysContent, "<ServerName>Sovereign</ServerName>") {
		t.Errorf("system.xml missing ServerName: %s", sysContent)
	}
	if !strings.Contains(sysContent, "<Port>8096</Port>") {
		t.Errorf("system.xml missing Port 8096: %s", sysContent)
	}
	if !strings.Contains(sysContent, "<PublicPort>8096</PublicPort>") {
		t.Errorf("system.xml missing PublicPort 8096: %s", sysContent)
	}
	if !strings.Contains(sysContent, "<EnableUPnP>true</EnableUPnP>") {
		t.Errorf("system.xml missing EnableUPnP: %s", sysContent)
	}
	if !strings.Contains(sysContent, "<IsStartupWizardCompleted>false</IsStartupWizardCompleted>") {
		t.Errorf("system.xml missing IsStartupWizardCompleted false: %s", sysContent)
	}

	// 2. Verify encoding.xml prioritizes direct-play
	encFile := filepath.Join(configDir, "encoding.xml")
	encBytes, err := os.ReadFile(encFile)
	if err != nil {
		t.Fatalf("failed to read encoding.xml: %v", err)
	}
	encContent := string(encBytes)

	if !strings.Contains(encContent, "<EnableAudioVbr>true</EnableAudioVbr>") {
		t.Errorf("encoding.xml missing EnableAudioVbr: %s", encContent)
	}
	if !strings.Contains(encContent, "<EnableDirectPlay>true</EnableDirectPlay>") {
		t.Errorf("encoding.xml missing EnableDirectPlay: %s", encContent)
	}
	if !strings.Contains(encContent, "<EnableDirectStream>true</EnableDirectStream>") {
		t.Errorf("encoding.xml missing EnableDirectStream: %s", encContent)
	}
	if !strings.Contains(encContent, "<HardwareAccelerationType>none</HardwareAccelerationType>") {
		t.Errorf("encoding.xml expected HardwareAccelerationType none, got: %s", encContent)
	}
}

func TestJellyfin_DirectPlayConfigGeneration_ReusesExisting(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	configDir := filepath.Join(dataDir, "state", "services", "jellyfin", "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	customSys := "<ServerConfiguration><ServerName>CustomJellyfin</ServerName></ServerConfiguration>"
	customEnc := "<EncodingOptions><EnableDirectPlay>true</EnableDirectPlay><CustomUserParam>1</CustomUserParam></EncodingOptions>"

	sysFile := filepath.Join(configDir, "system.xml")
	encFile := filepath.Join(configDir, "encoding.xml")

	if err := os.WriteFile(sysFile, []byte(customSys), 0644); err != nil {
		t.Fatalf("failed to write custom system.xml: %v", err)
	}
	if err := os.WriteFile(encFile, []byte(customEnc), 0644); err != nil {
		t.Fatalf("failed to write custom encoding.xml: %v", err)
	}

	prov := NewJellyfinProvisioner(dataDir, nil, HardwareProbeOptions{})

	_, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify custom configuration was strictly preserved!
	readSys, _ := os.ReadFile(sysFile)
	if string(readSys) != customSys {
		t.Errorf("system.xml was overwritten! got: %s", string(readSys))
	}
	readEnc, _ := os.ReadFile(encFile)
	if string(readEnc) != customEnc {
		t.Errorf("encoding.xml was overwritten! got: %s", string(readEnc))
	}
}

func TestJellyfin_HardwareDetection_IntelDRM(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")

	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte("mock-device"), 0644)

	// Vendor file for Intel (0x8086)
	vendorDir := filepath.Join(sysDir, "class", "drm", "renderD128", "device")
	_ = os.MkdirAll(vendorDir, 0755)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor"), []byte("0x8086\n"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelQSV {
		t.Errorf("expected HWAccelQSV for Intel vendor 0x8086, got %s", hw.Type)
	}
	if len(hw.Devices) != 1 || hw.Devices[0] != "/dev/dri/renderD128:/dev/dri/renderD128" {
		t.Errorf("expected devices [/dev/dri/renderD128:/dev/dri/renderD128], got %v", hw.Devices)
	}
	if hw.VaapiDevice != "/dev/dri/renderD128" {
		t.Errorf("expected vaapi device /dev/dri/renderD128, got %s", hw.VaapiDevice)
	}

	// Verify XML generated with QSV
	dataDir := filepath.Join(tempDir, "data")
	prov := NewJellyfinProvisioner(dataDir, nil, opts)
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	if res.HWAccel.Type != HWAccelQSV {
		t.Errorf("expected provision result HWAccel.Type == qsv, got %s", res.HWAccel.Type)
	}

	encBytes, _ := os.ReadFile(filepath.Join(dataDir, "state", "services", "jellyfin", "config", "encoding.xml"))
	if !strings.Contains(string(encBytes), "<HardwareAccelerationType>qsv</HardwareAccelerationType>") {
		t.Errorf("expected encoding.xml to contain qsv, got: %s", string(encBytes))
	}
	if !strings.Contains(string(encBytes), "<VaapiDevice>/dev/dri/renderD128</VaapiDevice>") {
		t.Errorf("expected encoding.xml to contain renderD128 vaapi device, got: %s", string(encBytes))
	}
}

func TestJellyfin_HardwareDetection_AMDDRM(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")

	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte("mock-device"), 0644)

	// Vendor file for AMD (0x1002)
	vendorDir := filepath.Join(sysDir, "class", "drm", "renderD128", "device")
	_ = os.MkdirAll(vendorDir, 0755)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor"), []byte("0x1002\n"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelVAAPI {
		t.Errorf("expected HWAccelVAAPI for AMD vendor 0x1002, got %s", hw.Type)
	}
	if len(hw.Devices) != 1 || hw.Devices[0] != "/dev/dri/renderD128:/dev/dri/renderD128" {
		t.Errorf("expected devices [/dev/dri/renderD128:/dev/dri/renderD128], got %v", hw.Devices)
	}
}

func TestJellyfin_HardwareDetection_NvidiaDRM(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")

	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte("mock-device"), 0644)

	// Vendor file for Nvidia (0x10de) in DRM
	vendorDir := filepath.Join(sysDir, "class", "drm", "renderD128", "device")
	_ = os.MkdirAll(vendorDir, 0755)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor"), []byte("0x10de\n"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelNVENC {
		t.Errorf("expected HWAccelNVENC for Nvidia DRM vendor 0x10de, got %s", hw.Type)
	}
}

func TestJellyfin_HardwareDetection_EmptyDRI(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")

	// Empty dri folder without character nodes or vendor
	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelNone {
		t.Errorf("expected HWAccelNone for empty DRI directory, got %s", hw.Type)
	}
	if len(hw.Devices) != 0 {
		t.Errorf("expected empty devices slice for empty DRI, got %v", hw.Devices)
	}
}

func TestJellyfin_HardwareDetection_Nvidia(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	_ = os.MkdirAll(devDir, 0755)

	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte("mock-nvidia0"), 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidiactl"), []byte("mock-nvidiactl"), 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidia-uvm"), []byte("mock-uvm"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: filepath.Join(tempDir, "sys"),
		ProcRoot:  filepath.Join(tempDir, "proc"),
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelNVENC {
		t.Errorf("expected HWAccelNVENC, got %s", hw.Type)
	}
	if len(hw.Devices) == 0 {
		t.Fatalf("expected non-empty nvidia devices")
	}

	hasNvidia0 := false
	for _, d := range hw.Devices {
		if strings.Contains(d, "nvidia0") {
			hasNvidia0 = true
		}
	}
	if !hasNvidia0 {
		t.Errorf("expected nvidia0 in devices, got %v", hw.Devices)
	}
}

func TestJellyfin_HardwareDetection_None(t *testing.T) {
	tempDir := t.TempDir()
	opts := HardwareProbeOptions{
		DevRoot:   filepath.Join(tempDir, "dev"),
		SysfsRoot: filepath.Join(tempDir, "sys"),
		ProcRoot:  filepath.Join(tempDir, "proc"),
	}

	hw := DetectHardwareAcceleration(opts)
	if hw.Type != HWAccelNone {
		t.Errorf("expected HWAccelNone on system without acceleration, got %s", hw.Type)
	}
	if len(hw.Devices) != 0 {
		t.Errorf("expected nil or empty devices when acceleration absent, got %v", hw.Devices)
	}
	if hw.VaapiDevice != "" {
		t.Errorf("expected empty VaapiDevice, got %s", hw.VaapiDevice)
	}
	if !hw.CPUOptimized {
		t.Errorf("expected CPUOptimized to be true on system without acceleration")
	}
}

func TestJellyfin_HardwareDetection_HybridGPU(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")
	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)

	// Discrete NVIDIA GPU device nodes
	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte("mock-nvidia0"), 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidiactl"), []byte("mock-nvidiactl"), 0644)

	// Integrated DRI render node (Intel QSV 0x8086)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte("mock-renderD128"), 0644)
	vendorDir := filepath.Join(sysDir, "class", "drm", "renderD128", "device")
	_ = os.MkdirAll(vendorDir, 0755)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor"), []byte("0x8086\n"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
		ProcRoot:  filepath.Join(tempDir, "proc"),
	}

	if !IsHybridGPU(opts) {
		t.Fatalf("expected IsHybridGPU to return true for hybrid setup")
	}

	hw := DetectHardwareAcceleration(opts)
	if !hw.IsHybrid {
		t.Errorf("expected IsHybrid to be true")
	}
	// In hybrid mode, Jellyfin must be pinned to the iGPU (QSV/VAAPI), not NVENC
	if hw.Type != HWAccelQSV {
		t.Errorf("expected Jellyfin to be pinned to HWAccelQSV (iGPU) in hybrid mode, got %s", hw.Type)
	}
	if hw.VaapiDevice != filepath.Join(driDir, "renderD128") {
		t.Errorf("expected VaapiDevice to be iGPU %s, got %s", filepath.Join(driDir, "renderD128"), hw.VaapiDevice)
	}
}

func TestJellyfin_ComposeGeneration_PortsAndDevices(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	spec := ContainerSpec{
		Name:                   ServiceJellyfin,
		Image:                  ImageJellyfin,
		MemoryLimitBytes:       MemLimitJellyfin,
		MemoryReservationBytes: MemReservationJellyfin,
		Ports:                  map[int]int{8096: 8096},
		UDPPorts:               map[int]int{7359: 7359, 1900: 1900},
		Devices:                []string{"/dev/dri:/dev/dri"},
		Volumes: []string{
			filepath.Join(dataDir, "media/movies") + ":/data/media/movies",
			filepath.Join(dataDir, "media/tv") + ":/data/media/tv",
			filepath.Join(dataDir, "state/services/jellyfin/config") + ":/config",
			filepath.Join(dataDir, "state/services/jellyfin/cache") + ":/cache",
		},
		Networks: []string{"sovereign-ingress", "sovereign-control"},
	}

	composePath, err := SynthesizeComposeSpec(dataDir, spec)
	if err != nil {
		t.Fatalf("SynthesizeComposeSpec failed: %v", err)
	}

	contentBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read synthesized compose file: %v", err)
	}
	content := string(contentBytes)

	// Assert port mappings
	if !strings.Contains(content, "127.0.0.1:8096:8096") {
		t.Errorf("compose spec missing 127.0.0.1:8096:8096: %s", content)
	}
	if !strings.Contains(content, "7359:7359/udp") {
		t.Errorf("compose spec missing 7359:7359/udp auto-discovery port: %s", content)
	}
	if !strings.Contains(content, "1900:1900/udp") {
		t.Errorf("compose spec missing 1900:1900/udp DLNA port: %s", content)
	}

	// Assert devices
	if !strings.Contains(content, "/dev/dri:/dev/dri") {
		t.Errorf("compose spec missing /dev/dri:/dev/dri device: %s", content)
	}

	// Assert volume mounts
	if !strings.Contains(content, "/data/media/movies") {
		t.Errorf("compose spec missing movies volume mount: %s", content)
	}
	if !strings.Contains(content, "/data/media/tv") {
		t.Errorf("compose spec missing tv volume mount: %s", content)
	}
}

func TestJellyfin_TriggerLibraryRefresh(t *testing.T) {
	// 1. Success case with mock HTTP server
	var receivedPath string
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	prov := NewJellyfinProvisioner(t.TempDir(), nil, HardwareProbeOptions{})
	prov.SetBaseURL(ts.URL)

	if err := prov.TriggerLibraryRefresh(context.Background()); err != nil {
		t.Fatalf("TriggerLibraryRefresh failed: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedPath != "/Library/Refresh" {
		t.Errorf("expected path /Library/Refresh, got %s", receivedPath)
	}

	// 2. Error case with HTTP 500
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsErr.Close()

	provErr := NewJellyfinProvisioner(t.TempDir(), nil, HardwareProbeOptions{})
	provErr.SetBaseURL(tsErr.URL)

	if err := provErr.TriggerLibraryRefresh(context.Background()); err == nil {
		t.Fatalf("expected error on HTTP 500 response, got nil")
	}

	// 3. Custom RefreshFunc override
	calledCustom := false
	provCustom := NewJellyfinProvisioner(t.TempDir(), nil, HardwareProbeOptions{})
	provCustom.RefreshFunc = func(ctx context.Context) error {
		calledCustom = true
		return nil
	}

	if err := provCustom.TriggerLibraryRefresh(context.Background()); err != nil {
		t.Fatalf("unexpected error with custom RefreshFunc: %v", err)
	}
	if !calledCustom {
		t.Errorf("expected custom RefreshFunc to be invoked")
	}
}

func TestSupervisor_Jellyfin_RefreshService_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewMockRuntimeEngine()

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  engine,
	})
	defer mgr.Close()

	ctx := context.Background()

	// 1. Inactive service should fail with ErrServiceInactive
	err := mgr.RefreshService(ctx, ServiceJellyfin)
	if !errors.Is(err, ErrServiceInactive) {
		t.Errorf("expected ErrServiceInactive for inactive service, got: %v", err)
	}

	// 2. Nonexistent service should fail with ErrServiceNotFound
	errNotFound := mgr.RefreshService(ctx, "nonexistent")
	if !errors.Is(errNotFound, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound for nonexistent service, got: %v", errNotFound)
	}

	// 3. Activate Jellyfin
	_, err = mgr.ActivateService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("failed to activate Jellyfin: %v", err)
	}

	// Mock refresh dispatch
	refreshDispatched := false
	mgr.JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		refreshDispatched = true
		return nil
	}

	// 4. Refresh active Jellyfin service
	err = mgr.RefreshService(ctx, ServiceJellyfin)
	if err != nil {
		t.Fatalf("RefreshService failed on active service: %v", err)
	}
	if !refreshDispatched {
		t.Errorf("expected refresh to be dispatched to Jellyfin")
	}

	// 5. Test alias "video" resolves to Jellyfin
	aliasDispatched := false
	mgr.JellyfinProvisioner().RefreshFunc = func(ctx context.Context) error {
		aliasDispatched = true
		return nil
	}
	err = mgr.RefreshService(ctx, "video")
	if err != nil {
		t.Fatalf("RefreshService(\"video\") failed: %v", err)
	}
	if !aliasDispatched {
		t.Errorf("expected alias 'video' to dispatch refresh to Jellyfin")
	}

	// 6. Test unsupported service returns ErrInvalidOperation
	mgr.definitions["unsupported"] = ServiceDef{ID: "unsupported", Name: "Unsupported"}
	mgr.statuses["unsupported"] = &ServiceStatus{ID: "unsupported", Status: StatusRunning}
	errUnsupported := mgr.RefreshService(ctx, "unsupported")
	if !errors.Is(errUnsupported, ErrInvalidOperation) {
		t.Errorf("expected ErrInvalidOperation for unsupported refresh, got: %v", errUnsupported)
	}
}

func TestIsHybridGPU_NvidiaDRM_NotHybrid(t *testing.T) {
	tempDir := t.TempDir()
	devDir := filepath.Join(tempDir, "dev")
	sysDir := filepath.Join(tempDir, "sys")

	// Create NVIDIA character device
	_ = os.MkdirAll(devDir, 0755)
	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidiactl"), []byte{}, 0644)

	// Create DRI render node
	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte{}, 0644)

	// Set DRM vendor to NVIDIA (0x10de)
	vendorDir := filepath.Join(sysDir, "class", "drm", "renderD128", "device")
	_ = os.MkdirAll(vendorDir, 0755)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor"), []byte("0x10de\n"), 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: sysDir,
		ProcRoot:  filepath.Join(tempDir, "proc"),
	}

	// Because renderD128 is from NVIDIA (0x10de), this is a discrete NVIDIA GPU with DRM, NOT a hybrid setup
	if IsHybridGPU(opts) {
		t.Errorf("expected IsHybridGPU to return false when renderD128 belongs to Nvidia (0x10de)")
	}
}

