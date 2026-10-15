package auth_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sovereign/internal/auth"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

// fastTestManager creates a Manager with smaller memory/time for rapid test suite execution
func fastTestManager(t *testing.T, db *sql.DB) *auth.Manager {
	t.Helper()
	mgr, err := auth.NewManager(db, nil, auth.ManagerConfig{
		Time:    1,
		Memory:  8 * 1024, // 8MB for fast test execution
		Threads: 2,
		KeyLen:  32,
		SaltLen: 16,
	})
	if err != nil {
		t.Fatalf("failed to create auth manager: %v", err)
	}
	return mgr
}

func TestArgon2idHashingAndVerification(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)

	password := "SecureAdminPass123!"

	hash1, err := mgr.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !strings.HasPrefix(hash1, "$argon2id$v=19$m=") {
		t.Fatalf("unexpected hash format: %s", hash1)
	}

	// Verify matching password
	if !mgr.VerifyPassword(password, hash1) {
		t.Fatalf("expected password verification to succeed")
	}

	// Verify wrong password
	if mgr.VerifyPassword("WrongPassword123!", hash1) {
		t.Fatalf("expected verification to fail for wrong password")
	}

	// Verify empty password
	if mgr.VerifyPassword("", hash1) {
		t.Fatalf("expected verification to fail for empty password")
	}

	// Verify corrupted hash
	if mgr.VerifyPassword(password, "invalid-hash-string") {
		t.Fatalf("expected verification to fail for invalid hash format")
	}

	// Verify salt randomness: two hashes of same password must differ
	hash2, err := mgr.HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}
	if hash1 == hash2 {
		t.Fatalf("expected different salts to produce different hashes")
	}
}

func TestSetupValidation(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	// 1. Password too short (< 8 chars)
	_, err := mgr.CompleteSetup(ctx, auth.SetupRequest{
		Password: "short",
		Email:    "admin@sovereign.local",
	})
	if err != auth.ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got: %v", err)
	}

	// 2. Invalid email format
	_, err = mgr.CompleteSetup(ctx, auth.SetupRequest{
		Password: "validPassword123",
		Email:    "invalid-email-no-at",
	})
	if err != auth.ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got: %v", err)
	}

	// 3. Successful setup
	sess, err := mgr.CompleteSetup(ctx, auth.SetupRequest{
		Username: "admin",
		Password: "ValidPassword123!",
		Email:    "admin@sovereign.local",
		Language: "de",
		Features: []string{"video", "audio", "dms", "ai"},
		Hostname: "sovereign.local",
	})
	if err != nil {
		t.Fatalf("CompleteSetup failed: %v", err)
	}
	if sess == nil || sess.Token == "" || sess.Role != auth.RoleAdmin {
		t.Fatalf("expected valid admin session, got: %+v", sess)
	}

	// 4. Verify IsConfigured is now true
	configured, err := mgr.IsConfigured(ctx)
	if err != nil || !configured {
		t.Fatalf("expected IsConfigured to be true, got: %v, err: %v", configured, err)
	}

	// 5. Duplicate setup should be rejected
	_, err = mgr.CompleteSetup(ctx, auth.SetupRequest{
		Password: "AnotherValidPass123!",
		Email:    "another@sovereign.local",
	})
	if err != auth.ErrAlreadyConfigured {
		t.Fatalf("expected ErrAlreadyConfigured on re-run, got: %v", err)
	}
}

func TestAuthenticationFlow(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	password := "HouseholdAdminPass2026!"
	_, err := mgr.CompleteSetup(ctx, auth.SetupRequest{
		Username: "admin",
		Password: password,
		Email:    "lead@family.net",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Correct credentials
	sess, err := mgr.Authenticate(ctx, "admin", password)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if sess.Role != auth.RoleAdmin || sess.Username != "admin" {
		t.Fatalf("unexpected session details: %+v", sess)
	}

	// Wrong password
	_, err = mgr.Authenticate(ctx, "admin", "IncorrectPassword!")
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}

	// Unknown username
	_, err = mgr.Authenticate(ctx, "unknown_user", password)
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got: %v", err)
	}
}

func TestSessionLifecycleAndExpiry(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	// Create session
	sess, err := mgr.CreateSession(ctx, "testuser", auth.RoleUser)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Validate valid session
	validated, err := mgr.ValidateSession(ctx, sess.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if validated.Username != "testuser" || validated.Role != auth.RoleUser {
		t.Fatalf("mismatched session details: %+v", validated)
	}

	// Revoke session
	if err := mgr.RevokeSession(ctx, sess.Token); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}

	// Validate revoked session
	_, err = mgr.ValidateSession(ctx, sess.Token)
	if err != auth.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after revocation, got: %v", err)
	}

	// Test expired session
	expiredToken := "expired-token-12345"
	pastExpiry := time.Now().Add(-1 * time.Hour).Unix()
	_, err = db.Exec("INSERT INTO sessions (token, username, role, expires_at) VALUES (?, ?, ?, ?)",
		expiredToken, "expireduser", "user", pastExpiry)
	if err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	_, err = mgr.ValidateSession(ctx, expiredToken)
	if err != auth.ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got: %v", err)
	}
}

func TestRBACMiddleware(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	adminSess, err := mgr.CreateSession(ctx, "admin", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	userSess, err := mgr.CreateSession(ctx, "regular_user", auth.RoleUser)
	if err != nil {
		t.Fatalf("failed to create user session: %v", err)
	}

	protectedHandler := mgr.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := auth.SessionFromContext(r.Context())
		if !ok || sess == nil {
			t.Errorf("session missing from request context")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("admin-ok"))
	}))

	// Case 1: Unauthenticated request -> HTTP 401
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rr1 := httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got: %d", rr1.Code)
	}

	// Case 2: User role session -> HTTP 403
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req2.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: userSess.Token,
	})
	rr2 := httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got: %d", rr2.Code)
	}

	// Case 3: Admin role session via Cookie -> HTTP 200
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req3.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: adminSess.Token,
	})
	rr3 := httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", rr3.Code)
	}
	if rr3.Body.String() != "admin-ok" {
		t.Fatalf("unexpected response body: %s", rr3.Body.String())
	}

	// Case 4: Admin role session via Authorization: Bearer -> HTTP 200
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req4.Header.Set("Authorization", "Bearer "+adminSess.Token)
	rr4 := httptest.NewRecorder()
	protectedHandler.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with Bearer header, got: %d", rr4.Code)
	}

	// Case 5: Cookie setting helpers
	w := httptest.NewRecorder()
	mgr.SetSessionCookie(w, "sample-token")
	cookieHeader := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookieHeader, "sovereign_session=sample-token") ||
		!strings.Contains(cookieHeader, "HttpOnly") ||
		!strings.Contains(cookieHeader, "SameSite=Strict") {
		t.Fatalf("SetSessionCookie set unexpected cookie header: %s", cookieHeader)
	}

	wClear := httptest.NewRecorder()
	mgr.ClearSessionCookie(wClear)
	clearHeader := wClear.Header().Get("Set-Cookie")
	if !strings.Contains(clearHeader, "Max-Age=0") && !strings.Contains(clearHeader, "Max-Age=-1") {
		t.Fatalf("ClearSessionCookie did not expire cookie: %s", clearHeader)
	}
}
