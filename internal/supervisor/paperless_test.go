package supervisor

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

func TestPaperless_CalculateOCRWorkers(t *testing.T) {
	tests := []struct {
		name     string
		numCores int
		expected int
	}{
		{
			name:     "Single CPU Core Host",
			numCores: 1,
			expected: 1, // max(1, 1/2) == 1
		},
		{
			name:     "Dual Core Host",
			numCores: 2,
			expected: 1, // max(1, 2/2) == 1
		},
		{
			name:     "Quad Core Host",
			numCores: 4,
			expected: 2, // max(1, 4/2) == 2
		},
		{
			name:     "Octa Core Host",
			numCores: 8,
			expected: 4, // max(1, 8/2) == 4
		},
		{
			name:     "16 Core Host",
			numCores: 16,
			expected: 8, // max(1, 16/2) == 8
		},
		{
			name:     "Zero or negative fallback to runtime NumCPU",
			numCores: 0,
			expected: CalculateOCRWorkers(0), // Uses runtime.NumCPU() >= 1
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateOCRWorkers(tc.numCores)
			if got != tc.expected {
				t.Errorf("CalculateOCRWorkers(%d) = %d; expected %d", tc.numCores, got, tc.expected)
			}
			if got < 1 {
				t.Errorf("OCR worker concurrency must enforce floor of 1, got %d", got)
			}
		})
	}
}

func TestPaperless_DirectoryProvisioningAndVolumePreservation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paperless-prov-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
		NumCPU: func() int { return 4 },
	})

	// Pre-create an existing file in /data/documents/consume to verify preservation
	consumeDir := filepath.Join(tempDir, "documents", "consume")
	if err := os.MkdirAll(consumeDir, 0755); err != nil {
		t.Fatalf("failed to create consume dir: %v", err)
	}
	existingFile := filepath.Join(consumeDir, "pre-existing-policy.pdf")
	dummyContent := []byte("%PDF-1.4 simulated existing policy")
	if err := os.WriteFile(existingFile, dummyContent, 0644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}

	// 1. First provisioning run
	res, err := prov.Provision(ctx)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	if res.ServiceID != ServicePaperless {
		t.Errorf("expected ServiceID %s, got %s", ServicePaperless, res.ServiceID)
	}
	if res.OCRWorkers != 2 {
		t.Errorf("expected 2 OCR workers for 4 cores, got %d", res.OCRWorkers)
	}

	// Verify required storage directories exist with 0755 permissions
	expectedDirs := []string{
		filepath.Join(tempDir, "documents"),
		filepath.Join(tempDir, "documents", "media"),
		filepath.Join(tempDir, "documents", "data"),
		filepath.Join(tempDir, "documents", "consume"),
		filepath.Join(tempDir, "documents", "export"),
		filepath.Join(tempDir, "state", "services", "paperless"),
	}

	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected directory %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
		perm := info.Mode().Perm()
		if perm != 0755 {
			t.Errorf("expected %s permissions to be 0755, got %04o", dir, perm)
		}
	}

	// Verify existing document file was strictly preserved
	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatalf("existing file was removed or corrupted: %v", err)
	}
	if string(content) != string(dummyContent) {
		t.Errorf("expected preserved content %s, got %s", string(dummyContent), string(content))
	}

	// Verify SQLite database was initialized with WAL mode
	dbPath := filepath.Join(tempDir, "documents", "data", "db.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&mode); err != nil {
		t.Fatalf("failed to query PRAGMA journal_mode: %v", err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", mode)
	}

	// Insert test table and data into SQLite DB to test preservation across repeat provisioning
	_, err = db.Exec("CREATE TABLE test_docs (id INTEGER PRIMARY KEY, title TEXT);")
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test_docs (title) VALUES ('Insurance Policy');")
	if err != nil {
		t.Fatalf("failed to insert test record: %v", err)
	}
	db.Close()

	// 2. Second provisioning run (simulating restart or container re-creation)
	res2, err := prov.Provision(ctx)
	if err != nil {
		t.Fatalf("Repeat provision failed: %v", err)
	}
	if res2.OCRWorkers != 2 {
		t.Errorf("expected repeat provision OCRWorkers 2, got %d", res2.OCRWorkers)
	}

	// Verify test table and record still exist intact
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to reopen sqlite database: %v", err)
	}
	defer db2.Close()
	var title string
	if err := db2.QueryRow("SELECT title FROM test_docs WHERE id = 1;").Scan(&title); err != nil {
		t.Fatalf("failed to query preserved record: %v", err)
	}
	if title != "Insurance Policy" {
		t.Errorf("expected preserved title 'Insurance Policy', got %q", title)
	}
}

