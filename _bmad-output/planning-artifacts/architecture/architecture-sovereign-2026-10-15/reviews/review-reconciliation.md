# Architecture Reconciliation Review: ARCHITECTURE-SPINE.md vs. PRD & UX Specs

**Document Reviewed**: `_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md`  
**Inputs Reconciled**:
- `_bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/prd.md`
- `_bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/architecture-decisions.md`
- `_bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/addendum.md`
- `_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md`

**Review Date**: 2026-10-15  
**Reviewer**: Antigravity Architecture Reviewer  
**Verdict**: **CONCERNS**

---

## 1. Executive Summary

The `ARCHITECTURE-SPINE.md` provides a well-structured architectural foundation based on the **Supervised Hub-and-Spoke** paradigm. The separation of concerns between `sovereign-daemon` (Go control plane), `sovereign-web` (Next.js UI), and upstream containerized applications is sound and directly supports the appliance design goals. The network isolation into three bridge networks (`sovereign-ingress`, `sovereign-control`, `sovereign-ai`), SQLite WAL durability, and atomic dropzone ingestion are strong, high-leverage architectural patterns.

However, a rigorous reconciliation against the governing PRD, UX Experience Spine, and ADRs reveals several critical discrepancies, dropped load-bearing requirements, and under-specified operational constraints:
1. **Direct Contradiction in Rollback Window**: The spine specifies a 60-second healthcheck rollback window, contradicting the PRD's 180-second window and omitting the PRD/UX watchdog debouncing requirement (3 consecutive failures over 60 seconds). This will trigger false-positive rollbacks during legitimate container database migrations.
2. **Missing Core Media Stack Components**: Navidrome (music streaming / party playlists / Subsonic API) and Audiobookshelf (audiobooks / podcast syncing) from PRD FR-10 are completely missing from the Spine's Stack table, structural seed, and Caddy ingress routing.
3. **Local AI & RAG Subsystem Omissions**: Load-bearing operational constraints—including the Two-Lane Priority Request Governor, the Pre-flight Storage Safety Guard (`model_size + 5.0 GB`), the architectural wiring for FTS5 context extraction/truncation, and the Safe Stub Mode (HTTP 503)—are absent from the spine's architectural decisions and namespace mappings.
4. **Under-specified Hardware & Background Workflows**: Optical disc ripping (ARM device passthrough, temporary staging, key management) and the scheduled off-peak transcoding worker (01:00–07:00 batch window) have no defined execution home in the supervisor or storage modules.
5. **Storage Resilience Gaps**: External NAS mount management (handling unreachable NAS shares with Amber warning rather than container crashes) and the 10% disk headroom threshold (pausing background imports to prevent database locks) are omitted.
6. **Notification & Event Dispatch Gaps**: Home Assistant webhook/MQTT event dispatch for detected document deadlines (FR-13) and outbound SMTP notification dispatch for degraded states/failures are missing from `/internal`.

---

## 2. Traceability Matrix

