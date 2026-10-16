---
title: 'Story 4.2: Multi-Channel Ingestion (Mobile Scanner & Automated IMAP Email Polling)'
type: 'feature'
created: '2026-10-16'
status: 'review'
route: 'dispatch'
baseline_commit: '930d325a69c8e25c53b58ca4fe47ff9f06af319a'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sarah and household family members need zero-friction ingestion of both physical mail scanned via mobile phones and electronic PDF bills received via email into the private Paperless-ngx archive without manual sorting, file transfers, or risk of container restarts when mailbox credentials expire.

**Approach:** Implement multi-channel document ingestion supporting Paperless mobile companion apps and HTTP document uploads into `/data/documents/consume`, coupled with a resilient background IMAP/SSL email polling worker in `sovereign-daemon` that extracts PDF attachments, marks emails as processed, and gracefully degrades Document Archive ServiceCard status to Amber (*"Automatic Text Search Active · Mailbox Disconnected"*) upon IMAP connection or authentication errors without terminating container runtimes.

## Boundaries & Constraints

**Always:**
- Route mobile companion scanner uploads and direct document drops into `/data/documents/consume/` with secure file permissions (`0644`).
- Support document file types: `.pdf`, `.png`, `.jpg`, `.jpeg`, `.tiff`, `.tif`.
- Store IMAP mailbox configurations in SQLite table `imap_configs` (`id`, `host`, `port`, `username`, `password_encrypted`, `folder`, `ssl_enabled`, `enabled`, `last_polled_at`, `last_error`, `created_at`, `updated_at`) with WAL durability.
- Encrypt stored IMAP passwords at rest using AES-256-GCM authenticated encryption.
- Connect over standard TLS/SSL (default port 993) and support user-specified folders (default `INBOX`).
- Search for unseen messages (`SEARCH UNSEEN`), extract PDF and image attachments into `/data/documents/consume/`, and mark processed messages as seen (`+FLAGS (\Seen)`).
- When IMAP authentication or connection fails, update `last_error`, transition Document Archive ServiceCard health to `degraded`, and display reassurance microcopy (*"Mailbox disconnected: We couldn't connect to your email account. Please check your email password in Settings."*) without restarting or stopping Paperless container runtimes.
- Emit Server-Sent Events (`/api/events`) for `dms.document_ingested` and `dms.mailbox_status`.
- Provide authenticated REST endpoints for IMAP configuration, connection testing, and document upload.

**Never:**
- Never crash, stop, or restart Paperless or spoke containers when an IMAP polling attempt fails.
- Never store plaintext email passwords in the SQLite database or log files.
- Never delete or modify non-attachment message content or purge mailboxes without explicit configuration.
- Never expose raw Linux socket errors, stack traces, or IMAP protocol strings to the user UI.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Mobile / Direct Upload | `POST /api/v1/documents/upload` with multipart PDF/image | File written to `/data/documents/consume/<uuid>_<name>`, emits `dms.document_ingested`, returns 201 Created | 400 Bad Request for unsupported file format or empty payload |
| Dropzone Category "documents" | `POST /api/v1/drop/upload` with category `documents` and PDF file | File staged and moved to `/data/documents/consume/`, emits `dms.document_ingested` | 400 Bad Request if file format incompatible |
| IMAP Poll Success | Active IMAP config with 1 unread message containing invoice.pdf | invoice.pdf saved to `/data/documents/consume/`, message marked `\Seen`, `last_polled_at` updated, emits `dms.document_ingested` and `dms.mailbox_status` (connected: true) | Logs error if attachment extraction fails |
| IMAP Auth Failure | IMAP server responds `NO [AUTHENTICATIONFAILED]` | `last_error` recorded, ServiceCard degrades to Amber (status: running, health: degraded, reassurance: "Mailbox disconnected..."), emits `dms.mailbox_status` (connected: false) | Container runtime remains untouched and healthy |
| IMAP Connection Timeout | Remote IMAP server unreachable / offline | `last_error` recorded, ServiceCard degrades to Amber, poller backs off gracefully | 502/504 on manual test; container unaffected |
| IMAP Test Connection | `POST /api/v1/documents/imap/test` with host/credentials | Connects, logs in, checks folder, disconnects; returns 200 `{ "success": true }` | 400/502 with reassuring consumer message |
| IMAP Config Save | `POST /api/v1/documents/imap` with new credentials | Encrypts password, persists to `imap_configs`, triggers immediate test/poll | 400 Bad Request on invalid host/port/fields |
| Get IMAP Settings | `GET /api/v1/documents/imap` | Returns config with masked password (`"********"`), connection status, last polled timestamp, and last error | 200 OK |

</frozen-after-approval>

## Code Map

