package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAES256GCM_Encryption(t *testing.T) {
	key := []byte("super-secret-key-32-bytes-long!!")
	plaintext := []byte("my-secure-email-password-12345")

	ciphertext, err := EncryptSecret(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptSecret failed: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatalf("ciphertext must not match plaintext")
	}

	decrypted, err := DecryptSecret(key, ciphertext)
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("expected decrypted %s, got %s", string(plaintext), string(decrypted))
	}

	// Tampered ciphertext fails
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF
	_, err = DecryptSecret(key, tampered)
	if err == nil {
		t.Fatalf("expected error on tampered ciphertext, got nil")
	}

	// Wrong key fails
	wrongKey := []byte("wrong-secret-key-32-bytes-long!!")
	_, err = DecryptSecret(wrongKey, ciphertext)
	if err == nil {
		t.Fatalf("expected error with wrong key, got nil")
	}

	// Truncated ciphertext fails
	_, err = DecryptSecret(key, ciphertext[:5])
	if err == nil {
		t.Fatalf("expected error on truncated ciphertext, got nil")
	}
}

func setupTestDB(t *testing.T) (*DB, string) {
	tempDir, err := os.MkdirTemp("", "sovereign-imap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}
	return db, tempDir
}

func TestIMAPStore_CRUD(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}()

	key := []byte("test-encryption-key-for-imap-app")
	store, err := NewIMAPStore(db, key)
	if err != nil {
		t.Fatalf("NewIMAPStore failed: %v", err)
	}

	ctx := context.Background()

	// Initial active config should return ErrIMAPConfigNotFound
	_, err = store.GetActive(ctx)
	if err != ErrIMAPConfigNotFound {
		t.Fatalf("expected ErrIMAPConfigNotFound, got: %v", err)
	}

	// Save new config
	cfg := &IMAPConfig{
		Host:       "imap.example.com",
		Port:       993,
		Username:   "alex@family.net",
		Password:   "VerySecretPassword99!",
		Folder:     "INBOX",
		SSLEnabled: true,
		Enabled:    true,
	}

	err = store.Save(ctx, cfg)
	if err != nil {
		t.Fatalf("store.Save failed: %v", err)
	}

	if cfg.ID == "" {
		t.Fatalf("expected non-empty ID after Save")
	}

	// Verify at-rest encryption in raw SQLite
	var rawEncrypted []byte
	row := db.QueryRowContext(ctx, "SELECT password_encrypted FROM imap_configs WHERE id = ?", cfg.ID)
	if err := row.Scan(&rawEncrypted); err != nil {
		t.Fatalf("failed to query raw encrypted password: %v", err)
	}

	if bytes.Contains(rawEncrypted, []byte("VerySecretPassword99!")) {
		t.Fatalf("SECURITY VIOLATION: Plaintext password found in SQLite storage!")
	}

	// Fetch via Get
	fetched, err := store.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("store.Get failed: %v", err)
	}

	if fetched.Password != "VerySecretPassword99!" {
		t.Fatalf("expected decrypted password 'VerySecretPassword99!', got %q", fetched.Password)
	}
	if fetched.Host != "imap.example.com" || fetched.Port != 993 || fetched.Username != "alex@family.net" {
		t.Fatalf("unexpected fields in fetched config: %+v", fetched)
	}

	// Fetch via GetActive
	active, err := store.GetActive(ctx)
	if err != nil {
		t.Fatalf("store.GetActive failed: %v", err)
	}
	if active.ID != cfg.ID {
		t.Fatalf("expected active ID %s, got %s", cfg.ID, active.ID)
	}

	// Update status
	now := time.Now().UTC()
	err = store.UpdateStatus(ctx, cfg.ID, &now, "Connection timed out")
	if err != nil {
		t.Fatalf("store.UpdateStatus failed: %v", err)
	}

	updated, err := store.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("store.Get after update failed: %v", err)
	}
	if updated.LastError != "Connection timed out" {
		t.Fatalf("expected LastError 'Connection timed out', got %q", updated.LastError)
	}
	if updated.LastPolledAt == nil {
		t.Fatalf("expected non-nil LastPolledAt")
	}

	// Update config without changing password
	cfgUpdate := &IMAPConfig{
		ID:         cfg.ID,
		Host:       "mail.example.com",
		Port:       993,
		Username:   "alex@family.net",
		Folder:     "Archive",
		SSLEnabled: true,
		Enabled:    true,
	}
	err = store.Save(ctx, cfgUpdate)
	if err != nil {
		t.Fatalf("store.Save update without password failed: %v", err)
	}

	refetched, err := store.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("store.Get failed: %v", err)
	}
	if refetched.Host != "mail.example.com" || refetched.Folder != "Archive" {
		t.Fatalf("expected updated host and folder, got: %+v", refetched)
	}
	if refetched.Password != "VerySecretPassword99!" {
		t.Fatalf("expected existing password preserved, got %q", refetched.Password)
	}

	// List
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 config in list, got %d", len(list))
	}

	// Delete
	err = store.Delete(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("store.Delete failed: %v", err)
	}

	_, err = store.Get(ctx, cfg.ID)
	if err != ErrIMAPConfigNotFound {
		t.Fatalf("expected ErrIMAPConfigNotFound after delete, got %v", err)
	}
}

