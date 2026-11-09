package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sovereign/internal/ai"
	"sovereign/internal/supervisor"
	"strings"
	"time"
)

// handleAIGateway mediates /v1 inference requests to the internal llama-server spoke,
// falling back to Safe Stub Mode (HTTP 503 Problem Details) when no models are active or provisioned,
// or returning a 503 Problem Details response when the AI service is stopped/inactive.
func (s *Server) handleAIGateway(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if gw == nil {
		ai.SafeStubHandler(w, r)
		return
	}

	if mgr != nil {
		st, err := mgr.GetService(r.Context(), supervisor.ServiceLlama)
		if err == nil && st != nil {
			if st.Status == supervisor.StatusInactive || st.Status == supervisor.StatusStopped {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type":     "https://sovereign.local/problems/service-inactive",
					"title":    "AI Service Inactive",
					"status":   http.StatusServiceUnavailable,
					"detail":   "The AI inference service is currently stopped or inactive. Please activate it in Settings to enable local inference.",
					"instance": r.URL.Path,
				})
				return
			}
			if st.Health == supervisor.HealthDegraded {
				ai.SafeStubHandler(w, r)
				return
			}
		}
	}

	gw.ServeHTTP(w, r)
}

// handleAIPresets returns the catalogue of pre-calibrated generation intent presets.
func (s *Server) handleAIPresets(w http.ResponseWriter, r *http.Request) {
	presets := ai.ListPresets()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"presets": presets,
	})
}

// handleAIGovernorStatus reports live request governor concurrency metrics and queue depths.
func (s *Server) handleAIGovernorStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	s.mu.RUnlock()

	var status ai.GovernorStatus
	if gw != nil && gw.Governor() != nil {
		status = gw.Governor().Status()
	} else {
		status = ai.GovernorStatus{
			TotalSlots:          2,
			ReservedInteractive: 1,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// handleAIStatus reports real-time GPU arbitration state, idle countdown, day/night habit mode, and hybrid GPU assignments.
func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	gw := s.aiGateway
	s.mu.RUnlock()

	var status ai.ArbitratorStatus
	if gw != nil && gw.Arbitrator() != nil {
		status = gw.Arbitrator().Status()
	} else {
		status = ai.ArbitratorStatus{
			State:              ai.ArbitratorStateSafeStub,
			HardwareMode:       "cpu",
			IsGPU:              false,
			IsHybrid:           false,
			CPUOptimized:       true,
			IsDaytime:          ai.IsDaytime(time.Now()),
			IdleSeconds:        0,
			IdleTimeoutSeconds: 300,
			ReassuranceMessage: ai.ReassuranceCPUOptimized,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// handleListAIPersonas returns all localized AI personas with their active status and presets.
func (s *Server) handleListAIPersonas(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	store := s.personaStore
	s.mu.RUnlock()

	if store == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Persona store unavailable", r.URL.Path)
		return
	}

	lang := r.URL.Query().Get("lang")
	if strings.TrimSpace(lang) == "" {
		lang = "en"
	}
	personas := store.ListPersonas(lang)
	activeID, _ := store.GetActive()
	targetLang := ai.NormalizeLanguage(lang)

	s.writeJSON(w, http.StatusOK, ai.PersonaListResponse{
		Personas: personas,
		ActiveID: activeID,
		Language: targetLang,
	})
}

// handleGetAIPersona returns detailed persona configuration including localized prompt.
func (s *Server) handleGetAIPersona(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	store := s.personaStore
	s.mu.RUnlock()

	if store == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Persona store unavailable", r.URL.Path)
		return
	}

	id := r.PathValue("id")
	lang := r.URL.Query().Get("lang")
	if strings.TrimSpace(lang) == "" {
		lang = "en"
	}

	persona, err := store.GetPersona(id, lang)
	if err != nil {
		s.writeProblem(w, http.StatusNotFound, "Persona Not Found", fmt.Sprintf("Persona %q not found", id), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, persona)
}

// handleSetActiveAIPersona updates the active persona and language preference, seeding Open WebUI.
func (s *Server) handleSetActiveAIPersona(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	store := s.personaStore
	s.mu.RUnlock()

	if store == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Persona store unavailable", r.URL.Path)
		return
	}

	var req ai.SetActivePersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request Body", fmt.Sprintf("Failed to parse request JSON: %v", err), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Persona ID", "Persona ID is required", r.URL.Path)
		return
	}

	persona, err := store.SetActive(req.ID, req.Language)
	if err != nil {
		if strings.Contains(err.Error(), "invalid persona id") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Persona ID", fmt.Sprintf("Unknown persona ID: %s", req.ID), r.URL.Path)
			return
		}
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to activate persona: %v", err), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, persona)
}

