package storage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"sovereign/internal/store"
)

// Storage overall status levels
const (
	StatusLevelOK            = "ok"
	StatusLevelDegraded      = "degraded"
	StatusLevelNotConfigured = "not_configured"
)

// Storage pool types
const (
	PoolTypeLocal  = "local"
	PoolTypeNAS    = "nas"
	PoolTypeHybrid = "hybrid"
	PoolTypeNone   = "none"
)

// StorageStatusResponse represents the complete storage telemetry and status payload.
type StorageStatusResponse struct {
	Status         string            `json:"status"` // "ok", "degraded", "not_configured"
	PoolType       string            `json:"pool_type"` // "local", "nas", "hybrid", "none"
	MountPoint     string            `json:"mount_point"`
	TotalBytes     uint64            `json:"total_bytes"`
	AvailableBytes uint64            `json:"available_bytes"`
	UsedBytes      uint64            `json:"used_bytes"`
	LocalPool      *StorageLocalPool `json:"local_pool,omitempty"`
	NAS            *StorageNASStatus `json:"nas,omitempty"`
}

// StorageProbe monitors the health of storage pools and mounted network shares.
type StorageProbe struct {
	diskOps         DiskOps
	nasOps          NASOps
	db              *store.DB
	dataDir         string
	logger          *slog.Logger
	interval        time.Duration
	mu              sync.RWMutex
	currentStatus   StorageStatusResponse
	activeNASConfig *NASConfig
	activeLocalPool *StorageLocalPool
	stopCh          chan struct{}
	running         bool
}

// NewStorageProbe initializes the storage health probe.
func NewStorageProbe(dataDir string, db *store.DB, diskOps DiskOps, nasOps NASOps, logger *slog.Logger) *StorageProbe {
	if logger == nil {
		logger = slog.Default()
	}
	if diskOps == nil {
		diskOps = NewDefaultDiskOps(logger)
	}
	if nasOps == nil {
		nasOps = NewDefaultNASOps(logger)
	}

	sp := &StorageProbe{
		diskOps:  diskOps,
		nasOps:   nasOps,
		db:       db,
		dataDir:  dataDir,
		logger:   logger,
		interval: 5 * time.Second,
		stopCh:   make(chan struct{}),
		currentStatus: StorageStatusResponse{
			Status:     StatusLevelNotConfigured,
			PoolType:   PoolTypeNone,
			MountPoint: dataDir,
		},
	}

	return sp
}

// SetInterval configures the periodic probe interval (useful for tests).
func (p *StorageProbe) SetInterval(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.interval = d
}

// SetActiveNASConfig sets the current active NAS configuration for probing.
func (p *StorageProbe) SetActiveNASConfig(cfg *NASConfig) {
	p.mu.Lock()
	p.activeNASConfig = cfg
	p.mu.Unlock()
}

// SetActiveLocalPool sets the current active local storage pool.
func (p *StorageProbe) SetActiveLocalPool(pool *StorageLocalPool) {
	p.mu.Lock()
	p.activeLocalPool = pool
	p.mu.Unlock()
}

// Start begins the periodic probing loop in a background goroutine.
func (p *StorageProbe) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	interval := p.interval
	stopCh := p.stopCh
	p.mu.Unlock()

	// Initial probe execution
	p.ProbeOnce(ctx)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("recovered from storage probe panic", "panic", r)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				p.ProbeOnce(probeCtx)
				cancel()
			}
		}
	}()
}

// Stop stops the background probe loop.
func (p *StorageProbe) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		p.running = false
		close(p.stopCh)
		p.stopCh = make(chan struct{})
	}
}

// ProbeOnce runs a single storage and NAS reachability verification pass.
// Invariant: This method must never panic or crash the supervisor daemon.
func (p *StorageProbe) ProbeOnce(ctx context.Context) StorageStatusResponse {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("recovered from storage ProbeOnce panic", "panic", r)
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Try loading NAS config from DB if not already set
	if p.activeNASConfig == nil && p.db != nil {
		if cfg, err := LoadNASConfig(ctx, p.db); err == nil && cfg != nil {
			p.activeNASConfig = cfg
		}
	}

	nasStatus := &StorageNASStatus{
		Configured: false,
	}

	nasDegraded := false
	if p.activeNASConfig != nil {
		nasStatus.Configured = true
		nasStatus.Type = p.activeNASConfig.Type
		nasStatus.Host = p.activeNASConfig.Host
		nasStatus.Share = p.activeNASConfig.Share
		nasStatus.Username = p.activeNASConfig.Username
		nasStatus.MountPoint = p.activeNASConfig.MountPoint

		// Invariant: password never returned in plaintext, redacted as "***"
		if p.activeNASConfig.Password != "" {
			nasStatus.Password = "***"
		}

		// Non-blocking accessibility check with timeout guard
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := p.nasOps.CheckAccessibility(checkCtx, *p.activeNASConfig)
		cancel()

		if err != nil {
			nasDegraded = true
			nasStatus.Status = NASStatusDegraded
			nasStatus.Message = ConsumerNASDegradedMsg
			p.logger.Warn("network storage share temporarily disconnected, transitioning to degraded status",
				"host", p.activeNASConfig.Host,
				"share", p.activeNASConfig.Share,
				"error", err)
		} else {
			nasStatus.Status = NASStatusConnected
			nasStatus.Message = "Connected"
		}
	}

	// Calculate overall pool type and status
	poolType := PoolTypeNone
	if p.activeLocalPool != nil && nasStatus.Configured {
		poolType = PoolTypeHybrid
	} else if p.activeLocalPool != nil {
		poolType = PoolTypeLocal
	} else if nasStatus.Configured {
		poolType = PoolTypeNAS
	}

	overallStatus := StatusLevelOK
	if poolType == PoolTypeNone {
		overallStatus = StatusLevelNotConfigured
	} else if nasDegraded {
		overallStatus = StatusLevelDegraded
	}

	// Fetch filesystem usage statistics for dataDir
	total, avail, used, err := p.diskOps.GetDiskUsage(p.dataDir)
	if err != nil {
		p.logger.Debug("unable to retrieve disk usage", "dir", p.dataDir, "error", err)
	}

	p.currentStatus = StorageStatusResponse{
		Status:         overallStatus,
		PoolType:       poolType,
		MountPoint:     p.dataDir,
		TotalBytes:     total,
		AvailableBytes: avail,
		UsedBytes:      used,
		LocalPool:      p.activeLocalPool,
		NAS:            nasStatus,
	}

	return p.currentStatus
}

// GetStatus returns the current storage telemetry snapshot.
func (p *StorageProbe) GetStatus() StorageStatusResponse {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentStatus
}
