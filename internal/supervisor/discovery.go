package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// UpdateCandidate represents a discovered newer release tag qualifying for the current channel.
type UpdateCandidate struct {
	TargetTag      string  `json:"target_tag"`
	TargetVersion  Version `json:"target_version"`
	CurrentVersion Version `json:"current_version"`
	Channel        string  `json:"channel"`
	Changelog      string  `json:"changelog"`
}

// GitRunner abstracts git command execution for testing and production.
type GitRunner func(ctx context.Context, dir string, args ...string) (string, error)

// defaultGitRunner runs git CLI commands via os/exec.
func defaultGitRunner(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errDetail := strings.TrimSpace(stderr.String())
		if errDetail != "" {
			return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, errDetail)
		}
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// UpdateDiscoveryConfig contains settings for the update discovery engine.
type UpdateDiscoveryConfig struct {
	RepoDir        string
	RemoteName     string
	CurrentVersion string
	Channel        string
	PollInterval   time.Duration
	GitRun         GitRunner
	OnUpdateFound  func(candidate *UpdateCandidate)
	Logger         *slog.Logger
}

// UpdateDiscoveryEngine manages git-native upstream tag discovery over Smart HTTP.
type UpdateDiscoveryEngine struct {
	mu             sync.RWMutex
	repoDir        string
	remoteName     string
	currentVersion string
	channel        string
	pollInterval   time.Duration
	gitRun         GitRunner
	onUpdateFound  func(candidate *UpdateCandidate)
	logger         *slog.Logger
	cancel         context.CancelFunc
	lastCheckTime  time.Time
	lastCandidate  *UpdateCandidate
}

// NewUpdateDiscoveryEngine constructs an UpdateDiscoveryEngine.
func NewUpdateDiscoveryEngine(cfg UpdateDiscoveryConfig) *UpdateDiscoveryEngine {
	repoDir := cfg.RepoDir
	if repoDir == "" {
		repoDir = os.Getenv("SOVEREIGN_REPO_DIR")
		if repoDir == "" {
			repoDir = "/opt/sovereign"
		}
	}

	remoteName := cfg.RemoteName
	if remoteName == "" {
		remoteName = "origin"
	}

	currentVersion := cfg.CurrentVersion
	if currentVersion == "" {
		currentVersion = "v0.1.0"
	}

	channel := cfg.Channel
	if channel == "" {
		channel = ChannelStable
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 1 * time.Hour
	}

	gitRun := cfg.GitRun
	if gitRun == nil {
		gitRun = defaultGitRunner
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &UpdateDiscoveryEngine{
		repoDir:        repoDir,
		remoteName:     remoteName,
		currentVersion: currentVersion,
		channel:        channel,
		pollInterval:   pollInterval,
		gitRun:         gitRun,
		onUpdateFound:  cfg.OnUpdateFound,
		logger:         logger,
	}
}

// SetChannel updates the active release channel.
func (e *UpdateDiscoveryEngine) SetChannel(channel string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.channel = channel
}

// Channel returns the current channel.
func (e *UpdateDiscoveryEngine) Channel() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.channel
}

// LastCandidate returns the last discovered qualifying update candidate, if any.
func (e *UpdateDiscoveryEngine) LastCandidate() *UpdateCandidate {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastCandidate
}

// ResolveCurrentVersion resolves the currently running version from `git describe --tags --exact-match`
// or falls back to the configured currentVersion string.
func (e *UpdateDiscoveryEngine) ResolveCurrentVersion(ctx context.Context) Version {
	e.mu.RLock()
	repoDir := e.repoDir
	fallback := e.currentVersion
	e.mu.RUnlock()

	out, err := e.gitRun(ctx, repoDir, "describe", "--tags", "--exact-match")
	if err == nil && out != "" {
		v, err := ParseVersion(out)
		if err == nil {
			return v
		}
	}

	v, err := ParseVersion(fallback)
	if err == nil {
		return v
	}

	return Version{Major: 0, Minor: 1, Patch: 0}
}

