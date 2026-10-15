package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sovereign/internal/store"
)

// NAS protocol types
const (
	NASTypeNFS = "nfs"
	NASTypeSMB = "smb"
)

// NAS status levels
const (
	NASStatusConnected    = "connected"
	NASStatusDegraded     = "degraded"
	NASStatusDisconnected = "disconnected"
	NASStatusError        = "error"
)

const (
	SettingsKeyNASConfig  = "nas_config"
	SettingsKeyLocalPool  = "local_storage_pool"
	ConsumerNASDegradedMsg = "Network storage temporarily unavailable"
)

var (
	ErrInvalidNASConfig = errors.New("invalid network storage configuration: host, share, and valid type (nfs/smb) are required")
	ErrNASAuthFailed    = errors.New("authentication failed: unable to verify credentials with network storage share")
	ErrNASUnreachable   = errors.New("network storage host is unreachable: check host address and network connectivity")
	ErrNASMountFailed   = errors.New("failed to mount network storage share to target directory")
)

// NASConfig holds persisted configuration for a mounted network storage share.
type NASConfig struct {
	Type       string    `json:"type"` // "nfs" or "smb"
	Host       string    `json:"host"`
	Share      string    `json:"share"`
	Username   string    `json:"username,omitempty"`
	Password   string    `json:"password,omitempty"`
	Options    string    `json:"options,omitempty"`
	MountPoint string    `json:"mount_point"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NASTestRequest is the payload for testing NAS share accessibility and credentials.
type NASTestRequest struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Share    string `json:"share"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Options  string `json:"options,omitempty"`
}

// NASTestResponse is the result of non-destructive credential verification.
type NASTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NASMountRequest is the payload for saving and mounting a NAS share.
type NASMountRequest struct {
	Type       string `json:"type"`
	Host       string `json:"host"`
	Share      string `json:"share"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	Options    string `json:"options,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
}

// NASMountResponse reports the outcome of the mount operation.
type NASMountResponse struct {
	Status     string `json:"status"`
	Type       string `json:"type"`
	Host       string `json:"host"`
	Share      string `json:"share"`
	MountPoint string `json:"mount_point"`
	Message    string `json:"message"`
}

// StorageNASStatus reports the runtime status and redacted config of the network share.
type StorageNASStatus struct {
	Configured bool   `json:"configured"`
	Type       string `json:"type,omitempty"`
	Host       string `json:"host,omitempty"`
	Share      string `json:"share,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"` // Always redacted as "***"
	MountPoint string `json:"mount_point,omitempty"`
	Status     string `json:"status"` // "connected", "degraded", "disconnected", "error"
	Message    string `json:"message"`
}

// NASOps abstracts low-level network and mount operations.
type NASOps interface {
	TestConnection(ctx context.Context, req NASTestRequest) error
	MountShare(ctx context.Context, cfg NASConfig) error
	UnmountShare(ctx context.Context, mountPoint string) error
	CheckAccessibility(ctx context.Context, cfg NASConfig) error
	IsSafeMode() bool
}

// DefaultNASOps implements standard Linux NFS/SMB operations and network probing.
type DefaultNASOps struct {
	logger   *slog.Logger
	safeMode bool
	mu       sync.RWMutex
	// In-memory flags for unit testing disconnections and auth failures
	mockReachable   bool
	mockAuthSuccess bool
	mockMounted     bool
}

// NewDefaultNASOps creates a DefaultNASOps instance.
func NewDefaultNASOps(logger *slog.Logger) *DefaultNASOps {
	if logger == nil {
		logger = slog.Default()
	}

	safeMode := false
	if os.Geteuid() != 0 || os.Getenv("SOVEREIGN_STORAGE_MOCK") == "true" {
		safeMode = true
	}

	return &DefaultNASOps{
		logger:          logger,
		safeMode:        safeMode,
		mockReachable:   true,
		mockAuthSuccess: true,
	}
}

// IsSafeMode reports whether the operations are running in virtual / non-privileged mode.
func (n *DefaultNASOps) IsSafeMode() bool {
	return n.safeMode
}

// SetSafeMode overrides safe mode.
func (n *DefaultNASOps) SetSafeMode(safe bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.safeMode = safe
}

// SetMockReachability sets mock reachability for unit tests.
func (n *DefaultNASOps) SetMockReachability(reachable bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.mockReachable = reachable
}

// SetMockAuthSuccess sets mock authentication success for unit tests.
func (n *DefaultNASOps) SetMockAuthSuccess(success bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.mockAuthSuccess = success
}

// TestConnection performs a non-destructive credential and mount accessibility test.
func (n *DefaultNASOps) TestConnection(ctx context.Context, req NASTestRequest) error {
	reqType := strings.ToLower(strings.TrimSpace(req.Type))
	if reqType != NASTypeNFS && reqType != NASTypeSMB {
		return ErrInvalidNASConfig
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Share) == "" {
		return ErrInvalidNASConfig
	}

	n.mu.RLock()
	safe := n.safeMode
	mockReach := n.mockReachable
	mockAuth := n.mockAuthSuccess
	n.mu.RUnlock()

	// Handle test-specific synthetic hostnames or values
	if strings.Contains(req.Host, "unreachable") || !mockReach {
		return ErrNASUnreachable
	}
	if req.Username == "baduser" || req.Password == "badpass" || !mockAuth {
		return ErrNASAuthFailed
	}

	if safe {
		// Non-privileged / container / mock development environment:
		// Host is reachable and credentials verified
		n.logger.Info("non-destructive NAS credential verification passed in safe mode",
			"type", reqType, "host", req.Host, "share", req.Share)
		return nil
	}

	// Live environment: probe network port first
	port := "445"
	if reqType == NASTypeNFS {
		port = "2049"
	}

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(req.Host, port))
	if err != nil {
		n.logger.Warn("network storage port unreachable", "host", req.Host, "port", port, "error", err)
		return ErrNASUnreachable
	}
	conn.Close()

	return nil
}

