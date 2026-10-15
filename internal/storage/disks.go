package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Disk types
const (
	DiskTypeSATA    = "sata"
	DiskTypeNVMe    = "nvme"
	DiskTypeSCSI    = "scsi"
	DiskTypeVirtual = "virtual"
)

var (
	ErrInvalidDevice         = errors.New("invalid block device path")
	ErrSystemDeviceProtected = errors.New("device contains root or boot partitions and cannot be formatted")
	ErrDeviceInUse           = errors.New("device is currently in use or mounted")
	ErrFormattingFailed      = errors.New("failed to initialize ext4 filesystem with write-ahead journaling")
	ErrMountFailed           = errors.New("failed to mount device to target path")
)

// DiskInfo represents physical or virtual disk metadata.
type DiskInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Model      string `json:"model"`
	SizeBytes  int64  `json:"size_bytes"`
	Type       string `json:"type"`
	Rotational bool   `json:"rotational"`
	Mounted    bool   `json:"mounted"`
	MountPoint string `json:"mount_point,omitempty"`
	IsSystem   bool   `json:"is_system"`
}

// StorageLocalPool represents active local storage pool information.
type StorageLocalPool struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	Filesystem string `json:"filesystem"`
	Status     string `json:"status"` // "mounted", "unmounted", "error"
}

// DiskOps defines the platform operations required for disk detection and formatting.
type DiskOps interface {
	ListDisks(ctx context.Context) ([]DiskInfo, error)
	IsSystemDevice(device string) (bool, error)
	IsDeviceMounted(device string) (bool, string, error)
	FormatDeviceExt4(ctx context.Context, device string) error
	MountDevice(ctx context.Context, device, target string) error
	GetDiskUsage(path string) (total uint64, available uint64, used uint64, err error)
	IsSafeMode() bool
}

// DefaultDiskOps implements DiskOps on Linux platforms.
type DefaultDiskOps struct {
	logger   *slog.Logger
	safeMode bool
	// mockDisks allows injecting simulated disks in container/test environments
	mockDisks []DiskInfo
}

// NewDefaultDiskOps creates a DefaultDiskOps instance.
func NewDefaultDiskOps(logger *slog.Logger) *DefaultDiskOps {
	if logger == nil {
		logger = slog.Default()
	}

	safeMode := false
	if os.Geteuid() != 0 || os.Getenv("SOVEREIGN_STORAGE_MOCK") == "true" {
		safeMode = true
	}

	return &DefaultDiskOps{
		logger:   logger,
		safeMode: safeMode,
	}
}

// SetMockDisks configures mock disks for testing and safe virtual development.
func (d *DefaultDiskOps) SetMockDisks(disks []DiskInfo) {
	d.mockDisks = disks
}

// IsSafeMode reports whether the operations are running in non-privileged / mock mode.
func (d *DefaultDiskOps) IsSafeMode() bool {
	return d.safeMode
}

// SetSafeMode overrides safe mode flag (useful for testing).
func (d *DefaultDiskOps) SetSafeMode(safe bool) {
	d.safeMode = safe
}

// ListDisks enumerates block devices.
func (d *DefaultDiskOps) ListDisks(ctx context.Context) ([]DiskInfo, error) {
	if d.mockDisks != nil {
		return append([]DiskInfo(nil), d.mockDisks...), nil
	}

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		if os.IsNotExist(err) || d.safeMode {
			d.logger.Debug("cannot read /sys/block, returning empty disk list in safe mode", "error", err)
			return []DiskInfo{}, nil
		}
		return nil, fmt.Errorf("reading /sys/block failed: %w", err)
	}

	mounts, _ := d.readMounts()

	var disks []DiskInfo
	for _, entry := range entries {
		name := entry.Name()
		// Filter out non-disk block devices: loopback, ramdisks, optical (sr), device-mapper (dm)
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "sr") || strings.HasPrefix(name, "dm-") {
			continue
		}

		sysBlockPath := filepath.Join("/sys/block", name)

		// Size in 512-byte sectors
		sizeBytes := int64(0)
		if sizeContent, err := os.ReadFile(filepath.Join(sysBlockPath, "size")); err == nil {
			if sectors, err := strconv.ParseInt(strings.TrimSpace(string(sizeContent)), 10, 64); err == nil {
				sizeBytes = sectors * 512
			}
		}

		if sizeBytes == 0 {
			continue
		}

		// Rotational
		rotational := false
		if rotaContent, err := os.ReadFile(filepath.Join(sysBlockPath, "queue", "rotational")); err == nil {
			rotational = strings.TrimSpace(string(rotaContent)) == "1"
		}

		// Model
		model := ""
		if modelContent, err := os.ReadFile(filepath.Join(sysBlockPath, "device", "model")); err == nil {
			model = strings.TrimSpace(string(modelContent))
		}

		// Device type
		devType := DiskTypeSATA
		if strings.HasPrefix(name, "nvme") {
			devType = DiskTypeNVMe
		} else if strings.HasPrefix(name, "sd") {
			devType = DiskTypeSATA
		} else if strings.HasPrefix(name, "vd") {
			devType = DiskTypeVirtual
		}

		devPath := filepath.Join("/dev", name)

		// Check system and mounted state against /proc/mounts
		isSystem := false
		mounted := false
		mountPoint := ""

		for _, m := range mounts {
			if isDeviceOrPartition(m.Device, devPath) {
				mounted = true
				if mountPoint == "" {
					mountPoint = m.MountPoint
				}
				if m.MountPoint == "/" || m.MountPoint == "/boot" || strings.HasPrefix(m.MountPoint, "/boot/") {
					isSystem = true
				}
			}
		}

		disks = append(disks, DiskInfo{
			Name:       name,
			Path:       devPath,
			Model:      model,
			SizeBytes:  sizeBytes,
			Type:       devType,
			Rotational: rotational,
			Mounted:    mounted,
			MountPoint: mountPoint,
			IsSystem:   isSystem,
		})
	}

	return disks, nil
}

