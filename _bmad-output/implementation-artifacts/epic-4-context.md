# Epic 4 Context: Private Document Archiving & Smart Home Alert Bridge

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Provide private, centralized document management and proactive smart home deadline alerting for Sovereign: deploy an on-demand Paperless-ngx instance with capped, low-priority OCR worker concurrency to prevent CPU starvation of interactive services; support zero-friction document ingestion via mobile scanner companion apps (iOS/Android) and automated IMAP/SSL email polling with attachment extraction; execute automated deadline and payment obligation scanning over OCR text using a lightweight deterministic regex parser paired with scheduled nightly LLM refinement; and bridge impending contractual or financial due dates (within 14 days) to household ambient displays via Home Assistant webhooks and MQTT (`sovereign/alerts/documents`) alongside consolidated administrative email digests. This transforms physical household paperwork and PDF billing attachments into an indexed, full-text searchable archive with automated obligation tracking that prevents missed payments and contract renewals without sysadmin effort or cloud privacy compromises.

## Stories

- Story 4.1: Centralized Document Management Integration (Paperless-ngx)
- Story 4.2: Multi-Channel Ingestion (Mobile Scanner & Automated IMAP Email Polling)
- Story 4.3: Deadline Extraction Parser & Obligation Scanner
- Story 4.4: Home Assistant Webhook, MQTT & Email Alert Dispatcher

## Requirements & Constraints

- **On-Demand Container Deployment**: Paperless-ngx must pull and deploy strictly on-demand from official upstream OCI images (`paperlessngx/paperless-ngx`) only after explicit user activation, mounting persistent storage under `/data/documents` and state under `/data/state/paperless`.
- **OCR Worker Concurrency & CPU Throttling**: Tesseract/Paperless OCR worker processes must be strictly resource-bounded: worker concurrency is capped to `max(1, physical_cores / 2)` and executed at low priority (`nice -n 10` / low CPU shares). Bulk document intake must never starve interactive AI chat, video streaming, or real-time system health probes.
- **Full-Text Indexing & SQLite Durability**: All ingested documents (PDF, PNG, JPEG, TIFF) must be OCR'd and indexed into SQLite FTS5 tables across content, tags, correspondents, and dates. All database operations must enforce `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`.
- **Dual-Path Ingress Routing**: Ingress operates via dedicated port `:8000` (for ecosystem mobile companion scanner apps on iOS and Android) and reverse-proxied subpath `/docs` (via Caddy with path trimming and `X-Forwarded-Prefix`) for single-domain web navigation.
- **Multi-Channel Mobile Scanning**: Seamless local network compatibility with standard Paperless mobile companion apps on iOS and Android without requiring custom mobile clients.
- **Automated IMAP/SSL Email Polling**: `sovereign-daemon` polls configured household mailboxes over IMAP/SSL using user-supplied credentials, automatically downloading and queuing PDF attachments into the ingestion pipeline, and marking emails as processed.
- **Graceful Mailbox Error Handling & Degradation**: If mailbox polling fails or credentials change, the Document Archive ServiceCard transitions to Amber status (*"Automatic Text Search Active · Mailbox Disconnected"*) with a direct re-authentication action without crashing or restarting container runtimes.
- **Deterministic Obligation Scanning**: Newly indexed document text and metadata are automatically evaluated by a lightweight deterministic regex parser to identify expiration dates, payment totals, and due dates.
- **Nighttime Batch LLM Refinement**: Supplement deterministic parsing with an off-peak nightly LLM extraction pass during the nighttime batch window (01:00 to 07:00) via the internal AI Gateway, validating ambiguous contractual renewal clauses without daytime GPU contention.
- **Impending Obligation Queue**: Critical deadlines within a configurable threshold (default: 14 days) are deduplicated and queued in `sovereign.db` with document title, due date, monetary total, and source excerpt citation.
- **Home Assistant Webhook & MQTT Event Bridge**: Impending deadline events dispatch to Home Assistant via webhook and publish to MQTT topic `sovereign/alerts/documents`, enabling ambient kitchen wall tablet glance cards.
- **Consolidated Email Digest**: Sovereign dispatches consolidated notification emails via SMTP (`/internal/notify`) to the administrator email address detailing upcoming obligations with direct links to the archived documents.

