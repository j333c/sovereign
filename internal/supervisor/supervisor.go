package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Canonical service identifiers
const (
	ServiceJellyfin       = "jellyfin"
	ServiceNavidrome      = "navidrome"
	ServiceAudiobookshelf = "audiobookshelf"
	ServicePaperless      = "paperless"
)

// Official project OCI image pins
const (
	ImageJellyfin       = "jellyfin/jellyfin:12.2.0"
	ImageNavidrome      = "deluan/navidrome:0.54.5"
	ImageAudiobookshelf = "advplyr/audiobookshelf:2.19.5"
	ImagePaperless      = "paperlessngx/paperless-ngx:3.3.1"
)

// Strict memory bounds
const (
	MemLimitJellyfin       int64 = 2048 * 1024 * 1024 // 2048 MB
	MemReservationJellyfin int64 = 1024 * 1024 * 1024 // 1024 MB

	MemLimitNavidrome       int64 = 512 * 1024 * 1024 // 512 MB
	MemReservationNavidrome int64 = 256 * 1024 * 1024 // 256 MB

	MemLimitAudiobookshelf       int64 = 512 * 1024 * 1024 // 512 MB
	MemReservationAudiobookshelf int64 = 256 * 1024 * 1024 // 256 MB

	MemLimitPaperless       int64 = 1024 * 1024 * 1024 // 1024 MB
	MemReservationPaperless int64 = 512 * 1024 * 1024  // 512 MB
)

// Service lifecycle states
const (
	StatusInactive = "inactive"
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusDegraded = "degraded"
	StatusStopped  = "stopped"
	StatusError    = "error"
)

// Service health states
const (
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
	HealthUnknown   = "unknown"
)

var (
	ErrServiceNotFound  = errors.New("service not found")
	ErrPullFailed       = errors.New("image pull failed")
	ErrInactiveService  = errors.New("cannot restart inactive service; activate it first")
	ErrServiceInactive  = errors.New("service is inactive")
	ErrInvalidOperation = errors.New("invalid operation")
)

// ServiceDef holds the immutable specification of a managed service.
type ServiceDef struct {
	ID                  string
	Name                string
	Description         string
	Image               string
	MemLimitBytes       int64
	MemReservationBytes int64
	Port                int
	Subpath             string
	VolumeTemplates     []string // relative to dataDir or absolute
}

// ServiceStatus represents the current runtime and health snapshot of a service.
type ServiceStatus struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Status              string   `json:"status"`
	Health              string   `json:"health"`
	Image               string   `json:"image"`
	ImageDownloaded     bool     `json:"image_downloaded"`
	MemLimitBytes       int64    `json:"mem_limit_bytes"`
	MemReservationBytes int64    `json:"mem_reservation_bytes"`
	Subpath             string   `json:"subpath"`
	Port                int      `json:"port"`
	Volumes             []string `json:"volumes,omitempty"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
}

// ServicePullProgress matches the OpenAPI service.pull_progress event payload.
type ServicePullProgress struct {
	ServiceID       string  `json:"service_id"`
	ProgressPercent float64 `json:"progress_percent"`
	Status          string  `json:"status"`
}

// ServiceHealthChanged matches the OpenAPI service.health_changed event payload.
type ServiceHealthChanged struct {
	ServiceID string `json:"service_id"`
	Status    string `json:"status"`
	Health    string `json:"health"`
	Detail    string `json:"detail"`
	Timestamp string `json:"timestamp"`
}

// ServiceError matches the OpenAPI service.error event payload.
type ServiceError struct {
	ServiceID string `json:"service_id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// EventBroadcaster callback emits named SSE events to subscribers.
type EventBroadcaster func(event string, data any)

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

