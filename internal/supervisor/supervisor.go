package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sovereign/internal/store"
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
	StatusInactive     = "inactive"
	StatusStarting     = "starting"
	StatusRunning      = "running"
	StatusDegraded     = "degraded"
	StatusStopped      = "stopped"
	StatusError        = "error"
	StatusSafeRecovery = "safe_recovery"
)

// Service health states
const (
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
	HealthUnknown   = "unknown"
)

// Consumer reassurance messages
const (
	ReassuranceRollbackRestored = "Update couldn't start properly. We've safely restored your previous working version."
	ReassuranceSafeRecovery     = "Service update encountered recurring issues. Container safely paused in recovery mode."
)

var (
	ErrServiceNotFound        = errors.New("service not found")
	ErrPullFailed             = errors.New("image pull failed")
	ErrInactiveService        = errors.New("cannot restart inactive service; activate it first")
	ErrServiceInactive        = errors.New("service is inactive")
	ErrInvalidOperation       = errors.New("invalid operation")
	ErrNoPriorRevision        = errors.New("no prior revision found")
	ErrRollbackCircuitBreaker = errors.New("rollback circuit breaker tripped: maximum rollback attempts exceeded")
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
	ReassuranceMessage  string   `json:"reassurance_message,omitempty"`
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

// RevisionStore defines the interface needed by ServiceManager for revisioning, pre-upgrade snapshots, and rollback.
type RevisionStore interface {
	CreateSnapshot(ctx context.Context, destPath string) error
	RestoreSnapshot(ctx context.Context, snapshotPath string) error
	RecordRevision(ctx context.Context, rev *store.ComposeRevision) error
	GetRevision(ctx context.Context, revision int) (*store.ComposeRevision, error)
	GetActiveRevision(ctx context.Context) (*store.ComposeRevision, error)
	ListRevisions(ctx context.Context) ([]store.ComposeRevision, error)
	UpdateRevisionStatus(ctx context.Context, revision int, status string) error
	GetNextRevisionNumber(ctx context.Context) (int, error)
	SetActiveRevisionNumber(ctx context.Context, revision int) error
	GetActiveRevisionNumber(ctx context.Context) (int, error)
}

// AlertNotifier defines the interface for dispatching rollback alerts.
type AlertNotifier interface {
	SendRollbackAlert(ctx context.Context, serviceID, serviceName, failedVersion, restoredVersion, reason string) error
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
	LlamaProvisioner     *LlamaProvisioner
	OpenWebUIProvisioner *OpenWebUIProvisioner
	RevisionStore        RevisionStore
	Store                RevisionStore
	AlertNotifier        AlertNotifier
	VerificationEngine   *VerificationEngine
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
	llamaProvisioner     *LlamaProvisioner
	openwebuiProvisioner *OpenWebUIProvisioner
	revisionStore        RevisionStore
	alertNotifier        AlertNotifier
	verificationEngine   *VerificationEngine
	rollbackAttempts     map[string]int
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

	lp := cfg.LlamaProvisioner
	if lp == nil {
		lp = NewLlamaProvisioner(cfg.DataDir, cfg.Logger, LlamaProvisionerOptions{
			DevRoot:   cfg.HardwareOpts.DevRoot,
			SysfsRoot: cfg.HardwareOpts.SysfsRoot,
			ProcRoot:  cfg.HardwareOpts.ProcRoot,
		})
	}

	owp := cfg.OpenWebUIProvisioner
	if owp == nil {
		owp = NewOpenWebUIProvisioner(cfg.DataDir, cfg.Logger)
	}

	timeFunc := cfg.WatchdogConfig.TimeFunc
	if timeFunc == nil {
		timeFunc = time.Now
	}

	debouncer := NewWatchdogDebouncer(cfg.WatchdogConfig)

	// Canonical service definitions
	defs := DefaultServiceCatalog()

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

	rs := cfg.RevisionStore
	if rs == nil {
		rs = cfg.Store
	}

	sm := &ServiceManager{
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
		llamaProvisioner:     lp,
		openwebuiProvisioner: owp,
		revisionStore:        rs,
		alertNotifier:        cfg.AlertNotifier,
		rollbackAttempts:     make(map[string]int),
	}

	if cfg.VerificationEngine != nil {
		sm.verificationEngine = cfg.VerificationEngine
	} else {
		sm.verificationEngine = NewVerificationEngine(sm, DefaultVerificationConfig(), cfg.Logger)
	}

	return sm
}

// ResetRollbackAttempts clears rollback attempt counters for the specified service.
func (m *ServiceManager) ResetRollbackAttempts(serviceID string) {
	id := m.resolveID(serviceID)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rollbackAttempts != nil {
		delete(m.rollbackAttempts, id)
	}
}

// GetRollbackAttempts returns the current rollback attempt count for service.
func (m *ServiceManager) GetRollbackAttempts(serviceID string, revision int) int {
	id := m.resolveID(serviceID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rollbackAttempts == nil {
		return 0
	}
	return m.rollbackAttempts[id]
}

// AlertNotifier returns the registered AlertNotifier.
func (m *ServiceManager) AlertNotifier() AlertNotifier {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertNotifier
}

// SetAlertNotifier sets the alert notifier callback.
func (m *ServiceManager) SetAlertNotifier(notifier AlertNotifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertNotifier = notifier
}

// VerificationEngine returns the verification engine.
func (m *ServiceManager) VerificationEngine() *VerificationEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verificationEngine
}

// SetVerificationEngine sets the verification engine.
func (m *ServiceManager) SetVerificationEngine(v *VerificationEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verificationEngine = v
}

// RevisionStore returns the registered RevisionStore.
func (m *ServiceManager) RevisionStore() RevisionStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revisionStore
}

