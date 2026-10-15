// Package auth provides authentication, password hashing using Argon2id,
// session management, and RBAC authorization for appliance administrative operations.
//
// Invariants & Boundaries:
// - Password Hashes: Stored exclusively as Argon2id hashes in sovereign.db.
// - Session Security: Sessions are backed by secure HttpOnly, SameSite=Strict cookies.
// - Public Endpoints: Dashboard read-only telemetry, /drop, and /boot endpoints are open to LAN.
// - Destructive Mutations: All administrative mutations (restarts, rollbacks, settings, model swaps)
//   require verified admin session tokens.
package auth

import (
	"context"
)

// Role defines role-based access levels.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

// Session represents an authenticated appliance user session.
type Session struct {
	Token     string
	Role      Role
	Username  string
	ExpiresAt int64
}

// Authenticator provides credential verification and session lifecycle management.
type Authenticator interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) bool
	CreateSession(ctx context.Context, username string, role Role) (*Session, error)
	ValidateSession(ctx context.Context, token string) (*Session, error)
	RevokeSession(ctx context.Context, token string) error
}
