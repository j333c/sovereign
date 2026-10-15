---
title: 'Story 2.4: Music & Audiobook Streaming Integration (Navidrome & Audiobookshelf)'
type: 'feature'
created: '2026-10-15'
status: 'review'
route: 'dispatch'
baseline_commit: 'ffdc94178cc3016f19fc8b652ce9576c95a6db28'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Household members listening to music playlists and audiobook collections need dedicated streaming backends that integrate with standard mobile clients (Substreamer, Symfonium, DSub, Audiobookshelf mobile apps) with zero manual server configuration, instant playback, chapter marker preservation, and multi-device sync.

**Approach:** Integrate Navidrome and Audiobookshelf into `sovereign-daemon` supervisor with automated library directory provisioning (`/data/media/music`, `/data/media/audiobooks`), persistent state and metadata management (`/data/state/services/navidrome/data`, `/data/state/services/audiobookshelf/config`, `/data/state/services/audiobookshelf/metadata`), dynamic Caddy ingress routing on dedicated ports (`:4533` for Navidrome, `:13378` for Audiobookshelf) and subpaths (`/music`, `/audiobooks`), Subsonic API compatibility, library refresh triggers, and individual or bundled audio activation.

## Boundaries & Constraints

**Always:**
- Use official pinned images: `deluan/navidrome:0.54.5` and `advplyr/audiobookshelf:2.19.5`.
- Automatically provision library directories `/data/media/music` and `/data/media/audiobooks` with `0755` permissions, strictly preserving existing files.
- Automatically provision state and configuration volumes (`/data/state/services/navidrome/data`, `/data/state/services/audiobookshelf/config`, `/data/state/services/audiobookshelf/metadata`) with `0755` permissions.
- Mount `/data/media/music` as read-only (`:ro`) into Navidrome at `/music`.
- Mount `/data/media/audiobooks` into Audiobookshelf at `/audiobooks`.
- Preserve Audiobookshelf chapter markers, narration speed settings, listening progress, and user database in `/config` and `/metadata` across container restarts and updates.
- Expose Navidrome web player and Subsonic API via dedicated port `:4533` and subpath `/music` through Caddy.
- Expose Audiobookshelf via dedicated port `:13378` and subpath `/audiobooks` through Caddy.
- Support library refresh triggers via `POST /api/v1/services/{id}/refresh` for both `navidrome` and `audiobookshelf`.
- Provide automated unit and integration tests verifying directory creation, compose spec generation, volume mounts, port bindings, Caddy ingress synchronization, and library refresh handling.

**Never:**
- Never delete or alter user media in `/data/media/music` or `/data/media/audiobooks` during container stops, restarts, or updates.
- Never mount the host Docker socket (`/var/run/docker.sock`) into audio streaming containers.
- Never expose unauthenticated direct host bindings that bypass Caddy when Caddy ingress is enabled.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Navidrome Activation | `POST /api/v1/services/navidrome/activate` | Provisions `/data/media/music` & state dir, starts container with `deluan/navidrome:0.54.5`, syncs Caddy routes (`/music`, port 4533) | 502 on pull failure; emits SSE progress |
| Audiobookshelf Activation | `POST /api/v1/services/audiobookshelf/activate` | Provisions `/data/media/audiobooks` & config/metadata dirs, starts container with `advplyr/audiobookshelf:2.19.5`, syncs Caddy routes (`/audiobooks`, port 13378) | 502 on pull failure; emits SSE progress |
| Audio Bundle Activation | `POST /api/v1/services/audio/activate` | Activates audio streaming services via alias resolution | Graceful error if service fails to launch |
| Library Scan Trigger | `POST /api/v1/services/{navidrome\|audiobookshelf}/refresh` | Triggers background library scan; returns 200 OK | Returns 503 if inactive; 404 if unknown service |
| Ingress Routing | Active services | Caddy proxies `/music` -> `sovereign-navidrome:4533` and `/audiobooks` -> `sovereign-audiobookshelf:13378`, plus direct port listeners `srv_4533` and `srv_13378` | Ingress status lists active spoke routes |
| Subsonic API Client | Substreamer client connects to `:4533/rest/*` | Navidrome responds with Subsonic API protocol responses | Authentication handled by Navidrome |

</frozen-after-approval>

## Code Map

