---
title: 'Story 2.3: In-House Video Streaming Integration (Jellyfin)'
type: 'feature'
created: '2026-10-15'
status: 'done'
route: 'dispatch'
baseline_commit: 'd5f2f9c9ab36e180faf4379e9aec98fa75b08899'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Alex and the household want high-definition video streaming to Smart TVs and mobile clients from their local media collection without manual server setup, transcode lag, or complex network configuration.

**Approach:** Integrate turnkey Jellyfin video streaming into `sovereign-daemon` supervisor with automated library mounts (`/data/media/movies`, `/data/media/tv`), pre-configured direct-play streaming preferences, hardware acceleration detection (Intel QuickSync, Nvidia NVENC, VAAPI), DLNA and client auto-discovery broadcast, dynamic Caddy ingress routing (`:8096` and `/media`), and library refresh webhook support.

## Boundaries & Constraints

**Always:**
- Use official pinned image `jellyfin/jellyfin:12.2.0`.
- Automatically provision library paths `/data/media/movies` and `/data/media/tv` with proper permissions.
- Pre-configure Jellyfin system and encoding configs (`system.xml`, `encoding.xml`) with direct-play prioritized as the primary stream path without GPU transcoding overhead.
- Detect host hardware acceleration devices (`/dev/dri` for Intel QSV / VAAPI, Nvidia drivers for NVENC) and pass devices conditionally to the container.
- Enable local discovery: native Jellyfin discovery (port 7359/udp) and DLNA (port 1900/udp).
- Dynamically register Caddy ingress route for subpath `/media` and port `:8096` with `X-Forwarded-Prefix: /media`.
- Implement Jellyfin library refresh trigger (`POST /api/v1/services/jellyfin/refresh` or internal trigger calling Jellyfin API).
- Provide automated tests with mock engine verifying library directory creation, compose generation with devices and ports, direct-play configuration, and Caddy ingress sync.

**Never:**
- Never force GPU transcoding on compatible direct-play media formats.
- Never mount host root filesystem or host Docker socket inside Jellyfin container.
- Never delete or alter `/data/media/movies` or `/data/media/tv` contents during restart or upgrade.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Activation | `POST /api/v1/services/jellyfin/activate` | Creates `/data/media/movies` & `tv`, pre-configures direct-play XML configs, launches container, syncs Caddy route `/media` | Emits SSE progress; 502 on pull failure |
| Direct-Play Config | Initial config generation | `encoding.xml` configured with `EnableAudioVbr: true`, direct stream enabled; hardware fallback set | Reuses existing config if present |
| Hardware Detection | System has `/dev/dri/renderD128` | Container spec includes device mount `/dev/dri:/dev/dri` and enables VAAPI/QSV fallback | Gracefully omits device if absent |
| Ingress Routing | Jellyfin active | Caddy proxies `/media/*` with prefix stripping to `sovereign-jellyfin:8096`, and port 8096 direct | Ingress status reports healthy |
| Library Refresh | `POST /api/v1/services/jellyfin/refresh` | Dispatches refresh call to Jellyfin API; returns 200 OK | Returns 404 or 503 if service inactive |

</frozen-after-approval>

## Code Map

- `internal/supervisor/jellyfin.go` -- Jellyfin service specialized provisioner, direct-play config generator, hardware acceleration probe, and library scan trigger.
- `internal/supervisor/supervisor.go` -- Integrate Jellyfin provisioning into service activation and deactivation.
- `internal/supervisor/jellyfin_test.go` -- Unit tests for Jellyfin config generation, hardware device detection, volume creation, and Caddy ingress route generation.
- `internal/ingress/caddy.go` -- Ensure dynamic addition of service spoke routes on activation.
- `cmd/sovereign-daemon/server.go` -- Register `/api/v1/services/jellyfin/refresh` route and ingress sync on service activation.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for Jellyfin activation, library refresh, and Caddy routing.

## Tasks & Acceptance

**Execution:**
- [x] `internal/supervisor/jellyfin.go` -- Implement Jellyfin config generator, library layout provisioner, and hardware acceleration detection -- Provide direct-play config and hardware acceleration fallback.
- [x] `internal/supervisor/supervisor.go` -- Wire Jellyfin provisioner into `ActivateService` and `DeactivateService` -- Ensure media paths and config exist before start.
- [x] `internal/supervisor/jellyfin_test.go` -- Unit tests for Jellyfin config, volume mounts, hardware device mapping, and refresh dispatch -- Verify acceptance criteria.
- [x] `cmd/sovereign-daemon/server.go` -- Add `POST /api/v1/services/{id}/refresh` endpoint and trigger Caddy route sync on service activation -- Connect API and ingress.
- [x] `cmd/sovereign-daemon/server_test.go` -- Test Jellyfin activation lifecycle and refresh endpoint -- Ensure end-to-end integration.

