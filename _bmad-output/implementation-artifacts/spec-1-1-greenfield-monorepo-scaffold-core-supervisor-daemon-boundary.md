---
title: 'Story 1.1: Greenfield Monorepo Scaffold & Core Supervisor Daemon Boundary'
type: 'feature'
created: '2026-10-15'
status: 'review'
baseline_commit: 'ebaca11d28fa65fdb90b31390cef38c02cf2fec9'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sovereign is a greenfield repository lacking the foundational monorepo structure, secure boundary separation, database persistence, and API contract needed for the appliance control plane.

**Approach:** Scaffold the decoupled Go supervisor daemon and Next.js presentation frontend, establish Compose v2 socket isolation, initialize SQLite WAL state persistence at `/data/state/sovereign.db`, define the OpenAPI 3.1 contract with a generated TypeScript client, and configure Nordic/Braun Tailwind CSS v4 design tokens.

## Boundaries & Constraints

**Always:**
- Segregate privileges: only `sovereign-daemon` mounts `/var/run/docker.sock`; `sovereign-web` never mounts the Docker socket or executes host shell commands.
- Enforce SQLite durability: `/data/state/sovereign.db` must open with `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`.
- Use OpenAPI 3.1 as the single source of truth for the API contract, and code-generate the web frontend TypeScript client from it.
- Implement Nordic/Braun design tokens in `/web` matching `DESIGN.md` (Canvas `#F8F9FA`, Surface `#FFFFFF`, Text Primary `#0F172A`, Accent Blue `#2563EB`, rounded 8px).
- Organize daemon packages cleanly under `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}`.

**Never:**
- Never expose `/var/run/docker.sock` to `sovereign-web`.
- Never use hand-rolled frontend API client types that bypass the OpenAPI 3.1 specification.
- Never write database configuration without WAL mode enabled.
- Never introduce sysadmin terminal log jargon into user-facing web boundaries.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Daemon Start with Fresh Storage | Empty `/data/state` directory | Creates `/data/state/sovereign.db`, applies WAL mode and schema, exits 0 | Logs structured error and exits with code 1 if directory is unwritable |
| Daemon Health Probe | GET `/api/v1/health` | Returns HTTP 200 `{"status": "ok", "version": "0.1.0", "database": "connected"}` | Returns HTTP 503 Problem Details if DB check fails |
| Web Client Invocation | API call via generated TS client | HTTP request dispatched to daemon REST endpoint with correct typings | Throws typed Problem Details error on HTTP 4xx/5xx |
| Missing /data Directory | Daemon booted without `/data` subdirs | Recursively creates missing `/data/{media,documents,models,state,drop}` subdirectories | Returns fatal initialization error if permissions prevent directory creation |

</frozen-after-approval>

## Code Map

- `/cmd/sovereign-daemon/main.go` -- Supervisor daemon entrypoint, HTTP server listener, lifecycle signals.
- `/internal/store/db.go` -- SQLite WAL connection pool, `PRAGMA` execution, migration engine.
- `/internal/store/schema.sql` -- Baseline database schema (system state, settings, boot status).
- `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}/` -- Package stubs with package doc and boundaries.
- `/api/openapi.yaml` -- OpenAPI 3.1 specification for daemon endpoints (`/api/v1/health`, `/api/v1/system/boot-status`).
- `/docker-compose.yml` -- Declarative container execution substrate defining `sovereign-daemon` (with socket mount) and `sovereign-web` (isolated).
- `/web/package.json` -- Next.js project configuration (Next.js App Router, React 19, Tailwind CSS v4, TypeScript).
- `/web/src/app/globals.css` -- Tailwind CSS v4 theme variables and design tokens from `DESIGN.md`.
- `/web/src/lib/api/` -- Generated TypeScript client from OpenAPI specification.

## Tasks & Acceptance

