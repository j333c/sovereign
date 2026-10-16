package optical

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupTestRipper(t *testing.T) (*OpticalRipper, *MockOpticalDriver, *KEYDBManager, string) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	keysPath := filepath.Join(dataDir, "state", "keys", "KEYDB.cfg")
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs")

	driver := NewMockOpticalDriver()
	keyMgr := NewKEYDBManager(keysPath, nil)

	ripper := NewOpticalRipper(RipperConfig{
		DataDir:         dataDir,
		StagingDir:      stagingDir,
		Driver:          driver,
		KeyMgr:          keyMgr,
		PollingInterval: 20 * time.Millisecond,
		RetryDelay:      10 * time.Millisecond,
		EjectResetDelay: -1,
		HeadroomChecker: func(path string) (float64, error) {
			return 0.50, nil // 50% free space by default
		},
	})

	return ripper, driver, keyMgr, dataDir
}

// TestOpticalRipper_InitialState verifies default idle status.
func TestOpticalRipper_InitialState(t *testing.T) {
	ripper, _, _, _ := setupTestRipper(t)
	st := ripper.GetStatus()

	if st.State != StateIdle {
		t.Errorf("expected state %q, got %q", StateIdle, st.State)
	}
	if st.StatusText != StatusTextIdle {
		t.Errorf("expected status text %q, got %q", StatusTextIdle, st.StatusText)
	}
	if st.DiscPresent {
		t.Errorf("expected DiscPresent to be false")
	}
	if st.KeysActive {
		t.Errorf("expected KeysActive to be false initially")
	}
}

// TestOpticalRipper_UnencryptedDVD_AutomaticRip verifies unencrypted DVD insertion,
// ripping progress, staging, and tray auto-ejection.
func TestOpticalRipper_UnencryptedDVD_AutomaticRip(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	var eventsMu sync.Mutex
	var events []string
	ripper.SetEventBroadcaster(func(event string, data any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "HOME_MOVIES_2024",
		ID:        "home_movies_2024",
		Encrypted: false,
		SizeBytes: 1024,
	}

	driver.InsertDisc(disc)

	// Poll disc
	ripper.PollOnce(ctx)

	// Verify driver eject was called upon completion
	if driver.EjectCount() != 1 {
		t.Errorf("expected tray to auto-eject once, got %d", driver.EjectCount())
	}

	// Verify staged disc file exists under /data/media/staging/discs/:disc_id/
	expectedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "disc.iso")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected staged file %q to exist: %v", expectedFile, err)
	}

	// Verify emitted SSE events
	eventsMu.Lock()
	defer eventsMu.Unlock()

	hasDetected := false
	hasProgress := false
	hasEjected := false
	for _, e := range events {
		if e == "optical.disc_detected" {
			hasDetected = true
		}
		if e == "optical.rip_progress" {
			hasProgress = true
		}
		if e == "optical.disc_ejected" {
			hasEjected = true
		}
	}

	if !hasDetected {
		t.Errorf("expected optical.disc_detected event")
	}
	if !hasProgress {
		t.Errorf("expected optical.rip_progress event")
	}
	if !hasEjected {
		t.Errorf("expected optical.disc_ejected event")
	}
}

// TestOpticalRipper_AudioCD_AutomaticRip verifies audio CD automatic ripping to staging.
func TestOpticalRipper_AudioCD_AutomaticRip(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	disc := DiscInfo{
		DiscType:  DiscTypeCD,
		Label:     "ABBA_GOLD",
		ID:        "abba_gold",
		Encrypted: false,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	if driver.EjectCount() != 1 {
		t.Errorf("expected tray to auto-eject once for CD, got %d", driver.EjectCount())
	}

	expectedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "track01.cdda")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected audio track %q to exist: %v", expectedFile, err)
	}
}

