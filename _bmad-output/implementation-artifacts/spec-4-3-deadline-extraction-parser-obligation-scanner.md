---
title: 'Story 4.3: Deadline Extraction Parser & Obligation Scanner'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: '3f231f61ec72c5a4867d33973cb75468192bfc24'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Family members receive bills, policy renewals, and contractual cancellation deadlines scattered across various paper scans and emailed PDFs. Without automated inspection, impending payment obligations or renewal dates slip past unnoticed until late fees or unwanted automatic renewals occur.

**Approach:** Implement a two-tier deadline and obligation extraction engine: Tier 1 executes immediate deterministic regex parsing on indexed document text and OCR output to extract due dates, invoice totals, and renewal terms; Tier 2 schedules an off-peak nighttime LLM extraction pass (01:00 to 07:00) using the internal AI Gateway with the *Precise & Factual* intent preset (temperature 0.1) and low-priority lane headers to refine ambiguous contractual clauses without daytime CPU/GPU contention; detected deadlines within a configurable threshold (default: 14 days) are deduplicated and queued in SQLite table `document_deadlines` and broadcast via SSE (`dms.deadline_detected`).

## Boundaries & Constraints

**Always:**
- Store extracted deadlines in SQLite table `document_deadlines` (`id`, `document_id`, `title`, `due_date`, `amount`, `currency`, `snippet`, `status`, `created_at`, `updated_at`) enforcing WAL durability and foreign key consistency.
- Implement Tier 1 deterministic regex parsing supporting ISO format (`YYYY-MM-DD`), European numerical formats (`DD.MM.YYYY`, `DD/MM/YYYY`), and multilingual full-text date patterns (English and German month names, e.g. "due by October 25, 2026", "fällig am 15. November 2026", "Kündigungsfrist bis 31.12.2026").
- Parse monetary amounts across currencies: USD (`$120.00`), EUR (`120,00 €`, `€120.00`), GBP (`£120.00`), CHF (`120.00 CHF`).
- Implement Tier 2 off-peak nighttime LLM batch pass scheduled exclusively between 01:00 and 07:00 local time, querying `/v1/chat/completions` using low-priority lane headers (`X-Priority: background`, `X-Lane: batch`).
- Gracefully handle LLM gateway absence or offline state: Tier 1 regex extraction serves as the authoritative baseline, guaranteeing full function without LLM availability.
- Deduplicate deadline entries by `document_id` and normalized `due_date` so repeated scans or polling runs do not produce duplicate alerts.
- Filter pending obligations to those within the configurable alert threshold (default: 14 days from current date, ignoring past dates older than 30 days).
- Broadcast `dms.deadline_detected` Server-Sent Event for each newly identified impending deadline.
- Provide REST endpoints for querying detected deadlines (`GET /api/v1/documents/deadlines`), updating status (`POST /api/v1/documents/deadlines/{id}/dismiss`), and triggering manual scan (`POST /api/v1/documents/deadlines/scan`).

**Never:**
- Never block interactive user actions, media playback, or document ingestion on LLM inference.
- Never run LLM extraction passes during peak daytime hours (07:00 to 01:00) unless explicitly requested via administrative manual scan.
- Never crash the daemon or fail document ingestion if regex parsing or LLM inference encounters unparseable text or malformed JSON responses.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Tier 1 Regex Parse (Invoice) | Document text: *"Invoice #482. Total Due: $150.00 by 2026-10-28"* | Extracts `due_date: "2026-10-28"`, `amount: 150.00`, `currency: "USD"`, queues in `document_deadlines`, emits `dms.deadline_detected` | Logs parse failure, skips unparseable dates |
| Tier 1 German Format | Document text: *"Fälligkeitsdatum: 25.10.2026. Gesamtbetrag: 89,50 €"* | Extracts `due_date: "2026-10-25"`, `amount: 89.50`, `currency: "EUR"`, status `pending` | Ignores invalid calendar dates (e.g. 31.02) |
| Deadline Beyond Threshold | Document due date is 45 days in future (threshold: 14 days) | Record stored in `document_deadlines` as `future`, excluded from active 14-day alert queue | Non-critical obligation tracked for future alerts |
| Duplicate Ingestion | Document ingested second time with identical text | Deduplication recognizes existing `document_id` and `due_date`, updates existing record without duplicate ID | Prevents duplicate dashboard alert cards |
| Tier 2 Nighttime LLM Pass | Current time 02:30, unparsed contractual document with ambiguous renewal clause | Sends prompt to internal AI Gateway at `temperature: 0.1`, parses structured JSON response `{ "due_date": "...", "amount": ... }`, updates deadline record | Falls back gracefully to Tier 1 if LLM times out or returns malformed JSON |
| Daytime LLM Invariance | Document ingested at 14:00 | Tier 1 regex runs immediately; Tier 2 LLM pass is queued for off-peak nighttime window (01:00-07:00) | No GPU usage during daytime hours |
| Dismiss Deadline | `POST /api/v1/documents/deadlines/{id}/dismiss` | Status updated to `dismissed`, excluded from upcoming alert queries | 404 Not Found if deadline ID does not exist |
| Query Deadlines | `GET /api/v1/documents/deadlines?status=pending&within_days=14` | Returns JSON array of impending obligations ordered by `due_date ASC` | 200 OK |