func TestIMAPStore_AutoMasterKeyGeneration(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}()

	// Pass empty key so it generates and stores in settings
	store1, err := NewIMAPStore(db, nil)
	if err != nil {
		t.Fatalf("NewIMAPStore with nil key failed: %v", err)
	}

	ctx := context.Background()
	cfg := &IMAPConfig{
		Host:     "imap.test.org",
		Username: "user@test.org",
		Password: "SecretStoredKey123",
	}
	if err := store1.Save(ctx, cfg); err != nil {
		t.Fatalf("store1.Save failed: %v", err)
	}

	// Reopen store with nil key on same DB; it should load the same key from settings
	store2, err := NewIMAPStore(db, nil)
	if err != nil {
		t.Fatalf("NewIMAPStore 2 failed: %v", err)
	}

	loaded, err := store2.Get(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("store2.Get failed: %v", err)
	}
	if loaded.Password != "SecretStoredKey123" {
		t.Fatalf("expected loaded password 'SecretStoredKey123', got %q", loaded.Password)
	}
}

func TestIMAPStore_GetFirst_DisabledConfig(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}()

	key := []byte("32-bytes-long-key-for-testing!!")
	store, err := NewIMAPStore(db, key)
	if err != nil {
		t.Fatalf("NewIMAPStore failed: %v", err)
	}

	ctx := context.Background()

	// When no config exists, GetFirst returns ErrIMAPConfigNotFound
	_, err = store.GetFirst(ctx)
	if err != ErrIMAPConfigNotFound {
		t.Fatalf("expected ErrIMAPConfigNotFound, got: %v", err)
	}

	// Save a disabled config
	cfg := &IMAPConfig{
		Host:       "imap.disabled.net",
		Port:       993,
		Username:   "disabled@family.net",
		Password:   "MySecretPass!",
		Folder:     "INBOX",
		SSLEnabled: true,
		Enabled:    false,
	}
	if err := store.Save(ctx, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// GetActive must fail because enabled == false
	_, err = store.GetActive(ctx)
	if err != ErrIMAPConfigNotFound {
		t.Fatalf("expected GetActive to fail for disabled config, got: %v", err)
	}

	// GetFirst must succeed and return the disabled config
	first, err := store.GetFirst(ctx)
	if err != nil {
		t.Fatalf("GetFirst failed: %v", err)
	}
	if first.Host != "imap.disabled.net" || first.Enabled != false {
		t.Fatalf("expected disabled config from GetFirst, got: %+v", first)
	}
}
