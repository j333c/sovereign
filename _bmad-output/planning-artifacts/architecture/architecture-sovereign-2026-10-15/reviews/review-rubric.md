# Architecture Spine Review — Sovereign Home Server

> **Review Target:** `_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md`  
> **Review Date:** 2026-10-15  
> **Reviewer:** Rubric Walker (BMad Architecture Gate)  
> **Altitude:** Feature / Platform Substrate  

---

## Overall Verdict: CONCERNS

The Sovereign architecture spine provides a well-structured, concise, and conceptually sound build substrate. It establishes the **Supervised Hub-and-Spoke** paradigm, enforces strict boundary isolation between the Go supervisor and the Next.js presentation frontend, introduces declarative Compose v2 rollbacks, partitions persistent storage cleanly under `/data`, and institutes dirty-boot diagnostic gates.

However, the spine cannot receive an unconditional **PASS** in its current draft due to one critical networking bug in AD-3, missing invariants around authentication and AI request mediation topology, a capability map omission/misnumbering against the PRD, and discrepancies in rollback timeout parameters.

---

## Rubric Scorecard

| # | Checklist Criterion | Status | Summary Finding |
|---|---|---|---|
| 1 | **Fixes real divergence points for level below and misses none** | **CONCERNS** | Omits authentication/authorization model; AI RAG context retrieval and Two-Lane Governor routing are topologically ambiguous. |
| 2 | **Every AD's Rule is enforceable and actually prevents stated divergence** | **CONCERNS** | AD-3 specifies `localhost:2019` for Caddy Admin API, which fails across Docker bridge networks (`sovereign-control`). AD-2 verification window (60s) risks false-positive rollbacks compared to PRD (180s). |
| 3 | **Nothing under Deferred could let two units diverge** | **PASS** | Clear, disciplined deferrals (multi-node clustering, Ceph/MinIO, WAN VPN, ZFS DKMS) prevent distributed systems over-engineering. |
| 4 | **Covers spec/PRD capabilities** | **CONCERNS** | Capability Map omits PRD UJ-2 (Paperless document intake & Home Assistant alerts) and scrambles journey numbering; Stack table omits Navidrome & Audiobookshelf. |
| 5 | **Preserves and respects inherited invariants** | **PASS** | Binds and honors ADR-001 through ADR-004 without weakening or contradiction. |
| 6 | **Operational / environmental envelope completeness** | **CONCERNS** | Leaves host daemon bootstrap (systemd vs container) and container hardware device passthrough (`/dev/dri`, `/dev/sr0`) unspecified. |

---

## Detailed Evaluation by Checklist Item

### 1. Fixes real divergence points for the level below and misses none

- **Strengths:**
  - **AD-1** cleanly severs the presentation layer (`sovereign-web`) from the container engine socket, preventing web crashes from bringing down the appliance supervisor.
  - **AD-4** strictly isolates containers into three purpose-built bridge networks (`sovereign-ingress`, `sovereign-control`, `sovereign-ai`), ensuring raw LLM ports and database sockets are not reachable on the LAN.
  - **AD-5** and **AD-7** eliminate file-copy latency and corruption by co-locating dropzone and media libraries under a unified `/data` mount, enabling atomic `os.Rename` operations.
  - Consistency conventions (UUIDv7, PascalCase/kebab-case, RFC 7807 problem details, atomic temp-file config sync) eliminate trivial code drift.

