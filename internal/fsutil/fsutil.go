// Package fsutil provides shared, crash-safe filesystem operations including
// atomic file persistence, cross-device atomic file promotion, and Statfs capacity inspection.
package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// DiskFreeFunc returns available bytes for a filesystem path.
type DiskFreeFunc func(path string) (int64, error)

func statfsExistingParent(path string) (syscall.Statfs_t, string, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	checkPath := filepath.Clean(path)
	for checkPath != "/" && checkPath != "." {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			break
		}
		checkPath = parent
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkPath, &stat); err != nil {
		return stat, checkPath, fmt.Errorf("statfs failed for %q: %w", checkPath, err)
	}
	return stat, checkPath, nil
}

// DiskFree returns the available unprivileged bytes (Bavail * Bsize) for the filesystem
// containing path. If path does not yet exist, it walks up parent directories until an
// existing directory is found.
func DiskFree(path string) (int64, error) {
	stat, _, err := statfsExistingParent(path)
	if err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// DiskFreeRatio returns the ratio [0.0, 1.0] of available blocks to total blocks (Bavail / Blocks).
func DiskFreeRatio(path string) (float64, error) {
	stat, _, err := statfsExistingParent(path)
	if err != nil {
		return 0, err
	}
	if stat.Blocks == 0 {
		return 1.0, nil
	}
	return float64(stat.Bavail) / float64(stat.Blocks), nil
}

// DiskFreePercent returns the percentage [0.0, 100.0] of available blocks to total blocks.
func DiskFreePercent(path string) (float64, error) {
	stat, _, err := statfsExistingParent(path)
	if err != nil {
		return 0, err
	}
	if stat.Blocks == 0 {
		return 0, nil
	}
	return (float64(stat.Bavail) / float64(stat.Blocks)) * 100.0, nil
}

// AtomicWriteFile writes data to a temporary file in the same directory as destPath,
// flushes it to disk via Sync(), sets permissions, closes it, and atomically renames it to destPath.
func AtomicWriteFile(destPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to ensure parent directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %q: %w", dir, err)
	}
	tmpName := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("failed to write temp file %q: %w", tmpName, err)
	}

	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to fsync temp file %q: %w", tmpName, err)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("failed to chmod temp file %q: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file %q: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to atomically rename %q to %q: %w", tmpName, destPath, err)
	}

	// Best-effort directory sync to guarantee directory entry persistence
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}

// AtomicMoveOrCopy moves srcPath to destPath atomically via os.Rename when on the same
// filesystem, or falls back to copying into a temporary file in the destination directory,
// syncing, atomically renaming to destPath, and removing srcPath across filesystem boundaries.
func AtomicMoveOrCopy(srcPath, destPath string) error {
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure destination directory: %w", err)
	}

	err := os.Rename(srcPath, destPath)
	if err == nil {
		return nil
	}

	var linkErr *os.LinkError
	isExdev := errors.As(err, &linkErr) && (errors.Is(linkErr.Err, syscall.EXDEV) || strings.Contains(strings.ToLower(linkErr.Err.Error()), "cross-device"))
	if !isExdev && !strings.Contains(strings.ToLower(err.Error()), "cross-device") {
		return err
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file for move fallback: %w", err)
	}
	defer srcFile.Close()

	tmpFile, err := os.CreateTemp(destDir, ".moving-*")
	if err != nil {
		return fmt.Errorf("failed to create temp destination file: %w", err)
	}
	tmpDest := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpDest)
	}

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		cleanup()
		return fmt.Errorf("failed to copy across device boundary: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpDest)
		return fmt.Errorf("failed to close destination file: %w", err)
	}

	if err := os.Rename(tmpDest, destPath); err != nil {
		_ = os.Remove(tmpDest)
		return fmt.Errorf("failed to rename temp destination file: %w", err)
	}

	_ = srcFile.Close()
	_ = os.Remove(srcPath)
	return nil
}