// TestOpticalRipper_StagingUniqueHashing verifies that re-ripping a disc with the same ID
// does not overwrite existing staging directories and creates a unique directory suffix.
func TestOpticalRipper_StagingUniqueHashing(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)
	ctx := context.Background()

	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "FAMILY_VACATION",
		ID:        "vacation_dvd",
		Encrypted: false,
	}

	// Pre-create existing staging directory with a file
	existingDir := filepath.Join(dataDir, "media", "staging", "discs", disc.ID)
	_ = os.MkdirAll(existingDir, 0755)
	_ = os.WriteFile(filepath.Join(existingDir, "prior.txt"), []byte("prior content"), 0644)

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	// Ensure prior file was untouched
	if _, err := os.Stat(filepath.Join(existingDir, "prior.txt")); err != nil {
		t.Errorf("prior content was destroyed: %v", err)
	}

	// Find the unique sibling directory created with a timestamp suffix
	entries, err := os.ReadDir(filepath.Join(dataDir, "media", "staging", "discs"))
	if err != nil {
		t.Fatalf("failed to read staging dir: %v", err)
	}

	foundHashed := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), disc.ID+"-") {
			foundHashed = true
			break
		}
	}

	if !foundHashed {
		t.Errorf("expected uniquely hashed staging directory starting with %s-, found entries: %+v", disc.ID, entries)
	}
}

// TestOpticalRipper_EncryptedBluray_NoKeys verifies that commercial encrypted Blu-rays safely auto-eject
// with consumer microcopy when KEYDB.cfg is absent.
func TestOpticalRipper_EncryptedBluray_NoKeys(t *testing.T) {
	ripper, driver, _, _ := setupTestRipper(t)

	var alertEmitted bool
	var alertNotice string
	ripper.SetEventBroadcaster(func(event string, data any) {
		if event == "optical.alert" {
			alertEmitted = true
			if m, ok := data.(map[string]any); ok {
				alertNotice = fmt.Sprintf("%v", m["notice"])
			}
		}
	})

	ctx := context.Background()
	disc := DiscInfo{
		DiscType:  DiscTypeBluray,
		Label:     "INCEPTION_BD",
		ID:        "inception_bd",
		Encrypted: true,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	// Verify tray was automatically ejected
	if driver.EjectCount() != 1 {
		t.Errorf("expected tray to auto-eject on missing keys, got %d", driver.EjectCount())
	}

	// Verify alert was emitted
	if !alertEmitted {
		t.Errorf("expected optical.alert event for missing keys")
	}

	// Verify exact microcopy specified in spec:
	// "This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."
	expectedMicrocopy := "This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."
	if alertNotice != expectedMicrocopy {
		t.Errorf("expected microcopy %q, got %q", expectedMicrocopy, alertNotice)
	}
}

// TestOpticalRipper_EncryptedBluray_WithKeys verifies that commercial encrypted Blu-rays
// proceed with ripping when valid KEYDB.cfg keys are available.
func TestOpticalRipper_EncryptedBluray_WithKeys(t *testing.T) {
	ripper, driver, keyMgr, dataDir := setupTestRipper(t)

	// Upload valid KEYDB.cfg content with disc entry
	keydbContent := []byte(`
; KEYDB.cfg for testing
0x11223344556677889900AABBCCDDEEFF = Inception | V | 0x00112233445566778899AABBCCDDEEFF
`)
	if _, err := keyMgr.SaveKeys(keydbContent); err != nil {
		t.Fatalf("failed to save keys: %v", err)
	}

	ctx := context.Background()
	disc := DiscInfo{
		DiscType:  DiscTypeBluray,
		Label:     "INCEPTION",
		ID:        "11223344556677889900aabbccddeeff",
		Encrypted: true,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	if driver.EjectCount() != 1 {
		t.Errorf("expected tray to auto-eject after successful rip, got %d", driver.EjectCount())
	}

	stagedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "disc.iso")
	if _, err := os.Stat(stagedFile); err != nil {
		t.Errorf("expected staged Blu-ray file %q: %v", stagedFile, err)
	}
}

// TestOpticalRipper_StorageHeadroomGuard verifies that ripping pauses and auto-ejects
// when free storage capacity drops below the 10% safety floor.
func TestOpticalRipper_StorageHeadroomGuard(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	keysPath := filepath.Join(dataDir, "state", "keys", "KEYDB.cfg")
	stagingDir := filepath.Join(dataDir, "media", "staging", "discs")

	driver := NewMockOpticalDriver()
	keyMgr := NewKEYDBManager(keysPath, nil)

	var alertType string
	ripper := NewOpticalRipper(RipperConfig{
		DataDir:         dataDir,
		StagingDir:      stagingDir,
		Driver:          driver,
		KeyMgr:          keyMgr,
		PollingInterval: 20 * time.Millisecond,
		EjectResetDelay: -1,
		EventBroadcaster: func(event string, data any) {
			if event == "optical.alert" {
				if m, ok := data.(map[string]any); ok {
					alertType = fmt.Sprintf("%v", m["type"])
				}
			}
		},
		HeadroomChecker: func(path string) (float64, error) {
			return 0.08, nil // 8% free space (< 10% safety floor)
		},
	})

	ctx := context.Background()
	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "TEST_DVD",
		ID:        "test_dvd",
		Encrypted: false,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	// Verify tray was auto-ejected to protect system WAL databases
	if driver.EjectCount() != 1 {
		t.Errorf("expected tray to auto-eject when headroom < 10%%, got %d", driver.EjectCount())
	}

	if alertType != "storage_low" {
		t.Errorf("expected alert type storage_low, got %q", alertType)
	}

	// Verify no rip files were staged
	stagedFile := filepath.Join(stagingDir, disc.ID, "disc.iso")
	if _, err := os.Stat(stagedFile); !os.IsNotExist(err) {
		t.Errorf("file should not have been staged during storage headroom violation: %v", err)
	}
}

// TestOpticalRipper_ReadRetries_TransientErrorSuccess verifies that transient read errors
// are retried up to 3 times, succeeding on attempt 2 or 3.
func TestOpticalRipper_ReadRetries_TransientErrorSuccess(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	// Fail 2 times with scratch error, then succeed on 3rd attempt
	driver.SetRipFailures(2, errors.New("read scratch sector 4096"))

	ctx := context.Background()
	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "RECOVERABLE_DISC",
		ID:        "recoverable_disc",
		Encrypted: false,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	// Should succeed on 3rd attempt and auto-eject
	if driver.EjectCount() != 1 {
		t.Errorf("expected auto-eject after successful retry, got %d", driver.EjectCount())
	}

	stagedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "disc.iso")
	if _, err := os.Stat(stagedFile); err != nil {
		t.Errorf("expected staged file after retrying: %v", err)
	}
}