| Requirement / Constraint | Source Document | Present in Spine? | Status / Gap Analysis |
| :--- | :--- | :--- | :--- |
| **First-Run Wizard & mDNS (`sovereign.local`)** | PRD FR-1, UX § IA | Partial | Daemon entrypoint mentioned, but mDNS responder and `/setup` first-run routing gate are omitted. |
| **Container Lifecycle & Watchdog Dampening** | PRD FR-2, UX §1 | Partial | Supervisor module exists, but watchdog debouncing (3 failures / 60s) is omitted. |
| **Atomic Rollback Engine (180s window)** | PRD FR-3, SM-6, UX UJ-5 | **Contradicted** | Spine AD-2 specifies a **60s** window instead of **180s**, creating high risk of false-positive rollbacks on heavy migrations. |
| **Power-Loss Resilience & Diagnostics** | PRD FR-4, UX § IA | Yes | Captured in AD-5 and AD-6. |
| **NAS Storage Mount & Failure Handling** | PRD FR-5, UX § Microcopy | No | AD-5 assumes single host POSIX `/data`; NAS mount failure isolation is dropped. |
| **Media Dropzone Ingestion** | PRD FR-6, UX § IA | Yes | Captured in AD-7 (`/data/drop` -> `/data/media`). |
| **Optical Disc Automation (ARM)** | PRD FR-7, UX § Components | Partial | ARM listed in Stack table, but device passthrough, Blu-ray keys, and staging storage are omitted. |
| **Scheduled Transcoding Queue (01:00-07:00)** | PRD FR-8, FR-18 | No | No worker, queue manager, or scheduling engine specified in `/internal`. |
| **Video Streaming (Jellyfin)** | PRD FR-9 | Yes | Captured in AD-3, AD-7, stack table. |
| **Audio Streaming (Navidrome & Audiobookshelf)** | PRD FR-10, UX § IA | **Missing** | Omitted from Stack table, Caddy routes, and architectural mapping. |
| **Document Archiving (Paperless-ngx)** | PRD FR-11 | Yes | Captured in stack and network segregation. |
| **Multi-Channel Ingestion (IMAP / Mobile)** | PRD FR-12 | Partial | Paperless included, but IMAP outbound internet egress from segregated network is unspecified. |
| **Smart Home Event Bridge (HA / MQTT)** | PRD FR-13 | **Missing** | No webhook or MQTT event bridge specified in `/internal`. |
| **Email Alert Dispatch** | PRD FR-1, FR-3, FR-7 | **Missing** | No notification/mailer subsystem in `/internal` for system alerts. |
| **Local AI Chat & Turnkey llama-server** | PRD FR-14, FR-15, ADR-001 | Yes | Captured in AD-4, stack, and inherited invariants. |
| **Safe Stub Mode (HTTP 503 on boot without model)** | PRD FR-15, ADR-004, UX §2 | Partial | Mentioned in ADR table, but no structural implementation (handler/container) defined. |
| **Pre-flight Storage Safety Guard (`+ 5GB`)** | PRD FR-15, Addendum §8, UX | **Missing** | Model download staging and 5GB headroom guard omitted from `/internal/storage`. |
| **Two-Lane Priority Request Governor** | PRD FR-15, ADR-003, Addendum §8 | **Missing** | No request priority queue/proxy between interactive chat and background extraction. |
| **CPU-Only Host Protection (`cores - 2`)** | PRD FR-15, ADR-001, UX | Partial | Inherited ADR noted, but daemon thread pinning logic omitted from `/internal/hardware`. |
| **Single-Source RAG with FTS5 Context Injection** | PRD FR-16, ADR-002 | Partial | Diagram shows GoDaemon querying Paperless DB, but execution flow to Open WebUI is unspecified. |
| **Context Window Headroom Truncator** | PRD FR-16, ADR-004 | **Missing** | 512-token buffer reservation and context truncation logic omitted. |
| **Dynamic Idle Surrender & Daytime Reservation** | PRD FR-18, ADR-003, UX | Partial | Ticker in `/internal/hardware` noted; reverse proxy `status: warming_up` guard omitted. |
| **10% Low Disk Headroom Import Guard** | UX §2 (State 4), Addendum §8 | **Missing** | Threshold logic pausing background imports is omitted. |

---

## 3. Detailed Findings & Architectural Impact

### Finding 1: Rollback Window Contradiction and Missing Watchdog Dampening
- **Location**: `ARCHITECTURE-SPINE.md` § Invariants & Rules, `AD-2` (lines 82–87).
- **Discrepancy**: AD-2 defines: *"Container state transitions must pass container healthchecks before completing; any healthcheck failure within a 60-second verification window must automatically trigger an atomic rollback to `rev-{N-1}`."*
- **Source Expectation**: PRD §4.1 FR-2 and FR-3 explicitly mandate:
  1. A **180-second** verification window before declaring an update failed (to accommodate database migrations and cold cache initialization in services like Paperless and Jellyfin).
  2. **Watchdog dampening**: Probes require **3 consecutive failures over 60 seconds with exponential backoff** before marking a service degraded or initiating automated recovery, preventing transient I/O or swap spikes from triggering cascade restart loops.
- **Impact**: A rigid 60-second cutoff will cause false-positive update rollbacks on larger media or document libraries where SQLite/Postgres migrations legitimately take 75–120 seconds on first boot after an image update.
- **Action Required**: Amend AD-2 to adopt the 180-second verification window and formally codify the 3-failure / 60-second watchdog debouncing rule.

---

### Finding 2: Missing Core Media Stack Components (Navidrome & Audiobookshelf)
- **Location**: `ARCHITECTURE-SPINE.md` § Stack (lines 142–160), § AD-3 (lines 88–93), § Structural Seed (lines 164–180).
- **Discrepancy**: PRD §4.3 FR-10 and EXPERIENCE.md specify a dual-container audio streaming architecture: **Navidrome** for music/playlists (Subsonic API on port `:4533`) and **Audiobookshelf** for audiobooks/podcasts (port `:13378`). 
- **Source Expectation**: Both services are in-scope for v1 MVP. In `ARCHITECTURE-SPINE.md`, neither service appears in the Stack table, and neither is assigned an ingress route, port mapping, or compose template.
- **Impact**: Without explicit architectural definitions in the spine, downstream implementation will omit audio streaming or leave its networking, storage mounts (`/data/media/music`, `/data/media/audiobooks`), and ingress unprotected.
- **Action Required**:
  1. Add Navidrome (v0.54+) and Audiobookshelf (v2.19+) to the Stack table.
  2. Update AD-3 to specify ingress routing for music (`:4533` / `/music`) and audiobooks (`:13378` / `/audiobooks`).
  3. Ensure `/compose/templates` and `/data/media` include dedicated configurations for both.

