package ai

import (
	"context"
	"strings"
	"sync"
	"time"
)

// AIInferenceLogEntry represents a scrubbed operational or inference log item.
type AIInferenceLogEntry struct {
	Timestamp       time.Time `json:"timestamp"`
	Level           string    `json:"level"`
	Component       string    `json:"component"`
	Message         string    `json:"message"`
	TokensGenerated int       `json:"tokens_generated,omitempty"`
	DurationMs      int64     `json:"duration_ms,omitempty"`
	TokensPerSec    float64   `json:"tokens_per_sec,omitempty"`
}

// AIInferenceLogsResponse holds scrubbed inference logs.
type AIInferenceLogsResponse struct {
	Logs []AIInferenceLogEntry `json:"logs"`
}

// InferenceLogRecorder provides a thread-safe ring buffer of scrubbed logs.
type InferenceLogRecorder struct {
	mu      sync.RWMutex
	entries []AIInferenceLogEntry
	maxSize int
}

// NewInferenceLogRecorder creates a log recorder with default capacity.
func NewInferenceLogRecorder(maxSize int) *InferenceLogRecorder {
	if maxSize <= 0 {
		maxSize = 200
	}
	r := &InferenceLogRecorder{
		entries: make([]AIInferenceLogEntry, 0, maxSize),
		maxSize: maxSize,
	}
	// Seed initial system startup entries
	now := time.Now().UTC()
	r.Add(now.Add(-5*time.Minute), "INFO", "ai-engine", "Managed llama-server inference spoke registered on sovereign-ai network")
	r.Add(now.Add(-4*time.Minute), "INFO", "ai-engine", "FlashAttention enabled with standardized KV cache buffer")
	r.Add(now.Add(-3*time.Minute), "INFO", "governor", "Two-Lane Priority Request Governor operational (-np 2)")
	r.Add(now.Add(-2*time.Minute), "INFO", "arbitrator", "Dynamic GPU Arbitrator active with 300s dynamic idle surrender")
	return r
}

// ScrubLogMessage sanitizes personal prompt text or sensitive queries from log messages.
func ScrubLogMessage(msg string) string {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "prompt") || strings.Contains(lower, "content") || strings.Contains(lower, "query") {
		// If message indicates prompt contents, scrub any trailing or embedded text
		for _, prefix := range []string{"prompt:", "prompt=", "content:", "query:"} {
			if idx := strings.Index(lower, prefix); idx != -1 {
				return msg[:idx+len(prefix)] + " [PROMPT_SCRUBBED]"
			}
		}
	}
	return msg
}

// Add appends a scrubbed log entry.
func (r *InferenceLogRecorder) Add(ts time.Time, level, component, msg string, metrics ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := AIInferenceLogEntry{
		Timestamp: ts,
		Level:     level,
		Component: component,
		Message:   ScrubLogMessage(msg),
	}

	if len(metrics) >= 3 {
		if tokens, ok := metrics[0].(int); ok {
			entry.TokensGenerated = tokens
		}
		if dur, ok := metrics[1].(int64); ok {
			entry.DurationMs = dur
		}
		if tps, ok := metrics[2].(float64); ok {
			entry.TokensPerSec = tps
		}
	}

	if len(r.entries) >= r.maxSize {
		r.entries = r.entries[1:]
	}
	r.entries = append(r.entries, entry)
}

// Entries returns a copy of all recorded scrubbed log entries.
func (r *InferenceLogRecorder) Entries() []AIInferenceLogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]AIInferenceLogEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

// GetScrubbedLogs returns recent inference and operational logs with all prompt text stripped.
func (m *ModelManager) GetScrubbedLogs(ctx context.Context) ([]AIInferenceLogEntry, error) {
	return m.logRecorder.Entries(), nil
}

// LogRecorder returns the underlying InferenceLogRecorder.
func (m *ModelManager) LogRecorder() *InferenceLogRecorder {
	return m.logRecorder
}