type mountEntry struct {
	Device     string
	MountPoint string
	FSType     string
}

func (d *DefaultDiskOps) readMounts() ([]mountEntry, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []mountEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			entries = append(entries, mountEntry{
				Device:     fields[0],
				MountPoint: fields[1],
				FSType:     fields[2],
			})
		}
	}
	return entries, scanner.Err()
}

// isDeviceOrPartition checks if candidate is exact device or a partition of diskDev.
// Matches e.g. /dev/sda1 or /dev/sda2 for /dev/sda, and /dev/nvme0n1p1 for /dev/nvme0n1,
// while correctly rejecting false prefixes such as /dev/sdab for /dev/sda.
func isDeviceOrPartition(candidate, diskDev string) bool {
	if candidate == diskDev {
		return true
	}
	if !strings.HasPrefix(candidate, diskDev) {
		return false
	}
	suffix := candidate[len(diskDev):]
	if strings.HasPrefix(suffix, "p") && len(suffix) > 1 {
		suffix = suffix[1:]
	}
	if len(suffix) == 0 {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// IsSystemDevice checks if the device or any of its partitions are mounted as root or boot.
func (d *DefaultDiskOps) IsSystemDevice(device string) (bool, error) {
	// If mock disks are active, check mock disk state
	for _, md := range d.mockDisks {
		if isDeviceOrPartition(device, md.Path) || isDeviceOrPartition(md.Path, device) {
			return md.IsSystem, nil
		}
	}

	mounts, err := d.readMounts()
	if err != nil {
		if os.IsNotExist(err) && d.safeMode {
			return false, nil
		}
		return false, fmt.Errorf("reading /proc/mounts failed: %w", err)
	}

	for _, m := range mounts {
		if isDeviceOrPartition(m.Device, device) {
			if m.MountPoint == "/" || m.MountPoint == "/boot" || strings.HasPrefix(m.MountPoint, "/boot/") {
				return true, nil
			}
		}
	}

	return false, nil
}

// IsDeviceMounted checks whether the device is currently mounted anywhere.
func (d *DefaultDiskOps) IsDeviceMounted(device string) (bool, string, error) {
	for _, md := range d.mockDisks {
		if isDeviceOrPartition(device, md.Path) || isDeviceOrPartition(md.Path, device) {
			return md.Mounted, md.MountPoint, nil
		}
	}

	mounts, err := d.readMounts()
	if err != nil {
		if os.IsNotExist(err) && d.safeMode {
			return false, "", nil
		}
		return false, "", fmt.Errorf("reading /proc/mounts failed: %w", err)
	}

	for _, m := range mounts {
		if isDeviceOrPartition(m.Device, device) {
			return true, m.MountPoint, nil
		}
	}

	return false, "", nil
}

// FormatDeviceExt4 formats the target disk with ext4 write-ahead journaling.
func (d *DefaultDiskOps) FormatDeviceExt4(ctx context.Context, device string) error {
	d.logger.Info("formatting block device with ext4 write-ahead journaling", "device", device, "safe_mode", d.safeMode)

	if d.safeMode {
		// In safe/virtual mode, simulate formatting without invoking raw mkfs.ext4 on host block devices
		return nil
	}

	cmd := exec.CommandContext(ctx, "mkfs.ext4", "-F", "-O", "has_journal", device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("mkfs.ext4 execution failed", "device", device, "error", err, "output", string(out))
		return ErrFormattingFailed
	}

	return nil
}

// MountDevice mounts the formatted block device to the specified target directory.
func (d *DefaultDiskOps) MountDevice(ctx context.Context, device, target string) error {
	d.logger.Info("mounting block device to target directory", "device", device, "target", target, "safe_mode", d.safeMode)

	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("failed to create target mount directory %q: %w", target, err)
	}

	if d.safeMode {
		// In safe mode, write metadata marker acknowledging virtual mount
		markerPath := filepath.Join(target, ".sovereign_pool")
		markerContent := fmt.Sprintf(`{"device":"%s","filesystem":"ext4","mode":"safe_virtual"}`, device)
		_ = os.WriteFile(markerPath, []byte(markerContent), 0644)
		return nil
	}

	cmd := exec.CommandContext(ctx, "mount", "-t", "ext4", device, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("mount execution failed", "device", device, "target", target, "error", err, "output", string(out))
		return ErrMountFailed
	}

	return nil
}

// GetDiskUsage retrieves filesystem usage for the specified path using statfs.
func (d *DefaultDiskOps) GetDiskUsage(path string) (total uint64, available uint64, used uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("statfs failed for %q: %w", path, err)
	}

	total = stat.Blocks * uint64(stat.Bsize)
	available = stat.Bavail * uint64(stat.Bsize)
	used = total - available
	return total, available, used, nil
}
