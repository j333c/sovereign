package ai

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GGUFMagic represents the standard binary magic header (0x47, 0x47, 0x55, 0x46 = "GGUF").
var GGUFMagic = [4]byte{0x47, 0x47, 0x55, 0x46}

var (
	// ErrInvalidGGUFMagic is returned when a file does not begin with the GGUF binary magic bytes.
	ErrInvalidGGUFMagic = errors.New("invalid GGUF magic header")
	// ErrInvalidGGUFVersion is returned when a GGUF file has an invalid or unsupported version.
	ErrInvalidGGUFVersion = errors.New("invalid or unsupported GGUF version")
)

// GGUFHeader holds the parsed binary header metadata of a verified GGUF model file.
type GGUFHeader struct {
	Magic           [4]byte `json:"magic"`
	Version         uint32  `json:"version"`
	TensorCount     uint64  `json:"tensor_count"`
	MetadataKVCount uint64  `json:"metadata_kv_count"`
}

// ModelMetadata represents a verified GGUF model found on disk.
type ModelMetadata struct {
	Name       string      `json:"name"`
	Filename   string      `json:"filename"`
	Path       string      `json:"path"`
	SizeBytes  int64       `json:"size_bytes"`
	Header     *GGUFHeader `json:"header,omitempty"`
	ModifiedAt time.Time   `json:"modified_at"`
	Valid      bool        `json:"valid"`
}

// ModelInfo is an alias for ModelMetadata.
type ModelInfo = ModelMetadata

// ValidateGGUF parses and verifies the GGUF binary header from an io.Reader.
// Returns ErrInvalidGGUFMagic if magic bytes do not match 0x47, 0x47, 0x55, 0x46.
func ValidateGGUF(r io.Reader) (*GGUFHeader, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrInvalidGGUFMagic
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidGGUFMagic, err)
	}

	if !bytes.Equal(magic[:], GGUFMagic[:]) {
		return nil, ErrInvalidGGUFMagic
	}

	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: unexpected EOF reading version", ErrInvalidGGUFVersion)
		}
		return nil, fmt.Errorf("failed to read GGUF version: %w", err)
	}

	// GGUF specifications define versions 1, 2, and 3.
	if version == 0 || version > 100 {
		return nil, ErrInvalidGGUFVersion
	}

	var tensorCount uint64
	if err := binary.Read(r, binary.LittleEndian, &tensorCount); err != nil {
		return nil, fmt.Errorf("failed to read tensor count: %w", err)
	}

	var metadataKVCount uint64
	if err := binary.Read(r, binary.LittleEndian, &metadataKVCount); err != nil {
		return nil, fmt.Errorf("failed to read metadata KV count: %w", err)
	}

	return &GGUFHeader{
		Magic:           magic,
		Version:         version,
		TensorCount:     tensorCount,
		MetadataKVCount: metadataKVCount,
	}, nil
}

// ValidateGGUFHeader opens the file at path and inspects its binary header.
// Returns ErrInvalidGGUFMagic if the file lacks GGUF magic bytes or is truncated.
func ValidateGGUFHeader(path string) (*GGUFHeader, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open model file %q: %w", path, err)
	}
	if fi.Size() < 24 {
		return nil, ErrInvalidGGUFMagic
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open model file %q: %w", path, err)
	}
	defer f.Close()

	return ValidateGGUF(f)
}

// ScanModels inspects dir for .gguf model files, verifies their headers, and returns verified metadata.
// Incomplete or downloading files (.downloading) and hidden files are ignored.
// Corrupted files failing ValidateGGUFHeader are rejected and omitted from the returned slice.
// Duplicate symlinks referencing the same underlying model file are deduplicated.
func ScanModels(dir string) ([]ModelMetadata, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ModelMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read models directory %q: %w", dir, err)
	}

	var models []ModelMetadata
	seenCanonical := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".downloading") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		canonicalPath, err := filepath.EvalSymlinks(fullPath)
		if err == nil {
			if seenCanonical[canonicalPath] {
				continue
			}
			seenCanonical[canonicalPath] = true
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		header, err := ValidateGGUFHeader(fullPath)
		if err != nil {
			// Skip corrupt or invalid models
			continue
		}

		models = append(models, ModelMetadata{
			Name:       strings.TrimSuffix(name, filepath.Ext(name)),
			Filename:   name,
			Path:       fullPath,
			SizeBytes:  info.Size(),
			Header:     header,
			ModifiedAt: info.ModTime(),
			Valid:      true,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Filename < models[j].Filename
	})

	return models, nil
}
