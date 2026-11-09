package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sovereign/internal/supervisor"
	"strings"
	"time"
)

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

// ServiceUpgradeRequest matches the OpenAPI 3.1 contract.
type ServiceUpgradeRequest struct {
	ImageTag string `json:"image_tag,omitempty"`
	Image    string `json:"image,omitempty"`
}

func (s *Server) handleUpgradeService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "upgrade")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	var req ServiceUpgradeRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON payload", r.URL.Path)
			return
		}
	}

	imageTag := req.ImageTag
	if imageTag == "" {
		imageTag = req.Image
	}

	rev, err := mgr.UpgradeService(r.Context(), id, imageTag)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to upgrade service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to upgrade service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(r.Context())

	s.writeJSON(w, http.StatusOK, rev)
}

func (s *Server) handleRollbackService(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if mgr == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Service manager is not initialized", r.URL.Path)
		return
	}

	id := extractServiceID(r, "rollback")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Service ID missing in request path", r.URL.Path)
		return
	}

	failedRevNum := 0
	if r.Body != nil {
		var req struct {
			Revision int `json:"revision"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil && !errors.Is(err, io.EOF) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request body", r.URL.Path)
			return
		}
		if req.Revision > 0 {
			failedRevNum = req.Revision
		}
	}

	rollbackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	svc, err := mgr.ManualRollbackService(rollbackCtx, id, failedRevNum)
	if err != nil {
		if errors.Is(err, supervisor.ErrServiceNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Service Not Found", fmt.Sprintf("Service '%s' is not defined", id), r.URL.Path)
			return
		}
		if errors.Is(err, supervisor.ErrNoPriorRevision) {
			s.writeProblem(w, http.StatusNotFound, "Not Found", fmt.Sprintf("No prior revision found for service '%s'", id), r.URL.Path)
			return
		}
		s.logger.Error("failed to rollback service", "service", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to rollback service '%s': %v", id, err), r.URL.Path)
		return
	}

	s.syncIngressRoutes(rollbackCtx)

	s.writeJSON(w, http.StatusOK, svc)
}
