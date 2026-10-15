# Adversarial Architectural Review: Unit Clash & Incompatibility Analysis

**Target Document:** `ARCHITECTURE-SPINE.md`  
**Review Date:** 2026-10-15  
**Reviewer:** Adversarial System Architect (BMad Architecture Review Lens)  
**Status / Verdict:** **FAIL** (Architectural Clashes Detected — Blocking Implementation)

---

## 1. Executive Summary

An adversarial review of `ARCHITECTURE-SPINE.md` was conducted by constructing independent sub-units one level down from the architectural decisions (ADs). By strictly adhering to every rule in AD-1 through AD-8 and ADR-001 through ADR-004, multiple irreconcilable runtime collisions, lock contentions, state mutation conflicts, and unhandled deadlocks were identified.

While the high-level **Supervised Hub-and-Spoke** concept is sound, the current architecture decisions leave critical boundaries under-specified or contradictory. If two engineering units (e.g., core daemon vs. web frontend, ingress gateway vs. upstream containers, storage manager vs. media applications) were implemented to the letter of the current spine, the resulting system would fail to boot, deadlock on GPU arbitration, corrupt media scans during ingestion, and suffer from unresolvable reverse-proxy URL rewriting issues.

---

## 2. Adversarial Clash Scenarios

### Clash 1: Dual-Path Ingress Contradiction vs. Upstream Base Path Handling
- **Units in Conflict:** `/internal/ingress` (Caddy Gateway) vs. `/internal/supervisor` (Docker Compose Upstream Application Templates)
- **Rules Obeyed:**
  - Ingress unit obeys **AD-3**: "Upstream services must be reachable via both dedicated ports (e.g. `:8096`, `:8000`, `:3001`) and standard subpaths (e.g. `/media`, `/docs`, `/chat`)."
  - Supervisor unit obeys **AD-2**: "All managed services must be declared in versioned compose specifications (`/data/state/compose/rev-{N}.yaml`)."
- **The Incompatibility:**
  - Modern web applications (Open WebUI/SvelteKit, Jellyfin Web, Paperless-ngx) generate absolute asset URLs (`<script src="/_app/...">` or `<link href="/assets/...">`) and rely on either a root base path (`/`) or a preconfigured subpath prefix (e.g., `WEBROOT=/chat` or `URL_BASE_PATH=/chat`).
  - **Case A:** If the Supervisor configures upstream containers with root `/` so that direct port access (`http://host:3001` or `http://host:8096`) works:
    When a user accesses the service via Caddy subpath (`http://host/chat`), Caddy strips `/chat` and proxies to the upstream. The upstream returns HTML referencing `/_app/immutable/...`. The browser requests `http://host/_app/immutable/...`, which hits Caddy's root proxy (`sovereign-web`), resulting in immediate HTTP 404s and a broken UI.
  - **Case B:** If the Supervisor configures upstream containers with base path `/chat`:
    Accessing the service via direct port `http://host:3001/` immediately returns HTTP 404 or fails redirects because the upstream does not serve assets at root `/`.
  - Neither Caddy nor Compose templates can solve this without active HTML response rewriting (which breaks sub-resource integrity, HTTP/2 streaming, and WebSocket handshakes) or dual-listener deployments.
- **Hole in Spine:** AD-3 mandates a dual-path ingress model that is technically incompatible with upstream Single Page Applications (SPAs) without specifying how base URL resolution and asset routing are negotiated.

---

### Clash 2: AI Inference Data-Path Bypass vs. Dynamic GPU Idle Surrender
- **Units in Conflict:** `/internal/hardware` (GPU Idle Surrender Ticker) vs. Upstream `Open WebUI` & `llama-server` (`sovereign-ai` Network)
- **Rules Obeyed:**
  - Hardware supervisor obeys **ADR-003**: "5-minute dynamic idle VRAM surrender" and "Two-lane API priority governor ensures interactive human chat takes precedence over background document extraction."
  - Ingress & Security obeys **AD-4**: "`sovereign-ai` (Daemon, Open WebUI, llama-server, FTS5 retrieval service). `llama-server` must never publish host ports... inference access must be mediated through Open WebUI or the daemon."
  - Diagram states: `OpenWebUI -->|Internal /v1 (sovereign-ai)| LlamaServer`.
