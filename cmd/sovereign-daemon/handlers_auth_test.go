package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServer_AuthAndSetupWizardFlow(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Initial boot: verify setup-status reports is_configured: false
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-status", nil)
	wStatus := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 from setup-status, got: %d", wStatus.Code)
	}

	var statusMap map[string]any
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusMap); err != nil {
		t.Fatalf("failed to decode setup status response: %v", err)
	}
	if isConfigured, ok := statusMap["is_configured"].(bool); !ok || isConfigured {
		t.Fatalf("expected is_configured: false, got: %v", statusMap["is_configured"])
	}

	// 2. Setup validation: password < 8 characters
	bodyShortPass := bytes.NewBufferString(`{"password": "short", "email": "admin@family.net"}`)
	reqShortPass := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bodyShortPass)
	wShortPass := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wShortPass, reqShortPass)
	if wShortPass.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got: %d", wShortPass.Code)
	}

	// 3. Setup validation: invalid email
	bodyBadEmail := bytes.NewBufferString(`{"password": "validPassword123!", "email": "not-an-email"}`)
	reqBadEmail := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bodyBadEmail)
	wBadEmail := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBadEmail, reqBadEmail)
	if wBadEmail.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got: %d", wBadEmail.Code)
	}

	// 4. Valid setup submission (Step 5)
	setupPayload := `{
		"username": "admin",
		"password": "HouseholdMasterKey2026!",
		"email": "alex@family.net",
		"language": "de",
		"features": ["video", "audio", "dms", "ai"],
		"hostname": "sovereign.local"
	}`
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(setupPayload))
	wSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSetup, reqSetup)
	if wSetup.Code != http.StatusOK {
		t.Fatalf("expected 200 for setup completion, got: %d: %s", wSetup.Code, wSetup.Body.String())
	}

	// Check cookie was set
	cookieHeader := wSetup.Header().Get("Set-Cookie")
	if !strings.Contains(cookieHeader, "sovereign_session=") ||
		!strings.Contains(cookieHeader, "HttpOnly") ||
		!strings.Contains(cookieHeader, "SameSite=Strict") {
		t.Fatalf("expected HttpOnly SameSite=Strict cookie in Set-Cookie, got: %s", cookieHeader)
	}

	var setupRes map[string]any
	if err := json.Unmarshal(wSetup.Body.Bytes(), &setupRes); err != nil {
		t.Fatalf("failed to decode setup response: %v", err)
	}
	if configured, ok := setupRes["is_configured"].(bool); !ok || !configured {
		t.Fatalf("expected is_configured: true in setup response, got: %v", setupRes)
	}

	// Extract session token from cookie
	cookies := wSetup.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "sovereign_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected sovereign_session cookie to be returned")
	}

	// 5. Subsequent setup-status should now return is_configured: true
	reqStatus2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-status", nil)
	wStatus2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wStatus2, reqStatus2)
	if wStatus2.Code != http.StatusOK {
		t.Fatalf("expected 200 from setup-status, got: %d", wStatus2.Code)
	}
	var statusMap2 map[string]any
	_ = json.Unmarshal(wStatus2.Body.Bytes(), &statusMap2)
	if isConfigured, ok := statusMap2["is_configured"].(bool); !ok || !isConfigured {
		t.Fatalf("expected is_configured: true post-setup, got: %v", statusMap2)
	}

	// 6. Duplicate setup attempt must be rejected with 409 Conflict Problem Details
	reqDupSetup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(setupPayload))
	wDupSetup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDupSetup, reqDupSetup)
	if wDupSetup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate setup, got: %d", wDupSetup.Code)
	}

	// 7. Session inspection: unauthenticated vs authenticated
	reqNoSess := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	wNoSess := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wNoSess, reqNoSess)
	if wNoSess.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated session check, got: %d", wNoSess.Code)
	}

	reqWithSess := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	reqWithSess.AddCookie(sessionCookie)
	wWithSess := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wWithSess, reqWithSess)
	if wWithSess.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated session check, got: %d: %s", wWithSess.Code, wWithSess.Body.String())
	}

	// 8. Protected Settings endpoint: unauthenticated vs authenticated
	reqSettingsNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	wSettingsNoAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSettingsNoAuth, reqSettingsNoAuth)
	if wSettingsNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /api/v1/settings without auth, got: %d", wSettingsNoAuth.Code)
	}

	reqSettingsAuth := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	reqSettingsAuth.AddCookie(sessionCookie)
	wSettingsAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wSettingsAuth, reqSettingsAuth)
	if wSettingsAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/settings with session cookie, got: %d", wSettingsAuth.Code)
	}
	var settingsRes map[string]any
	_ = json.Unmarshal(wSettingsAuth.Body.Bytes(), &settingsRes)
	if settingsRes["language"] != "de" || settingsRes["admin_email"] != "alex@family.net" {
		t.Errorf("unexpected settings content: %+v", settingsRes)
	}
	if settingsRes["features"] != "video,audio,dms,ai" {
		t.Errorf("expected features 'video,audio,dms,ai', got: %v", settingsRes["features"])
	}
	if settingsRes["hostname"] != "sovereign.local" {
		t.Errorf("expected hostname 'sovereign.local', got: %v", settingsRes["hostname"])
	}

	// 9. Update Settings via POST /api/v1/settings: unauthenticated vs authenticated
	reqUpdateNoAuth := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewBufferString(`{"language": "en"}`))
	wUpdateNoAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUpdateNoAuth, reqUpdateNoAuth)
	if wUpdateNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on POST /api/v1/settings without auth, got: %d", wUpdateNoAuth.Code)
	}

	reqUpdateAuth := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewBufferString(`{"language": "en"}`))
	reqUpdateAuth.AddCookie(sessionCookie)
	wUpdateAuth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wUpdateAuth, reqUpdateAuth)
	if wUpdateAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST /api/v1/settings with auth, got: %d", wUpdateAuth.Code)
	}

	// Verify setting updated
	reqCheckUpdated := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	reqCheckUpdated.AddCookie(sessionCookie)
	wCheckUpdated := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wCheckUpdated, reqCheckUpdated)
	var updatedSettings map[string]any
	_ = json.Unmarshal(wCheckUpdated.Body.Bytes(), &updatedSettings)
	if updatedSettings["language"] != "en" {
		t.Fatalf("expected updated language 'en', got: %v", updatedSettings["language"])
	}

	// 10. Post-setup storage endpoint protection: unauthenticated call must return 401
	reqPostSetupStorage := httptest.NewRequest(http.MethodPost, "/api/v1/storage/initialize", bytes.NewBufferString(`{"device": "/dev/sda"}`))
	wPostSetupStorage := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wPostSetupStorage, reqPostSetupStorage)
	if wPostSetupStorage.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /api/v1/storage/initialize post-setup without auth, got: %d", wPostSetupStorage.Code)
	}

	// 11. Login flow: invalid password vs valid password
	reqBadLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username": "admin", "password": "WrongPassword!"}`))
	wBadLogin := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wBadLogin, reqBadLogin)
	if wBadLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad login, got: %d", wBadLogin.Code)
	}

	reqGoodLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username": "admin", "password": "HouseholdMasterKey2026!"}`))
	wGoodLogin := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wGoodLogin, reqGoodLogin)
	if wGoodLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid login, got: %d", wGoodLogin.Code)
	}
	loginCookieHeader := wGoodLogin.Header().Get("Set-Cookie")
	if !strings.Contains(loginCookieHeader, "sovereign_session=") {
		t.Fatalf("expected Set-Cookie on login, got: %s", loginCookieHeader)
	}

	// 12. Logout: revokes session and clears cookie
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	wLogout := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wLogout, reqLogout)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got: %d", wLogout.Code)
	}

	// Verify session is revoked
	reqRevokedCheck := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	reqRevokedCheck.AddCookie(sessionCookie)
	wRevokedCheck := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wRevokedCheck, reqRevokedCheck)
	if wRevokedCheck.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout session revocation, got: %d", wRevokedCheck.Code)
	}
}

