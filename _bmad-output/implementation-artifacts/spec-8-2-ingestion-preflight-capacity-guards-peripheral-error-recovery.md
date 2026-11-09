---
title: 'Story 8.2: Ingestion Pre-Flight Capacity Guards & Peripheral Error Recovery'
type: 'feature'
created: '2026-11-09'
status: 'done'
baseline_commit: 'ef5f4db96f8a26f9738e8cc09bc0c5aa1a9a08b7'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/implementation-artifacts/epic-8-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Large uploads to the media dropzone can exhaust disk space mid-stream leaving broken files; peripheral disconnects (`EIO`) or tray eject failures during optical ripping can cause daemon lockups or orphaned staging files; external API rate limits (HTTP 429) or transient network hiccups on TMDb/MusicBrainz trigger tight retry loops and API bans; and mail servers with custom internal CAs or self-signed TLS certificates fail IMAP document polling.

**Approach:** Implement pre-flight disk capacity verification (upload size + 1.0 GB safety buffer) rejecting insufficient storage with HTTP 507 Problem Details; harden optical ripping to gracefully catch drive disconnects (`EIO`, `ENODEV`) and eject errors, purging staging files without daemon lockup; equip metadata enrichment HTTP requests with jittered exponential backoff (up to 3 retries) and deferral cooldowns; and extend IMAP configuration and TLS connection handlers to support custom CA certificate paths and `insecure_skip_verify`.

## Boundaries & Constraints

**Always:**
- Verify storage pool free capacity before accepting dropzone file uploads (`/drop`), requiring `file_size + 1.0 GB` buffer (`1 * 1024 * 1024 * 1024` bytes) and returning HTTP 507 Insufficient Storage with RFC 7807 problem details when space is lacking.
- Check capacity in the background SMB drop folder scanner before promoting dropped files, skipping promotion and logging calm warnings if capacity is insufficient.
- Abort active optical ripping cleanly if the drive disconnects (`EIO`, `ENODEV`), purge partial staging directories, log descriptive warnings, and handle tray eject failures without panicking or blocking daemon routines.
- Retry external TMDb and MusicBrainz requests up to 3 times with jittered exponential backoff (`base_delay = 1s, factor = 2, max_retries = 3, jitter = 250ms`) on HTTP 429 rate limits or network drops before deferring the disc with a cooldown.
- Support `insecure_skip_verify` (boolean) and `ca_cert_path` (string) in IMAP configuration schemas, persistence models, connection testing, and background polling.

**Never:**
- Never accept a dropzone upload when target storage has less than `upload_size + 1.0 GB` available.
- Never leave partial optical rip staging folders (`/data/media/staging/discs/:disc_id`) behind upon read or disconnect failures.
- Never enter a tight retry loop against external metadata APIs when HTTP 429 or connection errors occur.
- Never reject an IMAP connection due to unknown authority when the administrator explicitly provided a custom CA certificate path or enabled `insecure_skip_verify`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Dropzone Upload with Insufficient Space | POST `/api/v1/drop/upload` with `Content-Length: 10GB` when free space is 5GB | HTTP 507 with RFC 7807 problem details (`https://sovereign.local/errors/insufficient-storage`) | Reject before staging; 0 bytes written to disk |
| SMB Drop Folder Insufficient Space | File placed in `/data/drop/incoming/movies` exceeds free space + 1GB | File promotion skipped, warning logged, file preserved in incoming drop without unlinking | Next scan cycle re-evaluates after space freed |
| Optical Drive Disconnection (`EIO`) | Optical drive disconnected (`syscall.EIO` or `ENODEV`) during active rip | Rip aborts cleanly, staging folder purged, `StateError` set with calm copy, tray eject error handled without deadlock | Daemon continues normal operation; drive set to not ready |
| Optical Tray Eject Failure | Hardware eject command returns failure after rip error | Warning logged, state transitions safely to `StateIdle` without blocking daemon threads | Status text reflects safe idle/ready state |
| Metadata API Rate Limit (429) | TMDb/MusicBrainz returns HTTP 429 or connection drop | Request retried with exponential backoff (1s, 2s, 4s) + jitter up to 3 times before deferring task | Task deferred with cooldown to prevent bans |
| IMAP Custom Internal CA | IMAP server with self-signed certificate and valid `ca_cert_path` configured | TLS handshake succeeds via loaded `x509.CertPool`; mailbox connects and polls documents | Return error if CA file is unreadable or malformed |
| IMAP Insecure Skip Verify | IMAP server with self-signed certificate and `insecure_skip_verify: true` | TLS handshake skips hostname/CA verification; mailbox connects successfully | Log notice indicating insecure TLS mode enabled |

