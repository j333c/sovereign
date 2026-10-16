---
title: 'Story 3.1: Optical Drive Detection & Automated Disc Ripping Engine'
type: 'feature'
created: '2026-10-15'
status: 'done'
route: 'dispatch'
baseline_commit: '36cce8a399872d46c7ec42d187cc7fe52e8066a2'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Alex has boxes of physical Blu-rays, DVDs, and Audio CDs and wants to insert a disc into the server's optical drive and walk away while it automatically rips, monitors read health, and auto-ejects, without fiddling with desktop GUI software, command-line ripping tools, or manual file staging.

**Approach:** Implement an optical drive supervisor and automated disc ripping engine in `internal/optical/` that polls hardware optical block devices (`/dev/sr0`, sysfs) within 10 seconds of disc insertion, inspects disc types (CD, DVD, Blu-ray), enforces storage headroom safety (pausing if storage free < 10%), automatically rips unencrypted DVDs and Audio CDs to `/data/media/staging/discs/`, validates user-supplied `KEYDB.cfg` (`/data/state/keys/KEYDB.cfg`) for encrypted Blu-rays with graceful auto-ejection and clear guidance if keys are absent, retries read errors up to 3 times before failing safely, automatically ejects the disc tray upon rip completion, updates live SSE telemetry for `TelemetryBar` (`Idle · Ready for disc` -> `Disc detected` -> `Ripping (X%)` -> `Auto-ejecting disc`), and exposes REST management endpoints (`GET /api/v1/optical/status`, `POST /api/v1/optical/eject`, `POST /api/v1/optical/keys`).

## Boundaries & Constraints

**Always:**
- Detect optical disc insertion within 10 seconds (default polling interval <= 3s).
- Identify disc media type: Audio CD, DVD Video/Data, Blu-ray (BD-ROM).
- Enforce a 10% disk capacity headroom floor; pause ripping if free storage space drops below 10% to protect system WAL databases.
- Stage ripped discs under `/data/media/staging/discs/:disc_id/`.
- For commercial encrypted Blu-rays (AACS): verify presence of `/data/state/keys/KEYDB.cfg`. If keys are missing, safely auto-eject the tray with consumer microcopy: *"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."*
- Automatically eject the drive tray upon rip completion or non-recoverable error.
- Enforce a maximum of 3 retries on read/scratch errors before safe ejection and dispatching an error alert.
- Stream real-time ripping state and progress percentage through `TelemetryBar` optical widget via daemon SSE telemetry.
- Provide a `MockOpticalDriver` interface allowing deterministic unit and integration testing without physical optical hardware.

**Never:**
- Never execute destructive or raw shell commands directly on host devices without boundary validation.
- Never hang or crash the daemon when an optical disc is unreadable, dirty, or scratched.
- Never overwrite existing ripped staging archives without unique ID hashing.
- Never mount the host Docker socket into optical worker containers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Disc Insertion (DVD/CD) | Unencrypted DVD or Audio CD inserted | Detected in <=10s; state transitions to `Disc detected` -> `Ripping (0%)` -> `Ripping (100%)` -> auto-eject | Retries read up to 3 times on read error |
| Encrypted Blu-ray (No Keys) | Commercial AACS Blu-ray inserted; `KEYDB.cfg` missing | Disc detected; checks key presence; safely auto-ejects with explanatory notice | Reassuring consumer microcopy; no crash |
| Encrypted Blu-ray (With Keys) | Commercial Blu-ray inserted; valid `KEYDB.cfg` present | Uses `KEYDB.cfg` keys; rips raw streams to staging; auto-ejects on complete | Retries on read error |
| Storage Headroom < 10% | Free storage space is < 10% | Pauses ripping; emits warning in status; ejects disc with storage warning notice | Prevents disk full condition |
| Scratched / Unreadable Disc | Read errors encountered during ripping | Retries up to 3 times; if unreadable, ejects tray and emits alert event | Logs error; sends maintenance alert |
| Manual Eject Request | `POST /api/v1/optical/eject` | Issues eject command to optical driver; transitions to `Idle · Ready for disc` | Returns 200 OK |
| Upload KEYDB.cfg | `POST /api/v1/optical/keys` | Saves file atomically to `/data/state/keys/KEYDB.cfg`; marks keys active | 400 on empty or invalid file |

