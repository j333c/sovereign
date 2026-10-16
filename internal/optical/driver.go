package optical

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DiscType classifies the optical media type.
type DiscType string

const (
	DiscTypeNone    DiscType = "none"
	DiscTypeCD      DiscType = "cd"
	DiscTypeDVD     DiscType = "dvd"
	DiscTypeBluray  DiscType = "bluray"
	DiscTypeUnknown DiscType = "unknown"
)

// DiscInfo contains metadata regarding an inserted optical disc.
type DiscInfo struct {
	Inserted  bool     `json:"inserted"`
	DiscType  DiscType `json:"disc_type"`
	Label     string   `json:"label"`
	ID        string   `json:"id"`
	Encrypted bool     `json:"encrypted"`
	SizeBytes int64    `json:"size_bytes"`
}

// DriveStatus contains the current hardware status of the optical drive.
type DriveStatus struct {
	DevicePath string   `json:"device_path"`
	DriveReady bool     `json:"drive_ready"`
	TrayOpen   bool     `json:"tray_open"`
	Disc       DiscInfo `json:"disc"`
}

// OpticalDriver defines the hardware operations interface for optical drive management and disc reading.
type OpticalDriver interface {
	GetDriveStatus(ctx context.Context) (DriveStatus, error)
	Eject(ctx context.Context) error
	CloseTray(ctx context.Context) error
	RipDisc(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error
}

// Driver is an alias for OpticalDriver.
type Driver = OpticalDriver

var srDeviceRegex = regexp.MustCompile(`^/dev/sr[0-9]+$`)

// LinuxOpticalDriver implements OpticalDriver using standard Linux block interfaces and safe CLI tools.
type LinuxOpticalDriver struct {
	devicePath string
	logger     *slog.Logger
	mu         sync.Mutex
	trayOpen   bool
}

// NewLinuxOpticalDriver creates a LinuxOpticalDriver for the specified device path (e.g. /dev/sr0).
func NewLinuxOpticalDriver(devicePath string, logger *slog.Logger) *LinuxOpticalDriver {
	if devicePath == "" {
		devicePath = "/dev/sr0"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LinuxOpticalDriver{
		devicePath: devicePath,
		logger:     logger,
	}
}

// GetDriveStatus checks hardware optical drive availability and queries disc presence and metadata.
func (d *LinuxOpticalDriver) GetDriveStatus(ctx context.Context) (DriveStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Boundary validation: verify device path is a standard optical block device.
	if !srDeviceRegex.MatchString(d.devicePath) {
		return DriveStatus{
			DevicePath: d.devicePath,
			DriveReady: false,
		}, fmt.Errorf("invalid optical block device path: %q", d.devicePath)
	}

	if _, err := os.Stat(d.devicePath); err != nil {
		// Drive hardware not attached or not accessible
		return DriveStatus{
			DevicePath: d.devicePath,
			DriveReady: false,
			TrayOpen:   d.trayOpen,
		}, nil
	}

	status := DriveStatus{
		DevicePath: d.devicePath,
		DriveReady: true,
		TrayOpen:   d.trayOpen,
	}

	// Probe disc presence using blkid with boundary protection
	cmd := exec.CommandContext(ctx, "blkid", "-c", "/dev/null", "-o", "export", d.devicePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// No readable media or tray empty
		status.Disc = DiscInfo{
			Inserted: false,
			DiscType: DiscTypeNone,
		}
		return status, nil
	}

	disc := parseBlkidOutput(string(out))
	disc.Inserted = true
	d.trayOpen = false
	status.TrayOpen = false
	status.Disc = disc

	return status, nil
}

// parseBlkidOutput parses the exported key-value pairs from `blkid -o export`.
func parseBlkidOutput(output string) DiscInfo {
	disc := DiscInfo{
		DiscType: DiscTypeUnknown,
	}
	var fsType string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "LABEL=") {
			disc.Label = strings.TrimPrefix(line, "LABEL=")
		} else if strings.HasPrefix(line, "UUID=") {
			disc.ID = strings.TrimPrefix(line, "UUID=")
		} else if strings.HasPrefix(line, "TYPE=") {
			fsType = strings.ToLower(strings.TrimPrefix(line, "TYPE="))
		}
	}

	switch fsType {
	case "iso9660":
		disc.DiscType = DiscTypeDVD
	case "udf":
		disc.DiscType = DiscTypeBluray
	default:
		if disc.Label != "" {
			disc.DiscType = DiscTypeDVD
		} else {
			disc.DiscType = DiscTypeCD
		}
	}

	if disc.ID == "" {
		disc.ID = strings.ToLower(strings.ReplaceAll(disc.Label, " ", "_"))
	}

	return disc
}

// Eject sends the hardware eject signal to the optical drive.
func (d *LinuxOpticalDriver) Eject(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !srDeviceRegex.MatchString(d.devicePath) {
		return fmt.Errorf("invalid optical block device path: %q", d.devicePath)
	}

	d.logger.Info("ejecting optical drive tray", "device", d.devicePath)
	cmd := exec.CommandContext(ctx, "eject", d.devicePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.logger.Warn("eject command returned error", "device", d.devicePath, "error", err, "output", string(out))
	}
	d.trayOpen = true
	return nil
}

// CloseTray attempts to close the hardware optical drive tray.
func (d *LinuxOpticalDriver) CloseTray(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !srDeviceRegex.MatchString(d.devicePath) {
		return fmt.Errorf("invalid optical block device path: %q", d.devicePath)
	}

	d.logger.Info("closing optical drive tray", "device", d.devicePath)
	cmd := exec.CommandContext(ctx, "eject", "-t", d.devicePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.logger.Warn("eject -t command returned error", "device", d.devicePath, "error", err, "output", string(out))
	}
	d.trayOpen = false
	return nil
}