</frozen-after-approval>

## Code Map

- `internal/storage/dropzone.go` -- Dropzone engine: pre-flight capacity guard (`upload_size + 1GB`), disk free inspection helper, SMB scan capacity guard.
- `internal/storage/dropzone_test.go` -- Unit tests for dropzone pre-flight capacity rejections and SMB drop space checks.
- `cmd/sovereign-daemon/server.go` -- `handleDropUpload`: pre-flight `Content-Length` inspection, HTTP 507 Problem Details response; IMAP request structs and `handleSaveIMAPConfig`, `handleTestIMAPConnection`.
- `internal/optical/ripper.go` -- Optical ripper: detection of `syscall.EIO`, `syscall.ENODEV`, and drive disappearance; staging cleanup; graceful eject error handling.
- `internal/optical/ripper_test.go` -- Unit tests simulating drive disconnection (`EIO`) and eject failures during ripping.
- `internal/enrichment/client.go` -- `HTTPMetadataClient`: exponential backoff with jitter on HTTP 429 and network errors (up to 3 retries).
- `internal/enrichment/engine.go` -- `EnrichmentEngine`: deferred disc tracking with cooldown on persistent metadata failures.
- `internal/enrichment/client_test.go` -- Unit tests verifying exponential backoff retries on HTTP 429 and transient network failures.
- `internal/store/imap.go` -- `IMAPConfig` schema: `InsecureSkipVerify` and `CACertPath` fields, persistence in SQLite `imap_configs`.
- `internal/store/schema.sql` -- Schema migration/columns for `insecure_skip_verify` and `ca_cert_path`.
- `internal/documents/poller.go` & `internal/documents/imap_client.go` -- TLS configuration builder supporting custom CA cert pool and insecure skip verify.
- `api/openapi.yaml` -- Updated schemas for `IMAPConfigRequest`, `IMAPConfigResponse`, `IMAPTestRequest`, and drop upload 507 response.

## Tasks & Acceptance

**Execution:**
- [x] `internal/storage/dropzone.go` & `cmd/sovereign-daemon/server.go` -- Implement pre-flight capacity verification in `handleDropUpload` and `HandleUploadStream` with 1.0 GB buffer, returning HTTP 507 Problem Details.
- [x] `internal/storage/dropzone_test.go` & `cmd/sovereign-daemon/server_test.go` -- Add tests verifying dropzone upload rejection and SMB drop skip when storage is insufficient.
- [x] `internal/optical/ripper.go` -- Add detection of `EIO`/`ENODEV` disconnect errors in rip loop, ensure immediate staging cleanup, and handle tray eject failure safely.
- [x] `internal/optical/ripper_test.go` -- Add unit test verifying that `EIO` drive disconnection during active rip terminates cleanly without daemon hang.
- [x] `internal/enrichment/client.go` & `internal/enrichment/engine.go` -- Implement exponential backoff with jitter (max 3 retries) on HTTP 429 / network errors, and add deferral cooldown.
- [x] `internal/enrichment/client_test.go` -- Add unit tests verifying retry behavior on HTTP 429 rate-limiting.
- [x] `internal/store/imap.go` & `internal/store/schema.sql` -- Add `insecure_skip_verify` and `ca_cert_path` to `IMAPConfig` and database queries.
- [x] `internal/documents/poller.go` & `internal/documents/imap_client.go` -- Add `BuildTLSConfig` helper supporting custom CA certs and `InsecureSkipVerify`.
- [x] `api/openapi.yaml` & `cmd/sovereign-daemon/server.go` -- Update IMAP API schemas with TLS options and drop upload 507 response; run `npm --prefix web run codegen`.
- [x] `cmd/sovereign-daemon/server_test.go` & `internal/documents/imap_test.go` -- Add tests verifying IMAP connection with custom CA cert and insecure skip verify.

