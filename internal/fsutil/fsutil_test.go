package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskFree(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nested", "subdir", "file.txt")

	freeBytes, err := DiskFree(nonExistent)
	if err != nil {
		t.Fatalf("DiskFree failed: %v", err)
	}
	if freeBytes <= 0 {
		t.Fatalf("expected positive free bytes, got %d", freeBytes)
	}

	ratio, err := DiskFreeRatio(nonExistent)
	if err != nil {
		t.Fatalf("DiskFreeRatio failed: %v", err)
	}
	if ratio <= 0 || ratio > 1.0 {
		t.Fatalf("expected ratio in (0, 1.0], got %f", ratio)
	}

	pct, err := DiskFreePercent(nonExistent)
	if err != nil {
		t.Fatalf("DiskFreePercent failed: %v", err)
	}
	if pct <= 0 || pct > 100.0 {
		t.Fatalf("expected pct in (0, 100.0], got %f", pct)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "sub", "config.json")
	payload := []byte(`{"status":"ok"}`)

	if err := AtomicWriteFile(target, payload, 0640); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("content mismatch: got %q, want %q", got, payload)
	}
}

func TestAtomicMoveOrCopy(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.bin")
	dest := filepath.Join(tmpDir, "dest", "target.bin")
	payload := []byte("sample-media-payload")

	if err := os.WriteFile(src, payload, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	if err := AtomicMoveOrCopy(src, dest); err != nil {
		t.Fatalf("AtomicMoveOrCopy failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed, stat err: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("content mismatch: got %q, want %q", got, payload)
	}
}