// ServiceManagerConfig holds options for initializing the ServiceManager.
type ServiceManagerConfig struct {
	DataDir              string
	Engine               RuntimeEngine
	Logger               *slog.Logger
	WatchdogConfig       WatchdogConfig
	EventBroadcaster     EventBroadcaster
	HardwareOpts         HardwareProbeOptions
	JellyfinProvisioner  *JellyfinProvisioner
	AudioProvisioner     *AudioProvisioner
	PaperlessProvisioner *PaperlessProvisioner
}

// ServiceManager orchestrates on-demand container lifecycles, health probing, and volume preservation.
type ServiceManager struct {
	mu                   sync.RWMutex
	dataDir              string
	engine               RuntimeEngine
	logger               *slog.Logger
	broadcaster          EventBroadcaster
	debouncer            *WatchdogDebouncer
	definitions          map[string]ServiceDef
	statuses             map[string]*ServiceStatus
	activating           map[string]bool
	cancelWatch          context.CancelFunc
	stopOnce             sync.Once
	timeNow              func() time.Time
	jellyfinProvisioner  *JellyfinProvisioner
	audioProvisioner     *AudioProvisioner
	paperlessProvisioner *PaperlessProvisioner
}

// NewServiceManager constructs and initializes the appliance service lifecycle manager.
func NewServiceManager(cfg ServiceManagerConfig) *ServiceManager {
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Engine == nil {
		cfg.Engine = NewMockRuntimeEngine()
	}

	jp := cfg.JellyfinProvisioner
	if jp == nil {
		jp = NewJellyfinProvisioner(cfg.DataDir, cfg.Logger, cfg.HardwareOpts)
	}

	ap := cfg.AudioProvisioner
	if ap == nil {
		ap = NewAudioProvisioner(cfg.DataDir, cfg.Logger)
	}

	pp := cfg.PaperlessProvisioner
	if pp == nil {
		pp = NewPaperlessProvisioner(cfg.DataDir, cfg.Logger)
	}

	timeFunc := cfg.WatchdogConfig.TimeFunc
	if timeFunc == nil {
		timeFunc = time.Now
	}

	debouncer := NewWatchdogDebouncer(cfg.WatchdogConfig)

	// Canonical service definitions
	defs := map[string]ServiceDef{
		ServiceJellyfin: {
			ID:                  ServiceJellyfin,
			Name:                "Jellyfin",
			Description:         "Video Streaming",
			Image:               ImageJellyfin,
			MemLimitBytes:       MemLimitJellyfin,
			MemReservationBytes: MemReservationJellyfin,
			Port:                8096,
			Subpath:             "/media",
			VolumeTemplates: []string{
				"media/movies:/data/media/movies",
				"media/tv:/data/media/tv",
				"state/services/jellyfin/config:/config",
				"state/services/jellyfin/cache:/cache",
			},
		},
		ServiceNavidrome: {
			ID:                  ServiceNavidrome,
			Name:                "Navidrome",
			Description:         "Music Streaming",
			Image:               ImageNavidrome,
			MemLimitBytes:       MemLimitNavidrome,
			MemReservationBytes: MemReservationNavidrome,
			Port:                4533,
			Subpath:             "/music",
			VolumeTemplates: []string{
				"media/music:/music:ro",
				"state/services/navidrome/data:/data",
			},
		},
		ServiceAudiobookshelf: {
			ID:                  ServiceAudiobookshelf,
			Name:                "Audiobookshelf",
			Description:         "Audiobook Streaming",
			Image:               ImageAudiobookshelf,
			MemLimitBytes:       MemLimitAudiobookshelf,
			MemReservationBytes: MemReservationAudiobookshelf,
			Port:                13378,
			Subpath:             "/audiobooks",
			VolumeTemplates: []string{
				"media/audiobooks:/audiobooks",
				"state/services/audiobookshelf/config:/config",
				"state/services/audiobookshelf/metadata:/metadata",
			},
		},
		ServicePaperless: {
			ID:                  ServicePaperless,
			Name:                "Paperless-ngx",
			Description:         "Document Management",
			Image:               ImagePaperless,
			MemLimitBytes:       MemLimitPaperless,
			MemReservationBytes: MemReservationPaperless,
			Port:                8000,
			Subpath:             "/docs",
			VolumeTemplates: []string{
				"documents/data:/usr/src/paperless/data",
				"documents/media:/usr/src/paperless/media",
				"documents/consume:/usr/src/paperless/consume",
				"documents/export:/usr/src/paperless/export",
				"state/services/paperless:/usr/src/paperless/state",
			},
		},
	}

	statuses := make(map[string]*ServiceStatus)
	for id, def := range defs {
		volumes := resolveVolumes(cfg.DataDir, def.VolumeTemplates)
		statuses[id] = &ServiceStatus{
			ID:                  id,
			Name:                def.Name,
			Description:         def.Description,
			Status:              StatusInactive,
			Health:              HealthUnknown,
			Image:               def.Image,
			ImageDownloaded:     false,
			MemLimitBytes:       def.MemLimitBytes,
			MemReservationBytes: def.MemReservationBytes,
			Subpath:             def.Subpath,
			Port:                def.Port,
			Volumes:             volumes,
			UpdatedAt:           timeFunc().UTC().Format(time.RFC3339),
		}
	}

	return &ServiceManager{
		dataDir:              cfg.DataDir,
		engine:               cfg.Engine,
		logger:               cfg.Logger,
		broadcaster:          cfg.EventBroadcaster,
		debouncer:            debouncer,
		definitions:          defs,
		statuses:             statuses,
		activating:           make(map[string]bool),
		timeNow:              timeFunc,
		jellyfinProvisioner:  jp,
		audioProvisioner:     ap,
		paperlessProvisioner: pp,
	}
}

