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

// VerificationConfig controls the 180-second post-upgrade health verification window.
type VerificationConfig struct {
	WindowDuration      time.Duration // Default: 180s
	ProbeInterval       time.Duration // Default: 5s (in tests: lower)
	ConsecutiveFailures int           // Default: 3
	TimeFunc            func() time.Time
}

// DefaultVerificationConfig returns production defaults (180s window, 5s interval, 3 consecutive failures).
func DefaultVerificationConfig() VerificationConfig {
	return VerificationConfig{
		WindowDuration:      180 * time.Second,
		ProbeInterval:       5 * time.Second,
		ConsecutiveFailures: 3,
		TimeFunc:            time.Now,
	}
}

// VerificationSession represents an active verification session for an upgraded service.
type VerificationSession struct {
	ServiceID           string
	Revision            int
	StartTime           time.Time
	Deadline            time.Time
	ConsecutiveFailures int
	Cancel              context.CancelFunc
	Done                chan struct{}
	Passed              bool
	RolledBack          bool
}

// VerificationEngine manages health verification sessions following image upgrades or maintenance.
type VerificationEngine struct {
	mu       sync.Mutex
	mgr      *ServiceManager
	cfg      VerificationConfig
	logger   *slog.Logger
	sessions map[string]*VerificationSession
}

// NewVerificationEngine initializes a VerificationEngine.
func NewVerificationEngine(mgr *ServiceManager, cfg VerificationConfig, logger *slog.Logger) *VerificationEngine {
	if cfg.WindowDuration <= 0 {
		cfg.WindowDuration = 180 * time.Second
	}
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = 5 * time.Second
	}
	if cfg.ConsecutiveFailures <= 0 {
		cfg.ConsecutiveFailures = 3
	}
	if cfg.TimeFunc == nil {
		cfg.TimeFunc = time.Now
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &VerificationEngine{
		mgr:      mgr,
		cfg:      cfg,
		logger:   logger,
		sessions: make(map[string]*VerificationSession),
	}
}

// StartSession initiates an automated 180-second health verification session for an upgraded service.
func (v *VerificationEngine) StartSession(ctx context.Context, serviceID string, revision int) *VerificationSession {
	id := serviceID
	if v.mgr != nil {
		id = v.mgr.resolveID(serviceID)
	}

	v.mu.Lock()
	// Cancel any existing session for this service
	if existing, exists := v.sessions[id]; exists {
		existing.Cancel()
	}

	sessCtx, cancel := context.WithCancel(context.Background())
	now := v.cfg.TimeFunc()
	sess := &VerificationSession{
		ServiceID: id,
		Revision:  revision,
		StartTime: now,
		Deadline:  now.Add(v.cfg.WindowDuration),
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}
	v.sessions[id] = sess
	v.mu.Unlock()

	v.logger.Info("started 180-second health verification session",
		"service", id,
		"revision", revision,
		"window", v.cfg.WindowDuration,
	)

	go v.runVerificationLoop(sessCtx, sess)

	return sess
}

func (v *VerificationEngine) runVerificationLoop(ctx context.Context, sess *VerificationSession) {
	defer close(sess.Done)
	defer func() {
		v.mu.Lock()
		if current, exists := v.sessions[sess.ServiceID]; exists && current == sess {
			delete(v.sessions, sess.ServiceID)
		}
		v.mu.Unlock()
	}()

	ticker := time.NewTicker(v.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			v.logger.Debug("health verification session cancelled", "service", sess.ServiceID)
			return
		case <-ticker.C:
			now := v.cfg.TimeFunc()
			if now.After(sess.Deadline) {
				sess.Passed = true
				v.logger.Info("service successfully passed 180-second health verification window",
					"service", sess.ServiceID,
					"revision", sess.Revision,
				)
				return
			}

			// Probe container health
			healthy := true
			if v.mgr != nil && v.mgr.engine != nil {
				h, err := v.mgr.engine.ProbeHealth(ctx, sess.ServiceID)
				if err != nil || !h {
					healthy = false
				}
			}

			// Check if container terminated / entered crash loop
			if v.mgr != nil {
				st, err := v.mgr.GetService(ctx, sess.ServiceID)
				if err == nil && st != nil {
					if st.Status == StatusStopped || st.Status == StatusError {
						healthy = false
						sess.ConsecutiveFailures = v.cfg.ConsecutiveFailures
					}
				}
			}

			if healthy {
				sess.ConsecutiveFailures = 0
				if v.mgr != nil && v.mgr.debouncer != nil {
					v.mgr.debouncer.RecordProbe(sess.ServiceID, true)
				}
				continue
			}

			sess.ConsecutiveFailures++
			if v.mgr != nil && v.mgr.debouncer != nil {
				v.mgr.debouncer.RecordProbe(sess.ServiceID, false)
			}
			v.logger.Warn("health verification probe failed",
				"service", sess.ServiceID,
				"consecutive_failures", sess.ConsecutiveFailures,
				"threshold", v.cfg.ConsecutiveFailures,
			)

			if sess.ConsecutiveFailures >= v.cfg.ConsecutiveFailures {
				sess.RolledBack = true
				v.logger.Warn("health verification failed across consecutive retries: executing automated atomic rollback",
					"service", sess.ServiceID,
					"revision", sess.Revision,
				)
				if v.mgr != nil {
					if _, err := v.mgr.RollbackService(context.Background(), sess.ServiceID, sess.Revision); err != nil {
						v.logger.Error("automated rollback execution encountered error",
							"service", sess.ServiceID,
							"error", err,
						)
					}
				}
				return
			}
		}
	}
}

