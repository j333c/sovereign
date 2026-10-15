// Package storage manages the single POSIX filesystem hierarchy under /data,
// staged file dropzones, atomic media moves, and media library synchronization.
//
// Invariants & Boundaries:
// - Storage Hierarchy: Root partition under /data with dedicated subdirectories:
//   /data/media, /data/documents, /data/models, /data/state, /data/drop.
// - Dropzone Staging: Uploads stream to /data/drop/incoming/:category/.uploading-:uuid-:filename.
// - Atomic Promotion: Upon complete upload and fsync(), files are validated for magic bytes
//   and atomically moved (os.Rename) into /data/media/:category.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sovereign/internal/store"
)

// RequiredDataSubdirectories defines the mandatory subdirectories of the /data hierarchy.
var RequiredDataSubdirectories = []string{
	"media",
	"documents",
	"models",
	"state",
	"drop",
}

// EnsureDataLayout verifies and recursively creates missing subdirectories under the given data root.
func EnsureDataLayout(dataRoot string) error {
	if dataRoot == "" {
		return fmt.Errorf("data root directory path cannot be empty")
	}

	for _, sub := range RequiredDataSubdirectories {
		dir := filepath.Join(dataRoot, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory %q: %w", dir, err)
		}
	}

	return nil
}

// EngineConfig holds configuration for the Storage Engine.
type EngineConfig struct {
	DataDir string
	DB      *store.DB
	Logger  *slog.Logger
	DiskOps DiskOps
	NASOps  NASOps
}

// Engine coordinates local block device detection, ext4 formatting,
// network NAS mount management, and dynamic health probing.
type Engine struct {
	dataDir string
	db      *store.DB
	logger  *slog.Logger
	diskOps DiskOps
	nasOps  NASOps
	probe   *StorageProbe
	mu      sync.RWMutex
}

// NewEngine creates and initializes a Storage Engine.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.DiskOps == nil {
		cfg.DiskOps = NewDefaultDiskOps(cfg.Logger)
	}
	if cfg.NASOps == nil {
		cfg.NASOps = NewDefaultNASOps(cfg.Logger)
	}

	probe := NewStorageProbe(cfg.DataDir, cfg.DB, cfg.DiskOps, cfg.NASOps, cfg.Logger)

	engine := &Engine{
		dataDir: cfg.DataDir,
		db:      cfg.DB,
		logger:  cfg.Logger,
		diskOps: cfg.DiskOps,
		nasOps:  cfg.NASOps,
		probe:   probe,
	}

	// Restore any existing local pool state from database
	if cfg.DB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if val, err := cfg.DB.GetSetting(ctx, SettingsKeyLocalPool); err == nil && val != "" {
			var localPool StorageLocalPool
			if err := json.Unmarshal([]byte(val), &localPool); err == nil {
				probe.SetActiveLocalPool(&localPool)
			}
		}
	}

	return engine, nil
}

// ListAvailableDisks returns unmounted SATA/NVMe disks suitable for formatting.
func (e *Engine) ListAvailableDisks(ctx context.Context) ([]DiskInfo, error) {
	disks, err := e.diskOps.ListDisks(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing block devices failed: %w", err)
	}

	var available []DiskInfo
	for _, d := range disks {
		// Only include disks that are NOT mounted and NOT system disks
		if !d.Mounted && !d.IsSystem {
			available = append(available, d)
		}
	}

	if available == nil {
		available = []DiskInfo{}
	}

	return available, nil
}

// StoragePoolInfo represents the response after formatting and mounting a storage pool.
type StoragePoolInfo struct {
	Status     string `json:"status"`
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	Message    string `json:"message"`
}

