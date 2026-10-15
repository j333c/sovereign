package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDataLayout(t *testing.T) {
	tempDir := t.TempDir()
	dataRoot := filepath.Join(tempDir, "data")

	if err := EnsureDataLayout(dataRoot); err != nil {
		t.Fatalf("EnsureDataLayout failed: %v", err)
	}

	for _, sub := range RequiredDataSubdirectories {
		subPath := filepath.Join(dataRoot, sub)
		info, err := os.Stat(subPath)
		if err != nil {
			t.Errorf("expected directory %q to exist, got error: %v", subPath, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", subPath)
		}
	}
}

func TestEnsureDataLayout_EmptyPath(t *testing.T) {
	if err := EnsureDataLayout(""); err == nil {
		t.Fatalf("expected error for empty data root, got nil")
	}
}