func TestPaperless_EnvironmentVariables(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paperless-env-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	t.Run("Single Core Concurrency and Subpath", func(t *testing.T) {
		prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
			NumCPU: func() int { return 1 },
		})
		res, err := prov.Provision(ctx)
		if err != nil {
			t.Fatalf("Provision failed: %v", err)
		}

		env := res.Env
		if env["PAPERLESS_OCR_WORKERS"] != "1" {
			t.Errorf("expected PAPERLESS_OCR_WORKERS=1, got %s", env["PAPERLESS_OCR_WORKERS"])
		}
		if env["PAPERLESS_TASK_WORKERS"] != "1" {
			t.Errorf("expected PAPERLESS_TASK_WORKERS=1, got %s", env["PAPERLESS_TASK_WORKERS"])
		}
		if env["PAPERLESS_OCR_PROCESSES"] != "1" {
			t.Errorf("expected PAPERLESS_OCR_PROCESSES='1', got %s", env["PAPERLESS_OCR_PROCESSES"])
		}
		if env["PAPERLESS_THREADS_PER_WORKER"] != "1" {
			t.Errorf("expected PAPERLESS_THREADS_PER_WORKER='1', got %s", env["PAPERLESS_THREADS_PER_WORKER"])
		}
		if env["PAPERLESS_FORCE_SCRIPT_NAME"] != "/docs" {
			t.Errorf("expected PAPERLESS_FORCE_SCRIPT_NAME='/docs', got %s", env["PAPERLESS_FORCE_SCRIPT_NAME"])
		}
		if env["PAPERLESS_URL"] != "http://127.0.0.1:8000/docs" {
			t.Errorf("expected PAPERLESS_URL='http://127.0.0.1:8000/docs', got %s", env["PAPERLESS_URL"])
		}
		if env["PAPERLESS_REDIS"] != "redis://sovereign-redis:6379" {
			t.Errorf("expected PAPERLESS_REDIS='redis://sovereign-redis:6379', got %s", env["PAPERLESS_REDIS"])
		}
		if env["USERMAP_UID"] != "1000" {
			t.Errorf("expected USERMAP_UID='1000', got %s", env["USERMAP_UID"])
		}
		if env["USERMAP_GID"] != "1000" {
			t.Errorf("expected USERMAP_GID='1000', got %s", env["USERMAP_GID"])
		}
		if env["PAPERLESS_OCR_LANGUAGES"] != "deu eng" {
			t.Errorf("expected PAPERLESS_OCR_LANGUAGES='deu eng', got %s", env["PAPERLESS_OCR_LANGUAGES"])
		}
		if env["PAPERLESS_OCR_LANGUAGE"] != "eng" {
			t.Errorf("expected PAPERLESS_OCR_LANGUAGE='eng', got %s", env["PAPERLESS_OCR_LANGUAGE"])
		}
		if env["PAPERLESS_PORT"] != "8000" {
			t.Errorf("expected PAPERLESS_PORT='8000', got %s", env["PAPERLESS_PORT"])
		}
		if env["PAPERLESS_DATA_DIR"] != "/usr/src/paperless/data" {
			t.Errorf("expected PAPERLESS_DATA_DIR='/usr/src/paperless/data', got %s", env["PAPERLESS_DATA_DIR"])
		}
		if env["PAPERLESS_MEDIA_ROOT"] != "/usr/src/paperless/media" {
			t.Errorf("expected PAPERLESS_MEDIA_ROOT='/usr/src/paperless/media', got %s", env["PAPERLESS_MEDIA_ROOT"])
		}
		if env["PAPERLESS_CONSUMPTION_DIR"] != "/usr/src/paperless/consume" {
			t.Errorf("expected PAPERLESS_CONSUMPTION_DIR='/usr/src/paperless/consume', got %s", env["PAPERLESS_CONSUMPTION_DIR"])
		}
		if env["PAPERLESS_EXPORT_DIR"] != "/usr/src/paperless/export" {
			t.Errorf("expected PAPERLESS_EXPORT_DIR='/usr/src/paperless/export', got %s", env["PAPERLESS_EXPORT_DIR"])
		}
	})

	t.Run("Multi-Core Concurrency Capped at Half Cores", func(t *testing.T) {
		prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
			NumCPU: func() int { return 8 },
		})
		res, err := prov.Provision(ctx)
		if err != nil {
			t.Fatalf("Provision failed: %v", err)
		}

		env := res.Env
		if env["PAPERLESS_OCR_WORKERS"] != "4" {
			t.Errorf("expected PAPERLESS_OCR_WORKERS=4 for 8 cores, got %s", env["PAPERLESS_OCR_WORKERS"])
		}
		if env["PAPERLESS_TASK_WORKERS"] != "4" {
			t.Errorf("expected PAPERLESS_TASK_WORKERS=4 for 8 cores, got %s", env["PAPERLESS_TASK_WORKERS"])
		}
	})

	t.Run("Custom BaseURL Resolution", func(t *testing.T) {
		prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
			BaseURL: "https://sovereign.lan:8443/",
		})
		res, err := prov.Provision(ctx)
		if err != nil {
			t.Fatalf("Provision failed: %v", err)
		}
		if res.Env["PAPERLESS_URL"] != "https://sovereign.lan:8443/docs" {
			t.Errorf("expected PAPERLESS_URL='https://sovereign.lan:8443/docs', got %s", res.Env["PAPERLESS_URL"])
		}
	})
}

