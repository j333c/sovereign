---
title: 'Story 3.3: Off-Peak Transcoding Queue & Universal Direct-Play Pre-Encoding'
type: 'feature'
created: '2026-10-16'
status: 'review'
route: 'dispatch'
baseline_commit: '0a884be76e7a25048248d8918028568a43989b8e'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Large raw disc rips (ISOs, raw MPEG-2, VC-1, high-bitrate MKVs) buffer or force heavy real-time GPU transcoding when streamed to Smart TVs and mobile clients, exhausting system VRAM and competing with daytime interactive local AI chat.

**Approach:** Implement an off-peak transcoding queue and universal direct-play pre-encoding engine in `internal/transcode/` that schedules batch video encoding during configured off-peak night hours (default: 01:00 to 07:00), converts raw imports into standardized H.264/H.265 MP4 files with universal stereo/multichannel AAC audio compatibility, pauses or throttles during daytime hours (07:00 to 01:00) to keep GPU compute and VRAM available for AI inference, enforces the 10% disk capacity headroom floor, supports hardware acceleration fallback (Intel QuickSync, Nvidia NVENC, VAAPI, CPU), updates real-time SSE telemetry (`gpu_status`), and exposes management REST API endpoints (`/api/v1/transcode/queue`, `/api/v1/transcode/schedule`, `/api/v1/transcode/override`).

## Boundaries & Constraints

**Always:**
- Default scheduled off-peak window is 01:00 to 07:00 local time (configurable).
- Automatically pause or throttle batch video transcoding during daytime hours (07:00 to 01:00) unless explicitly overridden by user action.
- Pre-encode raw video inputs into universal direct-play MP4 containers with H.264/H.265 video and universal AAC stereo/multichannel audio.
- Automatically detect and leverage host hardware acceleration devices (`/dev/dri` for Intel QSV/VAAPI, Nvidia NVENC via hardware detection), falling back gracefully to software encoding.
- Enforce the 10% storage capacity headroom floor: pause transcoding if free storage drops below 10% to protect SQLite WAL flushes.
- Atomically place completed direct-play encodes into the media library and trigger immediate Jellyfin library refresh.
- Persist queue state across daemon restarts in `sovereign.db` (`transcode_queue` table).
- Provide a `Transcoder` interface with deterministic mock implementations for unit and integration testing without invoking system FFmpeg binaries.
- Connect schedule state to SSE telemetry `gpu_status` so `TelemetryBar` displays daytime/nighttime status accurately.

**Never:**
- Never run heavy batch transcoding during daytime hours without an explicit user manual override.
- Never delete or corrupt the original master file until the new transcode has completed and passed verification.
- Never mount the host Docker socket into transcode workers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Night Window Arrival | Time transitions to 01:00 | Worker wakes; state transitions to `Off-Peak (Transcoding Active)`; starts encoding oldest queued job | Pauses if storage < 10% |
| Daytime Transition | Time transitions to 07:00 | Active batch encoding pauses gracefully; state transitions to `Daytime (AI Priority)`; releases GPU VRAM | Resumes at next night window |
| Enqueue Raw Media | Media added via `POST /api/v1/transcode/queue` | Job queued in database with state `queued`; if off-peak, begins processing | 400 on missing or invalid file path |
| Manual Override | `POST /api/v1/transcode/override` with `force: true` | Bypasses daytime restriction; immediately begins processing active job | Returns 200 with active status |
| Low Storage Headroom (<10%) | Free storage space drops below 10% | Pauses active encoding; records `storage_low` warning; emits telemetry alert | Prevents disk full condition |
| Transcode Completion | Encoding finishes successfully | Validates output; updates library path; triggers Jellyfin refresh; marks job `completed` | Retries on transient failure |
| Hardware Device Failure | Hardware encoder fails mid-encode | Retries with software encoder fallback | Marks `failed` if software fails |

</frozen-after-approval>

## Code Map

