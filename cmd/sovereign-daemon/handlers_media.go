package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sovereign/internal/optical"
	"sovereign/internal/transcode"
	"strings"
	"time"
)

// TranscodeJobResponse matches the OpenAPI 3.1 contract.
type TranscodeJobResponse struct {
	ID           string  `json:"id"`
	SourcePath   string  `json:"source_path"`
	TargetPath   string  `json:"target_path"`
	Profile      string  `json:"profile"`
	Status       string  `json:"status"`
	Progress     float64 `json:"progress"`
	EncoderUsed  string  `json:"encoder_used,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
	CreatedAt    string  `json:"created_at"`
	StartedAt    string  `json:"started_at,omitempty"`
	CompletedAt  string  `json:"completed_at,omitempty"`
}

// TranscodeQueueResponse matches the OpenAPI 3.1 contract.
type TranscodeQueueResponse struct {
	Jobs        []TranscodeJobResponse `json:"jobs"`
	Total       int                    `json:"total"`
	ActiveJobID string                 `json:"active_job_id,omitempty"`
}

// EnqueueTranscodeRequest matches the OpenAPI 3.1 contract.
type EnqueueTranscodeRequest struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path,omitempty"`
	Profile    string `json:"profile,omitempty"`
}

// EnqueueMobileRequest matches the OpenAPI 3.1 contract.
type EnqueueMobileRequest struct {
	SourcePath    string `json:"source_path"`
	TargetProfile string `json:"target_profile"`
	Profile       string `json:"profile,omitempty"`
}

// TranscodeScheduleResponse matches the OpenAPI 3.1 contract.
type TranscodeScheduleResponse struct {
	StartHour      int    `json:"start_hour"`
	StartMinute    int    `json:"start_minute"`
	EndHour        int    `json:"end_hour"`
	EndMinute      int    `json:"end_minute"`
	IsOffPeak      bool   `json:"is_off_peak"`
	ManualOverride bool   `json:"manual_override"`
	Status         string `json:"status"`
}

// UpdateTranscodeScheduleRequest matches the OpenAPI 3.1 contract.
type UpdateTranscodeScheduleRequest struct {
	StartHour   int `json:"start_hour"`
	StartMinute int `json:"start_minute,omitempty"`
	EndHour     int `json:"end_hour"`
	EndMinute   int `json:"end_minute,omitempty"`
}

// TranscodeOverrideRequest matches the OpenAPI 3.1 contract.
type TranscodeOverrideRequest struct {
	Force bool `json:"force"`
}

// TranscodeOverrideResponse matches the OpenAPI 3.1 contract.
type TranscodeOverrideResponse struct {
	Status         string `json:"status"`
	ManualOverride bool   `json:"manual_override"`
	Message        string `json:"message"`
}

// EnrichmentScanResponse matches the OpenAPI 3.1 contract.
type EnrichmentScanResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	ItemsProcessed int    `json:"items_processed"`
}

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

	const maxKeyUploadSize = 10 << 20 // 10MB
	if r.ContentLength > maxKeyUploadSize {
		s.writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Key file payload exceeds 10MB limit", r.URL.Path)
		return
	}

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxKeyUploadSize); err != nil {
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
		content, readErr = io.ReadAll(io.LimitReader(r.Body, maxKeyUploadSize+1))
		if readErr != nil {
			s.writeProblem(w, http.StatusBadRequest, "Read Error", "Failed to read request body: "+readErr.Error(), r.URL.Path)
			return
		}
		if len(content) > maxKeyUploadSize {
			s.writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Key file payload exceeds 10MB limit", r.URL.Path)
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

// handleEnrichmentStatus returns current media metadata and artwork enrichment engine state.
func (s *Server) handleEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Enrichment Engine Unavailable", "Enrichment engine is not initialized", r.URL.Path)
		return
	}

	status := s.enrichmentEngine.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

// handleEnrichmentScan initiates an immediate scan and enrichment of staged optical discs.
func (s *Server) handleEnrichmentScan(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Enrichment Engine Unavailable", "Enrichment engine is not initialized", r.URL.Path)
		return
	}

	count, err := s.enrichmentEngine.Scan(r.Context())
	if err != nil {
		s.logger.Error("enrichment scan failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Scan Failed", "Enrichment scan encountered an error: "+err.Error(), r.URL.Path)
		return
	}

	s.writeJSON(w, http.StatusOK, EnrichmentScanResponse{
		Status:         "ok",
		Message:        fmt.Sprintf("Enrichment scan completed successfully: %d items processed", count),
		ItemsProcessed: count,
	})
}