// SetEventBroadcaster registers or updates the SSE event broadcast callback.
func (m *ServiceManager) SetEventBroadcaster(broadcaster EventBroadcaster) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcaster = broadcaster
}

func resolveVolumes(dataDir string, templates []string) []string {
	resolved := make([]string, len(templates))
	for i, t := range templates {
		parts := strings.Split(t, ":")
		hostRel := parts[0]
		hostAbs := filepath.Join(dataDir, hostRel)
		if len(parts) == 2 {
			resolved[i] = fmt.Sprintf("%s:%s", hostAbs, parts[1])
		} else if len(parts) >= 3 {
			resolved[i] = fmt.Sprintf("%s:%s:%s", hostAbs, parts[1], parts[2])
		}
	}
	return resolved
}

// ListServices returns all managed services and their current status.
func (m *ServiceManager) ListServices(ctx context.Context) ([]ServiceStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Ordered list for deterministic responses: jellyfin, navidrome, audiobookshelf, paperless
	order := []string{ServiceJellyfin, ServiceNavidrome, ServiceAudiobookshelf, ServicePaperless}
	result := make([]ServiceStatus, 0, len(order))

	for _, id := range order {
		if st, exists := m.statuses[id]; exists {
			// Check if image is downloaded in engine if status says not downloaded
			stCopy := *st
			if !stCopy.ImageDownloaded && m.engine != nil {
				if present, err := m.engine.IsImagePresent(ctx, stCopy.Image); err == nil && present {
					stCopy.ImageDownloaded = true
				}
			}
			result = append(result, stCopy)
		}
	}

	return result, nil
}

func (m *ServiceManager) resolveID(id string) string {
	switch id {
	case "video":
		return ServiceJellyfin
	case "audio", "music":
		return ServiceNavidrome
	case "audiobook", "audiobooks", "books":
		return ServiceAudiobookshelf
	case "dms", "documents", "paperless":
		return ServicePaperless
	default:
		return id
	}
}

// GetService retrieves status for a single service ID.
func (m *ServiceManager) GetService(ctx context.Context, id string) (*ServiceStatus, error) {
	id = m.resolveID(id)
	m.mu.RLock()
	defer m.mu.RUnlock()

	st, exists := m.statuses[id]
	if !exists {
		return nil, ErrServiceNotFound
	}

	stCopy := *st
	if !stCopy.ImageDownloaded && m.engine != nil {
		if present, err := m.engine.IsImagePresent(ctx, stCopy.Image); err == nil && present {
			stCopy.ImageDownloaded = true
		}
	}

	return &stCopy, nil
}

