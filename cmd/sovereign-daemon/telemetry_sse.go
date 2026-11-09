package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"
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
	DockerAvailable   bool    `json:"docker_available"`
	DockerStatus      string  `json:"docker_status,omitempty"`
	DockerMessage     string  `json:"docker_message,omitempty"`
	Timestamp         string  `json:"timestamp"`
}

type sseEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
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

	// When an optical rip completes, automatically trigger enrichment scan
	if event == "optical.disc_ejected" {
		if m, ok := data.(map[string]any); ok && m["reason"] == "complete" {
			if s.enrichmentEngine != nil {
				go func() {
					_, _ = s.enrichmentEngine.Scan(context.Background())
				}()
			}
		}
	}

	// When a document is ingested, automatically trigger obligation scanning asynchronously
	if event == "dms.document_ingested" {
		if m, ok := data.(map[string]any); ok {
			docID, _ := m["id"].(string)
			filename, _ := m["filename"].(string)
			consumePath, _ := m["consume_path"].(string)
			if consumePath == "" {
				consumePath, _ = m["path"].(string)
			}
			if s.obligationScanner != nil && consumePath != "" {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_, _ = s.obligationScanner.ScanDocumentFile(ctx, docID, filename, consumePath, false)
				}()
			}
		}
	}
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
	if s.enrichmentEngine != nil && s.enrichmentEngine.IsEnriching() {
		opticalStatus = "Matching metadata"
	}

	gpuStatus := "Daytime (AI Priority)"
	if s.transcodeEngine != nil {
		gpuStatus = s.transcodeEngine.GetGPUStatus()
	}

	dockerPath := s.dockerSocketPath
	if dockerPath == "" {
		dockerPath = "/var/run/docker.sock"
	}

	dockerAvailable := false
	dockerStatus := "degraded"
	dockerMessage := "Container engine unavailable — background tasks paused"

	if fi, err := os.Stat(dockerPath); err == nil && fi != nil && (fi.Mode()&os.ModeSocket != 0) {
		dockerAvailable = true
		dockerStatus = "connected"
		dockerMessage = "Container engine operational"
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
		GPUStatus:         gpuStatus,
		DockerAvailable:   dockerAvailable,
		DockerStatus:      dockerStatus,
		DockerMessage:     dockerMessage,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Disable HTTP server write deadlines on persistent SSE streaming connection
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

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
	lastDockerAvailable := initEvt.DockerAvailable
	if !initEvt.DockerAvailable {
		if unavailData, err := json.Marshal(map[string]any{
			"docker_available": false,
			"status":           "degraded",
			"message":          initEvt.DockerMessage,
			"timestamp":        initEvt.Timestamp,
		}); err == nil {
			_, _ = fmt.Fprintf(w, "event: system.docker_unavailable\ndata: %s\n\n", unavailData)
			flusher.Flush()
		}
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
			if !evt.DockerAvailable && lastDockerAvailable {
				if unavailData, err := json.Marshal(map[string]any{
					"docker_available": false,
					"status":           "degraded",
					"message":          evt.DockerMessage,
					"timestamp":        evt.Timestamp,
				}); err == nil {
					_, _ = fmt.Fprintf(w, "event: system.docker_unavailable\ndata: %s\n\n", unavailData)
				}
			}
			lastDockerAvailable = evt.DockerAvailable
			flusher.Flush()
		}
	}
}