**Execution:**
- [x] `/cmd/sovereign-daemon/main.go` -- Implement Go daemon entrypoint with HTTP REST router and graceful shutdown -- Core appliance supervisor entrypoint.
- [x] `/internal/store/db.go` -- Implement SQLite store initialization enforcing WAL mode and normal synchronous pragma -- Ensures power-loss database durability.
- [x] `/internal/store/db_test.go` -- Unit test verifying WAL mode and synchronous pragma on temporary database -- Validates AC requirement.
- [x] `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}/` -- Scaffold internal packages with package documentation -- Monorepo layer boundary establishment.
- [x] `/api/openapi.yaml` -- Author OpenAPI 3.1 specification for initial daemon endpoints (`/api/v1/health`, `/api/v1/system/boot-status`) -- API contract single source of truth.
- [x] `/docker-compose.yml` -- Define Compose specification with network segregation, mounting `/var/run/docker.sock` exclusively into daemon -- Enforces control plane boundary.
- [x] `/web/` -- Bootstrap Next.js App Router project with React 19, TypeScript, and Tailwind CSS v4 -- Frontend presentation layer scaffold.
- [x] `/web/src/app/globals.css` -- Configure CSS custom properties and theme tokens matching `DESIGN.md` -- Establishes Nordic/Braun visual token system.
- [x] `/web/src/lib/api/` -- Generate TypeScript API client from `/api/openapi.yaml` and configure typed fetch wrapper -- Enforces contract-driven frontend integration.
- [x] `/Makefile` -- Add build, test, and codegen commands -- Ergonomic developer substrate.

**Acceptance Criteria:**
- Given a clean development environment, when the monorepo is bootstrapped, then directories `/cmd/sovereign-daemon`, `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}`, `/web`, and `/data/{media,documents,models,state,drop}` exist.
- Given `sovereign-daemon` startup, when connecting to the SQLite database at `/data/state/sovereign.db`, then `PRAGMA journal_mode` reports `wal` and `PRAGMA synchronous` reports `1` (NORMAL).
- Given the Docker Compose specification, when inspected, then `/var/run/docker.sock` is mounted in `sovereign-daemon` and absent from `sovereign-web`.
- Given the OpenAPI 3.1 specification, when the code generator runs, then TypeScript client types and functions in `/web/src/lib/api` match the contract.
- Given `/web/src/app/globals.css`, when inspecting design tokens, then colors Canvas `#F8F9FA`, Surface `#FFFFFF`, Text Primary `#0F172A`, and Accent Blue `#2563EB` are defined.

## Implementation Notes

- Scaffolding & Boundary: Created pure Go supervisor daemon in `/cmd/sovereign-daemon` and decoupled Next.js 16 frontend in `/web`.
- Internal Packages: Scaffolded `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}` with package docstrings and interfaces adhering to architecture invariants.
- Durability & Persistence: Enforced SQLite WAL mode and synchronous=NORMAL (1) via `modernc.org/sqlite` in `/internal/store/db.go`. Verified with unit test `db_test.go`.
- OpenAPI Contract & Client: Authored `/api/openapi.yaml` (OpenAPI 3.1) covering `/api/v1/health` and `/api/v1/system/boot-status`. Generated typed client definitions using `openapi-typescript` and implemented typed fetch client with RFC 7807 `ProblemDetailsError` handling in `/web/src/lib/api/client.ts`. Verified with unit test `client.test.ts`.
- Compose Boundary: Authored `/docker-compose.yml` with network segregation (`sovereign-control`, `sovereign-ingress`, `sovereign-ai`), mounting `/var/run/docker.sock` exclusively into `sovereign-daemon` and completely isolated from `sovereign-web`.
- Design Tokens: Configured Nordic/Braun visual design tokens (Canvas `#F8F9FA`, Surface `#FFFFFF`, Text Primary `#0F172A`, Accent Blue `#2563EB`, rounded 8px) in `/web/src/app/globals.css`.
- Makefile & Developer Tooling: Authored root `/Makefile` with `build`, `test`, `codegen`, `config-check`, and `lint` targets.

## Spec Change Log

## Review Triage Log

## Design Notes

- SQLite WAL Configuration: SQLite WAL mode requires `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`. `modernc.org/sqlite` is used for pure Go CGO-free portability across all architectures.
- OpenAPI Codegen: Uses `openapi-typescript` in `/web` to generate TypeScript types directly from `/api/openapi.yaml`.

## Verification

**Commands:**
- `go test ./...` -- expected: All unit tests pass, verifying SQLite WAL mode and package integrity.
- `npm --prefix web run build` -- expected: Next.js frontend builds cleanly with Tailwind CSS v4 design tokens.
- `docker compose config` -- expected: Valid Compose v2 syntax confirming socket mount in daemon and isolation in web.
