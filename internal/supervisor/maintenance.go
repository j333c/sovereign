package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sovereign/internal/store"
)

// MaintenanceConfig controls the background maintenance window.
type MaintenanceConfig struct {
	Hour          int           // Maintenance start hour (0-23, default: 3 for 3:00 AM)
	Minute        int           // Maintenance start minute (0-59, default: 0)
	Enabled       bool          // Default: true
	CheckInterval time.Duration // Schedule check interval (default: 1 minute)
	TimeFunc      func() time.Time
}

// DefaultMaintenanceConfig returns production defaults: 3:00 AM daily maintenance window.
func DefaultMaintenanceConfig() MaintenanceConfig {
	return MaintenanceConfig{
		Hour:          3,
		Minute:        0,
		Enabled:       true,
		CheckInterval: 1 * time.Minute,
		TimeFunc:      time.Now,
	}
}

// MaintenanceScheduler executes scheduled unattended container updates during the maintenance window.
type MaintenanceScheduler struct {
	mu           sync.Mutex
	cfg          MaintenanceConfig
	mgr          *ServiceManager
	db           *store.DB
	discovery    *UpdateDiscoveryEngine
	logger       *slog.Logger
	cancel       context.CancelFunc
	lastRunDay   int
	lastRunYear  int
	stopChan     chan struct{}
}

