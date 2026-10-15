package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Hardware acceleration types for Jellyfin
const (
	HWAccelNone  = "none"
	HWAccelQSV   = "qsv"
	HWAccelVAAPI = "vaapi"
	HWAccelNVENC = "nvenc"
)

// HardwareProbeOptions specifies root paths for inspecting hardware devices.
type HardwareProbeOptions struct {
	DevRoot   string // Defaults to "/dev"
	SysfsRoot string // Defaults to "/sys"
	ProcRoot  string // Defaults to "/proc"
}

// HardwareDetectResult describes detected hardware acceleration capabilities.
type HardwareDetectResult struct {
	Type        string   // HWAccelNone, HWAccelQSV, HWAccelVAAPI, HWAccelNVENC
	Devices     []string // e.g. ["/dev/dri:/dev/dri"] or ["/dev/nvidia0:/dev/nvidia0", ...]
	VaapiDevice string   // e.g. "/dev/dri/renderD128"
}

// JellyfinProvisionResult contains provisioning parameters for starting Jellyfin.
type JellyfinProvisionResult struct {
	HWAccel  HardwareDetectResult
	Devices  []string
	UDPPorts map[int]int
}

// JellyfinProvisioner prepares storage paths, direct-play configuration, hardware acceleration,
// and handles library refresh dispatches for the Jellyfin video streaming container.
type JellyfinProvisioner struct {
	dataDir     string
	logger      *slog.Logger
	hwOpts      HardwareProbeOptions
	httpClient  *http.Client
	baseURL     string
	RefreshFunc func(ctx context.Context) error
}

