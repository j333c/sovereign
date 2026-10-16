package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sovereign/internal/auth"
	"sovereign/internal/hardware"
	"sovereign/internal/ingress"
	"sovereign/internal/optical"
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
)

const (
	AppVersion = "0.1.0"
)

// OpticalEjectResponse matches the OpenAPI 3.1 contract.
type OpticalEjectResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// OpticalKeysUploadResponse matches the OpenAPI 3.1 contract.
type OpticalKeysUploadResponse struct {
	Status    string `json:"status"`
	KeysCount int    `json:"keys_count"`
	Message   string `json:"message"`
}

// TelemetryUpdate matches the OpenAPI 3.1 real-time telemetry schema.
type TelemetryUpdate struct {
	CPUPercent        float64 `json:"cpu_percent"`
	MemoryUsedBytes   uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes  uint64  `json:"memory_total_bytes"`
	StorageUsedBytes  uint64  `json:"storage_used_bytes"`
	StorageTotalBytes uint64  `json:"storage_total_bytes"`
	StoragePercent    float64 `json:"storage_percent"`
	NASStatus         string  `json:"nas_status"`
	NASMessage        string  `json:"nas_message"`
	OverallStatus     string  `json:"overall_status"`
	OpticalStatus     string  `json:"optical_status"`
	GPUStatus         string  `json:"gpu_status"`
	Timestamp         string  `json:"timestamp"`
}

// ProblemDetails implements RFC 7807 error responses.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// HealthResponse matches the OpenAPI 3.1 contract.
type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

// IngressRoute matches the OpenAPI 3.1 contract.
type IngressRoute struct {
	Service  string `json:"service"`
	Subpath  string `json:"subpath"`
	Upstream string `json:"upstream"`
}

// IngressStatusResponse matches the OpenAPI 3.1 contract.
type IngressStatusResponse struct {
	Status         string         `json:"status"`
	CaddyConnected bool           `json:"caddy_connected"`
	MDNSActive     bool           `json:"mdns_active"`
	Hostname       string         `json:"hostname"`
	Routes         []IngressRoute `json:"routes"`
}

// Config specifies runtime options for the daemon server.
type Config struct {
	ListenAddr        string
	DataDir           string
	DBPath            string
	StatePath         string
	CaddyURL          string
	Logger            *slog.Logger
	IngressMgr        ingress.IngressManager
	MDNSService       *ingress.MDNSService
	BootMgr           *store.BootManager
	StorageEngine     *storage.Engine
	AuthMgr           *auth.Manager
	HardwareCollector hardware.Collector
	TelemetryInterval time.Duration
	ServiceMgr        *supervisor.ServiceManager
	DropzoneEngine    *storage.DropzoneEngine
	OpticalDriver     optical.OpticalDriver
	OpticalRipper     *optical.OpticalRipper
	KEYDBManager      *optical.KEYDBManager
}

type sseEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Server is the appliance core supervisor HTTP daemon.
type Server struct {
	cfg               Config
	db                *store.DB
	bootMgr           *store.BootManager
	storageEngine     *storage.Engine
	authMgr           *auth.Manager
	hardwareCollector hardware.Collector
	telemetryInterval time.Duration
	logger            *slog.Logger
	router            *http.ServeMux
	ingressMgr        ingress.IngressManager
	mdnsService       *ingress.MDNSService
	serviceMgr        *supervisor.ServiceManager
	dropzoneEngine    *storage.DropzoneEngine
	opticalRipper     *optical.OpticalRipper
	stopChan          chan struct{}
	closeOnce         sync.Once
	mu                sync.RWMutex

	eventsMu         sync.RWMutex
	eventSubscribers map[chan sseEvent]struct{}
}


