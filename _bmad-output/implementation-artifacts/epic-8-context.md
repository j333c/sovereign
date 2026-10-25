# Epic 8 Context: Operational Hardening, Edge-Case Resilience & Verification Safety Net

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Establish bulletproof operational resilience, error recovery, and verification safeguards across the Sovereign appliance substrate: enforce atomic SQLite database transactions during setup and graceful WAL checkpointing before snapshot restorations; guard rolling service updates with rollback circuit breakers; provide pre-flight storage guards for dropzone uploads and model downloads; implement robust error handling and retries for optical ripping, metadata enrichment, and IMAP SSL connections; establish an end-to-end integration safety net for long-running SSE connections and notification retry queues; fix multilingual confirmation token validation and hardware calculator baselines; and synchronize public GitHub distribution with internal development via dual remotes. This ensures the appliance operates reliably under dirty shutdowns, peripheral disconnects, and edge conditions with zero data loss or unrecoverable states.

## Stories

- Story 8.1: Core Daemon Transaction Atomicity, WAL Restoration Checkpointing & Rollback Circuit Breaker
- Story 8.2: Ingestion Pre-Flight Capacity Guards & Peripheral Error Recovery
- Story 8.3: Critical Path Fault-Injection & Long-Running Stream Verification Suite
- Story 8.4: Multilingual Modal Token Validation & Hardware Calculator Baseline Fallback
- Story 8.5: Public GitHub Repository Distribution & Dual-Remote Synchronization

## Requirements & Constraints

- **Setup Transaction Atomicity**: The First-Run Setup completion endpoint (`CompleteSetup`) must wrap all state mutations in an atomic database transaction. Any failure or abort during provisioning must roll back cleanly, leaving no orphaned rows or partial state in `sovereign.db`.
- **Database Snapshot Restoration Protocol**: When restoring a database snapshot during automated or manual service rollback, the active SQLite connection must be cleanly closed and a `PRAGMA wal_checkpoint(TRUNCATE)` executed prior to replacing `/data/state/sovereign.db` on disk, preventing file locking conflicts and corrupted `.db-shm` / `.db-wal` files.
- **Rollback Circuit Breaker**: Rolling container updates that fail the 180-second healthcheck trigger automated rollback. Rollback retry attempts must be capped (maximum 2 attempts) to prevent infinite rollback loops, escalating to a logged emergency alert state if the previous revision also fails.
- **Docker Socket Disconnect Guard**: When the supervisor daemon runs without access to `/var/run/docker.sock`, telemetry streaming (SSE) and polling must surface an explicit, reassuring warning indicator rather than panicking with nil-pointer dereferences or silent container state failures.
- **Dropzone Pre-Flight Capacity Guard**: Ingestion endpoints for `/drop` (and local network file drops) must inspect target storage pool capacity before accepting file data, enforcing an upload size + 1.0 GB safety buffer and rejecting insufficient capacity with a calm, informative error message.
- **Peripheral & Optical Error Recovery**: The optical disc ripping worker must handle bus resets, drive disconnects (`EIO`), and tray eject failures gracefully by terminating the rip task cleanly, purging temporary staging files, logging descriptive errors, and preventing daemon deadlocks.
- **Resilient Metadata Enrichment**: Third-party metadata enrichment queries (TMDb, MusicBrainz) must handle transient connection drops and HTTP 429 rate limits by applying exponential backoff with jitter (up to 3 retries) before deferring, preventing API bans and tight retry loops.
- **IMAP Custom TLS Support**: Document email ingestion must allow configuring custom or self-signed TLS certificates and trusted internal CAs without failing the TLS handshake.
- **Long-Running Stream Endurance**: Server-Sent Events (SSE) telemetry connections must withstand continuous streaming for $\ge 300$ seconds without server-side write timeout drops, mid-stream memory leaks, or unhandled socket closure errors.
- **Persistent Notification Dispatch Queue**: Urgent alerts (contract deadlines, hardware degraded states) dispatched to SMTP or Home Assistant webhooks must be enqueued in a persistent retry queue that retries upon failure and guarantees delivery when endpoints recover.
- **Model Storage Reservation Guard**: Local AI GGUF downloads must verify free storage before transfer; downloads must abort with HTTP 507 Insufficient Storage if free space is less than `model_size + 5.0 GB`, leaving the reserve untouched and generating no partial files.
- **Multilingual Token Matching**: The `DestructiveModal` confirmation input must validate localized tokens ("RESET", "LÖSCHEN", "ELIMINAR", "SUPPRIMER", "DELETE") case-insensitively according to the user's active locale.
- **Hardware Calculator Baseline**: When all feature switches and sliders are zero or unselected, the hardware calculator must render the Light Tier baseline (Dual-core x86/ARM64, 4 GB RAM, 64 GB storage) rather than rendering empty or broken cards.
- **Turnkey Warranty Boundaries**: Hardware roadmap and documentation must clearly articulate Apache 2.0 open-source warranty disclaimers and community support boundaries for DIY vs future pre-assembled appliance offerings.
- **Dual-Remote Repository Synchronization**: Sovereign maintains dual remotes: authoritative internal development on `origin` (`http://git.crins:3000/jens/sovereign`) and public distribution on `github` (`https://github.com/j333c/sovereign`), excluding private configs (`*.user.toml`) while ensuring open-source users have everything needed to build, update, and run the appliance.

