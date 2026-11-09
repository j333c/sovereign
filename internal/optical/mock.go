package optical

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MockOpticalDriver simulates an optical drive for deterministic testing without physical hardware.
type MockOpticalDriver struct {
	mu          sync.RWMutex
	status      DriveStatus
	ejectCount  int
	closeCount  int
	ripFailures int
	ripErr      error
	ripDelay    time.Duration
	onRip       func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error
	onEject     func(ctx context.Context) error
}

// NewMockOpticalDriver initializes a mock optical drive in ready, tray-closed, empty state.
func NewMockOpticalDriver() *MockOpticalDriver {
	return &MockOpticalDriver{
		status: DriveStatus{
			DevicePath: "/dev/sr0",
			DriveReady: true,
			TrayOpen:   false,
			Disc: DiscInfo{
				Inserted: false,
				DiscType: DiscTypeNone,
			},
		},
	}
}

// GetDriveStatus returns the simulated drive status.
func (m *MockOpticalDriver) GetDriveStatus(ctx context.Context) (DriveStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, nil
}

// SetDriveStatus sets the entire simulated drive status.
func (m *MockOpticalDriver) SetDriveStatus(st DriveStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = st
}

// InsertDisc simulates disc insertion into the tray.
func (m *MockOpticalDriver) InsertDisc(disc DiscInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	disc.Inserted = true
	m.status.Disc = disc
	m.status.TrayOpen = false
	m.status.DriveReady = true
}

// RemoveDisc clears any inserted disc.
func (m *MockOpticalDriver) RemoveDisc() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Disc = DiscInfo{
		Inserted: false,
		DiscType: DiscTypeNone,
	}
}

// Eject simulates tray opening and disc ejection.
func (m *MockOpticalDriver) Eject(ctx context.Context) error {
	m.mu.Lock()
	m.ejectCount++
	m.status.TrayOpen = true
	m.status.Disc = DiscInfo{Inserted: false, DiscType: DiscTypeNone}
	onEject := m.onEject
	m.mu.Unlock()

	if onEject != nil {
		return onEject(ctx)
	}
	return nil
}

// CloseTray simulates closing the drive tray.
func (m *MockOpticalDriver) CloseTray(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCount++
	m.status.TrayOpen = false
	return nil
}

// EjectCount returns the number of times Eject was called.
func (m *MockOpticalDriver) EjectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ejectCount
}

// SetRipFailures configures how many consecutive times RipDisc will return an error before succeeding.
func (m *MockOpticalDriver) SetRipFailures(count int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ripFailures = count
	m.ripErr = err
}

// SetRipDelay sets an optional simulated delay per rip.
func (m *MockOpticalDriver) SetRipDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ripDelay = delay
}

// SetOnRip sets a custom rip execution hook.
func (m *MockOpticalDriver) SetOnRip(fn func(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRip = fn
}

// SetOnEject sets a custom eject execution hook.
func (m *MockOpticalDriver) SetOnEject(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEject = fn
}

// RipDisc performs simulated ripping with progress callbacks and retry simulation.
func (m *MockOpticalDriver) RipDisc(ctx context.Context, disc DiscInfo, destDir string, progressCb func(percent float64)) error {
	m.mu.Lock()
	if m.ripFailures > 0 {
		m.ripFailures--
		err := m.ripErr
		if err == nil {
			err = errors.New("read scratch error at sector 1024")
		}
		m.mu.Unlock()
		return err
	}
	customRip := m.onRip
	delay := m.ripDelay
	m.mu.Unlock()

	if customRip != nil {
		return customRip(ctx, disc, destDir, progressCb)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	steps := 5
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if delay > 0 {
			time.Sleep(delay / time.Duration(steps))
		}
		if progressCb != nil {
			progressCb(float64(i) * 100.0 / float64(steps))
		}
	}

	fileName := "disc.iso"
	if disc.DiscType == DiscTypeCD {
		fileName = "track01.cdda"
	}
	content := []byte(fmt.Sprintf("MOCK RIP: %s (Type: %s, ID: %s)", disc.Label, disc.DiscType, disc.ID))
	return os.WriteFile(filepath.Join(destDir, fileName), content, 0644)
}
