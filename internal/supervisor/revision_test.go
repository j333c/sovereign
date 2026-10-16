package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"sovereign/internal/store"
)

func TestSynthesizeRevisionCompose(t *testing.T) {
	tempDir := t.TempDir()

	spec := ContainerSpec{
		Name:                   "jellyfin",
		Image:                  "jellyfin/jellyfin:12.3.0",
		MemoryLimitBytes:       2048 * 1024 * 1024,
		MemoryReservationBytes: 1024 * 1024 * 1024,
		Ports:                  map[int]int{8096: 8096},
		Volumes: []string{
			filepath.Join(tempDir, "media/movies") + ":/data/media/movies",
		},
		Networks: []string{"sovereign-ingress", "sovereign-control"},
	}

	path, err := SynthesizeRevisionCompose(tempDir, 1, spec)
	if err != nil {
		t.Fatalf("SynthesizeRevisionCompose failed: %v", err)
	}

	expectedPath := filepath.Join(tempDir, "state", "compose", "rev-1.yaml")
	if path != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read synthesized revision compose file: %v", err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("failed to unmarshal synthesized compose yaml: %v", err)
	}

	svc, exists := compose.Services["jellyfin"]
	if !exists {
		t.Fatalf("service jellyfin not found in synthesized compose")
	}

	if svc.Image != "jellyfin/jellyfin:12.3.0" {
		t.Errorf("expected image 'jellyfin/jellyfin:12.3.0', got %q", svc.Image)
	}
	if svc.ContainerName != "sovereign-jellyfin" {
		t.Errorf("expected container name 'sovereign-jellyfin', got %q", svc.ContainerName)
	}
	if svc.Deploy == nil || svc.Deploy.Resources.Limits.Memory != "2048M" {
		t.Errorf("expected memory limit 2048M, got %+v", svc.Deploy)
	}
}

