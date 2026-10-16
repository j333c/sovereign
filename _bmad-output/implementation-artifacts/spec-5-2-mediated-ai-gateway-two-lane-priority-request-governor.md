---
title: 'Story 5.2: Mediated AI Gateway & Two-Lane Priority Request Governor'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '7e92c65445f076599a63171062015deb4103725b'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In a household home server, automated background AI tasks (e.g. OCR document parsing, deadline obligation extraction, metadata enrichment) compete directly for inference capacity with real-time interactive user chats. If an automated batch job saturates the inference slots, family members chatting with the AI on Open WebUI experience seconds of perceived freezing or timeouts. Furthermore, consumer users should not need machine learning degrees to configure raw sampling parameters (`temperature`, `top_p`, `repeat_penalty`).

**Approach:** Implement a Mediated AI Gateway with a Two-Lane Priority Request Governor and Pre-Calibrated Generation Intent Presets in `sovereign-daemon`:
1. **Two-Lane Priority Request Governor**:
   - **Lane 1: Interactive Chat (`PriorityInteractive`)**: High-priority lane for human interactions (Open WebUI, direct user chat). Interactive requests acquire inference execution slots immediately, jumping ahead of queued background tasks.
   - **Lane 2: Background Tasks (`PriorityBackground`)**: Lower-priority lane for automated appliance jobs (Paperless obligation scanner, document OCR, media enrichment). Background tasks are queued and yield whenever interactive requests are executing or queued, preventing chat stall.
   - Slot coordination: Tracks active slots (matching `llama-server`'s `-np 2` parallel slots) ensuring at least 1 slot is immediately reserved for interactive human requests whenever interactive traffic is pending.
2. **Pre-Calibrated Generation Intent Presets**:
   - *Precise & Factual* (`precise`): `temperature: 0.1`, `top_p: 0.9` (default for deadline extraction, invoice parsing, fact retrieval).
   - *Balanced Assistant* (`balanced`): `temperature: 0.7`, `top_p: 0.95` (default for conversational assistant chat).
   - *Creative Drafting* (`creative`): `temperature: 0.85`, `top_p: 0.98` (for brainstorming, story drafting, ideation).
   - Gateway transparently maps requested preset/intent (via JSON field `preset` / `intent` or HTTP header `X-Generation-Intent`) into standard OpenAI-compatible `temperature` and `top_p` sampling parameters when forwarding to `llama-server`.
3. **Observability & Inspection Endpoints**:
   - `GET /api/v1/ai/presets`: Lists available generation presets with descriptions, parameters, and recommended use-cases.
   - `GET /api/v1/ai/governor/status`: Reports live queue metrics (active interactive requests, active background requests, queued background requests, total served).
4. **Context & Cancellation Lifecycle**:
   - If an interactive client disconnects or aborts (`ctx.Done()`), the slot is freed immediately so queued background work or new chats can proceed without stranded computation.

## Boundaries & Constraints

**Always:**
- Prioritize `PriorityInteractive` over `PriorityBackground` requests under all concurrency loads.
- Preserve backward-compatibility with standard OpenAI `/v1/chat/completions`, `/v1/completions`, `/v1/models` endpoints.
- Support priority classification via `X-Inference-Priority: interactive|background` header and request path detection (e.g. Open WebUI traffic defaults to interactive, internal daemon scanner jobs default to background).
- Enforce generation presets (`precise`, `balanced`, `creative`) while permitting explicit numeric overrides if specifically provided by power users.
- Immediately release concurrency slots upon client connection cancellation (`r.Context().Done()`).
- Maintain thread-safe governor state using sync primitives and atomic counters.

**Never:**
- Never let background batch processing block interactive chat requests.
- Never crash or leak goroutines on aborted or timed-out client HTTP connections.
- Never drop requests silently without standard HTTP errors (HTTP 429 Too Many Requests or HTTP 504 Gateway Timeout if queue depth / wait timeout expires).
- Never bypass Safe Stub Mode when no models are available or when the service is stopped.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Interactive Chat Request | Inbound `/v1/chat/completions` with interactive priority | Dispatched immediately to `llama-server`; acquires active slot | 503 if in Safe Stub Mode |
| Background Task During Idle | Inbound `/v1/chat/completions` with background priority, no active chats | Dispatched to available slot; executes normally | 503 if in Safe Stub Mode |
| Concurrent Interactive & Background | Background tasks running, incoming interactive chat arrives | Interactive chat preempts waiting queue and executes immediately; background tasks yield slots | 429 if max queue limit exceeded |
| Generation Preset Override | Payload contains `{"preset": "creative", "messages": [...]}` | Gateway injects `temperature: 0.85, top_p: 0.98` into upstream request payload | Ignores invalid preset, falls back to balanced |
| Explicit Temperature Override | Payload contains `{"preset": "precise", "temperature": 0.2}` | User explicit temperature `0.2` preserved; `top_p: 0.9` set from preset | Respects user power-use override |
| Client Disconnects Mid-Inference | Client terminates TCP socket during generation | Upstream context cancelled, slot released immediately to pending requests | Context cancellation handled cleanly |
| Governor Queue Saturation | Background queue exceeds capacity (e.g. > 50 queued jobs) | Rejects excess background request with HTTP 429 Problem Details | RFC 7807 Too Many Requests |

</frozen-after-approval>

## Code Map

- `internal/ai/governor.go` -- Two-Lane Priority Request Governor with slot acquisition, priority queues, and timeout metrics.
- `internal/ai/presets.go` -- Generation Intent Presets (`precise`, `balanced`, `creative`), payload inspector, and normalizer.
- `internal/ai/gateway.go` -- Integrate Request Governor and Intent Presets into `DefaultGateway.ServeHTTP`.
- `internal/documents/gateway_client.go` -- Update obligation scanner LLM extractor to tag requests with `X-Inference-Priority: background` and `preset: precise`.
- `cmd/sovereign-daemon/server.go` -- Register `/api/v1/ai/presets` and `/api/v1/ai/governor/status` endpoints.
- `api/openapi.yaml` -- Document preset and governor status endpoints.

## Implementation Plan

1. **Generation Presets (`internal/ai/presets.go`)**:
   - Define `Preset` struct and catalogue (`precise`, `balanced`, `creative`).
   - Implement `NormalizeInferencePayload(body []byte, defaultPreset string) ([]byte, error)` injecting calibrated temperature and top_p.
2. **Two-Lane Request Governor (`internal/ai/governor.go`)**:
   - Implement `RequestGovernor` managing active interactive and background execution slots.
   - Implement slot acquisition with channel signaling and priority preference for interactive requests.
   - Implement cancellation listening on request context.
3. **Gateway Integration (`internal/ai/gateway.go`)**:
   - Wire `RequestGovernor` and payload normalizer into `ServeHTTP`.
   - Classify priority via headers and endpoints.
4. **Daemon Integration & Obligation Scanner Alignment**:
   - Add `/api/v1/ai/presets` and `/api/v1/ai/governor/status` routes in `server.go`.
   - Update `internal/documents/llm.go` / `gateway_client.go` to send background priority and `precise` intent.
5. **Testing & Verification**:
   - Unit tests for presets, governor priority dispatching, concurrency queuing, client cancellations, and payload transformation.
   - Run full test suites (`go test ./...` and `npm --prefix web test`).

## Review Triage Log

- **Iteration 1**:
  - Triaged review findings from Blind Hunter, Edge Case Hunter, and Verification Gap reviewers:
    1. *Payload Normalization Upstream Sanitation*: Stripped non-standard `preset` and `intent` helper fields from outbound JSON body before forwarding to OpenAI-compatible upstream `llama-server` to prevent upstream schema rejection. Guarded against `null` literal JSON payloads.
    2. *Governor Waiter Cancellation Scheduling*: Added `g.scheduleNext()` in the cancellation path of `waitForSlot` when waiters abort or timeout mid-queue, preventing queued background tasks from starving when capacity is available.
    3. *Cancellation Metrics Tracking*: Added `TotalCanceled` counter to `GovernorStatus` and updated `makeReleaseFunc` to count canceled contexts as canceled rather than served.
    4. *Read-Only Gateway Bypass*: Ensured `GET`, `HEAD`, and `OPTIONS` preflight requests bypass governor slot acquisition directly so models listing and CORS remain non-blocking during saturated inference workloads.
    5. *Verification Tests Added*:
       - `TestDefaultGateway_QueueTimeout_504`: Confirmed RFC 7807 504 Gateway Timeout when governor wait queue expires.
       - `TestDefaultGateway_ReadonlyModelsBypassesSaturatedGovernor`: Confirmed `GET /v1/models` and `OPTIONS` bypass saturated governor immediately.
       - `TestNormalizeInferencePayload_NilAndStripKeys`: Confirmed `preset`/`intent` key stripping and null/nil payload handling.
       - `TestRequestGovernor_CancellationTracksTotalCanceled`: Confirmed cancellation metric increments.
    6. *Full Test Suite Run*: 100% passing across Go unit tests (`go test ./...`) and web test suites (`npm --prefix web test`).

