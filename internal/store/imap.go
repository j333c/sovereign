package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	ErrIMAPConfigNotFound = errors.New("imap configuration not found")
	ErrInvalidEncryptionKey = errors.New("encryption key cannot be empty")
	ErrCiphertextTooShort   = errors.New("ciphertext too short for AES-GCM nonce")
)

// IMAPConfig represents a mailbox polling configuration.
type IMAPConfig struct {
	ID                string     `json:"id"`
	Host              string     `json:"host"`
	Port              int        `json:"port"`
	Username          string     `json:"username"`
	Password          string     `json:"password,omitempty"`
	PasswordEncrypted []byte     `json:"-"`
	Folder            string     `json:"folder"`
	SSLEnabled        bool       `json:"ssl_enabled"`
	Enabled           bool       `json:"enabled"`
	LastPolledAt      *time.Time `json:"last_polled_at,omitempty"`
	LastError         string     `json:"last_error"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DeriveKey ensures the encryption key is exactly 32 bytes (for AES-256).
func DeriveKey(rawKey []byte) []byte {
	if len(rawKey) == 32 {
		return rawKey
	}
	hash := sha256.Sum256(rawKey)
	return hash[:]
}

// EncryptSecret encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// The nonce is prepended to the resulting ciphertext.
func EncryptSecret(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrInvalidEncryptionKey
	}
	derivedKey := DeriveKey(key)

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptSecret decrypts an AES-256-GCM ciphertext whose first 12 bytes are the nonce.
func DecryptSecret(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrInvalidEncryptionKey
	}
	derivedKey := DeriveKey(key)

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret: %w", err)
	}

	return plaintext, nil
}

// IMAPStore handles persistent storage and AES-256-GCM encryption of IMAP credentials.
type IMAPStore struct {
	db  *DB
	key []byte
}

// NewIMAPStore creates an IMAPStore instance, deriving or loading the AES-256 master key.
func NewIMAPStore(db *DB, key []byte) (*IMAPStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	var derivedKey []byte
	if len(key) > 0 {
		derivedKey = DeriveKey(key)
	} else {
		envKey := os.Getenv("SOVEREIGN_SECRET_KEY")
		if envKey != "" {
			derivedKey = DeriveKey([]byte(envKey))
		} else {
			// Check settings table for persistent secret key
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			storedKeyHex, err := db.GetSetting(ctx, "secret_encryption_key")
			if err != nil {
				return nil, fmt.Errorf("failed to query master encryption key from settings: %w", err)
			}

			if storedKeyHex != "" {
				decoded, decErr := hex.DecodeString(strings.TrimSpace(storedKeyHex))
				if decErr == nil && len(decoded) == 32 {
					derivedKey = decoded
				}
			}

			if len(derivedKey) == 0 {
				newKey := make([]byte, 32)
				if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
					return nil, fmt.Errorf("failed to generate master encryption key: %w", err)
				}
				if err := db.SetSetting(ctx, "secret_encryption_key", hex.EncodeToString(newKey)); err != nil {
					return nil, fmt.Errorf("failed to persist master encryption key in settings: %w", err)
				}
				derivedKey = newKey
			}
		}
	}

	return &IMAPStore{
		db:  db,
		key: derivedKey,
	}, nil
}

func generateUUIDv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Save inserts or updates an IMAPConfig record, encrypting the password at rest.
func (s *IMAPStore) Save(ctx context.Context, cfg *IMAPConfig) error {
	if cfg == nil {
		return fmt.Errorf("imap config cannot be nil")
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if cfg.Port <= 0 {
		cfg.Port = 993
	}
	if strings.TrimSpace(cfg.Folder) == "" {
		cfg.Folder = "INBOX"
	}

	if cfg.ID == "" {
		cfg.ID = generateUUIDv4()
	}

	// Encrypt password if provided in plaintext
	if cfg.Password != "" {
		enc, err := EncryptSecret(s.key, []byte(cfg.Password))
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		cfg.PasswordEncrypted = enc
	} else if len(cfg.PasswordEncrypted) == 0 {
		// If both are empty and updating, retain existing encrypted password
		existing, err := s.Get(ctx, cfg.ID)
		if err == nil && existing != nil && len(existing.PasswordEncrypted) > 0 {
			cfg.PasswordEncrypted = existing.PasswordEncrypted
		}
	}

	if len(cfg.PasswordEncrypted) == 0 {
		return fmt.Errorf("password cannot be empty for new imap configuration")
	}

	sslEnabledInt := 0
	if cfg.SSLEnabled {
		sslEnabledInt = 1
	}
	enabledInt := 0
	if cfg.Enabled {
		enabledInt = 1
	}

	var lastPolledAtVal any
	if cfg.LastPolledAt != nil {
		lastPolledAtVal = cfg.LastPolledAt.UTC().Format(time.RFC3339)
	}

	query := `
		INSERT INTO imap_configs (
			id, host, port, username, password_encrypted, folder, ssl_enabled, enabled, last_polled_at, last_error, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			password_encrypted = excluded.password_encrypted,
			folder = excluded.folder,
			ssl_enabled = excluded.ssl_enabled,
			enabled = excluded.enabled,
			last_polled_at = excluded.last_polled_at,
			last_error = excluded.last_error,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query,
		cfg.ID,
		cfg.Host,
		cfg.Port,
		cfg.Username,
		cfg.PasswordEncrypted,
		cfg.Folder,
		sslEnabledInt,
		enabledInt,
		lastPolledAtVal,
		cfg.LastError,
	)
	if err != nil {
		return fmt.Errorf("failed to save imap config: %w", err)
	}

	return nil
}

