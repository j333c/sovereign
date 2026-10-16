package optical

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseBlkidOutput(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedType  DiscType
		expectedLabel string
		expectedID    string
		expectedEnc   bool
	}{
		{
			name: "ISO9660 DVD with UUID and Label",
			output: `DEVNAME=/dev/sr0
LABEL=FAMILY_VACATION
UUID=2024-05-10-14-30-00-00
TYPE=iso9660
`,
			expectedType:  DiscTypeDVD,
			expectedLabel: "FAMILY_VACATION",
			expectedID:    "2024-05-10-14-30-00-00",
			expectedEnc:   false,
		},
		{
			name: "UDF Commercial Blu-ray with AACS",
			output: `DEVNAME=/dev/sr0
LABEL=THE_MATRIX
UUID=A1B2C3D4E5F6
TYPE=udf
SEC_TYPE=aacs
`,
			expectedType:  DiscTypeBluray,
			expectedLabel: "THE_MATRIX",
			expectedID:    "A1B2C3D4E5F6",
			expectedEnc:   true,
		},
		{
			name: "UDF Blu-ray with BD designation",
			output: `DEVNAME=/dev/sr0
LABEL=INCEPTION_BD
UUID=112233445566
TYPE=udf
`,
			expectedType:  DiscTypeBluray,
			expectedLabel: "INCEPTION_BD",
			expectedID:    "112233445566",
			expectedEnc:   true, // "BD" in upperOutput triggers Encrypted
		},
		{
			name: "UDF Blu-ray with explicit ENCRYPTED=1",
			output: `DEVNAME=/dev/sr0
LABEL=INTERSTELLAR
UUID=998877665544
TYPE=udf
ENCRYPTED=1
`,
			expectedType:  DiscTypeBluray,
			expectedLabel: "INTERSTELLAR",
			expectedID:    "998877665544",
			expectedEnc:   true,
		},
		{
			name: "Audio CD without label or UUID",
			output: `DEVNAME=/dev/sr0
TYPE=unknown
`,
			expectedType:  DiscTypeCD,
			expectedLabel: "",
			expectedID:    "audio_cd",
			expectedEnc:   false,
		},
		{
			name: "ISO9660 DVD without UUID falls back to normalized label ID",
			output: `DEVNAME=/dev/sr0
LABEL=SUMMER ROADTRIP
TYPE=iso9660
`,
			expectedType:  DiscTypeDVD,
			expectedLabel: "SUMMER ROADTRIP",
			expectedID:    "summer_roadtrip",
			expectedEnc:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			disc := parseBlkidOutput(tc.output)
			if disc.DiscType != tc.expectedType {
				t.Errorf("expected DiscType %q, got %q", tc.expectedType, disc.DiscType)
			}
			if disc.Label != tc.expectedLabel {
				t.Errorf("expected Label %q, got %q", tc.expectedLabel, disc.Label)
			}
			if disc.ID != tc.expectedID {
				t.Errorf("expected ID %q, got %q", tc.expectedID, disc.ID)
			}
			if disc.Encrypted != tc.expectedEnc {
				t.Errorf("expected Encrypted %v, got %v", tc.expectedEnc, disc.Encrypted)
			}
		})
	}
}

func TestLinuxOpticalDriver_DevicePathValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	invalidPaths := []string{
		"/dev/sda",
		"/dev/sr",
		"/dev/sr0; rm -rf /",
		"/dev/null",
		"../../dev/sr0",
		"/dev/sra",
	}

	for _, path := range invalidPaths {
		t.Run("invalid_path_"+path, func(t *testing.T) {
			driver := NewLinuxOpticalDriver(path, logger)

			// GetDriveStatus
			st, err := driver.GetDriveStatus(ctx)
			if err == nil {
				t.Errorf("expected error from GetDriveStatus for %q, got nil (status=%+v)", path, st)
			}

			// Eject
			if err := driver.Eject(ctx); err == nil {
				t.Errorf("expected error from Eject for %q, got nil", path)
			}

			// CloseTray
			if err := driver.CloseTray(ctx); err == nil {
				t.Errorf("expected error from CloseTray for %q, got nil", path)
			}

			// RipDisc
			tempDir := t.TempDir()
			disc := DiscInfo{DiscType: DiscTypeDVD, Label: "TEST", ID: "test"}
			if err := driver.RipDisc(ctx, disc, tempDir, nil); err == nil {
				t.Errorf("expected error from RipDisc for %q, got nil", path)
			}
		})
	}

	// Test default device path when empty
	defaultDriver := NewLinuxOpticalDriver("", logger)
	if defaultDriver.devicePath != "/dev/sr0" {
		t.Errorf("expected default device path /dev/sr0, got %q", defaultDriver.devicePath)
	}

	// Valid path pattern matching test
	validPaths := []string{
		"/dev/sr0",
		"/dev/sr1",
		"/dev/sr12",
	}

	for _, path := range validPaths {
		if !srDeviceRegex.MatchString(path) {
			t.Errorf("expected %q to match srDeviceRegex", path)
		}
	}
}

