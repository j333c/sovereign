// Package ingress coordinates network ingress, route synchronization, and TLS certificate
// automation with Caddy via the Caddy Admin API.
//
// Invariants & Boundaries:
// - Control Plane Network: Communicates with Caddy via http://caddy:2019 over sovereign-control.
// - Dual-Path Ingress: Manages both direct dedicated ports and subpath routing (/media, /docs, /chat, /music, /audiobooks).
// - Dynamic TLS: Synchronizes Let's Encrypt / ACME, internal CA, and user-provided certificates.
package ingress

import (
	"context"
)

// RouteConfig specifies an upstream reverse-proxy route.
type RouteConfig struct {
	Service  string
	Port     int
	Subpath  string
	TLSEnabled bool
}

// IngressManager abstracts dynamic ingress updates.
type IngressManager interface {
	SyncRoutes(ctx context.Context, routes []RouteConfig) error
	GetStatus(ctx context.Context) (map[string]any, error)
}
