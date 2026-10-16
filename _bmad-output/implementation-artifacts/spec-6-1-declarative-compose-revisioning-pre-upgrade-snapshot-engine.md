---
title: 'Story 6.1: Declarative Compose Revisioning & Pre-Upgrade Snapshot Engine'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: 'bad8baa2c3ba006ef7a6a83ca076b792955068a9'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'internal/supervisor/supervisor.go'
  - 'internal/supervisor/docker.go'
  - 'internal/store/db.go'
  - 'internal/store/schema.sql'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In an unattended household appliance, container image updates or service configuration changes can fail, leaving the system in an unbootable state or causing silent forward database migrations that prevent manual or automatic downgrades. Administrators need every configuration change and image update to produce a declarative Compose revision file (`/data/state/compose/rev-:rev.yaml`), accompanied by an atomic SQLite snapshot (`/data/state/backups/pre-upgrade-:rev.db`) and a configuration volume backup prior to pulling new images, with the active revision tracked in `sovereign.db`.

**Approach:**
1. **SQLite Database Snapshotting (`internal/store/db.go`, `schema.sql`)**:
   - Add `compose_revisions` table to track revision numbers, service ID, old image, new image, compose file path, sqlite snapshot path, volume backup path, timestamp, and status (`active`, `rolled_back`, `superseded`).
   - Implement `CreateSnapshot(ctx context.Context, destPath string) error` on `*store.DB` using `VACUUM INTO ?` with directory creation and overwrite safety.
   - Implement `RecordRevision(ctx context.Context, rev *ComposeRevision) error`, `GetRevision(ctx context.Context, revision int) (*ComposeRevision, error)`, `GetActiveRevision(ctx context.Context) (*ComposeRevision, error)`, `ListRevisions(ctx context.Context) ([]ComposeRevision, error)`, and `UpdateRevisionStatus(ctx context.Context, revision int, status string) error`.
   - Store and track `active_compose_revision` in SQLite.
2. **Declarative Revision & Volume Snapshot Engine (`internal/supervisor/revision.go`)**:
   - `RevisionEngine` / `ServiceManager` methods:
     - `CreatePreUpgradeRevision(ctx context.Context, serviceID string, newImageTag string) (*ComposeRevision, error)`:
       1. Allocates next revision number atomically.
       2. Creates atomic SQLite snapshot at `/data/state/backups/pre-upgrade-:rev.db`.
       3. Snapshots the service's configuration volumes (e.g. host config/data dirs in `/data/state/<service>`) to `/data/state/backups/volumes-rev-:rev/<service>` via directory copy or tar archive.
       4. Generates declarative versioned Compose specification at `/data/state/compose/rev-:rev.yaml` matching Compose v2 schema.
       5. Persists revision record in `compose_revisions` and marks it as active revision in `sovereign.db`.
     - `SynthesizeRevisionCompose(dataDir string, revision int, spec ContainerSpec) (string, error)` writes atomic revision file to `/data/state/compose/rev-:rev.yaml`.
3. **Upgrade & Revision API Endpoints (`cmd/sovereign-daemon/server.go`)**:
   - `POST /api/v1/services/{id}/upgrade`: Accepts optional new image tag or triggers upgrade with pre-upgrade snapshot and revision tracking.
   - `GET /api/v1/system/revisions`: Lists all revisions.
   - `GET /api/v1/system/revisions/active`: Returns active revision metadata.
4. **OpenAPI & TypeScript Client (`api/openapi.yaml`, `web/src/lib/api/`)**:
   - Define schemas: `ComposeRevision`, `ComposeRevisionsResponse`, `ServiceUpgradeRequest`.
   - Add TypeScript client methods: `getRevisions()`, `getActiveRevision()`, `upgradeService(id, req)`.
5. **Testing**:
   - Unit tests in `internal/store/db_test.go` and `internal/supervisor/revision_test.go`.
   - Server integration tests in `cmd/sovereign-daemon/server_test.go`.
   - Web client tests in `web/src/lib/api/client.test.ts`.

</frozen-after-approval>

## Acceptance Criteria

1. **Declarative Revision Generation**: Applying a service configuration or image change generates a declarative compose file at `/data/state/compose/rev-:rev.yaml`.
2. **Atomic SQLite Pre-Upgrade Snapshot**: Prior to pulling new container images, `sovereign-daemon` generates an atomic SQLite snapshot at `/data/state/backups/pre-upgrade-:rev.db` via `VACUUM INTO`.
3. **Configuration Volume Snapshotting**: Prior to applying updates, service configuration directory volumes are snapshotted into `/data/state/backups/volumes-rev-:rev/<service>`.
4. **Persistent Active Revision Tracking**: The active revision number, timestamp, and status are tracked and persisted in `sovereign.db`.
5. **API & Client Integration**: REST endpoints `GET /api/v1/system/revisions`, `GET /api/v1/system/revisions/active`, and `POST /api/v1/services/{id}/upgrade` return typed RFC 7807 problem details or valid revision metadata.
6. **Automated Verification**: Comprehensive Go and TypeScript unit/integration tests pass with 100% success.

## Review Triage Log

- **Iteration 1**:
  - **Blind Hunter findings**:
    - Fixed error masking in `CreateSnapshot` fallback to return `errFallback` rather than parameter syntax error `err`.
    - Wrapped `RecordRevision` operations (`INSERT`, `UPDATE superseded`, and `system_state` active revision pointer) in an atomic database transaction (`BeginTx`).
    - Validated `rev.Status == "active"` in `GetActiveRevision` when resolving pointer from `system_state`.
    - Updated `UpdateRevisionStatus` to clear or point to the latest active revision when a revision's status is updated away from `active`.
  - **Edge Case Hunter findings**:
    - Fixed registry image tag splitting using `strings.LastIndex(oldImage, ":")` instead of `strings.Split(oldImage, ":")[0]` so private registries with port numbers (`registry:5000/...`) are correctly parsed.
    - Updated `UpgradeService` to mark revision record status as `"failed"` if image pulling or container starting encounters an error.
    - Ensured canonical Compose specification is synthesized and persisted even when upgrading an inactive service.
    - Preserved file mode permissions and safely handled symlinks in `CopyDir` / `copyFile`.
    - Added volume snapshotting coverage for Paperless persistent data directory (`documents/data`) while preventing directory collision duplication.
    - Added HTTP 400 Problem Details rejection for malformed JSON request bodies in `handleUpgradeService`.
    - Added `Content-Type: application/json` header in TypeScript client `upgradeService`.
  - **Verification Gap findings**:
    - Added test assertion in `internal/supervisor/revision_test.go` verifying `engine.GetStartedHistory()` contains the updated image container spec.
    - Added test in `internal/supervisor/revision_test.go` verifying that pull failures set revision status to `"failed"` in the database.
    - Added test in `internal/store/db_test.go` verifying `GetActiveRevision` does not return a revision after it is marked `"rolled_back"`.
    - Added test `TestServer_ServiceUpgrade_MalformedJSON` in `cmd/sovereign-daemon/server_test.go` asserting HTTP 400 RFC 7807 response for invalid JSON payloads.
  - **Outcome**: All remediations implemented and verified. Full test suites pass with 100% success.

