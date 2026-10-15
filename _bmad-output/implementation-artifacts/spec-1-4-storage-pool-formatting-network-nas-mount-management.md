---
title: 'Story 1.4: Storage Pool Formatting & Network NAS Mount Management'
type: 'feature'
created: '2026-10-15'
status: 'done'
baseline_commit: '26be1555237e3cc8e48d9137acb75a2fd480cca3'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The home server requires configuring physical internal storage (SATA/NVMe disks) or connecting an external network NAS (NFS/SMB shares) without manual Linux CLI commands, and must gracefully survive NAS network disconnections without crashing runtime containers.

**Approach:** Implement a storage management engine in `internal/storage` supporting local block device detection and journaled filesystem formatting under `/data`, non-destructive NFS/SMB credential verification tests, persistent mount configuration in SQLite, and dynamic health probing that transitions disconnected network shares to Amber degraded status without crashing services.

## Boundaries & Constraints

**Always:**
- Internal storage formatting must format with ext4 write-ahead journaling and mount the unified hierarchy under `/data` (`/data/{media,documents,models,state,drop}`).
- External NAS configurations (NFS, SMB) must undergo a non-destructive credential and mount accessibility test before being saved.
- Periodic health probes must detect NAS disconnects; when disconnected, storage status must transition to `degraded` (Amber) with consumer-friendly copy (*"Network storage temporarily unavailable"*), never crashing supervisor runtimes.
- Sensitive NAS credentials (SMB password) must be stored securely in the SQLite settings store.
- Support non-privileged / container / mock development environments through safe execution abstraction.

**Never:**
- Never execute destructive disk formatting on mounted root or boot partitions.
- Never crash supervisor daemon or container runtimes when external network storage drops out.
- Never expose raw mount error traces or kernel fs panic strings to users.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Enumerate Available Disks | GET `/api/v1/storage/disks` | Returns list of unmounted/available SATA/NVMe block devices with capacity | Returns empty list if no unmounted disks available |
| Format Local Storage Pool | POST `/api/v1/storage/initialize` with device path | Partitions and formats disk, mounts under `/data`, initializes subdirectories, returns 200 | Returns HTTP 400 Problem Details if device is invalid or in use |
| Test NAS Credentials | POST `/api/v1/storage/nas/test` with host, share, credentials | Performs non-destructive test mount/stat, returns 200 `{"success": true}` | Returns HTTP 400 Problem Details with clear explanation if auth fails |
| Save and Mount NAS | POST `/api/v1/storage/nas/mount` with verified credentials | Mounts share to `/data/media` or target folder, persists config in SQLite | Returns HTTP 500 Problem Details if mounting fails |
| NAS Disconnect in Runtime | Mounted NAS becomes unreachable | Periodic probe marks NAS `degraded` (Amber); daemon remains healthy; returns HTTP 200 with degraded telemetry | Automatically transitions back to `connected` (Green) when network recovers |

</frozen-after-approval>

## Code Map

- `/internal/storage/disks.go` -- Block device detection, SATA/NVMe inspection, and journaled formatting engine.
- `/internal/storage/nas.go` -- NFS/SMB mount management, non-destructive credential verification, and mount state persistence.
- `/internal/storage/probe.go` -- Periodic storage health monitor detecting NAS disconnections and updating degraded status.
- `/internal/storage/nas_test.go` -- Unit tests for NAS credential testing, mount management, and disconnection status transitions.
- `/cmd/sovereign-daemon/server.go` -- Storage REST endpoints: `/api/v1/storage/disks`, `/api/v1/storage/initialize`, `/api/v1/storage/nas/test`, `/api/v1/storage/nas/mount`, `/api/v1/storage/status`.
- `/cmd/sovereign-daemon/server_test.go` -- Unit tests covering storage endpoints, local disk format simulation, and NAS test/mount/disconnect states.
- `/api/openapi.yaml` -- Storage API definitions added to OpenAPI contract.
- `/web/src/lib/api/schema.d.ts` -- Generated TypeScript types for storage management.
- `/web/src/lib/api/client.ts` -- Storage API client methods: `getDisks`, `initializeStorage`, `testNAS`, `mountNAS`, `getStorageStatus`.