## Technical Decisions

- **Containerization & Upstream Image**:
  - Paperless-ngx pinned to official upstream release `paperlessngx/paperless-ngx:3.3.1` (or latest stable 3.3.x).
  - Managed via Docker Compose v2 template synthesized by `/internal/supervisor`.
  - Required Redis task broker runs as an isolated companion container on `sovereign-internal`.
- **Resource Allocation & Concurrency Bounds**:
  - Environment variables enforce CPU limits: `PAPERLESS_OCR_WORKERS=max(1, physical_cores / 2)`, `PAPERLESS_TASK_WORKERS=max(1, physical_cores / 2)`, `PAPERLESS_OCR_PROCESSES=1`.
  - Background processes scheduled with `nice -n 10` and bounded memory limits (`mem_limit`).
- **Filesystem Layout & Volume Mounts**:
  - Documents library: `/data/documents/media` (archive), `/data/documents/data` (index/app data), `/data/documents/consume` (ingestion queue), `/data/documents/export` (backups).
  - State directory: `/data/state/paperless`.
  - Unified POSIX storage layout under `/data` guarantees zero-copy moves and atomic directory operations.
- **Subsystem Architecture & Code Ownership**:
  - `/internal/supervisor`: Container lifecycle, healthchecks, on-demand activation/deactivation, and compose synthesis.
  - `/internal/storage`: Directory provisioning, permissions, and `/data/documents/consume` drop pipeline.
  - `/internal/documents` (or `/internal/parser`): IMAP client polling loop, deterministic regex extraction engine, and nighttime LLM batch coordination.
  - `/internal/notify`: Home Assistant webhook dispatcher, MQTT client publisher, and SMTP email digest delivery.
  - `/internal/store`: SQLite schema migrations and repositories for `document_deadlines` and `imap_configs`.
- **Data Models & State Storage**:
  - Persistent database entities use UUIDv7 identifiers.
  - `document_deadlines`: `id`, `document_id`, `title`, `due_date`, `amount`, `currency`, `snippet`, `status` (`pending`, `notified`, `dismissed`), `created_at`, `updated_at`.
  - `imap_configs`: `id`, `host`, `port`, `username`, `password_encrypted`, `folder`, `ssl_enabled`, `last_polled_at`, `last_error`.
  - All SQLite operations strictly enforce WAL journal mode.
- **Two-Tier Extraction Pipeline**:
  - Tier 1: Real-time deterministic regex parser evaluates dates (ISO and European formats) and payment amounts upon document indexing.
  - Tier 2: Scheduled nightly LLM batch worker (01:00–07:00) queries the internal AI Gateway (`/v1/chat/completions`) using Generation Intent Preset *Precise & Factual* (temperature 0.1) with low-priority lane headers to prevent interference with interactive chat.
- **Alert Dispatch Protocol**:
  - Home Assistant webhook: HTTP POST with JSON payload (`event: "document_deadline"`, `title`, `due_date`, `days_remaining`, `amount`, `url`).
  - MQTT event: Topic `sovereign/alerts/documents`, QoS 1, non-retained JSON payload.
  - Email notification: HTML and plain-text digest sent via SMTP containing direct local document links (`http://sovereign.local/docs/documents/:id`).
- **Telemetry & Event Dispatching**:
  - Ingestion progress, IMAP polling status, and deadline events stream via Server-Sent Events (`/api/events`) using domain actions: `dms.document_ingested`, `dms.ocr_completed`, `dms.deadline_detected`, `dms.mailbox_status`.

## UX & Interaction Patterns

