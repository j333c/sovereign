package ingress

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaddyClient_BuildConfigurationPayload(t *testing.T) {
	client := NewCaddyClient("http://caddy:2019", nil)

	routes := []RouteConfig{
		{
			Service:    "jellyfin",
			Port:       8096,
			Subpath:    "/media",
			TLSEnabled: false,
		},
		{
			Service:    "paperless",
			Port:       8000,
			Subpath:    "docs/",
			TLSEnabled: false,
		},
		// Empty and duplicate subpaths should be guarded and ignored
		{
			Service: "ignored-empty",
			Port:    7000,
			Subpath: "/",
		},
		{
			Service: "ignored-duplicate",
			Port:    7001,
			Subpath: "/media",
		},
	}

	cfg := client.BuildCaddyConfig(routes)

	apps, ok := cfg["apps"].(map[string]any)
	if !ok {
		t.Fatalf("expected apps block in caddy config")
	}

	httpApp, ok := apps["http"].(map[string]any)
	if !ok {
		t.Fatalf("expected http app block in caddy config")
	}

	servers, ok := httpApp["servers"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers block in caddy config")
	}

	srv0, ok := servers["srv0"].(map[string]any)
	if !ok {
		t.Fatalf("expected srv0 server block in caddy config")
	}

	autoHTTPS, ok := srv0["auto_https"].(map[string]any)
	if !ok || autoHTTPS["disable_redirects"] != true {
		t.Fatalf("expected auto_https.disable_redirects = true, got %v", srv0["auto_https"])
	}

	caddyRoutes, ok := srv0["routes"].([]map[string]any)
	if !ok {
		t.Fatalf("expected routes list in srv0 block")
	}

	// Route 0: /api and /api/* -> sovereign-daemon:8080
	// Route 1: /media -> jellyfin:8096
	// Route 2: /docs -> paperless:8000
	// Route 3: fallback -> sovereign-web:3000
	if len(caddyRoutes) != 4 {
		t.Fatalf("expected 4 routes in caddy config, got %d", len(caddyRoutes))
	}

	// Verify route 0: API
	if caddyRoutes[0]["@id"] != "route-api" {
		t.Errorf("expected route 0 to have id route-api, got %v", caddyRoutes[0]["@id"])
	}
	matchList, ok := caddyRoutes[0]["match"].([]map[string]any)
	if !ok || len(matchList) == 0 {
		t.Fatalf("expected match block in route-api")
	}
	paths, ok := matchList[0]["path"].([]string)
	if !ok || len(paths) != 2 || paths[0] != "/api" || paths[1] != "/api/*" {
		t.Errorf("expected route-api path match [/api /api/*], got %v", matchList[0]["path"])
	}

	// Verify route 1: Jellyfin prefix rewrite and upstream
	jellyfinRoute := caddyRoutes[1]
	if jellyfinRoute["@id"] != "route-spoke-media" {
		t.Errorf("expected jellyfin route id route-spoke-media, got %v", jellyfinRoute["@id"])
	}
	handles, ok := jellyfinRoute["handle"].([]map[string]any)
	if !ok || len(handles) < 2 {
		t.Fatalf("expected at least 2 handle steps (rewrite + reverse_proxy) in spoke route")
	}
	if handles[0]["handler"] != "rewrite" || handles[0]["strip_path_prefix"] != "/media" {
		t.Errorf("expected rewrite handler with strip_path_prefix /media, got %v", handles[0])
	}
	if handles[1]["handler"] != "reverse_proxy" {
		t.Errorf("expected reverse_proxy handler, got %v", handles[1])
	}
	headers, ok := handles[1]["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers map in reverse_proxy")
	}
	requestHeaders, ok := headers["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request headers map in reverse_proxy")
	}
	setHeaders, ok := requestHeaders["set"].(map[string][]string)
	if !ok || len(setHeaders["X-Forwarded-Host"]) == 0 || len(setHeaders["X-Forwarded-Proto"]) == 0 || len(setHeaders["X-Forwarded-For"]) == 0 {
		t.Errorf("missing proxy forwarded headers: %v", setHeaders)
	}

	// Verify route 3: Web Fallback
	webRoute := caddyRoutes[3]
	if webRoute["@id"] != "route-web" {
		t.Errorf("expected last route to be route-web, got %v", webRoute["@id"])
	}
}

func TestCaddyClient_SyncRoutes_Success(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedContentType string
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")

		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &receivedBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewCaddyClient(server.URL, nil)

	routes := []RouteConfig{
		{Service: "jellyfin", Port: 8096, Subpath: "/media"},
	}

	err := client.SyncRoutes(context.Background(), routes)
	if err != nil {
		t.Fatalf("expected successful SyncRoutes, got error: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedPath != "/load" {
		t.Errorf("expected path /load, got %s", receivedPath)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected application/json content type, got %s", receivedContentType)
	}
	if receivedBody == nil {
		t.Fatalf("expected non-empty json body sent to /load")
	}
}

func TestCaddyClient_SyncRoutes_CaddyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid configuration syntax"}`))
	}))
	defer server.Close()

	client := NewCaddyClient(server.URL, nil)

	err := client.SyncRoutes(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected SyncRoutes to fail on HTTP 400, got nil")
	}
}

func TestCaddyClient_SyncRoutes_Unreachable(t *testing.T) {
	client := NewCaddyClient("http://127.0.0.1:59998", nil)

	err := client.SyncRoutes(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected SyncRoutes to fail on unreachable host, got nil")
	}
}

func TestCaddyClient_GetStatus_SuccessAndError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"apps": {"http": {}}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewCaddyClient(server.URL, nil)

	status, err := client.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("expected successful GetStatus, got err: %v", err)
	}
	if status["status"] != "ok" || status["connected"] != true {
		t.Errorf("expected ok status and connected=true, got %v", status)
	}

	// Test unreachable
	offlineClient := NewCaddyClient("http://127.0.0.1:59998", nil)
	errStatus, err := offlineClient.GetStatus(context.Background())
	if err == nil {
		t.Fatalf("expected error from offline client, got nil")
	}
	if errStatus["status"] != "error" || errStatus["connected"] != false {
		t.Errorf("expected status=error and connected=false, got %v", errStatus)
	}
}