- `internal/supervisor/audio.go` -- Specialized provisioners and refresh handlers for Navidrome and Audiobookshelf.
- `internal/supervisor/supervisor.go` -- Wire Navidrome and Audiobookshelf provisioners into `ActivateService`, `RefreshService`, and alias resolution.
- `internal/supervisor/docker.go` -- Ensure environment variables from `ContainerSpec.Env` are passed to Docker container create payload and compose file synthesis.
- `internal/ingress/caddy.go` -- Verify route handling for `/music` and `/audiobooks` with dedicated port listeners (`:4533`, `:13378`).
- `internal/supervisor/audio_test.go` -- Unit tests for audio services provisioning, volume persistence, and refresh triggers.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for Navidrome and Audiobookshelf activation, Caddy sync, and refresh endpoints.

## Tasks & Acceptance

**Execution:**
- [x] `internal/supervisor/audio.go` -- Implement `AudioProvisioner` for Navidrome and Audiobookshelf directory provisioning and library refresh hooks.
- [x] `internal/supervisor/docker.go` -- Wire `ContainerSpec.Env` into Docker container creation API payload.
- [x] `internal/supervisor/supervisor.go` -- Integrate audio provisioners into `ActivateService`, configure environment variables, and implement `RefreshService` for `navidrome` and `audiobookshelf`.
- [x] `internal/ingress/caddy.go` -- Verify and ensure subpath routing and direct port listeners for `:4533` and `:13378`.
- [x] `internal/supervisor/audio_test.go` -- Unit tests for Navidrome & Audiobookshelf volume layout, config persistence, compose spec synthesis, and refresh hooks.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for Navidrome and Audiobookshelf activation, ingress status, and refresh endpoints.

**Acceptance Criteria:**
- Given Audio Streaming activation, Navidrome mounts `/data/media/music` and Audiobookshelf mounts `/data/media/audiobooks`.
- Navidrome exposes web player and Subsonic API on dedicated port `:4533` and subpath `/music`.
- Audiobookshelf exposes port `:13378` and subpath `/audiobooks`, preserving chapter markers, narration speed settings, and multi-device sync in persistent volumes.
- Library refresh triggers on `POST /api/v1/services/navidrome/refresh` and `POST /api/v1/services/audiobookshelf/refresh` succeed when active and return 503 when inactive.

## Implementation Notes

- Implemented `AudioProvisioner` in `internal/supervisor/audio.go` managing automated library and state directory provisioning (`/data/media/music`, `/data/media/audiobooks`, `/data/state/services/navidrome/data`, `/data/state/services/audiobookshelf/config`, `/data/state/services/audiobookshelf/metadata`) with `0755` permissions, strictly preserving user media and metadata across restarts.
- Synthesized runtime environment variables for Navidrome (`ND_MUSICFOLDER`, `ND_DATAFOLDER`, `ND_PORT`, `ND_LOGLEVEL`) and Audiobookshelf (`PORT: 13378`, `CONFIG_PATH`, `METADATA_PATH`, `AUDIOBOOKSHELF_UID: 0`, `AUDIOBOOKSHELF_GID: 0`).
- Implemented library refresh webhook dispatchers for Navidrome (`POST /api/scan`) and Audiobookshelf (`POST /api/libraries/scan`) with configurable base URLs and mock callbacks for deterministic testing.
- Wired `ContainerSpec.Env` into the Docker engine container creation payload (`createPayload["Env"]`) as a deterministically sorted array in `internal/supervisor/docker.go`.
- Integrated `AudioProvisioner` into `ServiceManager.ActivateService` in `internal/supervisor/supervisor.go`, ensuring volume paths exist, environment variables are populated into compose synthesis, and containers start with memory limit bounds (`512MB` limit, `256MB` reservation).
- Extended `RefreshService` and `resolveID` in `supervisor.go` to handle `navidrome` (`music`), `audiobookshelf` (`audiobooks`), and bundled audio activation/refresh via `audio`.
- Verified dynamic Caddy ingress routing in `internal/ingress/caddy.go` and added `TestCaddyClient_AudioRoutes_MusicAndAudiobooks` confirming `/music` and `/audiobooks` subpath proxying (with prefix stripping and `X-Forwarded-Prefix`) as well as dedicated port listeners `srv_4533` and `srv_13378`.
- Added unit tests in `internal/supervisor/audio_test.go` covering directory provisioning, volume preservation, Docker socket mount exclusion, HTTP refresh webhooks, compose YAML generation, bundle activation, and lifecycle refresh states.
- Added integration tests in `cmd/sovereign-daemon/server_test.go` (`TestServer_AudioServices_ActivationAndRefresh`, `TestServer_AudioBundle_ActivationAndRefresh`) testing the complete lifecycle over HTTP, Caddy route table synchronization, and error handling (503 inactive, 404 unknown, 502 pull failure).
- Updated `/api/v1/services/{id}/refresh` description in `api/openapi.yaml`.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

*(To be populated during code review)*
