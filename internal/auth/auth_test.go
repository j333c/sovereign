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

func TestVerifyPasswordEdgeCases(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)

	// 1. Zero cost parameters must return false, not panic
	zeroCostHash := "$argon2id$v=19$m=0,t=0,p=0$c29tZXNhbHQ$dGVzdGhhc2g"
	if mgr.VerifyPassword("password", zeroCostHash) {
		t.Fatalf("expected false for zero-cost parameters")
	}

	// 2. Empty salt must return false
	emptySaltHash := "$argon2id$v=19$m=65536,t=1,p=4$$dGVzdGhhc2g"
	if mgr.VerifyPassword("password", emptySaltHash) {
		t.Fatalf("expected false for empty salt")
	}

	// 3. Empty hash component must return false
	emptyHashComponent := "$argon2id$v=19$m=65536,t=1,p=4$c29tZXNhbHQ$"
	if mgr.VerifyPassword("password", emptyHashComponent) {
		t.Fatalf("expected false for empty hash component")
	}

	// 4. Excessive password length (> 256) rejected
	longPassword := strings.Repeat("A", 300)
	_, err := mgr.HashPassword(longPassword)
	if err != auth.ErrPasswordTooLong {
		t.Fatalf("expected ErrPasswordTooLong for 300-char password, got: %v", err)
	}
}

func TestRoleHierarchy(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	adminSess, _ := mgr.CreateSession(ctx, "admin", auth.RoleAdmin)
	userSess, _ := mgr.CreateSession(ctx, "user", auth.RoleUser)
	guestSess, _ := mgr.CreateSession(ctx, "guest", auth.RoleGuest)

	userLevelHandler := mgr.RequireRole(auth.RoleUser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user-level-ok"))
	}))

	// Admin accesses user level -> 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/user/resource", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: adminSess.Token})
	rrAdmin := httptest.NewRecorder()
	userLevelHandler.ServeHTTP(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusOK {
		t.Fatalf("expected Admin to access RoleUser endpoint, got: %d", rrAdmin.Code)
	}

	// User accesses user level -> 200 OK
	reqUser := httptest.NewRequest(http.MethodGet, "/api/v1/user/resource", nil)
	reqUser.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: userSess.Token})
	rrUser := httptest.NewRecorder()
	userLevelHandler.ServeHTTP(rrUser, reqUser)
	if rrUser.Code != http.StatusOK {
		t.Fatalf("expected User to access RoleUser endpoint, got: %d", rrUser.Code)
	}

	// Guest accesses user level -> 403 Forbidden
	reqGuest := httptest.NewRequest(http.MethodGet, "/api/v1/user/resource", nil)
	reqGuest.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: guestSess.Token})
	rrGuest := httptest.NewRecorder()
	userLevelHandler.ServeHTTP(rrGuest, reqGuest)
	if rrGuest.Code != http.StatusForbidden {
		t.Fatalf("expected Guest to be forbidden from RoleUser endpoint, got: %d", rrGuest.Code)
	}
}

