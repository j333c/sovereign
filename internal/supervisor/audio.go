package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Default base URLs for audio streaming services
const (
	DefaultNavidromeContainerURL      = "http://sovereign-navidrome:4533"
	DefaultNavidromeLocalURL          = "http://127.0.0.1:4533"
	DefaultAudiobookshelfContainerURL = "http://sovereign-audiobookshelf:13378"
	DefaultAudiobookshelfLocalURL     = "http://127.0.0.1:13378"
)

// AudioProvisionResult contains provisioning output for starting an audio service container.
type AudioProvisionResult struct {
	ServiceID string
	Env       map[string]string
	Devices   []string
}

// AudioProvisioner manages volume layout provisioning, environment synthesis,
// and library scan triggers for Navidrome and Audiobookshelf.
type AudioProvisioner struct {
	mu                        sync.RWMutex
	dataDir                   string
	logger                    *slog.Logger
	httpClient                *http.Client
	navidromeBaseURL          string
	audiobookshelfBaseURL     string
	NavidromeRefreshFunc      func(ctx context.Context) error
	AudiobookshelfRefreshFunc func(ctx context.Context) error
}

func resolveAudioBaseURL(envKey, containerHostPort, localhostPort string) string {
	if env := os.Getenv(envKey); env != "" {
		return strings.TrimRight(env, "/")
	}
	// Check if container host resolves on network
	host := strings.Split(strings.TrimPrefix(containerHostPort, "http://"), ":")[0]
	if _, err := net.LookupHost(host); err == nil {
		return containerHostPort
	}
	return localhostPort
}

// NewAudioProvisioner constructs a new AudioProvisioner.
func NewAudioProvisioner(dataDir string, logger *slog.Logger) *AudioProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}

	navURL := resolveAudioBaseURL("SOVEREIGN_NAVIDROME_URL", DefaultNavidromeContainerURL, DefaultNavidromeLocalURL)
	absURL := resolveAudioBaseURL("SOVEREIGN_AUDIOBOOKSHELF_URL", DefaultAudiobookshelfContainerURL, DefaultAudiobookshelfLocalURL)

	return &AudioProvisioner{
		dataDir:               dataDir,
		logger:                logger,
		httpClient:            &http.Client{Timeout: 5 * time.Second},
		navidromeBaseURL:      navURL,
		audiobookshelfBaseURL: absURL,
	}
}

// SetNavidromeBaseURL overrides the Navidrome base URL in a thread-safe manner.
func (p *AudioProvisioner) SetNavidromeBaseURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.navidromeBaseURL = strings.TrimRight(url, "/")
}

// SetAudiobookshelfBaseURL overrides the Audiobookshelf base URL in a thread-safe manner.
func (p *AudioProvisioner) SetAudiobookshelfBaseURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.audiobookshelfBaseURL = strings.TrimRight(url, "/")
}

// SetHTTPClient overrides the HTTP client in a thread-safe manner.
func (p *AudioProvisioner) SetHTTPClient(client *http.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.httpClient = client
}

// SetNavidromeRefreshFunc overrides the Navidrome refresh callback for testing.
func (p *AudioProvisioner) SetNavidromeRefreshFunc(fn func(ctx context.Context) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.NavidromeRefreshFunc = fn
}

// SetAudiobookshelfRefreshFunc overrides the Audiobookshelf refresh callback for testing.
func (p *AudioProvisioner) SetAudiobookshelfRefreshFunc(fn func(ctx context.Context) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.AudiobookshelfRefreshFunc = fn
}

// Provision ensures directories exist and returns runtime parameters for the specified audio service.
func (p *AudioProvisioner) Provision(ctx context.Context, serviceID string) (*AudioProvisionResult, error) {
	switch serviceID {
	case ServiceNavidrome:
		return p.ProvisionNavidrome(ctx)
	case ServiceAudiobookshelf:
		return p.ProvisionAudiobookshelf(ctx)
	default:
		return nil, fmt.Errorf("%w: unknown audio service %s", ErrServiceNotFound, serviceID)
	}
}