// handleListAIPrompts returns system prompt templates across languages and personas.
func (s *Server) handleListAIPrompts(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	store := s.personaStore
	s.mu.RUnlock()

	if store == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Persona store unavailable", r.URL.Path)
		return
	}

	lang := r.URL.Query().Get("lang")
	prompts := store.ListPrompts(lang)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ai.PromptsResponse{
		Prompts: prompts,
	})
}

// handleRAGSearch processes standalone document search queries against Paperless-ngx FTS5.
func (s *Server) handleRAGSearch(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.ragEngine
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "RAG Engine Unavailable", "Document RAG retrieval engine is not initialized.", r.URL.Path)
		return
	}

	var req ai.RAGSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request Body", fmt.Sprintf("Failed to decode JSON search request: %v", err), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Query", "Search query cannot be empty.", r.URL.Path)
		return
	}

	resp, err := engine.Search(r.Context(), req)
	if err != nil {
		s.logger.Error("rag search failed", "query", req.Query, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Document Search Failed", fmt.Sprintf("Failed to execute document search: %v", err), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRAGQuery orchestrates document retrieval, headroom truncation, prompt synthesis, and AI inference.
func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	engine := s.ragEngine
	gw := s.aiGateway
	mgr := s.serviceMgr
	s.mu.RUnlock()

	if engine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "RAG Engine Unavailable", "Document RAG retrieval engine is not initialized.", r.URL.Path)
		return
	}

	var req ai.RAGQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request Body", fmt.Sprintf("Failed to decode JSON query request: %v", err), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Query", "Search query cannot be empty.", r.URL.Path)
		return
	}

	// First, check if database is unprovisioned or empty
	dbPath := engine.ResolveDBPath()
	if dbPath == "" {
		normLang := ai.NormalizeLanguage(req.Language)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ai.RAGQueryResponse{
			Answer:    ai.GetNoMatchingDocumentsMessage(normLang),
			Citations: []ai.RAGSearchResult{},
			Truncated: false,
			Notice:    ai.GetUnprovisionedDBNotice(normLang),
		})
		return
	}

	// Verify AI inference spoke liveness and model availability
	if mgr != nil {
		st, err := mgr.GetService(r.Context(), supervisor.ServiceLlama)
		if err == nil && st != nil {
			if st.Status == supervisor.StatusInactive || st.Status == supervisor.StatusStopped {
				s.writeProblem(w, http.StatusServiceUnavailable, "AI Service Unavailable", "AI inference spoke is currently stopped or inactive.", r.URL.Path)
				return
			}
		}
	}

	if gw != nil {
		avail, err := gw.CheckModelAvailability(r.Context())
		if err != nil || !avail {
			s.writeProblem(w, http.StatusServiceUnavailable, "AI Service Unavailable", "No verified GGUF model is provisioned in storage.", r.URL.Path)
			return
		}
	}

	resp, err := engine.Query(r.Context(), req)
	if err != nil {
		if errors.Is(err, ai.ErrSafeStub) {
			s.writeProblem(w, http.StatusServiceUnavailable, "AI Service Unavailable", "No verified GGUF model is provisioned in storage.", r.URL.Path)
			return
		}
		s.logger.Error("rag query execution failed", "query", req.Query, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "RAG Query Failed", fmt.Sprintf("RAG query failed: %v", err), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListAIModels returns all installed GGUF models, their RAM/VRAM footprint, tokens/sec, and active status.
func (s *Server) handleListAIModels(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	resp, err := mm.ListModels(r.Context())
	if err != nil {
		s.logger.Error("failed to list AI models", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Model Scan Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleSetActiveAIModel switches the active GGUF model by filename with pre-flight header validation.
func (s *Server) handleSetActiveAIModel(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	var req struct {
		Filename string `json:"filename"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	target := req.Filename
	if target == "" {
		target = req.Model
	}
	if target == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Filename or model name is required", r.URL.Path)
		return
	}

	if err := mm.SetActiveModel(r.Context(), target); err != nil {
		if errors.Is(err, ai.ErrModelNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Model Not Found", err.Error(), r.URL.Path)
			return
		}
		s.logger.Warn("failed to switch active AI model", "filename", target, "error", err)
		s.writeProblem(w, http.StatusBadRequest, "Model Switch Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"active_model": target,
		"message":      fmt.Sprintf("Active model successfully switched to %s", target),
	})
}

// handleDownloadAIModel initiates a background model download with pre-flight storage reservation.
func (s *Server) handleDownloadAIModel(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	var req ai.AIModelDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	if req.URL == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Download URL is required", r.URL.Path)
		return
	}

	progress, err := mm.StartDownload(r.Context(), req)
	if err != nil {
		if errors.Is(err, ai.ErrInsufficientStorageSpace) {
			s.writeProblemWithType(w, "https://sovereign.local/errors/insufficient-storage", http.StatusInsufficientStorage, "Insufficient Storage", err.Error(), r.URL.Path)
			return
		}
		if errors.Is(err, ai.ErrDownloadInProgress) {
			s.writeProblem(w, http.StatusConflict, "Download In Progress", err.Error(), r.URL.Path)
			return
		}
		s.logger.Warn("failed to initiate model download", "url", req.URL, "error", err)
		s.writeProblem(w, http.StatusBadRequest, "Download Initiation Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(progress)
}

// handleListAIModelDownloads returns all active and historical download jobs with progress metrics.
func (s *Server) handleListAIModelDownloads(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	downloads := mm.ListDownloads()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"downloads": downloads,
	})
}

// handleCancelAIModelDownload terminates an in-flight download job.
func (s *Server) handleCancelAIModelDownload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Download ID is required", r.URL.Path)
		return
	}

	if err := mm.CancelDownload(id); err != nil {
		if errors.Is(err, ai.ErrDownloadNotFound) {
			s.writeProblem(w, http.StatusNotFound, "Download Not Found", err.Error(), r.URL.Path)
			return
		}
		s.writeProblem(w, http.StatusInternalServerError, "Cancel Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "cancelled",
		"id":     id,
	})
}

// handleDeleteAIModel deletes an unmounted GGUF model file from disk.
func (s *Server) handleDeleteAIModel(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	filename := r.PathValue("filename")
	if filename == "" {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Model filename is required", r.URL.Path)
		return
	}

	if err := mm.DeleteModel(r.Context(), filename); err != nil {
		if errors.Is(err, ai.ErrActiveModelDeletion) {
			s.writeProblem(w, http.StatusConflict, "Active Model Conflict", err.Error(), r.URL.Path)
			return
		}
		if errors.Is(err, ai.ErrModelNotFound) || errors.Is(err, os.ErrNotExist) {
			s.writeProblem(w, http.StatusNotFound, "Model Not Found", fmt.Sprintf("Model %q not found", filename), r.URL.Path)
			return
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "pointer symlink") || strings.Contains(err.Error(), "non-GGUF") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Request", err.Error(), r.URL.Path)
			return
		}
		s.logger.Warn("failed to delete AI model", "filename", filename, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Delete Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"deleted": filename,
	})
}

// handleGetAISettings returns operational AI parameters and hardware footprints.
func (s *Server) handleGetAISettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	settings, err := mm.GetSettings(r.Context())
	if err != nil {
		s.logger.Error("failed to get AI settings", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Settings Query Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(settings)
}

// handleUpdateAISettings updates operational AI parameters with immediate effect.
func (s *Server) handleUpdateAISettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	var update ai.AISettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request payload", r.URL.Path)
		return
	}

	updated, err := mm.UpdateSettings(r.Context(), update)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Settings Update Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// handleReloadAIEngine triggers a graceful reload of the AI inference engine container.
func (s *Server) handleReloadAIEngine(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	if err := mm.ReloadAIEngine(r.Context()); err != nil {
		s.logger.Error("failed to reload AI backend engine", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Engine Reload Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"message": "AI inference engine reloaded successfully",
	})
}

// handleGetAIInferenceLogs returns scrubbed inference and operational logs.
func (s *Server) handleGetAIInferenceLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mm := s.modelManager
	s.mu.RUnlock()

	if mm == nil {
		s.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Model manager is not initialized", r.URL.Path)
		return
	}

	logs, err := mm.GetScrubbedLogs(r.Context())
	if err != nil {
		s.writeProblem(w, http.StatusInternalServerError, "Log Query Failed", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ai.AIInferenceLogsResponse{
		Logs: logs,
	})
}
