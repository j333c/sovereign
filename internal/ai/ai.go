// Package ai hosts the AI Gateway & mediation proxy, governing local LLM inference via llama-server.
//
// Invariants & Boundaries:
// - Two-Lane Priority: Arbitrates human chat vs. background document OCR/indexing extraction.
// - Safe Stub Mode: Returns RFC 7807 Problem Details (HTTP 503) when no GGUF models are provisioned in /data/models.
// - Model Verification: GGUF magic byte validation before loading.
// - Dynamic Idle Surrender: Unloads inference models and releases VRAM after 300 seconds of inactivity.
// - Air-Gapped Operation: Raw llama-server is isolated inside the sovereign-ai network without public ports.
package ai

import (
	"context"
	"net/http"
)

// InferencePriority denotes the execution lane for inference tasks.
type InferencePriority int

const (
	PriorityBackground InferencePriority = iota
	PriorityInteractive
)

// Gateway mediates inference requests to the internal llama-server spoke.
type Gateway interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	CheckModelAvailability(ctx context.Context) (bool, error)
	SurrenderIdleVRAM(ctx context.Context) error
	Governor() *RequestGovernor
	Arbitrator() *GPUArbitrator
	Close()
}

