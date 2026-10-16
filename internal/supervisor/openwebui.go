package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Open WebUI constants and configuration defaults.
const (
	ServiceOpenWebUI = "openwebui"
	ImageOpenWebUI   = "ghcr.io/open-webui/open-webui:0.12.0"

	MemLimitOpenWebUI       int64 = 1024 * 1024 * 1024 // 1024 MB
	MemReservationOpenWebUI int64 = 512 * 1024 * 1024  // 512 MB

	DefaultOpenWebUIPort     = 3001
	DefaultOpenWebUISubpath  = "/chat"
	DefaultOpenWebUIGateway  = "http://sovereign-daemon:8080/v1"
	DefaultOpenWebUIName     = "Sovereign AI"
	DefaultOpenWebUIDataPath = "/app/backend/data"
)

// OpenWebUIProvisionResult contains synthesized container spec and runtime parameters for Open WebUI.
type OpenWebUIProvisionResult struct {
	ContainerSpec ContainerSpec
	Env           map[string]string
	StateDir      string
}

// OpenWebUIProvisionerOptions allows overriding default endpoints or state directory for testing.
type OpenWebUIProvisionerOptions struct {
	GatewayURL string
	StateDir   string
}

// OpenWebUIProvisioner manages persistent storage directory creation, air-gapped network configuration,
// telemetry suppression, and container specification synthesis for the Open WebUI spoke.
type OpenWebUIProvisioner struct {
	mu         sync.RWMutex
	dataDir    string
	stateDir   string
	gatewayURL string
	logger     *slog.Logger
}

// NewOpenWebUIProvisioner initializes an OpenWebUIProvisioner instance.
func NewOpenWebUIProvisioner(dataDir string, logger *slog.Logger, opts ...OpenWebUIProvisionerOptions) *OpenWebUIProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}

	stateDir := filepath.Join(dataDir, "state/services/openwebui")
	gatewayURL := DefaultOpenWebUIGateway

	if envGW := os.Getenv("SOVEREIGN_AI_GATEWAY_URL"); envGW != "" {
		gatewayURL = strings.TrimRight(envGW, "/")
	}

	if len(opts) > 0 {
		o := opts[0]
		if o.StateDir != "" {
			stateDir = o.StateDir
		}
		if o.GatewayURL != "" {
			gatewayURL = strings.TrimRight(o.GatewayURL, "/")
		}
	}

	return &OpenWebUIProvisioner{
		dataDir:    dataDir,
		stateDir:   stateDir,
		gatewayURL: gatewayURL,
		logger:     logger,
	}
}

// StateDir returns the resolved local state directory for Open WebUI data.
func (p *OpenWebUIProvisioner) StateDir() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stateDir
}

// GatewayURL returns the configured upstream AI Gateway URL.
func (p *OpenWebUIProvisioner) GatewayURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.gatewayURL
}

// EnsureStateDir ensures that the persistent state directory exists with 0755 permissions.
func (p *OpenWebUIProvisioner) EnsureStateDir() error {
	p.mu.RLock()
	dir := p.stateDir
	p.mu.RUnlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create openwebui state directory %s: %w", dir, err)
	}
	return nil
}

// SynthesizeContainerSpec generates the complete container specification for Open WebUI,
// enforcing strict telemetry suppression, zero Docker socket mounts, and isolated network attachments.
func (p *OpenWebUIProvisioner) SynthesizeContainerSpec() (ContainerSpec, error) {
	p.mu.RLock()
	stateDir := p.stateDir
	gwURL := p.gatewayURL
	p.mu.RUnlock()

	// Strict telemetry suppression and local AI Gateway configuration
	env := map[string]string{
		"PORT":                     fmt.Sprintf("%d", DefaultOpenWebUIPort),
		"OPENAI_API_BASE_URL":      gwURL,
		"OPENAI_API_KEY":           "sk-sovereign-local",
		"WEBUI_AUTH":               "False",
		"SCARF_NO_ANALYTICS":       "True",
		"DO_NOT_TRACK":             "True",
		"ANONYMIZED_TELEMETRY":     "False",
		"ENABLE_COMMUNITY_SHARING": "False",
		"ENABLE_VERSION_CHECK":     "False",
		"ENABLE_OLLAMA_API":        "False",
		"ENABLE_SEARCH_QUERY":      "False",
		"WEBUI_BUILD_HASH":         "",
		"WEBUI_NAME":               DefaultOpenWebUIName,
		"DATA_DIR":                 DefaultOpenWebUIDataPath,
	}

	// Read seeded system_prompt.txt if present
	promptFile := filepath.Join(stateDir, "system_prompt.txt")
	if promptBytes, err := os.ReadFile(promptFile); err == nil && len(promptBytes) > 0 {
		env["DEFAULT_SYSTEM_PROMPT"] = string(promptBytes)
	}

	// Persistent volume for chat history and settings
	volumeMount := fmt.Sprintf("%s:%s", stateDir, DefaultOpenWebUIDataPath)
	volumes := []string{volumeMount}

	// Invariant: zero Docker socket mounts
	for _, v := range volumes {
		if strings.Contains(v, "docker.sock") {
			return ContainerSpec{}, fmt.Errorf("security violation: docker.sock mounting is strictly prohibited")
		}
	}

	spec := ContainerSpec{
		Name:                   ServiceOpenWebUI,
		Image:                  ImageOpenWebUI,
		MemoryLimitBytes:       MemLimitOpenWebUI,
		MemoryReservationBytes: MemReservationOpenWebUI,
		Ports: map[int]int{
			DefaultOpenWebUIPort: DefaultOpenWebUIPort,
		},
		Networks: []string{"sovereign-ingress", "sovereign-ai"},
		Volumes:  volumes,
		Env:      env,
	}

	return spec, nil
}

// Provision ensures storage durability and synthesizes the container specification for startup.
func (p *OpenWebUIProvisioner) Provision(ctx context.Context) (*OpenWebUIProvisionResult, error) {
	if err := p.EnsureStateDir(); err != nil {
		return nil, err
	}

	spec, err := p.SynthesizeContainerSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to synthesize openwebui container spec: %w", err)
	}

	return &OpenWebUIProvisionResult{
		ContainerSpec: spec,
		Env:           spec.Env,
		StateDir:      p.StateDir(),
	}, nil
}