// SetRevisionStore updates the registered RevisionStore.
func (m *ServiceManager) SetRevisionStore(rs RevisionStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revisionStore = rs
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

	// Ordered list for deterministic responses: jellyfin, navidrome, audiobookshelf, paperless, llama, openwebui
	order := []string{ServiceJellyfin, ServiceNavidrome, ServiceAudiobookshelf, ServicePaperless, ServiceLlama, ServiceOpenWebUI}
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
	case "llama", "ai", "llm", "llama-server":
		return ServiceLlama
	case "chat", "open-webui", "openwebui":
		return ServiceOpenWebUI
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
	if m.llamaProvisioner == nil {
		m.llamaProvisioner = NewLlamaProvisioner(m.dataDir, m.logger)
	}
	if m.openwebuiProvisioner == nil {
		m.openwebuiProvisioner = NewOpenWebUIProvisioner(m.dataDir, m.logger)
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

	if id == "ai" {
		llamaSvc, err := m.ActivateService(ctx, ServiceLlama)
		if err != nil {
			return nil, fmt.Errorf("ai bundle failed to activate %s: %w", ServiceLlama, err)
		}
		if _, err := m.ActivateService(ctx, ServiceOpenWebUI); err != nil {
			m.logger.Warn("ai bundle failed to activate openwebui", "error", err)
		}
		return llamaSvc, nil
	}

	if id == ServiceOpenWebUI {
		// Ensure backend Llama is also active so Open WebUI does not fail inference requests
		if _, err := m.ActivateService(ctx, ServiceLlama); err != nil {
			m.logger.Warn("openwebui activation: backend llama activation encountered error", "error", err)
		}
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
	if st.Status == StatusRunning && !(id == ServiceLlama && st.Health == HealthDegraded) {
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

	// For Llama, probe models before image pull to gracefully enter Safe Stub Mode without pulling multi-GB image
	var preLlamaRes *LlamaProvisionResult
	if id == ServiceLlama && m.llamaProvisioner != nil {
		var err error
		preLlamaRes, err = m.llamaProvisioner.Provision(ctx)
		if err != nil {
			m.logger.Error("failed to provision llama specialized environment", "service", id, "error", err)
			emitError(fmt.Sprintf("Failed to provision %s: %v", id, err))
			return nil, fmt.Errorf("failed to provision %s: %w", id, err)
		}
		if preLlamaRes.InSafeStub {
			m.mu.Lock()
			st.Status = StatusRunning
			st.Health = HealthDegraded
			st.ImageDownloaded = false
			st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
			stCopy := *st
			m.mu.Unlock()

			m.debouncer.Reset(id)

			if m.broadcaster != nil {
				m.broadcaster("service.health_changed", ServiceHealthChanged{
					ServiceID: id,
					Status:    StatusRunning,
					Health:    HealthDegraded,
					Detail:    "Safe Stub Mode: No AI model provisioned in /data/models",
					Timestamp: m.timeNow().UTC().Format(time.RFC3339),
				})
			}

			m.logger.Info("service activated in safe stub mode (no model provisioned)", "service", id)
			return &stCopy, nil
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
	var containerSpec ContainerSpec

	if id == ServiceLlama && preLlamaRes != nil {
		containerSpec = preLlamaRes.ContainerSpec
	} else if id == ServiceOpenWebUI && m.openwebuiProvisioner != nil {
		webuiRes, err := m.openwebuiProvisioner.Provision(ctx)
		if err != nil {
			m.logger.Error("failed to provision openwebui specialized environment", "service", id, "error", err)
			emitError(fmt.Sprintf("Failed to provision %s: %v", id, err))
			return nil, fmt.Errorf("failed to provision %s: %w", id, err)
		}
		containerSpec = webuiRes.ContainerSpec
	} else {
		containerSpec = ContainerSpec{
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

	if id == "ai" {
		_, _ = m.DeactivateService(ctx, ServiceOpenWebUI)
		return m.DeactivateService(ctx, ServiceLlama)
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

	if id == "ai" {
		llamaSt, errLlama := m.RestartService(ctx, ServiceLlama)
		if errLlama != nil {
			return nil, errLlama
		}
		_, _ = m.RestartService(ctx, ServiceOpenWebUI)
		return llamaSt, nil
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

	if id == ServiceLlama && m.llamaProvisioner != nil {
		llamaRes, err := m.llamaProvisioner.Provision(ctx)
		if err == nil && llamaRes != nil && llamaRes.InSafeStub {
			m.debouncer.Reset(id)
			m.mu.Lock()
			st.Status = StatusRunning
			st.Health = HealthDegraded
			st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
			stCopy := *st
			m.mu.Unlock()
			return &stCopy, nil
		}
		if err == nil && llamaRes != nil && !llamaRes.InSafeStub {
			_, _ = SynthesizeComposeSpec(m.dataDir, llamaRes.ContainerSpec)
			if present, _ := m.engine.IsImagePresent(ctx, llamaRes.ContainerSpec.Image); !present {
				_ = m.engine.PullImage(ctx, llamaRes.ContainerSpec.Image, nil)
			}
			if err := m.engine.RestartContainer(ctx, id); err != nil {
				if startErr := m.engine.StartContainer(ctx, llamaRes.ContainerSpec); startErr != nil {
					m.logger.Error("failed to start container on restart", "service", id, "error", startErr)
					return nil, fmt.Errorf("failed to restart service: %w", startErr)
				}
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
	}

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

// OpenWebUIProvisioner returns the provisioner used for Open WebUI.
func (m *ServiceManager) OpenWebUIProvisioner() *OpenWebUIProvisioner {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.openwebuiProvisioner == nil {
		m.openwebuiProvisioner = NewOpenWebUIProvisioner(m.dataDir, m.logger)
	}
	return m.openwebuiProvisioner
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
		ve := m.verificationEngine
		m.mu.Unlock()
		if ve != nil {
			ve.Close()
		}
	})
	return nil
}
