package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Default base URLs for audio streaming services
const (
	DefaultNavidromeBaseURL      = "http://127.0.0.1:4533"
	DefaultAudiobookshelfBaseURL = "http://127.0.0.1:13378"
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
	dataDir                   string
	logger                    *slog.Logger
	httpClient                *http.Client
	navidromeBaseURL          string
	audiobookshelfBaseURL     string
	NavidromeRefreshFunc      func(ctx context.Context) error
	AudiobookshelfRefreshFunc func(ctx context.Context) error
}

// NewAudioProvisioner constructs a new AudioProvisioner.
func NewAudioProvisioner(dataDir string, logger *slog.Logger) *AudioProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}

	navURL := os.Getenv("SOVEREIGN_NAVIDROME_URL")
	if navURL == "" {
		navURL = DefaultNavidromeBaseURL
	}

	absURL := os.Getenv("SOVEREIGN_AUDIOBOOKSHELF_URL")
	if absURL == "" {
		absURL = DefaultAudiobookshelfBaseURL
	}

	return &AudioProvisioner{
		dataDir:               dataDir,
		logger:                logger,
		httpClient:            &http.Client{Timeout: 5 * time.Second},
		navidromeBaseURL:      strings.TrimRight(navURL, "/"),
		audiobookshelfBaseURL: strings.TrimRight(absURL, "/"),
	}
}

// SetNavidromeBaseURL overrides the Navidrome base URL.
func (p *AudioProvisioner) SetNavidromeBaseURL(url string) {
	p.navidromeBaseURL = strings.TrimRight(url, "/")
}

// SetAudiobookshelfBaseURL overrides the Audiobookshelf base URL.
func (p *AudioProvisioner) SetAudiobookshelfBaseURL(url string) {
	p.audiobookshelfBaseURL = strings.TrimRight(url, "/")
}

// SetHTTPClient overrides the HTTP client.
func (p *AudioProvisioner) SetHTTPClient(client *http.Client) {
	p.httpClient = client
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
		"ND_SCANINTERVAL": "0",
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
		"AUDIOBOOKSHELF_UID": "0",
		"AUDIOBOOKSHELF_GID": "0",
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
	if p.NavidromeRefreshFunc != nil {
		return p.NavidromeRefreshFunc(ctx)
	}

	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	baseURL := p.navidromeBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("SOVEREIGN_NAVIDROME_URL")
		if baseURL == "" {
			baseURL = DefaultNavidromeBaseURL
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")

	refreshURL := fmt.Sprintf("%s/api/scan", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, strings.NewReader(`{"fullScan":false}`))
	if err != nil {
		return fmt.Errorf("failed to create navidrome refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
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
	if p.AudiobookshelfRefreshFunc != nil {
		return p.AudiobookshelfRefreshFunc(ctx)
	}

	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	baseURL := p.audiobookshelfBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("SOVEREIGN_AUDIOBOOKSHELF_URL")
		if baseURL == "" {
			baseURL = DefaultAudiobookshelfBaseURL
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")

	refreshURL := fmt.Sprintf("%s/api/libraries/scan", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create audiobookshelf refresh request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
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