- **ServiceCard Presentation**:
  - Card title: "Document Archive", Tech descriptor: "Paperless-ngx".
  - StatusBadge: Green Circle (●) "Running Healthy", Amber Diamond (◆) "Attention Required · Mailbox Disconnected", Red Triangle (▲) "Stopped / Error".
  - Metric counters: Document count (e.g., "142 Documents") with subtitle "Automatic Text Search Active · Mailbox Connected".
  - Action button: Full-width "Open Document Archive UI" opening `:8000` or `/docs` in a new browser tab.
  - Overflow menu (`•••`): Accessible actions for *Restart Service*, *Roll Back to Previous Version*, and *Download Diagnostic Logs*.
- **Administrative Settings Surface (`/settings#dms` / `/settings#alerts`)**:
  - Mailbox Configuration Card: IMAP server host, port (993), SSL toggle, username, password with "Test Mailbox Connection" button for immediate status validation.
  - Smart Home & Alert Settings: Home Assistant webhook URL input, MQTT broker settings (host, port, topic prefix), deadline alert threshold slider (default: 14 days), and notification email recipient.
- **Consumer-Grade Microcopy & Error Handling**:
  - Strictly eliminates Linux command-line jargon, Python tracebacks, and raw regex errors:
    - Normal operation: *"Automatic Text Search Active · Mailbox Connected"*
    - Mailbox disconnection: *"Mailbox disconnected: We couldn't connect to your email account. Please check your email password in Settings."*
    - Impending bill alert: *"Upcoming Bill: Insurance payment of $120.00 is due in 7 days."*
    - Ingestion progress: *"Scanning and indexing incoming document..."*
- **Accessibility & Touch Standards**:
  - Full WCAG 2.1 AA compliance with minimum 4.5:1 text contrast and 3:1 heading contrast.
  - Interactive touch targets enforce minimum 44x44px dimensions.
  - Visible focus indicators (`2px #2563EB`) on all inputs and buttons.
  - `aria-live="polite"` for asynchronous status toasts and live badge updates.

## Cross-Story Dependencies

- **Story 4.1 (Centralized Document Management Integration - Paperless-ngx)** provides the core container environment, persistent storage mounts (`/data/documents`), OCR engine, SQLite FTS5 indexing, dedicated port `:8000`, and `/docs` Caddy routing. Stories 4.2, 4.3, and 4.4 rely on this running container and its filesystem/API.
- **Story 4.2 (Multi-Channel Ingestion - Mobile Scanner & Automated IMAP Email Polling)** routes mobile scanner uploads into `/data/documents/consume` and implements the background IMAP/SSL email polling worker in `sovereign-daemon`, delivering PDFs into Story 4.1's OCR pipeline and providing mailbox status telemetry.
- **Story 4.3 (Deadline Extraction Parser & Obligation Scanner)** monitors newly indexed documents from Story 4.1, executes deterministic regex parsing and scheduled nightly AI refinement, and queues detected obligations in `sovereign.db`.
- **Story 4.4 (Home Assistant Webhook, MQTT & Email Alert Dispatcher)** consumes impending obligations from Story 4.3 and dispatches alerts across Home Assistant webhooks, MQTT topic `sovereign/alerts/documents`, and SMTP email digests.
- **Upstream Foundation**: Depends on Epic 1 for the Go supervisor daemon (`sovereign-daemon`), SQLite WAL storage engine (`/data/state/sovereign.db`), Caddy ingress gateway, and base admin dashboard; and Epic 2 for the container lifecycle engine, ServiceCard / StatusBadge UI components, and the staged dropzone ingestion pattern.
- **Downstream Capabilities**: Directly feeds Epic 5 (Local AI Knowledge & Turnkey Inference Platform), where Paperless-ngx's SQLite FTS5 index and document store serve as the single source of truth for conversational local AI querying (RAG) with page citations, without redundant vector database overhead.