func TestSnapshotConfigurationVolumes(t *testing.T) {
	tempDir := t.TempDir()

	// Create dummy service config directory and files
	cfgDir := filepath.Join(tempDir, "state", "services", "jellyfin", "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}
	sampleFile := filepath.Join(cfgDir, "system.xml")
	if err := os.WriteFile(sampleFile, []byte("<test>sample</test>"), 0644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	templates := []string{
		"state/services/jellyfin/config:/config",
	}

	backupDir, err := SnapshotConfigurationVolumes(tempDir, 1, "jellyfin", templates)
	if err != nil {
		t.Fatalf("SnapshotConfigurationVolumes failed: %v", err)
	}

	expectedBackupDir := filepath.Join(tempDir, "state", "backups", "volumes-rev-1", "jellyfin")
	if backupDir != expectedBackupDir {
		t.Fatalf("expected backup dir %q, got %q", expectedBackupDir, backupDir)
	}

	// Verify backup file exists
	backedUpFile := filepath.Join(backupDir, "config", "system.xml")
	if _, err := os.Stat(backedUpFile); err != nil {
		// Could also be backed up directly or under config
		altFile := filepath.Join(backupDir, "system.xml")
		if _, errAlt := os.Stat(altFile); errAlt != nil {
			t.Fatalf("backed up file not found at %q or %q: %v", backedUpFile, altFile, err)
		}
	}
}

func TestCreatePreUpgradeRevision(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer db.Close()

	// Seed dummy config in service state dir
	cfgDir := filepath.Join(tempDir, "state", "services", "jellyfin")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("failed to create cfg dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "jellyfin.json"), []byte(`{"version": 1}`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		RevisionStore: db,
	})

	ctx := context.Background()

	// 1. Create Pre-Upgrade Revision
	rev, err := mgr.CreatePreUpgradeRevision(ctx, "jellyfin", "12.3.0")
	if err != nil {
		t.Fatalf("CreatePreUpgradeRevision failed: %v", err)
	}

	if rev.Revision != 1 {
		t.Errorf("expected revision 1, got %d", rev.Revision)
	}
	if rev.ServiceID != "jellyfin" {
		t.Errorf("expected service jellyfin, got %q", rev.ServiceID)
	}
	if rev.OldImage != ImageJellyfin {
		t.Errorf("expected old image %q, got %q", ImageJellyfin, rev.OldImage)
	}
	if rev.NewImage != "jellyfin/jellyfin:12.3.0" {
		t.Errorf("expected new image 'jellyfin/jellyfin:12.3.0', got %q", rev.NewImage)
	}

	// Verify SQLite snapshot file exists and is intact
	if _, err := os.Stat(rev.SQLiteSnapshotPath); err != nil {
		t.Fatalf("sqlite snapshot file not found: %v", err)
	}
	snapDB, err := store.Open(rev.SQLiteSnapshotPath)
	if err != nil {
		t.Fatalf("failed to open snapshot db: %v", err)
	}
	defer snapDB.Close()

	ok, msg, err := snapDB.CheckIntegrity(ctx)
	if err != nil || !ok || msg != "ok" {
		t.Fatalf("snapshot integrity check failed: ok=%v, msg=%s, err=%v", ok, msg, err)
	}

	// Verify declarative compose rev file exists
	if _, err := os.Stat(rev.ComposeFilePath); err != nil {
		t.Fatalf("compose revision file not found: %v", err)
	}
	composeBytes, err := os.ReadFile(rev.ComposeFilePath)
	if err != nil {
		t.Fatalf("failed to read compose file: %v", err)
	}
	if !strings.Contains(string(composeBytes), "jellyfin/jellyfin:12.3.0") {
		t.Errorf("compose file does not contain updated image: %s", string(composeBytes))
	}

	// Verify active revision tracked in DB
	activeRev, err := db.GetActiveRevision(ctx)
	if err != nil {
		t.Fatalf("GetActiveRevision failed: %v", err)
	}
	if activeRev.Revision != 1 || activeRev.Status != "active" {
		t.Fatalf("unexpected active revision in store: %+v", activeRev)
	}

	// 2. Non-existent service returns ErrServiceNotFound
	if _, err := mgr.CreatePreUpgradeRevision(ctx, "nonexistent", "1.0.0"); err != ErrServiceNotFound {
		t.Fatalf("expected ErrServiceNotFound, got: %v", err)
	}
}

func TestUpgradeService(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state", "sovereign.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer db.Close()

	engine := NewMockRuntimeEngine()
	mgr := NewServiceManager(ServiceManagerConfig{
		DataDir:       tempDir,
		Engine:        engine,
		RevisionStore: db,
	})

	ctx := context.Background()

	// Activate service first
	st, err := mgr.ActivateService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("ActivateService failed: %v", err)
	}
	if st.Status != StatusRunning {
		t.Fatalf("expected status running, got %q", st.Status)
	}

	// Upgrade service to 12.3.0
	rev, err := mgr.UpgradeService(ctx, "jellyfin", "12.3.0")
	if err != nil {
		t.Fatalf("UpgradeService failed: %v", err)
	}

	if rev.Revision != 1 {
		t.Errorf("expected revision 1, got %d", rev.Revision)
	}
	if rev.NewImage != "jellyfin/jellyfin:12.3.0" {
		t.Errorf("expected new image jellyfin/jellyfin:12.3.0, got %q", rev.NewImage)
	}

	// Verify engine pulled the new image
	pulled := engine.GetPulledHistory()
	found := false
	for _, img := range pulled {
		if img == "jellyfin/jellyfin:12.3.0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected image 'jellyfin/jellyfin:12.3.0' to be pulled, got history: %v", pulled)
	}

	// Verify in-memory status was updated
	updatedSt, err := mgr.GetService(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("GetService failed: %v", err)
	}
	if updatedSt.Image != "jellyfin/jellyfin:12.3.0" {
		t.Errorf("expected updated status image 'jellyfin/jellyfin:12.3.0', got %q", updatedSt.Image)
	}

	// Upgrade with tag prefix (e.g. ":12.4.0")
	rev2, err := mgr.UpgradeService(ctx, "jellyfin", ":12.4.0")
	if err != nil {
		t.Fatalf("UpgradeService to :12.4.0 failed: %v", err)
	}
	if rev2.Revision != 2 || rev2.NewImage != "jellyfin/jellyfin:12.4.0" {
		t.Errorf("unexpected rev2: %+v", rev2)
	}

	// Non-existent service upgrade fails
	if _, err := mgr.UpgradeService(ctx, "unknown-svc", "1.0"); err != ErrServiceNotFound {
		t.Fatalf("expected ErrServiceNotFound, got %v", err)
	}
}