func TestPruneExpiredSessions(t *testing.T) {
	db := newTestDB(t)
	mgr := fastTestManager(t, db)
	ctx := context.Background()

	// 1. Create a valid session and an expired session
	validSess, err := mgr.CreateSession(ctx, "valid_user", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	expiredToken := "expired-token-to-prune"
	_, err = db.Exec("INSERT INTO sessions (token, username, role, expires_at) VALUES (?, ?, ?, ?)",
		expiredToken, "expired_user", "admin", time.Now().Add(-2*time.Hour).Unix())
	if err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	// 2. Prune expired sessions
	if err := mgr.PruneExpiredSessions(ctx); err != nil {
		t.Fatalf("PruneExpiredSessions failed: %v", err)
	}

	// 3. Valid session should still exist
	valSess, err := mgr.ValidateSession(ctx, validSess.Token)
	if err != nil || valSess == nil {
		t.Fatalf("expected valid session to remain, got err: %v", err)
	}

	// 4. Expired session should have been deleted from SQLite
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = ?", expiredToken).Scan(&count)
	if count != 0 {
		t.Fatalf("expected expired session to be pruned, found count: %d", count)
	}
}

func TestCompleteSetup_Atomicity(t *testing.T) {
	ctx := context.Background()

	t.Run("Injected session creation failure rolls back users and settings", func(t *testing.T) {
		db := newTestDB(t)
		mgr := fastTestManager(t, db)

		// Inject failure right when sessions table is inserted into
		_, err := db.Exec(`
			CREATE TRIGGER fail_sessions_insert BEFORE INSERT ON sessions
			BEGIN
				SELECT RAISE(ABORT, 'simulated session creation failure');
			END;
		`)
		if err != nil {
			t.Fatalf("failed to create failure trigger: %v", err)
		}

		// Attempt setup wizard completion
		sess, err := mgr.CompleteSetup(ctx, auth.SetupRequest{
			Username: "admin",
			Password: "ValidPassword123!",
			Email:    "admin@sovereign.local",
			Language: "en",
			Features: []string{"video", "audio"},
			Hostname: "sovereign.local",
		})
		if err == nil {
			t.Fatalf("expected CompleteSetup to fail due to injected trigger, got session: %+v", sess)
		}

		// Verify zero rows in users table
		var userCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
			t.Fatalf("failed to count users: %v", err)
		}
		if userCount != 0 {
			t.Fatalf("atomicity violation: expected 0 users after rollback, found %d", userCount)
		}

		// Verify zero rows in settings table
		var settingsCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&settingsCount); err != nil {
			t.Fatalf("failed to count settings: %v", err)
		}
		if settingsCount != 0 {
			t.Fatalf("atomicity violation: expected 0 settings after rollback, found %d", settingsCount)
		}

		// Verify zero rows in sessions table
		var sessionCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount); err != nil {
			t.Fatalf("failed to count sessions: %v", err)
		}
		if sessionCount != 0 {
			t.Fatalf("atomicity violation: expected 0 sessions after rollback, found %d", sessionCount)
		}

		// Verify IsConfigured is still false
		configured, err := mgr.IsConfigured(ctx)
		if err != nil {
			t.Fatalf("IsConfigured check failed: %v", err)
		}
		if configured {
			t.Fatalf("expected IsConfigured to be false after rollback")
		}
	})

	t.Run("Injected settings failure rolls back user creation", func(t *testing.T) {
		db := newTestDB(t)
		mgr := fastTestManager(t, db)

		// Inject failure when settings table is inserted into
		_, err := db.Exec(`
			CREATE TRIGGER fail_settings_insert BEFORE INSERT ON settings
			BEGIN
				SELECT RAISE(ABORT, 'simulated settings persistence failure');
			END;
		`)
		if err != nil {
			t.Fatalf("failed to create failure trigger: %v", err)
		}

		sess, err := mgr.CompleteSetup(ctx, auth.SetupRequest{
			Username: "admin",
			Password: "ValidPassword123!",
			Email:    "admin@sovereign.local",
		})
		if err == nil {
			t.Fatalf("expected CompleteSetup to fail due to injected trigger, got session: %+v", sess)
		}

		var userCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
		if userCount != 0 {
			t.Fatalf("expected 0 users after rollback, found %d", userCount)
		}

		var settingsCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&settingsCount)
		if settingsCount != 0 {
			t.Fatalf("expected 0 settings after rollback, found %d", settingsCount)
		}

		var sessionCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
		if sessionCount != 0 {
			t.Fatalf("expected 0 sessions after rollback, found %d", sessionCount)
		}
	})

	t.Run("CreateSessionTx returns error when tx is nil", func(t *testing.T) {
		db := newTestDB(t)
		defer db.Close()
		mgr := fastTestManager(t, db)

		sess, err := mgr.CreateSessionTx(context.Background(), nil, "admin", auth.RoleAdmin)
		if err == nil {
			t.Fatalf("expected error when tx is nil, got session: %+v", sess)
		}
	})
}

