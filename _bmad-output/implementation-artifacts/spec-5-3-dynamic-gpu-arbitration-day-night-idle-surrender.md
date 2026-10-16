---
title: 'Story 5.3: Dynamic GPU Arbitration & Day/Night Idle Surrender'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '190aa9df95b4fef19f95a4fb5fa55629ae7fb870'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In a single-box household server, high-demand local AI inference models monopolize gigabytes of video RAM (VRAM) even when nobody is actively chatting. If a family member attempts to stream a 4K HDR movie or run hardware-accelerated video transcoding while the GPU is saturated with idle model weights, video playback or transcoding abruptly crashes with Out-of-Memory (OOM) errors. Furthermore, on hybrid GPU hardware (integrated GPU + discrete GPU), lack of hardware pinning leads to contention, and CPU-only systems can freeze other household containers if inference threads are not strictly bounded.

**Approach:** Implement Dynamic GPU Arbitration & Day/Night Idle Surrender in `sovereign-daemon` and `internal/ai`:
1. **Dynamic 300-Second Idle Surrender**:
   - The arbitrator monitors inference activity. After 300 seconds of inactivity, it automatically unloads model weights or stops the container, completely releasing VRAM so active media playback, live transcoding, and offline batch encoding have full GPU headroom.
2. **Day/Night Habit Scheduling & Smooth Daytime Wakeup**:
   - During daytime hours (07:00 to 01:00), incoming chat queries automatically wake the surrendered model.
   - Waking queries transition through `status: "warming_up"` with an extended >=60s timeout buffer (default: 90s) and provide comforting in-UI notification microcopy: *"Waking up private AI (ready in 5s)..."*.
   - During nighttime off-peak hours (01:00 to 07:00), GPU priority naturally belongs to scheduled batch transcoding and media library indexing.
3. **Hybrid GPU Device Pinning**:
   - On systems equipped with both an integrated GPU (Intel QSV or AMD VAAPI via `/dev/dri/renderD128`) and a discrete GPU (NVIDIA CUDA via `/dev/nvidia*`), the system automatically pins video transcoding to the iGPU and LLM inference to the dGPU, eliminating VRAM contention altogether.
4. **CPU-Only Thread Capping & "CPU-Optimized Mode"**:
   - On hosts without hardware acceleration, inference threads are capped at `max(1, physical_cores - 2)` to reserve CPU capacity for the host and other containers.
   - The central dashboard and AI status endpoints display a reassuring "CPU-Optimized Mode" indicator across English, German, Spanish, and French.

## Boundaries & Constraints

**Always:**
- Release VRAM after 300 seconds of continuous AI inactivity when running on shared GPU hardware.
- Provide extended timeout buffer (>=60s) during daytime wake-up so users don't encounter gateway 504 timeouts while weights reload into memory.
- Display consumer-friendly reassurance copy (*"Waking up private AI (ready in 5s)..."*) without raw Linux device codes or stack traces.
- Pin video transcoding to iGPU and LLM to dGPU when hybrid GPU hardware is present.
- Cap CPU inference threads to `max(1, physical_cores - 2)` on CPU-only hosts and expose the "CPU-Optimized Mode" indicator.
- Support deterministic time injection (`NowFunc`) and mock unload/wake hooks for reproducible unit and integration testing.

**Never:**
- Never crash active video playback or transcoding due to dormant AI model allocations.
- Never freeze the host operating system by assigning all CPU cores to inference on CPU-only appliances.
- Never reject daytime waking requests prematurely before the extended >=60s warm-up buffer expires.
- Never emit external telemetry or ping cloud endpoints during wake/sleep arbitration.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| AI Inactivity Ticker Expires | 300s elapsed since last completed inference request | Arbitrator executes VRAM surrender; transitions state to `idle_surrendered`; VRAM freed | Logs transition; remains resilient if already unloaded |
| Daytime Chat During Idle | Inbound `/v1/chat/completions` between 07:00 and 01:00 while `idle_surrendered` | Triggers wake-up; status enters `warming_up`; timeout buffer >=60s applied; UI shows reassuring microcopy | Returns 504 only if warm-up exceeds 60s buffer |
| Nighttime Request | Inbound request between 01:00 and 07:00 | Wakes model or coordinates with off-peak batch transcoding queue | Respects scheduler priority |
| Hybrid GPU System Detected | Both `/dev/dri/renderD128` and `/dev/nvidia*` detected | Jellyfin assigned iGPU; Llama assigned dGPU; hybrid status flagged | Fallback to single GPU if either device fails probe |
| CPU-Only System Detected | No NVIDIA or DRI render devices found | Threads capped to `physical_cores - 2`; UI displays "CPU-Optimized Mode" | Minimum thread count clamped to 1 |
| Immediate Request While Warming Up | Second request arrives while model is already loading | Enqueued in governor queue; waits for warming up to complete; served immediately upon readiness | Saturated queue rejects with HTTP 429 |

</frozen-after-approval>

## Code Map

- `internal/ai/arbitrator.go` -- Dynamic GPU Arbitrator managing idle surrender timer (300s), day/night schedule, warming up states, and waking buffers.
- `internal/ai/gateway.go` -- Wire arbitrator into AI Gateway to trigger wake-up on demand and update activity timestamp.
- `internal/supervisor/llama.go` -- Hybrid GPU detection, dGPU pinning for Llama, and CPU-optimized mode indicator.
- `internal/supervisor/jellyfin.go` -- Hybrid GPU detection, iGPU pinning for Jellyfin video transcoding.
- `cmd/sovereign-daemon/server.go` -- Register `/api/v1/ai/status` endpoint reporting state, hardware mode, daytime status, and hybrid GPU assignments.
- `api/openapi.yaml` -- Document `/api/v1/ai/status` and `AIStatusResponse`.
- `web/src/app/page.tsx` & `web/src/lib/i18n.ts` -- Central dashboard display of "CPU-Optimized Mode" and warming up notification banner.