func TestServer_ProtectedEndpoints_RequireAdminAfterSetup(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	cookie := loginTestAdmin(t, srv)

	protectedRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/api/v1/services/jellyfin/activate", ""},
		{"POST", "/api/v1/services/jellyfin/deactivate", ""},
		{"POST", "/api/v1/services/jellyfin/refresh", ""},
		{"POST", "/api/v1/optical/eject", ""},
		{"POST", "/api/v1/enrichment/scan", ""},
		{"POST", "/api/v1/transcode/queue", `{"source_path":"/data/media/raw/test.mkv"}`},
		{"GET", "/api/v1/documents/deadlines", ""},
		{"POST", "/api/v1/ai/personas/active", `{"persona_id":"legal"}`},
		{"POST", "/api/v1/ai/rag/search", `{"query":"test"}`},
		{"POST", "/api/v1/ai/rag/query", `{"query":"test"}`},
		{"GET", "/api/v1/ai/logs", ""},
	}

	for _, tc := range protectedRoutes {
		t.Run("Unauthenticated_"+tc.method+"_"+tc.path, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bodyReader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized for %s %s without cookie, got %d", tc.method, tc.path, rr.Code)
			}
		})

		t.Run("Authenticated_"+tc.method+"_"+tc.path, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bodyReader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.AddCookie(cookie)
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)

			if rr.Code == http.StatusUnauthorized {
				t.Errorf("unexpected 401 Unauthorized for authenticated %s %s", tc.method, tc.path)
			}
		})
	}
}
