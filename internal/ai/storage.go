package ai

import (
	"errors"
	"fmt"
	"math"

	"sovereign/internal/fsutil"
)

// SafetyBufferBytes specifies the strict 5.0 GB disk space buffer (5 * 1024 * 1024 * 1024)
// required above the model file size to protect the appliance from storage exhaustion.
const SafetyBufferBytes int64 = 5 * 1024 * 1024 * 1024 // 5.0 GB

// ErrInsufficientStorageSpace is returned when available disk space is less than model size + 5 GB.
var ErrInsufficientStorageSpace = errors.New("insufficient storage space for model download")

// StorageSpaceError contains detailed metrics when storage reservation checks fail.
type StorageSpaceError struct {
	AvailableBytes int64 `json:"available_bytes"`
	RequiredBytes  int64 `json:"required_bytes"`
	BufferBytes    int64 `json:"buffer_bytes"`
}

func (e *StorageSpaceError) Error() string {
	return fmt.Sprintf("insufficient storage space: available %d bytes, required %d bytes (including %d bytes safety buffer)",
		e.AvailableBytes, e.RequiredBytes, e.BufferBytes)
}

func (e *StorageSpaceError) Unwrap() error {
	return ErrInsufficientStorageSpace
}

// DiskFreeFunc defines a function prototype for checking available bytes at a path.
type DiskFreeFunc = fsutil.DiskFreeFunc

func defaultDiskFree(path string) (int64, error) {
	return fsutil.DiskFree(path)
}

// CheckModelStorageSpace verifies that path has at least requiredBytes + 5.0 GB of free disk space.
// If free space is insufficient, returns ErrInsufficientStorageSpace (wrapped as *StorageSpaceError).
func CheckModelStorageSpace(path string, requiredBytes int64) error {
	return CheckModelStorageSpaceWithFunc(path, requiredBytes, defaultDiskFree)
}

// CheckModelStorageSpaceWithFunc allows dependency injection for disk space checks in tests.
func CheckModelStorageSpaceWithFunc(path string, requiredBytes int64, freeFn DiskFreeFunc) error {
	freeBytes, err := freeFn(path)
	if err != nil {
		return fmt.Errorf("failed to check free disk space: %w", err)
	}

	return CheckModelStorageSpaceWithFree(freeBytes, requiredBytes)
}

// CheckModelStorageSpaceWithFree compares available free bytes directly against requiredBytes + 5.0 GB.
func CheckModelStorageSpaceWithFree(freeBytes int64, requiredBytes int64) error {
	if requiredBytes < 0 || requiredBytes > math.MaxInt64-SafetyBufferBytes {
		return errors.New("invalid required bytes")
	}
	totalRequired := requiredBytes + SafetyBufferBytes
	if freeBytes < totalRequired {
		return &StorageSpaceError{
			AvailableBytes: freeBytes,
			RequiredBytes:  totalRequired,
			BufferBytes:    SafetyBufferBytes,
		}
	}
	return nil
}