## Tasks & Acceptance

**Execution:**
- [x] `/internal/storage/disks.go` -- Implement block device inspection and ext4 formatting with write-ahead journaling -- Enables internal disk pool setup.
- [x] `/internal/storage/nas.go` -- Implement NFS/SMB non-destructive credential testing, mount execution, and SQLite config persistence -- Enables network NAS connectivity.
- [x] `/internal/storage/probe.go` -- Implement storage health probing detecting NAS disconnects and maintaining Amber degraded state -- Prevents runtime service crashes.
- [x] `/internal/storage/nas_test.go` -- Unit tests for disk enumeration, NAS credential testing, and disconnection handling -- Validates storage engine.
- [x] `/cmd/sovereign-daemon/server.go` -- Register `/api/v1/storage/*` HTTP REST endpoints with RFC 7807 error handling -- Exposes storage APIs.
- [x] `/cmd/sovereign-daemon/server_test.go` -- Test storage endpoints (disks, initialize, nas test, nas mount, status) -- Verifies daemon storage integration.
- [x] `/api/openapi.yaml` -- Document storage endpoints in OpenAPI 3.1 contract -- Contract single source of truth.
- [x] `/web/src/lib/api/` -- Regenerate TypeScript types and add storage methods to `SovereignApiClient` -- Provides frontend client integration.

**Acceptance Criteria:**
- Given unformatted internal SATA/NVMe disks, when selecting local storage initialization, then `sovereign-daemon` partitions and mounts the disk under `/data` with write-ahead journaling.
- Given an existing NAS configuration, when testing credentials, then NFS or SMB share credentials undergo a non-destructive mount verification test before being saved.
- Given a mounted NAS that becomes temporarily disconnected, then the daemon transitions storage status to Amber degraded with an explanatory notice without crashing service runtimes.
- Given the OpenAPI specification, when inspected, then `/api/v1/storage/*` endpoints are defined and matched by generated TypeScript client methods.

## Implementation Notes

- Implemented `internal/storage/disks.go` with `DiskOps` abstraction, `/sys/block` block device detection, ext4 write-ahead journaling format simulation/execution, and protection guards preventing formatting of mounted root or boot partitions.
- Implemented `internal/storage/nas.go` supporting NFS and SMB non-destructive credential verification, mount execution, SQLite `settings` table persistence, and credential redaction (`***`).
- Implemented `internal/storage/probe.go` with non-blocking network socket reachability checks, periodic probing, and automatic transition to Amber degraded status (`"Network storage temporarily unavailable"`) on disconnect and recovery back to Green connected status when network restores, ensuring the supervisor daemon never crashes.
- Implemented `internal/storage/storage.go` coordinating the unified `Engine`.
- Registered REST endpoints in `cmd/sovereign-daemon/server.go` (`/api/v1/storage/disks`, `/api/v1/storage/initialize`, `/api/v1/storage/nas/test`, `/api/v1/storage/nas/mount`, `/api/v1/storage/status`) with RFC 7807 error handling.
- Documented all endpoints and schemas in `api/openapi.yaml`, regenerated TypeScript types in `web/src/lib/api/schema.d.ts`, and added client methods to `SovereignApiClient`.
- Unit tests added in `internal/storage/nas_test.go`, `cmd/sovereign-daemon/server_test.go`, and `web/src/lib/api/client.test.ts`. All Go and TypeScript test suites pass.

## Spec Change Log

## Review Triage Log

