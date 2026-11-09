package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sovereign/internal/documents"
	"sovereign/internal/store"
	"sovereign/internal/supervisor"
	"strconv"
	"strings"
	"time"
)

// IMAPConfigResponse matches the OpenAPI 3.1 contract.
type IMAPConfigResponse struct {
	ID                 string `json:"id"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Folder             string `json:"folder"`
	SSLEnabled         bool   `json:"ssl_enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACertPath         string `json:"ca_cert_path"`
	Enabled            bool   `json:"enabled"`
	Connected          bool   `json:"connected"`
	LastPolledAt       string `json:"last_polled_at,omitempty"`
	LastError          string `json:"last_error"`
}

// IMAPConfigSaveRequest matches the OpenAPI 3.1 contract.
type IMAPConfigSaveRequest struct {
	ID                 string `json:"id,omitempty"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Folder             string `json:"folder"`
	SSLEnabled         *bool  `json:"ssl_enabled,omitempty"`
	InsecureSkipVerify *bool  `json:"insecure_skip_verify,omitempty"`
	CACertPath         string `json:"ca_cert_path,omitempty"`
	Enabled            *bool  `json:"enabled,omitempty"`
}

// IMAPTestRequest matches the OpenAPI 3.1 contract.
type IMAPTestRequest struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Folder             string `json:"folder"`
	SSLEnabled         *bool  `json:"ssl_enabled,omitempty"`
	InsecureSkipVerify *bool  `json:"insecure_skip_verify,omitempty"`
	CACertPath         string `json:"ca_cert_path,omitempty"`
}

// IMAPTestResponse matches the OpenAPI 3.1 contract.
type IMAPTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handleDocumentUpload handles multipart file uploads to /api/v1/documents/upload.
func (s *Server) handleDocumentUpload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.ingestEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Document ingest engine is not initialized", r.URL.Path)
		return
	}

	engine.IngestHTTPHandler(s.writeProblem, s.writeJSON)(w, r)
}

// handleGetIMAPConfig returns the active IMAP mailbox configuration with masked password.
func (s *Server) handleGetIMAPConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	s.mu.RUnlock()

	if imapStore == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "IMAP store is not initialized", r.URL.Path)
		return
	}

	cfg, err := imapStore.GetFirst(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrIMAPConfigNotFound) {
			s.writeJSON(w, http.StatusOK, IMAPConfigResponse{
				ID:         "",
				Host:       "",
				Port:       993,
				Username:   "",
				Password:   "",
				Folder:     "INBOX",
				SSLEnabled: true,
				Enabled:    false,
				Connected:  false,
				LastError:  "",
			})
			return
		}
		s.logger.Error("failed to get imap config", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve IMAP configuration: "+err.Error(), r.URL.Path)
		return
	}

	lastPolledStr := ""
	if cfg.LastPolledAt != nil {
		lastPolledStr = cfg.LastPolledAt.UTC().Format(time.RFC3339)
	}

	connected := cfg.Enabled && cfg.LastError == "" && cfg.LastPolledAt != nil

	resp := IMAPConfigResponse{
		ID:                 cfg.ID,
		Host:               cfg.Host,
		Port:               cfg.Port,
		Username:           cfg.Username,
		Password:           "********",
		Folder:             cfg.Folder,
		SSLEnabled:         cfg.SSLEnabled,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		CACertPath:         cfg.CACertPath,
		Enabled:            cfg.Enabled,
		Connected:          connected,
		LastPolledAt:       lastPolledStr,
		LastError:          cfg.LastError,
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleSaveIMAPConfig creates or updates an IMAP mailbox configuration with encrypted credentials.
func (s *Server) handleSaveIMAPConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	poller := s.imapPoller
	s.mu.RUnlock()

	if imapStore == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "IMAP store is not initialized", r.URL.Path)
		return
	}

	var req IMAPConfigSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Request body must be valid JSON: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Host) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'host' cannot be empty", r.URL.Path)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'username' cannot be empty", r.URL.Path)
		return
	}
	if req.Port <= 0 {
		req.Port = 993
	}
	if strings.TrimSpace(req.Folder) == "" {
		req.Folder = "INBOX"
	}

	sslEnabled := true
	if req.SSLEnabled != nil {
		sslEnabled = *req.SSLEnabled
	}
	insecureSkipVerify := false
	if req.InsecureSkipVerify != nil {
		insecureSkipVerify = *req.InsecureSkipVerify
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Check if this is an update and password is masked
	existingPass := ""
	if req.ID != "" {
		if existing, err := imapStore.Get(r.Context(), req.ID); err == nil && existing != nil {
			existingPass = existing.Password
		}
	} else {
		if existing, err := imapStore.GetActive(r.Context()); err == nil && existing != nil {
			req.ID = existing.ID
			existingPass = existing.Password
		}
	}

	passwordToSave := req.Password
	if passwordToSave == "********" || passwordToSave == "" {
		passwordToSave = existingPass
	}

	if passwordToSave == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'password' cannot be empty", r.URL.Path)
		return
	}

	configToSave := &store.IMAPConfig{
		ID:                 req.ID,
		Host:               req.Host,
		Port:               req.Port,
		Username:           req.Username,
		Password:           passwordToSave,
		Folder:             req.Folder,
		SSLEnabled:         sslEnabled,
		InsecureSkipVerify: insecureSkipVerify,
		CACertPath:         req.CACertPath,
		Enabled:            enabled,
	}

	if err := imapStore.Save(r.Context(), configToSave); err != nil {
		s.logger.Error("failed to save imap config", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Failed to save IMAP configuration: "+err.Error(), r.URL.Path)
		return
	}

	// Trigger immediate background poll if enabled
	if poller != nil && enabled {
		go func() {
			_ = poller.PollOnce(context.Background())
		}()
	}

	resp := IMAPConfigResponse{
		ID:                 configToSave.ID,
		Host:               configToSave.Host,
		Port:               configToSave.Port,
		Username:           configToSave.Username,
		Password:           "********",
		Folder:             configToSave.Folder,
		SSLEnabled:         configToSave.SSLEnabled,
		InsecureSkipVerify: configToSave.InsecureSkipVerify,
		CACertPath:         configToSave.CACertPath,
		Enabled:            configToSave.Enabled,
		Connected:          configToSave.Enabled && configToSave.LastError == "" && configToSave.LastPolledAt != nil,
		LastError:          "",
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleTestIMAPConnection tests connectivity and credentials with the specified IMAP server.
func (s *Server) handleTestIMAPConnection(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	imapStore := s.imapStore
	s.mu.RUnlock()

	var req IMAPTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Request body must be valid JSON: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Host) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'host' cannot be empty", r.URL.Path)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'username' cannot be empty", r.URL.Path)
		return
	}
	if req.Port <= 0 {
		req.Port = 993
	}
	if strings.TrimSpace(req.Folder) == "" {
		req.Folder = "INBOX"
	}

	sslEnabled := true
	if req.SSLEnabled != nil {
		sslEnabled = *req.SSLEnabled
	}
	insecureSkipVerify := false
	if req.InsecureSkipVerify != nil {
		insecureSkipVerify = *req.InsecureSkipVerify
	}

	passwordToTest := req.Password
	if passwordToTest == "" || passwordToTest == "********" {
		if imapStore != nil {
			if active, err := imapStore.GetFirst(r.Context()); err == nil && active != nil {
				passwordToTest = active.Password
			}
		}
	}

	if passwordToTest == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Field 'password' cannot be empty", r.URL.Path)
		return
	}

	client := documents.NewClient()
	defer func() {
		_ = client.Close()
	}()

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	tlsConfig, err := documents.BuildTLSConfig(req.Host, insecureSkipVerify, req.CACertPath)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid TLS Configuration", err.Error(), r.URL.Path)
		return
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := client.Connect(testCtx, addr, sslEnabled, tlsConfig); err != nil {
		s.logger.Warn("imap connection test failed", "host", req.Host, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	if err := client.Login(testCtx, req.Username, passwordToTest); err != nil {
		s.logger.Warn("imap login test failed", "host", req.Host, "username", req.Username, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	if _, err := client.Select(testCtx, req.Folder); err != nil {
		s.logger.Warn("imap folder select test failed", "folder", req.Folder, "error", err)
		if s.serviceMgr != nil {
			s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "degraded", documents.ReassuranceMailboxDisconnected)
		}
		s.writeProblem(w, http.StatusBadGateway, "Mailbox Connection Failed", documents.ReassuranceMailboxDisconnected, r.URL.Path)
		return
	}

	// On successful connection test, restore Paperless health if it was degraded
	if s.serviceMgr != nil {
		s.serviceMgr.SetServiceHealth(supervisor.ServicePaperless, "running", "healthy", "Automatic Text Search Active · Mailbox Connected")
	}

	s.writeJSON(w, http.StatusOK, IMAPTestResponse{
		Success: true,
		Message: "Mailbox connection successful",
	})
}

// handleGetDeadlines handles GET /api/v1/documents/deadlines.
func (s *Server) handleGetDeadlines(w http.ResponseWriter, r *http.Request) {
	if s.deadlineStore == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Store Unavailable", "Deadline store is not initialized", r.URL.Path)
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.DeadlineStatusActive
	}

	withinDays := 0
	if status == store.DeadlineStatusPending || status == store.DeadlineStatusActive {
		withinDays = 14
	}
	if q := r.URL.Query().Get("within_days"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			withinDays = parsed
		}
	}

	deadlines, err := s.deadlineStore.List(r.Context(), status, withinDays, time.Now())
	if err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Query Failed", fmt.Sprintf("Failed to list deadlines: %v", err), r.URL.Path)
		return
	}

	if deadlines == nil {
		deadlines = []*store.DocumentDeadline{}
	}

	s.writeJSON(w, http.StatusOK, deadlines)
}

// handleDismissDeadline handles POST /api/v1/documents/deadlines/{id}/dismiss.
func (s *Server) handleDismissDeadline(w http.ResponseWriter, r *http.Request) {
	if s.deadlineStore == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Store Unavailable", "Deadline store is not initialized", r.URL.Path)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid ID", "Deadline ID is required in path", r.URL.Path)
		return
	}

	updated, err := s.deadlineStore.Dismiss(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrDeadlineNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Not Found", fmt.Sprintf("Deadline with ID %q was not found", id), r.URL.Path)
			return
		}
		s.writeProblem(w, http.StatusInternalServerError, "Dismissal Failed", fmt.Sprintf("Failed to dismiss deadline: %v", err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, updated)
}

// ScanDeadlinesRequest matches the OpenAPI contract for manual obligation scan trigger.
type ScanDeadlinesRequest struct {
	Force      bool   `json:"force"`
	DocumentID string `json:"document_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
}