// RipDisc performs reading from the physical optical device to the staging directory.
func (d *LinuxOpticalDriver) RipDisc(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error {
	if !srDeviceRegex.MatchString(d.devicePath) {
		return fmt.Errorf("invalid optical block device path: %q", d.devicePath)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}

	destFile := filepath.Join(destDir, "disc.iso")
	d.logger.Info("starting optical disc rip to staging", "device", d.devicePath, "dest", destFile)

	// Stream read with bounded buffer and cancellation support
	in, err := os.Open(d.devicePath)
	if err != nil {
		return fmt.Errorf("failed to open optical device for reading: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open staging destination file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 1024*1024) // 1MB buffer
	var totalRead int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, rErr := in.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("write error during disc ripping: %w", wErr)
			}
			totalRead += int64(n)
			if disc.SizeBytes > 0 && progressCb != nil {
				pct := float64(totalRead) / float64(disc.SizeBytes) * 100.0
				if pct > 100.0 {
					pct = 100.0
				}
				progressCb(pct)
			}
		}
		if rErr != nil {
			if errors.Is(rErr, os.ErrClosed) || rErr.Error() == "EOF" {
				break
			}
			return fmt.Errorf("read error from optical device: %w", rErr)
		}
	}

	if progressCb != nil {
		progressCb(100.0)
	}
	return nil
}

// MockOpticalDriver simulates an optical drive for deterministic testing without physical hardware.
type MockOpticalDriver struct {
	mu          sync.RWMutex
	status      DriveStatus
	ejectCount  int
	closeCount  int
	ripFailures int
	ripErr      error
	ripDelay    time.Duration
	onRip       func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error
	onEject     func(ctx context.Context) error
}

// NewMockOpticalDriver initializes a mock optical drive in ready, tray-closed, empty state.
func NewMockOpticalDriver() *MockOpticalDriver {
	return &MockOpticalDriver{
		status: DriveStatus{
			DevicePath: "/dev/sr0",
			DriveReady: true,
			TrayOpen:   false,
			Disc: DiscInfo{
				Inserted: false,
				DiscType: DiscTypeNone,
			},
		},
	}
}

// GetDriveStatus returns the simulated drive status.
func (m *MockOpticalDriver) GetDriveStatus(ctx context.Context) (DriveStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, nil
}

// SetDriveStatus sets the entire simulated drive status.
func (m *MockOpticalDriver) SetDriveStatus(st DriveStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = st
}

// InsertDisc simulates disc insertion into the tray.
func (m *MockOpticalDriver) InsertDisc(disc DiscInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	disc.Inserted = true
	m.status.Disc = disc
	m.status.TrayOpen = false
	m.status.DriveReady = true
}

// RemoveDisc clears any inserted disc.
func (m *MockOpticalDriver) RemoveDisc() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Disc = DiscInfo{
		Inserted: false,
		DiscType: DiscTypeNone,
	}
}

// Eject simulates tray opening and disc ejection.
func (m *MockOpticalDriver) Eject(ctx context.Context) error {
	m.mu.Lock()
	m.ejectCount++
	m.status.TrayOpen = true
	m.status.Disc = DiscInfo{Inserted: false, DiscType: DiscTypeNone}
	onEject := m.onEject
	m.mu.Unlock()

	if onEject != nil {
		return onEject(ctx)
	}
	return nil
}

// CloseTray simulates closing the drive tray.
func (m *MockOpticalDriver) CloseTray(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCount++
	m.status.TrayOpen = false
	return nil
}

// EjectCount returns the number of times Eject was called.
func (m *MockOpticalDriver) EjectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ejectCount
}

// SetRipFailures configures how many consecutive times RipDisc will return an error before succeeding.
func (m *MockOpticalDriver) SetRipFailures(count int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ripFailures = count
	m.ripErr = err
}

// SetRipDelay sets an optional simulated delay per rip.
func (m *MockOpticalDriver) SetRipDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ripDelay = delay
}

// SetOnRip sets a custom rip execution hook.
func (m *MockOpticalDriver) SetOnRip(fn func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRip = fn
}

// SetOnEject sets a custom eject execution hook.
func (m *MockOpticalDriver) SetOnEject(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEject = fn
}

// RipDisc performs simulated ripping with progress callbacks and retry simulation.
func (m *MockOpticalDriver) RipDisc(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error {
	m.mu.Lock()
	if m.ripFailures > 0 {
		m.ripFailures--
		err := m.ripErr
		if err == nil {
			err = errors.New("read scratch error at sector 1024")
		}
		m.mu.Unlock()
		return err
	}
	customRip := m.onRip
	delay := m.ripDelay
	m.mu.Unlock()

	if customRip != nil {
		return customRip(ctx, disc, destDir, progressCb)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	steps := 5
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if delay > 0 {
			time.Sleep(delay / time.Duration(steps))
		}
		if progressCb != nil {
			progressCb(float64(i) * 100.0 / float64(steps))
		}
	}

	fileName := "disc.iso"
	if disc.DiscType == DiscTypeCD {
		fileName = "track01.cdda"
	}
	content := []byte(fmt.Sprintf("MOCK RIP: %s (Type: %s, ID: %s)", disc.Label, disc.DiscType, disc.ID))
	return os.WriteFile(filepath.Join(destDir, fileName), content, 0644)
}
