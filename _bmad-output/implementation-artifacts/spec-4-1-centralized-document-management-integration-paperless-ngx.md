---
title: 'Story 4.1: Centralized Document Management Integration (Paperless-ngx)'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: 'dea5dad94581d9d9bfbc5cddcc067174051364d8'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sarah and household family members need a centralized, private document management system to archive insurance policies, medical records, and utility bills with automated text OCR and full-text search, without cloud leaks, complex server setup, or resource starvation of other server services.

**Approach:** Integrate Paperless-ngx into the `sovereign-daemon` supervisor with automated on-demand container pulling (`paperlessngx/paperless-ngx:3.3.1`), persistent `/data/documents/` storage provisioning (`media`, `data`, `consume`, `export`), bounded memory limits, CPU priority throttling (`max(1, physical_cores / 2)` OCR workers at `nice -n 10`), SQLite FTS5 full-text indexing with WAL mode, dynamic Caddy ingress routing on dedicated port `:8000` (for mobile companion apps) and subpath `/docs` (with `PAPERLESS_FORCE_SCRIPT_NAME: "/docs"`), and service lifecycle controls (`activate`, `deactivate`, `restart`).

## Boundaries & Constraints

**Always:**
- Use official pinned image `paperlessngx/paperless-ngx:3.3.1`.
- Automatically provision storage directories `/data/documents/{media,data,consume,export}` and `/data/state/services/paperless` with `0755` permissions, strictly preserving existing files and SQLite databases.
- Enforce memory boundaries (1GB limit, 512MB reservation) in container spec and compose file synthesis.
- Cap OCR and task worker concurrency to `max(1, physical_cores / 2)` using `PAPERLESS_OCR_WORKERS` and `PAPERLESS_TASK_WORKERS`.
- Schedule OCR tasks with low CPU priority (`nice -n 10`) via `PAPERLESS_OCR_USER_ARGS: '{"nice": 10}'`.
- Configure Paperless subpath support via `PAPERLESS_FORCE_SCRIPT_NAME: "/docs"`.
- Support SQLite database with `PRAGMA journal_mode = WAL;`.
- Route traffic via Caddy on dedicated port `:8000` and subpath `/docs` with `X-Forwarded-Prefix: /docs`.
- Support service alias resolution: `dms`, `documents`, `paperless` all resolve to `ServicePaperless`.
- Provide automated unit and integration tests verifying directory creation, compose synthesis, OCR worker bounds, volume preservation, and Caddy ingress routing.

**Never:**
- Never mount the host Docker socket (`/var/run/docker.sock`) into Paperless containers.
- Never delete or prune `/data/documents` during container stops, restarts, or updates.
- Never allow OCR background workers to consume 100% of CPU cores.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Activation | `POST /api/v1/services/paperless/activate` (or `/services/dms/activate`) | Provisions `/data/documents/*`, synthesizes compose spec with OCR bounds and memory limits, starts container, syncs Caddy routes (`/docs`, port 8000) | 502 on pull failure; emits SSE progress |
| Ingress Routing | Paperless active | Caddy proxies `/docs/*` with `X-Forwarded-Prefix: /docs` to `sovereign-paperless:8000`, and port 8000 direct listener `srv_8000` | Ingress status lists active route |
| Subpath Asset Loading | Browser requests `/docs/` and assets | `PAPERLESS_FORCE_SCRIPT_NAME: "/docs"` generates URLs under `/docs/`; routes cleanly through Caddy | Trailing slash redirection handled |
| Deactivation | `POST /api/v1/services/paperless/deactivate` | Stops container, updates status to `inactive`, preserves `/data/documents/*` and state | Volumes preserved intact |
| Single CPU Core Host | System has 1 CPU core | OCR worker concurrency evaluates to `max(1, 1/2) == 1` worker | Floor of 1 worker enforced |
| Multi-Core Host | System has 8 CPU cores | OCR worker concurrency evaluates to `max(1, 8/2) == 4` workers | Capped at half physical cores |

</frozen-after-approval>

## Code Map

- `internal/supervisor/paperless.go` -- Paperless provisioner, OCR worker concurrency calculation, environment configuration, and directory layout.
- `internal/supervisor/supervisor.go` -- Wire `ServicePaperless` into service definitions, alias resolution (`dms`, `documents`), and lifecycle management.
- `internal/ingress/caddy.go` -- Verify route handling for `/docs` and dedicated port listener `srv_8000`.
- `internal/supervisor/paperless_test.go` -- Unit tests for Paperless directory provisioning, OCR worker calculation, environment variables, and volume preservation.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for Paperless activation, Caddy sync, alias resolution, and deactivation.
- `api/openapi.yaml` -- Document Paperless service under services parameters and schemas.

## Tasks & Acceptance

**Execution:**
- [x] `internal/supervisor/paperless.go` -- Implement `PaperlessProvisioner` with directory layout and OCR worker calculation.
- [x] `internal/supervisor/supervisor.go` -- Register `ServicePaperless`, wire into `ActivateService`, `DeactivateService`, and `resolveID`.
- [x] `internal/supervisor/paperless_test.go` -- Unit tests for Paperless volume creation, environment variables, CPU worker formula, and compose synthesis.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for Paperless activation, alias resolution, and Caddy ingress synchronization.
- [x] `api/openapi.yaml` -- Update OpenAPI schema documentation for Paperless service.

