package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var _ IngressManager = (*CaddyClient)(nil)

// CaddyClient communicates with the Caddy Admin API (http://caddy:2019)
// over the sovereign-control network to synchronize dynamic routes and check status.
type CaddyClient struct {
	adminURL   string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewCaddyClient initializes a new Caddy Admin API client.
func NewCaddyClient(adminURL string, logger *slog.Logger) *CaddyClient {
	if adminURL == "" {
		adminURL = "http://caddy:2019"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CaddyClient{
		adminURL: strings.TrimRight(adminURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// AdminURL returns the configured Caddy Admin API base URL.
func (c *CaddyClient) AdminURL() string {
	return c.adminURL
}

// BuildCaddyConfig constructs a Caddy JSON configuration payload
// supporting /api and /api/* proxying to sovereign-daemon:8080, subpath routing
// with prefix trimming for registered spokes, and fallback to sovereign-web:3000.
func (c *CaddyClient) BuildCaddyConfig(routes []RouteConfig) map[string]any {
	var caddyRoutes []map[string]any

	// 1. API route: reverse proxy /api and /api/* to sovereign-daemon:8080 preserving path
	caddyRoutes = append(caddyRoutes, map[string]any{
		"@id": "route-api",
		"match": []map[string]any{
			{"path": []string{"/api", "/api/*"}},
		},
		"handle": []map[string]any{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"dial": "sovereign-daemon:8080"},
				},
				"headers": map[string]any{
					"request": map[string]any{
						"set": map[string][]string{
							"Host":               {"{http.request.host}"},
							"X-Real-IP":          {"{http.request.remote.host}"},
							"X-Forwarded-For":    {"{http.request.remote.host}"},
							"X-Forwarded-Proto":  {"{http.request.scheme}"},
							"X-Forwarded-Host":   {"{http.request.host}"},
							"X-Forwarded-Prefix": {"/api"},
						},
					},
				},
			},
		},
	})

	// 2. Dynamic spoke routes: guard against empty subpaths or duplicates, subpath prefix trimming
	seenSubpaths := make(map[string]bool)
	for _, r := range routes {
		if r.Service == "" || r.Port == 0 {
			continue
		}

		trimmed := strings.Trim(r.Subpath, "/")
		if trimmed == "" {
			continue
		}

		cleanSubpath := "/" + trimmed
		if seenSubpaths[cleanSubpath] {
			continue
		}
		seenSubpaths[cleanSubpath] = true

		upstreamHost := r.Service
		if !strings.HasPrefix(upstreamHost, "sovereign-") && !strings.Contains(upstreamHost, ".") && upstreamHost != "localhost" {
			upstreamHost = "sovereign-" + upstreamHost
		}
		upstreamDial := fmt.Sprintf("%s:%d", upstreamHost, r.Port)

		caddyRoutes = append(caddyRoutes, map[string]any{
			"@id": fmt.Sprintf("route-spoke-%s", trimmed),
			"match": []map[string]any{
				{"path": []string{cleanSubpath, cleanSubpath + "/*"}},
			},
			"handle": []map[string]any{
				{
					"handler":           "rewrite",
					"strip_path_prefix": cleanSubpath,
				},
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]any{
						{"dial": upstreamDial},
					},
					"headers": map[string]any{
						"request": map[string]any{
							"set": map[string][]string{
								"Host":               {"{http.request.host}"},
								"X-Real-IP":          {"{http.request.remote.host}"},
								"X-Forwarded-For":    {"{http.request.remote.host}"},
								"X-Forwarded-Proto":  {"{http.request.scheme}"},
								"X-Forwarded-Host":   {"{http.request.host}"},
								"X-Forwarded-Prefix": {cleanSubpath},
							},
						},
					},
				},
			},
		})
	}

	// 3. Fallback route: reverse proxy / and all remaining paths to sovereign-web:3000
	caddyRoutes = append(caddyRoutes, map[string]any{
		"@id": "route-web",
		"handle": []map[string]any{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"dial": "sovereign-web:3000"},
				},
				"headers": map[string]any{
					"request": map[string]any{
						"set": map[string][]string{
							"Host":              {"{http.request.host}"},
							"X-Real-IP":         {"{http.request.remote.host}"},
							"X-Forwarded-For":   {"{http.request.remote.host}"},
							"X-Forwarded-Proto": {"{http.request.scheme}"},
							"X-Forwarded-Host":  {"{http.request.host}"},
						},
					},
				},
			},
		},
	})

	servers := map[string]any{
		"srv0": map[string]any{
			"listen": []string{":80", ":443"},
			"auto_https": map[string]any{
				"disable_redirects": true,
			},
			"routes": caddyRoutes,
		},
	}

	// Add dedicated port reverse proxy servers for spokes
	seenPorts := make(map[int]bool)
	for _, r := range routes {
		if r.Port > 0 && r.Port != 80 && r.Port != 443 && !seenPorts[r.Port] {
			seenPorts[r.Port] = true
			serverID := fmt.Sprintf("srv_%d", r.Port)
			upstreamHost := r.Service
			if !strings.HasPrefix(upstreamHost, "sovereign-") && !strings.Contains(upstreamHost, ".") && upstreamHost != "localhost" {
				upstreamHost = "sovereign-" + upstreamHost
			}
			upstreamDial := fmt.Sprintf("%s:%d", upstreamHost, r.Port)
			servers[serverID] = map[string]any{
				"listen": []string{fmt.Sprintf(":%d", r.Port)},
				"routes": []map[string]any{
					{
						"@id": fmt.Sprintf("route-direct-%d", r.Port),
						"handle": []map[string]any{
							{
								"handler": "reverse_proxy",
								"upstreams": []map[string]any{
									{"dial": upstreamDial},
								},
								"headers": map[string]any{
									"request": map[string]any{
										"set": map[string][]string{
											"Host":              {"{http.request.host}"},
											"X-Real-IP":         {"{http.request.remote.host}"},
											"X-Forwarded-For":   {"{http.request.remote.host}"},
											"X-Forwarded-Proto": {"{http.request.scheme}"},
											"X-Forwarded-Host":  {"{http.request.host}"},
										},
									},
								},
							},
						},
					},
				},
			}
		}
	}

	return map[string]any{
		"admin": map[string]any{
			"listen": "0.0.0.0:2019",
		},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": servers,
			},
		},
	}
}

