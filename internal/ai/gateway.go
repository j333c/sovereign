package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// GatewayConfig provides construction parameters for DefaultGateway.
type GatewayConfig struct {
	ModelsDir  string
	LlamaURL   string
	Logger     *slog.Logger
	Client     *http.Client
	Governor   *RequestGovernor
	Arbitrator *GPUArbitrator
}

// DefaultGateway implements Gateway mediating OpenAI-compatible /v1 requests to llama-server.
type DefaultGateway struct {
	mu         sync.RWMutex
	modelsDir  string
	llamaURL   string
	logger     *slog.Logger
	proxy      *httputil.ReverseProxy
	governor   *RequestGovernor
	arbitrator *GPUArbitrator
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

	if cfg.Governor == nil {
		cfg.Governor = NewRequestGovernor(DefaultGovernorConfig())
	}

	if cfg.Arbitrator == nil {
		cfg.Arbitrator = NewGPUArbitrator(ArbitratorConfig{
			Logger: cfg.Logger,
		})
	}

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
		modelsDir:  cfg.ModelsDir,
		llamaURL:   cfg.LlamaURL,
		logger:     cfg.Logger,
		proxy:      proxy,
		governor:   cfg.Governor,
		arbitrator: cfg.Arbitrator,
	}
}

// Governor returns the active Two-Lane RequestGovernor.
func (g *DefaultGateway) Governor() *RequestGovernor {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.governor
}

// Arbitrator returns the active GPUArbitrator.
func (g *DefaultGateway) Arbitrator() *GPUArbitrator {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.arbitrator
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
		proxy := httputil.NewSingleHostReverseProxy(u)
		if g.proxy != nil && g.proxy.Transport != nil {
			proxy.Transport = g.proxy.Transport
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			g.logger.Warn("ai gateway proxy error, falling back to safe stub", "error", err, "path", r.URL.Path)
			SafeStubHandler(w, r)
		}
		g.proxy = proxy
	}
}

// ClassifyPriority determines whether the request belongs to PriorityInteractive or PriorityBackground.
// Checks headers (X-Inference-Priority, X-Priority, X-Lane), query parameters, and paths, defaulting
// to PriorityInteractive for human chat / Open WebUI traffic.
func ClassifyPriority(r *http.Request) InferencePriority {
	p := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Inference-Priority")))
	if p == "" {
		p = strings.ToLower(strings.TrimSpace(r.Header.Get("X-Priority")))
	}
	if p == "" {
		p = strings.ToLower(strings.TrimSpace(r.Header.Get("X-Lane")))
	}
	if p == "background" || p == "batch" {
		return PriorityBackground
	}
	if p == "interactive" {
		return PriorityInteractive
	}

	if qp := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("priority"))); qp == "background" {
		return PriorityBackground
	}

	// Path-based classification for automated background endpoints
	path := strings.ToLower(r.URL.Path)
	if strings.Contains(path, "/batch") || strings.Contains(path, "/scan") {
		return PriorityBackground
	}

	return PriorityInteractive
}

