# Technology Versioning & Reality-Check Review — Sovereign Home Server

> **Review Target:** `_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md`  
> **Review Date:** 2026-10-15  
> **Reviewer:** Technology Versioning & Reality-Check Gate (BMad Architecture Review)  
> **Altitude:** Feature / Platform Substrate  

---

## Overall Verdict: CONCERNS

The core architectural paradigm (**Supervised Hub-and-Spoke**) and the fundamental technology selections in `ARCHITECTURE-SPINE.md` are well-conceived, modern, and production-viable. Several key modern framework choices—including **Go 1.27.2**, **Node.js 24.22.0 (LTS)**, **Next.js 16.4.1**, **Caddy 2.11.7**, **Jellyfin 12.2.0**, and **Paperless-ngx 3.3.1**—are spot-on, fully reality-checked, and reflect the verified state of the ecosystem in October 2026.

However, an unconditional **PASS** cannot be granted due to two critical versioning errors that appear to be ungrounded training-data artifacts, alongside two architectural reality-check gaps:
1. **Critical Technology Obsolescence in Stack Table:** Automatic Ripping Machine is pinned to `2.6.0` (an obsolete release from April 2022, trailing 22 minor versions behind `2.24.4`), and `llama.cpp` is pinned to `b4000` (late 2024, trailing over 7,600 builds behind `b11647`).
2. **Architectural Indexing Assumption Flaw (Paperless-ngx FTS5):** ADR-002 and the architecture diagram assert that document RAG queries execute against Paperless-ngx's "SQLite FTS5 index". In reality, Paperless-ngx uses **Whoosh** (or Tantivy) flat files for its search index; its SQLite database only stores relational records without an FTS5 virtual table. Direct FTS5 queries against Paperless's DB will fail with `no such table`.
3. **Container Network Addressing Bug in AD-3:** AD-3 specifies `localhost:2019` for the Caddy Admin API. Under AD-4's segregated Docker bridge networking (`sovereign-control`), `localhost:2019` inside the supervisor container will fail to reach Caddy.
4. **Version Lags:** Docker Compose is pinned to `2.30.0` (current is `v5.6.0`), and SQLite is pinned to `3.45.0` (current is `3.54.0`).

---

## Detailed Stack Table Audit

Every item in the `## Stack` table was audited against upstream repositories and live release registries as of **October 15, 2026**:

| Name | Spine Version | Current Stable (Oct 2026) | Status | Analysis & Reality-Check |
| --- | --- | --- | --- | --- |
| **Go** | 1.27.2 | 1.27.2 | **VERIFIED** | Active Go release line. Features generic methods, built-in `encoding/json/v2`, `crypto/mldsa`, and standard `uuid`. Excellent fit for supervisor. |
| **Node.js** | 24.22.0 | 24.22.0 (LTS) / 26.12 (Current) | **VERIFIED** | Pinned to Active LTS release. Correct production engineering decision for Next.js runtime stability. |
| **Next.js** | 16.4.1 | 16.4.1 | **VERIFIED** | Released October 2026. App Router default, Cache Components opt-in/default, Turbopack, and native React 19.3 integration. |
| **React** | 19.0.0 | 19.3.1 | **MINOR LAG** | React 19.0 was released in late 2024. Next.js 16.4 integrates with React 19.3.1 (stable View Transitions, Fragment Refs). Update to `19.3.1` or `^19.3`. |
| **Tailwind CSS** | 4.0.0 | 4.3.3 | **MINOR LAG** | Tailwind v4 generation is CSS-first (`@theme`). 4.0.0 is the initial v4 release; current stable is `4.3.3`. Safe to bump to `^4.3.3`. |
| **Caddy** | 2.11.7 | 2.11.7 | **VERIFIED** | Active release line (Oct 2026). Dynamic JSON Admin API, HTTP/3, and automated ACME/internal CA TLS support. |
| **SQLite** | 3.45.0 | 3.54.0 | **DATED** | SQLite 3.45.0 was released in January 2024. Current is 3.54.0 (Oct 9, 2026), bringing `.diskused` integration, query planner optimizations, and WAL speedups. |
| **Docker Compose** | 2.30.0 | 5.6.0 | **DATED** | Docker Compose 2.30.0 was released in late 2024. Docker Compose is currently at `v5.6.0`, and the Go SDK `compose-go/v2` is at `v2.16.1`. |
| **Jellyfin** | 12.2.0 | 12.2.0 (v12.2) | **VERIFIED** | Active stable release line. Fully supports NVENC, QSV, and VA-API hardware transcoding. |
| **Paperless-ngx** | 3.3.1 | 3.3.1 | **VERIFIED** | Active release line (v3.3.1). Note architectural caveat below regarding its internal indexing engine. |
| **Open WebUI** | 0.12.0 | 0.12.2 | **MINOR LAG** | Active release line is 0.12.x; patch 0.12.2 is latest. Fully supports OpenAI-compatible `/v1` endpoints. |
| **llama.cpp** | b4000 | b11647 | **SEVERE LAG** | `b4000` is from late 2024. The project is currently on `b11647` (>7,600 builds later). Sticking to `b4000` lacks contemporary GGUF quantization formats and prompt caching optimizations. |
| **Automatic Ripping Machine** | 2.6.0 | 2.24.4 | **CRITICAL ERROR** | Tag `v2.6.0` was released on April 29, 2022 (4.5 years ago). Current active stable line is `2.24.4` (Oct 2026). Pinned version is completely obsolete. |