// NewServer initializes the supervisor server, ensures storage directories,
// connects to SQLite WAL, and initializes ingress/mDNS managers.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}

	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.DataDir, "state", "sovereign.db")
	}

	if cfg.CaddyURL == "" {
		cfg.CaddyURL = os.Getenv("SOVEREIGN_CADDY_URL")
		if cfg.CaddyURL == "" {
			cfg.CaddyURL = "http://caddy:2019"
		}
	}

	// Ensure required storage layout: /data/{media,documents,models,state,drop}
	cfg.Logger.Info("verifying appliance storage layout", "data_dir", cfg.DataDir)
	if err := storage.EnsureDataLayout(cfg.DataDir); err != nil {
		cfg.Logger.Error("failed to initialize storage directories", "error", err, "data_dir", cfg.DataDir)
		return nil, fmt.Errorf("storage layout initialization failed: %w", err)
	}

	// Initialize SQLite WAL database
	cfg.Logger.Info("initializing SQLite WAL persistence", "db_path", cfg.DBPath)
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		cfg.Logger.Error("database initialization failed", "error", err, "db_path", cfg.DBPath)
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}

	if cfg.StatePath == "" {
		cfg.StatePath = filepath.Join(cfg.DataDir, "state", "boot_state.json")
	}

	if cfg.BootMgr == nil {
		cfg.BootMgr = store.NewBootManager(cfg.StatePath, db, cfg.Logger)
		if _, err := cfg.BootMgr.RecordBoot(context.Background()); err != nil {
			cfg.Logger.Warn("recording boot state resulted in non-fatal error", "error", err)
		}
	}

	if cfg.IngressMgr == nil {
		cfg.IngressMgr = ingress.NewCaddyClient(cfg.CaddyURL, cfg.Logger)
	}

	if cfg.MDNSService == nil {
		cfg.MDNSService = ingress.NewMDNSService(ingress.DefaultMDNSConfig(cfg.Logger))
	}

	if cfg.StorageEngine == nil {
		var err error
		cfg.StorageEngine, err = storage.NewEngine(storage.EngineConfig{
			DataDir: cfg.DataDir,
			DB:      db,
			Logger:  cfg.Logger,
		})
		if err != nil {
			cfg.Logger.Error("storage engine initialization failed", "error", err)
			return nil, fmt.Errorf("storage engine initialization failed: %w", err)
		}
	}

	if cfg.AuthMgr == nil {
		var err error
		cfg.AuthMgr, err = auth.NewManager(db.DB, cfg.Logger)
		if err != nil {
			cfg.Logger.Error("auth manager initialization failed", "error", err)
			return nil, fmt.Errorf("auth manager initialization failed: %w", err)
		}
	}

	if cfg.HardwareCollector == nil {
		cfg.HardwareCollector = hardware.NewSystemCollector()
	}

	if cfg.TelemetryInterval <= 0 {
		cfg.TelemetryInterval = 5 * time.Second
	}

	s := &Server{
		cfg:               cfg,
		db:                db,
		bootMgr:           cfg.BootMgr,
		storageEngine:     cfg.StorageEngine,
		authMgr:           cfg.AuthMgr,
		hardwareCollector: cfg.HardwareCollector,
		telemetryInterval: cfg.TelemetryInterval,
		logger:            cfg.Logger,
		router:            http.NewServeMux(),
		ingressMgr:        cfg.IngressMgr,
		mdnsService:       cfg.MDNSService,
		stopChan:          make(chan struct{}),
		eventSubscribers:  make(map[chan sseEvent]struct{}),
	}

	if cfg.ServiceMgr == nil {
		var engine supervisor.RuntimeEngine
		sockPath := "/var/run/docker.sock"
		if _, err := os.Stat(sockPath); err == nil {
			engine = supervisor.NewDockerEngine(sockPath, cfg.Logger)
		} else {
			engine = supervisor.NewMockRuntimeEngine()
		}

		s.serviceMgr = supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
			DataDir:          cfg.DataDir,
			Engine:           engine,
			Logger:           cfg.Logger,
			EventBroadcaster: s.BroadcastEvent,
		})
	} else {
		s.serviceMgr = cfg.ServiceMgr
		s.serviceMgr.SetEventBroadcaster(s.BroadcastEvent)
	}

	if cfg.DropzoneEngine == nil {
		var err error
		cfg.DropzoneEngine, err = storage.NewDropzoneEngine(storage.DropzoneConfig{
			DataDir:   cfg.DataDir,
			Logger:    cfg.Logger,
			Refresher: s.serviceMgr,
		})
		if err != nil {
			cfg.Logger.Error("dropzone engine initialization failed", "error", err)
			return nil, fmt.Errorf("dropzone engine initialization failed: %w", err)
		}
	} else if s.serviceMgr != nil {
		cfg.DropzoneEngine.SetRefresher(s.serviceMgr)
	}
	s.dropzoneEngine = cfg.DropzoneEngine

	if cfg.KEYDBManager == nil {
		keysPath := filepath.Join(cfg.DataDir, "state", "keys", "KEYDB.cfg")
		cfg.KEYDBManager = optical.NewKEYDBManager(keysPath, cfg.Logger)
	}

	if cfg.OpticalRipper == nil {
		if cfg.OpticalDriver == nil {
			cfg.OpticalDriver = optical.NewLinuxOpticalDriver("/dev/sr0", cfg.Logger)
		}
		cfg.OpticalRipper = optical.NewOpticalRipper(optical.RipperConfig{
			DataDir:          cfg.DataDir,
			Driver:           cfg.OpticalDriver,
			KeyMgr:           cfg.KEYDBManager,
			EventBroadcaster: s.BroadcastEvent,
			Logger:           cfg.Logger,
		})
	} else {
		cfg.OpticalRipper.SetEventBroadcaster(s.BroadcastEvent)
	}
	s.opticalRipper = cfg.OpticalRipper

	s.routes()
	return s, nil
}