// ServeHTTP handles inbound inference requests, falling back to SafeStubHandler if no model is provisioned,
// enforcing generation intent presets, and arbitrating concurrency via the RequestGovernor.
func (g *DefaultGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hasModel, err := g.CheckModelAvailability(r.Context())
	if err != nil || !hasModel {
		g.mu.RLock()
		arb := g.arbitrator
		g.mu.RUnlock()
		if arb != nil {
			arb.SetState(ArbitratorStateSafeStub)
		}
		SafeStubHandler(w, r)
		return
	}

	g.mu.RLock()
	proxy := g.proxy
	gov := g.governor
	arb := g.arbitrator
	g.mu.RUnlock()

	if arb != nil && arb.State() == ArbitratorStateSafeStub {
		arb.SetState(ArbitratorStateIdleSurrendered)
	}

	if proxy == nil {
		SafeStubHandler(w, r)
		return
	}

	// For read-only endpoints like GET /v1/models or preflight OPTIONS that do not perform inference,
	// proxy directly without acquiring an inference execution slot or waking dormant GPU models.
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		proxy.ServeHTTP(w, r)
		return
	}

	// Ensure arbitrator has woken model if idle_surrendered
	if arb != nil {
		if err := arb.EnsureReady(r.Context()); err != nil {
			if errors.Is(err, ErrSafeStub) {
				SafeStubHandler(w, r)
				return
			}
			if errors.Is(err, ErrWakeTimeout) || errors.Is(err, context.DeadlineExceeded) {
				writeGatewayProblem(w, http.StatusGatewayTimeout, "Gateway Timeout", "AI inference warm-up timeout expired.", r.URL.Path)
				return
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			writeGatewayProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to wake AI model: %v", err), r.URL.Path)
			return
		}
	}

	priority := ClassifyPriority(r)

	// Normalize payload and inject generation intent preset for completion endpoints if JSON body exists
	isCompletion := strings.HasSuffix(r.URL.Path, "/chat/completions") || strings.HasSuffix(r.URL.Path, "/completions")
	if isCompletion && r.Body != nil {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
		_ = r.Body.Close()
		if readErr != nil {
			writeGatewayProblem(w, http.StatusBadRequest, "Invalid Request Body", "Failed to read inference request payload.", r.URL.Path)
			return
		}

		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			defaultPreset := r.Header.Get("X-Generation-Intent")
			normalized, normErr := NormalizeInferencePayload(bodyBytes, defaultPreset)
			if normErr != nil {
				writeGatewayProblem(w, http.StatusBadRequest, "Invalid JSON", normErr.Error(), r.URL.Path)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(normalized))
			r.ContentLength = int64(len(normalized))
			r.Header.Set("Content-Length", strconv.Itoa(len(normalized)))
		} else {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	// Acquire concurrency slot from RequestGovernor
	if gov != nil {
		release, acqErr := gov.Acquire(r.Context(), priority)
		if acqErr != nil {
			if errors.Is(acqErr, ErrQueueFull) {
				w.Header().Set("Retry-After", "5")
				writeGatewayProblem(w, http.StatusTooManyRequests, "Too Many Requests", "AI inference queue capacity exceeded. Please retry your request shortly.", r.URL.Path)
				return
			}
			if errors.Is(acqErr, ErrQueueTimeout) || errors.Is(acqErr, context.DeadlineExceeded) {
				writeGatewayProblem(w, http.StatusGatewayTimeout, "Gateway Timeout", "AI inference queue wait timeout expired.", r.URL.Path)
				return
			}
			if errors.Is(acqErr, context.Canceled) {
				// Client cancelled connection
				return
			}
			writeGatewayProblem(w, http.StatusInternalServerError, "Internal Server Error", acqErr.Error(), r.URL.Path)
			return
		}
		defer release()
	}

	if arb != nil {
		releaseInference := arb.BeginInference()
		defer releaseInference()
	}

	proxy.ServeHTTP(w, r)
	if arb != nil {
		arb.RecordActivity()
	}
}

func writeGatewayProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	slug := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	prob := map[string]any{
		"type":     fmt.Sprintf("https://sovereign.local/problems/%s", slug),
		"title":    title,
		"status":   status,
		"detail":   detail,
		"instance": instance,
	}
	_ = json.NewEncoder(w).Encode(prob)
}

// SurrenderIdleVRAM releases inference resources when idle.
func (g *DefaultGateway) SurrenderIdleVRAM(ctx context.Context) error {
	g.mu.RLock()
	arb := g.arbitrator
	g.mu.RUnlock()
	if arb != nil {
		return arb.SurrenderVRAM(ctx)
	}
	return nil
}

// Close cleanly shuts down the gateway and underlying arbitrator.
func (g *DefaultGateway) Close() {
	g.mu.RLock()
	arb := g.arbitrator
	g.mu.RUnlock()
	if arb != nil {
		arb.Close()
	}
}

