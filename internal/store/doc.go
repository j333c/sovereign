// Package store manages SQLite database persistence for the Sovereign appliance control plane.
//
// Invariants & Durability:
// - Database path: /data/state/sovereign.db
// - Journal Mode: WAL (Write-Ahead Logging) is strictly enforced for power-loss resilience.
// - Synchronous: NORMAL (1) pragma is enforced to eliminate fsync overhead on non-checkpoint transactions while preserving ACID guarantees across power cuts.
// - Schema Migrations: Embedded SQL migrations configure core system state, user settings, and boot diagnostics history.
package store
