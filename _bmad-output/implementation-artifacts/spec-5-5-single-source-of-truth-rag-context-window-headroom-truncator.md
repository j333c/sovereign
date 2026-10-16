---
title: 'Story 5.5: Single-Source-of-Truth RAG & Context Window Headroom Truncator'
type: 'feature'
created: '2026-10-16'
status: 'in-progress'
route: 'dispatch'
baseline_commit: '421b591b619f7fa5160d8ff04983a45c3614fc2a'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Family members asking the local AI questions about insurance policies, utility bills, appliance manuals, or tax obligations need verified, hallucination-free answers cited directly from their household document archive. Existing RAG solutions duplicate gigabytes of text into heavy external vector databases (Chroma, Qdrant, Milvus) that consume excessive RAM and drift out of sync. Furthermore, naive context injection overflows the LLM's context window, causing HTTP 400 Bad Request errors or dropping crucial prompt instructions.

**Approach:** Implement Single-Source-of-Truth Retrieval-Augmented Generation (RAG) and a Context Window Headroom Truncator:
1. **Single-Source-of-Truth SQLite FTS5 Retrieval (`internal/ai/rag.go`)**:
   - Queries Paperless-ngx's existing SQLite database (`/data/state/services/paperless/db.sqlite3` or `/data/documents/data/db.sqlite3`) directly using FTS5 full-text search and BM25 rank scoring.
   - Zero external vector databases, zero duplicate indexing, zero data drift.
   - Exact citation tracking: extracts document ID, title, page numbers, and snippet offsets, formatting clickable links to `/docs/documents/:id`.
2. **Context Window Headroom Truncator**:
   - Validates total token length pre-flight against the model's context capacity (default 4096 tokens).
   - Reserves a dedicated, non-negotiable **512-token response buffer** for the LLM output.
   - Deducts system prompt tokens and user query tokens to compute available context headroom.
   - If retrieved document excerpts exceed headroom:
     - Automatically prioritizes and retains top-ranked chunks.
     - Appends an explanatory notice across all 4 supported languages:
       - EN: *"Note: Document excerpts were prioritized to fit your active context window."*
       - DE: *"Hinweis: Dokumentenauszüge wurden priorisiert, um in das aktive Kontextfenster zu passen."*
       - ES: *"Nota: Los extractos de documentos se priorizaron para ajustarse a su ventana de contexto activa."*
       - FR: *"Remarque : Les extraits de documents ont été hiérarchisés pour s'adapter à votre fenêtre de contexte active."*
   - Completely prevents HTTP 400 context overflow errors.
3. **Daemon RAG Pipeline & Gateway Orchestration (`cmd/sovereign-daemon/server.go`)**:
   - `POST /api/v1/ai/rag/query`: Orchestrates document retrieval, context truncation, prompt synthesis with citations, and inference execution via the AI Gateway.
   - `POST /api/v1/ai/rag/search`: Standalone document search endpoint returning ranked document matches, snippets, page numbers, and citation links.
   - Integrates with the Two-Lane Priority Request Governor (`Lane 1` for interactive chat).

## Boundaries & Constraints

**Always:**
- Query Paperless-ngx SQLite FTS5 directly without standing up external vector databases.
- Reserve a dedicated 512-token response buffer during context window headroom calculation.
- Include explicit document titles and page numbers with clickable links to `/docs/documents/:id` in citations.
- Append a localized explanatory notice when document excerpts are truncated to fit headroom.
- Handle missing, locked, or unpopulated Paperless databases gracefully (return empty results without panicking).
- Maintain CGO-free modernc.org/sqlite compatibility and WAL read concurrency.

**Never:**
- Never duplicate document text or run vector database containers (Chroma/Qdrant/Milvus).
- Never allow context length to exceed the active model's context limit minus the 512-token headroom buffer.
- Never crash or return HTTP 400 when document query yields large or numerous matches.
- Never emit document contents, search queries, or embeddings to cloud APIs.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| RAG Document Query | `POST /api/v1/ai/rag/query` with `{"query": "deductible on car insurance", "language": "en"}` | Searches FTS5, extracts snippets from matching policy doc, truncates with 512-token headroom, generates response citing doc title and page | Returns answer with citations; if no matches found, states no matching documents found |
| Context Exceeds Headroom | Query matches 20+ large document pages (>3500 tokens) | Truncator retains top-ranked chunks within headroom; appends localized notice; query succeeds | No HTTP 400; prompt truncated gracefully |
| Paperless Database Locked / Busy | Paperless is performing heavy ingest during query | SQLite driver uses WAL mode and busy timeout; reads succeed concurrently | Returns graceful error or fallback if db inaccessible |
| Paperless Database Unprovisioned | Paperless service not yet initialized / no db.sqlite3 | RAG engine detects missing DB; returns empty matches with helpful notice | Returns HTTP 200 with zero matches and explanatory notice |
| Multi-Language Query | Query submitted in German (`lang: "de"`) | Truncation warning, citations, and system prompt generated in German | Fallback to English if language unspecified |
| Exact Page Extraction | Document contains page markers (`\x0c` or OCR page headers) | Citations calculate and report exact page number (e.g. "Page 3") | Falls back to page 1 if page markers absent |

</frozen-after-approval>

## Code Map

- `internal/ai/rag.go` -- SQLite FTS5 retrieval engine, BM25 rank scoring, page citation calculator, and Context Window Headroom Truncator.
- `internal/ai/rag_test.go` -- Unit tests covering FTS5 search, headroom truncation, page citation extraction, and localization.
- `cmd/sovereign-daemon/server.go` -- Endpoints `POST /api/v1/ai/rag/query` and `POST /api/v1/ai/rag/search`.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for RAG query, headroom truncation notice, and empty database handling.
- `api/openapi.yaml` -- OpenAPI schemas for RAG query request/response and document citations.
- `web/src/lib/api/client.ts` -- TypeScript API client methods `queryRAG` and `searchRAG`.
- `web/src/lib/i18n.ts` -- Localized strings for RAG notices and citations.

## Implementation Plan

1. **RAG & Truncator Engine (`internal/ai/rag.go`, `internal/ai/rag_test.go`)**:
   - Implement `RAGEngine` connecting to Paperless SQLite (`db.sqlite3`).
   - Query `documents_document` and FTS5 table with BM25 ranking.
   - Implement `HeadroomTruncator`: token estimation, 512-token reserved buffer, top-chunk retention, localized notice injection.
   - Implement page number calculation from document OCR content (`\x0c` form feeds and page break markers).
   - Unit tests covering all scenarios.
2. **REST Endpoints (`cmd/sovereign-daemon/server.go`)**:
   - Register `POST /api/v1/ai/rag/query` and `POST /api/v1/ai/rag/search`.
   - Wire RAG engine into `Server` struct.
   - Integration tests in `server_test.go`.
3. **OpenAPI & Frontend Client (`api/openapi.yaml`, `web/`)**:
   - Update OpenAPI specification and generate TypeScript types (`npm run codegen`).
   - Add `queryRAG` and `searchRAG` in `SovereignApiClient`.
   - Update `i18n.ts`.
4. **Automated Verification**:
   - Run `go test ./...` and `npm --prefix web test`.