</frozen-after-approval>

## Code Map

- `internal/optical/driver.go` -- Hardware optical driver interface, Linux ioctl / `eject` wrapper, and `MockOpticalDriver`.
- `internal/optical/ripper.go` -- Automated disc ripping engine: disc type detection, AACS key verification, 10% headroom guard, 3-retry read loop, and auto-ejection.
- `internal/optical/keys.go` -- `KEYDB.cfg` storage manager and parser.
- `internal/optical/ripper_test.go` -- Unit tests for disc detection, ripping progress, encryption key checks, headroom checks, read retries, and auto-eject.
- `cmd/sovereign-daemon/server.go` -- Wire optical ripper into daemon background services, link status to SSE `optical_status`, and register `/api/v1/optical/*` endpoints.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for optical status API, manual eject, KEYDB upload, and telemetry stream updates.
- `api/openapi.yaml` -- OpenAPI specifications for optical endpoints (`/api/v1/optical/status`, `/api/v1/optical/eject`, `/api/v1/optical/keys`).

## Tasks & Acceptance

**Execution:**
- [x] `internal/optical/driver.go` -- Implement optical driver interface and mock driver for hardware simulation.
- [x] `internal/optical/keys.go` -- Implement `KEYDBManager` for `/data/state/keys/KEYDB.cfg` upload and verification.
- [x] `internal/optical/ripper.go` -- Implement `OpticalRipper` with 10s detection loop, 10% headroom guard, retry loop, and auto-eject.
- [x] `internal/optical/ripper_test.go` -- Unit tests covering DVD/CD ripping, encrypted Blu-ray handling, headroom limits, retry guards, and auto-eject.
- [x] `cmd/sovereign-daemon/server.go` -- Register `/api/v1/optical/status`, `/api/v1/optical/eject`, `/api/v1/optical/keys` and wire optical status into SSE telemetry.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for optical API endpoints and SSE telemetry updates.
- [x] `api/openapi.yaml` -- Document optical endpoints in OpenAPI schema.

**Acceptance Criteria:**
- Given an optical drive connected to the server, when a disc is inserted, `sovereign-daemon` detects media insertion within 10 seconds.
- Unencrypted DVDs and CDs automatically rip to `/data/media/staging/discs/` without user intervention.
- Commercial encrypted Blu-rays utilize `KEYDB.cfg` uploaded in Settings, or safely eject with an explanatory notice.
- Upon rip completion, the optical drive tray automatically ejects.
- If reading fails after 3 retries, the disc ejects and dispatches an alert.

## Implementation Notes

- **Optical Driver Subsystem** (`internal/optical/driver.go`):
  - Defined `OpticalDriver` interface with `GetDriveStatus`, `Eject`, `CloseTray`, and `RipDisc`.
  - Implemented `LinuxOpticalDriver` using boundary-validated device checks (`/dev/sr[0-9]+`), blkid disc inspection, and safe eject commands.
  - Implemented `MockOpticalDriver` with simulated drive status, configurable retry failures, rip delays, custom hooks, and eject counters.
- **KEYDB Decryption Key Storage** (`internal/optical/keys.go`):
  - Created `KEYDBManager` managing `/data/state/keys/KEYDB.cfg`.
  - Validates Doom9 libaacs format (disc VUKs, processing keys PK, device keys DK, host certs HC); rejects empty or binary payloads.
  - Atomically writes file using unique temp files, `fsync()`, and rename.
- **Automated Ripping Engine** (`internal/optical/ripper.go`):
  - Created `OpticalRipper` with configurable polling interval (default 2s <= 3s) detecting discs within 10 seconds.
  - Added 10% storage capacity headroom protection (`headroomChecker`); automatically pauses ripping and safely ejects the disc with warning microcopy if storage drops below 10%.
  - Added commercial encrypted Blu-ray check; auto-ejects with exact consumer notice: *"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."*
  - Added unique directory hashing under `/data/media/staging/discs/:disc_id/` to prevent overwriting prior staging archives.
  - Added 3-retry read loop with backoff; on hard failure auto-ejects with notice: *"Disc reading failed after multiple attempts. The disc has been safely ejected."* and broadcasts `optical.read_failed` and `optical.alert`.
  - Added rip completion handler updating progress to 100%, setting state to `Auto-ejecting disc`, and calling hardware eject.