| ID | Layer | Finding | Verdict | Evidence / Disposition |
|---|---|---|---|---|
| B1 | blind-hunter | Initializing local pool at /data obscures running SQLite DB | medium | State path within mount point can be affected. Route: patch. |
| B2 | blind-hunter | Boot-time remounting from persisted config missing | medium | Engine should re-establish mount configuration on daemon boot. Route: patch. |
| B3 | blind-hunter | Command-line SMB credential exposure in mount arguments | medium | /proc/pid/cmdline exposes password; use temporary credentials file. Route: patch. |
| B4 | blind-hunter | Commas in SMB passwords corrupt mount options | medium | Fixed by using temporary credentials file. Route: patch. |
| B5 | blind-hunter | Unvalidated target_path allows arbitrary mount points | high | target_path must be restricted to subdirectories within dataDir. Route: patch. |
| B6 | blind-hunter | Unvalidated options string in NAS requests | low | Whitelist safe mount options. Route: patch. |
| B7 | blind-hunter | TestConnection does not verify credentials | medium | TCP dial only verifies port; test authentication or mount probe. Route: patch. |
| B8 | blind-hunter | ProbeOnce holds exclusive lock across network I/O | high | Serializes all HTTP requests during network latency. Route: patch. |
| B9 | blind-hunter | Missing unmount endpoint in API | low | Unmount endpoint scheduled for settings hub in Epic 6. Route: reject. |
| B10 | blind-hunter | Concurrency synchronization on format and mount missing | low | Add mutex in storage Engine to serialize mutating operations. Route: patch. |
| B11 | blind-hunter | Goroutine leak on hung NFS stat | low | Context with timeout already used. Route: reject. |
| B12 | blind-hunter | Capacity telemetry reflects root rather than NAS | low | Acceptable for scaffold; fine-grained volume metrics in Epic 1 Story 1.6. Route: reject. |
| B13 | blind-hunter | StorageNASStatus enum and required message violation | low | Return valid status enum and message when unconfigured. Route: patch. |
| B14 | blind-hunter | Flawed device prefix matching in IsSystemDevice | medium | Check exact device or partition prefix ('p' or digits). Route: patch. |
| B15 | blind-hunter | Safe mode detection ignores container UID 0 without caps | low | Safe mode fallback verified. Route: reject. |
| E1 | edge-case-hunter | Device name prefix collision in IsSystemDevice | medium | Duplicate of B14. Route: patch. |
| E2 | edge-case-hunter | target_path traversal outside dataDir | high | Duplicate of B5. Route: patch. |
| E3 | edge-case-hunter | Password comma delimiter in mount options | medium | Duplicate of B4. Route: patch. |
| E4 | edge-case-hunter | Database persistence failure after mount orphans share | low | Rollback mount if DB persistence fails. Route: patch. |
| E5 | edge-case-hunter | Probe loop running flag not reset on context cancel | low | Reset running flag in defer. Route: patch. |
| E6 | edge-case-hunter | Mutex contention during probe accessibility check | high | Duplicate of B8. Route: patch. |
| E7 | edge-case-hunter | Host string already contains port in net.JoinHostPort | low | Parse with net.SplitHostPort. Route: patch. |
| V1 | verification-gap | Persistence restoration from SQLite on daemon startup unverified | medium | Missing test verifying restored storage status across new server instance. Route: patch. |
| V2 | verification-gap | ErrDeviceInUse guard for mounted non-system devices unverified | medium | Missing test for mounted non-system device rejection. Route: patch. |
| V3 | verification-gap | POST /nas/mount error handling unverified | medium | Missing tests for invalid credentials and unreachable host errors. Route: patch. |
| V4 | verification-gap | Initial unconfigured state of GET /storage/status unverified | medium | Missing test asserting status: not_configured on fresh boot. Route: patch. |
| V5 | verification-gap | ListDisks checks len(mockDisks) > 0 instead of mockDisks != nil | low | Empty slice []DiskInfo{} falls through to host sys/block. Route: patch. |


## Design Notes

- Mock / Container Driver: When running in testing or containerized dev without raw block device access, the storage engine detects absence of root block capabilities and operates in virtual/safe mode using directory mounts.
- NAS Credentials Security: Passwords stored in SQLite `settings` table are isolated and never returned in plaintext in GET `/api/v1/storage/status` (only redacted `***`).

## Verification

**Commands:**
- `go test -v ./...` -- expected: All unit tests pass, verifying disk formatting simulation, NAS testing, and degraded disconnection handling.
- `npm --prefix web test` -- expected: All client tests pass with storage API methods.
- `npm --prefix web run build` -- expected: Next.js builds cleanly with generated storage types.