func TestPaperless_ComposeSynthesis_AndSecurityBoundaries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paperless-compose-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockEngine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
		PaperlessProvisioner: NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
			NumCPU: func() int { return 8 },
		}),
	})
	defer mgr.Close()

	ctx := context.Background()
	svc, err := mgr.ActivateService(ctx, ServicePaperless)
	if err != nil {
		t.Fatalf("failed to activate Paperless: %v", err)
	}

	// 1. Verify memory limits in ServiceStatus
	if svc.MemLimitBytes != MemLimitPaperless {
		t.Errorf("expected MemLimitBytes %d (1024MB), got %d", MemLimitPaperless, svc.MemLimitBytes)
	}
	if svc.MemReservationBytes != MemReservationPaperless {
		t.Errorf("expected MemReservationBytes %d (512MB), got %d", MemReservationPaperless, svc.MemReservationBytes)
	}
	if svc.Port != 8000 {
		t.Errorf("expected Port 8000, got %d", svc.Port)
	}
	if svc.Subpath != "/docs" {
		t.Errorf("expected Subpath /docs, got %s", svc.Subpath)
	}

	// 2. Inspect synthesized Compose YAML file
	composePath := filepath.Join(tempDir, "state", "compose", "paperless.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("compose file not found at %s: %v", composePath, err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("failed to parse synthesized compose yaml: %v", err)
	}

	spec, exists := compose.Services["paperless"]
	if !exists {
		t.Fatalf("expected service 'paperless' in compose file")
	}

	if spec.Image != ImagePaperless {
		t.Errorf("expected image %s, got %s", ImagePaperless, spec.Image)
	}
	if spec.ContainerName != "sovereign-paperless" {
		t.Errorf("expected container_name 'sovereign-paperless', got %s", spec.ContainerName)
	}
	if spec.Restart != "unless-stopped" {
		t.Errorf("expected restart 'unless-stopped', got %s", spec.Restart)
	}

	// Verify memory limits in Compose deploy block
	if spec.Deploy == nil {
		t.Fatalf("expected deploy block in compose spec")
	}
	if spec.Deploy.Resources.Limits.Memory != "1024M" {
		t.Errorf("expected memory limit '1024M', got %s", spec.Deploy.Resources.Limits.Memory)
	}
	if spec.Deploy.Resources.Reservations.Memory != "512M" {
		t.Errorf("expected memory reservation '512M', got %s", spec.Deploy.Resources.Reservations.Memory)
	}

	// Verify environment variables in Compose spec
	if spec.Environment["PAPERLESS_OCR_WORKERS"] != "4" {
		t.Errorf("expected PAPERLESS_OCR_WORKERS=4 in compose env, got %s", spec.Environment["PAPERLESS_OCR_WORKERS"])
	}
	if spec.Environment["PAPERLESS_OCR_PROCESSES"] != "1" {
		t.Errorf("expected PAPERLESS_OCR_PROCESSES=1 in compose env, got %s", spec.Environment["PAPERLESS_OCR_PROCESSES"])
	}
	if spec.Environment["PAPERLESS_THREADS_PER_WORKER"] != "1" {
		t.Errorf("expected PAPERLESS_THREADS_PER_WORKER=1 in compose env, got %s", spec.Environment["PAPERLESS_THREADS_PER_WORKER"])
	}
	if spec.Environment["PAPERLESS_FORCE_SCRIPT_NAME"] != "/docs" {
		t.Errorf("expected PAPERLESS_FORCE_SCRIPT_NAME=/docs in compose env, got %s", spec.Environment["PAPERLESS_FORCE_SCRIPT_NAME"])
	}
	if spec.Environment["PAPERLESS_URL"] != "http://127.0.0.1:8000/docs" {
		t.Errorf("expected PAPERLESS_URL=http://127.0.0.1:8000/docs in compose env, got %s", spec.Environment["PAPERLESS_URL"])
	}
	if spec.Environment["PAPERLESS_REDIS"] != "redis://sovereign-redis:6379" {
		t.Errorf("expected PAPERLESS_REDIS=redis://sovereign-redis:6379 in compose env, got %s", spec.Environment["PAPERLESS_REDIS"])
	}

	// Verify ports: bound to 127.0.0.1:8000:8000
	foundPort := false
	for _, p := range spec.Ports {
		if strings.Contains(p, "8000:8000") {
			foundPort = true
		}
	}
	if !foundPort {
		t.Errorf("expected port 8000:8000 in compose ports, got: %v", spec.Ports)
	}

	// Verify security boundary: Docker socket must NEVER be mounted
	for _, v := range spec.Volumes {
		if strings.Contains(v, "docker.sock") {
			t.Fatalf("SECURITY VIOLATION: docker.sock mounted into Paperless container: %s", v)
		}
	}

	// Verify volumes mapped under /data/documents
	foundMedia := false
	foundData := false
	foundConsume := false
	foundExport := false
	for _, v := range spec.Volumes {
		if strings.Contains(v, "documents/media") {
			foundMedia = true
		}
		if strings.Contains(v, "documents/data") {
			foundData = true
		}
		if strings.Contains(v, "documents/consume") {
			foundConsume = true
		}
		if strings.Contains(v, "documents/export") {
			foundExport = true
		}
	}
	if !foundMedia || !foundData || !foundConsume || !foundExport {
		t.Errorf("expected all documents subdirectories mounted, got volumes: %v", spec.Volumes)
	}

	// 3. Test Deactivation strictly preserves /data/documents
	deactSvc, err := mgr.DeactivateService(ctx, ServicePaperless)
	if err != nil {
		t.Fatalf("DeactivateService failed: %v", err)
	}
	if deactSvc.Status != StatusInactive {
		t.Errorf("expected inactive status, got %s", deactSvc.Status)
	}

	// Verify storage directories and SQLite database are still intact
	if _, err := os.Stat(filepath.Join(tempDir, "documents", "data", "db.sqlite3")); err != nil {
		t.Fatalf("SQLite database was deleted during deactivation: %v", err)
	}
}

