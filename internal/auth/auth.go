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
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var (
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong    = errors.New("password must not exceed 256 characters")
	ErrInvalidEmail       = errors.New("invalid email address format")
	ErrAlreadyConfigured  = errors.New("appliance first-run setup has already been completed")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session has expired")
	ErrUnauthorized       = errors.New("unauthorized: active session required")
	ErrForbidden          = errors.New("forbidden: administrative privileges required")
)

const (
	DefaultArgon2Time    uint32 = 1
	DefaultArgon2Memory  uint32 = 64 * 1024 // 64 MB
	DefaultArgon2Threads uint8  = 4
	DefaultArgon2KeyLen  uint32 = 32
	DefaultSaltLen       uint32 = 16
	MaxPasswordLength           = 256
	SessionCookieName           = "sovereign_session"
	DefaultSessionMaxAge        = 2592000 // 30 days in seconds
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail verifies email address format.
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 5 || len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// Role defines role-based access levels.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

// Session represents an authenticated appliance user session.
type Session struct {
	Token     string `json:"token"`
	Role      Role   `json:"role"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}

// SetupRequest defines the body for completing the 5-step setup wizard.
type SetupRequest struct {
	Username string   `json:"username,omitempty"`
	Password string   `json:"password"`
	Email    string   `json:"email"`
	Language string   `json:"language,omitempty"`
	Features []string `json:"features,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
}

// SetupStatusResponse matches the API contract for checking setup readiness.
type SetupStatusResponse struct {
	IsConfigured bool   `json:"is_configured"`
	Language     string `json:"language,omitempty"`
}

// LoginRequest defines credentials for administrative authentication.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SessionResponse represents the auth state and active session info.
type SessionResponse struct {
	Authenticated bool     `json:"authenticated"`
	Session       *Session `json:"session,omitempty"`
}

// Authenticator provides credential verification and session lifecycle management.
type Authenticator interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) bool
	CreateSession(ctx context.Context, username string, role Role) (*Session, error)
	ValidateSession(ctx context.Context, token string) (*Session, error)
	RevokeSession(ctx context.Context, token string) error
	IsConfigured(ctx context.Context) (bool, error)
	CompleteSetup(ctx context.Context, req SetupRequest) (*Session, error)
	Authenticate(ctx context.Context, username, password string) (*Session, error)
}

type contextKey string

const sessionContextKey contextKey = "sovereign_session"

// ContextWithSession stores the active session in context.
func ContextWithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, s)
}

// SessionFromContext retrieves the active session from context.
func SessionFromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionContextKey).(*Session)
	return s, ok
}

// ManagerConfig allows tuning Argon2id parameters (useful for tests).
type ManagerConfig struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

// Manager implements Authenticator with SQLite persistence and Argon2id.
type Manager struct {
	db      *sql.DB
	logger  *slog.Logger
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// NewManager initializes the authentication manager with default Argon2id parameters.
func NewManager(db *sql.DB, logger *slog.Logger, cfgOpts ...ManagerConfig) (*Manager, error) {
	if db == nil {
		return nil, errors.New("db cannot be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	t := DefaultArgon2Time
	mem := DefaultArgon2Memory
	threads := DefaultArgon2Threads
	keyLen := DefaultArgon2KeyLen
	saltLen := DefaultSaltLen

	if len(cfgOpts) > 0 {
		c := cfgOpts[0]
		if c.Time > 0 {
			t = c.Time
		}
		if c.Memory > 0 {
			mem = c.Memory
		}
		if c.Threads > 0 {
			threads = c.Threads
		}
		if c.KeyLen > 0 {
			keyLen = c.KeyLen
		}
		if c.SaltLen > 0 {
			saltLen = c.SaltLen
		}
	}

	m := &Manager{
		db:      db,
		logger:  logger,
		time:    t,
		memory:  mem,
		threads: threads,
		keyLen:  keyLen,
		saltLen: saltLen,
	}

	if err := m.EnsureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure auth schema: %w", err)
	}

	return m, nil
}

// EnsureSchema ensures the users and sessions tables exist.
func (m *Manager) EnsureSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'admin',
		email TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := m.db.ExecContext(ctx, schema)
	return err
}

// HashPassword generates a cryptographically secure Argon2id hash with a random salt.
func (m *Manager) HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordLength {
		return "", ErrPasswordTooLong
	}

	salt := make([]byte, m.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, m.time, m.memory, m.threads, m.keyLen)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, m.memory, m.time, m.threads, saltB64, hashB64), nil
}