// StartBackgroundServices initializes background supervisor routines:
// starts mDNS responder broadcasting sovereign.local, and launches
// background route sync loop with Caddy.
func (s *Server) StartBackgroundServices(ctx context.Context) {
	// 1. Start mDNS responder
	if s.mdnsService != nil {
		go func() {
			if err := s.mdnsService.Start(); err != nil {
				s.logger.Warn("mDNS discovery responder failed to start", "error", err)
			}
		}()
	}

	// 2. Initial Caddy route synchronization with retry in background
	if s.ingressMgr != nil {
		go func() {
			s.logger.Info("starting background caddy route synchronization")
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			// Try initial sync immediately
			syncCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := s.syncIngressRoutes(syncCtx)
			cancel()
			if err == nil {
				s.logger.Info("initial caddy route synchronization succeeded")
				return
			}

			// Retry until successful or context cancelled
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					retryCtx, retryCancel := context.WithTimeout(ctx, 3*time.Second)
					err := s.syncIngressRoutes(retryCtx)
					retryCancel()
					if err == nil {
						s.logger.Info("caddy route synchronization succeeded after retry")
						return
					}
					s.logger.Debug("retrying caddy route sync...", "error", err)
				}
			}
		}()
	}

	// 3. Automatic integrity check trigger during diagnostics mode
	if s.bootMgr != nil && s.bootMgr.IsDiagnosticsMode() {
		go func() {
			s.logger.Info("unclean shutdown detected: initializing in diagnostics mode and running integrity checks")
			diagCtx, diagCancel := context.WithTimeout(ctx, 15*time.Second)
			defer diagCancel()
			if err := s.bootMgr.RunIntegrityCheck(diagCtx); err != nil {
				s.logger.Error("diagnostics integrity check failed", "error", err)
			} else {
				s.logger.Info("boot diagnostics integrity check succeeded, system operational")
			}
		}()
	}

	// 4. Background storage health probe
	if s.storageEngine != nil {
		s.storageEngine.StartProbing(ctx)
	}

	// 5. Prune expired auth sessions on startup
	if s.authMgr != nil {
		if err := s.authMgr.PruneExpiredSessions(ctx); err != nil {
			s.logger.Warn("failed to prune expired sessions on startup", "error", err)
		}
	}

	// 6. Start service watchdog probing
	if s.serviceMgr != nil {
		s.serviceMgr.StartWatchdog(ctx)
	}

	// 7. Start dropzone background folder scanner
	if s.dropzoneEngine != nil {
		s.dropzoneEngine.Start(ctx)
	}

	// 8. Start optical ripper hardware supervisor loop
	if s.opticalRipper != nil {
		s.opticalRipper.Start(ctx)
	}
}

// BootManager returns the active BootManager instance.
func (s *Server) BootManager() *store.BootManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bootMgr
}

// StorageEngine returns the active storage engine.
func (s *Server) StorageEngine() *storage.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storageEngine
}

// AuthManager returns the active auth manager.
func (s *Server) AuthManager() *auth.Manager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authMgr
}

// HardwareCollector returns the active hardware collector.
func (s *Server) HardwareCollector() hardware.Collector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hardwareCollector
}

// ServiceManager returns the active ServiceManager instance.
func (s *Server) ServiceManager() *supervisor.ServiceManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serviceMgr
}

// DropzoneEngine returns the active DropzoneEngine instance.
func (s *Server) DropzoneEngine() *storage.DropzoneEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dropzoneEngine
}

// OpticalRipper returns the active OpticalRipper instance.
func (s *Server) OpticalRipper() *optical.OpticalRipper {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.opticalRipper
}

// BroadcastEvent emits an SSE event to all actively connected SSE clients.
func (s *Server) BroadcastEvent(event string, data any) {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()

	evt := sseEvent{Event: event, Data: data}
	for ch := range s.eventSubscribers {
		select {
		case ch <- evt:
		default:
			// Non-blocking drop if client channel buffer is full
		}
	}
}