</frozen-after-approval>

## Code Map

- `internal/store/schema.sql` -- `document_deadlines` SQLite table schema with WAL durability.
- `internal/store/deadlines.go` -- Repository for `document_deadlines` (Create, Update, Get, ListPending, Dismiss, Deduplicate).
- `internal/documents/regex_parser.go` -- Deterministic Tier 1 regex engine for date formats (ISO, European, full month names in EN/DE), currency symbols, and monetary amounts.
- `internal/documents/llm_extractor.go` -- Tier 2 LLM extractor client querying `/v1/chat/completions` with Precise & Factual preset and background lane headers.
- `internal/documents/scanner.go` -- Obligation scanner coordinator: triggers Tier 1 on document ingest, schedules nighttime batch window (01:00-07:00), queues impending deadlines within threshold, emits SSE events.
- `cmd/sovereign-daemon/server.go` -- Wire `ObligationScanner`, register REST endpoints (`GET /api/v1/documents/deadlines`, `POST /api/v1/documents/deadlines/{id}/dismiss`, `POST /api/v1/documents/deadlines/scan`), hook into document ingestion event.
- `internal/documents/deadlines_test.go` -- Unit tests for regex parser (EN/DE dates, currencies, amounts), LLM extractor mock, deduplication, and window scheduling.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for deadline endpoints, dismissal, and ingestion pipeline triggering.
- `api/openapi.yaml` -- Document deadline endpoints, request/response schemas, and `dms.deadline_detected` SSE event.
- `web/src/lib/api/` -- Codegen and client SDK methods for querying and dismissing deadlines.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/schema.sql` & `internal/store/deadlines.go` -- Implement `document_deadlines` table schema and CRUD repository.
- [x] `internal/documents/regex_parser.go` -- Implement deterministic Tier 1 date, amount, and currency parser.
- [x] `internal/documents/llm_extractor.go` -- Implement Tier 2 off-peak LLM extractor client.
- [x] `internal/documents/scanner.go` -- Implement `ObligationScanner` managing 2-tier pipeline, 14-day threshold, nighttime window, deduplication, and SSE events.
- [x] `cmd/sovereign-daemon/server.go` -- Register `/api/v1/documents/deadlines` (GET), `/api/v1/documents/deadlines/{id}/dismiss` (POST), `/api/v1/documents/deadlines/scan` (POST), and wire scanner into ingest pipeline.
- [x] `internal/documents/deadlines_test.go` & `cmd/sovereign-daemon/server_test.go` -- Unit and integration tests covering regex parsing, currency detection, threshold filtering, LLM fallback, and dismissal.
- [x] `api/openapi.yaml` & `web/src/lib/api/` -- Update OpenAPI specification and regenerate client typings.

**Acceptance Criteria:**
- Given newly indexed document text, deterministic regex parser inspects dates, payment totals, and contract terms.
- An off-peak nightly LLM extraction pass (01:00-07:00) validates and refines detected deadlines without daytime contention.
- Detected deadlines within the threshold (default: 14 days) are deduplicated and queued in `sovereign.db` with document title, due date, monetary total, and source excerpt citation.

## Implementation Notes

- **Store**: Added `document_deadlines` SQLite table schema to `internal/store/schema.sql` with unique index on `(document_id, due_date)` and index on `(status, due_date)`. Implemented `store.DeadlineStore` with WAL durability and deduplication logic that preserves existing IDs and dismissed status on rescan.
- **Deterministic Regex Parser**: Implemented `internal/documents/regex_parser.go` supporting ISO (`YYYY-MM-DD`), European (`DD.MM.YYYY`, `DD/MM/YYYY`), English and German textual month names (full and abbreviated), calendar date validation via `ValidateCalendarDate` (rejecting 31.02, 29.02 in non-leap years), and multilingual currency amounts across USD (`$`), EUR (`€`, `EUR`), GBP (`£`), and CHF.
- **Tier 2 LLM Extractor**: Implemented `internal/documents/llm_extractor.go` targeting `/v1/chat/completions` on the internal AI gateway with background priority lane headers (`X-Priority: background`, `X-Lane: batch`), `temperature: 0.1` (*Precise & Factual* preset), JSON response parsing with markdown code block stripping, and graceful fallback to Tier 1 regex results upon gateway absence, timeouts, or malformed responses.
- **Obligation Scanner Coordinator**: Implemented `internal/documents/scanner.go` managing the two-tier extraction pipeline, 14-day alert threshold (`pending` within threshold vs `future` beyond threshold, ignoring past dates older than 30 days), nighttime window guard (01:00-07:00 exclusively for off-peak LLM inference), deduplication, and SSE `dms.deadline_detected` broadcasting.
- **Daemon Integration**: Wired `DeadlineStore` and `ObligationScanner` in `cmd/sovereign-daemon/server.go`. Registered REST endpoints `GET /api/v1/documents/deadlines`, `POST /api/v1/documents/deadlines/{id}/dismiss`, and `POST /api/v1/documents/deadlines/scan`. Hooked into `dms.document_ingested` SSE events to asynchronously trigger obligation scanning on newly ingested documents without blocking uploads.
- **OpenAPI & Web SDK**: Updated `api/openapi.yaml` with deadline routes, request/response schemas, and `DMSDeadlineDetectedEvent`. Regenerated TypeScript typings in `web/src/lib/api/schema.d.ts` and extended `SovereignApiClient` with `getDeadlines`, `dismissDeadline`, and `scanDeadlines`.
- **Verification**: Verified with full Go unit/integration test suites (`go test ./...`) and web TypeScript build/test suite (`npm test`, `npm run build`), achieving 100% test pass rate.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

### Review Loop 1 (3-Layer Parallel Review: Blind Hunter, Edge Case Hunter, Verification Gap)
- Unified Diff under review: `/tmp/spec_4_3_diff.patch` (112 KB against baseline commit `3f231f61ec72c5a4867d33973cb75468192bfc24`)
- **Triaged Findings & Remediations**:
  1. *Blind Hunter Finding - German Month Names & Non-Due Date Fallback*: Added missing German months ("mai", "mrz") and abbreviations in `regex_parser.go`. Prevented fallback to all date matches when candidate dates were exclusively marked as non-due dates (`isNonDue`, e.g. invoice issue date "Rechnungsdatum: 12.03.2026"), avoiding false deadline extractions.
  2. *Blind Hunter Finding - Amount Selection & Thousands Separator*: Refactored `findAmountInString` in `regex_parser.go` to collect candidate amounts across all patterns, prioritize total indicators ("Gesamt", "Total", "Zahlbetrag", etc.), and select the largest amount on the document rather than breaking on the first line item. Updated `parseNumericAmount` and amount regexes with word boundary anchors `\b` to accurately parse thousands commas (e.g. `$1,250`) without mistaking them for decimal amounts (`$1.25`).
  3. *Edge Case Hunter Finding - Timezone Normalization in Day Offset Calculation*: Replaced `math.Floor(dueDate.Sub(truncDate(now)).Hours() / 24)` with `calendarDaysDiff(dueDateStr, now)` in `scanner.go`, normalizing both `now` and `dueDate` to midnight UTC to prevent off-by-one errors from local timezone offsets or DST shifts.
  4. *Edge Case Hunter Finding - SSE Event Deduplication*: In `ScanDocumentText`, added inspection of `prior` record to only broadcast `dms.deadline_detected` for newly created pending obligations or promotions from `future` to `pending`, avoiding duplicate SSE spam when re-scanning indexed documents.
  5. *Edge Case Hunter Finding - LLM Gateway URL Configuration*: Updated daemon initialization in `server.go` to inspect `SOVEREIGN_AI_GATEWAY_URL` and `SOVEREIGN_LLM_URL` with fallback to `http://127.0.0.1:8080`, replacing the erroneous `cfg.CaddyURL` (port 2019).
  6. *Edge Case Hunter Finding - Default Query Window on Status=Future*: In `handleGetDeadlines`, set `withinDays = 0` (unconstrained) when querying future deadlines and `within_days` is omitted, rather than defaulting to 14 days which would filter out all future deadlines.
  7. *Verification Gap Finding - Web SSE Listener Integration*: Updated `web/src/lib/useTelemetry.ts` to export `onDMSDeadlineDetected` and listen for `dms.deadline_detected` SSE events, adding test coverage in `useTelemetry.test.ts`.
  8. *Verification Gap Finding - Unit & Integration Test Coverage*: Added test cases in `deadlines_test.go` and `server_test.go` covering nightly batch promotion, LLM 503 fallback to regex baseline, unparameterized deadline queries, empty body manual scans, and thousands comma formatting.
- **Verification Status**:
  - `go test ./...`: 100% PASS across all packages.
  - `npm --prefix web test`: 100% PASS across all 91 test cases.