// TestOpticalRipper_ReadRetries_HardFailure verifies that hard unreadable discs fail safely
// after 3 retries, eject the tray, and emit an alert event.
func TestOpticalRipper_ReadRetries_HardFailure(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	// Permanently fail (more than 3 times)
	driver.SetRipFailures(5, errors.New("permanent laser read error"))

	var readFailedEvent bool
	ripper.SetEventBroadcaster(func(event string, data any) {
		if event == "optical.read_failed" {
			readFailedEvent = true
		}
	})

	ctx := context.Background()
	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "SCRATCHED_DISC",
		ID:        "scratched_disc",
		Encrypted: false,
	}

	driver.InsertDisc(disc)
	ripper.PollOnce(ctx)

	// Ejected after failing 3 retries
	if driver.EjectCount() != 1 {
		t.Errorf("expected auto-eject after 3 failed retries, got %d", driver.EjectCount())
	}

	if !readFailedEvent {
		t.Errorf("expected optical.read_failed event to be broadcast")
	}

	// Verify no partial corrupt staging file was left finalized
	stagedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "disc.iso")
	if _, err := os.Stat(stagedFile); !os.IsNotExist(err) {
		t.Errorf("corrupt rip should not be finalized: %v", err)
	}
}

// TestOpticalRipper_ManualEject verifies that the Eject method opens the tray and returns to idle.
func TestOpticalRipper_ManualEject(t *testing.T) {
	ripper, driver, _, _ := setupTestRipper(t)

	ctx := context.Background()
	if err := ripper.Eject(ctx); err != nil {
		t.Fatalf("unexpected error during manual eject: %v", err)
	}

	if driver.EjectCount() != 1 {
		t.Errorf("expected driver.Eject to be called once, got %d", driver.EjectCount())
	}

	st := ripper.GetStatus()
	if st.State != StateIdle {
		t.Errorf("expected state %q after manual eject, got %q", StateIdle, st.State)
	}
	if st.StatusText != StatusTextIdle {
		t.Errorf("expected status text %q, got %q", StatusTextIdle, st.StatusText)
	}
}