- **The Incompatibility:**
  - Open WebUI communicates directly with `llama-server:8080` over the internal Docker network, completely bypassing `sovereign-daemon`.
  - Because `sovereign-daemon` is out-of-band for chat requests:
    1. The "two-lane API priority governor" in `sovereign-daemon` cannot intercept, throttle, or prioritize Open WebUI chat queries against background Paperless document RAG queries.
    2. The GPU idle surrender ticker in `sovereign-daemon` cannot know when an active chat session is ongoing except by polling raw NVML GPU utilization. Between prompt tokens or user thinking time, NVML utilization drops to 0%.
    3. If the idle surrender ticker triggers after 5 minutes of inactivity and unloads `llama-server` (or stops its container), Open WebUI's next user prompt sent to `http://llama-server:8080/v1` immediately fails with `ECONNREFUSED` or timeout. Open WebUI has no capability to invoke `sovereign-daemon` to re-wake the model.
- **Hole in Spine:** ADR-003 assumes daemon-mediated arbitration, but the network topology in AD-4 and the architecture diagram route Open WebUI directly to `llama-server`, rendering GPU wake-on-request and priority governance impossible.

---

### Clash 3: Multi-Process SQLite Access, POSIX Permission Deadlocks & Schema Drift
- **Units in Conflict:** `/internal/store` & `/internal/supervisor` (Daemon Context RAG) vs. `Paperless-ngx` Container (Django/Celery)
- **Rules Obeyed:**
  - Store unit obeys **ADR-002**: "Document RAG using Paperless-ngx SQLite FTS5 direct context injection; no secondary vector DB or embedding daemon... Queries execute against Paperless-ngx's internal SQLite FTS5 full-text search index."
  - System obeys **AD-5**: "All SQLite databases (including `sovereign.db` and Paperless) must enforce `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`."
  - Boot supervisor obeys **AD-6**: "If missing on boot, `sovereign-daemon` must enter `DiagnosticsMode`, running `PRAGMA integrity_check` on all SQLite databases."
- **The Incompatibility:**
  - `Paperless-ngx` runs as an unprivileged container user (e.g., `PUID=1000`), whereas `sovereign-daemon` runs with host supervisor privileges to control Docker.
  - When Paperless writes to its SQLite DB, it creates `-wal` and `-shm` shared memory files owned by `1000:1000`. If `sovereign-daemon` opens `/data/documents/paperless.sqlite` without `SQLITE_OPEN_READONLY` (`mode=ro`), SQLite attempts to open `-shm` in read-write mode, immediately throwing POSIX `EACCES (Permission Denied)`.
  - While Paperless is processing an OCR batch, Django/Celery holds exclusive SQLite write transactions. If `sovereign-daemon` issues an FTS5 search query during this window, it encounters `SQLITE_BUSY` errors unless non-blocking retry governors and explicit busy timeouts are mandated.
  - On unclean shutdown (AD-6), `sovereign-daemon` attempts to run `PRAGMA integrity_check` on Paperless's DB before launching containers. If the database has uncheckpointed WAL frames from a crashed container process, running integrity checks as host root will alter WAL file ownership or lock out Paperless when it boots.
  - ADR-002 binds `sovereign-daemon` directly to Paperless's internal Django database schema without an abstraction layer, causing daemon crashes whenever Paperless updates its internal migrations.
- **Hole in Spine:** Direct cross-process SQLite access across a container-host boundary violates isolation invariants and causes lock and permission contention.

---

### Clash 4: Race Conditions in Dropzone Ingestion vs. In-Flight Client Uploads
- **Units in Conflict:** `/internal/storage` (Dropzone Validation Engine) vs. Client File Streams (SMB / Web UI Chunked Upload)
- **Rules Obeyed:**
  - Storage unit obeys **AD-7**: "Uploads must stream to `/data/drop/incoming/:category`, undergo mime/magic-byte validation, and be atomically renamed (`os.Rename` on the same filesystem) into `/data/media/:category`. Upon move completion, `sovereign-daemon` must trigger immediate HTTP library refresh webhooks to Jellyfin or Audiobookshelf."
  - Storage unit obeys **AD-5**: Structured `/data` layout under a single POSIX mount.
- **The Incompatibility:**
  - A client starts uploading a 30 GB 4K video file (`avatar.mkv`) over SMB or HTTP into `/data/drop/incoming/movies/avatar.mkv`.
  - As soon as the first 16 bytes are written to disk, an inotify file watcher detects file creation.
  - The validation routine in AD-7 checks the initial bytes: the Matroska magic bytes (`0x1A45DFA3`) are present and valid.
  - Obeying AD-7, `sovereign-daemon` immediately executes `os.Rename("/data/drop/incoming/movies/avatar.mkv", "/data/media/movies/avatar.mkv")`.
  - On POSIX filesystems, `rename(2)` succeeds instantaneously even while the upload client still holds an open file handle writing to the source inode.
  - Obeying AD-7, `sovereign-daemon` immediately fires the library refresh webhook to Jellyfin.
  - Jellyfin immediately scans the partially written file (e.g., only 5 MB of 30 GB written), fails media probe/demuxing, marks the video as corrupted or 0 seconds long, and caches corrupt metadata.
  - When the upload finally completes 10 minutes later, no new event is fired, leaving the library in a corrupted state.