// VerifyPassword checks if a plaintext password matches an Argon2id PHC hash.
func (m *Manager) VerifyPassword(password, encodedHash string) bool {
	if password == "" || encodedHash == "" || len(password) > MaxPasswordLength {
		return false
	}

	parts := strings.Split(encodedHash, "$")
	// Format: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	// parts: ["", "argon2id", "v=19", "m=65536,t=1,p=4", saltB64, hashB64]
	if len(parts) != 6 {
		return false
	}
	if parts[1] != "argon2id" {
		return false
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}

	var memory, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &t, &p); err != nil {
		return false
	}
	if memory == 0 || t == 0 || p == 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expectedHash) == 0 {
		return false
	}

	actualHash := argon2.IDKey([]byte(password), salt, t, memory, p, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(expectedHash, actualHash) == 1
}

// IsConfigured returns true if first-run setup has completed.
func (m *Manager) IsConfigured(ctx context.Context) (bool, error) {
	var val string
	err := m.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'setup_completed'").Scan(&val)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check setup status: %w", err)
	}
	return val == "true", nil
}

// CompleteSetup processes the Step 5 wizard submission, records administrator credentials,
// marks the appliance as configured, and returns an active admin session.
func (m *Manager) CompleteSetup(ctx context.Context, req SetupRequest) (*Session, error) {
	configured, err := m.IsConfigured(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to verify setup status: %w", err)
	}
	if configured {
		return nil, ErrAlreadyConfigured
	}

	if len(req.Password) < 8 {
		return nil, ErrPasswordTooShort
	}
	if len(req.Password) > MaxPasswordLength {
		return nil, ErrPasswordTooLong
	}

	if !IsValidEmail(req.Email) {
		return nil, ErrInvalidEmail
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "admin"
	}

	lang := strings.TrimSpace(req.Language)
	if lang == "" {
		lang = "en"
	}

	hash, err := m.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash administrator password: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin setup transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify setup_completed inside transaction to prevent TOCTOU race
	var existingSetup string
	err = tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'setup_completed'").Scan(&existingSetup)
	if err == nil && existingSetup == "true" {
		return nil, ErrAlreadyConfigured
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, email, updated_at)
		VALUES (?, ?, 'admin', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash, email = excluded.email, role = 'admin', updated_at = CURRENT_TIMESTAMP
	`, username, hash, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to save admin user: %w", err)
	}

	settingsToSet := map[string]string{
		"setup_completed": "true",
		"language":        lang,
		"admin_email":     req.Email,
	}
	if len(req.Features) > 0 {
		settingsToSet["features"] = strings.Join(req.Features, ",")
	}
	if req.Hostname != "" {
		settingsToSet["hostname"] = req.Hostname
	}

	for k, v := range settingsToSet {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO settings (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
		`, k, v)
		if err != nil {
			return nil, fmt.Errorf("failed to persist setting %q: %w", k, err)
		}
	}

	session, err := m.CreateSessionTx(ctx, tx, username, RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to issue admin session inside setup transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit setup transaction: %w", err)
	}

	return session, nil
}

// Authenticate verifies username and password, returning an active session upon success.
func (m *Manager) Authenticate(ctx context.Context, username, password string) (*Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" || len(password) > MaxPasswordLength {
		return nil, ErrInvalidCredentials
	}

	var hash, roleStr string
	err := m.db.QueryRowContext(ctx, `
		SELECT password_hash, role FROM users WHERE username = ?
	`, username).Scan(&hash, &roleStr)
	if err == sql.ErrNoRows {
		// Run a dummy Argon2id verification to equalize execution time and prevent username enumeration
		dummyHash := "$argon2id$v=19$m=65536,t=1,p=4$c29tZXNhbHQxMjM0NTY3OA$dGVzdGhhc2hkdW1teTEyMzQ1Njc4OWFiY2RlZg"
		_ = m.VerifyPassword(password, dummyHash)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if !m.VerifyPassword(password, hash) {
		return nil, ErrInvalidCredentials
	}

	return m.CreateSession(ctx, username, Role(roleStr))
}