// ActivateService pulls the image on-demand, streams SSE progress, synthesizes Compose spec, and starts the container.
func (m *ServiceManager) ActivateService(ctx context.Context, id string) (*ServiceStatus, error) {
	m.mu.Lock()
	if m.audioProvisioner == nil {
		m.audioProvisioner = NewAudioProvisioner(m.dataDir, m.logger)
	}
	if m.paperlessProvisioner == nil {
		m.paperlessProvisioner = NewPaperlessProvisioner(m.dataDir, m.logger)
	}
	m.mu.Unlock()

	if id == "audio" {
		navSvc, err := m.ActivateService(ctx, ServiceNavidrome)
		if err != nil {
			return nil, fmt.Errorf("audio bundle failed to activate %s: %w", ServiceNavidrome, err)
		}
		if _, err := m.ActivateService(ctx, ServiceAudiobookshelf); err != nil {
			m.logger.Warn("audio bundle failed to activate audiobookshelf", "error", err)
			_, _ = m.DeactivateService(ctx, ServiceNavidrome)
			return nil, fmt.Errorf("audio bundle failed to activate %s: %w", ServiceAudiobookshelf, err)
		}
		return navSvc, nil
	}

	id = m.resolveID(id)
	m.mu.Lock()
	if m.activating[id] {
		m.mu.Unlock()
		return nil, errors.New("service activation already in progress")
	}
	def, exists := m.definitions[id]
	if !exists {
		m.mu.Unlock()
		return nil, ErrServiceNotFound
	}

	st := m.statuses[id]
	if st.Status == StatusRunning {
		stCopy := *st
		m.mu.Unlock()
		return &stCopy, nil
	}

	m.activating[id] = true
	st.Status = StatusStarting
	st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activating, id)
		m.mu.Unlock()
	}()

	emitError := func(errMsg string) {
		m.mu.Lock()
		st.Status = StatusError
		st.Health = HealthUnhealthy
		st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
		m.mu.Unlock()

		if m.broadcaster != nil {
			m.broadcaster("service.error", ServiceError{
				ServiceID: id,
				Message:   errMsg,
				Timestamp: m.timeNow().UTC().Format(time.RFC3339),
			})
		}
	}

	// 1. On-Demand Image Pull if not present
	present, _ := m.engine.IsImagePresent(ctx, def.Image)
	if !present {
		m.logger.Info("pulling service image on-demand", "service", id, "image", def.Image)
		progressCb := func(percent float64, status string) {
			if m.broadcaster != nil {
				m.broadcaster("service.pull_progress", ServicePullProgress{
					ServiceID:       id,
					ProgressPercent: percent,
					Status:          status,
				})
			}
		}

		if err := m.engine.PullImage(ctx, def.Image, progressCb); err != nil {
			m.logger.Error("failed to pull image for service", "service", id, "error", err)
			m.mu.Lock()
			st.Status = StatusInactive
			st.Health = HealthUnknown
			st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
			m.mu.Unlock()

			if m.broadcaster != nil {
				m.broadcaster("service.error", ServiceError{
					ServiceID: id,
					Message:   fmt.Sprintf("Failed to pull image %s: %v", def.Image, err),
					Timestamp: m.timeNow().UTC().Format(time.RFC3339),
				})
			}
			return nil, fmt.Errorf("%w: %v", ErrPullFailed, err)
		}
	}

	// 2. Ensure Volume Directories exist without altering existing content
	volumes := resolveVolumes(m.dataDir, def.VolumeTemplates)
	for _, vol := range volumes {
		parts := strings.Split(vol, ":")
		hostPath := parts[0]
		if err := os.MkdirAll(hostPath, 0755); err != nil {
			m.logger.Error("failed to ensure volume directory", "path", hostPath, "error", err)
			emitError(fmt.Sprintf("Failed to prepare volume directory %s: %v", hostPath, err))
			return nil, fmt.Errorf("failed to prepare volume directory %s: %w", hostPath, err)
		}
	}

	var devices []string
	var udpPorts map[int]int
	var env map[string]string

	if id == ServiceJellyfin && m.jellyfinProvisioner != nil {
		jellyfinRes, err := m.jellyfinProvisioner.Provision(ctx)
		if err != nil {
			m.logger.Error("failed to provision jellyfin specialized environment", "error", err)
			emitError(fmt.Sprintf("Failed to provision Jellyfin: %v", err))
			return nil, fmt.Errorf("failed to provision jellyfin: %w", err)
		}
		devices = jellyfinRes.Devices
		udpPorts = jellyfinRes.UDPPorts
	} else if (id == ServiceNavidrome || id == ServiceAudiobookshelf) && m.audioProvisioner != nil {
		audioRes, err := m.audioProvisioner.Provision(ctx, id)
		if err != nil {
			m.logger.Error("failed to provision audio specialized environment", "service", id, "error", err)
			emitError(fmt.Sprintf("Failed to provision %s: %v", id, err))
			return nil, fmt.Errorf("failed to provision %s: %w", id, err)
		}
		env = audioRes.Env
		devices = audioRes.Devices
	} else if id == ServicePaperless && m.paperlessProvisioner != nil {
		paperlessRes, err := m.paperlessProvisioner.Provision(ctx)
		if err != nil {
			m.logger.Error("failed to provision paperless specialized environment", "service", id, "error", err)
			emitError(fmt.Sprintf("Failed to provision %s: %v", id, err))
			return nil, fmt.Errorf("failed to provision %s: %w", id, err)
		}
		env = paperlessRes.Env
		devices = paperlessRes.Devices
	}

	// 3. Synthesize Compose specification
	containerSpec := ContainerSpec{
		Name:                   def.ID,
		Image:                  def.Image,
		MemoryLimitBytes:       def.MemLimitBytes,
		MemoryReservationBytes: def.MemReservationBytes,
		Ports:                  map[int]int{def.Port: def.Port},
		UDPPorts:               udpPorts,
		Devices:                devices,
		Volumes:                volumes,
		Networks:               []string{"sovereign-ingress", "sovereign-control"},
		Env:                    env,
	}

	if _, err := SynthesizeComposeSpec(m.dataDir, containerSpec); err != nil {
		m.logger.Error("failed to synthesize compose spec", "service", id, "error", err)
		emitError(fmt.Sprintf("Failed to synthesize compose spec: %v", err))
		return nil, fmt.Errorf("failed to synthesize compose spec: %w", err)
	}

	// 4. Start Container Runtime
	if err := m.engine.StartContainer(ctx, containerSpec); err != nil {
		m.logger.Error("failed to start container", "service", id, "error", err)
		emitError(fmt.Sprintf("Failed to start service container: %v", err))
		return nil, fmt.Errorf("failed to start service container: %w", err)
	}

	// 5. Update Status & Reset Watchdog
	m.mu.Lock()
	st.Status = StatusRunning
	st.Health = HealthHealthy
	st.ImageDownloaded = true
	st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	stCopy := *st
	m.mu.Unlock()

	m.debouncer.Reset(id)

	if m.broadcaster != nil {
		m.broadcaster("service.health_changed", ServiceHealthChanged{
			ServiceID: id,
			Status:    StatusRunning,
			Health:    HealthHealthy,
			Detail:    "Service activated and running",
			Timestamp: m.timeNow().UTC().Format(time.RFC3339),
		})
	}

	m.logger.Info("service activated successfully", "service", id)
	return &stCopy, nil
}