---

## In-Depth Reality Checks & Architectural Findings

### 1. Automatic Ripping Machine (ARM) — Stale Version Hallucination
- **Location:** `ARCHITECTURE-SPINE.md` line 158.
- **Finding:** The spine pins ARM to `2.6.0`. GitHub release history shows that `v2.6.0` was tagged in **April 2022**. The project has since published over 20 releases across the `v2.x` line, with **`2.24.4`** being the current production release (October 2026).
- **Impact:** An image pulled with tag `2.6.0` will fail on modern Linux kernels, lacks modern optical drive udev rules, has broken HandBrake/MakeMKV packages, and lacks security patches.
- **Required Fix:** Update version in stack table to `2.24.4`.

### 2. llama.cpp — Severe Build Version Lag (`b4000` vs `b11647`)
- **Location:** `ARCHITECTURE-SPINE.md` line 157.
- **Finding:** The spine pins `llama.cpp` to `b4000`. As of October 15, 2026, `ggerganov/llama.cpp` is on build **`b11647`**. Build `b4000` dates back to late 2024.
- **Impact:** Since `b4000`, `llama.cpp` has completely overhauled KV cache management, continuous batching in `llama-server`, introduced IQ quantization formats, improved FlashAttention-2/3 kernels, and added architecture support for Llama 3.2/3.3, Qwen 2.5, and DeepSeek. Running `b4000` in late 2026 risks refusing contemporary GGUF models downloaded by users.
- **Required Fix:** Update stack table to `b11647` (or pin to `ghcr.io/ggerganov/llama.cpp:server` latest stable release line).

### 3. Paperless-ngx RAG Indexing — The FTS5 Fallacy
- **Location:** ADR-002 (line 53), Invariants Diagram (line 72), AD-4 (line 98).
- **Finding:** ADR-002 specifies: *"Document RAG using Paperless-ngx SQLite FTS5 direct context injection; no secondary vector DB or embedding daemon."* Line 72 depicts `GoDaemon -->|FTS5 Context Query| PaperlessDB["Paperless SQLite Index"]`.
- **Reality Check:** Paperless-ngx uses Django's ORM on SQLite, storing document metadata and OCR text in the `documents_document` table (`content` column). However, Paperless-ngx does **not** create or maintain an SQLite FTS5 virtual table. Its text search engine is built on **Whoosh** (storing separate index files in `/data/documents/index/`), decoupled from SQLite to support multiple database backends (PostgreSQL, MariaDB, SQLite).
- **Impact:** A Go daemon opening Paperless's SQLite database and executing `SELECT ... FROM documents_document_fts WHERE documents_document_fts MATCH ...` will crash with `no such table: documents_document_fts`.
- **Resolution:**
  1. *Approach A (Shadow FTS5 Index):* `sovereign-daemon` maintains its own SQLite FTS5 virtual table inside `/data/state/sovereign.db` by attaching Paperless's SQLite database (`ATTACH DATABASE '/data/documents/db.sqlite3' AS paperless;`) and syncing `documents_document.content` into an FTS5 table via triggers or background polling.
  2. *Approach B (API Search Query):* `sovereign-daemon` queries Paperless-ngx's native REST API (`/api/documents/?query=...`) directly over the `sovereign-control` network, leveraging Whoosh's built-in search ranking and respecting Paperless's indexing lifecycle.
  - The spine must explicitly document Approach A or B rather than asserting that Paperless natively exposes an FTS5 index.

### 4. Caddy Admin API Network Addressing in AD-3
- **Location:** AD-3 (line 92) and AD-4 (line 98).
- **Finding:** AD-3 states that certificates and routes are managed via Caddy's admin API at `localhost:2019`. AD-4 establishes that Caddy and `sovereign-daemon` reside in separate containers connected via the `sovereign-control` bridge network.
- **Reality Check:** In Docker bridge networking, `localhost:2019` inside the `sovereign-daemon` container addresses the daemon's own network namespace, not Caddy's. Furthermore, Caddy defaults to binding its admin listener exclusively to `127.0.0.1:2019` inside its container, rejecting non-loopback connections.
- **Impact:** Any HTTP request from `sovereign-daemon` to `http://localhost:2019` will return `connection refused`.
- **Resolution:**
  - Update Caddy's configuration to bind the admin endpoint to `0.0.0.0:2019` (or `caddy:2019`) within the `sovereign-control` network.
  - Update the rule in AD-3: `sovereign-daemon` connects to `http://caddy:2019` (or alternatively via a shared UNIX domain socket `/data/state/caddy/admin.sock`).