---

### Finding 3: Local AI & RAG Subsystem Operational Gaps
- **Location**: `ARCHITECTURE-SPINE.md` § AD-4, § ADR bindings (lines 50–56), § Invariants (lines 94–99).
- **Discrepancy**: Several load-bearing operational constraints and safety guards from PRD FR-15, FR-16, ADR-001–004, and Addendum §8 are missing:
  1. **Two-Lane Priority Request Governor**: The PRD and ADR-003 require an API proxy mechanism prioritizing interactive user queries (Open WebUI) over automated background tasks (OCR metadata extraction, nightly scans). In the spine, the diagram shows Open WebUI directly connecting to `llama-server:8080`, bypassing any governor.
  2. **Pre-flight Storage Safety Guard for Model Downloads**: PRD FR-15 and Addendum §8 enforce a `model_size + 5.0 GB safety reserve` check on `/data/models` before initiating GGUF downloads. Downloads exceeding this floor must be rejected to prevent database lockups. This guard is missing from `/internal/storage`.
  3. **RAG Context Window Headroom Truncator & Data Flow**: While SQLite FTS5 injection is inherited from ADR-002, the spine diagram depicts `GoDaemon -->|FTS5 Context Query| PaperlessDB`, but does not specify how Open WebUI receives the retrieved context. Does `sovereign-daemon` expose a RAG pipeline endpoint or proxy `/v1/chat/completions`? The 512-token response buffer reservation and truncation notification logic are unspecified.
  4. **Safe Stub Mode (HTTP 503)**: ADR-004 defines a safe HTTP 503 stub when `/data/models` has no valid models. The spine does not specify whether `sovereign-daemon` proxies and stubs this endpoint or if a dedicated lightweight stub container is deployed.
  5. **Reverse Proxy Warm-up Guard**: Waking `llama-server` from idle surrender requires Caddy/daemon proxy guards (>= 60s timeout) and returning `status: "warming_up"` to prevent 504 Gateway Timeouts.
- **Impact**: Under concurrent load, background batch tasks will cause interactive chat to stutter; disk exhaustion may corrupt SQLite databases; and users waking models from idle will encounter HTTP 504 timeouts.
- **Action Required**: Add an explicit architectural decision (e.g. `AD-9 — AI Subsystem Gateway, Priority Governor & Storage Guard`) or expand AD-4 to define the request arbitration proxy, download staging rules, and RAG execution flow.

---

### Finding 4: Optical Disc Automation & Scheduled Transcoding Under-specified
- **Location**: `ARCHITECTURE-SPINE.md` § Structural Seed (lines 164–180), § Capability Map (line 192).
- **Discrepancy**:
  1. **Optical Ingestion**: Automatic Ripping Machine (ARM) is listed in the stack, but the spine does not specify host hardware device passthrough (`/dev/sr0`, `/dev/sg*`), where Blu-ray decryption keys (`KEYDB.cfg`) reside, or where raw rips are staged before atomic movement to `/data/media`.
  2. **Scheduled Transcoding Queue**: PRD §4.2 FR-8 and FR-18 require a scheduled transcoding worker running during off-peak hours (01:00 to 07:00) to produce standardized Direct Play files and iPad mobile profiles. The spine does not define what worker executes this queue (embedded ffmpeg runner in daemon, HandBrake container, or Jellyfin scheduled task), or how the schedule is persisted and governed.
- **Impact**: Disc ripping and mobile profile generation will lack clear ownership in the codebase, leading to ad-hoc scripts or unmanaged container privileges.
- **Action Required**: Specify ARM container volume/device bindings in `/compose/templates` and define the scheduled transcode runner within `/internal/supervisor` or `/internal/storage`.

---

### Finding 5: Storage Architecture Gaps (NAS Mounts & Low-Disk Headroom)
- **Location**: `ARCHITECTURE-SPINE.md` § AD-5 (lines 100–105).
- **Discrepancy**:
  1. **External NAS Mount Management**: PRD §4.2 FR-5 and EXPERIENCE.md require supporting external NFS/SMB storage pools alongside local drives, with fault isolation: if a NAS disconnects, affected feature cards transition to Amber (*"Home NAS storage is unreachable"*) without crashing the daemon or containers. AD-5 assumes a strictly local single POSIX `/data` mount.
  2. **Storage Headroom Warning State (10% Threshold)**: EXPERIENCE.md § State Patterns (State 4) and Addendum §8 require pausing background media imports and downloads when free disk space drops below 10%, safeguarding SQLite WAL flushes. This is missing from AD-5 and `/internal/storage`.