**Acceptance Criteria:**
- Given video streaming activation, `/data/media/movies` and `/data/media/tv` are automatically mounted as pre-configured library paths.
- Jellyfin configuration defaults to direct-play without GPU transcoding overhead for compatible media.
- Hardware acceleration (Intel QuickSync, Nvidia NVENC, VAAPI) is auto-detected and configured as fallback.
- Jellyfin is accessible via port `:8096` and subpath `/media` through Caddy.
- Library refresh trigger endpoint responds successfully.

## Implementation Notes

- Implemented `JellyfinProvisioner` in `internal/supervisor/jellyfin.go` with automated layout creation (`/data/media/movies`, `/data/media/tv`, `/data/state/services/jellyfin/config`, `/data/state/services/jellyfin/cache`).
- Generated `system.xml` and direct-play prioritized `encoding.xml` configuration, preserving any existing configuration files without overwriting.
- Added hardware acceleration detection (`DetectHardwareAcceleration`) supporting Intel QSV (`0x8086`), AMD VAAPI (`0x1002`), and Nvidia NVENC (`/dev/nvidia*`), gracefully omitting devices when absent.
- Extended `ContainerSpec` and `SynthesizeComposeSpec` to support UDP ports (`7359/udp`, `1900/udp`) and device mappings (`/dev/dri:/dev/dri`).
- Updated Caddy configuration generator to map direct spoke ports (`:8096`) alongside subpaths (`/media`) with `X-Forwarded-Prefix: /media`.
- Added `POST /api/v1/services/{id}/refresh` endpoint in `cmd/sovereign-daemon/server.go` with 503 handling for inactive services and 404 for unknown services.
- Documented `/api/v1/services/{id}/refresh` in `api/openapi.yaml`.
- Added unit and integration tests in `internal/supervisor/jellyfin_test.go` and `cmd/sovereign-daemon/server_test.go`.

## Spec Change Log

## Review Triage Log

- **Iteration 1 (2026-10-15)**:
  - *Finding 1 (Setup Wizard Access)*: Default `IsStartupWizardCompleted` changed to `false` in `system.xml` template so administrator first-run account setup wizard can complete on clean installation.
  - *Finding 2 (Hardware Acceleration Robustness)*: Prioritized Nvidia GPU detection over DRI; added Nvidia DRM vendor `10de` detection; mapped individual character device nodes (`/dev/dri/renderD128`, `/dev/dri/card0`) instead of whole directory; handled empty DRI folder gracefully returning `HWAccelNone`.
  - *Finding 3 (Refresh Validation & Problem Details)*: Added `ErrInvalidOperation` in supervisor package when invoking refresh on unsupported services; mapped to HTTP 400 Bad Request with ProblemDetails in daemon HTTP API; added `400` response to `api/openapi.yaml`.
  - *Finding 4 (Ingress Sync on Daemon Boot)*: Fixed background route synchronization loop in `StartBackgroundServices` to call `s.syncIngressRoutes(ctx)` instead of passing `nil` routes.
  - *Finding 5 (Admin Port Ingress Conflict Guard)*: Excluded port 2019 from spoke listener generation in `internal/ingress/caddy.go` to prevent clashing with Caddy admin API.
  - *Finding 6 (Refresh Trigger Fallback & Safety)*: Added nil safety and zero-value URL guard with `SOVEREIGN_JELLYFIN_URL` env fallback in `TriggerLibraryRefresh`.
  - *Finding 7 (Test Coverage Expansions)*: Added tests for port 2019 exclusion, direct spoke listener structure, Nvidia DRM vendor detection, empty DRI handling, Jellyfin UDP container ports, and unsupported service refresh returning HTTP 400 Bad Request.
  - *Finding 8 (Verification & Sprint Updates)*: Verified all packages (`go test -count=1 ./...` passing 100%), updated spec status to `done`, and marked `2-3-in-house-video-streaming-integration-jellyfin: done` in `sprint-status.yaml`.

## Design Notes

- Jellyfin library layout: `/data/media/movies` (Movies), `/data/media/tv` (Television Series).
- Jellyfin config path: `/data/state/services/jellyfin/config`.
- Jellyfin cache path: `/data/state/services/jellyfin/cache`.
- Ports: `8096` (HTTP), `7359/udp` (Auto-Discovery), `1900/udp` (DLNA).