- **Findings:**
  - **high (Divergence Risk)** — **Undefined Authentication & Authorization Boundary**: The spine is completely silent on whether or how the admin dashboard and API endpoints (`/api/v1/*`) are authenticated. In `EXPERIENCE.md`, `/drop` is explicitly designed as an unprivileged household landing zone, while `/settings` and service management (`/api/v1/services`) allow destructive actions (restarts, rollbacks, model swaps). Without a rule or convention (e.g. LAN single-admin session cookie, HTTP Basic Auth via Caddy, or bearer API tokens), frontend and backend builders will implement incompatible security boundaries.
  - **high (Divergence Risk)** — **Topological Disconnect in AI Request Mediation & RAG**: Inherited invariant ADR-002 mandates direct SQLite FTS5 RAG context injection, and ADR-003 / PRD FR-15 mandate a Two-Lane Priority Request Governor prioritizing interactive human chat over background tasks. However, the Mermaid diagram (lines 71–72) routes Open WebUI directly to `llama-server:8080`, bypassing `sovereign-daemon`. If Open WebUI connects directly to llama-server, the daemon cannot arbitrate priority, and it remains undefined how FTS5 search results are injected into the prompt context.
  - **medium (Divergence Risk)** — **Hardware Device Passthrough Convention**: The stack requires Intel QuickSync/Nvidia GPU transcoding (Jellyfin) and optical disc drive access (Automatic Ripping Machine). How Compose templates parameterize host device mounts (`/dev/dri`, `/dev/sr0`, `/dev/sg*`, or NVIDIA Container Toolkit) is not defined, inviting ad-hoc compose template authoring.

---

### 2. Every AD's Rule is enforceable and actually prevents its stated divergence

- **Strengths:**
  - **AD-1, AD-5, AD-6, AD-7, AD-8** have crisp, unambiguous rules that can be validated via static linter or CI integration tests.
  - **AD-4** provides a testable network boundary by forbidding published host ports (`-p 8080:8080`) on `llama-server`.

- **Findings:**
  - **critical (Enforceability Bug)** — **Invalid Caddy Admin API Address in AD-3**: AD-3 states: *"SSL certificates... must be managed dynamically through the Go daemon via Caddy's admin API (`localhost:2019`)."* Under AD-4, Caddy and Go daemon communicate across the Docker bridge network `sovereign-control`. In Docker bridge networking, `localhost:2019` inside the daemon container (or host) will fail with `connection refused` because Caddy listens inside its own container network namespace.  
    *Fix:* Change rule to specify `http://caddy:2019` on `sovereign-control` or a shared UNIX domain socket `/var/run/caddy/admin.sock`.
  - **medium (Verification Threshold Tension)** — **Rollback Window Discrepancy in AD-2**: AD-2 specifies: *"any healthcheck failure within a 60-second verification window must automatically trigger an atomic rollback to `rev-{N-1}`."* However, PRD FR-3 specifies a 180-second window, and PRD FR-2 mandates watchdog dampening (3 consecutive failures over 60 seconds with exponential backoff). Heavy services like Paperless-ngx routinely take 45–90 seconds on first boot to run database migrations. A strict 60-second cutoff will trigger false-positive rollbacks during normal startup.  
    *Fix:* Expand the verification window in AD-2 to 180 seconds and mandate healthcheck watchdog dampening.

---

### 3. Nothing under Deferred could let two units diverge

- **Strengths:**
  - The Deferred table explicitly excludes multi-node clustering/HA, distributed object storage (Ceph/MinIO), remote WAN tunnels, and ZFS DKMS compilation.
  - These deferrals cleanly protect the single-node physical/virtual appliance model and avoid distributed consensus complexity.
  - No deferred item leaves a gaping hole that would allow two internal modules to implement competing solutions.

- **Findings:**
  - **low** — **Explicit Scope Boundary Deferrals**: While clustering and storage are deferred, enterprise SSO/LDAP and automated cloud offsite backup should be explicitly recorded in the Deferred table to prevent scope creep during epic implementation.

---

### 4. Covers spec/PRD capabilities

- **Strengths:**
  - First-run wizard, video streaming, standalone media dropzone, power-loss resilience, and optical disc digitization are represented in the architecture.

