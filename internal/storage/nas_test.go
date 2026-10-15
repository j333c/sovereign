package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"sovereign/internal/store"
)

func setupTestStorageEngine(t *testing.T) (*Engine, string, *store.DB) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	dbPath := filepath.Join(dataDir, "state", "sovereign.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	diskOps := NewDefaultDiskOps(nil)
	diskOps.SetSafeMode(true)

	nasOps := NewDefaultNASOps(nil)
	nasOps.SetSafeMode(true)

	engine, err := NewEngine(EngineConfig{
		DataDir: dataDir,
		DB:      db,
		DiskOps: diskOps,
		NASOps:  nasOps,
	})
	if err != nil {
		t.Fatalf("failed to create storage engine: %v", err)
	}

	return engine, dataDir, db
}

func TestStorageEngine_DiskEnumeration(t *testing.T) {
	engine, _, _ := setupTestStorageEngine(t)
	defer engine.Close()

	defaultOps := engine.DiskOps().(*DefaultDiskOps)

	// Case 1: Mock disks present
	mockDisks := []DiskInfo{
		{
			Name:       "sda",
			Path:       "/dev/sda",
			Model:      "Samsung SSD 980 1TB",
			SizeBytes:  1000204886016,
			Type:       DiskTypeNVMe,
			Rotational: false,
			Mounted:    false,
			IsSystem:   false,
		},
		{
			Name:       "sdb",
			Path:       "/dev/sdb",
			Model:      "WDC WD40EFAX",
			SizeBytes:  4000787030016,
			Type:       DiskTypeSATA,
			Rotational: true,
			Mounted:    true,
			MountPoint: "/",
			IsSystem:   true, // System root disk!
		},
		{
			Name:       "sdc",
			Path:       "/dev/sdc",
			Model:      "Crucial MX500",
			SizeBytes:  500107862016,
			Type:       DiskTypeSATA,
			Rotational: false,
			Mounted:    true,
			MountPoint: "/mnt/backup",
			IsSystem:   false, // Mounted elsewhere
		},
	}
	defaultOps.SetMockDisks(mockDisks)

	ctx := context.Background()
	available, err := engine.ListAvailableDisks(ctx)
	if err != nil {
		t.Fatalf("ListAvailableDisks failed: %v", err)
	}

	// Only /dev/sda should be available (sdb is system root, sdc is mounted)
	if len(available) != 1 {
		t.Fatalf("expected 1 available disk, got %d", len(available))
	}
	if available[0].Path != "/dev/sda" {
		t.Errorf("expected available disk /dev/sda, got %s", available[0].Path)
	}

	// Case 2: No unmounted disks available returns empty slice
	defaultOps.SetMockDisks([]DiskInfo{mockDisks[1], mockDisks[2]})
	emptyDisks, err := engine.ListAvailableDisks(ctx)
	if err != nil {
		t.Fatalf("ListAvailableDisks failed: %v", err)
	}
	if len(emptyDisks) != 0 {
		t.Errorf("expected empty list, got %d disks", len(emptyDisks))
	}
}