func TestMockOpticalDriver_RipAndEject(t *testing.T) {
	mock := NewMockOpticalDriver()
	ctx := context.Background()

	// 1. Initial status
	st, err := mock.GetDriveStatus(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.DriveReady || st.TrayOpen || st.Disc.Inserted {
		t.Fatalf("unexpected initial state: %+v", st)
	}

	// 2. CD Rip produces track01.cdda
	cdDir := t.TempDir()
	cdDisc := DiscInfo{
		DiscType: DiscTypeCD,
		Label:    "Jazz Album",
		ID:       "jazz_album",
	}
	mock.InsertDisc(cdDisc)

	var recordedProgress []float64
	err = mock.RipDisc(ctx, cdDisc, cdDir, func(pct float64) {
		recordedProgress = append(recordedProgress, pct)
	})
	if err != nil {
		t.Fatalf("RipDisc failed: %v", err)
	}
	if len(recordedProgress) == 0 || recordedProgress[len(recordedProgress)-1] != 100.0 {
		t.Errorf("expected progress reaching 100%%, got %v", recordedProgress)
	}

	cddaFile := filepath.Join(cdDir, "track01.cdda")
	if _, err := os.Stat(cddaFile); err != nil {
		t.Errorf("expected %s to be created for CD rip: %v", cddaFile, err)
	}

	// 3. DVD Rip produces disc.iso
	dvdDir := t.TempDir()
	dvdDisc := DiscInfo{
		DiscType: DiscTypeDVD,
		Label:    "Action Movie",
		ID:       "action_movie",
	}
	err = mock.RipDisc(ctx, dvdDisc, dvdDir, nil)
	if err != nil {
		t.Fatalf("RipDisc failed: %v", err)
	}
	isoFile := filepath.Join(dvdDir, "disc.iso")
	if _, err := os.Stat(isoFile); err != nil {
		t.Errorf("expected %s to be created for DVD rip: %v", isoFile, err)
	}

	// 4. Rip failure simulation
	mock.SetRipFailures(2, nil)
	failDir := t.TempDir()
	if err := mock.RipDisc(ctx, dvdDisc, failDir, nil); err == nil {
		t.Errorf("expected simulated rip failure 1, got nil")
	}
	if err := mock.RipDisc(ctx, dvdDisc, failDir, nil); err == nil {
		t.Errorf("expected simulated rip failure 2, got nil")
	}
	if err := mock.RipDisc(ctx, dvdDisc, failDir, nil); err != nil {
		t.Errorf("expected 3rd attempt to succeed, got %v", err)
	}

	// 5. Eject and Close tray
	if err := mock.Eject(ctx); err != nil {
		t.Fatalf("Eject failed: %v", err)
	}
	if mock.EjectCount() != 1 {
		t.Errorf("expected eject count 1, got %d", mock.EjectCount())
	}
	st, _ = mock.GetDriveStatus(ctx)
	if !st.TrayOpen || st.Disc.Inserted {
		t.Errorf("expected tray open and disc removed after eject: %+v", st)
	}

	if err := mock.CloseTray(ctx); err != nil {
		t.Fatalf("CloseTray failed: %v", err)
	}
	st, _ = mock.GetDriveStatus(ctx)
	if st.TrayOpen {
		t.Errorf("expected tray closed: %+v", st)
	}
}

func TestMockOpticalDriver_ContextCancellation(t *testing.T) {
	mock := NewMockOpticalDriver()
	mock.SetRipDelay(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	disc := DiscInfo{DiscType: DiscTypeDVD, Label: "SlowDisc", ID: "slow"}
	destDir := t.TempDir()

	err := mock.RipDisc(ctx, disc, destDir, nil)
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
}