- **Findings:**
  - **high (Capability Omission & Misnumbering)** — **Omission of PRD UJ-2 & Scrambled Journey IDs**:
    In PRD Section 2.3, the canonical journeys are:
    - *UJ-1*: Setup & Video Streaming
    - *UJ-2*: Sarah digitizes household paperwork (Mobile scan, IMAP email polling, OCR, Home Assistant alert bridge)
    - *UJ-3*: Optical disc ripping & mobile transcoding sync
    - *UJ-4*: Private document query & RAG
    - *UJ-5*: Power outage recovery & automated updates
    
    In `ARCHITECTURE-SPINE.md` lines 188–193:
    - UJ-2 is re-labeled "Private document query & RAG" (PRD UJ-4).
    - UJ-3 is re-labeled "Family media dropzone" (PRD FR-6).
    - UJ-4 is re-labeled "Sudden power-loss resilience" (PRD UJ-5).
    - UJ-5 is re-labeled "Automated physical disc digitization" (PRD UJ-3).
    
    *Result:* The actual PRD UJ-2 (Paperless-ngx document intake, email polling, and Home Assistant notification event dispatch) is completely absent from the Capability Map, despite being one of the four cornerstone pillars of the PRD!
  - **medium (Incomplete Stack Seed)** — **Missing Audio Streaming Containers**: PRD FR-10 and AD-7 explicitly govern audio streaming and webhooks for `Audiobookshelf` and `Navidrome`. However, neither is listed in the `Stack` table (§Stack, lines 144–159).

---

### 5. Preserves and respects inherited invariants

- **Strengths:**
  - All four binding ADRs from `architecture-decisions.md` are documented in the Inherited Invariants table:
    - ADR-001 (managed `llama.cpp` / GGUF in `/data/models` / OpenAI `/v1`)
    - ADR-002 (Paperless SQLite FTS5 context injection without vector DB)
    - ADR-003 (ingestion direct-play pre-encoding, daytime AI priority, 5-min idle surrender)
    - ADR-004 (safe stub mode, GGUF magic check, `-np 2`, air-gapped inference)
  - None of the local decisions (AD-1 through AD-8) weakens or contradicts any inherited invariant.

- **Findings:**
  - *No blocking findings.*

---

### 6. Operational and environmental envelope completeness

- **Strengths:**
  - SQLite WAL mode and synchronous=NORMAL are mandated across all databases.
  - Unclean shutdown detection via `/data/state/boot_state.json` triggering `PRAGMA integrity_check` before starting services provides strong power-loss resilience.

- **Findings:**
  - **medium (Envelope Gap)** — **Host Bootstrap & Daemon Execution Model**: The spine leaves open how `sovereign-daemon` itself is installed and started on the host OS. Does it run as a native `systemd` service, or does it run as a privileged container mounting `/var/run/docker.sock`? If it runs as a container, what starts the container on host boot, and how does it trap host shutdown signals to write `clean_shutdown = true` to `boot_state.json`?
  - **low (Envelope Gap)** — **Smart Home Event Bridge (FR-13)**: PRD FR-13 requires Home Assistant webhook/MQTT event publication for document deadline alerts. The spine should identify which layer handles event dispatch (the Go daemon or a Paperless post-consumption script).

---

## Actionable Fixes to Achieve PASS

1. **Fix AD-3 Caddy Endpoint**: Update `localhost:2019` to `http://caddy:2019` on `sovereign-control` (or specify a shared UNIX socket).
2. **Add AD-9 or Convention for Authentication**: Establish the authentication invariant for `sovereign-daemon` API and `sovereign-web` (e.g. single-admin password initialized in `/setup`, session cookie for admin routes, unauthenticated access for `/drop`).
3. **Clarify AI / RAG Proxy Routing in Mermaid & AD-4**: Update the architecture diagram and AD-4 to explicitly show whether Open WebUI connects to `sovereign-daemon` as an OpenAI proxy (enabling the Two-Lane Governor and FTS5 context injection) or how FTS5 context reaches Open WebUI.
4. **Reconcile Capability Map with PRD**: Restore PRD UJ-2 (Household document digitization & alerts) into the Capability Map, align UJ-1 through UJ-5 numbering with PRD Section 2.3, and add `Navidrome` and `Audiobookshelf` to the Stack table.
5. **Align Rollback Verification Window**: Adjust AD-2 verification window from 60s to 180s to accommodate database migration startup times.
6. **Decide Host Execution Model**: Clarify whether `sovereign-daemon` runs as a host `systemd` service or a bootstrap container, and note container GPU/optical device passthrough conventions.
