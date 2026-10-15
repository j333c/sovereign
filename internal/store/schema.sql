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

-- Seed initial boot status if table is empty
INSERT OR IGNORE INTO boot_status (id, mode, clean_shutdown, diagnostics_completed, message)
VALUES (1, 'operational', 1, 1, 'All systems operational');