- **Hole in Spine:** AD-7 lacks an in-flight upload protocol (e.g., staging with `.uploading` extensions, `IN_CLOSE_WRITE` gating, or atomic commit tokens).

---

### Clash 5: Declarative Compose Rollback vs. Irreversible Database State Drift
- **Units in Conflict:** `/internal/supervisor` (Compose Rollback Engine) vs. Persistent Storage (`/data/state` & Upstream DBs)
- **Rules Obeyed:**
  - Supervisor obeys **AD-2**: "All managed services must be declared in versioned compose specifications (`/data/state/compose/rev-{N}.yaml`). Container state transitions must pass container healthchecks before completing; any healthcheck failure within a 60-second verification window must automatically trigger an atomic rollback to `rev-{N-1}`."
- **The Incompatibility:**
  - When updating from `rev-1` to `rev-2`, `sovereign-daemon` launches an updated container image (e.g., Paperless v3.4 or Jellyfin v12.3).
  - Upon startup, the new container immediately executes automated, irreversible database schema migrations against `/data/state/` or `/data/documents/`.
  - At $t = 45$ seconds, a co-dependent container fails its healthcheck.
  - Obeying AD-2, `sovereign-daemon` triggers an automatic rollback of the Compose specification to `rev-1`.
  - The older `rev-1` container image is started, but it mounts a database schema that has already been migrated forward to `rev-2`.
  - The `rev-1` application fails to parse the new schema, panics on unknown columns, or enters a crash-loop.
  - The rollback is **not atomic** because AD-2 only rolls back the container definition, not the persistent volume state.
- **Hole in Spine:** Container-level rollbacks without volume snapshotting or pre-migration backup invariants cause fatal state corruption during failed updates.

---

### Clash 6: Boot Diagnostics Catch-22 (Headless Ingress Black Hole)
- **Units in Conflict:** `/cmd/sovereign-daemon` (DiagnosticsMode) vs. `/web` (`sovereign-web` UI) & `/internal/ingress` (Caddy)
- **Rules Obeyed:**
  - Daemon obeys **AD-6**: "If missing on boot, `sovereign-daemon` must enter `DiagnosticsMode`, running `PRAGMA integrity_check` on all SQLite databases and volume journal checks **before launching services**. The Web UI must lock navigation to `/boot` until `/api/system/boot-status` returns healthy."
  - Frontend obeys **AD-1**: Web presentation container has zero direct host access and communicates strictly via HTTP REST to `sovereign-daemon`.
  - Ingress obeys **AD-3**: Ingress is handled exclusively by dedicated Caddy container.
- **The Incompatibility:**
  - If `sovereign-daemon` runs diagnostics **before launching services**, neither Caddy nor `sovereign-web` are running during `DiagnosticsMode`.
  - A user whose appliance lost power turns on the server and browses to `http://sovereign.local`.
  - The browser displays `ERR_CONNECTION_REFUSED` (no Caddy, no web container).
  - The user assumes the appliance is bricked, power-cycles the machine during the SQLite integrity check, and causes the exact filesystem corruption AD-6 was designed to prevent.
  - The requirement "The Web UI must lock navigation to `/boot`" is impossible to fulfill if the Web UI itself has not been launched.
- **Hole in Spine:** AD-6 fails to classify services into "Infrastructure Ingress Core" (Caddy, Web UI, Daemon) and "Managed Workload Services" (Jellyfin, Paperless, etc.), creating a headless dead-end on recovery.

---

### Clash 7: API Route Hierarchy & Contract Drift
- **Units in Conflict:** `/internal/api` (Go Daemon HTTP Router) vs. `/web/lib` (OpenAPI Generated TypeScript Client)
- **Rules Obeyed:**
  - Client obeys **AD-8**: "All daemon endpoints must be documented in a single OpenAPI 3.1 specification. The frontend TypeScript API client must be strictly generated from this spec."
  - Consistency Conventions dictate: "Naming (routes): `/api/v1/:resource` in lowercase kebab-case."
- **The Incompatibility:**
  - AD-6 explicitly specifies an unversioned endpoint: `/api/system/boot-status`.
  - AD-8 explicitly specifies an unversioned endpoint: `/api/events`.
  - Consistency Conventions mandate `/api/v1/:resource`.
  - If the OpenAPI specification strictly enforces `/api/v1/`, the endpoints defined in AD-6 and AD-8 break client routing or bypass versioning rules.