- `internal/transcode/scheduler.go` -- Day/Night time window evaluator (default 01:00–07:00), schedule configuration, and manual override tracking.
- `internal/transcode/engine.go` -- Queue manager and encoding orchestrator: job lifecycle, 10% headroom guard, hardware acceleration detection, atomic library replacement, and refresh dispatch.
- `internal/transcode/transcoder.go` -- `Transcoder` interface, FFmpeg command wrapper, and `MockTranscoder`.
- `internal/transcode/store.go` -- SQLite persistence for `transcode_queue` in `sovereign.db`.
- `internal/transcode/transcode_test.go` -- Unit tests for schedule evaluation, day/night transitions, queue processing, headroom guards, override controls, and fallback.
- `cmd/sovereign-daemon/server.go` -- Register `/api/v1/transcode/*` endpoints, wire engine into background services, and update SSE `gpu_status`.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for transcode endpoints, schedule status, and telemetry updates.
- `api/openapi.yaml` -- Document transcode queue, schedule, and override endpoints.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transcode/store.go` -- Implement SQLite storage schema and CRUD for `transcode_queue`.
- [x] `internal/transcode/scheduler.go` -- Implement Day/Night schedule evaluator (01:00-07:00 off-peak) with manual override support.
- [x] `internal/transcode/transcoder.go` -- Implement `Transcoder` interface and `MockTranscoder`.
- [x] `internal/transcode/engine.go` -- Implement `TranscodeEngine` with queue processing, 10% headroom guard, and Jellyfin refresh integration.
- [x] `internal/transcode/transcode_test.go` -- Unit tests for schedule evaluation, queue transitions, mock encoding, and headroom checks.
- [x] `cmd/sovereign-daemon/server.go` -- Register `/api/v1/transcode/queue`, `/api/v1/transcode/schedule`, `/api/v1/transcode/override`, and wire `gpu_status` in SSE telemetry.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for transcode queue lifecycle and telemetry integration.
- [x] `api/openapi.yaml` -- Document transcode endpoints in OpenAPI schema.

**Acceptance Criteria:**
- Given raw media waiting in the transcoding queue, when the scheduled off-peak window arrives (01:00 to 07:00), the background transcoding worker starts.
- Raw videos encode into standardized H.264/H.265 MP4 files with universal stereo/multichannel AAC audio compatibility.
- During daytime hours (07:00 to 01:00), batch video transcoding is paused or throttled to keep GPU compute and VRAM available for interactive AI chat, unless manually overridden.
- Completed transcodes trigger an immediate Jellyfin library refresh.

## Implementation Notes

- Created `internal/transcode` package with:
  - `store.go`: SQLite-backed persistence for `transcode_queue` table with indexes on `(status, created_at)`, supporting enqueue, lookup, list, atomic status updates, progress tracking, and restart recovery.
  - `scheduler.go`: Day/Night window evaluator defaulting to 01:00-07:00 with support for midnight-crossing windows, manual override, and telemetry formatting for `gpu_status` (`Daytime (AI Priority)` vs `Off-Peak (Transcoding Active)`).
  - `transcoder.go`: `Transcoder` interface and `FFmpegTranscoder` generating standardized MP4 files (`-pix_fmt yuv420p`, `-movflags +faststart`, AAC audio `-ac 2`) with hardware acceleration detection (`/dev/dri`, `/dev/nvidia*`) and deterministic `MockTranscoder` for unit/integration testing.
  - `engine.go`: Orchestrator managing background queue execution, 10% storage headroom safety floor enforcement, automatic software encoder fallback upon hardware encoder failure, atomic library placement, and Jellyfin library refresh webhooks.
  - `transcode_test.go`: 10 comprehensive unit tests covering schedule evaluation, day/night transitions, manual override, store CRUD, headroom guards, hardware failure fallback, and queue validation.
- Extended `cmd/sovereign-daemon/server.go`:
  - Registered `/api/v1/transcode/queue` (GET, POST), `/api/v1/transcode/schedule` (GET, POST), and `/api/v1/transcode/override` (POST).
  - Integrated `TranscodeEngine` into background lifecycle (`StartBackgroundServices`, `Close`).
  - Wired live dynamic `gpu_status` calculation into real-time SSE telemetry updates.
- Extended `cmd/sovereign-daemon/server_test.go`:
  - Added 5 integration tests covering queue lifecycle, schedule retrieval/updates, manual override toggles, SSE `gpu_status` telemetry transitions, and end-to-end batch execution with Jellyfin refresh.
- Updated `api/openapi.yaml`:
  - Documented `/api/v1/transcode/queue`, `/api/v1/transcode/schedule`, and `/api/v1/transcode/override`.
  - Added component schemas `TranscodeJobResponse`, `TranscodeQueueResponse`, `EnqueueTranscodeRequest`, `TranscodeScheduleResponse`, `UpdateTranscodeScheduleRequest`, `TranscodeOverrideRequest`, and `TranscodeOverrideResponse`.

## Spec Change Log

*(No changes)*

## Review Triage Log

*(To be populated during code review)*