// StopSession terminates an ongoing verification session.
func (v *VerificationEngine) StopSession(serviceID string) {
	id := serviceID
	if v.mgr != nil {
		id = v.mgr.resolveID(serviceID)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if sess, exists := v.sessions[id]; exists {
		sess.Cancel()
		delete(v.sessions, id)
	}
}

// Close terminates all active verification sessions.
func (v *VerificationEngine) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for id, sess := range v.sessions {
		sess.Cancel()
		delete(v.sessions, id)
	}
}

// GetSession returns the active session for a service, if any.
func (v *VerificationEngine) GetSession(serviceID string) *VerificationSession {
	id := serviceID
	if v.mgr != nil {
		id = v.mgr.resolveID(serviceID)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sessions[id]
}

// ManualRollbackService executes an intentional manual rollback initiated by an administrator.
// The container is restored to its prior stable release and returned to Green running/healthy status once healthy.
func (m *ServiceManager) ManualRollbackService(ctx context.Context, serviceID string, failedRevNum int) (*ServiceStatus, error) {
	return m.rollbackServiceInternal(ctx, serviceID, failedRevNum, true)
}

// RollbackService executes an automated atomic rollback for a failing upgraded container.
// The service is restored to its prior release and marked degraded with reassurance microcopy.
func (m *ServiceManager) RollbackService(ctx context.Context, serviceID string, failedRevNum int) (*ServiceStatus, error) {
	return m.rollbackServiceInternal(ctx, serviceID, failedRevNum, false)
}

func (m *ServiceManager) rollbackServiceInternal(ctx context.Context, serviceID string, failedRevNum int, isManual bool) (*ServiceStatus, error) {
	id := m.resolveID(serviceID)

	m.mu.RLock()
	def, exists := m.definitions[id]
	st, stExists := m.statuses[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	serviceName := def.Name
	if stExists && st.Name != "" {
		serviceName = st.Name
	}

	// Cancel any active verification session for this service
	if m.verificationEngine != nil {
		m.verificationEngine.StopSession(id)
	}

	// Identify the prior active revision (prev_rev) and failed revision from compose_revisions
	var failedRev *store.ComposeRevision
	var prevRev *store.ComposeRevision

	if m.revisionStore != nil {
		if failedRevNum > 0 {
			failedRev, _ = m.revisionStore.GetRevision(ctx, failedRevNum)
		} else {
			activeRev, err := m.revisionStore.GetActiveRevision(ctx)
			if err == nil && activeRev != nil {
				failedRev = activeRev
				failedRevNum = activeRev.Revision
			}
		}

		revs, err := m.revisionStore.ListRevisions(ctx)
		if err == nil {
			for _, r := range revs {
				if failedRevNum > 0 && r.Revision >= failedRevNum {
					if failedRev == nil && r.Revision == failedRevNum {
						rCopy := r
						failedRev = &rCopy
					}
					continue
				}
				if r.ServiceID == id || r.ServiceID == "" {
					rCopy := r
					prevRev = &rCopy
					break
				}
			}
		}

		if (failedRev == nil && prevRev == nil) || len(revs) == 0 {
			return nil, ErrNoPriorRevision
		}
	} else {
		return nil, ErrNoPriorRevision
	}

	// 1. Terminate the failing upgraded container (engine.StopContainer)
	if m.engine != nil {
		_ = m.engine.StopContainer(ctx, id)
	}

	var restoredImage string
	var prevRevNum int
	if prevRev != nil {
		prevRevNum = prevRev.Revision
		restoredImage = prevRev.NewImage
		if restoredImage == "" {
			restoredImage = prevRev.OldImage
		}
	} else if failedRev != nil {
		restoredImage = failedRev.OldImage
	}
	if restoredImage == "" {
		restoredImage = def.Image
	}

	// 3. Restore declarative compose specification from /data/state/compose/rev-:prev_rev.yaml
	if prevRevNum > 0 {
		prevComposePath := filepath.Join(m.dataDir, "state", "compose", fmt.Sprintf("rev-%d.yaml", prevRevNum))
		activeComposePath := filepath.Join(m.dataDir, "state", "compose", "docker-compose.yaml")
		if _, err := os.Stat(prevComposePath); err == nil {
			_ = copyFile(prevComposePath, activeComposePath)
		}
	}

	// 4. Restore service configuration volume state from /data/state/backups/volumes-rev-:prev_rev (or pre-upgrade backup directory)
	var volumeSource string
	if prevRevNum > 0 {
		p := filepath.Join(m.dataDir, "state", "backups", fmt.Sprintf("volumes-rev-%d", prevRevNum), id)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			volumeSource = p
		}
	}
	if volumeSource == "" && failedRev != nil && failedRev.VolumeBackupPath != "" {
		if fi, err := os.Stat(failedRev.VolumeBackupPath); err == nil && fi.IsDir() {
			volumeSource = failedRev.VolumeBackupPath
		}
	}
	if volumeSource == "" && failedRevNum > 0 {
		p := filepath.Join(m.dataDir, "state", "backups", fmt.Sprintf("volumes-rev-%d", failedRevNum), id)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			volumeSource = p
		}
	}

	if volumeSource != "" {
		targetDir := filepath.Join(m.dataDir, "state", "services", id)
		_ = CopyDir(volumeSource, targetDir)
	}

	// 5. Restore SQLite state from /data/state/backups/pre-upgrade-:rev.db via db.RestoreSnapshot
	if m.revisionStore != nil {
		var snapshotPath string
		if failedRev != nil && failedRev.SQLiteSnapshotPath != "" {
			snapshotPath = failedRev.SQLiteSnapshotPath
		} else if failedRevNum > 0 {
			snapshotPath = filepath.Join(m.dataDir, "state", "backups", fmt.Sprintf("pre-upgrade-%d.db", failedRevNum))
		}
		if snapshotPath != "" {
			if _, err := os.Stat(snapshotPath); err == nil {
				if err := m.revisionStore.RestoreSnapshot(ctx, snapshotPath); err != nil {
					m.logger.Error("failed to restore SQLite snapshot during rollback", "snapshot", snapshotPath, "error", err)
				}
			}
		}
	}

	// 6. Restart the container with the restored container specification
	containerSpec, err := m.buildContainerSpec(ctx, id, restoredImage)
	if err == nil {
		_, _ = SynthesizeComposeSpec(m.dataDir, containerSpec)
		if m.engine != nil {
			if err := m.engine.StartContainer(ctx, containerSpec); err != nil {
				m.logger.Error("failed to restart container with restored specification", "service", id, "error", err)
			}
		}
	}

	// 7. Update revision records: mark failed revision as rolled_back and prior revision as active
	if m.revisionStore != nil {
		if failedRev != nil {
			failedRev.Status = "rolled_back"
			if _, err := m.revisionStore.GetRevision(ctx, failedRev.Revision); err != nil {
				_ = m.revisionStore.RecordRevision(ctx, failedRev)
			}
			_ = m.revisionStore.UpdateRevisionStatus(ctx, failedRev.Revision, "rolled_back")
		} else if failedRevNum > 0 {
			_ = m.revisionStore.UpdateRevisionStatus(ctx, failedRevNum, "rolled_back")
		}
		if prevRevNum > 0 {
			_ = m.revisionStore.UpdateRevisionStatus(ctx, prevRevNum, "active")
		}
	}

	// 8. Update service runtime status:
	// For automated rollbacks, set to degraded with reassurance microcopy:
	// "Update couldn't start properly. We've safely restored your previous working version."
	// For manual rollbacks, probe container health; if healthy, restore green running/healthy state.
	m.mu.Lock()
	status := StatusDegraded
	health := HealthDegraded
	reassurance := ReassuranceRollbackRestored

	if isManual {
		healthy := true
		if m.engine != nil {
			h, probeErr := m.engine.ProbeHealth(ctx, id)
			if probeErr == nil && !h {
				healthy = false
			}
		}
		if healthy {
			status = StatusRunning
			health = HealthHealthy
			reassurance = ""
		}
	}

	if st, exists := m.statuses[id]; exists {
		st.Status = status
		st.Health = health
		st.Image = restoredImage
		st.ReassuranceMessage = reassurance
		st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	}
	if def, exists := m.definitions[id]; exists {
		def.Image = restoredImage
		m.definitions[id] = def
	}
	m.mu.Unlock()

	m.debouncer.Reset(id)

	// 9. Dispatch alert email notification via notify.AlertDispatcher
	if m.alertNotifier != nil {
		failedVer := ""
		if failedRev != nil {
			failedVer = failedRev.NewImage
		}
		reason := "Container health verification failed (consecutive probe failures or crash loop)"
		if isManual {
			reason = "Manual rollback requested by administrator"
		}
		_ = m.alertNotifier.SendRollbackAlert(ctx, id, serviceName, failedVer, restoredImage, reason)
	}

	// 10. Broadcast SSE events (service.health_changed, service.status_changed)
	if m.broadcaster != nil {
		m.broadcaster("service.health_changed", ServiceHealthChanged{
			ServiceID: id,
			Status:    status,
			Health:    health,
			Detail:    reassurance,
			Timestamp: m.timeNow().UTC().Format(time.RFC3339),
		})
		m.broadcaster("service.status_changed", map[string]any{
			"service_id":          id,
			"status":              status,
			"health":              health,
			"reassurance_message": reassurance,
			"timestamp":           m.timeNow().UTC().Format(time.RFC3339),
		})
	}

	m.logger.Warn("service rollback completed",
		"service", id,
		"failed_revision", failedRevNum,
		"restored_revision", prevRevNum,
		"restored_image", restoredImage,
	)

	return m.GetService(ctx, id)
}