func TestStorageEngine_InitializeStorage_SuccessAndProtection(t *testing.T) {
	engine, dataDir, db := setupTestStorageEngine(t)
	defer engine.Close()

	defaultOps := engine.DiskOps().(*DefaultDiskOps)
	defaultOps.SetMockDisks([]DiskInfo{
		{
			Name:     "nvme0n1",
			Path:     "/dev/nvme0n1",
			IsSystem: false,
			Mounted:  false,
		},
		{
			Name:       "sda",
			Path:       "/dev/sda",
			IsSystem:   true,
			Mounted:    true,
			MountPoint: "/",
		},
	})

	ctx := context.Background()

	// Boundary test: Empty device path
	_, err := engine.InitializeStorage(ctx, "")
	if err != ErrInvalidDevice {
		t.Errorf("expected ErrInvalidDevice, got %v", err)
	}

	// Boundary test: Non /dev path
	_, err = engine.InitializeStorage(ctx, "sda")
	if err != ErrInvalidDevice {
		t.Errorf("expected ErrInvalidDevice, got %v", err)
	}

	// Boundary test: Never format root or boot partitions
	_, err = engine.InitializeStorage(ctx, "/dev/sda")
	if err != ErrSystemDeviceProtected {
		t.Errorf("expected ErrSystemDeviceProtected, got %v", err)
	}

	// Success test: Format valid unmounted disk
	res, err := engine.InitializeStorage(ctx, "/dev/nvme0n1")
	if err != nil {
		t.Fatalf("InitializeStorage failed: %v", err)
	}

	if res.Status != "initialized" || res.Device != "/dev/nvme0n1" || res.MountPoint != dataDir {
		t.Errorf("unexpected initialize response: %+v", res)
	}

	// Verify unified layout exists
	for _, sub := range RequiredDataSubdirectories {
		p := filepath.Join(dataDir, sub)
		info, err := filepath.Glob(p)
		if err != nil || len(info) == 0 {
			t.Errorf("expected data subdirectory %q to exist", p)
		}
	}

	// Verify persistence in SQLite
	val, err := db.GetSetting(ctx, SettingsKeyLocalPool)
	if err != nil || val == "" {
		t.Errorf("expected local pool config in database, got: %q (err=%v)", val, err)
	}

	// Verify telemetry status
	status := engine.GetStorageStatus(ctx)
	if status.Status != StatusLevelOK || status.PoolType != PoolTypeLocal {
		t.Errorf("expected status ok/local, got status=%s poolType=%s", status.Status, status.PoolType)
	}
	if status.LocalPool == nil || status.LocalPool.Device != "/dev/nvme0n1" {
		t.Errorf("expected local pool /dev/nvme0n1 in status, got %+v", status.LocalPool)
	}
}

func TestStorageEngine_NASTestCredentials(t *testing.T) {
	engine, _, _ := setupTestStorageEngine(t)
	defer engine.Close()

	ctx := context.Background()
	defaultNASOps := engine.NASOps().(*DefaultNASOps)

	// Valid test credentials
	testRes, err := engine.TestNAS(ctx, NASTestRequest{
		Type:     "smb",
		Host:     "192.168.1.100",
		Share:    "media",
		Username: "validuser",
		Password: "validpassword",
	})
	if err != nil {
		t.Fatalf("expected test success, got error: %v", err)
	}
	if !testRes.Success {
		t.Errorf("expected test success = true, got %v", testRes.Success)
	}

	// Invalid type
	_, err = engine.TestNAS(ctx, NASTestRequest{
		Type:  "ftp",
		Host:  "192.168.1.100",
		Share: "media",
	})
	if err != ErrInvalidNASConfig {
		t.Errorf("expected ErrInvalidNASConfig, got %v", err)
	}

	// Unreachable host
	defaultNASOps.SetMockReachability(false)
	_, err = engine.TestNAS(ctx, NASTestRequest{
		Type:  "smb",
		Host:  "unreachable.nas.local",
		Share: "media",
	})
	if err != ErrNASUnreachable {
		t.Errorf("expected ErrNASUnreachable, got %v", err)
	}
	defaultNASOps.SetMockReachability(true)

	// Auth failure
	defaultNASOps.SetMockAuthSuccess(false)
	_, err = engine.TestNAS(ctx, NASTestRequest{
		Type:     "smb",
		Host:     "192.168.1.100",
		Share:    "media",
		Username: "baduser",
		Password: "badpass",
	})
	if err != ErrNASAuthFailed {
		t.Errorf("expected ErrNASAuthFailed, got %v", err)
	}
}

func TestStorageEngine_NASMount_PersistenceAndRedaction(t *testing.T) {
	engine, dataDir, db := setupTestStorageEngine(t)
	defer engine.Close()

	ctx := context.Background()

	mountRes, err := engine.MountNAS(ctx, NASMountRequest{
		Type:     "smb",
		Host:     "nas.local",
		Share:    "household_media",
		Username: "family",
		Password: "supersecretpassword",
	})
	if err != nil {
		t.Fatalf("MountNAS failed: %v", err)
	}

	if mountRes.Status != "mounted" || mountRes.Type != "smb" || mountRes.Host != "nas.local" {
		t.Errorf("unexpected mount response: %+v", mountRes)
	}

	expectedMountPoint := filepath.Join(dataDir, "media")
	if mountRes.MountPoint != expectedMountPoint {
		t.Errorf("expected mount point %q, got %q", expectedMountPoint, mountRes.MountPoint)
	}

	// Invariant test: Sensitive credentials in SQLite settings store
	savedCfg, err := LoadNASConfig(ctx, db)
	if err != nil || savedCfg == nil {
		t.Fatalf("failed to load saved NAS config from DB: %v", err)
	}
	if savedCfg.Password != "supersecretpassword" {
		t.Errorf("expected saved password in SQLite, got %q", savedCfg.Password)
	}

	// Invariant test: Password is NEVER returned in plaintext in GET /api/v1/storage/status
	status := engine.GetStorageStatus(ctx)
	if status.NAS == nil {
		t.Fatalf("expected NAS in status response, got nil")
	}
	if status.NAS.Password != "***" {
		t.Errorf("expected redacted password '***', got %q", status.NAS.Password)
	}
	if status.NAS.Status != NASStatusConnected {
		t.Errorf("expected NAS status 'connected', got %q", status.NAS.Status)
	}
	if status.Status != StatusLevelOK {
		t.Errorf("expected overall status 'ok', got %q", status.Status)
	}
}