func TestPaperless_AliasResolution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paperless-alias-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockEngine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir: tempDir,
		Engine:  mockEngine,
	})
	defer mgr.Close()

	ctx := context.Background()

	aliases := []string{"dms", "documents", "paperless"}
	for _, alias := range aliases {
		svc, err := mgr.GetService(ctx, alias)
		if err != nil {
			t.Fatalf("GetService(%q) failed: %v", alias, err)
		}
		if svc.ID != ServicePaperless {
			t.Errorf("GetService(%q) returned ID %s, expected %s", alias, svc.ID, ServicePaperless)
		}
		if svc.Port != 8000 {
			t.Errorf("GetService(%q) returned Port %d, expected 8000", alias, svc.Port)
		}
		if svc.Subpath != "/docs" {
			t.Errorf("GetService(%q) returned Subpath %s, expected /docs", alias, svc.Subpath)
		}
	}
}

func TestEnsureSQLiteWAL_EmptyPath(t *testing.T) {
	err := EnsureSQLiteWAL("")
	if err == nil {
		t.Fatalf("expected error when passing empty db path to EnsureSQLiteWAL, got nil")
	}
}

func TestPaperless_SetNumCPUFuncSafety(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paperless-cpu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{
		NumCPU: nil, // should safely fallback to runtime.NumCPU
	})
	prov.SetNumCPUFunc(nil) // setting nil explicitly should not panic or corrupt
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed with nil NumCPU func: %v", err)
	}
	if res.OCRWorkers < 1 {
		t.Errorf("expected at least 1 OCR worker, got %d", res.OCRWorkers)
	}
}

func TestPaperless_ProvisionContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	prov := NewPaperlessProvisioner(tempDir, nil, PaperlessProvisionerOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := prov.Provision(ctx)
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}
}
