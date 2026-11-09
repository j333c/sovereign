---
title: 'Automated Retry Backoff Jitter Telemetry in Metadata Client'
type: 'feature'
created: '2026-11-09'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Third-party metadata API requests (TMDb, MusicBrainz) retry upon rate limits (HTTP 429) and transient network drops using exponential backoff with random jitter, but lack structured telemetry metrics and debug logging, making it difficult to monitor retry volume, backoff durations, and jitter distributions.

**Approach:** Equip `HTTPMetadataClient` in `internal/enrichment/client.go` with a structured `slog.Logger` emitting debug logs on every retry attempt (recording endpoint, attempt number, max retries, backoff delay, jitter offset, and failure reason) and expose thread-safe `RetryMetrics` telemetry accessible via `GetRetryMetrics()`.

</frozen-after-approval>

## Implementation Notes

- Added `Logger *slog.Logger` to `HTTPMetadataConfig` defaulting to `slog.Default()` when omitted.
- Introduced `RetryMetrics` struct recording `TotalRetries`, `RateLimitRetries`, `NetworkRetries`, `TotalBackoffDelay`, `LastBackoffDelay`, `TotalJitterOffset`, `LastJitterOffset`, and `LastRetryTimestamp`.
- Exposed thread-safe `GetRetryMetrics()`, `ResetRetryMetrics()`, and `SetLogger()` on `HTTPMetadataClient`.
- Defined `RetryMetricsProvider` interface and exposed `GetRetryMetrics() (RetryMetrics, bool)` on `EnrichmentEngine`.
- Updated `calculateBackoffDelay(attempt int) (time.Duration, time.Duration)` returning computed delay and jitter offset, clamped to `MaxDelay` (default 30s).
- Implemented `sanitizeURL` and `sanitizeError` ensuring API keys (`api_key=...`) are redacted from logged URLs and error strings.
- Deferred `recordRetryTelemetry` until backoff sleep completes without context cancellation, preventing premature telemetry increments on canceled/timed out requests.
- Preserved unwrapped context errors (`context.Canceled`, `context.DeadlineExceeded`) in `FetchMovieMetadata`, `FetchTVMetadata`, `FetchMusicMetadata`, and `DownloadArtwork`.
- Added unit tests in `internal/enrichment/client_test.go` covering structured log attributes, rate limit and network retries, secret key redaction, context cancellation during backoff, concurrency safety under `-race`, and engine metrics integration.
- Wired `cfg.Logger` to `HTTPMetadataClient` in `cmd/sovereign-daemon/server.go`.
- Marked action item `epic-8-retro-item-2-retry-telemetry-metrics` as `done` in `sprint-status.yaml`.

## Review Triage Log

- `high` (`patch`): Network error strings (`err.Error()`) containing raw `api_key` in `*url.Error` were logged. Fixed by introducing `sanitizeError` to redact query params in wrapped error messages.
- `medium` (`patch`): Telemetry was recorded before waiting on `select`, causing overcounting on context cancellation. Fixed by recording metrics only after sleep completes.
- `medium` (`patch`): `context.DeadlineExceeded` was treated as retryable network error. Fixed by checking `req.Context().Err() != nil` and `context.DeadlineExceeded` before retry handling.
- `medium` (`patch`): Jitter offsets were omitted from `RetryMetrics`. Added `TotalJitterOffset` and `LastJitterOffset` to `RetryMetrics`.
- `low` (`patch`): `RetryMetrics` was not accessible through `EnrichmentEngine`. Added `RetryMetricsProvider` interface and `GetRetryMetrics() (RetryMetrics, bool)` method on `EnrichmentEngine`.
- `low` (`patch`): Runaway exponential delays lacked upper bound. Added `MaxDelay` parameter to `BackoffConfig` (default 30s) and clamped delays.
- `low` (`patch`): Broad substring matching on `"eof"` could match query parameters. Fixed by matching `io.EOF`, `io.ErrUnexpectedEOF`, and checking root error.
- `low` (`patch`): Added concurrent `-race` test, context cancellation test, secret redaction test, and engine integration test in `client_test.go`.
- `low` (`patch`): Documented nanosecond JSON duration units in struct docstrings.
