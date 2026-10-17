---
title: 'Story 6.2: Automated 180-Second Health Verification & Atomic Rollback Execution'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '66b17b4061a58987f506d2b70d6656a0a385d469'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'internal/supervisor/supervisor.go'
  - 'internal/supervisor/revision.go'
  - 'internal/store/db.go'
  - 'internal/notify/dispatcher.go'
  - 'internal/notify/email.go'
  - 'web/src/components/ServiceCard.tsx'
  - 'web/src/lib/i18n.ts'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When container updates are applied (such as during the 3:00 AM maintenance window), a newly released upstream image might fail to boot, crash continuously, or fail critical health checks. In an unattended household appliance, this leaves family members waking up to broken core services (streaming, music, document management) with no automated recourse.

**Approach:**
1. **Automated 180-Second Health Verification Engine (`internal/supervisor/rollback.go`)**:
   - Following an image upgrade or scheduled maintenance update, initialize a 180-second health verification session for the updated service.
   - Monitor container health checks across the 180s window using the supervisor watchdog debouncer.
   - If health checks fail across 3 consecutive retries or the container enters a crash loop / terminates unexpectedly during the verification window, trigger an automated atomic rollback.
2. **Atomic Rollback Execution Engine (`internal/supervisor/rollback.go`, `internal/store/db.go`)**:
   - `db.RestoreSnapshot(ctx context.Context, snapshotPath string) error`: Safely replaces the active SQLite database with the pre-upgrade snapshot `/data/state/backups/pre-upgrade-:rev.db`, resetting WAL state and restoring previous schema/state without corrupting database connections.
   - `RollbackService(ctx context.Context, serviceID string, failedRevNum int) error`:
     1. Terminates the failing upgraded container (`engine.StopContainer`).
     2. Identifies the prior active revision (`prev_rev`) from `compose_revisions`.
     3. Restores declarative compose specification from `/data/state/compose/rev-:prev_rev.yaml`.
     4. Restores service configuration volume state from `/data/state/backups/volumes-rev-:prev_rev` (or pre-upgrade backup directory).
     5. Restores SQLite state from `/data/state/backups/pre-upgrade-:rev.db` via `db.RestoreSnapshot`.
     6. Restarts the container with the restored container specification.
     7. Updates revision records: marks failed revision as `rolled_back` and prior revision as `active`.
     8. Updates service runtime status to `degraded` with reassurance message:
        *"Update couldn't start properly. We've safely restored your previous working version."*
     9. Dispatches an alert email notification via `notify.AlertDispatcher` notifying the homeowner of the failed update and automated safe rollback.
     10. Broadcasts SSE events (`service.health_changed`, `service.status_changed`) to update connected dashboards immediately.
3. **Maintenance Window Execution (`internal/supervisor/maintenance.go`)**:
   - Configurable maintenance window (default: 3:00 AM) that verifies available updates and executes upgrades under the protection of the 180-second health verification window.
4. **Appliance Microcopy & Internationalization (`web/src/lib/i18n.ts`, `ServiceCard.tsx`)**:
   - Define localized reassurance copy for automated rollback across EN, DE, ES, and FR:
     - EN: *"Update couldn't start properly. We've safely restored your previous working version."*
     - DE: *"Das Update konnte nicht ordnungsgemäß gestartet werden. Ihre vorherige funktionierende Version wurde sicher wiederhergestellt."*
     - ES: *"La actualización no se pudo iniciar correctamente. Hemos restaurado de forma segura su versión de trabajo anterior."*
     - FR: *"La mise à jour n'a pas pu démarrer correctement. Nous avons restauré en toute sécurité votre version précédente fonctionnelle."*
   - Render Amber status badge (◆) and reassurance alert banner when `reassurance_message` is present on the service status.
5. **API Contract & TypeScript Client (`api/openapi.yaml`, `web/src/lib/api/`)**:
   - Include `reassurance_message` field in `ServiceStatus`.
   - Expose `POST /api/v1/services/{id}/rollback` to allow rollback execution and verification.