// InitializeStorage partitions, formats disk with ext4 write-ahead journaling,
// mounts it under /data, and initializes subdirectories.
func (e *Engine) InitializeStorage(ctx context.Context, device string) (*StoragePoolInfo, error) {
	device = strings.TrimSpace(device)
	if device == "" || !strings.HasPrefix(device, "/dev/") {
		return nil, ErrInvalidDevice
	}

	// Boundary check: Never format root or boot partitions
	isSys, err := e.diskOps.IsSystemDevice(device)
	if err != nil {
		return nil, fmt.Errorf("failed to check device status: %w", err)
	}
	if isSys {
		return nil, ErrSystemDeviceProtected
	}

	// Boundary check: Device must not be in use/mounted
	mounted, mountPt, err := e.diskOps.IsDeviceMounted(device)
	if err != nil {
		return nil, fmt.Errorf("failed to check mount status: %w", err)
	}
	if mounted {
		e.logger.Warn("cannot initialize device: currently mounted", "device", device, "mount_point", mountPt)
		return nil, ErrDeviceInUse
	}

	// Format with ext4 write-ahead journaling
	if err := e.diskOps.FormatDeviceExt4(ctx, device); err != nil {
		return nil, err
	}

	// Mount under unified data directory
	if err := e.diskOps.MountDevice(ctx, device, e.dataDir); err != nil {
		return nil, err
	}

	// Initialize standard data subdirectories: media, documents, models, state, drop
	if err := EnsureDataLayout(e.dataDir); err != nil {
		return nil, fmt.Errorf("initializing unified layout failed: %w", err)
	}

	localPool := &StorageLocalPool{
		Device:     device,
		MountPoint: e.dataDir,
		Filesystem: "ext4",
		Status:     "mounted",
	}

	// Persist local pool config in SQLite settings
	if e.db != nil {
		data, _ := json.Marshal(localPool)
		_ = e.db.SetSetting(ctx, SettingsKeyLocalPool, string(data))
	}

	e.probe.SetActiveLocalPool(localPool)
	e.probe.ProbeOnce(ctx)

	return &StoragePoolInfo{
		Status:     "initialized",
		Device:     device,
		MountPoint: e.dataDir,
		Message:    "Storage pool formatted and mounted successfully",
	}, nil
}

// TestNAS performs non-destructive verification of credentials and network reachability.
func (e *Engine) TestNAS(ctx context.Context, req NASTestRequest) (*NASTestResponse, error) {
	if err := e.nasOps.TestConnection(ctx, req); err != nil {
		return nil, err
	}

	return &NASTestResponse{
		Success: true,
		Message: "Connection and credentials verified successfully",
	}, nil
}

// MountNAS mounts a verified network share and persists configuration in SQLite.
func (e *Engine) MountNAS(ctx context.Context, req NASMountRequest) (*NASMountResponse, error) {
	// First perform non-destructive verification
	testReq := NASTestRequest{
		Type:     req.Type,
		Host:     req.Host,
		Share:    req.Share,
		Username: req.Username,
		Password: req.Password,
		Options:  req.Options,
	}
	if err := e.nasOps.TestConnection(ctx, testReq); err != nil {
		return nil, err
	}

	targetPath := req.TargetPath
	if strings.TrimSpace(targetPath) == "" {
		targetPath = filepath.Join(e.dataDir, "media")
	}

	cfg := NASConfig{
		Type:       strings.ToLower(strings.TrimSpace(req.Type)),
		Host:       strings.TrimSpace(req.Host),
		Share:      strings.TrimSpace(req.Share),
		Username:   req.Username,
		Password:   req.Password,
		Options:    req.Options,
		MountPoint: targetPath,
	}

	if err := e.nasOps.MountShare(ctx, cfg); err != nil {
		return nil, err
	}

	if e.db != nil {
		if err := SaveNASConfig(ctx, e.db, cfg); err != nil {
			e.logger.Error("failed to persist NAS config to SQLite", "error", err)
			return nil, fmt.Errorf("persisting network storage configuration failed: %w", err)
		}
	}

	e.probe.SetActiveNASConfig(&cfg)
	e.probe.ProbeOnce(ctx)

	return &NASMountResponse{
		Status:     "mounted",
		Type:       cfg.Type,
		Host:       cfg.Host,
		Share:      cfg.Share,
		MountPoint: cfg.MountPoint,
		Message:    "Network storage share mounted successfully",
	}, nil
}

// GetStorageStatus returns the current storage telemetry snapshot.
func (e *Engine) GetStorageStatus(ctx context.Context) StorageStatusResponse {
	return e.probe.ProbeOnce(ctx)
}

// StartProbing starts the background health monitoring loop.
func (e *Engine) StartProbing(ctx context.Context) {
	e.probe.Start(ctx)
}

// Probe returns the active StorageProbe instance (useful for tests).
func (e *Engine) Probe() *StorageProbe {
	return e.probe
}

// DiskOps returns the DiskOps implementation.
func (e *Engine) DiskOps() DiskOps {
	return e.diskOps
}

// NASOps returns the NASOps implementation.
func (e *Engine) NASOps() NASOps {
	return e.nasOps
}

// Close stops background probing and releases resources.
func (e *Engine) Close() error {
	e.probe.Stop()
	return nil
}
