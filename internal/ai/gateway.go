package ai

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// GatewayConfig provides construction parameters for DefaultGateway.
type GatewayConfig struct {
	ModelsDir string
	LlamaURL  string
	Logger    *slog.Logger
	Client    *http.Client
}

// DefaultGateway implements Gateway mediating OpenAI-compatible /v1 requests to llama-server.
type DefaultGateway struct {
	mu        sync.RWMutex
	modelsDir string
	llamaURL  string
	logger    *slog.Logger
	proxy     *httputil.ReverseProxy
}

// NewGateway creates and initializes a DefaultGateway.
func NewGateway(cfg GatewayConfig) *DefaultGateway {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.LlamaURL == "" {
		cfg.LlamaURL = "http://127.0.0.1:8080"
	}
	cfg.LlamaURL = strings.TrimRight(cfg.LlamaURL, "/")

	targetURL, _ := url.Parse(cfg.LlamaURL)
	var proxy *httputil.ReverseProxy
	if targetURL != nil {
		proxy = httputil.NewSingleHostReverseProxy(targetURL)
		if cfg.Client != nil && cfg.Client.Transport != nil {
			proxy.Transport = cfg.Client.Transport
		}
		originalErrorHandler := proxy.ErrorHandler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			cfg.Logger.Warn("ai gateway proxy error, falling back to safe stub", "error", err, "path", r.URL.Path)
			if originalErrorHandler != nil {
				SafeStubHandler(w, r)
			} else {
				SafeStubHandler(w, r)
			}
		}
	}

	return &DefaultGateway{
		modelsDir: cfg.ModelsDir,
		llamaURL:  cfg.LlamaURL,
		logger:    cfg.Logger,
		proxy:     proxy,
	}
}

// CheckModelAvailability checks if at least one verified GGUF model is provisioned in the models directory.
func (g *DefaultGateway) CheckModelAvailability(ctx context.Context) (bool, error) {
	g.mu.RLock()
	dir := g.modelsDir
	g.mu.RUnlock()

	if dir == "" {
		return false, nil
	}

	models, err := ScanModels(dir)
	if err != nil {
		return false, fmt.Errorf("failed to scan models: %w", err)
	}

	return len(models) > 0, nil
}

// SetModelsDir updates the models directory.
func (g *DefaultGateway) SetModelsDir(dir string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.modelsDir = dir
}

// SetLlamaURL updates the backend llama-server URL.
func (g *DefaultGateway) SetLlamaURL(rawURL string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.llamaURL = strings.TrimRight(rawURL, "/")
	if u, err := url.Parse(g.llamaURL); err == nil {
		g.proxy = httputil.NewSingleHostReverseProxy(u)
	}
}

// ServeHTTP handles inbound inference requests, falling back to SafeStubHandler if no model is provisioned.
func (g *DefaultGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hasModel, err := g.CheckModelAvailability(r.Context())
	if err != nil || !hasModel {
		SafeStubHandler(w, r)
		return
	}

	g.mu.RLock()
	proxy := g.proxy
	g.mu.RUnlock()

	if proxy == nil {
		SafeStubHandler(w, r)
		return
	}

	proxy.ServeHTTP(w, r)
}

// SurrenderIdleVRAM releases inference resources when idle.
func (g *DefaultGateway) SurrenderIdleVRAM(ctx context.Context) error {
	// Baseline idle surrender hook (expanded in Story 5.3)
	return nil
}