- `internal/store/schema.sql` -- SQLite table `imap_configs` schema with WAL durability.
- `internal/store/imap.go` -- IMAP config repository, AES-256-GCM encryption/decryption of mailbox secrets, CRUD operations.
- `internal/documents/imap_client.go` -- Lightweight RFC 3501 IMAP/SSL client implementation (Connect, Login, Select, SearchUnseen, FetchMessage, MarkSeen, Close).
- `internal/documents/poller.go` -- Background polling worker, attachment extraction to `/data/documents/consume/`, status tracking, and event emission.
- `internal/documents/ingest.go` -- Direct document upload handler and file validator (`.pdf`, `.png`, `.jpg`, `.jpeg`, `.tiff`).
- `internal/storage/dropzone.go` -- Extend `ValidCategories` with `CategoryDocuments = "documents"`, route to `/data/documents/consume/`.
- `cmd/sovereign-daemon/server.go` -- Wire IMAP poller, register REST endpoints (`/api/v1/documents/upload`, `/api/v1/documents/imap`, `/api/v1/documents/imap/test`), and update ServiceCard health / reassurance messages.
- `internal/documents/imap_test.go` -- Unit tests for IMAP client, mock server, attachment extraction, and password encryption.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for document upload, IMAP settings, connection testing, and Amber degradation without container restarts.
- `api/openapi.yaml` -- Document new document endpoints, IMAP schemas, and SSE events.
- `web/src/app/page.tsx` -- Update ServiceCard status mapping and reassurance display for Amber mailbox state.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/schema.sql` & `internal/store/imap.go` -- Implement `imap_configs` table and AES-256-GCM encrypted credential repository.
- [x] `internal/documents/imap_client.go` & `internal/documents/poller.go` -- Implement IMAP/SSL client and background polling loop with attachment extraction to `/data/documents/consume/`.
- [x] `internal/documents/ingest.go` & `internal/storage/dropzone.go` -- Support document ingestion via direct upload and dropzone category `documents`.
- [x] `cmd/sovereign-daemon/server.go` -- Register `/api/v1/documents/upload`, `/api/v1/documents/imap`, `/api/v1/documents/imap/test`, wire poller events, and update ServiceCard health.
- [x] `internal/documents/imap_test.go` & `cmd/sovereign-daemon/server_test.go` -- Comprehensive unit and integration tests covering happy path, auth failure, network timeout, and volume preservation.
- [x] `api/openapi.yaml` -- Update OpenAPI specification for IMAP configuration, document upload, and event payloads.

**Acceptance Criteria:**
- Given a Paperless-compatible mobile scanner or direct HTTP upload, documents upload directly to `/data/documents/consume/` for automated OCR.
- When IMAP credentials are configured, `sovereign-daemon` polls the mailbox over SSL, downloading PDF attachments and marking emails as processed.
- If mailbox polling fails or credentials change, the Document Archive ServiceCard transitions to Amber status (*"Automatic Text Search Active · Mailbox Disconnected"*) with reassurance microcopy, without stopping or restarting container runtimes.

## Implementation Notes

- **Credential Encryption & Durability**: `imap_configs` table created in SQLite with WAL durability (`schema.sql`). Passwords encrypted at rest using AES-256-GCM with PBKDF2/SHA256 key derivation and settings salt (`internal/store/imap.go`). Masked as `********` across all public API and log outputs.
- **Lightweight RFC 3501 IMAP/SSL Client**: Built native TLS socket client (`internal/documents/imap_client.go`) with multi-part MIME parsing, quoted-printable/base64 decoding, magic byte verification, and safe `\Seen` flag persistence.
- **Background Polling & Zero Container Interruptions**: Poller worker runs non-blocking loop (`internal/documents/poller.go`), moving attachments to `/data/documents/consume/` with 0644 permissions. On authentication or connectivity errors, transitions Paperless health to amber (`degraded`) with reassurance copy while ensuring container runtimes are strictly preserved without termination or restarts.
- **Multi-Channel Ingestion**: Implemented direct upload engine (`internal/documents/ingest.go`) and dropzone category `documents` (`internal/storage/dropzone.go`), strictly rejecting incompatible non-document files with HTTP 400/422.
- **REST Endpoints & SSE Events**: Added `/api/v1/documents/upload`, `/api/v1/documents/imap` (GET/POST), `/api/v1/documents/imap/test`, and broadcasted `dms.document_ingested` and `dms.mailbox_status`.
- **OpenAPI & Frontend Dashboard**: Contract documented in `api/openapi.yaml`. Updated TypeScript client (`web/src/lib/api/client.ts`), telemetry hooks (`web/src/lib/useTelemetry.ts`), and ServiceCard rendering (`web/src/app/page.tsx`) with full multilingual reassurance copy across EN, DE, ES, and FR (`web/src/lib/i18n.ts`).
- **Test Coverage**: All unit, integration, and frontend tests pass cleanly (89 frontend tests; 100% passing Go tests across `sovereign/cmd/sovereign-daemon`, `internal/documents`, `internal/store`, `internal/storage`).

## Spec Change Log

*(No changes)*

## Review Triage Log

*(Populated during review)*