// Close closes database handles and frees ingress/mDNS/storage/supervisor resources.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opticalRipper != nil {
		_ = s.opticalRipper.Close()
	}

	if s.dropzoneEngine != nil {
		_ = s.dropzoneEngine.Close()
	}

	if s.serviceMgr != nil {
		_ = s.serviceMgr.Close()
	}

	if s.storageEngine != nil {
		_ = s.storageEngine.Close()
	}

	if s.mdnsService != nil {
		s.mdnsService.Close()
	}

	if s.bootMgr != nil {
		_ = s.bootMgr.Close()
	}

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// requireSetupOrAdmin allows unauthenticated access during first-run setup, but strictly requires
// admin authentication once the appliance setup has been completed.
func (s *Server) requireSetupOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		authMgr := s.authMgr
		s.mu.RUnlock()

		if authMgr != nil {
			configured, err := authMgr.IsConfigured(r.Context())
			if err == nil && configured {
				authMgr.RequireAdmin(http.HandlerFunc(next)).ServeHTTP(w, r)
				return
			}
		}
		next(w, r)
	}
}

// routes registers HTTP REST handlers.
func (s *Server) routes() {
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/system/boot-status", s.handleBootStatus)
	s.router.HandleFunc("GET /api/v1/system/ingress", s.handleIngressStatus)
	s.router.HandleFunc("GET /api/v1/system/events", s.handleEvents)
	s.router.HandleFunc("GET /api/events", s.handleEvents)
	s.router.HandleFunc("GET /api/v1/storage/disks", s.handleStorageDisks)
	s.router.HandleFunc("POST /api/v1/storage/initialize", s.requireSetupOrAdmin(s.handleStorageInitialize))
	s.router.HandleFunc("POST /api/v1/storage/nas/test", s.requireSetupOrAdmin(s.handleStorageNASTest))
	s.router.HandleFunc("POST /api/v1/storage/nas/mount", s.requireSetupOrAdmin(s.handleStorageNASMount))
	s.router.HandleFunc("GET /api/v1/storage/status", s.handleStorageStatus)

	// Auth and Setup Wizard endpoints
	s.router.HandleFunc("GET /api/v1/auth/setup-status", s.handleSetupStatus)
	s.router.HandleFunc("POST /api/v1/auth/setup", s.handleSetup)
	s.router.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.router.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.router.HandleFunc("GET /api/v1/auth/session", s.handleSession)

	// Protected administrative settings endpoints
	s.router.Handle("GET /api/v1/settings", s.authMgr.RequireAdmin(http.HandlerFunc(s.handleGetSettings)))
	s.router.Handle("POST /api/v1/settings", s.authMgr.RequireAdmin(http.HandlerFunc(s.handleUpdateSettings)))

	// Managed Services lifecycle endpoints
	s.router.HandleFunc("GET /api/v1/services", s.handleListServices)
	s.router.HandleFunc("GET /api/v1/services/{id}", s.handleGetService)
	s.router.HandleFunc("POST /api/v1/services/{id}/activate", s.handleActivateService)
	s.router.HandleFunc("POST /api/v1/services/{id}/deactivate", s.handleDeactivateService)
	s.router.HandleFunc("POST /api/v1/services/{id}/restart", s.requireSetupOrAdmin(s.handleRestartService))
	s.router.HandleFunc("POST /api/v1/services/{id}/refresh", s.handleRefreshService)

	// Media Dropzone ingestion endpoints
	s.router.HandleFunc("POST /api/v1/drop/upload", s.handleDropUpload)
	s.router.HandleFunc("GET /api/v1/drop/history", s.handleDropHistory)

	// Optical drive and automated disc ripping endpoints
	s.router.HandleFunc("GET /api/v1/optical/status", s.handleOpticalStatus)
	s.router.HandleFunc("POST /api/v1/optical/eject", s.handleOpticalEject)
	s.router.HandleFunc("POST /api/v1/optical/keys", s.requireSetupOrAdmin(s.handleOpticalKeys))
}


// isAllowedOrigin checks if the origin is trusted (localhost, 127.0.0.1, sovereign.local, or matching host/LAN IP).
func isAllowedOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	if originHost == "localhost" || originHost == "127.0.0.1" || originHost == "sovereign.local" {
		return true
	}
	reqHost := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		reqHost = h
	}
	if strings.EqualFold(originHost, reqHost) {
		return true
	}
	ip := net.ParseIP(originHost)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		return true
	}
	return false
}

// Handler returns the HTTP handler for the server with CORS middleware.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		s.router.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	prob := ProblemDetails{
		Type:     fmt.Sprintf("https://sovereign.local/errors/http-%d", status),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
	_ = json.NewEncoder(w).Encode(prob)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Database is not initialized", r.URL.Path)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := dbHandle.PingContext(ctx); err != nil {
		s.logger.Error("health probe database check failed", "error", err)
		s.writeProblem(w, http.StatusServiceUnavailable, "Database Unavailable", "Database ping probe failed", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:   "ok",
		Version:  AppVersion,
		Database: "connected",
	})
}