6. **Testing**:
   - Unit tests for `db.RestoreSnapshot` in `internal/store/db_test.go`.
   - Health verification and rollback unit tests in `internal/supervisor/rollback_test.go`.
   - Server API and SSE integration tests in `cmd/sovereign-daemon/server_test.go`.
   - Frontend reassurance microcopy and ServiceCard tests in `web/src/components/ServiceCard.test.tsx`.

</frozen-after-approval>

## Acceptance Criteria

1. **180-Second Verification Window**: Upon starting an updated container image, a 180-second health verification watchdog monitors probe responses.
2. **Automatic Rollback Trigger**: If health checks fail across 3 consecutive retries or enter a crash loop within the 180-second window, the daemon automatically terminates the container.
3. **Declarative Revision & Snapshot Restoration**: The daemon restores the previous compose revision `rev-:prev_rev.yaml`, restores configuration volumes from backup, and restores `/data/state/backups/pre-upgrade-:rev.db`.
4. **Service Status & Reassurance Banner**: The affected service card displays Amber status (◆) with the reassuring message: *"Update couldn't start properly. We've safely restored your previous working version."*
5. **Alert Email Dispatch**: The daemon dispatches an alert email to the configured administrator address detailing the rollback and reassurance status.
6. **Automated Verification**: Comprehensive unit and integration test suites pass with 100% success.

---

## Review Triage Log

### Cycle 1 (Baseline: `66b17b4061a58987f506d2b70d6656a0a385d469`)

Three review lenses (`Blind Hunter`, `Edge Case Hunter`, `Verification Gap Reviewer`) reviewed the implementation diff.

| Finding ID | Source Lens | Description | Resolution / Action Taken |
|---|---|---|---|
| SEC-01 | Edge Case Hunter | SQLite online backup connection leak in `RestoreSnapshot` if error occurs prior to close. | Applied: wrapped backup connection acquisition in clean `defer sqlConn.Close()`, added context cancellation loop checks, and verified `PRAGMA journal_mode = WAL` return. |
| SUP-01 | Blind Hunter | `RollbackService` with zero revisions returned nil error instead of `ErrNoPriorRevision`. | Applied: updated revision existence checks to explicitly return `ErrNoPriorRevision` (404) if revisions are empty or no prior revision exists. |
| SUP-02 | Edge Case Hunter | `runVerificationLoop` did not sync debouncer state when health check probes fail or succeed. | Applied: updated probe outcomes in `runVerificationLoop` to notify `v.mgr.debouncer.RecordProbe(sess.ServiceID, healthy)`. |
| SUP-03 | Edge Case Hunter | Verification engine routines not cancelled when `ServiceManager.Close()` is invoked. | Applied: added `Close()` to `VerificationEngine` and invoked it from `ServiceManager.Close()`. |
| API-01 | Blind Hunter | `handleRollbackService` swallowed malformed JSON payloads and relied on cancelable client HTTP context. | Applied: added strict JSON validation returning RFC 7807 400 Bad Request on malformed body, and executed rollback using detached 2-minute timeout context to prevent premature aborts on browser disconnect. |
| SPEC-01 | Verification Gap | `POST /api/v1/services/{id}/rollback` OpenAPI specification and TypeScript client did not declare optional revision body. | Applied: added optional `{ revision?: integer }` to OpenAPI 3.1 contract, 400 response, and updated client + schema. |
| UI-01 | Edge Case Hunter | Inactive/stopped services with reassurance message could be misclassified as amber degraded. | Applied: guarded `isAmber` in `ServiceCard.tsx` with `!isStoppedOrInactive` and verified stopped service retains stopped status badge. |
| TEST-01 | Verification Gap | Verification gaps for rollback server endpoints, multi-channel rollback alerts, and client rollback methods. | Applied: added comprehensive test cases across `rollback_test.go`, `notify_test.go`, `server_test.go`, `client.test.ts`, and `page.test.tsx`. |

### Verification Status
- `go test -count=1 ./...`: **PASS** (100% across all packages)
- `npm --prefix web test`: **PASS** (115/115 tests passing)

