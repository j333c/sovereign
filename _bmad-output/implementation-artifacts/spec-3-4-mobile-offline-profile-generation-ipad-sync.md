---
title: 'Story 3.4: Mobile Offline Profile Generation & iPad Sync'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '0d026a47f06cab287bd22b87fa5e2acffc525638'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Alex and household family members travelling with iPads and smartphones want to download movies for offline viewing during flights and road trips. Downloading full-bitrate 20GB–50GB master files exhausts mobile device storage and triggers slow, real-time on-the-fly server transcoding that buffers or drains the server's CPU/GPU.

**Approach:** Extend the off-peak transcoding pipeline in `internal/transcode/` to automatically generate space-efficient, direct-play mobile profiles (`mobile-720p` and `mobile-1080p` with bounded bitrates and faststart MP4 containers) saved alongside master files following Jellyfin's multi-version naming scheme (e.g., `<Title> - 720p.mp4` / `<Title> - 1080p.mp4`). When a mobile user taps "Download" in the Jellyfin client, the pre-encoded mobile profile transfers directly over the local network in seconds with zero server transcoding overhead.

## Boundaries & Constraints

**Always:**
- Implement standardized mobile profiles:
  - `mobile-720p`: 720p max resolution (`scale=-2:720` maintaining aspect ratio), bounded bitrate (~2.5 Mbps, `-maxrate 3000k -bufsize 5000k`), stereo AAC (`-c:a aac -b:a 128k -ac 2`), `-movflags +faststart`.
  - `mobile-1080p`: 1080p max resolution (`scale=-2:1080` maintaining aspect ratio), bounded bitrate (~4.5 Mbps, `-maxrate 5500k -bufsize 9000k`), stereo AAC (`-c:a aac -b:a 192k -ac 2`), `-movflags +faststart`.
- Name mobile profiles alongside master media following Jellyfin multi-version naming conventions (e.g., `/data/media/movies/<Title> (<Year>)/<Title> (<Year>) - 720p.mp4`), preserving the master video intact.
- Enforce off-peak schedule: mobile profile generation runs during off-peak night hours (01:00–07:00) or when manual override is enabled.
- Enforce the 10% storage capacity headroom floor before and during mobile profile encoding.
- Trigger an immediate Jellyfin library refresh (`RefreshService(ctx, "jellyfin")`) upon mobile profile completion so Jellyfin registers the mobile version.
- Expose REST API endpoint (`POST /api/v1/transcode/mobile`) to enqueue mobile profile generation for a movie or TV episode.
- Provide automated unit and integration tests verifying mobile profile generation, parameter scaling, multi-version naming, headroom protection, and Jellyfin refresh invocation.

**Never:**
- Never delete or modify the master video file when generating mobile profiles.
- Never exceed 1080p resolution or unconstrained bitrates for mobile profiles.
- Never mount the host Docker socket into mobile transcoding workers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Mobile 720p Enqueue | `POST /api/v1/transcode/mobile` with `target_profile: "mobile-720p"` | Enqueues job in `transcode_queue`; target path formatted as `<Base> - 720p.mp4` | 400 on invalid path or unknown profile |
| Mobile 1080p Enqueue | `POST /api/v1/transcode/mobile` with `target_profile: "mobile-1080p"` | Enqueues job in `transcode_queue`; target path formatted as `<Base> - 1080p.mp4` | 400 on invalid path |
| Off-Peak Mobile Execution | Night window active; mobile job queued | Encodes video with 720p/1080p scaling, bounded bitrate, AAC stereo; creates faststart MP4; triggers Jellyfin refresh | Retries on transient failure |
| Pre-Existing Mobile Profile | Target `<Base> - 720p.mp4` already exists on disk | Skips redundant encode or validates existing file; returns existing status | Prevents duplicate re-encoding |
| Low Storage Headroom (<10%) | Free storage space is < 10% | Pauses mobile profile generation; emits storage alert | Prevents disk exhaustion |
| Jellyfin Mobile Sync Client | Client requests download for title | Jellyfin serves direct-play pre-encoded profile without real-time transcode | Stream delivered directly |

