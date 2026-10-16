package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWebUIProvisioner_ContainerSpec(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openwebui-spec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	customStateDir := filepath.Join(tempDir, "custom-openwebui")
	customGateway := "http://sovereign-daemon:8080/v1"

	provisioner := NewOpenWebUIProvisioner(tempDir, nil, OpenWebUIProvisionerOptions{
		StateDir:   customStateDir,
		GatewayURL: customGateway,
	})

	res, err := provisioner.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	spec := res.ContainerSpec

	// 1. Image pin
	if spec.Image != ImageOpenWebUI {
		t.Errorf("expected image %q, got %q", ImageOpenWebUI, spec.Image)
	}

	// 2. Memory bounds (1024MB limit, 512MB reservation)
	if spec.MemoryLimitBytes != MemLimitOpenWebUI {
		t.Errorf("expected memory limit %d, got %d", MemLimitOpenWebUI, spec.MemoryLimitBytes)
	}
	if spec.MemoryReservationBytes != MemReservationOpenWebUI {
		t.Errorf("expected memory reservation %d, got %d", MemReservationOpenWebUI, spec.MemoryReservationBytes)
	}

	// 3. Ports
	if spec.Ports[3001] != 3001 {
		t.Errorf("expected port 3001:3001, got %v", spec.Ports)
	}

	// 4. Networks
	hasIngress := false
	hasAI := false
	for _, net := range spec.Networks {
		if net == "sovereign-ingress" {
			hasIngress = true
		}
		if net == "sovereign-ai" {
			hasAI = true
		}
	}
	if !hasIngress || !hasAI {
		t.Errorf("expected networks [sovereign-ingress, sovereign-ai], got %v", spec.Networks)
	}

	// 5. Zero Docker socket mounts invariant
	for _, v := range spec.Volumes {
		if strings.Contains(v, "docker.sock") {
			t.Fatalf("security violation: docker.sock mounted in volume %q", v)
		}
	}

	// 6. Persistence volume to /app/backend/data
	foundMount := false
	expectedMount := customStateDir + ":/app/backend/data"
	for _, v := range spec.Volumes {
		if v == expectedMount {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Errorf("expected volume %q in spec volumes %v", expectedMount, spec.Volumes)
	}

	// 7. Strict telemetry suppression and AI Gateway connection
	expectedEnvs := map[string]string{
		"PORT":                     "3001",
		"OPENAI_API_BASE_URL":      customGateway,
		"OPENAI_API_KEY":           "sk-sovereign-local",
		"WEBUI_AUTH":               "False",
		"SCARF_NO_ANALYTICS":       "True",
		"DO_NOT_TRACK":             "True",
		"ANONYMIZED_TELEMETRY":     "False",
		"ENABLE_COMMUNITY_SHARING": "False",
		"WEBUI_NAME":               "Sovereign AI",
		"DATA_DIR":                 "/app/backend/data",
	}

	for k, expectedVal := range expectedEnvs {
		if val, ok := spec.Env[k]; !ok || val != expectedVal {
			t.Errorf("expected env %s=%q, got %q (exists=%v)", k, expectedVal, val, ok)
		}
	}
}

func TestOpenWebUIProvisioner_StateDirectoryCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openwebui-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateDir := filepath.Join(tempDir, "state/services/openwebui")
	p := NewOpenWebUIProvisioner(tempDir, nil, OpenWebUIProvisionerOptions{
		StateDir: stateDir,
	})

	if err := p.EnsureStateDir(); err != nil {
		t.Fatalf("EnsureStateDir failed: %v", err)
	}

	fi, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("state directory does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected state directory to be a directory")
	}
}

func TestSupervisor_OpenWebUI_Lifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openwebui-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockEngine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})

	ctx := context.Background()

	// 1. ResolveID aliases
	aliases := []string{"chat", "open-webui", "openwebui"}
	for _, a := range aliases {
		resolved := mgr.resolveID(a)
		if resolved != ServiceOpenWebUI {
			t.Errorf("resolveID(%q) = %q, expected %q", a, resolved, ServiceOpenWebUI)
		}
	}

	// 2. GetService initial inactive state
	svc, err := mgr.GetService(ctx, "chat")
	if err != nil {
		t.Fatalf("GetService(chat) failed: %v", err)
	}
	if svc.ID != ServiceOpenWebUI {
		t.Errorf("expected ID %q, got %q", ServiceOpenWebUI, svc.ID)
	}
	if svc.Status != StatusInactive {
		t.Errorf("expected initial status %q, got %q", StatusInactive, svc.Status)
	}
	if svc.Port != 3001 || svc.Subpath != "/chat" {
		t.Errorf("expected port 3001 and subpath /chat, got port %d and subpath %q", svc.Port, svc.Subpath)
	}

	// 3. ActivateService
	activated, err := mgr.ActivateService(ctx, "chat")
	if err != nil {
		t.Fatalf("ActivateService(chat) failed: %v", err)
	}
	if activated.Status != StatusRunning {
		t.Errorf("expected activated status %q, got %q", StatusRunning, activated.Status)
	}

	// Verify started in mock engine
	started := mockEngine.GetStartedHistory()
	var spec ContainerSpec
	found := false
	for _, s := range started {
		if s.Name == ServiceOpenWebUI {
			spec = s
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected container %q in mock engine started history", ServiceOpenWebUI)
	}
	if spec.Image != ImageOpenWebUI {
		t.Errorf("expected image %q, got %q", ImageOpenWebUI, spec.Image)
	}
	if spec.Env["DO_NOT_TRACK"] != "True" {
		t.Errorf("expected DO_NOT_TRACK=True in container spec env")
	}

	// 4. RestartService
	restarted, err := mgr.RestartService(ctx, "open-webui")
	if err != nil {
		t.Fatalf("RestartService(open-webui) failed: %v", err)
	}
	if restarted.Status != StatusRunning {
		t.Errorf("expected restarted status %q, got %q", StatusRunning, restarted.Status)
	}
}
