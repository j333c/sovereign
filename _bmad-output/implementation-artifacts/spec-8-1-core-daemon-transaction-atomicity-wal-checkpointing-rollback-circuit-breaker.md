---
title: 'Story 8.1: Core Daemon Transaction Atomicity, WAL Restoration Checkpointing & Rollback Circuit Breaker'
type: 'feature'
created: '2026-10-25'
status: 'in-review'
baseline_commit: '8943d107e134743043b57a3ef7b3694b26c9be0c'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/implementation-artifacts/epic-8-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Partial failures during first-run setup can leave orphaned database rows, database file replacement during rollback can encounter file locks or WAL desynchronization, uncontrolled rollback retry loops can consume CPU/logs during persistent container failures, and running without a Docker socket can fail silently.

**Approach:** Wrap all `CompleteSetup` operations in a single atomic SQLite transaction; execute `PRAGMA wal_checkpoint(TRUNCATE)` and close active SQLite connections before replacing database snapshot files on disk; enforce a circuit breaker capping rollback attempts to 2 with SafeRecovery state and emergency alert events; and surface explicit, reassuring telemetry indicators and logs when `/var/run/docker.sock` is unavailable.

## Boundaries & Constraints

**Always:**
- Wrap admin user insertion, initial configuration settings, and initial session generation within a single atomic SQLite transaction in `CompleteSetup`.
- Roll back all changes if any step in `CompleteSetup` fails, leaving zero rows in `users`, `settings`, or `sessions`.
- Execute `PRAGMA wal_checkpoint(TRUNCATE)` and cleanly close active database connections before replacing `sovereign.db` on disk during snapshot restoration.
- Cap automated rollback attempts per service revision to a maximum of 2, transitioning to `safe_recovery` state and emitting an `emergency.rollback_loop` SSE event if exceeded.
- Surface an explicit warning and degraded status in telemetry (`TelemetryUpdate`) and daemon logs when `/var/run/docker.sock` is absent or unresponsive.

**Never:**
- Never commit partial setup state to `sovereign.db` if session generation or setting persistence fails.
- Never overwrite the active database file on disk while connection pool handles remain open without WAL truncation.
- Never allow automated rollback execution to enter an unbounded retry loop.
- Never panic, crash, or fail silently with nil-pointer dereferences when the Docker socket is missing.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Setup Success | Valid `SetupRequest` payload | Admin user, settings, and session committed atomically; HTTP 200 with session cookie | N/A |
| Setup Mid-Flight Failure | Injected DB write failure during setup | Transaction rolled back; zero rows in `users`, `settings`, `sessions`; HTTP 500 RFC 7807 problem | Rollback transaction, return `ProblemDetails` |
| Database Snapshot Restore | Pre-upgrade snapshot file exists | `PRAGMA wal_checkpoint(TRUNCATE)` run, active connection closed, file replaced on disk, DB reopened, integrity checked | Return error if snapshot corrupted or missing |
| Rollback Loop Exceeded | Upgraded container fails probe repeatedly (>2 rollbacks) | Circuit breaker trips after attempt 2; enters `safe_recovery`; logs emergency alert; emits `emergency.rollback_loop` | Halt rollback loop, keep container stopped |
| Docker Socket Missing | `/var/run/docker.sock` does not exist on boot or runtime | Telemetry stream includes `docker_available: false`, status `degraded`, reassuring copy logged | Daemon continues non-container features cleanly |

</frozen-after-approval>

## Code Map

- `internal/auth/auth.go` -- `CompleteSetup` method; atomicity of user creation, settings persistence, and session record creation within `tx`.
- `internal/auth/auth_test.go` -- Verification of transaction rollback on simulated failure during setup.
- `internal/store/db.go` -- `RestoreSnapshot` method; `PRAGMA wal_checkpoint(TRUNCATE)`, connection pool teardown, disk file replacement, and clean reopen with integrity check.
- `internal/store/db_test.go` -- Unit tests for snapshot restore durability and connection cycling.
- `internal/supervisor/rollback.go` -- `ServiceManager` rollback logic, attempt tracking, and circuit breaker threshold enforcement (`max_rollback_attempts = 2`).
- `internal/supervisor/revision.go` -- Revision tracking, status updates, and emergency event broadcast.
- `internal/supervisor/rollback_test.go` -- Tests for rollback circuit breaker and max attempt limits.
- `cmd/sovereign-daemon/server.go` -- `getTelemetryUpdate` and `handleEvents`; Docker socket presence detection, warning telemetry reporting, and SSE broadcast.
- `api/openapi.yaml` -- `TelemetryUpdate` schema definition with optional `docker_available`, `docker_status`, and `docker_message` fields.

## Tasks & Acceptance