</frozen-after-approval>

## Code Map

- `internal/transcode/profiles.go` -- Mobile profile definitions (`mobile-720p`, `mobile-1080p`), FFmpeg scaling/bitrate parameters, and multi-version filename resolver.
- `internal/transcode/transcoder.go` -- Update FFmpeg argument builder to apply mobile resolution scaling, bounded bitrates, and stereo AAC.
- `internal/transcode/engine.go` -- Support mobile profile batch generation and enqueue helpers.
- `internal/transcode/mobile_test.go` -- Unit tests for mobile profile generation, scaling arguments, multi-version naming, and duplicate avoidance.
- `cmd/sovereign-daemon/server.go` -- Register `POST /api/v1/transcode/mobile` endpoint with admin auth and input validation.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for mobile profile endpoint and Jellyfin refresh integration.
- `api/openapi.yaml` -- Document `POST /api/v1/transcode/mobile` in OpenAPI schema.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transcode/profiles.go` -- Implement mobile profile definitions and Jellyfin multi-version filename generator.
- [x] `internal/transcode/transcoder.go` -- Implement video scaling and bitrate capping for `mobile-720p` and `mobile-1080p`.
- [x] `internal/transcode/engine.go` -- Add mobile profile enqueue logic to `TranscodeEngine`.
- [x] `internal/transcode/mobile_test.go` -- Unit tests for mobile profile parameters, filename conventions, and duplicate skip logic.
- [x] `cmd/sovereign-daemon/server.go` -- Register `POST /api/v1/transcode/mobile` endpoint.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for mobile profile generation and Jellyfin refresh webhook.
- [x] `api/openapi.yaml` -- Document `POST /api/v1/transcode/mobile` in OpenAPI specification.

**Acceptance Criteria:**
- Given an active video title in the library, when the off-peak queue runs, the encoder generates an optimized mobile direct-play profile (720p/1080p, bounded bitrate) alongside the master file.
- Mobile profile files follow Jellyfin multi-version naming conventions without modifying or deleting the master file.
- When tapping "Download" in the Jellyfin mobile app, the pre-encoded mobile profile transfers directly without requiring real-time server transcoding.
- Completed mobile encodes trigger an immediate Jellyfin library refresh.

## Implementation Notes

- **Mobile Profile Definitions & Filename Resolution (`internal/transcode/profiles.go`)**:
  - Implemented `ProfileMobile720p` (`scale=-2:'min(720,ih)'`, bounded 2.5 Mbps with `-maxrate 3000k -bufsize 5000k`, 128k stereo AAC, faststart MP4) and `ProfileMobile1080p` (`scale=-2:'min(1080,ih)'`, bounded 4.5 Mbps with `-maxrate 5500k -bufsize 9000k`, 192k stereo AAC, faststart MP4).
  - Implemented `ResolveMultiVersionPath` following Jellyfin's multi-version file naming convention (e.g. `<Title> - 720p.mp4` / `<Title> - 1080p.mp4`) placed directly alongside the master file in the library, stripping existing quality suffixes iteratively to prevent duplicate suffix stacking, and preventing self-overwrite.
- **FFmpeg Argument Builder (`internal/transcode/transcoder.go`)**:
  - Updated `BuildFFmpegArgs` to apply resolution scaling (`-vf scale=-2:'min(720,ih)'` / `-vf scale=-2:'min(1080,ih)'`), bounded video bitrates, and stereo AAC audio. Dedicated VAAPI surface scaling filter `scale_vaapi=w=-2:h='min(...,ih)'` ensures hardware VAAPI surface compatibility. Enforces universal direct-play H.264 video codec across QSV, NVENC, VAAPI, and software CPU encoders. Removed conflicting `-crf` flag for software encoding when constrained bitrate flags are present.
- **Transcode Persistence Store (`internal/transcode/store.go`)**:
  - Added `FindJobByTarget` method and `idx_transcode_queue_target` index to SQLiteStore for rapid lookup by target path.
- **Transcode Engine (`internal/transcode/engine.go`)**:
  - Added `EnqueueMobile` method with full path and profile validation, pre-existing profile detection, and deduplication across queued/processing/paused states.
  - Implemented duplicate encode skipping in `executeJob` when the target profile already exists and is non-empty on disk, preserving master media intact and dispatching Jellyfin library refresh webhooks.
  - Resolved concurrency locking and dispatch deadlocks by running Jellyfin library refresh outside the `e.mu` critical section.
  - Integrated mobile target path resolution into `Enqueue`.
- **API Endpoint (`cmd/sovereign-daemon/server.go`)**:
  - Registered `POST /api/v1/transcode/mobile` with admin authentication (`requireSetupOrAdmin`) and input validation.
  - Returns `201 Created` for newly enqueued jobs, `200 OK` for pre-existing profiles or duplicate pending requests, and RFC 7807 problem details on error (HTTP 400 for validation errors, HTTP 500 for internal store errors).
- **OpenAPI 3.1 Documentation (`api/openapi.yaml`)**:
  - Documented `POST /api/v1/transcode/mobile` and added `EnqueueMobileRequest` schema, including HTTP 200, 201, 400, 401, 403, and 500 status codes.
- **Automated Verification (`internal/transcode/mobile_test.go` & `cmd/sovereign-daemon/server_test.go`)**:
  - Unit tests covering mobile profile parameters, Jellyfin multi-version naming, FFmpeg flags (NVENC, VAAPI, QSV, software no-CRF), non-upscaling bound filters, stacked suffix stripping, master file preservation, duplicate skip avoidance, and storage headroom floor enforcement.
  - Integration tests covering REST API validation, 720p/1080p enqueuing, duplicate pending enqueuing, pre-existing skip (200 OK), auth gating post-setup (401 vs 200), end-to-end background execution, and Jellyfin library refresh dispatching.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

- **Non-Upscaling Bound Filters & VAAPI Surface Scaling**: Changed resolution filters to `scale=-2:'min(720,ih)'` and `scale=-2:'min(1080,ih)'` to prevent low-resolution inputs from upscaling. Added `VAAPIScaleFilter` (`scale_vaapi=w=-2:h=...`) to properly scale VAAPI hardware surfaces without software pipeline failures.
- **Software Encoder Rate-Control**: Eliminated conflicting `-crf` argument in `BuildFFmpegArgs` when bounded bitrate parameters (`-b:v`, `-maxrate`, `-bufsize`) are set for software `libx264`.
- **Iterative Suffix Stripping**: In `ResolveMultiVersionPath`, implemented an iterative stripping loop with case-insensitivity to handle stacked suffixes (e.g., `Movie - 720p - 720p.mp4`).
- **Encapsulation & Shared Sentinels**: Encapsulated `mobileProfiles` map with `GetMobileProfile`, and exported shared sentinels (`ErrUnsupportedVideoFormat`, `ErrSourceOutsideDataDir`, `ErrSourceNotFound`, `ErrSourceIsDir`).
- **Concurrency & Deadlock Resolution**: Added mutex protection around target lookup/job insertion, deduplication across `StatusQueued`, `StatusProcessing`, and `StatusPaused` states, and refactored critical sections so background Jellyfin refresh dispatch runs outside `e.mu.Lock()`.
- **API Error Code Differentiation**: Differentiated validation errors (HTTP 400) from store errors (HTTP 500) in `handleEnqueueMobile`. Added HTTP 401 and 403 responses in `api/openapi.yaml`.
- **Test Coverage Expansion**: Added comprehensive tests verifying NVENC/VAAPI argument generation, software VBR rate-control, stacked suffix cleanup, synthetic job completion on pre-existing disk targets, deduplication across pending states, and single-entry queue preservation in daemon integration tests.
