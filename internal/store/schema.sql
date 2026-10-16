-- Baseline database schema for Sovereign appliance control plane

CREATE TABLE IF NOT EXISTS system_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS boot_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mode TEXT NOT NULL DEFAULT 'operational',
    clean_shutdown BOOLEAN NOT NULL DEFAULT 1,
    diagnostics_completed BOOLEAN NOT NULL DEFAULT 1,
    message TEXT NOT NULL DEFAULT 'All systems operational',
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'admin',
    email TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    role TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS transcode_queue (
    id TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    target_path TEXT NOT NULL,
    profile TEXT NOT NULL DEFAULT 'universal',
    status TEXT NOT NULL DEFAULT 'queued',
    progress REAL NOT NULL DEFAULT 0.0,
    encoder_used TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_transcode_queue_status ON transcode_queue(status, created_at);

-- Seed initial boot status if table is empty
INSERT OR IGNORE INTO boot_status (id, mode, clean_shutdown, diagnostics_completed, message)
VALUES (1, 'operational', 1, 1, 'All systems operational');

CREATE TABLE IF NOT EXISTS imap_configs (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 993,
    username TEXT NOT NULL,
    password_encrypted BLOB NOT NULL,
    folder TEXT NOT NULL DEFAULT 'INBOX',
    ssl_enabled BOOLEAN NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    last_polled_at TIMESTAMP NULL,
    last_error TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_imap_configs_enabled ON imap_configs(enabled, updated_at);

CREATE TABLE IF NOT EXISTS document_deadlines (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    title TEXT NOT NULL,
    due_date TEXT NOT NULL,
    amount REAL NULL,
    currency TEXT DEFAULT '',
    snippet TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_deadlines_doc_due ON document_deadlines(document_id, due_date);
CREATE INDEX IF NOT EXISTS idx_document_deadlines_status_due ON document_deadlines(status, due_date);

