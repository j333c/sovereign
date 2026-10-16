package ai

import (
	"encoding/json"
	"net/http"
)

// SafeStubProblemDetails defines the RFC 7807 Problem Details payload when in Safe Stub Mode.
type SafeStubProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

// SafeStubHandler returns an HTTP 503 response formatted as RFC 7807 Problem Details,
// reassuring the user with instructions to download a model in Settings rather than crashing.
func SafeStubHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Retry-After", "30")
	w.WriteHeader(http.StatusServiceUnavailable)

	instance := ""
	if r != nil && r.URL != nil {
		instance = r.URL.Path
	}

	prob := SafeStubProblemDetails{
		Type:     "https://sovereign.local/problems/no-model-provisioned",
		Title:    "No AI Model Available",
		Status:   http.StatusServiceUnavailable,
		Detail:   "No valid AI model was found in /data/models. Please download or place a GGUF model in Settings to enable AI chat and document analysis.",
		Instance: instance,
	}

	_ = json.NewEncoder(w).Encode(prob)
}

// NewSafeStubHandler returns an http.Handler serving SafeStubHandler.
func NewSafeStubHandler() http.Handler {
	return http.HandlerFunc(SafeStubHandler)
}