// CreateSessionTx generates a new session and persists it in SQLite within the provided transaction.
func (m *Manager) CreateSessionTx(ctx context.Context, tx *sql.Tx, username string, role Role) (*Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random session token: %w", err)
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(time.Duration(DefaultSessionMaxAge) * time.Second).Unix()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (token, username, role, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, username, string(role), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to persist session record: %w", err)
	}

	return &Session{
		Token:     token,
		Username:  username,
		Role:      role,
		ExpiresAt: expiresAt,
	}, nil
}

// CreateSession generates a new cryptographically secure session and persists it in SQLite.
func (m *Manager) CreateSession(ctx context.Context, username string, role Role) (*Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random session token: %w", err)
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(time.Duration(DefaultSessionMaxAge) * time.Second).Unix()

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO sessions (token, username, role, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, username, string(role), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to persist session record: %w", err)
	}

	return &Session{
		Token:     token,
		Username:  username,
		Role:      role,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateSession checks token validity in SQLite, expiring stale sessions if found.
func (m *Manager) ValidateSession(ctx context.Context, token string) (*Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrSessionNotFound
	}

	var username, roleStr string
	var expiresAt int64
	err := m.db.QueryRowContext(ctx, `
		SELECT username, role, expires_at FROM sessions WHERE token = ?
	`, token).Scan(&username, &roleStr, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	if time.Now().Unix() > expiresAt {
		_, _ = m.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
		return nil, ErrSessionExpired
	}

	return &Session{
		Token:     token,
		Username:  username,
		Role:      Role(roleStr),
		ExpiresAt: expiresAt,
	}, nil
}

// RevokeSession deletes the session record from SQLite.
func (m *Manager) RevokeSession(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

// GetSessionFromRequest inspects cookies (sovereign_session) and Authorization Bearer headers.
func (m *Manager) GetSessionFromRequest(r *http.Request) (*Session, error) {
	// 1. Try sovereign_session cookie
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		sess, err := m.ValidateSession(r.Context(), cookie.Value)
		if err == nil {
			return sess, nil
		}
	}

	// 2. Try Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[7:])
		if token != "" {
			sess, err := m.ValidateSession(r.Context(), token)
			if err == nil {
				return sess, nil
			}
		}
	}

	return nil, ErrSessionNotFound
}

// SetSessionCookie sets the HttpOnly, SameSite=Strict session cookie.
func (m *Manager) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   DefaultSessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie unsets the session cookie.
func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})
}

// PruneExpiredSessions removes stale sessions whose expires_at is before now.
func (m *Manager) PruneExpiredSessions(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to prune expired sessions: %w", err)
	}
	return nil
}

// RequireRole returns middleware checking that the caller has at least the specified role.
func (m *Manager) RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := m.GetSessionFromRequest(r)
			if err != nil || session == nil {
				writeProblem(w, http.StatusUnauthorized, "Unauthorized", "Authentication required", r.URL.Path)
				return
			}

			// Role hierarchy: Admin (highest) > User > Guest (lowest)
			allowed := false
			switch role {
			case RoleAdmin:
				allowed = session.Role == RoleAdmin
			case RoleUser:
				allowed = session.Role == RoleAdmin || session.Role == RoleUser
			case RoleGuest:
				allowed = session.Role == RoleAdmin || session.Role == RoleUser || session.Role == RoleGuest
			default:
				allowed = false
			}

			if !allowed {
				writeProblem(w, http.StatusForbidden, "Forbidden", "Insufficient privileges", r.URL.Path)
				return
			}

			ctx := ContextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin protects endpoints requiring admin role privileges.
func (m *Manager) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireRole(RoleAdmin)(next)
}

func writeProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":     fmt.Sprintf("https://sovereign.local/errors/http-%d", status),
		"title":    title,
		"status":   status,
		"detail":   detail,
		"instance": instance,
	})
}