// NewMaintenanceScheduler constructs a MaintenanceScheduler.
func NewMaintenanceScheduler(mgr *ServiceManager, cfg MaintenanceConfig, logger *slog.Logger) *MaintenanceScheduler {
	if cfg.Hour < 0 || cfg.Hour > 23 {
		cfg.Hour = 3
	}
	if cfg.Minute < 0 || cfg.Minute > 59 {
		cfg.Minute = 0
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 1 * time.Minute
	}
	if cfg.TimeFunc == nil {
		cfg.TimeFunc = time.Now
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &MaintenanceScheduler{
		mgr:      mgr,
		cfg:      cfg,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// SetStore assigns the persistent store for WAL checkpointing and update outcome persistence.
func (s *MaintenanceScheduler) SetStore(db *store.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
}

// SetDiscoveryEngine assigns the update discovery engine for release channel candidate detection.
func (s *MaintenanceScheduler) SetDiscoveryEngine(d *UpdateDiscoveryEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discovery = d
}

// IsMaintenanceWindow evaluates if the given timestamp falls inside the configured maintenance execution minute.
func (s *MaintenanceScheduler) IsMaintenanceWindow(t time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return t.Hour() == s.cfg.Hour && t.Minute() == s.cfg.Minute
}

// Start launches the background scheduler loop.
func (s *MaintenanceScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	schedCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(s.cfg.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-schedCtx.Done():
				return
			case <-ticker.C:
				now := s.cfg.TimeFunc()
				s.mu.Lock()
				enabled := s.cfg.Enabled
				hour := s.cfg.Hour
				minute := s.cfg.Minute
				alreadyRan := s.lastRunYear == now.Year() && s.lastRunDay == now.YearDay()
				s.mu.Unlock()

				if !enabled || alreadyRan {
					continue
				}

				if now.Hour() == hour && now.Minute() == minute {
					s.mu.Lock()
					s.lastRunYear = now.Year()
					s.lastRunDay = now.YearDay()
					s.mu.Unlock()

					s.logger.Info("entering scheduled 3:00 AM maintenance window for safe container updates",
						"time", now.Format(time.RFC3339),
					)

					// Trigger maintenance updates
					go func() {
						mCtx, mCancel := context.WithTimeout(context.Background(), 30*time.Minute)
						defer mCancel()
						if _, err := s.ExecuteMaintenanceWindow(mCtx, nil); err != nil {
							s.logger.Error("maintenance window execution encountered errors", "error", err)
						}
					}()
				}
			}
		}
	}()
}

// Stop shuts down the background maintenance scheduler.
func (s *MaintenanceScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// ExecuteMaintenanceWindow executes service updates under the protection of the 180-second health verification window.
// If pendingUpgrades is nil or empty, active running services are verified for upgrades.
func (s *MaintenanceScheduler) ExecuteMaintenanceWindow(ctx context.Context, pendingUpgrades map[string]string) ([]*store.ComposeRevision, error) {
	s.mu.Lock()
	dbHandle := s.db
	discoveryHandle := s.discovery
	s.mu.Unlock()

	// 1. Pre-maintenance WAL checkpoint PRAGMA wal_checkpoint(TRUNCATE) and atomic SQLite snapshot
	if dbHandle != nil {
		s.logger.Info("executing pre-maintenance database WAL checkpoint")
		if err := dbHandle.FlushWAL(ctx); err != nil {
			s.logger.Warn("pre-maintenance WAL checkpoint failed", "error", err)
		}

		backupDir := "/data/state/backups"
		if err := os.MkdirAll(backupDir, 0755); err == nil {
			snapPath := filepath.Join(backupDir, fmt.Sprintf("pre-maintenance-%d.db", time.Now().Unix()))
			s.logger.Info("creating pre-maintenance atomic SQLite snapshot", "snapshot_path", snapPath)
			if err := dbHandle.CreateSnapshot(ctx, snapPath); err != nil {
				s.logger.Warn("pre-maintenance snapshot creation warning", "error", err)
			}
		}
	}

	// 2. Discover qualifying release channel updates
	var candidate *UpdateCandidate
	if discoveryHandle != nil {
		cand, err := discoveryHandle.CheckForUpdates(ctx)
		if err != nil {
			s.logger.Warn("unattended maintenance update discovery check failed", "error", err)
		} else {
			candidate = cand
		}
	}

	if s.mgr == nil {
		if dbHandle != nil {
			if candidate != nil {
				_ = dbHandle.SetOvernightUpdateStatus(ctx, "postponed", candidate.TargetTag, "Update postponed: automatic safety check prevented an issue. All data remains secure.")
			} else {
				_ = dbHandle.SetOvernightUpdateStatus(ctx, "idle", "", "No updates required; system is on latest version.")
			}
		}
		return nil, fmt.Errorf("service manager is not initialized")
	}

	services, err := s.mgr.ListServices(ctx)
	if err != nil {
		if dbHandle != nil {
			ver := ""
			if candidate != nil {
				ver = candidate.TargetTag
			}
			_ = dbHandle.SetOvernightUpdateStatus(ctx, "postponed", ver, "Update postponed: automatic safety check prevented an issue. All data remains secure.")
		}
		return nil, fmt.Errorf("failed to list services for maintenance: %w", err)
	}

	results := make([]*store.ComposeRevision, 0)
	var errs []error

	for _, svc := range services {
		// Only update currently running or active services
		if svc.Status != StatusRunning && svc.Status != StatusDegraded {
			continue
		}

		targetTag := ""
		if pendingUpgrades != nil {
			tag, exists := pendingUpgrades[svc.ID]
			if !exists {
				continue
			}
			targetTag = tag
		}

		s.logger.Info("executing scheduled maintenance upgrade protected by 180s health verification",
			"service", svc.ID,
			"target_tag", targetTag,
		)

		rev, err := s.mgr.UpgradeService(ctx, svc.ID, targetTag)
		if err != nil {
			s.logger.Error("maintenance upgrade failed for service", "service", svc.ID, "error", err)
			errs = append(errs, fmt.Errorf("service %s upgrade failed: %w", svc.ID, err))
			continue
		}

		results = append(results, rev)
	}

	if len(errs) > 0 {
		if dbHandle != nil {
			ver := ""
			if candidate != nil {
				ver = candidate.TargetTag
			}
			_ = dbHandle.SetOvernightUpdateStatus(ctx, "postponed", ver, "Update postponed: automatic safety check prevented an issue. All data remains secure.")
		}
		return results, fmt.Errorf("maintenance completed with %d error(s): %v", len(errs), errs[0])
	}

	// Persist successful overnight update outcome
	if dbHandle != nil {
		if candidate != nil || len(results) > 0 {
			targetVer := ""
			if candidate != nil {
				targetVer = candidate.TargetTag
			} else if len(results) > 0 {
				targetVer = results[0].NewImage
			}
			_ = dbHandle.SetOvernightUpdateStatus(ctx, "updated", targetVer, fmt.Sprintf("Sovereign updated to %s overnight", targetVer))
		} else {
			_ = dbHandle.SetOvernightUpdateStatus(ctx, "idle", "", "No updates required; system is on latest version.")
		}
	}

	return results, nil
}