func TestStorageEngine_NASDisconnection_AmberDegraded(t *testing.T) {
	engine, _, _ := setupTestStorageEngine(t)
	defer engine.Close()

	ctx := context.Background()
	defaultNASOps := engine.NASOps().(*DefaultNASOps)

	// Mount NAS
	_, err := engine.MountNAS(ctx, NASMountRequest{
		Type:     "nfs",
		Host:     "192.168.1.50",
		Share:    "/export/videos",
	})
	if err != nil {
		t.Fatalf("MountNAS failed: %v", err)
	}

	// Initial status is healthy connected
	initialStatus := engine.GetStorageStatus(ctx)
	if initialStatus.Status != StatusLevelOK || initialStatus.NAS.Status != NASStatusConnected {
		t.Fatalf("expected initial connected state, got: %+v", initialStatus)
	}

	// Simulate NAS network drop / unreachable
	defaultNASOps.SetMockReachability(false)

	// Run health probe
	probe := engine.Probe()
	degradedStatus := probe.ProbeOnce(ctx)

	// Invariant: Status must transition to degraded (Amber)
	if degradedStatus.Status != StatusLevelDegraded {
		t.Errorf("expected overall status 'degraded', got %q", degradedStatus.Status)
	}
	if degradedStatus.NAS.Status != NASStatusDegraded {
		t.Errorf("expected NAS status 'degraded', got %q", degradedStatus.NAS.Status)
	}
	// Invariant: Consumer-friendly copy ("Network storage temporarily unavailable")
	if degradedStatus.NAS.Message != ConsumerNASDegradedMsg {
		t.Errorf("expected consumer-friendly message %q, got %q", ConsumerNASDegradedMsg, degradedStatus.NAS.Message)
	}

	// Invariant: Automatically transitions back to connected (Green) when network recovers
	defaultNASOps.SetMockReachability(true)
	recoveredStatus := probe.ProbeOnce(ctx)
	if recoveredStatus.Status != StatusLevelOK {
		t.Errorf("expected recovered overall status 'ok', got %q", recoveredStatus.Status)
	}
	if recoveredStatus.NAS.Status != NASStatusConnected {
		t.Errorf("expected recovered NAS status 'connected', got %q", recoveredStatus.NAS.Status)
	}
}

func TestStorageEngine_PeriodicProbeLoop_NeverCrashes(t *testing.T) {
	engine, _, _ := setupTestStorageEngine(t)
	defer engine.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defaultNASOps := engine.NASOps().(*DefaultNASOps)

	// Mount NAS
	_, err := engine.MountNAS(ctx, NASMountRequest{
		Type:  "smb",
		Host:  "flaky.nas.local",
		Share: "data",
	})
	if err != nil {
		t.Fatalf("MountNAS failed: %v", err)
	}

	// Fast ticker for test
	engine.Probe().SetInterval(20 * time.Millisecond)
	engine.StartProbing(ctx)

	// Toggle reachability back and forth rapidly to ensure no race conditions or deadlocks
	for i := 0; i < 5; i++ {
		defaultNASOps.SetMockReachability(false)
		time.Sleep(30 * time.Millisecond)
		defaultNASOps.SetMockReachability(true)
		time.Sleep(30 * time.Millisecond)
	}

	status := engine.GetStorageStatus(ctx)
	if status.NAS == nil {
		t.Errorf("expected active NAS status, got nil")
	}
}