// DeactivateService stops the container runtime while strictly preserving all data volumes under /data.
func (m *ServiceManager) DeactivateService(ctx context.Context, id string) (*ServiceStatus, error) {
	if id == "audio" {
		var lastErr error
		var lastSt *ServiceStatus
		if st, err := m.DeactivateService(ctx, ServiceNavidrome); err != nil {
			lastErr = err
		} else {
			lastSt = st
		}
		if st, err := m.DeactivateService(ctx, ServiceAudiobookshelf); err != nil {
			lastErr = err
		} else if lastSt == nil {
			lastSt = st
		}
		if lastErr != nil && lastSt == nil {
			return nil, lastErr
		}
		return lastSt, nil
	}

	id = m.resolveID(id)
	m.mu.Lock()
	_, exists := m.definitions[id]
	if !exists {
		m.mu.Unlock()
		return nil, ErrServiceNotFound
	}
	st := m.statuses[id]
	m.mu.Unlock()

	m.logger.Info("deactivating service (preserving persistent volumes)", "service", id)

	// Stop runtime instance
	if err := m.engine.StopContainer(ctx, id); err != nil {
		m.logger.Warn("error stopping container on deactivation", "service", id, "error", err)
	}

	m.debouncer.Reset(id)

	m.mu.Lock()
	st.Status = StatusInactive
	st.Health = HealthUnknown
	st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	stCopy := *st
	m.mu.Unlock()

	if m.broadcaster != nil {
		m.broadcaster("service.health_changed", ServiceHealthChanged{
			ServiceID: id,
			Status:    StatusInactive,
			Health:    HealthUnknown,
			Detail:    "Service deactivated",
			Timestamp: m.timeNow().UTC().Format(time.RFC3339),
		})
	}

	return &stCopy, nil
}

