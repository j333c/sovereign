package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"sovereign/internal/supervisor"
)

func TestUpdateDiscovery_SSEBroadcast(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	mockTags := `
1111111111111111111111111111111111111111	refs/tags/v0.1.0
2222222222222222222222222222222222222222	refs/tags/v0.2.0
`
	runner := func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "ls-remote" {
			return mockTags, nil
		}
		if len(args) >= 2 && args[0] == "describe" {
			return "v0.1.0", nil
		}
		if len(args) >= 3 && args[0] == "tag" {
			return "V0.2.0 features and security improvements", nil
		}
		return "", nil
	}

	mockEngine := supervisor.NewUpdateDiscoveryEngine(supervisor.UpdateDiscoveryConfig{
		GitRun:         runner,
		CurrentVersion: "v0.1.0",
		Channel:        supervisor.ChannelStable,
	})

	srv, err := NewServer(Config{
		ListenAddr:   ":0",
		DataDir:      tempDir,
		DBPath:       dbPath,
		UpdateEngine: mockEngine,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	// Subscribe to SSE events
	subCh := make(chan sseEvent, 10)
	srv.eventsMu.Lock()
	srv.eventSubscribers[subCh] = struct{}{}
	srv.eventsMu.Unlock()

	defer func() {
		srv.eventsMu.Lock()
		delete(srv.eventSubscribers, subCh)
		srv.eventsMu.Unlock()
	}()

	// Wire mockEngine callback to srv.BroadcastEvent as done in production
	mockEngine = supervisor.NewUpdateDiscoveryEngine(supervisor.UpdateDiscoveryConfig{
		GitRun:         runner,
		CurrentVersion: "v0.1.0",
		Channel:        supervisor.ChannelStable,
		OnUpdateFound: func(candidate *supervisor.UpdateCandidate) {
			srv.BroadcastEvent("system.update_available", map[string]any{
				"target_tag":      candidate.TargetTag,
				"target_version":  candidate.TargetVersion.String(),
				"current_version": candidate.CurrentVersion.String(),
				"channel":         candidate.Channel,
				"changelog":       candidate.Changelog,
			})
		},
	})

	candidate, err := mockEngine.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error checking updates: %v", err)
	}
	if candidate == nil || candidate.TargetTag != "v0.2.0" {
		t.Fatalf("expected candidate v0.2.0, got %+v", candidate)
	}

	// Verify SSE broadcast received
	select {
	case ev := <-subCh:
		if ev.Event != "system.update_available" {
			t.Errorf("expected event 'system.update_available', got %q", ev.Event)
		}
		dataMap, ok := ev.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any data, got %T", ev.Data)
		}
		if dataMap["target_tag"] != "v0.2.0" {
			t.Errorf("expected target_tag 'v0.2.0', got %v", dataMap["target_tag"])
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for system.update_available SSE event")
	}
}