## Implementation Plan

1. **Hybrid Hardware Detection (`internal/supervisor/jellyfin.go`, `internal/supervisor/llama.go`)**:
   - Add `IsHybridGPU` detection: if both `/dev/dri/renderD128` (iGPU) and `/dev/nvidia*` (dGPU) exist.
   - In hybrid mode: pin Jellyfin to iGPU (`HWAccelQSV` or `HWAccelVAAPI`), pin Llama to dGPU (`HWAccelNVENC`).
   - If CPU-only: mark `CPUOptimized: true`.
2. **GPU Arbitrator (`internal/ai/arbitrator.go`)**:
   - Implement `GPUArbitrator` with states: `ready`, `warming_up`, `idle_surrendered`, `safe_stub`.
   - 300s inactivity timer with reset on inference activity.
   - Day/night evaluation (07:00–01:00 daytime, 01:00–07:00 off-peak).
   - Daytime wake-up orchestration with >=60s timeout buffer and `"Waking up private AI (ready in 5s)..."` microcopy.
   - Pluggable `UnloadFunc` and `WakeFunc` callbacks.
3. **Gateway & Daemon Wiring (`internal/ai/gateway.go`, `cmd/sovereign-daemon/server.go`)**:
   - Connect Gateway to Arbitrator; wake model when request arrives in `idle_surrendered` state.
   - Register `GET /api/v1/ai/status` endpoint.
4. **OpenAPI & Frontend (`api/openapi.yaml`, `web/`)**:
   - Update OpenAPI schema and TypeScript definitions (`npm run codegen`).
   - Add client method `getAIStatus()`.
   - Add localized microcopy in `i18n.ts` (EN, DE, ES, FR).
   - Display "CPU-Optimized Mode" and warming up notice in AI ServiceCard on dashboard.
5. **Testing & Verification**:
   - Unit tests for GPU arbitrator idle surrender, day/night transitions, wake-up timeout buffers, and hybrid GPU pinning.
   - Full automated test run: `go test ./...` and `npm --prefix web test`.

## Review Triage Log

- **Review Cycle**: Iteration 1
- **Reviewers**:
  - Blind Hunter Reviewer (`92255a13-d685-49ee-9931-5dca284692d7`)
  - Edge Case Hunter Reviewer (`c7b4864c-51f3-46d8-b4f8-b0e957593c48`)
  - Verification Gap Reviewer (`f37b0dd3-c0ba-4274-a1f0-848745dc19c3`)

### Findings Triaged & Remediations Applied

1. **UnloadFunc & WakeFunc Wiring in Production Server (`cmd/sovereign-daemon/server.go`)**:
   - *Finding*: Default arbitrator instance in `server.go` was created without wiring `UnloadFunc` and `WakeFunc` callbacks, meaning container lifecycle commands were never issued during idle surrender or daytime wake-up.
   - *Fix*: Wired `UnloadFunc` to call `s.serviceMgr.DeactivateService(ctx, supervisor.ServiceLlama)` and `WakeFunc` to call `s.serviceMgr.ActivateService(ctx, supervisor.ServiceLlama)`.
2. **In-Flight Inference vs. Idle Surrender Race Condition (`internal/ai/arbitrator.go`, `gateway.go`)**:
   - *Finding*: Idle timer could trigger VRAM surrender mid-stream during long prompts or complex generation.
   - *Fix*: Added `inFlight int` counter with `BeginInference() func()` guard. `SurrenderVRAM` and background ticker explicitly reject/skip surrender when `inFlight > 0`.
3. **Graceful Wakeup Context Cancellation (`internal/ai/arbitrator.go`)**:
   - *Finding*: Client disconnect or abort during `warming_up` caused the HTTP gateway to block for the full 90-second timeout.
   - *Fix*: Integrated caller context handling in `EnsureReady(ctx)`: returns caller's `ctx.Err()` immediately while letting the background wake complete cleanly so subsequent requests benefit from readiness.
4. **Resilient Gateway Shutdown (`internal/ai/gateway.go`, `arbitrator.go`)**:
   - *Finding*: Arbitrator goroutine ticker was not closed when `Gateway` was shut down, risking orphaned background goroutines.
   - *Fix*: Added `Gateway.Close()` and `GPUArbitrator.Close()` with `sync.Once` and channel shutdown. Wired into `Server.Close()`.
5. **SafeStub Auto-Recovery (`internal/ai/gateway.go`)**:
   - *Finding*: If the server booted with an empty `/data/models` directory, arbitrator state was trapped in `safe_stub` permanently even after models were downloaded.
   - *Fix*: Added auto-recovery logic transitioning arbitrator from `safe_stub` to `idle_surrendered` as soon as valid models are detected.
6. **Localization Precedence on Central Dashboard (`web/src/app/page.tsx`)**:
   - *Finding*: The server's English reassurance message took precedence over localized client strings in German, Spanish, and French.
   - *Fix*: Swapped evaluation order in `page.tsx` so `t.aiWarmingUpReassurance` takes precedence when AI service is warming up.
7. **Comprehensive Automated Verification (`internal/ai/ai_test.go`, `cmd/sovereign-daemon/server_test.go`, `internal/supervisor/jellyfin_test.go`)**:
   - *Finding*: Missing integration test verifying `gpu.idle_surrender` SSE event broadcast, hybrid GPU non-Intel fallback, and warmup 504 timeout response.
   - *Fix*: Added `TestServer_AIIdleSurrender_SSEBroadcast`, `TestIsHybridGPU_NvidiaDRM_NotHybrid`, and `TestGateway_WarmupTimeout_504`.