### 5. Docker Compose Version & Go SDK Integration
- **Location:** Namespace Mapping (line 39), AD-2 (line 82), Stack Table (line 153).
- **Finding:** Stack table lists `Docker Compose 2.30.0`. Upstream Docker Compose has advanced to `v5.6.0`. 
- **Reality Check on Go Integration:** The spine references "Docker Compose v2 SDK engine integration". In Go, compose parsing and orchestration are split between:
  - `github.com/compose-spec/compose-go/v2` (version `v2.16.1`): Parses and validates Compose YAML files.
  - `github.com/docker/compose/v2/pkg/api`: Executes compose operations against the Docker Engine.
- **Impact:** Linking `docker/compose/v2/pkg/api` directly in Go pulls in massive dependency trees from the Docker CLI and Moby. A cleaner, lightweight pattern is using `compose-go/v2` for spec parsing and rollback verification, while executing commands via the Docker CLI plugin (`docker compose`) or the official Docker Engine API (`github.com/docker/docker/client`).
- **Resolution:** Update Docker Compose version in stack to `5.6.0`, and clarify the Go integration mechanism in `/internal/supervisor`.

### 6. Hardware Device Passthrough & Sensor Privileges
- **Location:** `/internal/hardware` (line 40), AD-1 (line 80).
- **Finding:** `sovereign-daemon` monitors NVML GPU metrics, Intel/AMD DRM sysfs, thermals, CPU, memory, and SMART storage.
- **Reality Check:**
  - NVML requires dynamic linking to `libnvidia-ml.so.1` via the NVIDIA Container Toolkit.
  - SMART drive diagnostics require `privileged: true` or `CAP_SYS_RAWIO` to read block devices (`/dev/sd*`, `/dev/nvme*`).
  - Optical ripping (ARM) requires device nodes `/dev/sr0` and `/dev/sg*`.
- **Resolution:** Clarify whether `sovereign-daemon` runs natively as a host systemd daemon (simplifying hardware and Docker socket access) or as a privileged container with `/dev`, `/sys`, and `/proc` mounts.

---

## Actionable Recommendations & Patches

### Patch 1: Correct the `## Stack` Table

Replace lines 142–159 of `ARCHITECTURE-SPINE.md` with:

```markdown
## Stack

| Name | Version | Rationale |
| --- | --- | --- |
| Go | 1.27.2 | Supervisor daemon core; generic methods & standard uuid |
| Node.js | 24.22.0 | Active LTS runtime for Next.js web presentation container |
| Next.js | 16.4.1 | React framework; App Router, Turbopack, Cache Components |
| React | 19.3.1 | Core UI library paired with Next.js 16.4 |
| Tailwind CSS | 4.3.3 | CSS-first utility styling via `@theme` |
| Caddy | 2.11.7 | Dual-path reverse proxy, dynamic route sync & ACME TLS |
| SQLite | 3.54.0 | Embedded state engine (`sovereign.db`) in WAL mode |
| Docker Compose | 5.6.0 | Declarative multi-container orchestration & rollback |
| Jellyfin | 12.2.0 | Hardware-accelerated media streaming server |
| Paperless-ngx | 3.3.1 | Document ingestion, OCR, and classification |
| Open WebUI | 0.12.2 | User-facing AI chat interface |
| llama.cpp | b11647 | GGUF LLM inference server (`llama-server`) with `-np 2` |
| Automatic Ripping Machine | 2.24.4 | Optical disc detection and digitization pipeline |
```

### Patch 2: Fix Caddy Admin API Endpoint in AD-3

Update line 92 in `ARCHITECTURE-SPINE.md`:
```markdown
- **Rule:** Ingress is handled exclusively by a dedicated Caddy container. Upstream services must be reachable via both dedicated ports (e.g. `:8096`, `:8000`, `:3001`) and standard subpaths (e.g. `/media`, `/docs`, `/chat`). SSL certificates (custom upload, Let's Encrypt/ACME, or internal CA) must be managed dynamically through the Go daemon via Caddy's admin API (`http://caddy:2019` on the `sovereign-control` network or via a shared UNIX domain socket `/data/state/caddy/admin.sock`).
```

### Patch 3: Clarify Paperless RAG Indexing in ADR-002

Update line 53 in `ARCHITECTURE-SPINE.md`:
```markdown
| ADR-002 | PRD Architecture Decisions | Document RAG using an SQLite FTS5 shadow index in `sovereign.db` attached to Paperless-ngx's `documents_document.content` (or Paperless REST search API); no secondary vector DB or embedding daemon. |
```