// ScanDeadlinesResponse matches the OpenAPI contract for manual obligation scan results.
type ScanDeadlinesResponse struct {
	Status         string                    `json:"status"`
	ScannedCount   int                       `json:"scanned_count"`
	DeadlinesFound int                       `json:"deadlines_found"`
	Deadlines      []*store.DocumentDeadline `json:"deadlines"`
}

// handleScanDeadlines handles POST /api/v1/documents/deadlines/scan.
func (s *Server) handleScanDeadlines(w http.ResponseWriter, r *http.Request) {
	if s.obligationScanner == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Scanner Unavailable", "Obligation scanner is not initialized", r.URL.Path)
		return
	}

	var req ScanDeadlinesRequest
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err == nil && len(strings.TrimSpace(string(bodyBytes))) > 0 {
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				s.writeProblem(w, http.StatusBadRequest, "Invalid Request", fmt.Sprintf("Failed to decode scan request: %v", err), r.URL.Path)
				return
			}
		}
	}

	var found []*store.DocumentDeadline
	scannedCount := 0

	if strings.TrimSpace(req.Text) != "" {
		docID := req.DocumentID
		if docID == "" {
			docID = "manual-scan"
		}
		title := req.Title
		if title == "" {
			title = "Scanned Document"
		}
		results, err := s.obligationScanner.ScanDocumentText(r.Context(), docID, title, req.Text, req.Force)
		if err != nil {
			s.writeProblem(w, http.StatusInternalServerError, "Scan Failed", fmt.Sprintf("Document text scan failed: %v", err), r.URL.Path)
			return
		}
		found = append(found, results...)
		scannedCount = 1
	} else {
		// Run batch pass over queued/pending documents
		processed, err := s.obligationScanner.ProcessNightlyBatch(r.Context())
		if err != nil {
			s.logger.Warn("batch scan completed with warnings", "error", err)
		}
		scannedCount = processed
		if s.deadlineStore != nil {
			list, err := s.deadlineStore.List(r.Context(), store.DeadlineStatusPending, s.obligationScanner.AlertThreshold(), time.Now())
			if err == nil {
				found = list
			}
		}
	}

	if found == nil {
		found = []*store.DocumentDeadline{}
	}

	s.writeJSON(w, http.StatusOK, ScanDeadlinesResponse{
		Status:         "completed",
		ScannedCount:   scannedCount,
		DeadlinesFound: len(found),
		Deadlines:      found,
	})
}