// Get retrieves an IMAPConfig by ID and decrypts its password.
func (s *IMAPStore) Get(ctx context.Context, id string) (*IMAPConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, host, port, username, password_encrypted, folder, ssl_enabled, enabled, last_polled_at, last_error, created_at, updated_at
		FROM imap_configs
		WHERE id = ?
	`, id)

	cfg, err := s.scanConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIMAPConfigNotFound
		}
		return nil, fmt.Errorf("failed to query imap config %q: %w", id, err)
	}

	return cfg, nil
}

// GetActive returns the first active (enabled) IMAPConfig, ordered by updated_at DESC.
func (s *IMAPStore) GetActive(ctx context.Context) (*IMAPConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, host, port, username, password_encrypted, folder, ssl_enabled, enabled, last_polled_at, last_error, created_at, updated_at
		FROM imap_configs
		WHERE enabled = 1
		ORDER BY updated_at DESC
		LIMIT 1
	`)

	cfg, err := s.scanConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIMAPConfigNotFound
		}
		return nil, fmt.Errorf("failed to query active imap config: %w", err)
	}

	return cfg, nil
}

// List returns all IMAP configurations.
func (s *IMAPStore) List(ctx context.Context) ([]*IMAPConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, host, port, username, password_encrypted, folder, ssl_enabled, enabled, last_polled_at, last_error, created_at, updated_at
		FROM imap_configs
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list imap configs: %w", err)
	}
	defer rows.Close()

	var result []*IMAPConfig
	for rows.Next() {
		cfg, err := s.scanConfig(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading imap configs: %w", err)
	}

	return result, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *IMAPStore) scanConfig(scanner rowScanner) (*IMAPConfig, error) {
	var cfg IMAPConfig
	var sslInt, enabledInt int
	var lastPolledStr sql.NullString
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&cfg.ID,
		&cfg.Host,
		&cfg.Port,
		&cfg.Username,
		&cfg.PasswordEncrypted,
		&cfg.Folder,
		&sslInt,
		&enabledInt,
		&lastPolledStr,
		&cfg.LastError,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	cfg.SSLEnabled = sslInt != 0
	cfg.Enabled = enabledInt != 0

	if lastPolledStr.Valid && lastPolledStr.String != "" {
		if t, err := time.Parse(time.RFC3339, lastPolledStr.String); err == nil {
			cfg.LastPolledAt = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05", lastPolledStr.String); err == nil {
			cfg.LastPolledAt = &t
		}
	}

	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		cfg.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
		cfg.CreatedAt = t
	}

	if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
		cfg.UpdatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
		cfg.UpdatedAt = t
	}

	// Decrypt password
	if len(cfg.PasswordEncrypted) > 0 {
		plain, err := DecryptSecret(s.key, cfg.PasswordEncrypted)
		if err == nil {
			cfg.Password = string(plain)
		}
	}

	return &cfg, nil
}

// UpdateStatus updates the polling timestamp and error status.
func (s *IMAPStore) UpdateStatus(ctx context.Context, id string, lastPolledAt *time.Time, lastError string) error {
	var lastPolledAtVal any
	if lastPolledAt != nil {
		lastPolledAtVal = lastPolledAt.UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE imap_configs
		SET last_polled_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, lastPolledAtVal, lastError, id)
	if err != nil {
		return fmt.Errorf("failed to update imap config status: %w", err)
	}

	return nil
}

// Delete removes an IMAP configuration by ID.
func (s *IMAPStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM imap_configs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete imap config: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrIMAPConfigNotFound
	}
	return nil
}
