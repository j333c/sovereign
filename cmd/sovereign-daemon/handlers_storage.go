package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sovereign/internal/storage"
	"strings"
)

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

	// Pre-flight Content-Length inspection: verify available capacity (upload size + 1.0 GB safety buffer)
	if r.ContentLength > 0 {
		if err := engine.CheckCapacity(r.ContentLength); err != nil {
			if errors.Is(err, storage.ErrInsufficientStorage) {
				s.writeProblemWithType(w, "https://sovereign.local/errors/insufficient-storage", http.StatusInsufficientStorage, "Insufficient Storage", "Target storage pool has insufficient space for this upload (1.0 GB safety buffer required).", r.URL.Path)
				return
			}
			s.logger.Error("failed to check dropzone storage capacity", "error", err)
			s.writeProblem(w, http.StatusInternalServerError, "Storage Check Failed", "Failed to check available storage capacity.", r.URL.Path)
			return
		}
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
				s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks, documents", r.URL.Path)
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
		if errors.Is(uploadErr, storage.ErrInsufficientStorage) {
			s.writeProblemWithType(w, "https://sovereign.local/errors/insufficient-storage", http.StatusInsufficientStorage, "Insufficient Storage", "Target storage pool has insufficient space for this upload (1.0 GB safety buffer required).", r.URL.Path)
			return
		}
		if errors.Is(uploadErr, storage.ErrInvalidCategory) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Category", "Category must be one of: movies, tv, music, audiobooks, documents", r.URL.Path)
			return
		}
		if category == storage.CategoryDocuments && (errors.Is(uploadErr, storage.ErrInvalidMedia) || errors.Is(uploadErr, storage.ErrCategoryMismatch)) {
			s.writeProblem(w, http.StatusBadRequest, "Invalid Document Format", "Only document files (.pdf, .png, .jpg, .jpeg, .tiff, .tif) are accepted.", r.URL.Path)
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

	if category == storage.CategoryDocuments && uploadedItem != nil {
		s.BroadcastEvent("dms.document_ingested", map[string]any{
			"id":           uploadedItem.ID,
			"filename":     uploadedItem.Filename,
			"path":         uploadedItem.Path,
			"size":         uploadedItem.Size,
			"content_type": uploadedItem.ContentType,
			"source":       "dropzone",
		})
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
