---
title: 'Story 8.3: Critical Path Fault-Injection & Long-Running Stream Verification Suite'
type: 'feature'
created: '2026-11-09'
status: 'done'
baseline_commit: 'e5aa7a1fac3c1ab784e475ac66587c0c04caafe2'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/implementation-artifacts/epic-8-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Critical operational invariants can silently regress without explicit verification: Server-Sent Events (SSE) telemetry connections might drop prematurely if write timeouts or response flushing are misconfigured; urgent document deadlines or hardware degraded alerts might be permanently lost if remote endpoints (SMTP server or Home Assistant webhook) are temporarily down during delivery; and local AI model downloads might exhaust storage if the 5.0 GB reservation check is bypassed or fails to abort cleanly without leaving partial staging files.

**Approach:** Establish an automated fault-injection and verification suite that tests:
1. Long-running SSE stream endurance: verifies that `/api/v1/telemetry/events` remains open and stable for continuous streaming without server-side write timeout drops, mid-stream memory leaks, or unhandled socket closure errors.
2. Persistent alert dispatch queue: introduces `alert_dispatch_queue` backed by SQLite, capturing failed notification attempts and automatically retrying with exponential backoff until remote targets recover, verified by fault-injection tests.
3. Model storage reservation guard: verifies that GGUF model download requests abort with HTTP 507 Insufficient Storage and RFC 7807 problem details when free storage is less than `model_size + 5.0 GB`, leaving the reserve untouched and generating zero partial files.

## Boundaries & Constraints

**Always:**
- Keep HTTP server write timeouts disabled (`WriteTimeout = 0` at connection level) or use `http.ResponseController` to disable deadlines on streaming SSE endpoints (`/api/v1/telemetry/events`).
- Store failed notification dispatches (SMTP, Home Assistant webhook, MQTT) in a persistent SQLite table `alert_dispatch_queue` with fields `status` (`pending`, `delivered`, `failed`), `retry_count`, `next_attempt_at`, and `last_error`.
- Re-evaluate pending alert queue items during dispatch cycles and retry with exponential backoff until delivered or reaching maximum retries (`max_retries = 5`).
- Ensure `handleDownloadAIModel` validates storage reservation (`model_size + 5.0 GB`) before creating any file on disk, returning HTTP 507 Insufficient Storage with RFC 7807 problem details (`https://sovereign.local/errors/insufficient-storage`).
- Ensure aborted or failed model downloads leave zero partial files (`*.downloading`) behind in `/data/models`.

**Never:**
- Never drop an SSE telemetry stream due to server-side write timeouts or lack of keep-alive flushing.
- Never drop an urgent deadline or hardware alert permanently when a remote notification endpoint experiences temporary downtime or connection failure.
- Never accept a model download when free storage is less than `model_size + 5.0 GB`.
- Never leave partial `.downloading` files in `/data/models` when a pre-flight storage check fails.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| SSE Long-Running Stream Endurance | Client connects to `/api/v1/telemetry/events` and maintains connection | Continuous telemetry updates and keep-alives delivered stably for extended duration ($\ge 300$s) | No write timeout drops; client disconnect cleans up subscriber channel |
| Unreachable Notification Target (Webhook/SMTP) | Alert triggered while webhook returns 503 or SMTP connection refused | Alert enqueued in `alert_dispatch_queue` as `pending`; error logged | Retried on next dispatch cycle with backoff; transitions to `delivered` when target recovers |
| Alert Queue Max Retries Exhausted | Target remains persistently unreachable for 5 consecutive attempts | Queue item marked as `failed`; logged with calm warning copy | Prevent infinite retry loop; status visible in store |
| Model Download with Insufficient Storage | Model download requested with `contentLength = 4GB` when free space is `8GB` (requires $4 + 5 = 9\text{GB}$) | Pre-flight check returns HTTP 507 Insufficient Storage with RFC 7807 problem details | Aborted immediately; 0 bytes written to disk; no `.downloading` file created |
| Model Download Mid-Stream ENOSPC | Disk space exhausts during download stream | Download marked as `failed`, partial `.downloading` file removed immediately | No corrupted or orphan file remains in `/data/models` |