**Acceptance Criteria:**
- Given a user uploading files to the media dropzone (`/drop` or `\\sovereign\drop`), when the upload request starts, then a pre-flight disk capacity check verifies that the target storage pool has sufficient free space (upload size + 1 GB safety buffer), rejecting with a calm, informative error message if capacity is insufficient.
- Given an active optical disc ripping operation, when an unexpected drive disconnection (`EIO`), bus reset, or tray eject failure occurs, then the rip task terminates cleanly, cleans up temporary staging files, logs a descriptive error, and prevents daemon lockup.
- Given metadata enrichment requests for media or albums (TMDb, MusicBrainz), when external API rate limits or network connection failures occur, then the enrichment service retries requests with exponential backoff and jitter up to 3 times before deferring the task, avoiding rate-limit bans or tight retry loops.
- Given an administrator configuring IMAP email polling for document archiving, when connecting to a mail server with a custom internal CA or self-signed TLS certificate, then the configuration interface allows specifying custom TLS certificate options (or enabling trusted internal CA verification) without failing the TLS handshake.

## Implementation Notes
- Pre-flight capacity check in `internal/storage/dropzone.go` enforces a `DropzoneSafetyBufferBytes = 1 * 1024 * 1024 * 1024` (1.0 GB) headroom requirement. Rejections return HTTP 507 Insufficient Storage with RFC 7807 problem type `https://sovereign.local/errors/insufficient-storage`.
- Ingestion via SMB drop folder evaluates target category storage headroom and skips promotion without unlinking when capacity is insufficient.
- Optical ripper detects `syscall.EIO`, `syscall.ENODEV`, and `/dev/sr*` PathErrors, aborting retry loops immediately, clearing `/data/media/staging/discs/:id`, setting `DriveReady = false`, and recovering gracefully from hardware tray eject failures.
- HTTP metadata client implements jittered exponential backoff (base 1s, factor 2, max 3 retries, jitter 250ms) for HTTP 429 and transient connection drops. EnrichmentEngine defers discs with a 5-minute cooldown when rate limits persist, avoiding API bans while maintaining offline metadata label fallback for unreachable external services.
- Added `insecure_skip_verify` and `ca_cert_path` to SQLite schema, `IMAPConfig` store models, `BuildTLSConfig` helper in `internal/documents/tls.go`, background document polling in `poller.go`, and daemon API schemas (`/api/v1/documents/imap` and `/api/v1/documents/imap/test`).
- Generated updated TypeScript API types via `npm --prefix web run codegen`.


## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence & Routing |
|---|---|---|
| `internal/storage/dropzone.go`: integer overflow in `CheckCapacity` | `medium` | If `requiredBytes < 0` or exceeds `math.MaxInt64 - DropzoneSafetyBufferBytes`, integer addition wraps around and bypasses capacity check. Route: `patch`. |
| `cmd/sovereign-daemon/server.go`: unhandled non-507 error in `handleDropUpload` | `medium` | Filesystem inspection errors (e.g. permission/statfs) returned by `engine.CheckCapacity` were silently ignored instead of returning 500. Route: `patch`. |
| `internal/documents/tls.go`: system root CAs dropped when `caCertPath` set | `medium` | `BuildTLSConfig` created an empty cert pool with `x509.NewCertPool()`, dropping system trusted CAs. Must load `x509.SystemCertPool()`. Route: `patch`. |
| `internal/enrichment/client.go`: jitter integer negation overflow on `math.MinInt64` | `low` | Negating `int64(binary.LittleEndian.Uint64(...))` on `math.MinInt64` results in negative value. Must use unsigned modulo. Route: `patch`. |
| `internal/enrichment/client.go`: MusicBrainz recording tracks rate limit propagation | `low` | Detail recordings sub-query swallowed `ErrRateLimited`, returning partial metadata instead of triggering deferral cooldown. Route: `patch`. |
| `cmd/sovereign-daemon/server_test.go`: `TestDropUpload_InsufficientStorage` runtime bypass | `medium` | Pre-flight check intercepted request before `HandleUploadStream` because `httptest.NewRequest` populates ContentLength. Set `ContentLength = -1` to test runtime check. Route: `patch`. |
| `internal/enrichment/client_test.go`: missing transient network error retry test | `medium` | Verified gap: `TestClient_RateLimit_Backoff` only tested HTTP 429, not transient network drops. Route: `patch`. |
| `internal/documents/imap_test.go`: missing `Poller.PollOnce` with custom TLS test | `medium` | Verified gap: `PollOnce` with custom TLS against mock server was not tested. Route: `patch`. |
| `internal/optical/ripper.go`: over-broad `/dev/sr` path error match in `isDriveDisconnectError` | `low` | Path error on `/dev/sr` matched any error instead of checking underlying `EIO`/`ENODEV`/`ENOENT`/not-exist. Route: `patch`. |


## Design Notes

### Dropzone Capacity Check
- Safety buffer: `DropzoneSafetyBufferBytes = 1 * 1024 * 1024 * 1024` (1.0 GB).
- When `Content-Length` header is present on upload request, check available space before reading multipart stream: `available < contentLength + DropzoneSafetyBufferBytes`.
- If insufficient, respond immediately with HTTP 507 Insufficient Storage and RFC 7807 problem details:
  `{"type": "https://sovereign.local/errors/insufficient-storage", "title": "Insufficient Storage", "status": 507, "detail": "Target storage pool has insufficient space for this upload (1.0 GB safety buffer required)."}`

### Optical Drive Error Recovery
- Helper function `isDriveDisconnectError(err error) bool` tests for `syscall.EIO`, `syscall.ENODEV`, or os.PathError on `/dev/sr*`.
- When detected during `RipDisc`, break out of retry loop immediately, execute `os.RemoveAll(stagingTarget)`, set state to `StateError` with calm microcopy, and attempt `Eject` logging any hardware failure without blocking.

### Metadata Jittered Backoff
- `base_delay = 1s, factor = 2, max_retries = 3, jitter = 250ms`.
- Calculation: `delay = (base_delay * (1 << attempt)) + random_jitter(-250ms..+250ms)`.
- If final attempt fails with 429 or network offline, `EnrichmentEngine` adds disc to `deferredDiscs` map with `time.Now().Add(5 * time.Minute)` cooldown.

### IMAP TLS Customization
- In `internal/documents/tls.go`: `BuildTLSConfig(host string, insecureSkipVerify bool, caCertPath string) (*tls.Config, error)`.
- If `caCertPath != ""` and file exists, load PEM cert into `x509.NewCertPool()`.

## Verification

**Commands:**
- `go test -v -count=1 ./internal/storage -run TestDropzone_CapacityGuard` -- expected: PASS
- `go test -v -count=1 ./internal/optical -run TestRipper_DriveDisconnection` -- expected: PASS
- `go test -v -count=1 ./internal/enrichment -run TestClient_RateLimit_Backoff` -- expected: PASS
- `go test -v -count=1 ./internal/documents -run TestIMAP_CustomTLS` -- expected: PASS
- `go test -v -count=1 ./cmd/sovereign-daemon -run "TestDropUpload_InsufficientStorage|TestIMAP_CustomTLS"` -- expected: PASS
- `go test -count=1 ./...` -- expected: PASS
- `npm --prefix web run codegen` -- expected: PASS
- `npm --prefix web test -- --run` -- expected: PASS