// handleTranscodeQueue lists all jobs currently in the transcoding queue.
func (s *Server) handleTranscodeQueue(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	jobs, err := s.transcodeEngine.GetQueue(r.Context())
	if err != nil {
		s.logger.Error("failed to list transcode queue", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Queue Query Failed", err.Error(), r.URL.Path)
		return
	}

	st := s.transcodeEngine.GetStatus()
	resp := TranscodeQueueResponse{
		Jobs:        make([]TranscodeJobResponse, 0, len(jobs)),
		Total:       len(jobs),
		ActiveJobID: st.ActiveJobID,
	}

	for _, j := range jobs {
		item := TranscodeJobResponse{
			ID:           j.ID,
			SourcePath:   j.SourcePath,
			TargetPath:   j.TargetPath,
			Profile:      j.Profile,
			Status:       string(j.Status),
			Progress:     j.Progress,
			EncoderUsed:  j.EncoderUsed,
			ErrorMessage: j.ErrorMessage,
			CreatedAt:    j.CreatedAt.Format(time.RFC3339),
		}
		if j.StartedAt != nil {
			item.StartedAt = j.StartedAt.Format(time.RFC3339)
		}
		if j.CompletedAt != nil {
			item.CompletedAt = j.CompletedAt.Format(time.RFC3339)
		}
		resp.Jobs = append(resp.Jobs, item)
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleEnqueueTranscode enqueues a media file for background off-peak transcoding.
func (s *Server) handleEnqueueTranscode(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req EnqueueTranscodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse request body: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.SourcePath) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Source Path", "source_path is required", r.URL.Path)
		return
	}

	job, err := s.transcodeEngine.Enqueue(r.Context(), transcode.EnqueueRequest{
		SourcePath: req.SourcePath,
		TargetPath: req.TargetPath,
		Profile:    req.Profile,
	})
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid File Path", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeJobResponse{
		ID:           job.ID,
		SourcePath:   job.SourcePath,
		TargetPath:   job.TargetPath,
		Profile:      job.Profile,
		Status:       string(job.Status),
		Progress:     job.Progress,
		EncoderUsed:  job.EncoderUsed,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt.Format(time.RFC3339),
	}

	s.writeJSON(w, http.StatusCreated, resp)
}