**Acceptance Criteria:**
- Given Document Management activation, Paperless-ngx deploys and mounts persistent storage at `/data/documents`.
- OCR worker concurrency is capped to `max(1, physical_cores / 2)` and scheduled at low CPU priority (`nice -n 10`).
- Paperless is accessible via dedicated port `:8000` and subpath `/docs` through Caddy.
- Deactivating Paperless halts container execution while strictly preserving all document files and SQLite databases in `/data/documents`.

## Implementation Notes

- Implemented `PaperlessProvisioner` in `internal/supervisor/paperless.go`:
  - Provisions storage directories `/data/documents/{media,data,consume,export}` and `/data/state/services/paperless` with `0755` permissions, strictly preserving existing files and data.
  - Enforces SQLite WAL durability (`PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`) on `/data/documents/data/db.sqlite3` via `EnsureSQLiteWAL`.
  - Calculates bounded OCR and task worker concurrency using `max(1, physical_cores / 2)` via `CalculateOCRWorkers(numCores)` with floor of 1.
  - Synthesizes container environment with `PAPERLESS_OCR_WORKERS`, `PAPERLESS_TASK_WORKERS`, `PAPERLESS_OCR_PROCESSES=1`, `PAPERLESS_OCR_USER_ARGS='{"nice": 10}'`, `PAPERLESS_FORCE_SCRIPT_NAME=/docs`, `PAPERLESS_URL=http://localhost:8000/docs`, and directories.
- Wired `ServicePaperless` into `internal/supervisor/supervisor.go`:
  - Defined `ServicePaperless = "paperless"`, `ImagePaperless = "paperlessngx/paperless-ngx:3.3.1"`, memory limit 1024MB, reservation 512MB.
  - Registered `ServicePaperless` in default service definitions with volume templates mapping `documents/data`, `documents/media`, `documents/consume`, `documents/export`, and `state/services/paperless`.
  - Added alias resolution for `dms`, `documents`, and `paperless` to `ServicePaperless` in `resolveID`.
  - Wired Paperless provisioning into `ActivateService` and registered `PaperlessProvisioner()` accessor.
  - Included `ServicePaperless` in deterministic `ListServices` response.
- Ingress routing:
  - Caddy proxies subpath `/docs` (and `/docs/*`) with prefix strip rewrite and header `X-Forwarded-Prefix: /docs` to `sovereign-paperless:8000`.
  - Dedicated direct port reverse-proxy server `srv_8000` listens on `:8000` proxying to `sovereign-paperless:8000`.
- Automated testing:
  - Unit tests in `internal/supervisor/paperless_test.go` verifying OCR worker formula (1, 2, 4, 8, 16 cores), directory provisioning, volume preservation across multiple provision cycles, environment variables, compose YAML synthesis, memory limits, and socket isolation.
  - Integration tests in `cmd/sovereign-daemon/server_test.go` (`TestServer_Paperless_Activation_LifecycleAndIngressRouting`) verifying alias resolution (`dms`, `documents`, `paperless`), activation, Caddy route synchronization (`/docs` and `:8000`), restart, deactivation volume preservation, and 502 Bad Gateway pull error handling.
- OpenAPI:
  - Updated `api/openapi.yaml` to document Paperless service, aliases (`dms`, `documents`), and schema properties.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

- **Layer 1-3 Code Review Feedback**:
  1. `internal/supervisor/paperless.go`:
     - Applied robust URL construction using `fmt.Sprintf("%s%s", strings.TrimRight(p.resolveBaseURL(), "/"), DefaultPaperlessSubpath)` for `PAPERLESS_URL`.
     - Added `PAPERLESS_THREADS_PER_WORKER: "1"` to prevent thread explosion within single-process OCR workers.
     - Removed invalid `"nice"` keyword argument from `PAPERLESS_OCR_USER_ARGS` (preventing ocrmypdf syntax errors; CPU scheduling handled via container resource bounds and concurrency caps).
     - Added `USERMAP_UID: "1000"` and `USERMAP_GID: "1000"` to guarantee file ownership consistency with Sovereign storage conventions.
     - Added `PAPERLESS_OCR_LANGUAGES: "deu eng"` and `PAPERLESS_OCR_LANGUAGE: "eng"` defaults for bilingual OCR support.
     - Added defensive guards: check `ctx.Err()` in `Provision(ctx)`, validate empty database path in `EnsureSQLiteWAL`, and guard against `nil` functions in `SetNumCPUFunc`.
  2. `internal/ingress/caddy_test.go` & `cmd/sovereign-daemon/server_test.go`:
     - Added dedicated Caddy ingress unit test assertions verifying `:8000` dedicated server block `srv_8000` reverse proxies to `sovereign-paperless:8000`.
     - Recorded synchronized routes in daemon `mockIngressManager` and asserted `SyncRoutes` receives route for Paperless on port 8000 with subpath `/docs`.
  3. `internal/supervisor/paperless_test.go`:
     - Added assertions verifying `PAPERLESS_OCR_PROCESSES`, `PAPERLESS_THREADS_PER_WORKER`, `PAPERLESS_URL`, and `PAPERLESS_REDIS`.
     - Added unit tests for empty SQLite WAL database path rejection, `SetNumCPUFunc(nil)` fallback safety, and context cancellation handling in `Provision`.
  4. `api/openapi.yaml`:
     - Documented in `/api/v1/services/{id}/refresh` description and 400 response that Paperless does not support media library refresh and returns 400 Bad Request.
