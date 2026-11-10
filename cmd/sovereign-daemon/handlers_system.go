package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sovereign/internal/ingress"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"strings"
	"time"
)

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

	if s.db != nil {
		if ch, _ := s.db.GetSetting(r.Context(), "update_channel"); ch != "" {
			status.UpdateChannel = ch
		} else {
			status.UpdateChannel = "stable"
		}
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

// ComposeRevisionsResponse matches the OpenAPI 3.1 contract.
type ComposeRevisionsResponse struct {
	Revisions      []store.ComposeRevision `json:"revisions"`
	Total          int                     `json:"total"`
	ActiveRevision *int                    `json:"active_revision,omitempty"`
}

func (s *Server) handleListRevisions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database store is not initialized", r.URL.Path)
		return
	}

	revs, err := s.db.ListRevisions(r.Context())
	if err != nil {
		s.logger.Error("failed to list compose revisions", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve compose revisions", r.URL.Path)
		return
	}

	activeRevNum, _ := s.db.GetActiveRevisionNumber(r.Context())
	resp := ComposeRevisionsResponse{
		Revisions: revs,
		Total:     len(revs),
	}
	if activeRevNum > 0 {
		resp.ActiveRevision = &activeRevNum
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetActiveRevision(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Database store is not initialized", r.URL.Path)
		return
	}

	rev, err := s.db.GetActiveRevision(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeProblem(w, http.StatusNotFound, "Not Found", "No active compose revision found", r.URL.Path)
			return
		}
		s.logger.Error("failed to get active compose revision", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve active compose revision", r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, rev)
}

func (s *Server) handleSystemReboot(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("system reboot initiated by administrator")
	if s.bootMgr != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.bootMgr.RecordCleanShutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to record clean shutdown for reboot", "error", err)
		}
	}

	s.BroadcastEvent("system.rebooting", map[string]any{
		"status":  "rebooting",
		"message": "Appliance is restarting",
	})
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "rebooting",
		"message": "Appliance is rebooting. It will be back online in approximately 60 seconds.",
	})
}

func (s *Server) handleFactoryReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Confirmation) != "RESET" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Confirmation", "You must provide confirmation string 'RESET' to execute a factory reset.", r.URL.Path)
		return
	}

	s.logger.Warn("factory reset initiated by administrator")
	if s.db != nil {
		if err := s.db.SetSetting(r.Context(), "setup_completed", "false"); err != nil {
			s.logger.Error("failed to reset setup_completed setting", "error", err)
			s.writeProblem(w, http.StatusInternalServerError, "Reset Failed", "Failed to reset appliance configuration: "+err.Error(), r.URL.Path)
			return
		}
	}

	if s.authMgr != nil {
		if sess, err := s.authMgr.GetSessionFromRequest(r); err == nil && sess != nil {
			_ = s.authMgr.RevokeSession(r.Context(), sess.Token)
		}
		s.authMgr.ClearSessionCookie(w)
	}

	s.BroadcastEvent("system.factory_reset", map[string]any{
		"status":  "resetting",
		"message": "Appliance has been reset to factory defaults",
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "resetting",
		"message": "Appliance has been reset to factory defaults. Redirecting to setup...",
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
