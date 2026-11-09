package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WatchdogConfig specifies dampening thresholds for container health checking.
type WatchdogConfig struct {
	MinFailures int           // Minimum consecutive failures before degrading (default 3)
	MinWindow   time.Duration // Minimum elapsed window of failure before degrading (default 60s)
	Interval    time.Duration // Probing interval (default 10s)
	TimeFunc    func() time.Time
}

// ServiceWatchdogState tracks consecutive probe failures for a service.
type ServiceWatchdogState struct {
	ConsecutiveFailures int
	FirstFailureTime    time.Time
	LastProbeTime       time.Time
	CurrentHealth       string
}

// WatchdogDebouncer enforces debouncing invariants on container health probes.
type WatchdogDebouncer struct {
	mu     sync.Mutex
	cfg    WatchdogConfig
	states map[string]*ServiceWatchdogState
}

// NewWatchdogDebouncer initializes a debouncer with given configuration.
func NewWatchdogDebouncer(cfg WatchdogConfig) *WatchdogDebouncer {
	if cfg.MinFailures <= 0 {
		cfg.MinFailures = 3
	}
	if cfg.MinWindow <= 0 {
		cfg.MinWindow = 60 * time.Second
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.TimeFunc == nil {
		cfg.TimeFunc = time.Now
	}

	return &WatchdogDebouncer{
		cfg:    cfg,
		states: make(map[string]*ServiceWatchdogState),
	}
}

// RecordProbe evaluates a probe result and returns whether health transitioned, new health, new status, and if restart should be triggered.
func (w *WatchdogDebouncer) RecordProbe(serviceID string, healthy bool) (healthChanged bool, newHealth string, newStatus string, shouldRestart bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.cfg.TimeFunc()
	state, exists := w.states[serviceID]
	if !exists {
		state = &ServiceWatchdogState{
			CurrentHealth: HealthHealthy,
		}
		w.states[serviceID] = state
	}
	state.LastProbeTime = now

	if healthy {
		wasDegraded := state.CurrentHealth == HealthDegraded
		state.ConsecutiveFailures = 0
		state.FirstFailureTime = time.Time{}
		state.CurrentHealth = HealthHealthy

		if wasDegraded {
			return true, HealthHealthy, StatusRunning, false
		}
		return false, HealthHealthy, StatusRunning, false
	}

	// Probe failed
	state.ConsecutiveFailures++
	if state.FirstFailureTime.IsZero() {
		state.FirstFailureTime = now
	}

	elapsed := now.Sub(state.FirstFailureTime)
	// Sustained failure requires BOTH at least MinFailures consecutive failures AND elapsed >= MinWindow
	if state.ConsecutiveFailures >= w.cfg.MinFailures && elapsed >= w.cfg.MinWindow {
		wasNotDegraded := state.CurrentHealth != HealthDegraded
		state.CurrentHealth = HealthDegraded
		if wasNotDegraded {
			return true, HealthDegraded, StatusDegraded, true
		}
		// Already degraded, keep attempting recovery restart
		return false, HealthDegraded, StatusDegraded, true
	}

	// Transient blip (e.g. 1 or 2 failures, or < 60s elapsed)
	return false, state.CurrentHealth, StatusRunning, false
}

// Reset clears failure tracking for a service.
func (w *WatchdogDebouncer) Reset(serviceID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.states, serviceID)
}

// GetState returns a copy of watchdog state for a service.
func (w *WatchdogDebouncer) GetState(serviceID string) ServiceWatchdogState {
	w.mu.Lock()
	defer w.mu.Unlock()
	if s, exists := w.states[serviceID]; exists {
		return *s
	}
	return ServiceWatchdogState{CurrentHealth: HealthUnknown}
}

// StartWatchdog starts the background ticker probing health for running services.
func (m *ServiceManager) StartWatchdog(ctx context.Context) {
	watchCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancelWatch = cancel
	m.mu.Unlock()

	interval := m.debouncer.cfg.Interval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				m.probeActiveServices(watchCtx)
			}
		}
	}()
}

func (m *ServiceManager) probeActiveServices(ctx context.Context) {
	m.mu.RLock()
	activeIDs := make([]string, 0)
	for id, st := range m.statuses {
		if st.Status == StatusRunning || st.Status == StatusDegraded {
			activeIDs = append(activeIDs, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range activeIDs {
		// Under read-lock, verify status is still running or degraded before probing/acting
		m.mu.RLock()
		st, exists := m.statuses[id]
		if !exists || (st.Status != StatusRunning && st.Status != StatusDegraded) {
			m.mu.RUnlock()
			continue
		}
		m.mu.RUnlock()

		if id == ServiceLlama && m.llamaProvisioner != nil {
			res, err := m.llamaProvisioner.Provision(ctx)
			if err == nil && res != nil && res.InSafeStub {
				continue
			}
			if err == nil && res != nil && !res.InSafeStub {
				m.mu.RLock()
				isDegraded := (st.Health == HealthDegraded)
				m.mu.RUnlock()
				if isDegraded {
					// Model was placed on disk while in safe stub: auto-transition
					if present, _ := m.engine.IsImagePresent(ctx, res.ContainerSpec.Image); !present {
						_ = m.engine.PullImage(ctx, res.ContainerSpec.Image, nil)
					}
					if err := m.engine.StartContainer(ctx, res.ContainerSpec); err == nil {
						m.mu.Lock()
						st.Health = HealthHealthy
						st.Status = StatusRunning
						st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
						m.mu.Unlock()
						m.debouncer.Reset(id)
						if m.broadcaster != nil {
							m.broadcaster("service.health_changed", ServiceHealthChanged{
								ServiceID: id,
								Status:    StatusRunning,
								Health:    HealthHealthy,
								Detail:    "Model detected; AI service started",
								Timestamp: m.timeNow().UTC().Format(time.RFC3339),
							})
						}
					}
					continue
				}
			}
		}

		healthy, err := m.engine.ProbeHealth(ctx, id)
		if err != nil {
			healthy = false
		}

		healthChanged, newHealth, newStatus, shouldRestart := m.debouncer.RecordProbe(id, healthy)
		if healthChanged {
			m.mu.Lock()
			if st, exists := m.statuses[id]; exists {
				st.Health = newHealth
				st.Status = newStatus
				st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
			}
			m.mu.Unlock()

			m.logger.Warn("service health changed", "service", id, "health", newHealth, "status", newStatus)
			if m.broadcaster != nil {
				m.broadcaster("service.health_changed", ServiceHealthChanged{
					ServiceID: id,
					Status:    newStatus,
					Health:    newHealth,
					Detail:    fmt.Sprintf("Service health transitioned to %s", newHealth),
					Timestamp: m.timeNow().UTC().Format(time.RFC3339),
				})
			}
		}

		if shouldRestart {
			m.debouncer.Reset(id)
			m.logger.Warn("sustained health probe failure: attempting recovery restart", "service", id)
			go func(targetID string) {
				if err := m.engine.RestartContainer(context.Background(), targetID); err != nil {
					m.logger.Error("watchdog recovery restart failed", "service", targetID, "error", err)
				}
			}(id)
		}
	}
}

// Debouncer returns the active WatchdogDebouncer.
func (m *ServiceManager) Debouncer() *WatchdogDebouncer {
	return m.debouncer
}