</frozen-after-approval>

## Code Map

- `internal/store/schema.sql` -- SQLite schema definition for `alert_dispatch_queue` table and indexes.
- `internal/store/db.go` -- Schema migration ensuring `alert_dispatch_queue` is auto-provisioned.
- `internal/store/alert_queue.go` -- `AlertQueueStore` model and queries: Enqueue, GetPending, MarkDelivered, RecordFailure, List.
- `internal/store/alert_queue_test.go` -- Unit tests for `AlertQueueStore` CRUD, retry tracking, and status transitions.
- `internal/notify/dispatcher.go` -- AlertDispatcher integration with `AlertQueueStore`: automatic enqueue upon failure and `ProcessQueue(ctx)` retry loop.
- `internal/notify/notify_test.go` -- Fault-injection tests verifying alert delivery retry and recovery when endpoints become available.
- `cmd/sovereign-daemon/server.go` -- `handleDownloadAIModel`: enforce RFC 7807 type `https://sovereign.local/errors/insufficient-storage` on 507; wire alert queue in dispatcher.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for model download 507 rejection and zero partial file clutter.
- `cmd/sovereign-daemon/sse_endurance_test.go` -- Integration endurance harness verifying SSE stream stability, memory hygiene, and write timeout endurance.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/schema.sql` & `internal/store/db.go` -- Define and migrate `alert_dispatch_queue` table with indexes.
- [x] `internal/store/alert_queue.go` & `internal/store/alert_queue_test.go` -- Implement `AlertQueueStore` with Enqueue, GetPending, MarkDelivered, RecordFailure.
- [x] `internal/notify/dispatcher.go` -- Equip `AlertDispatcher` with `AlertQueueStore` reference, enqueuing failed alerts and retrying pending queue items in `ProcessQueue`.
- [x] `internal/notify/notify_test.go` -- Add fault-injection tests verifying failed webhook/email alerts are enqueued and successfully delivered upon recovery.
- [x] `cmd/sovereign-daemon/server.go` & `cmd/sovereign-daemon/server_test.go` -- Update `handleDownloadAIModel` 507 problem type to `https://sovereign.local/errors/insufficient-storage` and test zero partial files on rejection.
- [x] `cmd/sovereign-daemon/sse_endurance_test.go` -- Implement long-running SSE stream endurance test verifying connection stability and write timeout resilience.
- [x] Run full verification suite across Go backend and TypeScript web frontend.

**Acceptance Criteria:**
- Given a long-running Server-Sent Events (SSE) telemetry connection, when tested under an end-to-end connection duration harness, then the stream remains open and stable for >= 300 seconds without server-side write timeout drops, mid-stream memory leaks, or unhandled socket closure errors.
- Given an urgent document deadline or system alert dispatch via SMTP or Home Assistant webhook, when the remote notification target is unreachable during the initial attempt, then the alert payload is enqueued in a persistent retry queue and successfully re-dispatched when the target becomes reachable again, verified by automated fault-injection tests.
- Given the Local AI model download engine, when a GGUF model download is requested on a filesystem with less than (model size + 5 GB reserve) available, then the pre-flight storage guard aborts the download immediately, leaving the reserve buffer untouched and returning an actionable HTTP 507 Insufficient Storage status with zero partial file clutter.