// NewJellyfinProvisioner initializes a new provisioner for Jellyfin.
func NewJellyfinProvisioner(dataDir string, logger *slog.Logger, hwOpts HardwareProbeOptions) *JellyfinProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}
	if hwOpts.DevRoot == "" {
		hwOpts.DevRoot = "/dev"
	}
	if hwOpts.SysfsRoot == "" {
		hwOpts.SysfsRoot = "/sys"
	}
	if hwOpts.ProcRoot == "" {
		hwOpts.ProcRoot = "/proc"
	}

	return &JellyfinProvisioner{
		dataDir:    dataDir,
		logger:     logger,
		hwOpts:     hwOpts,
		baseURL:    "http://127.0.0.1:8096",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// SetBaseURL overrides the Jellyfin HTTP base URL (used for refresh webhook dispatch).
func (p *JellyfinProvisioner) SetBaseURL(url string) {
	p.baseURL = strings.TrimRight(url, "/")
}

// SetHTTPClient overrides the HTTP client.
func (p *JellyfinProvisioner) SetHTTPClient(client *http.Client) {
	p.httpClient = client
}

// DetectHardwareAcceleration inspects host devices to detect Intel QSV, AMD VAAPI, or Nvidia NVENC.
func DetectHardwareAcceleration(opts HardwareProbeOptions) HardwareDetectResult {
	if opts.DevRoot == "" {
		opts.DevRoot = "/dev"
	}
	if opts.SysfsRoot == "" {
		opts.SysfsRoot = "/sys"
	}
	if opts.ProcRoot == "" {
		opts.ProcRoot = "/proc"
	}

	// 1. Probe for DRI devices (/dev/dri or /dev/dri/renderD128)
	driPath := filepath.Join(opts.DevRoot, "dri")
	renderPath := filepath.Join(driPath, "renderD128")
	cardPath := filepath.Join(driPath, "card0")

	driExists := false
	if fi, err := os.Stat(driPath); err == nil && fi.IsDir() {
		driExists = true
	} else if _, err := os.Stat(renderPath); err == nil {
		driExists = true
	}

	if driExists {
		vaapiDev := "/dev/dri/renderD128"
		if _, err := os.Stat(renderPath); err != nil {
			if _, err := os.Stat(cardPath); err == nil {
				vaapiDev = "/dev/dri/card0"
			}
		}

		// Inspect sysfs for Intel vs AMD vendor ID
		// Intel: 0x8086, AMD: 0x1002
		vendor := probeDRMVendor(opts.SysfsRoot)
		accelType := HWAccelVAAPI
		if strings.Contains(vendor, "8086") {
			accelType = HWAccelQSV
		} else if strings.Contains(vendor, "1002") {
			accelType = HWAccelVAAPI
		}

		return HardwareDetectResult{
			Type:        accelType,
			Devices:     []string{"/dev/dri:/dev/dri"},
			VaapiDevice: vaapiDev,
		}
	}

	// 2. Probe for Nvidia devices (/dev/nvidia0, /dev/nvidiactl, or /proc/driver/nvidia)
	nvidia0 := filepath.Join(opts.DevRoot, "nvidia0")
	nvidiactl := filepath.Join(opts.DevRoot, "nvidiactl")
	nvidiaProc := filepath.Join(opts.ProcRoot, "driver/nvidia/version")

	nvidiaFound := false
	if _, err := os.Stat(nvidia0); err == nil {
		nvidiaFound = true
	} else if _, err := os.Stat(nvidiactl); err == nil {
		nvidiaFound = true
	} else if _, err := os.Stat(nvidiaProc); err == nil {
		nvidiaFound = true
	}

	if nvidiaFound {
		var nvidiaDevices []string
		candidates := []string{"nvidia0", "nvidiactl", "nvidia-uvm", "nvidia-modeset"}
		for _, c := range candidates {
			devPath := filepath.Join(opts.DevRoot, c)
			if _, err := os.Stat(devPath); err == nil {
				nvidiaDevices = append(nvidiaDevices, fmt.Sprintf("%s:%s", devPath, devPath))
			}
		}
		if len(nvidiaDevices) == 0 {
			nvidiaDevices = []string{"/dev/nvidia0:/dev/nvidia0"}
		}
		sort.Strings(nvidiaDevices)

		return HardwareDetectResult{
			Type:        HWAccelNVENC,
			Devices:     nvidiaDevices,
			VaapiDevice: "",
		}
	}

	// 3. No hardware acceleration devices detected
	return HardwareDetectResult{
		Type:        HWAccelNone,
		Devices:     nil,
		VaapiDevice: "",
	}
}

func probeDRMVendor(sysfsRoot string) string {
	vendorPaths := []string{
		filepath.Join(sysfsRoot, "class/drm/renderD128/device/vendor"),
		filepath.Join(sysfsRoot, "class/drm/card0/device/vendor"),
	}

	for _, p := range vendorPaths {
		if data, err := os.ReadFile(p); err == nil {
			return strings.ToLower(strings.TrimSpace(string(data)))
		}
	}
	return ""
}

// Provision prepares library paths, direct-play XML configs, and hardware devices.
func (p *JellyfinProvisioner) Provision(ctx context.Context) (*JellyfinProvisionResult, error) {
	// 1. Ensure required library paths exist with 0755 permissions
	// Never delete or alter existing media files.
	moviesPath := filepath.Join(p.dataDir, "media", "movies")
	tvPath := filepath.Join(p.dataDir, "media", "tv")
	configPath := filepath.Join(p.dataDir, "state", "services", "jellyfin", "config")
	cachePath := filepath.Join(p.dataDir, "state", "services", "jellyfin", "cache")

	for _, dir := range []string{moviesPath, tvPath, configPath, cachePath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// 2. Hardware acceleration detection
	hw := DetectHardwareAcceleration(p.hwOpts)
	p.logger.Info("detected hardware acceleration for jellyfin", "type", hw.Type, "devices", hw.Devices)

	// 3. Ensure direct-play and system XML configurations exist without overwriting existing files
	systemXMLPath := filepath.Join(configPath, "system.xml")
	if _, err := os.Stat(systemXMLPath); os.IsNotExist(err) {
		if err := writeAtomicXML(systemXMLPath, generateSystemXML()); err != nil {
			return nil, fmt.Errorf("failed to write system.xml: %w", err)
		}
		p.logger.Info("generated default direct-play system.xml configuration")
	} else {
		p.logger.Debug("existing system.xml found; preserving configuration")
	}

	encodingXMLPath := filepath.Join(configPath, "encoding.xml")
	if _, err := os.Stat(encodingXMLPath); os.IsNotExist(err) {
		encodingContent := generateEncodingXML(hw.Type, hw.VaapiDevice)
		if err := writeAtomicXML(encodingXMLPath, encodingContent); err != nil {
			return nil, fmt.Errorf("failed to write encoding.xml: %w", err)
		}
		p.logger.Info("generated direct-play prioritized encoding.xml configuration", "hwaccel", hw.Type)
	} else {
		p.logger.Debug("existing encoding.xml found; preserving configuration")
	}

	udpPorts := map[int]int{
		7359: 7359, // Jellyfin Native Auto-Discovery
		1900: 1900, // DLNA Discovery
	}

	return &JellyfinProvisionResult{
		HWAccel:  hw,
		Devices:  hw.Devices,
		UDPPorts: udpPorts,
	}, nil
}

// TriggerLibraryRefresh dispatches an HTTP request to Jellyfin's /Library/Refresh endpoint.
func (p *JellyfinProvisioner) TriggerLibraryRefresh(ctx context.Context) error {
	if p.RefreshFunc != nil {
		return p.RefreshFunc(ctx)
	}

	refreshURL := fmt.Sprintf("%s/Library/Refresh", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to dispatch jellyfin library refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("jellyfin library refresh responded with status %d", resp.StatusCode)
	}

	p.logger.Info("jellyfin library refresh dispatched successfully", "url", refreshURL)
	return nil
}

func generateSystemXML() []byte {
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	buf.WriteString("<ServerConfiguration xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\">\n")
	buf.WriteString("  <ServerName>Sovereign</ServerName>\n")
	buf.WriteString("  <EnableUPnP>true</EnableUPnP>\n")
	buf.WriteString("  <EnableRemoteAccess>true</EnableRemoteAccess>\n")
	buf.WriteString("  <Port>8096</Port>\n")
	buf.WriteString("  <PublicPort>8096</PublicPort>\n")
	buf.WriteString("  <IsStartupWizardCompleted>true</IsStartupWizardCompleted>\n")
	buf.WriteString("</ServerConfiguration>\n")
	return buf.Bytes()
}

func generateEncodingXML(hwType, vaapiDevice string) []byte {
	if hwType == "" {
		hwType = HWAccelNone
	}

	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	buf.WriteString("<EncodingOptions xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\">\n")
	buf.WriteString("  <EncodingThreadCount>-1</EncodingThreadCount>\n")
	buf.WriteString("  <TranscodingTempPath>/cache</TranscodingTempPath>\n")
	buf.WriteString("  <EnableAudioVbr>true</EnableAudioVbr>\n")
	buf.WriteString("  <EnableDirectPlay>true</EnableDirectPlay>\n")
	buf.WriteString("  <EnableDirectStream>true</EnableDirectStream>\n")
	buf.WriteString("  <EnableHardwareEncoding>true</EnableHardwareEncoding>\n")
	buf.WriteString(fmt.Sprintf("  <HardwareAccelerationType>%s</HardwareAccelerationType>\n", hwType))
	if vaapiDevice != "" {
		buf.WriteString(fmt.Sprintf("  <VaapiDevice>%s</VaapiDevice>\n", vaapiDevice))
	} else {
		buf.WriteString("  <VaapiDevice></VaapiDevice>\n")
	}
	buf.WriteString("</EncodingOptions>\n")
	return buf.Bytes()
}

func writeAtomicXML(destPath string, content []byte) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := destPath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}
