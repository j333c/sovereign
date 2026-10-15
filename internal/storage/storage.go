// Package storage manages the single POSIX filesystem hierarchy under /data,
// staged file dropzones, atomic media moves, and media library synchronization.
//
// Invariants & Boundaries:
// - Storage Hierarchy: Root partition under /data with dedicated subdirectories:
//   /data/media, /data/documents, /data/models, /data/state, /data/drop.
// - Dropzone Staging: Uploads stream to /data/drop/incoming/:category/.uploading-:uuid-:filename.
// - Atomic Promotion: Upon complete upload and fsync(), files are validated for magic bytes
//   and atomically moved (os.Rename) into /data/media/:category.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// RequiredDataSubdirectories defines the mandatory subdirectories of the /data hierarchy.
var RequiredDataSubdirectories = []string{
	"media",
	"documents",
	"models",
	"state",
	"drop",
}

// EnsureDataLayout verifies and recursively creates missing subdirectories under the given data root.
func EnsureDataLayout(dataRoot string) error {
	if dataRoot == "" {
		return fmt.Errorf("data root directory path cannot be empty")
	}

	for _, sub := range RequiredDataSubdirectories {
		dir := filepath.Join(dataRoot, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory %q: %w", dir, err)
		}
	}

	return nil
}