## Implementation Notes
- `alert_dispatch_queue` table:
  ```sql
  CREATE TABLE IF NOT EXISTS alert_dispatch_queue (
      id TEXT PRIMARY KEY,
      channel TEXT NOT NULL,
      recipient TEXT NOT NULL,
      payload TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending',
      retry_count INTEGER NOT NULL DEFAULT 0,
      max_retries INTEGER NOT NULL DEFAULT 5,
      next_attempt_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
      last_error TEXT DEFAULT '',
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```
- Exponential backoff for queue retries: `delay = min(10s * (2^retry_count), 1 hour)`.
- Model storage reservation: `CheckModelStorageSpace` rejects with HTTP 507 if `available < model_size + 5.0 GB`, verifying 0 bytes are written and no `.downloading` files remain.
- SSE stream endurance: verifies keep-alive heartbeat flushing and response flushing withstands extended connections without premature socket resets.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence & Routing |
|---|---|---|
| `internal/store/alert_queue.go`: `Enqueue` ON CONFLICT does not reset status/retry | `high` | Re-enqueuing an existing alert row leaves status as `delivered` or `failed`, preventing subsequent dispatch. Reset status to `excluded.status` and retry count. Route: `patch`. |
| `internal/store/alert_queue.go`: integer overflow in `CalculateBackoff` | `medium` | `10s * (1 << retryCount)` overflows 64-bit int when `retryCount >= 30`. Cap at 1 hour when `retryCount >= 9`. Route: `patch`. |
| `internal/notify/dispatcher.go`: `DispatchCycle` discards `processQueueLocked` delivered count | `medium` | `_, _ = d.processQueueLocked(ctx)` discarded count of delivered queued items, causing underreporting in API. Add count to returned value. Route: `patch`. |
| `internal/ai/models.go`: verify failure leaves orphaned `.downloading` file | `medium` | If SHA-256 or header validation fails after streaming, `tempFile` is not removed. Ensure `os.Remove(tempFile)` is called. Route: `patch`. |
| `internal/ai/models_test.go`: missing mid-stream write error test | `medium` | Verified gap: no test simulates `file.Write` error mid-stream to assert `.downloading` file cleanup and failure status. Route: `patch`. |
| `internal/notify/notify_test.go`: missing `DispatchCycle` queue retry caller test | `medium` | Verified gap: tests only invoked `ProcessQueue(ctx)` directly, not the production entry point `DispatchCycle(ctx)`. Route: `patch`. |
| `cmd/sovereign-daemon/server.go`: background queue retry periodic routine missing | `medium` | Without background ticker in `StartBackgroundServices`, queued alerts only retry when new documents or manual dispatches arrive. Route: `patch`. |
| `internal/notify/dispatcher.go`: malformed email payload fallback sends test email | `medium` | Deserialization failure fell back to `SendTestEmail` instead of recording failure on the queue item. Route: `patch`. |
| `cmd/sovereign-daemon/sse_endurance_test.go`: select loop lacks stalled-read timeout | `low` | If server stalls silently, test blocks indefinitely. Add per-message read timeout. Route: `patch`. |
| `internal/notify/dispatcher.go`: stale `now` timestamp in queue retry loop | `low` | Next attempt calculation used loop-start timestamp instead of current time when recording failures. Route: `patch`. |
| `internal/store/alert_queue.go`: `datetime()` wrapper on indexed column suppresses index | `low` | Wrapping `datetime(next_attempt_at)` prevents SQLite composite index usage. Compare `next_attempt_at <= ?` directly. Route: `patch`. |
| `internal/notify/dispatcher.go`: email digest ID collision on same minute | `low` | Truncating timestamp to minute in ID format causes collision on multiple digest failures. Include seconds or UUID suffix. Route: `patch`. |


## Design Notes

### Alert Dispatch Queue Schema
- Channels: `webhook`, `email`, `mqtt`.
- State transitions: `pending` -> `delivered` (on success) or `pending` (with incremented `retry_count` and future `next_attempt_at` timestamp). If `retry_count >= max_retries`, transitions to `failed`.

### Model Download Storage Guard
- `handleDownloadAIModel` returns `writeProblemWithType` with `https://sovereign.local/errors/insufficient-storage` when `errors.Is(err, ai.ErrInsufficientStorageSpace)`.

## Verification

**Commands:**
- `go test -v -count=1 ./internal/store -run TestAlertQueue` -- expected: PASS
- `go test -v -count=1 ./internal/notify -run TestAlertDispatcher_PersistentQueue` -- expected: PASS
- `go test -v -count=1 ./cmd/sovereign-daemon -run "TestModelDownload_InsufficientStorage|TestSSE_Endurance"` -- expected: PASS
- `go test -count=1 ./...` -- expected: PASS
- `npm --prefix web run codegen` -- expected: PASS
- `npm --prefix web test -- --run` -- expected: PASS