// RestartService restarts the container instance with current configuration.
func (m *ServiceManager) RestartService(ctx context.Context, id string) (*ServiceStatus, error) {
	if id == "audio" {
		navSt, errNav := m.RestartService(ctx, ServiceNavidrome)
		_, errAbs := m.RestartService(ctx, ServiceAudiobookshelf)
		if errNav != nil && errAbs != nil {
			return nil, fmt.Errorf("audio bundle failed to restart: navidrome: %w, audiobookshelf: %v", errNav, errAbs)
		}
		if errNav != nil {
			return nil, errNav
		}
		if errAbs != nil {
			return nil, errAbs
		}
		return navSt, nil
	}

	id = m.resolveID(id)
	m.mu.Lock()
	_, exists := m.definitions[id]
	if !exists {
		m.mu.Unlock()
		return nil, ErrServiceNotFound
	}
	st := m.statuses[id]
	if st.Status == StatusInactive {
		m.mu.Unlock()
		return nil, ErrInactiveService
	}
	m.mu.Unlock()

	m.logger.Info("restarting service", "service", id)

	if err := m.engine.RestartContainer(ctx, id); err != nil {
		m.logger.Error("failed to restart container", "service", id, "error", err)
		return nil, fmt.Errorf("failed to restart service: %w", err)
	}

	m.debouncer.Reset(id)

	m.mu.Lock()
	st.Status = StatusRunning
	st.Health = HealthHealthy
	st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	stCopy := *st
	m.mu.Unlock()

	if m.broadcaster != nil {
		m.broadcaster("service.health_changed", ServiceHealthChanged{
			ServiceID: id,
			Status:    StatusRunning,
			Health:    HealthHealthy,
			Detail:    "Service restarted",
			Timestamp: m.timeNow().UTC().Format(time.RFC3339),
		})
	}

	return &stCopy, nil
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

// Engine returns the runtime engine.
func (m *ServiceManager) Engine() RuntimeEngine {
	return m.engine
}

// JellyfinProvisioner returns the provisioner used for Jellyfin.
func (m *ServiceManager) JellyfinProvisioner() *JellyfinProvisioner {
	return m.jellyfinProvisioner
}

// AudioProvisioner returns the provisioner used for audio services.
func (m *ServiceManager) AudioProvisioner() *AudioProvisioner {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.audioProvisioner == nil {
		m.audioProvisioner = NewAudioProvisioner(m.dataDir, m.logger)
	}
	return m.audioProvisioner
}

// PaperlessProvisioner returns the provisioner used for Paperless-ngx.
func (m *ServiceManager) PaperlessProvisioner() *PaperlessProvisioner {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paperlessProvisioner == nil {
		m.paperlessProvisioner = NewPaperlessProvisioner(m.dataDir, m.logger)
	}
	return m.paperlessProvisioner
}

// RefreshService triggers a library scan or metadata refresh for the specified service.
func (m *ServiceManager) RefreshService(ctx context.Context, id string) error {
	m.mu.Lock()
	if m.audioProvisioner == nil {
		m.audioProvisioner = NewAudioProvisioner(m.dataDir, m.logger)
	}
	m.mu.Unlock()

	if id == "audio" {
		errNav := m.RefreshService(ctx, ServiceNavidrome)
		errAbs := m.RefreshService(ctx, ServiceAudiobookshelf)
		// Do not fail if at least one service was successfully refreshed
		if errNav == nil || errAbs == nil {
			return nil
		}
		return fmt.Errorf("audio bundle refresh failed: navidrome: %w, audiobookshelf: %v", errNav, errAbs)
	}

	id = m.resolveID(id)
	m.mu.RLock()
	_, exists := m.definitions[id]
	if !exists {
		m.mu.RUnlock()
		return ErrServiceNotFound
	}
	st, exists := m.statuses[id]
	if !exists || st.Status != StatusRunning {
		m.mu.RUnlock()
		return ErrServiceInactive
	}
	m.mu.RUnlock()

	switch id {
	case ServiceJellyfin:
		if m.jellyfinProvisioner != nil {
			return m.jellyfinProvisioner.TriggerLibraryRefresh(ctx)
		}
		return nil
	case ServiceNavidrome:
		if m.audioProvisioner != nil {
			return m.audioProvisioner.TriggerNavidromeRefresh(ctx)
		}
		return nil
	case ServiceAudiobookshelf:
		if m.audioProvisioner != nil {
			return m.audioProvisioner.TriggerAudiobookshelfRefresh(ctx)
		}
		return nil
	default:
		return fmt.Errorf("%w: service does not support library refresh", ErrInvalidOperation)
	}
}

// RegisterServiceDef registers a custom service definition.
func (m *ServiceManager) RegisterServiceDef(def ServiceDef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.definitions[def.ID] = def
	if _, exists := m.statuses[def.ID]; !exists {
		m.statuses[def.ID] = &ServiceStatus{
			ID:     def.ID,
			Name:   def.Name,
			Status: StatusInactive,
			Health: HealthUnknown,
		}
	}
}

// SetServiceStatusForTest overrides a service status for testing.
func (m *ServiceManager) SetServiceStatusForTest(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, exists := m.statuses[id]; exists {
		st.Status = status
	}
}

// SetServiceHealth updates a service's status and health, emitting service.health_changed if modified.
func (m *ServiceManager) SetServiceHealth(id, status, health, detail string) {
	id = m.resolveID(id)
	m.mu.Lock()
	st, exists := m.statuses[id]
	if !exists {
		m.mu.Unlock()
		return
	}
	if status != "" {
		st.Status = status
	}
	st.Health = health
	st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	currentStatus := st.Status
	m.mu.Unlock()

	if m.broadcaster != nil {
		m.broadcaster("service.health_changed", ServiceHealthChanged{
			ServiceID: id,
			Status:    currentStatus,
			Health:    health,
			Detail:    detail,
			Timestamp: m.timeNow().UTC().Format(time.RFC3339),
		})
	}
}

// Close stops background routines.
func (m *ServiceManager) Close() error {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		if m.cancelWatch != nil {
			m.cancelWatch()
		}
		m.mu.Unlock()
	})
	return nil
}