// TestKEYDBManager_ValidationAndStorage tests validation, error cases, and atomic saving of KEYDB.cfg.
func TestKEYDBManager_ValidationAndStorage(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "state", "keys", "KEYDB.cfg")
	mgr := NewKEYDBManager(keyPath, nil)

	// Case 1: Empty file rejected
	if err := mgr.Validate([]byte("   \n\t  ")); !errors.Is(err, ErrEmptyKeyFile) {
		t.Errorf("expected ErrEmptyKeyFile, got %v", err)
	}

	// Case 2: Binary content rejected
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF}
	if err := mgr.Validate(binaryData); !errors.Is(err, ErrBinaryContent) {
		t.Errorf("expected ErrBinaryContent, got %v", err)
	}

	// Case 3: Invalid text format without keys rejected
	randomText := []byte("This is just a random text file with no keys.")
	if err := mgr.Validate(randomText); !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}

	// Case 4: Valid KEYDB with processing key and VUK
	validKEYDB := []byte(`
; =========================================================================
; Doom9 AACS KEYDB.cfg
; =========================================================================

PK 0x0123456789ABCDEF0123456789ABCDEF
0x2E8B9075AC5F60B3F8FE325A65FE6B0000000000 = The Matrix (1999) | V | 0x33445566778899001122334455667788
0x99887766554433221100FFEEDDCCBBAA00000000 = Inception (2010) | V | 0xAABBCCDDEEFF00112233445566778899
`)

	count, err := mgr.SaveKeys(validKEYDB)
	if err != nil {
		t.Fatalf("unexpected error saving valid keys: %v", err)
	}
	if count != 3 { // PK + Matrix + Inception
		t.Errorf("expected 3 parsed keys, got %d", count)
	}

	if !mgr.HasKeys() {
		t.Errorf("expected HasKeys() to be true")
	}

	if !mgr.HasKeyForDisc("2e8b9075ac5f60b3f8fe325a65fe6b0000000000") {
		t.Errorf("expected HasKeyForDisc to be true for The Matrix")
	}

	// Verify file was written to disk
	info := mgr.GetInfo()
	if !info.Exists || info.KeyCount != 3 {
		t.Errorf("expected info.Exists true and 3 keys, got: %+v", info)
	}
}

// TestOpticalRipper_BackgroundPollingLoop verifies that ripper.Start(ctx) continuously
// polls the driver and detects inserted media asynchronously without manual PollOnce calls.
func TestOpticalRipper_BackgroundPollingLoop(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	var detectedDiscID string
	var mu sync.Mutex
	done := make(chan struct{})

	ripper.SetEventBroadcaster(func(event string, data any) {
		if event == "optical.disc_detected" {
			mu.Lock()
			if m, ok := data.(map[string]any); ok {
				detectedDiscID = fmt.Sprintf("%v", m["id"])
			}
			mu.Unlock()
		} else if event == "optical.disc_ejected" {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ripper.Start(ctx)
	defer ripper.Close()

	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "ASYNC_MOVIE",
		ID:        "async_movie_123",
		Encrypted: false,
	}

	// Insert disc while background loop is running
	driver.InsertDisc(disc)

	select {
	case <-done:
		// Succeeded
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for background polling loop to process disc")
	}

	mu.Lock()
	id := detectedDiscID
	mu.Unlock()

	if id != disc.ID {
		t.Errorf("expected detected disc ID %q, got %q", disc.ID, id)
	}

	expectedFile := filepath.Join(dataDir, "media", "staging", "discs", disc.ID, "disc.iso")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected staged file %q: %v", expectedFile, err)
	}
}

// TestOpticalRipper_EjectCancelsActiveRip verifies that calling Eject during an in-flight
// rip cancels reading immediately without performing remaining retries.
func TestOpticalRipper_EjectCancelsActiveRip(t *testing.T) {
	ripper, driver, _, _ := setupTestRipper(t)

	ripStarted := make(chan struct{})
	driver.SetOnRip(func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error {
		select {
		case <-ripStarted:
		default:
			close(ripStarted)
		}
		// Block until cancelled
		<-ctx.Done()
		return ctx.Err()
	})

	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "LONG_RIP",
		ID:        "long_rip_disc",
		Encrypted: false,
	}
	driver.InsertDisc(disc)

	ripDone := make(chan struct{})
	go func() {
		ripper.PollOnce(context.Background())
		close(ripDone)
	}()

	select {
	case <-ripStarted:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for rip to start")
	}

	// Calling Eject should cancel in-flight rip immediately
	if err := ripper.Eject(context.Background()); err != nil {
		t.Fatalf("unexpected error during eject: %v", err)
	}

	select {
	case <-ripDone:
		// Successful cancellation
	case <-time.After(1 * time.Second):
		t.Fatalf("PollOnce did not terminate quickly after eject")
	}

	st := ripper.GetStatus()
	if st.State != StateIdle {
		t.Errorf("expected state idle after eject, got %q", st.State)
	}
}