## Technical Decisions

- **Database Engine & WAL Lifecycle**: Embedded SQLite compiled without CGO (`modernc.org/sqlite`) with WAL mode enabled (`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL;`). Snapshot restorations explicitly invoke `PRAGMA wal_checkpoint(TRUNCATE);` before connection teardown and file replacement.
- **Supervisor Rollback State Machine**: Implemented in `internal/supervisor/revision.go`. Tracks rollback attempts per revision with a circuit breaker threshold (`max_rollback_attempts = 2`). If threshold is exceeded, enters `SafeRecovery` state and publishes an `emergency.rollback_loop` SSE event.
- **Docker Socket Health Probe**: Daemon initializes with a Docker ping check; if unavailable, daemon enters degraded state and streams `system.docker_unavailable` telemetry with calm consumer copy while keeping non-container features (storage, dropzone, setup) responsive.
- **Storage Pool Inspection**: Ingestion handlers (`internal/storage/`) use filesystem `Statfs` calls to verify free bytes before streaming uploads or downloading models, applying standardized safety buffers (`+1.0 GB` for media dropzone, `+5.0 GB` for local AI models).
- **HTTP 507 & RFC 7807 Error Shapes**: Pre-flight storage rejections return HTTP 507 with RFC 7807 Problem Details (`type: "https://sovereign.local/errors/insufficient-storage"`).
- **Exponential Backoff Helper**: Metadata enrichment requests use a jittered exponential backoff algorithm (`base_delay = 1s, factor = 2, max_retries = 3, jitter = 250ms`).
- **TLS Configuration for IMAP**: Ingestion configuration schema supports custom CA certificate paths (`ca_cert_path`) or an `insecure_skip_verify` boolean flag (strictly restricted to authenticated admin settings).
- **HTTP Server Write Timeout**: To support long-running SSE telemetry streams ($\ge 300\text{s}$), the HTTP server configuration in `cmd/sovereign-daemon/main.go` either sets `WriteTimeout = 0` (disabled) or wraps SSE endpoints in streaming response flushers bypassing standard request write deadlines.
- **Persistent Alert Queue**: Backed by SQLite table `alert_dispatch_queue` with status tracking (`pending`, `delivered`, `failed`), retry count, and next attempt timestamp.
- **Git Dual-Remote Conventions**: Development remote is `origin` (`http://git.crins:3000/jens/sovereign`), public release remote is `github` (`https://github.com/j333c/sovereign.git`). Personal configs (`*.user.toml`) are ignored in `.gitignore`.

## UX & Interaction Patterns

- **Reassuring Error Microcopy**: All error indicators (insufficient storage, Docker socket disconnection, optical rip failure) avoid raw unix errors (`EIO`, `ENOSPC`, stack traces) and provide calm, actionable next steps.
- **DestructiveModal Multilingual Token Entry**: Accessible confirmation dialog accepts case-insensitive localized tokens (e.g., German `LÖSCHEN`, Spanish `ELIMINAR`, French `SUPPRIMER`, English `RESET`/`DELETE`) mapped to the active language dictionary (`web/src/lib/i18n.ts`).
- **Telemetry Disconnect Alert**: When Docker socket or a peripheral is disconnected, the `TelemetryBar` transitions to an Amber diamond (◆) state displaying a clear explanation (e.g. *"Container engine unavailable — background tasks paused"*).
- **Dropzone Capacity Rejection**: Displays an inline banner in `DropzoneCard` stating required free space and current available capacity without failing silently or dropping the browser connection.
- **Hardware Calculator Fallback**: Interactive calculator defaults to the Light Tier card when inputs are cleared, showing base specifications and estimated power consumption.

## Cross-Story Dependencies

- **Story 8.1 (Core Daemon Atomicity & Circuit Breakers)** hardens the supervisor daemon and database store foundations (`internal/store`, `internal/supervisor`), which the testing suite in **Story 8.3** verifies under fault-injection.
- **Story 8.2 (Ingestion Capacity Guards & Peripheral Recovery)** adds storage safety checks and retry mechanisms that interact with the dropzone (`internal/storage`) and AI model downloader (`internal/ai`), which **Story 8.3** tests for HTTP 507 and queue retries.
- **Story 8.3 (Fault-Injection & Stream Endurance Suite)** builds the verification safety net ensuring guarantees introduced in Stories 8.1 and 8.2 (SSE longevity, 5GB reservation, notification queue) remain regression-free.
- **Story 8.4 (Multilingual Tokens & Calculator Fallback)** closes frontend UX gaps in `web/src/components/DestructiveModal.tsx` and `web/src/app/hardware/page.tsx`, building upon the i18n foundation from Epic 6 and the hardware profiles from Epic 7.
- **Story 8.5 (Public Distribution & Dual Remotes)** establishes the clean distribution baseline and remote synchronization required to publish the hardened appliance to the open-source community.