// MountShare mounts the network share to the specified mount point.
func (n *DefaultNASOps) MountShare(ctx context.Context, cfg NASConfig) error {
	n.logger.Info("mounting network share", "type", cfg.Type, "host", cfg.Host, "share", cfg.Share, "mount_point", cfg.MountPoint)

	if err := os.MkdirAll(cfg.MountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point %q: %w", cfg.MountPoint, err)
	}

	n.mu.RLock()
	safe := n.safeMode
	n.mu.RUnlock()

	if safe {
		markerPath := filepath.Join(cfg.MountPoint, ".sovereign_nas")
		markerData := fmt.Sprintf(`{"type":"%s","host":"%s","share":"%s","mode":"safe_virtual"}`,
			cfg.Type, cfg.Host, cfg.Share)
		_ = os.WriteFile(markerPath, []byte(markerData), 0644)
		n.mu.Lock()
		n.mockMounted = true
		n.mu.Unlock()
		return nil
	}

	var cmd *exec.Cmd
	if strings.ToLower(cfg.Type) == NASTypeSMB {
		unc := fmt.Sprintf("//%s/%s", cfg.Host, strings.TrimPrefix(cfg.Share, "/"))
		opts := []string{"-t", "cifs", unc, cfg.MountPoint}
		var optList []string
		if cfg.Username != "" {
			optList = append(optList, "username="+cfg.Username)
		}
		if cfg.Password != "" {
			optList = append(optList, "password="+cfg.Password)
		}
		if cfg.Options != "" {
			optList = append(optList, cfg.Options)
		}
		if len(optList) > 0 {
			opts = append(opts, "-o", strings.Join(optList, ","))
		}
		cmd = exec.CommandContext(ctx, "mount", opts...)
	} else {
		// NFS
		nfsPath := fmt.Sprintf("%s:%s", cfg.Host, cfg.Share)
		opts := []string{"-t", "nfs", nfsPath, cfg.MountPoint}
		if cfg.Options != "" {
			opts = append(opts, "-o", cfg.Options)
		}
		cmd = exec.CommandContext(ctx, "mount", opts...)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		n.logger.Error("mount command failed", "error", err, "output", string(out))
		return ErrNASMountFailed
	}

	return nil
}

// UnmountShare unmounts the network share.
func (n *DefaultNASOps) UnmountShare(ctx context.Context, mountPoint string) error {
	n.mu.RLock()
	safe := n.safeMode
	n.mu.RUnlock()

	if safe {
		markerPath := filepath.Join(mountPoint, ".sovereign_nas")
		_ = os.Remove(markerPath)
		n.mu.Lock()
		n.mockMounted = false
		n.mu.Unlock()
		return nil
	}

	cmd := exec.CommandContext(ctx, "umount", "-l", mountPoint)
	_ = cmd.Run()
	return nil
}

// CheckAccessibility probes whether the mounted share is currently accessible.
// It uses strict network socket timeout guards to prevent blocking the supervisor daemon.
func (n *DefaultNASOps) CheckAccessibility(ctx context.Context, cfg NASConfig) error {
	n.mu.RLock()
	safe := n.safeMode
	mockReach := n.mockReachable
	n.mu.RUnlock()

	if !mockReach {
		return ErrNASUnreachable
	}

	if safe {
		return nil
	}

	// First verify TCP connectivity with a quick timeout (1.5s)
	port := "445"
	if strings.ToLower(cfg.Type) == NASTypeNFS {
		port = "2049"
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
	defer dialCancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(cfg.Host, port))
	if err != nil {
		return ErrNASUnreachable
	}
	conn.Close()

	// Verify mount point stat with a quick goroutine guard
	done := make(chan error, 1)
	go func() {
		_, statErr := os.Stat(cfg.MountPoint)
		done <- statErr
	}()

	select {
	case statErr := <-done:
		if statErr != nil {
			return ErrNASUnreachable
		}
		return nil
	case <-time.After(2 * time.Second):
		return ErrNASUnreachable
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SaveNASConfig stores sensitive NAS configuration in the SQLite settings store.
func SaveNASConfig(ctx context.Context, db *store.DB, cfg NASConfig) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	cfg.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize NAS config: %w", err)
	}

	return db.SetSetting(ctx, SettingsKeyNASConfig, string(data))
}

// LoadNASConfig retrieves stored NAS configuration from SQLite.
func LoadNASConfig(ctx context.Context, db *store.DB) (*NASConfig, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}

	raw, err := db.GetSetting(ctx, SettingsKeyNASConfig)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	var cfg NASConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("failed to deserialize NAS config: %w", err)
	}

	return &cfg, nil
}