// handleEnqueueMobile enqueues a media file for mobile direct-play profile generation.
func (s *Server) handleEnqueueMobile(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req EnqueueMobileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse request body: "+err.Error(), r.URL.Path)
		return
	}

	if strings.TrimSpace(req.SourcePath) == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Source Path", "source_path is required", r.URL.Path)
		return
	}

	targetProfile := strings.TrimSpace(req.TargetProfile)
	if targetProfile == "" {
		targetProfile = strings.TrimSpace(req.Profile)
	}
	if targetProfile == "" {
		s.writeProblem(w, http.StatusBadRequest, "Missing Target Profile", "target_profile is required (e.g. 'mobile-720p' or 'mobile-1080p')", r.URL.Path)
		return
	}

	if !transcode.IsMobileProfile(targetProfile) {
		s.writeProblem(w, http.StatusBadRequest, "Unknown Mobile Profile", fmt.Sprintf("target_profile %q is not supported: must be 'mobile-720p' or 'mobile-1080p'", targetProfile), r.URL.Path)
		return
	}

	job, isExisting, err := s.transcodeEngine.EnqueueMobile(r.Context(), transcode.MobileEnqueueRequest{
		SourcePath:    req.SourcePath,
		TargetProfile: targetProfile,
	})
	if err != nil {
		if errors.Is(err, transcode.ErrUnknownMobileProfile) ||
			errors.Is(err, transcode.ErrInvalidSourcePath) ||
			errors.Is(err, transcode.ErrIdenticalPaths) ||
			errors.Is(err, transcode.ErrUnsupportedVideoFormat) ||
			errors.Is(err, transcode.ErrSourceOutsideDataDir) ||
			errors.Is(err, transcode.ErrSourceNotFound) ||
			errors.Is(err, transcode.ErrSourceIsDir) ||
			strings.Contains(err.Error(), "required") ||
			strings.Contains(err.Error(), "unsupported") ||
			strings.Contains(err.Error(), "cannot be empty") {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Mobile Request", err.Error(), r.URL.Path)
			return
		}
		s.logger.Error("failed to enqueue mobile transcode job", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, "Enqueue Failed", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeJobResponse{
		ID:           job.ID,
		SourcePath:   job.SourcePath,
		TargetPath:   job.TargetPath,
		Profile:      job.Profile,
		Status:       string(job.Status),
		Progress:     job.Progress,
		EncoderUsed:  job.EncoderUsed,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt.Format(time.RFC3339),
	}
	if job.StartedAt != nil {
		resp.StartedAt = job.StartedAt.Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		resp.CompletedAt = job.CompletedAt.Format(time.RFC3339)
	}

	statusCode := http.StatusCreated
	if isExisting {
		statusCode = http.StatusOK
	}

	s.writeJSON(w, statusCode, resp)
}

// handleGetTranscodeSchedule returns current off-peak schedule configuration and live status.
func (s *Server) handleGetTranscodeSchedule(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	sched := s.transcodeEngine.Scheduler()
	cfg := sched.GetConfig()
	resp := TranscodeScheduleResponse{
		StartHour:      cfg.StartHour,
		StartMinute:    cfg.StartMinute,
		EndHour:        cfg.EndHour,
		EndMinute:      cfg.EndMinute,
		IsOffPeak:      sched.IsOffPeakNow(),
		ManualOverride: sched.ManualOverride(),
		Status:         sched.GetGPUStatusNow(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleUpdateTranscodeSchedule updates off-peak window hours and minutes.
func (s *Server) handleUpdateTranscodeSchedule(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req UpdateTranscodeScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse schedule config: "+err.Error(), r.URL.Path)
		return
	}

	sched := s.transcodeEngine.Scheduler()
	currentCfg := sched.GetConfig()
	currentCfg.StartHour = req.StartHour
	currentCfg.StartMinute = req.StartMinute
	currentCfg.EndHour = req.EndHour
	currentCfg.EndMinute = req.EndMinute

	if err := sched.SetConfig(currentCfg); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid Schedule", err.Error(), r.URL.Path)
		return
	}

	resp := TranscodeScheduleResponse{
		StartHour:      currentCfg.StartHour,
		StartMinute:    currentCfg.StartMinute,
		EndHour:        currentCfg.EndHour,
		EndMinute:      currentCfg.EndMinute,
		IsOffPeak:      sched.IsOffPeakNow(),
		ManualOverride: sched.ManualOverride(),
		Status:         sched.GetGPUStatusNow(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleTranscodeOverride enables or disables manual override bypassing daytime schedule restrictions.
func (s *Server) handleTranscodeOverride(w http.ResponseWriter, r *http.Request) {
	if s.transcodeEngine == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "Transcode Engine Unavailable", "Transcode engine is not initialized", r.URL.Path)
		return
	}

	var req TranscodeOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeProblem(w, http.StatusBadRequest, "Invalid JSON", "Failed to parse override body: "+err.Error(), r.URL.Path)
		return
	}

	s.transcodeEngine.SetManualOverride(req.Force)

	statusStr := "inactive"
	msg := "Manual override disabled; daytime AI priority restored"
	if req.Force {
		statusStr = "active"
		msg = "Manual override enabled; daytime batch transcoding active"
	}

	s.writeJSON(w, http.StatusOK, TranscodeOverrideResponse{
		Status:         statusStr,
		ManualOverride: req.Force,
		Message:        msg,
	})
}