- **Hole in Spine:** Uncoordinated route naming across AD-6, AD-8, and the Consistency Conventions table.

---

## 3. Top Findings Summary

| ID | Severity | Conflicting Units | Core Failure Mechanism |
| --- | --- | --- | --- |
| **F-01** | **BLOCKER** | Caddy Ingress vs. Upstream Containers (AD-3 vs. AD-2) | Dual-path routing (ports + subpaths) breaks upstream SPA asset paths without URL rewriting. |
| **F-02** | **BLOCKER** | Hardware Surrender Ticker vs. Open WebUI (ADR-003 vs. AD-4) | Direct Open WebUI $\to$ llama-server path bypasses AI priority governor and causes `ECONNREFUSED` on idle surrender. |
| **F-03** | **CRITICAL** | Storage Dropzone vs. Upload Clients (AD-7 vs. AD-5) | Premature rename of in-flight file streams triggers corrupted media scans in Jellyfin. |
| **F-04** | **CRITICAL** | Boot Diagnostics vs. Web UI / Ingress (AD-6 vs. AD-1 / AD-3) | Running diagnostics "before launching services" blocks Caddy/Web UI, creating a connection-refused black hole for users. |
| **F-05** | **CRITICAL** | Compose Rollback vs. Database Migrations (AD-2 vs. AD-5) | Compose-only rollback leaves forward-migrated databases in `/data/state`, permanently crashing rolled-back containers. |
| **F-06** | **MAJOR** | Daemon FTS5 vs. Paperless SQLite (ADR-002 vs. AD-5) | Direct cross-process SQLite access causes POSIX permission denials and `SQLITE_BUSY` lock contention during OCR. |

---

## 4. Required Tightenings & New ADs

To close these holes, the following architectural decisions must be tightened or added to `ARCHITECTURE-SPINE.md`:

1. **Tighten AD-3 (Ingress Topology):**
   - Eliminate mandatory dual-path subpath proxies for third-party SPAs, OR designate dedicated subdomains/ports as the primary ingress with Caddy reverse-proxying dedicated subdomains (`chat.sovereign.local`, `media.sovereign.local`), OR specify port-based ingress only, reserving subpaths exclusively for the Sovereign Web UI.
2. **Tighten AD-4 & ADR-003 (AI Ingress Gateway):**
   - Mandate that **all** inference requests from Open WebUI must route through `sovereign-daemon`'s reverse proxy (`/api/v1/ai/v1`).
   - The daemon acts as the gateway: it handles wake-on-request (transparently waking `llama-server` within the 2-second `mmap` window before forwarding the prompt), manages the 5-minute surrender timer, and enforces the two-lane priority governor.
3. **Tighten AD-7 (Staging Ingestion Protocol):**
   - Mandate that all incoming uploads must write to a hidden staging directory (`/data/drop/.staging/` or file suffix `.uploading`).
   - The file mover must gate ingestion on `IN_CLOSE_WRITE` (inotify) or atomic client-side rename (`mv .uploading complete`), ensuring magic-byte validation and Jellyfin webhooks only execute on closed, fully written files.
4. **Tighten AD-6 (Tiered Boot Lifecycle):**
   - Partition services into **Tier-0 (Appliance Core)**: `sovereign-daemon`, `caddy`, and `sovereign-web`; and **Tier-1 (Workload Spokes)**: `jellyfin`, `paperless`, `llama-server`.
   - Tier-0 must boot immediately. If an unclean shutdown is detected, Tier-0 serves the `/boot` diagnostics page to the browser while Tier-1 workloads remain paused during SQLite and volume integrity checks.
5. **Tighten AD-2 (State-Aware Rollback):**
   - Require that `/data/state/` updates utilize storage snapshots (or SQLite backup dumps) prior to applying `rev-{N}` compose migrations, ensuring rolled-back compose specifications match database volume state.
6. **Tighten ADR-002 (Paperless Context Access):**
   - Specify that `/internal/storage` accesses Paperless SQLite strictly with URI parameters `file:/data/documents/paperless.sqlite?mode=ro&immutable=1` (or via Paperless REST API), preventing `-shm` write permission errors and avoiding write lock contention.
7. **Normalize Route Naming Conventions:**
   - Update AD-6 to `/api/v1/system/boot-status` and AD-8 to `/api/v1/events` to maintain strict compliance with the Consistency Conventions table.

---

## 5. Conclusion

The architecture spine cannot proceed to implementation in its current state. Implementing the specification as written will produce subtle, hard-to-debug integration failures across networking, storage ingestion, GPU lifecycle, and boot recovery. 

**Recommendation:** Address the 6 conflicting unit pairs by adopting the 7 tightened AD rules before starting story implementation.
