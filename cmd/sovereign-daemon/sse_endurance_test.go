package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"sovereign/internal/optical"
	"sovereign/internal/supervisor"
)

// TestSSE_Endurance verifies that /api/v1/telemetry/events remains open and stable
// for continuous streaming without server-side write timeout drops, mid-stream memory leaks,
// or unhandled socket closure errors.
//
// By default, the endurance test runs for 5 seconds against a server configured with
// an aggressive 1-second WriteTimeout, proving that http.ResponseController cleanly clears
// write deadlines and sustains the stream across multiple timeout cycles.
// To run the full 300-second endurance suite, set SOVEREIGN_SSE_ENDURANCE_SECONDS=300
// or SOVEREIGN_FULL_ENDURANCE=1.
func TestSSE_Endurance(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(dataDir, "state")
	_ = os.MkdirAll(stateDir, 0755)
	_ = os.WriteFile(filepath.Join(stateDir, "boot_state.json"), []byte(`{"clean_shutdown": true}`), 0644)

	mockEngine := supervisor.NewMockRuntimeEngine()
	svcMgr := supervisor.NewServiceManager(supervisor.ServiceManagerConfig{
		DataDir: dataDir,
		Engine:  mockEngine,
	})
	mockOptical := optical.NewMockOpticalDriver()

	// High-frequency telemetry interval (50ms) to thoroughly exercise buffer flushing and memory hygiene
	telemetryInterval := 50 * time.Millisecond
	srv, err := NewServer(Config{
		DataDir:           dataDir,
		ServiceMgr:        svcMgr,
		OpticalDriver:     mockOptical,
		TelemetryInterval: telemetryInterval,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	// 1. Establish HTTP server on local loopback with an aggressive WriteTimeout = 1s.
	// This directly verifies that handleEvents disables deadlines via http.ResponseController,
	// preventing Go's http.Server from forcibly terminating the long-lived SSE stream.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on local TCP: %v", err)
	}
	defer listener.Close()

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      1 * time.Second, // Aggressive timeout to test override
		IdleTimeout:       30 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	serverURL := fmt.Sprintf("http://%s", listener.Addr().String())

	// Determine endurance duration
	duration := 5 * time.Second
	if envSec := os.Getenv("SOVEREIGN_SSE_ENDURANCE_SECONDS"); envSec != "" {
		if s, err := strconv.Atoi(envSec); err == nil && s > 0 {
			duration = time.Duration(s) * time.Second
		}
	} else if os.Getenv("SOVEREIGN_FULL_ENDURANCE") == "1" {
		duration = 300 * time.Second
	}

	t.Logf("Running SSE endurance test against %s for %v (telemetry interval: %v, server WriteTimeout: %v)",
		serverURL, duration, telemetryInterval, httpServer.WriteTimeout)

	// 2. Connect client to /api/v1/telemetry/events
	endpoint := fmt.Sprintf("%s/api/v1/telemetry/events", serverURL)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		Timeout: 0, // No client-side timeout for streaming
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE endpoint %s: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 from SSE endpoint, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}

	// 3. Monitor memory baseline before stream endurance loop
	runtime.GC()
	var memStart runtime.MemStats
	runtime.ReadMemStats(&memStart)

	// 4. Stream endurance loop
	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(duration)
	eventsReceived := 0
	lastDockerStatus := ""

	type streamReadResult struct {
		line string
		err  error
	}

	lineCh := make(chan streamReadResult, 100)
	readCtx, readCancel := context.WithCancel(context.Background())
	defer readCancel()

	go func() {
		for {
			select {
			case <-readCtx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				lineCh <- streamReadResult{line: line, err: err}
				if err != nil {
					return
				}
			}
		}
	}()

	currentEvent := ""
	for time.Now().Before(deadline) {
		select {
		case res := <-lineCh:
			if res.err != nil {
				t.Fatalf("unexpected stream read error before duration expiry (%v elapsed): %v",
					duration-time.Until(deadline), res.err)
			}
			line := strings.TrimSpace(res.line)
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") && currentEvent != "" {
				dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				var update TelemetryUpdate
				if err := json.Unmarshal([]byte(dataStr), &update); err == nil {
					eventsReceived++
					lastDockerStatus = update.DockerStatus
				}
				currentEvent = ""
			}
		case err := <-serverErrCh:
			t.Fatalf("server encountered unexpected error during streaming: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatalf("stream stalled: timed out waiting for next line from SSE stream")
		}
	}

	// 5. Verify received events and continuity
	minExpectedEvents := int(duration / (telemetryInterval * 2)) // Conservatively allow for scheduling jitter
	if eventsReceived < minExpectedEvents {
		t.Errorf("expected at least %d telemetry events over %v, got %d", minExpectedEvents, duration, eventsReceived)
	}

	t.Logf("SSE stream endurance verified: received %d telemetry events stably over %v (last Docker status: %s)",
		eventsReceived, duration, lastDockerStatus)

	// 6. Memory hygiene verification: check heap allocation delta
	runtime.GC()
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	// Ensure heap growth is bounded (< 20MB after GC)
	var heapDelta int64
	if memEnd.HeapAlloc > memStart.HeapAlloc {
		heapDelta = int64(memEnd.HeapAlloc - memStart.HeapAlloc)
	}
	t.Logf("Memory stats: start heap=%d KB, end heap=%d KB (delta: %d KB)",
		memStart.HeapAlloc/1024, memEnd.HeapAlloc/1024, heapDelta/1024)

	const maxAllowedHeapGrowth = 20 * 1024 * 1024 // 20 MB
	if heapDelta > maxAllowedHeapGrowth {
		t.Errorf("potential memory leak detected: heap grew by %d MB during SSE stream", heapDelta/(1024*1024))
	}

	// 7. Verify subscriber channel cleanup upon client disconnect
	readCancel()
	_ = resp.Body.Close()

	// Wait up to 2 seconds for server handler defer to delete subscriber channel
	cleanupDeadline := time.Now().Add(2 * time.Second)
	cleanedUp := false
	for time.Now().Before(cleanupDeadline) {
		srv.eventsMu.RLock()
		subCount := len(srv.eventSubscribers)
		srv.eventsMu.RUnlock()
		if subCount == 0 {
			cleanedUp = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !cleanedUp {
		srv.eventsMu.RLock()
		subCount := len(srv.eventSubscribers)
		srv.eventsMu.RUnlock()
		t.Errorf("expected 0 active subscribers after client disconnect, got %d", subCount)
	} else {
		t.Log("Subscriber channel cleanly deregistered after client disconnect")
	}
}
