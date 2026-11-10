---
title: 'Story 10.3: Installer Channel Selection & Unattended Maintenance Pipeline Integration'
type: 'feature'
created: '2026-11-10'
status: 'done'
baseline_commit: '0e2f760f381f8876c12c759fa21d4be0df33177d'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/epics.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:**
Administrators need CLI and installer support for channel selection (`--channel stable|security-only|preview`) during both initial turnkey host installation and `--upgrade` execution. The chosen channel must be seeded into `/data/state/update_channel` so that `sovereign-daemon` initializes its SQLite channel on first boot. Furthermore, the 3:00 AM unattended maintenance runner must respect the active channel, perform `PRAGMA wal_checkpoint(TRUNCATE)` and atomic pre-maintenance SQLite snapshots before applying updates, record overnight update outcomes in SQLite (`last_update_status`, `last_update_version`, `last_update_timestamp`, `last_update_message`, `last_update_dismissed`), and expose them via REST (`GET /api/v1/system/boot-status` and `POST /api/v1/system/update-status/dismiss`) so the administrator can see and dismiss morning notification banners.

**Approach:**
1. **Host Installer (`install.sh` & `scripts/install.sh`):**
   - Add `--channel <channel>` argument with validation (`stable`, `security-only`, `preview`, default: `stable`).
   - If `--upgrade` is invoked without `--channel`, read existing channel from `${DATA_DIR}/state/update_channel` if present.
   - Resolve and checkout the newest qualifying release tag for the channel, falling back to `main` if no qualifying tags exist yet.
   - Write selected channel to `${DATA_DIR}/state/update_channel`.
   - Update `--help` documentation.
2. **Database & Storage Layer (`internal/store/`):**
   - Extend `BootStatusInfo` with `last_update_status`, `last_update_version`, `last_update_timestamp`, `last_update_message`, and `last_update_dismissed`.
   - Add `SetOvernightUpdateStatus`, `GetOvernightUpdateStatus`, and `DismissOvernightUpdateStatus` methods to `*store.DB`.
3. **Supervisor Maintenance Engine (`internal/supervisor/maintenance.go`):**
   - Integrate `*store.DB` and `*UpdateDiscoveryEngine` into `MaintenanceScheduler`.
   - Before applying updates, execute `db.FlushWAL(ctx)` (`PRAGMA wal_checkpoint(TRUNCATE)`) and atomic `db.CreateSnapshot(ctx, snapshotPath)`.
   - Query candidate tag from discovery engine for current channel and record overnight update status in SQLite (`updated`, `postponed`, or `idle`).
4. **OpenAPI & Daemon REST Handlers:**
   - Update `api/openapi.yaml` with `last_update_*` properties in `BootStatusResponse` and register `POST /api/v1/system/update-status/dismiss`.
   - Run `npm --prefix web run codegen`.
   - Register route in `cmd/sovereign-daemon/routes.go`.
   - Implement `handleDismissUpdateStatus` and wire `handleBootStatus` in `cmd/sovereign-daemon/handlers_system.go`.
5. **Testing & Verification:**
   - Add matrix shell tests in `tests/installer_test.sh` for `--channel` flags and validation.
   - Add unit tests in `internal/supervisor/maintenance_test.go` for WAL flushing, snapshotting, and update status recording.
   - Add HTTP tests in `cmd/sovereign-daemon/handlers_system_test.go` for boot-status update fields and dismiss endpoint.

</frozen-after-approval>
