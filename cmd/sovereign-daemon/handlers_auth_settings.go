package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sovereign/internal/auth"
	"sovereign/internal/supervisor"
)

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

func isValidPreviewConfirmationToken(token string) bool {
	normalized := strings.TrimSpace(strings.ToLower(token))
	switch normalized {
	case "preview", "vorschau", "preliminar", "vista previa", "aperçu", "apercu":
		return true
	default:
		return false
	}
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
	updateChannel, _ := dbHandle.GetSetting(ctx, "update_channel")
	if updateChannel == "" {
		updateChannel = supervisor.ChannelStable
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"language":       lang,
		"admin_email":    email,
		"hostname":       hostname,
		"features":       features,
		"update_channel": updateChannel,
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

	if newChannel, exists := payload["update_channel"]; exists {
		newChannel = strings.TrimSpace(strings.ToLower(newChannel))
		if newChannel != supervisor.ChannelStable &&
			newChannel != supervisor.ChannelSecurityOnly &&
			newChannel != supervisor.ChannelPreview {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Channel", "Update channel must be stable, security-only, or preview", r.URL.Path)
			return
		}

		if newChannel == supervisor.ChannelPreview {
			confirmation := payload["confirmation"]
			if confirmation == "" {
				confirmation = r.Header.Get("X-Confirmation-Token")
			}
			if confirmation == "" {
				confirmation = r.URL.Query().Get("confirmation")
			}
			if !isValidPreviewConfirmationToken(confirmation) {
				s.writeProblem(w, http.StatusBadRequest, "Preview Confirmation Required", "Changing to the preview release channel requires typing confirmation 'PREVIEW'.", r.URL.Path)
				return
			}
		}

		payload["update_channel"] = newChannel
		if s.updateEngine != nil {
			s.updateEngine.SetChannel(newChannel)
		}
	}

	ctx := r.Context()
	for k, v := range payload {
		if k == "confirmation" {
			continue
		}
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