func (s *Server) handleBootStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	bootMgr := s.bootMgr
	s.mu.RUnlock()

	if bootMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Boot manager is not initialized", r.URL.Path)
		return
	}

	status, degraded, err := bootMgr.GetStatus()
	if degraded {
		s.writeProblem(w, http.StatusServiceUnavailable, "Storage Degraded", "State storage filesystem is unwritable or read-only", r.URL.Path)
		return
	}
	if err != nil {
		s.logger.Error("retrieving boot status failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve boot status record", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleIngressStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	s.mu.RLock()
	ingressMgr := s.ingressMgr
	mdnsSvc := s.mdnsService
	s.mu.RUnlock()

	caddyConnected := false
	if ingressMgr != nil {
		caddyStatus, err := ingressMgr.GetStatus(ctx)
		if err == nil {
			if conn, ok := caddyStatus["connected"].(bool); ok && conn {
				caddyConnected = true
			}
		}
	}

	mdnsActive := false
	hostname := "sovereign.local"
	if mdnsSvc != nil {
		mdnsActive = mdnsSvc.IsActive()
		hostname = mdnsSvc.Hostname()
	}

	overallStatus := "ok"
	if !caddyConnected {
		overallStatus = "error"
	} else if !mdnsActive {
		overallStatus = "degraded"
	}

	routes := []IngressRoute{
		{Service: "web", Subpath: "/", Upstream: "sovereign-web:3000"},
		{Service: "web", Subpath: "/drop", Upstream: "sovereign-web:3000"},
		{Service: "daemon", Subpath: "/api/*", Upstream: "sovereign-daemon:8080"},
	}

	if s.serviceMgr != nil {
		if svcs, err := s.serviceMgr.ListServices(ctx); err == nil {
			for _, svc := range svcs {
				if svc.Status == supervisor.StatusRunning {
					routes = append(routes, IngressRoute{
						Service:  svc.ID,
						Subpath:  svc.Subpath,
						Upstream: fmt.Sprintf("sovereign-%s:%d", svc.ID, svc.Port),
					})
				}
			}
		}
	}

	s.writeJSON(w, http.StatusOK, IngressStatusResponse{
		Status:         overallStatus,
		CaddyConnected: caddyConnected,
		MDNSActive:     mdnsActive,
		Hostname:       hostname,
		Routes:         routes,
	})
}

func (s *Server) getTelemetryUpdate(ctx context.Context) TelemetryUpdate {
	s.mu.RLock()
	hwCollector := s.hardwareCollector
	storageEng := s.storageEngine
	ripper := s.opticalRipper
	s.mu.RUnlock()

	var cpuPercent float64 = 5.0
	var memUsed, memTotal uint64
	var hwStorageUsed, hwStorageTotal uint64
	if hwCollector != nil {
		if snap, err := hwCollector.CollectSnapshot(ctx); err == nil && snap != nil {
			cpuPercent = snap.CPUUsagePercent
			memUsed = snap.MemoryUsedBytes
			memTotal = snap.MemoryTotalBytes
			hwStorageUsed = snap.StorageUsedBytes
			hwStorageTotal = snap.StorageTotalBytes
		}
	}

	var storageUsed, storageTotal uint64
	var storagePercent float64
	nasStatus := "not_configured"
	nasMessage := "No network storage configured"
	overallStatus := "operational"

	if storageEng != nil {
		st := storageEng.GetStorageStatus(ctx)
		storageUsed = st.UsedBytes
		storageTotal = st.TotalBytes
		if storageTotal == 0 && hwStorageTotal > 0 {
			storageUsed = hwStorageUsed
			storageTotal = hwStorageTotal
		}
		if storageTotal > 0 {
			storagePercent = math.Round((float64(storageUsed)/float64(storageTotal))*1000) / 10
		}
		if st.NAS != nil && st.NAS.Configured {
			nasStatus = st.NAS.Status
			nasMessage = st.NAS.Message
		}
		if st.Status == "error" || nasStatus == "error" {
			overallStatus = "error"
		} else if st.Status == "degraded" || nasStatus == "degraded" || nasStatus == "disconnected" {
			overallStatus = "degraded"
		}
	} else if hwStorageTotal > 0 {
		storageUsed = hwStorageUsed
		storageTotal = hwStorageTotal
		storagePercent = math.Round((float64(storageUsed)/float64(storageTotal))*1000) / 10
	}

	opticalStatus := "Idle · Ready for disc"
	if ripper != nil {
		opticalStatus = ripper.GetStatus().StatusText
	}

	return TelemetryUpdate{
		CPUPercent:        cpuPercent,
		MemoryUsedBytes:   memUsed,
		MemoryTotalBytes:  memTotal,
		StorageUsedBytes:  storageUsed,
		StorageTotalBytes: storageTotal,
		StoragePercent:    storagePercent,
		NASStatus:         nasStatus,
		NASMessage:        nasMessage,
		OverallStatus:     overallStatus,
		OpticalStatus:     opticalStatus,
		GPUStatus:         "Daytime (AI Priority)",
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	eventCh := make(chan sseEvent, 64)
	s.eventsMu.Lock()
	s.eventSubscribers[eventCh] = struct{}{}
	s.eventsMu.Unlock()

	defer func() {
		s.eventsMu.Lock()
		delete(s.eventSubscribers, eventCh)
		s.eventsMu.Unlock()
	}()

	// Push initial event immediately on connection
	initEvt := s.getTelemetryUpdate(r.Context())
	if data, err := json.Marshal(initEvt); err == nil {
		if _, err := fmt.Fprintf(w, "event: telemetry.update\ndata: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
	}

	interval := s.telemetryInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopChan:
			return
		case evt := <-eventCh:
			data, err := json.Marshal(evt.Data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			evt := s.getTelemetryUpdate(r.Context())
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: telemetry.update\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleStorageDisks(w http.ResponseWriter, r *http.Request) {

	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	disks, err := storageEng.ListAvailableDisks(r.Context())
	if err != nil {
		s.logger.Error("failed to list available disks", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Error", "Failed to enumerate block devices", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"disks": disks,
	})
}

func (s *Server) handleStorageInitialize(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Device) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Device", "Device path cannot be empty", r.URL.Path)
		return
	}

	res, err := storageEng.InitializeStorage(r.Context(), req.Device)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidDevice) ||
			errors.Is(err, storage.ErrSystemDeviceProtected) ||
			errors.Is(err, storage.ErrDeviceInUse) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Device", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("storage initialization failed", "device", req.Device, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Initialization Failed", "Failed to format and mount storage pool", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageNASTest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req storage.NASTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	res, err := storageEng.TestNAS(r.Context(), req)
	if err != nil {
		s.logger.Warn("NAS test connection failed", "host", req.Host, "error", err)
		s.writeProblem(w, http.StatusBadRequest, "NAS Connection Failed", err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageNASMount(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	var req storage.NASMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	res, err := storageEng.MountNAS(r.Context(), req)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidNASConfig) ||
			errors.Is(err, storage.ErrNASAuthFailed) ||
			errors.Is(err, storage.ErrNASUnreachable) {
			s.writeProblem(w, http.StatusBadRequest, "NAS Configuration Error", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("NAS mount failed", "host", req.Host, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Mount Failed", "Failed to mount network storage share", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	storageEng := s.storageEngine
	s.mu.RUnlock()

	if storageEng == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Storage engine is not initialized", r.URL.Path)
		return
	}

	status := storageEng.GetStorageStatus(r.Context())
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	dbHandle := s.db
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	configured, err := authMgr.IsConfigured(r.Context())
	if err != nil {
		s.logger.Error("failed to check setup status", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to check setup configuration status", r.URL.Path)
		return
	}

	res := map[string]any{
		"is_configured": configured,
	}
	if configured && dbHandle != nil {
		if lang, err := dbHandle.GetSetting(r.Context(), "language"); err == nil && lang != "" {
			res["language"] = lang
		}
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	var req auth.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	sess, err := authMgr.CompleteSetup(r.Context(), req)
	if err != nil {
		if errors.Is(err, auth.ErrAlreadyConfigured) {
			s.writeProblem(w, http.StatusConflict, "Already Configured", "Appliance first-run setup has already been completed", r.URL.Path)
			return
		}
		if errors.Is(err, auth.ErrPasswordTooShort) || errors.Is(err, auth.ErrInvalidEmail) {
			s.writeProblem(w, http.StatusBadRequest, "Validation Error", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("setup completion failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Setup Failed", "Failed to complete appliance setup", r.URL.Path)
		return
	}

	authMgr.SetSessionCookie(w, sess.Token)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"is_configured": true,
		"session":       sess,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	sess, err := authMgr.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			s.writeProblem(w, http.StatusUnauthorized, "Unauthorized", "Invalid username or password", r.URL.Path)
			return
		}
		s.logger.Error("login authentication failed", "username", req.Username, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Authentication Error", "Authentication failed", r.URL.Path)
		return
	}

	authMgr.SetSessionCookie(w, sess.Token)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"session": sess,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr != nil {
		if sess, err := authMgr.GetSessionFromRequest(r); err == nil && sess != nil {
			_ = authMgr.RevokeSession(r.Context(), sess.Token)
		}
		authMgr.ClearSessionCookie(w)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "Logged out successfully",
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	authMgr := s.authMgr
	s.mu.RUnlock()

	if authMgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Auth manager is not initialized", r.URL.Path)
		return
	}

	sess, err := authMgr.GetSessionFromRequest(r)
	if err != nil || sess == nil {
		s.writeProblem(w, http.StatusUnauthorized, "Unauthorized", "No active session", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"session":       sess,
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database is not initialized", r.URL.Path)
		return
	}

	ctx := r.Context()
	lang, _ := dbHandle.GetSetting(ctx, "language")
	email, _ := dbHandle.GetSetting(ctx, "admin_email")
	hostname, _ := dbHandle.GetSetting(ctx, "hostname")
	features, _ := dbHandle.GetSetting(ctx, "features")

	s.writeJSON(w, http.StatusOK, map[string]any{
		"language":    lang,
		"admin_email": email,
		"hostname":    hostname,
		"features":    features,
	})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	dbHandle := s.db
	s.mu.RUnlock()

	if dbHandle == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database is not initialized", r.URL.Path)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	ctx := r.Context()
	for k, v := range payload {
		if err := dbHandle.SetSetting(ctx, k, v); err != nil {
			s.logger.Error("failed to set setting", "key", k, "error", err)
			s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update setting", r.URL.Path)
			return
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func extractServiceID(r *http.Request, action string) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	// Fallback parsing for /api/v1/services/{id}/{action} or /api/v1/services/{id}
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "services" && i+1 < len(parts) {
			val := parts[i+1]
			if val != action {
				return val
			}
		}
	}
	return ""
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	services, err := mgr.ListServices(r.Context())
	if err != nil {
		s.logger.Error("failed to list services", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve service list", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
	})
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to get service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to get service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleActivateService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "activate")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.ActivateService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrPullFailed) {
			s.writeProblem(w, http.StatusBadGateway, "Image Pull Failed", fmt.Sprintf("Failed to pull OCI image for '%s': %v", id, err), r.URL.Path)
			return
		}
		s.logger.Error("failed to activate service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to activate service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleDeactivateService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "deactivate")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.DeactivateService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to deactivate service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to deactivate service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "restart")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	svc, err := mgr.RestartService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrInactiveService) || strings.Contains(err.Error(), "cannot restart inactive service") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to restart service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to restart service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleRefreshService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "refresh")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	err := mgr.RefreshService(r.Context(), id)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrInvalidOperation) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Operation", err.Error(), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrServiceInactive) || errors.Is(err, supervisor.ErrInactiveService) {
			s.writeProblem(w, http.StatusServiceUnavailable, "Service Inactive", fmt.Sprintf("Service '%s' is not active", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to refresh service library", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Refresh Failed", fmt.Sprintf("Failed to refresh service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   id,
		"refreshed": true,
	})
}

// handleDropUpload handles multipart file uploads to /api/v1/drop/upload.
// Streams file into atomic staging path, validates magic bytes, moves to /data/media/:category,
// triggers library refresh within 5s, and returns 200 with metadata.
func (s *Server) handleDropUpload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.dropzoneEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Dropzone engine is not initialized", r.URL.Path)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Failed to parse multipart/form-data upload: "+err.Error(), r.URL.Path)
		return
	}

	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	if category == "" {
		category = strings.ToLower(strings.TrimSpace(r.Header.Get("X-Media-Category")))
	}

	var uploadedItem *storage.DropHistoryItem
	var uploadErr error
	var fileProcessed bool

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(r.Context().Err(), context.Canceled) {
				s.writeProblem(w, 499, "Upload Aborted", "Client closed connection during upload stream", r.URL.Path)
				return
			}
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Failed reading multipart part: "+err.Error(), r.URL.Path)
			return
		}

		formName := part.FormName()
		if formName == "category" {
			catBytes, _ := io.ReadAll(part)
			_ = part.Close()
			category = strings.ToLower(strings.TrimSpace(string(catBytes)))
			continue
		}

		if formName == "file" {
			filename := part.FileName()
			if category == "" || !storage.IsValidCategory(category) {
				_ = part.Close()
				s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks", r.URL.Path)
				return
			}

			fileProcessed = true
			uploadedItem, uploadErr = engine.HandleUploadStream(r.Context(), category, filename, part)
			_ = part.Close()
			break
		}
		_ = part.Close()
	}

	if !fileProcessed {
		s.writeProblem(w, http.StatusBadRequest, "Missing File", "Multipart field 'file' is required", r.URL.Path)
		return
	}

	if uploadErr != nil {
		if errors.Is(uploadErr, storage.ErrInvalidCategory) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrInvalidMedia) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Invalid Media Format", "Only media files are accepted. Please upload video or audio files.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrCategoryMismatch) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Category Mismatch", "The uploaded file format does not match the selected category.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrEmptyFile) {
			s.writeProblem(w, http.StatusUnprocessableEntity, "Empty File", "The uploaded file is empty.", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, context.Canceled) || errors.Is(uploadErr, storage.ErrStagingAborted) || errors.Is(r.Context().Err(), context.Canceled) {
			s.writeProblem(w, 499, "Upload Aborted", "Client closed connection during upload stream", r.URL.Path)
			return
		}
		s.logger.Error("dropzone upload processing failed", "category", category, "error", uploadErr)
		s.writeProblem(w, http.StatusInternalServerError, "Upload Failed", "An error occurred while processing the media upload.", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, uploadedItem)
}

// handleDropHistory returns recent ingestion and promotion history.
func (s *Server) handleDropHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.dropzoneEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}

	items := engine.GetHistory()
	if items == nil {
		items = []storage.DropHistoryItem{}
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *Server) syncIngressRoutes(ctx context.Context) error {
	s.mu.RLock()
	ingressMgr := s.ingressMgr
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if ingressMgr == nil {
		return nil
	}

	var routes []ingress.RouteConfig
	if mgr != nil {
		services, err := mgr.ListServices(ctx)
		if err != nil {
			s.logger.Warn("failed to list services for ingress route sync", "error", err)
			return err
		}

		for _, svc := range services {
			if svc.Status == supervisor.StatusRunning {
				routes = append(routes, ingress.RouteConfig{
					Service: svc.ID,
					Port:    svc.Port,
					Subpath: svc.Subpath,
				})
			}
		}
	}

	syncCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := ingressMgr.SyncRoutes(syncCtx, routes); err != nil {
		s.logger.Warn("failed to sync ingress routes with caddy", "error", err)
		return err
	}
	return nil
}

// handleOpticalStatus returns current optical hardware and disc ripping state.
func (s *Server) handleOpticalStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	status := ripper.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

// handleOpticalEject issues a hardware eject request to open the optical drive tray.
func (s *Server) handleOpticalEject(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	if err := ripper.Eject(r.Context()); err != nil {
		s.logger.Error("failed to eject optical drive", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Eject Failed", "Failed to eject optical drive: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, OpticalEjectResponse{
		Status:  "ejected",
		Message: "Disc ejected successfully",
	})
}

// handleOpticalKeys handles validation and atomic storage of KEYDB.cfg for Blu-ray decryption.
func (s *Server) handleOpticalKeys(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ripper := s.opticalRipper
	s.mu.RUnlock()

	if ripper == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Optical ripping engine is not initialized", r.URL.Path)
		return
	}

	var content []byte
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Form", "Failed to parse multipart form: "+err.Error(), r.URL.Path)
			return
		}

		var fileHeader io.Reader
		file, _, err := r.FormFile("file")
		if err == nil {
			fileHeader = file
			defer file.Close()
		} else {
			file, _, err := r.FormFile("keydb")
			if err == nil {
				fileHeader = file
				defer file.Close()
			}
		}

		if fileHeader == nil {
			s.writeProblem(w, http.StatusBadRequest, "Missing File", "No key file provided in form field 'file' or 'keydb'", r.URL.Path)
			return
		}

		var readErr error
		content, readErr = io.ReadAll(fileHeader)
		if readErr != nil {
			s.writeProblem(w, http.StatusBadRequest, "Read Error", "Failed to read uploaded key file: "+readErr.Error(), r.URL.Path)
			return
		}
	} else {
		var readErr error
		content, readErr = io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if readErr != nil {
			s.writeProblem(w, http.StatusBadRequest, "Read Error", "Failed to read request body: "+readErr.Error(), r.URL.Path)
			return
		}
	}

	if len(content) == 0 {
		s.writeProblem(w, http.StatusBadRequest, "Empty File", "Uploaded KEYDB.cfg is empty", r.URL.Path)
		return
	}

	count, err := ripper.SaveKeys(content)
	if err != nil {
		if errors.Is(err, optical.ErrEmptyKeyFile) || errors.Is(err, optical.ErrBinaryContent) || errors.Is(err, optical.ErrInvalidFormat) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Key File", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to save KEYDB.cfg", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Storage Error", "Failed to store key file: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, OpticalKeysUploadResponse{
		Status:    "active",
		KeysCount: count,
		Message:   "KEYDB.cfg uploaded successfully",
	})
}