**Execution:**
- [x] `internal/auth/auth.go` -- Refactor `CompleteSetup` to execute user creation, settings upsert, and session creation inside a single `tx`, adding `CreateSessionTx` helper.
- [x] `internal/auth/auth_test.go` -- Add test verifying that injected failure inside `CompleteSetup` leaves zero rows in `users`, `settings`, and `sessions`.
- [x] `internal/store/db.go` -- Update `RestoreSnapshot` to execute `PRAGMA wal_checkpoint(TRUNCATE)`, close active connection pool, replace file on disk, reopen DB, and run `CheckIntegrity`.
- [x] `internal/store/db_test.go` -- Add tests verifying connection closing and file replacement during `RestoreSnapshot`.
- [x] `internal/supervisor/rollback.go` -- Implement `rollbackAttempts` tracker and circuit breaker in `rollbackServiceInternal`, capping automated attempts at 2 and entering `safe_recovery`.
- [x] `internal/supervisor/rollback_test.go` -- Add test for rollback circuit breaker triggering `safe_recovery` and `emergency.rollback_loop`.
- [x] `api/openapi.yaml` & `cmd/sovereign-daemon/server.go` -- Add Docker availability fields to `TelemetryUpdate` schema and daemon telemetry builder, running codegen.
- [x] `cmd/sovereign-daemon/server_test.go` -- Add test verifying Docker socket missing telemetry indicator and warning log output.

**Acceptance Criteria:**
- Given the First-Run Setup completion endpoint `CompleteSetup`, when an error or abort occurs during any stage of setup execution, then the underlying database transaction rolls back atomically without leaving partial setup records or orphaned state in `sovereign.db`.
- Given an automated or manual database snapshot rollback, when restoring a database snapshot file, then `PRAGMA wal_checkpoint(TRUNCATE)` is executed and the active SQLite connection is cleanly closed before replacing the database file on disk.
- Given a rolling service update failure that triggers automatic rollback, when rollback execution is attempted, then a circuit breaker pattern caps rollback retry attempts (max 2 attempts) to prevent infinite rollback loops, logging an emergency alert state if the previous revision also fails health verification.
- Given the supervisor daemon running without access to `/var/run/docker.sock`, when system telemetry is polled or streamed via SSE, then an explicit, reassuring warning indicator is surfaced in the telemetry stream and daemon logs rather than unhandled nil-pointer or silent container state failures.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence | Disposition |
|---------|---------|----------|-------------|
| Destructive file truncate and missing self-restore guard in `RestoreSnapshot` (`internal/store/db.go`) | high | Destination opened with `O_TRUNC` before copy; self-restore would wipe DB and corrupted copy leaves database destroyed without rollback | patch |
| SSE event duplicate emission on ticker ticks (`cmd/sovereign-daemon/server.go`) | medium | Emits `system.docker_unavailable` on every ticker tick instead of once on state change, causing stream flooding and test races | patch |
| Missing adoption of `safe_recovery` in frontend `mapServiceStatus` (`web/src/app/page.tsx`) | medium | Status falls through all checks to "healthy" if health is empty, masking stopped containers in UI | patch |
| Missing positive verification test for active Docker socket | medium | Gap verified by verification-gap reviewer; only missing socket path was tested | patch |
| Re-verifying restored container after automated rollback (`internal/supervisor/rollback.go`) | medium | Restored container is restarted without starting verification session, preventing second verification failure in cascading loop | patch |
| Cascading rollback attempt tracking key (`internal/supervisor/rollback.go`) | medium | Keying attempts by service ID ensures total consecutive failures across chained revisions are counted against max attempts (2) | patch |
| Nil check in `CreateSessionTx` (`internal/auth/auth.go`) | low | Added defensive nil check for `tx` argument | patch |
| Dead error definition `ErrRollbackCircuitBreaker` (`internal/supervisor/rollback.go`) | low | Return exported error when circuit breaker trips so callers can detect it | patch |
| Inode mode check for Docker socket (`cmd/sovereign-daemon/server.go`) | low | Check `fi.Mode()&os.ModeSocket != 0` to ensure file is an actual socket | patch |


## Design Notes

### Rollback Circuit Breaker Design
The `ServiceManager` maintains an in-memory counter `rollbackAttempts map[string]int` keyed by `serviceID:revision`. When automated rollback is triggered:
1. If `rollbackAttempts[key] >= 2`, the breaker trips:
   - Service status transitions to `StatusSafeRecovery` (`"safe_recovery"`), health `degraded`.
   - Reassurance message: *"Service update encountered recurring issues. Container safely paused in recovery mode."*
   - Event broadcaster publishes `emergency.rollback_loop` with service ID and revision details.
   - Rollback aborts cleanly without restarting the faulty container.
2. Otherwise, `rollbackAttempts[key]++` and rollback proceeds.
3. Successful manual rollback or new upgrade resets the attempt counter.

## Verification

**Commands:**
- `go test -v -count=1 ./internal/auth -run TestCompleteSetup_Atomicity` -- expected: PASS
- `go test -v -count=1 ./internal/store -run TestRestoreSnapshot` -- expected: PASS
- `go test -v -count=1 ./internal/supervisor -run TestRollback_CircuitBreaker` -- expected: PASS
- `go test -v -count=1 ./cmd/sovereign-daemon -run TestDockerSocket_MissingWarning` -- expected: PASS
- `go test -count=1 ./...` -- expected: PASS
- `npm --prefix web run codegen` -- expected: PASS
- `npm --prefix web test -- --run` -- expected: PASS
