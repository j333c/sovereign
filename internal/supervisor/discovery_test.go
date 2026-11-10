package supervisor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUpdateDiscoveryEngine_QueryUpstreamTags(t *testing.T) {
	mockOutput := `
4f53cda18c2d4a89e8608304218683525a7677bf	refs/tags/v1.0.0
5d836bc4561b36f7842db13e2f41c9b68a88c221	refs/tags/v1.1.0-rc.1
7a8291b489ef48b6289b7cf91c4d98a2874bc981	refs/tags/v1.1.0
8b9291b489ef48b6289b7cf91c4d98a2874bc982	refs/tags/v1.1.0^{}
`
	runner := func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "ls-remote" {
			return mockOutput, nil
		}
		return "", errors.New("unhandled git command")
	}

	engine := NewUpdateDiscoveryEngine(UpdateDiscoveryConfig{
		GitRun:         runner,
		CurrentVersion: "v1.0.0",
		Channel:        ChannelStable,
	})

	tags, err := engine.QueryUpstreamTags(context.Background())
	if err != nil {
		t.Fatalf("unexpected error querying tags: %v", err)
	}

	// Peeled tag duplicate ^{} should be filtered out
	expected := []string{"v1.0.0", "v1.1.0-rc.1", "v1.1.0"}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d (%v)", len(expected), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("tag[%d]: expected %q, got %q", i, expected[i], tag)
		}
	}
}

func TestUpdateDiscoveryEngine_NetworkErrorQuietHandling(t *testing.T) {
	runner := func(ctx context.Context, dir string, args ...string) (string, error) {
		return "", errors.New("fatal: unable to access 'https://github.com/j333c/sovereign.git/': Could not resolve host: github.com")
	}

	engine := NewUpdateDiscoveryEngine(UpdateDiscoveryConfig{
		GitRun:         runner,
		CurrentVersion: "v1.0.0",
		Channel:        ChannelStable,
	})

	tags, err := engine.QueryUpstreamTags(context.Background())
	if err == nil {
		t.Fatalf("expected network error, got nil")
	}
	if tags != nil {
		t.Fatalf("expected nil tags on error")
	}

	// CheckForUpdates should also return error quietly without panic
	candidate, err := engine.CheckForUpdates(context.Background())
	if err == nil {
		t.Fatalf("expected error from CheckForUpdates, got nil")
	}
	if candidate != nil {
		t.Fatalf("expected nil candidate on network error")
	}
}

func TestUpdateDiscoveryEngine_CheckForUpdates(t *testing.T) {
	mockTags := `
1111111111111111111111111111111111111111	refs/tags/v1.0.0
2222222222222222222222222222222222222222	refs/tags/v1.0.1
3333333333333333333333333333333333333333	refs/tags/v1.1.0-rc.1
`
	runner := func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "ls-remote" {
			return mockTags, nil
		}
		if len(args) >= 2 && args[0] == "describe" {
			return "v1.0.0", nil
		}
		if len(args) >= 3 && args[0] == "tag" {
			return "Security fixes and stability improvements", nil
		}
		if len(args) >= 2 && args[0] == "fetch" {
			return "", nil
		}
		return "", nil
	}

	var foundCandidate *UpdateCandidate
	var mu sync.Mutex

	engine := NewUpdateDiscoveryEngine(UpdateDiscoveryConfig{
		GitRun:         runner,
		CurrentVersion: "v1.0.0",
		Channel:        ChannelStable,
		OnUpdateFound: func(c *UpdateCandidate) {
			mu.Lock()
			foundCandidate = c
			mu.Unlock()
		},
	})

	// 1. Stable channel: should pick v1.0.1 (not v1.1.0-rc.1)
	candidate, err := engine.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error checking updates: %v", err)
	}
	if candidate == nil {
		t.Fatalf("expected candidate, got nil")
	}
	if candidate.TargetTag != "v1.0.1" {
		t.Errorf("expected target tag v1.0.1, got %q", candidate.TargetTag)
	}
	if !strings.Contains(candidate.Changelog, "Security fixes") {
		t.Errorf("expected changelog to contain security fixes, got %q", candidate.Changelog)
	}

	mu.Lock()
	if foundCandidate == nil || foundCandidate.TargetTag != "v1.0.1" {
		t.Errorf("onUpdateFound callback not called or incorrect candidate")
	}
	mu.Unlock()

	// 2. Switch channel to Preview: should pick v1.1.0-rc.1
	engine.SetChannel(ChannelPreview)
	candidate, err = engine.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error checking preview updates: %v", err)
	}
	if candidate == nil || candidate.TargetTag != "v1.1.0-rc.1" {
		t.Errorf("expected preview target tag v1.1.0-rc.1, got %+v", candidate)
	}
}

func TestUpdateDiscoveryEngine_Lifecycle(t *testing.T) {
	runner := func(ctx context.Context, dir string, args ...string) (string, error) {
		return "", nil
	}

	engine := NewUpdateDiscoveryEngine(UpdateDiscoveryConfig{
		GitRun:       runner,
		PollInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	engine.Start(ctx)
	// Quick start/stop should not race
	time.Sleep(20 * time.Millisecond)
	engine.Stop()
	cancel()
}