// ProvisionNavidrome provisions Navidrome media and state directories with 0755 permissions,
// preserving existing content, and configures environment variables.
func (p *AudioProvisioner) ProvisionNavidrome(ctx context.Context) (*AudioProvisionResult, error) {
	musicDir := filepath.Join(p.dataDir, "media", "music")
	dataDir := filepath.Join(p.dataDir, "state", "services", "navidrome", "data")

	for _, dir := range []string{musicDir, dataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			p.logger.Error("failed to create navidrome directory", "dir", dir, "error", err)
			return nil, fmt.Errorf("failed to create navidrome directory %s: %w", dir, err)
		}
	}

	env := map[string]string{
		"ND_MUSICFOLDER":  "/music",
		"ND_DATAFOLDER":   "/data",
		"ND_PORT":         "4533",
		"ND_LOGLEVEL":     "info",
		"ND_SCANINTERVAL": "1m",
	}

	p.logger.Info("provisioned navidrome environment successfully", "music_dir", musicDir, "data_dir", dataDir)

	return &AudioProvisionResult{
		ServiceID: ServiceNavidrome,
		Env:       env,
		Devices:   nil,
	}, nil
}

// ProvisionAudiobookshelf provisions Audiobookshelf media, config, and metadata directories with 0755 permissions,
// preserving existing chapter markers, speed settings, and database, and configures environment variables.
func (p *AudioProvisioner) ProvisionAudiobookshelf(ctx context.Context) (*AudioProvisionResult, error) {
	audiobooksDir := filepath.Join(p.dataDir, "media", "audiobooks")
	configDir := filepath.Join(p.dataDir, "state", "services", "audiobookshelf", "config")
	metadataDir := filepath.Join(p.dataDir, "state", "services", "audiobookshelf", "metadata")

	for _, dir := range []string{audiobooksDir, configDir, metadataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			p.logger.Error("failed to create audiobookshelf directory", "dir", dir, "error", err)
			return nil, fmt.Errorf("failed to create audiobookshelf directory %s: %w", dir, err)
		}
	}

	env := map[string]string{
		"PORT":               "13378",
		"CONFIG_PATH":        "/config",
		"METADATA_PATH":      "/metadata",
		"AUDIOBOOKSHELF_UID": "1000",
		"AUDIOBOOKSHELF_GID": "1000",
	}

	p.logger.Info("provisioned audiobookshelf environment successfully", "audiobooks_dir", audiobooksDir, "config_dir", configDir)

	return &AudioProvisionResult{
		ServiceID: ServiceAudiobookshelf,
		Env:       env,
		Devices:   nil,
	}, nil
}

// TriggerNavidromeRefresh dispatches a library scan webhook to Navidrome.
func (p *AudioProvisioner) TriggerNavidromeRefresh(ctx context.Context) error {
	p.mu.RLock()
	refreshFn := p.NavidromeRefreshFunc
	client := p.httpClient
	baseURL := p.navidromeBaseURL
	p.mu.RUnlock()

	if refreshFn != nil {
		return refreshFn(ctx)
	}

	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
		p.mu.Lock()
		p.httpClient = client
		p.mu.Unlock()
	}

	if baseURL == "" {
		baseURL = resolveAudioBaseURL("SOVEREIGN_NAVIDROME_URL", DefaultNavidromeContainerURL, DefaultNavidromeLocalURL)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	refreshURL := fmt.Sprintf("%s/api/scan", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, strings.NewReader(`{"fullScan":false}`))
	if err != nil {
		return fmt.Errorf("failed to create navidrome refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to dispatch navidrome library refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("navidrome library refresh responded with status %d", resp.StatusCode)
	}

	p.logger.Info("navidrome library refresh dispatched successfully", "url", refreshURL)
	return nil
}

// TriggerAudiobookshelfRefresh dispatches a library scan webhook to Audiobookshelf.
func (p *AudioProvisioner) TriggerAudiobookshelfRefresh(ctx context.Context) error {
	p.mu.RLock()
	refreshFn := p.AudiobookshelfRefreshFunc
	client := p.httpClient
	baseURL := p.audiobookshelfBaseURL
	p.mu.RUnlock()

	if refreshFn != nil {
		return refreshFn(ctx)
	}

	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
		p.mu.Lock()
		p.httpClient = client
		p.mu.Unlock()
	}

	if baseURL == "" {
		baseURL = resolveAudioBaseURL("SOVEREIGN_AUDIOBOOKSHELF_URL", DefaultAudiobookshelfContainerURL, DefaultAudiobookshelfLocalURL)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	refreshURL := fmt.Sprintf("%s/api/libraries/scan", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create audiobookshelf refresh request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to dispatch audiobookshelf library refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("audiobookshelf library refresh responded with status %d", resp.StatusCode)
	}

	p.logger.Info("audiobookshelf library refresh dispatched successfully", "url", refreshURL)
	return nil
}
