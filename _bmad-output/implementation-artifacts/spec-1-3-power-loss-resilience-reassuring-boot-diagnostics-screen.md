---
title: 'Story 1.3: Power-Loss Resilience & Reassuring Boot Diagnostics Screen'
type: 'feature'
created: '2026-10-15'
status: 'review'
baseline_commit: '2c7622f2fc5e38a45db9534fb77c823e85e5b4cb'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Following an abrupt household power outage, server databases may risk corruption and household users need clear reassurance without terrifying Linux terminal logs or error codes while verification runs.

**Approach:** Implement a tiered boot recovery engine in `sovereign-daemon` that reads `/data/state/boot_state.json`, triggers `DiagnosticsMode` and runs SQLite `PRAGMA integrity_check` on unclean shutdown, serves reassuring consumer copy on `/boot` in the web client with circular progress animation, and writes atomic clean shutdown markers upon graceful termination.

## Boundaries & Constraints

**Always:**
- If `/data/state/boot_state.json` is missing or has `clean_shutdown: false`, `sovereign-daemon` must initialize in `DiagnosticsMode`.
- In `DiagnosticsMode`, calls to `/` or client-side checks must redirect to `/boot`.
- The `/boot` page must render the exact consumer reassurance copy: *"Checking system safety after power restoration... your files are secure."* with a circular progress animation.
- The daemon must run `PRAGMA integrity_check;` on `/data/state/sovereign.db` and verify `/data` storage mount writeability within 15 seconds.
- Upon passing integrity verification, the `/boot` page must automatically redirect to `/` within 2 seconds.
- A graceful shutdown (SIGINT/SIGTERM or explicit signal) must atomically write `clean_shutdown: true` with a UTC timestamp to `/data/state/boot_state.json`.
- When `clean_shutdown: true` is present, the server must boot into `Operational` mode and render `/` directly within 15 seconds without showing the diagnostics screen.
- All writes to `boot_state.json` must use atomic temporary-file creation, `fsync()`, and rename.

**Never:**
- Never display raw Linux kernel errors, database stack traces, or terminal log jargon to the consumer on `/boot`.
- Never launch upstream application containers while `DiagnosticsMode` integrity checks are in flight.
- Never write non-atomic changes directly to `/data/state/boot_state.json`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cold Boot after Unclean Power Cut | Missing or `clean_shutdown: false` in `boot_state.json` | Daemon enters `DiagnosticsMode`; `/api/v1/system/boot-status` returns `mode: "diagnostics"`; web client redirects to `/boot` | If integrity check fails, status becomes `failed` and prompts for automated snapshot restore |
| Integrity Check Passes | `PRAGMA integrity_check` returns `"ok"` | Updates state to `mode: "operational"`, `diagnostics_completed: true`; `/boot` auto-redirects to `/` in <=2s | If DB corrupted, marks `diagnostics_completed: false`, logs structured error |
| Graceful Appliance Shutdown | Daemon receives `SIGINT` or `SIGTERM` | Flushes DB WAL, writes `clean_shutdown: true` to `/data/state/boot_state.json`, exits 0 | If write fails, logs error before exit |
| Normal Boot after Clean Shutdown | `boot_state.json` contains `clean_shutdown: true` | Daemon enters `operational` mode directly; visits to `/` render dashboard in <15s | N/A |
| Storage Unwritable on Boot | `/data/state` filesystem read-only | Diagnostics detects read-only mount, reports degraded state | Returns HTTP 503 Problem Details |

</frozen-after-approval>

## Code Map

- `/internal/store/boot.go` -- Boot state file manager (`boot_state.json`), dirty shutdown detection, atomic write, and SQLite integrity verification.
- `/internal/store/boot_test.go` -- Unit tests for boot state reading, writing, atomic replacement, and integrity verification.
- `/cmd/sovereign-daemon/server.go` -- Update `/api/v1/system/boot-status` handler to read real-time boot status and integrity state from `BootManager`.
- `/cmd/sovereign-daemon/main.go` -- Integrate boot lifecycle verification on startup and clean shutdown writing on signal trap.
- `/web/src/app/boot/page.tsx` -- Dedicated reassurance boot screen with circular progress animation and auto-redirect.
- `/web/src/app/boot/boot-screen.test.tsx` -- Component test verifying copy, progress rendering, and redirect trigger.
- `/web/src/app/page.tsx` -- Updated dashboard page checking boot status and redirecting to `/boot` if in `diagnostics` mode.

## Tasks & Acceptance

**Execution:**
- [x] `/internal/store/boot.go` -- Implement `BootManager` tracking `/data/state/boot_state.json`, `RecordBoot`, `RecordCleanShutdown`, and `RunIntegrityCheck` -- Provides durable boot lifecycle and power resilience.
- [x] `/internal/store/boot_test.go` -- Unit tests verifying dirty boot detection, clean shutdown atomic persistence, and SQLite integrity checking -- Ensures power-loss safety verification.
- [x] `/cmd/sovereign-daemon/server.go` -- Wire `BootManager` into `/api/v1/system/boot-status` endpoint -- Exposes live boot diagnostics telemetry.
- [x] `/cmd/sovereign-daemon/main.go` -- Run diagnostics on startup if dirty boot detected; register signal trap for clean shutdown -- Implements appliance lifecycle invariants.
- [x] `/web/src/app/boot/page.tsx` -- Implement `/boot` page with circular progress indicator, reassurance copy, and 2s auto-redirect to `/` -- Delivers reassurance UX per UX-DR7.
- [x] `/web/src/app/page.tsx` -- Add client/SSR boot status check redirecting to `/boot` when `mode === "diagnostics"` -- Enforces diagnostics screen routing.
- [x] `/web/src/lib/api/client.test.ts` -- Add/update tests for boot status polling and diagnostics state -- Validates frontend client contract.

**Acceptance Criteria:**
- Given an ungraceful shutdown without a clean shutdown record in `/data/state/boot_state.json`, when the system powers on, then `sovereign-daemon` enters `DiagnosticsMode` and client visits redirect to `/boot`.
- Given the `/boot` screen, then it displays *"Checking system safety after power restoration... your files are secure."* with a circular progress animation.
- Given `sovereign-daemon` startup in `DiagnosticsMode`, then it executes `PRAGMA integrity_check` on `/data/state/sovereign.db` within 15 seconds.
- Given successful integrity verification, then the `/boot` screen automatically redirects to `/` within 2 seconds.
- Given a graceful reboot or shutdown, then the daemon writes `clean_shutdown: true` and renders `/` directly on the next boot within 15 seconds.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

- Reassurance Microcopy: Must adhere strictly to UX-DR7 & UX-DR11: *"Checking system safety after power restoration... your files are secure."* No stack traces or raw table counts.
- Circular Progress: Pure CSS / SVG circular indeterminate / determinate spinner using design token `accent-blue` (`#2563EB`) and smooth ease-in-out transition.
- Atomic File Write: Temp file in same directory (`/data/state/.boot_state.json.tmp`), `fsync()`, then `os.Rename`.

## Verification

**Commands:**
- `go test -v ./...` -- expected: All unit tests pass, verifying boot state transitions, integrity checks, and clean shutdown writing.
- `npm --prefix web test` -- expected: All web client tests pass.
- `npm --prefix web run build` -- expected: Next.js builds cleanly with `/boot` page.
