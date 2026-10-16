---
title: 'Story 6.1: Declarative Compose Revisioning & Pre-Upgrade Snapshot Engine'
type: 'feature'
created: '2026-10-16'
status: 'review'
route: 'dispatch'
baseline_commit: 'bad8baa2c3ba006ef7a6a83ca076b792955068a9'
review_loop_iteration: 0
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
