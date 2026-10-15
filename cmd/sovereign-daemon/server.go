package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sovereign/internal/ingress"
	"sovereign/internal/storage"
	"sovereign/internal/store"
)

const (
	AppVersion = "0.1.0"
)

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
	ListenAddr  string
	DataDir     string
	DBPath      string
	StatePath   string
	CaddyURL    string
	Logger      *slog.Logger
	IngressMgr  ingress.IngressManager
	MDNSService *ingress.MDNSService
	BootMgr     *store.BootManager
}

// Server is the appliance core supervisor HTTP daemon.
type Server struct {
	cfg         Config
	db          *store.DB
	bootMgr     *store.BootManager
	logger      *slog.Logger
	router      *http.ServeMux
	ingressMgr  ingress.IngressManager
	mdnsService *ingress.MDNSService
	mu          sync.RWMutex
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

	s := &Server{
		cfg:         cfg,
		db:          db,
		bootMgr:     cfg.BootMgr,
		logger:      cfg.Logger,
		router:      http.NewServeMux(),
		ingressMgr:  cfg.IngressMgr,
		mdnsService: cfg.MDNSService,
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
}

// BootManager returns the active BootManager instance.
func (s *Server) BootManager() *store.BootManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bootMgr
}

// Close closes database handles and frees ingress/mDNS resources.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// routes registers HTTP REST handlers.
func (s *Server) routes() {
	s.router.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.router.HandleFunc("GET /api/v1/system/boot-status", s.handleBootStatus)
	s.router.HandleFunc("GET /api/v1/system/ingress", s.handleIngressStatus)
}

// Handler returns the HTTP handler for the server with CORS middleware.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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

	s.writeJSON(w, http.StatusOK, IngressStatusResponse{
		Status:         overallStatus,
		CaddyConnected: caddyConnected,
		MDNSActive:     mdnsActive,
		Hostname:       hostname,
		Routes:         routes,
	})
}
