package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"sovereign/internal/storage"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
)

const (
	AppVersion = "0.1.0"
)

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
			err := s.ingressMgr.SyncRoutes(syncCtx, nil)
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
					err := s.ingressMgr.SyncRoutes(retryCtx, nil)
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
		OpticalStatus:     "Idle · Ready for disc",
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

func (s *Server) syncIngressRoutes(ctx context.Context) {
	s.mu.RLock()
	ingressMgr := s.ingressMgr
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if ingressMgr == nil || mgr == nil {
		return
	}

	services, err := mgr.ListServices(ctx)
	if err != nil {
		s.logger.Warn("failed to list services for ingress route sync", "error", err)
		return
	}

	var routes []ingress.RouteConfig
	for _, svc := range services {
		if svc.Status == supervisor.StatusRunning {
			routes = append(routes, ingress.RouteConfig{
				Service: svc.ID,
				Port:    svc.Port,
				Subpath: svc.Subpath,
			})
		}
	}

	syncCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := ingressMgr.SyncRoutes(syncCtx, routes); err != nil {
		s.logger.Warn("failed to sync ingress routes with caddy", "error", err)
	}
}