// QueryUpstreamTags queries upstream tag refs via `git ls-remote --tags --refs origin`.
func (e *UpdateDiscoveryEngine) QueryUpstreamTags(ctx context.Context) ([]string, error) {
	e.mu.RLock()
	repoDir := e.repoDir
	remoteName := e.remoteName
	e.mu.RUnlock()

	out, err := e.gitRun(ctx, repoDir, "ls-remote", "--tags", "--refs", remoteName)
	if err != nil {
		// Log quiet debug notice per AC: network disconnections or DNS failures log debug
		e.logger.Debug("git ls-remote update discovery query failed", "error", err)
		return nil, err
	}

	var tags []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <sha>\trefs/tags/<tag_name>
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			ref := strings.TrimSpace(parts[1])
			if strings.HasPrefix(ref, "refs/tags/") {
				tag := strings.TrimPrefix(ref, "refs/tags/")
				// Ensure not a peeled tag duplicate
				if !strings.HasSuffix(tag, "^{}") {
					tags = append(tags, tag)
				}
			}
		}
	}

	return tags, nil
}

// FetchTagChangelog fetches tag metadata and extracts annotated subject and body.
func (e *UpdateDiscoveryEngine) FetchTagChangelog(ctx context.Context, tag string) (string, error) {
	e.mu.RLock()
	repoDir := e.repoDir
	remoteName := e.remoteName
	e.mu.RUnlock()

	// Shallow tag fetch
	_, _ = e.gitRun(ctx, repoDir, "fetch", "--depth=1", remoteName, "tag", tag)

	// Extract annotated tag subject/body
	subject, err := e.gitRun(ctx, repoDir, "tag", "-l", "--format=%(contents:subject)", tag)
	if err != nil || subject == "" {
		// Try full contents format
		subject, _ = e.gitRun(ctx, repoDir, "tag", "-l", "--format=%(contents)", tag)
	}

	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = fmt.Sprintf("Sovereign release %s", tag)
	}

	return subject, nil
}

// CheckForUpdates performs a single update discovery check against upstream tags.
func (e *UpdateDiscoveryEngine) CheckForUpdates(ctx context.Context) (*UpdateCandidate, error) {
	e.mu.RLock()
	channel := e.channel
	e.mu.RUnlock()

	current := e.ResolveCurrentVersion(ctx)
	tags, err := e.QueryUpstreamTags(ctx)
	if err != nil {
		return nil, err
	}

	bestVer, bestTag, found := SelectLatestQualifyingTag(tags, current, channel)
	if !found {
		e.mu.Lock()
		e.lastCheckTime = time.Now()
		e.lastCandidate = nil
		e.mu.Unlock()
		return nil, nil
	}

	changelog, _ := e.FetchTagChangelog(ctx, bestTag)

	candidate := &UpdateCandidate{
		TargetTag:      bestTag,
		TargetVersion:  bestVer,
		CurrentVersion: current,
		Channel:        channel,
		Changelog:      changelog,
	}

	e.mu.Lock()
	e.lastCheckTime = time.Now()
	e.lastCandidate = candidate
	callback := e.onUpdateFound
	e.mu.Unlock()

	if callback != nil {
		callback(candidate)
	}

	return candidate, nil
}

// Start launches the periodic background update discovery loop.
func (e *UpdateDiscoveryEngine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	pollInterval := e.pollInterval
	e.mu.Unlock()

	go func() {
		// Execute initial check after brief startup delay (10s)
		select {
		case <-loopCtx.Done():
			return
		case <-time.After(10 * time.Second):
			checkCtx, checkCancel := context.WithTimeout(loopCtx, 30*time.Second)
			_, _ = e.CheckForUpdates(checkCtx)
			checkCancel()
		}

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(loopCtx, 30*time.Second)
				_, _ = e.CheckForUpdates(checkCtx)
				checkCancel()
			}
		}
	}()
}

// Stop terminates the background update discovery loop.
func (e *UpdateDiscoveryEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
}
