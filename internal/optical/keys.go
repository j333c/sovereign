package optical

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrEmptyKeyFile   = errors.New("key file is empty")
	ErrBinaryContent  = errors.New("invalid key file: binary content detected")
	ErrInvalidFormat  = errors.New("invalid KEYDB.cfg format: no recognizable keys or headers found")
)

// KeyDBInfo contains metadata about the active KEYDB.cfg configuration.
type KeyDBInfo struct {
	Exists    bool      `json:"exists"`
	Path      string    `json:"path"`
	KeyCount  int       `json:"key_count"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// KEYDBManager manages the storage, atomic update, and parsing of /data/state/keys/KEYDB.cfg.
type KEYDBManager struct {
	path      string
	logger    *slog.Logger
	mu        sync.RWMutex
	cachedKeys map[string]string // normalized disc ID / key -> title
	keyCount  int
	hasKeys   bool
	updatedAt time.Time
}

// NewKEYDBManager initializes a KEYDBManager for the given file path.
func NewKEYDBManager(path string, logger *slog.Logger) *KEYDBManager {
	if path == "" {
		path = "/data/state/keys/KEYDB.cfg"
	}
	if logger == nil {
		logger = slog.Default()
	}

	mgr := &KEYDBManager{
		path:       path,
		logger:     logger,
		cachedKeys: make(map[string]string),
	}

	// Load existing keys from disk if present
	_ = mgr.reload()
	return mgr
}

// GetPath returns the configured file path for KEYDB.cfg.
func (m *KEYDBManager) GetPath() string {
	return m.path
}

// HasKeys reports whether valid decryption keys are currently active.
func (m *KEYDBManager) HasKeys() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasKeys
}

// KeyCount returns the number of parsed keys in the active KEYDB.cfg.
func (m *KEYDBManager) KeyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.keyCount
}

// HasKeyForDisc checks whether keys exist for the given disc identifier.
func (m *KEYDBManager) HasKeyForDisc(discID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.hasKeys {
		return false
	}
	// If the database has global processing keys (PK), it can decrypt general discs
	if _, ok := m.cachedKeys["PK"]; ok {
		return true
	}
	if discID == "" {
		return m.hasKeys
	}

	normalized := normalizeDiscID(discID)
	_, found := m.cachedKeys[normalized]
	return found
}

// GetInfo returns the status and metrics of the current KEYDB.cfg.
func (m *KEYDBManager) GetInfo() KeyDBInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := KeyDBInfo{
		Exists:    m.hasKeys,
		Path:      m.path,
		KeyCount:  m.keyCount,
		UpdatedAt: m.updatedAt,
	}

	if fi, err := os.Stat(m.path); err == nil {
		info.SizeBytes = fi.Size()
		if info.UpdatedAt.IsZero() {
			info.UpdatedAt = fi.ModTime().UTC()
		}
	}

	return info
}

// Validate checks whether the provided content is a valid KEYDB.cfg file.
func (m *KEYDBManager) Validate(content []byte) error {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return ErrEmptyKeyFile
	}

	// Reject binary data
	if bytes.IndexByte(trimmed, 0) >= 0 {
		return ErrBinaryContent
	}

	parsedKeys, err := parseKeyDBContent(content)
	if err != nil {
		return err
	}

	if len(parsedKeys) == 0 {
		return ErrInvalidFormat
	}

	return nil
}

// SaveKeys validates the content and atomically writes it to KEYDB.cfg using temp file, fsync, and rename.
func (m *KEYDBManager) SaveKeys(content []byte) (int, error) {
	if err := m.Validate(content); err != nil {
		return 0, err
	}

	parsedKeys, err := parseKeyDBContent(content)
	if err != nil {
		return 0, err
	}

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Generate random suffix for atomic temp file
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	tempPath := fmt.Sprintf("%s.tmp.%s", m.path, hex.EncodeToString(randBytes))

	tmpFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to create temp key file: %w", err)
	}

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to write temp key file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to fsync temp key file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to close temp key file: %w", err)
	}

	// Atomic rename to final path
	if err := os.Rename(tempPath, m.path); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("failed to atomically replace KEYDB.cfg: %w", err)
	}

	m.mu.Lock()
	m.cachedKeys = parsedKeys
	m.keyCount = len(parsedKeys)
	m.hasKeys = len(parsedKeys) > 0
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()

	m.logger.Info("KEYDB.cfg successfully updated", "path", m.path, "key_count", len(parsedKeys))
	return len(parsedKeys), nil
}

// reload reads and parses KEYDB.cfg from disk if it exists.
func (m *KEYDBManager) reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		m.cachedKeys = make(map[string]string)
		m.keyCount = 0
		m.hasKeys = false
		return err
	}

	parsed, err := parseKeyDBContent(data)
	if err != nil || len(parsed) == 0 {
		m.cachedKeys = make(map[string]string)
		m.keyCount = 0
		m.hasKeys = false
		return err
	}

	m.cachedKeys = parsed
	m.keyCount = len(parsed)
	m.hasKeys = true
	if fi, err := os.Stat(m.path); err == nil {
		m.updatedAt = fi.ModTime().UTC()
	}
	return nil
}

// parseKeyDBContent scans KEYDB lines for comments, section headers, disc IDs, and processing keys.
func parseKeyDBContent(content []byte) (map[string]string, error) {
	keys := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Skip comment lines
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Disc VUK / title entries: 0x<disc_id> = <title> | ...
		if strings.HasPrefix(line, "0x") || strings.HasPrefix(line, "0X") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				rawID := strings.TrimSpace(parts[0])
				meta := strings.TrimSpace(parts[1])
				normID := normalizeDiscID(rawID)
				if normID != "" {
					keys[normID] = meta
					continue
				}
			}
		}

		// Processing Key: PK 0x...
		if strings.HasPrefix(line, "PK ") || strings.HasPrefix(line, "PK\t") {
			keys["PK"] = strings.TrimSpace(line)
			continue
		}

		// Device Key: DK 0x...
		if strings.HasPrefix(line, "DK ") || strings.HasPrefix(line, "DK\t") {
			keys["DK"] = strings.TrimSpace(line)
			continue
		}

		// Host Certificate: HC 0x...
		if strings.HasPrefix(line, "HC ") || strings.HasPrefix(line, "HC\t") {
			keys["HC"] = strings.TrimSpace(line)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading keydb content: %w", err)
	}

	return keys, nil
}

// normalizeDiscID trims leading 0x and converts hex IDs to lowercase.
func normalizeDiscID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "0x")
	id = strings.TrimPrefix(id, "0X")
	return strings.ToLower(id)
}