// TestOpticalRipper_HardFailureCleansStaging verifies that when ripping fails after 3 retries,
// partial files in the staging directory are completely cleaned up.
func TestOpticalRipper_HardFailureCleansStaging(t *testing.T) {
	ripper, driver, _, dataDir := setupTestRipper(t)

	driver.SetOnRip(func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error {
		_ = os.MkdirAll(destDir, 0755)
		_ = os.WriteFile(filepath.Join(destDir, "partial.iso"), []byte("partial data"), 0644)
		return errors.New("unrecoverable laser read error")
	})

	disc := DiscInfo{
		DiscType:  DiscTypeDVD,
		Label:     "CORRUPT_DISC",
		ID:        "corrupt_disc_99",
		Encrypted: false,
	}
	driver.InsertDisc(disc)

	ripper.PollOnce(context.Background())

	stagingTarget := filepath.Join(dataDir, "media", "staging", "discs", disc.ID)
	if _, err := os.Stat(stagingTarget); !os.IsNotExist(err) {
		t.Errorf("expected partial staging directory %q to be deleted on hard failure: %v", stagingTarget, err)
	}
}

// TestKEYDBManager_LoadExistingFromDisk verifies that KEYDBManager loads existing keys
// on startup from disk.
func TestKEYDBManager_LoadExistingFromDisk(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "state", "keys", "KEYDB.cfg")
	_ = os.MkdirAll(filepath.Dir(keyPath), 0755)

	content := []byte(`
PK 0x0123456789ABCDEF0123456789ABCDEF
0xAABBCCDDEEFF00112233445566778899 = Preloaded Title | V | 0x11223344556677889900AABBCCDDEEFF
`)
	if err := os.WriteFile(keyPath, content, 0644); err != nil {
		t.Fatalf("failed to write test keydb: %v", err)
	}

	mgr := NewKEYDBManager(keyPath, nil)
	if !mgr.HasKeys() {
		t.Errorf("expected HasKeys() to be true for pre-existing keydb on disk")
	}
	if mgr.KeyCount() != 2 {
		t.Errorf("expected 2 keys loaded on startup, got %d", mgr.KeyCount())
	}
	if !mgr.HasKeyForDisc("aabbccddeeff00112233445566778899") {
		t.Errorf("expected HasKeyForDisc to be true for preloaded title")
	}
}

// TestDefaultHeadroomChecker verifies that defaultHeadroomChecker calculates free ratio
// and handles non-existent paths by checking parent directories.
func TestDefaultHeadroomChecker(t *testing.T) {
	tempDir := t.TempDir()
	nestedNonExistent := filepath.Join(tempDir, "a", "b", "c")

	ratio, err := defaultHeadroomChecker(nestedNonExistent)
	if err != nil {
		t.Fatalf("defaultHeadroomChecker failed for %s: %v", nestedNonExistent, err)
	}

	if ratio <= 0.0 || ratio > 1.0 {
		t.Errorf("expected headroom ratio between 0 and 1, got %f", ratio)
	}
}

