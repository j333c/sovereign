---
title: 'Story 2.1: On-Demand Container Lifecycle Engine & Health Probing'
type: 'feature'
created: '2026-10-15'
status: 'done'
route: 'dispatch'
baseline_commit: 'd7c620ed031d7bc217987c01640ee732e4216c02'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Inactive services must not consume appliance disk space or network bandwidth during initial setup, while active containers need strict memory bounds and watchdog debouncing to prevent transient resource spikes from causing false-positive restart loops.

**Approach:** Implement an on-demand container lifecycle engine in `/internal/supervisor` that pulls official OCI images only upon explicit activation, streams pull progress over SSE, enforces bounded memory reservations and limits, runs watchdog-debounced health probes (3 failures over 60s before degrading), and halts runtime while preserving data volumes on deactivation.

## Boundaries & Constraints

**Always:**
- Pull images strictly from official upstream project registries: `jellyfin/jellyfin:12.2.0`, `deluan/navidrome:0.54.5`, `advplyr/audiobookshelf:2.19.5`.
- Never pre-download images during appliance setup or boot; images are pulled on-demand on explicit activation.
- Enforce memory limit (`mem_limit`) and soft reservation limits on running service containers.
- Watchdog dampening requires 3 consecutive probe failures over at least a 60-second window before escalating status to Amber degraded or attempting automated container restart.
- Deactivation stops containers and removes runtime instances without deleting or pruning volumes under `/data`.
- Support mockable runtime engine so all unit and integration tests run deterministically without requiring a live `/var/run/docker.sock` in test environments.
- Expose REST API under `/api/v1/services` and stream real-time events over SSE `/api/events` with RFC 7807 problem details on errors.

**Never:**
- Never expose or mount `/var/run/docker.sock` inside client web or proxy containers.
- Never delete `/data/media` or `/data/state` directories during service deactivation or restart.
- Never trigger immediate restart on a single transient health check failure.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| List Services (Initial) | `GET /api/v1/services` | Returns list with jellyfin, navidrome, audiobookshelf with status `inactive` and `image_downloaded: false` | 200 OK |
| On-Demand Activation | `POST /api/v1/services/jellyfin/activate` | Initiates image pull, streams SSE `service.pull_progress`, synthesizes compose spec, starts container, sets status `starting` | Returns 202 Accepted or 200 OK; 404 for unknown service |
| Pull Failure | Network unreachable during pull | Aborts activation, reverts state to `inactive`, emits SSE `service.error` | Returns 502 Bad Gateway with ProblemDetails |
| Watchdog Transient Blip | 1 or 2 probe failures within 60s | Status remains `healthy`; watchdog failure counter increments without restart | Internal logging, no user alarm |
| Watchdog Sustained Failure | 3 consecutive failures over >=60s | Status transitions to `degraded` (Amber), emits SSE `service.health_changed`, triggers recovery restart | Logs warning, updates overall status |
| Deactivation | `POST /api/v1/services/jellyfin/deactivate` | Stops container runtime, sets status `inactive`; `/data/media` and state volumes remain intact | 200 OK; preserves all volumes |
| Restart Action | `POST /api/v1/services/jellyfin/restart` | Restarts container with same config; requires admin auth if configured | 200 OK; 401 if unauthorized |

</frozen-after-approval>

## Code Map

- `internal/supervisor/supervisor.go` -- Core supervisor interface, service definitions, memory limit constants, watchdog debouncer logic.
- `internal/supervisor/docker.go` -- Compose/Docker lifecycle runner with on-demand image puller and mock engine support.
- `internal/supervisor/supervisor_test.go` -- Comprehensive unit tests verifying on-demand pulling, watchdog debouncing (3 failures / 60s), memory limits, and volume preservation.
- `cmd/sovereign-daemon/server.go` -- REST handlers for `/api/v1/services`, activation/deactivation/restart routing, and SSE event streaming.
- `cmd/sovereign-daemon/server_test.go` -- API endpoint tests for service lifecycle, auth requirements, and SSE progress events.
- `api/openapi.yaml` -- OpenAPI 3.1 schema definitions for services endpoints, request/response models, and SSE events.
- `web/src/lib/api/schema.d.ts` -- Generated TypeScript types via `npm --prefix web run codegen`.
- `web/src/lib/api/client.ts` -- Client methods for `getServices`, `activateService`, `deactivateService`, `restartService`.

## Tasks & Acceptance

**Execution:**
- [x] `api/openapi.yaml` -- Define `/api/v1/services` endpoints and data models -- Expose typed contracts for service lifecycle management.
- [x] `web/package.json` & `web/src/lib/api/` -- Run `npm --prefix web run codegen` and add service methods to `client.ts` -- Provide frontend TypeScript bindings.
- [x] `internal/supervisor/supervisor.go` -- Implement ServiceManager, memory bounds, and Watchdog debouncer -- Encapsulate container lifecycle and health dampening.
- [x] `internal/supervisor/supervisor_test.go` -- Unit test on-demand pull, 3-failure 60s watchdog debouncing, and volume preservation -- Validate invariants.
- [x] `cmd/sovereign-daemon/server.go` -- Integrate supervisor into daemon routes and SSE events stream -- Connect REST API and event broadcaster.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for services REST API and SSE telemetry -- Verify contract and auth gating.