// SyncRoutes loads the generated Caddy JSON route table into Caddy via POST /load.
func (c *CaddyClient) SyncRoutes(ctx context.Context, routes []RouteConfig) error {
	payload := c.BuildCaddyConfig(routes)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal caddy config: %w", err)
	}

	loadURL := c.adminURL + "/load"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create caddy /load request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("failed to communicate with caddy admin API", "error", err, "url", loadURL)
		return fmt.Errorf("caddy admin api unreachable at %s: %w", loadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("caddy admin API rejected config", "status", resp.StatusCode, "response", string(body))
		return fmt.Errorf("caddy load rejected with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	c.logger.Info("caddy route table synchronized successfully", "routes_count", len(routes))
	return nil
}

// GetStatus checks connectivity with Caddy Admin API via GET /config/.
func (c *CaddyClient) GetStatus(ctx context.Context) (map[string]any, error) {
	configURL := c.adminURL + "/config/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return map[string]any{
			"status":    "error",
			"connected": false,
			"admin_url": c.adminURL,
			"error":     err.Error(),
		}, fmt.Errorf("failed to create caddy status request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return map[string]any{
			"status":    "error",
			"connected": false,
			"admin_url": c.adminURL,
			"error":     err.Error(),
		}, fmt.Errorf("caddy admin api unreachable at %s: %w", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return map[string]any{
			"status":      "error",
			"connected":   false,
			"admin_url":   c.adminURL,
			"status_code": resp.StatusCode,
			"error":       strings.TrimSpace(string(body)),
		}, fmt.Errorf("caddy config returned status %d", resp.StatusCode)
	}

	var parsed any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return map[string]any{
			"status":    "ok",
			"connected": true,
			"admin_url": c.adminURL,
		}, nil
	}

	return map[string]any{
		"status":    "ok",
		"connected": true,
		"admin_url": c.adminURL,
		"config":    parsed,
	}, nil
}