- **Impact**: Unreachable NAS shares could hang daemon I/O calls indefinitely (e.g. NFS unmount hangs), and running out of disk space will corrupt databases.
- **Action Required**: Update AD-5 to include NAS mount monitoring/timeout guards and define the 10% disk capacity threshold in `/internal/storage`.

---

### Finding 6: Omitted Notification & Smart Home Integration Pipelines
- **Location**: `ARCHITECTURE-SPINE.md` § Namespace & Layer Mapping (lines 36–45), § Structural Seed (lines 164–180).
- **Discrepancy**:
  1. **Smart Home Event Bridge**: PRD §4.4 FR-13 requires dispatching urgent alerts (contract renewals, invoice due dates) to Home Assistant via webhooks and an MQTT event publisher.
  2. **Email Alert Dispatch**: PRD FR-1, FR-3, FR-7 mandate sending maintenance email alerts (e.g. failed update rollbacks, disc read failures, unrecoverable drive errors) to the household admin email configured during setup.
  3. Neither pipeline has an architectural home in `/internal` (e.g., no `/internal/notify` or `/internal/events`).
- **Impact**: Crucial proactive consumer features (e.g., notification on kitchen wall tablets, failure alert emails) will be dropped during implementation.
- **Action Required**: Add `/internal/notify` (or `/internal/bridge`) to the namespace mapping and structural seed to manage SMTP dispatch and Home Assistant webhooks/MQTT.

---

### Finding 7: User Journey Mapping Mismatches & Setup Routing
- **Location**: `ARCHITECTURE-SPINE.md` § Capability -> Architecture Map (lines 184–194).
- **Discrepancy**:
  1. **Mismatched UJ Identifiers**: The table in lines 186–193 lists:
     - `UJ-2 Private document query & RAG` (In PRD, UJ-2 is Sarah digitizing household paperwork; RAG is UJ-4).
     - `UJ-3 Family media dropzone` (In PRD, UJ-3 is Disc Digitization & Video Streaming; dropzone is FR-6).
     - `UJ-4 Sudden power-loss resilience` (In PRD, this is UJ-5).
     - `UJ-5 Automated physical disc digitization` (In PRD, this is UJ-3).
     - PRD `UJ-2` (Sarah's document intake, OCR, deadline extraction) is entirely missing from the table.
  2. **First-Run Setup Gate**: EXPERIENCE.md requires that on unprovisioned first boot, `/` automatically redirects to `/setup`. AD-6 covers `/boot` during diagnostics, but the first-run unconfigured gate is omitted from AD-1/AD-6.
- **Impact**: Causes confusion across planning artifacts and downstream test plans.
- **Action Required**: Re-align the Capability Map to accurately reflect PRD UJ-1 through UJ-5 and document the `/setup` routing gate in AD-1.

---

## 4. Reconciliation Matrix & Recommended Amendments

| Finding ID | Area | Severity | Recommended Fix for ARCHITECTURE-SPINE.md |
| :--- | :--- | :--- | :--- |
| **F-01** | Rollback Window | High | Amend **AD-2**: Replace 60s verification window with **180s**, and add watchdog dampening (3 consecutive failures over 60s with exponential backoff). |
| **F-02** | Media Stack | High | Add **Navidrome** and **Audiobookshelf** to Stack table, define Caddy routes in **AD-3**, and add storage subpaths in **AD-5**. |
| **F-03** | AI & RAG Subsystem | High | Add **AD-9** or expand **AD-4**: Define Two-Lane Priority Request Governor, Pre-flight Storage Safety Guard (`+ 5GB`), Safe Stub 503 mode, and reverse proxy warm-up timeout guard (>= 60s). |
| **F-04** | Disc & Transcode | Medium | Add hardware passthrough (`/dev/sr0`) and `KEYDB.cfg` volume mount to ARM specs; define scheduled transcoding queue runner in `/internal/supervisor`. |
| **F-05** | NAS & Disk Safety | Medium | Expand **AD-5**: Specify NAS mount health monitoring with non-blocking timeouts; add 10% disk capacity import pausing guard to `/internal/storage`. |
| **F-06** | Notifications & HA | Medium | Add `/internal/notify` module to namespace mapping and structural seed for SMTP alerts and Home Assistant webhook/MQTT dispatch. |
| **F-07** | Traceability & Setup | Low | Fix UJ numbering in Capability Map (UJ-1 to UJ-5); specify `/setup` first-run routing gate in **AD-1**. |

---

## 5. Conclusion

`ARCHITECTURE-SPINE.md` establishes a strong architectural paradigm. However, the identified contradictions (notably the 60s vs. 180s rollback window) and omitted operational guards (missing audio services, priority governor, 5GB storage safety floor, NAS error handling, and notification bridges) represent significant reconciliation gaps.

Addressing the 7 recommendations above will bring `ARCHITECTURE-SPINE.md` into complete alignment with the PRD, UX specs, and ADRs.
