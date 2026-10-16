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
		{
			Service: "caddy-internal",
			Port:    2019,
			Subpath: "/admin-api",
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
	// Route 1: /v1 and /v1/* -> sovereign-daemon:8080
	// Route 2: /media -> jellyfin:8096
	// Route 3: /docs -> paperless:8000
	// Route 4: /admin-api -> caddy-internal:2019
	// Route 5: fallback -> sovereign-web:3000
	if len(caddyRoutes) != 6 {
		t.Fatalf("expected 6 routes in caddy config, got %d", len(caddyRoutes))
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

	// Verify route 1: AI Inference API (/v1)
	if caddyRoutes[1]["@id"] != "route-v1" {
		t.Errorf("expected route 1 to have id route-v1, got %v", caddyRoutes[1]["@id"])
	}
	v1MatchList, ok := caddyRoutes[1]["match"].([]map[string]any)
	if !ok || len(v1MatchList) == 0 {
		t.Fatalf("expected match block in route-v1")
	}
	v1Paths, ok := v1MatchList[0]["path"].([]string)
	if !ok || len(v1Paths) != 2 || v1Paths[0] != "/v1" || v1Paths[1] != "/v1/*" {
		t.Errorf("expected route-v1 path match [/v1 /v1/*], got %v", v1MatchList[0]["path"])
	}

	// Verify route 2: Jellyfin prefix rewrite and upstream
	jellyfinRoute := caddyRoutes[2]
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

	// Verify fallback route (last route)
	webRoute := caddyRoutes[len(caddyRoutes)-1]
	if webRoute["@id"] != "route-web" {
		t.Errorf("expected last route to be route-web, got %v", webRoute["@id"])
	}

	// Verify dedicated spoke listener srv_8096
	srv8096, ok := servers["srv_8096"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers to contain srv_8096 for direct Jellyfin listener")
	}
	listen, ok := srv8096["listen"].([]string)
	if !ok || len(listen) != 1 || listen[0] != ":8096" {
		t.Errorf("expected srv_8096 listen [:8096], got %v", srv8096["listen"])
	}
	directRoutes, ok := srv8096["routes"].([]map[string]any)
	if !ok || len(directRoutes) == 0 {
		t.Fatalf("expected routes in srv_8096")
	}
	directHandles, ok := directRoutes[0]["handle"].([]map[string]any)
	if !ok || len(directHandles) == 0 {
		t.Fatalf("expected handle in srv_8096 route")
	}
	if directHandles[0]["handler"] != "reverse_proxy" {
		t.Errorf("expected reverse_proxy handler in direct spoke listener, got %v", directHandles[0]["handler"])
	}
	directUpstreams, ok := directHandles[0]["upstreams"].([]map[string]any)
	if !ok || len(directUpstreams) == 0 || directUpstreams[0]["dial"] != "sovereign-jellyfin:8096" {
		t.Errorf("expected upstream dial sovereign-jellyfin:8096, got %v", directHandles[0]["upstreams"])
	}

	// Verify dedicated spoke listener srv_8000 for Paperless
	srv8000, ok := servers["srv_8000"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers to contain srv_8000 for direct Paperless listener")
	}
	listen8000, ok := srv8000["listen"].([]string)
	if !ok || len(listen8000) != 1 || listen8000[0] != ":8000" {
		t.Errorf("expected srv_8000 listen [:8000], got %v", srv8000["listen"])
	}
	directRoutes8000, ok := srv8000["routes"].([]map[string]any)
	if !ok || len(directRoutes8000) == 0 {
		t.Fatalf("expected routes in srv_8000")
	}
	directHandles8000, ok := directRoutes8000[0]["handle"].([]map[string]any)
	if !ok || len(directHandles8000) == 0 {
		t.Fatalf("expected handle in srv_8000 route")
	}
	if directHandles8000[0]["handler"] != "reverse_proxy" {
		t.Errorf("expected reverse_proxy handler in srv_8000 listener, got %v", directHandles8000[0]["handler"])
	}
	directUpstreams8000, ok := directHandles8000[0]["upstreams"].([]map[string]any)
	if !ok || len(directUpstreams8000) == 0 || directUpstreams8000[0]["dial"] != "sovereign-paperless:8000" {
		t.Errorf("expected upstream dial sovereign-paperless:8000, got %v", directHandles8000[0]["upstreams"])
	}

	// Verify port 2019 is strictly excluded from spoke servers
	if _, exists := servers["srv_2019"]; exists {
		t.Errorf("port 2019 must not have a dedicated spoke listener because it is the Caddy admin port")
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

func TestCaddyClient_AudioRoutes_MusicAndAudiobooks(t *testing.T) {
	client := NewCaddyClient("http://caddy:2019", nil)

	routes := []RouteConfig{
		{
			Service: "navidrome",
			Port:    4533,
			Subpath: "/music",
		},
		{
			Service: "audiobookshelf",
			Port:    13378,
			Subpath: "/audiobooks",
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
	caddyRoutes, ok := srv0["routes"].([]map[string]any)
	if !ok {
		t.Fatalf("expected routes list in srv0 block")
	}

	// Route 0: /api
	// Route 1: /v1
	// Route 2: /music -> sovereign-navidrome:4533
	// Route 3: /audiobooks -> sovereign-audiobookshelf:13378
	// Route 4: fallback web
	if len(caddyRoutes) != 5 {
		t.Fatalf("expected 5 routes in caddy config, got %d", len(caddyRoutes))
	}

	// Verify Navidrome spoke route
	musicRoute := caddyRoutes[2]
	if musicRoute["@id"] != "route-spoke-music" {
		t.Errorf("expected music route id route-spoke-music, got %v", musicRoute["@id"])
	}
	musicHandles, ok := musicRoute["handle"].([]map[string]any)
	if !ok || len(musicHandles) < 2 {
		t.Fatalf("expected 2 handles in music route")
	}
	if musicHandles[0]["handler"] != "rewrite" || musicHandles[0]["strip_path_prefix"] != "/music" {
		t.Errorf("expected rewrite with strip_path_prefix /music, got %v", musicHandles[0])
	}
	musicProxyUpstreams := musicHandles[1]["upstreams"].([]map[string]any)
	if musicProxyUpstreams[0]["dial"] != "sovereign-navidrome:4533" {
		t.Errorf("expected dial sovereign-navidrome:4533, got %v", musicProxyUpstreams[0]["dial"])
	}

	// Verify Audiobookshelf spoke route
	absRoute := caddyRoutes[3]
	if absRoute["@id"] != "route-spoke-audiobooks" {
		t.Errorf("expected audiobooks route id route-spoke-audiobooks, got %v", absRoute["@id"])
	}
	absHandles, ok := absRoute["handle"].([]map[string]any)
	if !ok || len(absHandles) < 2 {
		t.Fatalf("expected 2 handles in audiobooks route")
	}
	if absHandles[0]["handler"] != "rewrite" || absHandles[0]["strip_path_prefix"] != "/audiobooks" {
		t.Errorf("expected rewrite with strip_path_prefix /audiobooks, got %v", absHandles[0])
	}
	absProxyUpstreams := absHandles[1]["upstreams"].([]map[string]any)
	if absProxyUpstreams[0]["dial"] != "sovereign-audiobookshelf:13378" {
		t.Errorf("expected dial sovereign-audiobookshelf:13378, got %v", absProxyUpstreams[0]["dial"])
	}

	// Verify direct port listener srv_4533
	srv4533, ok := servers["srv_4533"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers to contain srv_4533")
	}
	listen4533 := srv4533["listen"].([]string)
	if len(listen4533) != 1 || listen4533[0] != ":4533" {
		t.Errorf("expected srv_4533 listen [:4533], got %v", listen4533)
	}

	// Verify direct port listener srv_13378
	srv13378, ok := servers["srv_13378"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers to contain srv_13378")
	}
	listen13378 := srv13378["listen"].([]string)
	if len(listen13378) != 1 || listen13378[0] != ":13378" {
		t.Errorf("expected srv_13378 listen [:13378], got %v", listen13378)
	}
}