- **Server Integration & Endpoints** (`cmd/sovereign-daemon/server.go`):
  - Configured `OpticalDriver`, `OpticalRipper`, and `KEYDBManager` in daemon server.
  - Wired real-time `optical_status` into Server-Sent Events `telemetry.update` and domain events (`optical.disc_detected`, `optical.rip_progress`, `optical.disc_ejected`, `optical.read_failed`, `optical.alert`).
  - Registered `/api/v1/optical/status` (GET), `/api/v1/optical/eject` (POST), and `/api/v1/optical/keys` (POST) supporting multipart and raw uploads.
- **OpenAPI 3.1 Documentation** (`api/openapi.yaml`):
  - Added paths and schemas for `/api/v1/optical/status`, `/api/v1/optical/eject`, and `/api/v1/optical/keys`.
- **Verification**:
  - Full unit test coverage in `internal/optical/ripper_test.go`.
  - Full integration test coverage in `cmd/sovereign-daemon/server_test.go`.
  - All project packages pass `go test ./...`.

## Spec Change Log

- Status transitioned from `in-progress` to `review`.
- Status transitioned from `review` to `done` after 3-layer review triage and verification.

## Review Triage Log

- **Layer 1: Blind Hunter**:
  - Implemented cancellation propagation (`ripCancel context.CancelFunc`) in `OpticalRipper.Eject()` and `Close()` to immediately abort active rip loops without waiting on retries.
  - Added staging directory cleanup (`os.RemoveAll(stagingTarget)`) upon hard read failures to avoid leaving corrupt staging files on disk.
  - Added 10MB maximum request size limit on `POST /api/v1/optical/keys` returning RFC 7807 `413 Payload Too Large`. Documented 413, 401, 403 in `api/openapi.yaml`.
  - Upgraded collision suffix in staging directory generation from truncated high-order timestamp to cryptographic random hex (`crypto/rand`).
- **Layer 2: Edge Case Hunter**:
  - Retained multiple processing keys (`PK_%d_%s`, `DK_%d_%s`, `HC_%d_%s`) instead of overwriting a single `"PK"` in `KEYDBManager`.
  - Added strict hex string validation (`isHexString`) in `KEYDBManager` to reject malformed key entries.
  - Added deterministic fallback ID generation (`unlabeled_type_size` / `unlabeled_type_timestamp`) when label and UUID are empty, preventing infinite re-ripping loops.
  - Added sysfs fallback (`/sys/class/block/srX/size`) in `LinuxOpticalDriver` to reliably detect Audio CDs (CD-DA) when `blkid` exits non-zero.
  - Configured `RipDisc` to produce `track01.cdda` for `DiscTypeCD`.
  - Added tray reset delay (`ejectResetDelay`) so UI can observe `"Auto-ejecting disc"` status before resetting to idle, with `SetEjectResetDelay(0)` for unit testing.
  - Preserved `StatePaused` across polling intervals until storage headroom is restored.
  - Fixed exact consumer microcopy for `NoticeStorageLow`: `"Optical ripping paused: storage capacity is below 10%. Please free up space to continue."`.
- **Layer 3: Verification Gap Reviewer**:
  - Fixed polling loop lifecycle management using `sync.WaitGroup` to eliminate deadlocks when `Close()` was called on unstarted rippers.
  - Created comprehensive `internal/optical/driver_test.go` verifying `parseBlkidOutput`, sysfs Audio CD fallback, mock driver behaviors, and boundary device path regex rejection (`/dev/sda`, shell injections).
  - Added unit tests for cancellation, staging cleanup on hard failure, background polling loop, existing key loading from disk, and headroom checker in `internal/optical/ripper_test.go`.
  - Added integration tests for 10MB payload limit (413) and active ripping telemetry stream verification (`"Ripping (42%)"`, `"Auto-ejecting disc"`) in `cmd/sovereign-daemon/server_test.go`.