**Acceptance Criteria:**
- Given an inactive feature (e.g. video), when querying services, the image is not pre-downloaded (`image_downloaded: false`, `status: inactive`).
- When `POST /api/v1/services/{id}/activate` is invoked, daemon pulls image from official project repository and streams pull progress.
- Containers enforce configured memory bounds (`mem_limit` and soft reservation).
- Watchdog prober requires 3 consecutive probe failures over at least 60 seconds before escalating to degraded or attempting restart.
- Deactivating a service terminates running container instances without removing or pruning `/data` storage volumes.

## Implementation Notes

- Added OpenAPI 3.1 endpoints (`/api/v1/services`, `/api/v1/services/{id}/activate`, `/api/v1/services/{id}/deactivate`, `/api/v1/services/{id}/restart`) and schemas (`ServiceStatus`, `ServicesListResponse`, `ServicePullProgressEvent`, `ServiceHealthChangedEvent`, `ServiceErrorEvent`).
- Regenerated TypeScript types via `openapi-typescript` and implemented `getServices`, `activateService`, `deactivateService`, `restartService` on `SovereignApiClient` with complete test coverage in `web/src/lib/api/client.test.ts`.
- Implemented `internal/supervisor/docker.go` with `RuntimeEngine` interface, `MockRuntimeEngine` for deterministic isolated testing, `DockerEngine` communicating with `/var/run/docker.sock` over Unix socket HTTP, and atomic Compose v2 spec synthesis (`SynthesizeComposeSpec`).
- Implemented `internal/supervisor/supervisor.go` with canonical service definitions (`jellyfin`, `navidrome`, `audiobookshelf`), pinned upstream project OCI images, strict memory limits and soft reservations, watchdog debouncer requiring >= 3 failures over >= 60s, volume directory management, and persistent volume preservation upon container deactivation.
- Connected `ServiceManager` to `cmd/sovereign-daemon/server.go` with event broadcasting over SSE (`/api/events` and `/api/v1/system/events`), REST routing, and admin auth gating on restart actions.
- Verified with unit tests in `internal/supervisor/supervisor_test.go` and integration tests in `cmd/sovereign-daemon/server_test.go`, as well as full frontend test suite.

## Spec Change Log

## Review Triage Log

- **Watchdog Degradation & Recovery Debouncer Reset (`internal/supervisor/supervisor.go`)**: Fixed infinite restart storm by invoking `debouncer.Reset(id)` on recovery restart, checking active status under read-lock before acting, and running container restarts in non-blocking goroutines.
- **Docker Network Attachment & Localhost Binding (`internal/supervisor/docker.go`)**: Added secondary network attachment (`sovereign-ingress`) via Docker Engine API `POST /networks/{network}/connect`, bound host ports explicitly to `127.0.0.1`, and aligned container naming with `sovereign-<name>`.
- **SSE Pull Progress Calculation (`internal/supervisor/docker.go`)**: Implemented multi-layer progress aggregation with monotonic percentage reporting, eliminating erratic oscillations.
- **Error Cleanup & Lifecycle Guards (`internal/supervisor/supervisor.go`)**: Added in-flight activation tracking to prevent concurrent activation races, reverted status to error and broadcast `service.error` on startup failures, and rejected restarts on inactive services with `400 Bad Request`.
- **Test Isolation (`cmd/sovereign-daemon/server_test.go`)**: Explicitly configured `MockRuntimeEngine` in `setupTestServer` to guarantee zero dependency on host Docker daemon.
- **Individual Service Endpoint (`cmd/sovereign-daemon/server.go`, `client.ts`, `api/openapi.yaml`)**: Added `GET /api/v1/services/{id}` endpoint and `getService(id)` client method.
- **Test Coverage Expansion (`internal/supervisor/supervisor_test.go`, `server_test.go`)**: Added unit tests for watchdog loop, debouncer reset, recovery restart, in-flight activation guard, and inactive restart rejection. All 64 frontend tests and all Go package tests pass cleanly.

## Design Notes

- Service IDs: Canonical lowercase slugs `jellyfin`, `navidrome`, `audiobookshelf`.
- Memory limits: Jellyfin (2048MB limit, 1024MB reservation), Navidrome (512MB limit, 256MB reservation), Audiobookshelf (512MB limit, 256MB reservation).
- Watchdog: Ticker every 10-15s, tracking consecutive failures and timestamp of first failure. Only when count >= 3 AND elapsed >= 60s does state transition to degraded.
