---
title: 'Story 1.5: Five-Step First-Run Setup Wizard & RBAC Authentication'
type: 'feature'
created: '2026-10-15'
status: 'done'
baseline_commit: 'be12bd4323692c60fe64571ee94fc55b32d5dad8'
route: 'dispatch'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** An out-of-the-box appliance must tailor its language, network, storage, initial features, and administrative credentials without command-line intervention, while protecting administrative settings through secure session authentication and RBAC.

**Approach:** Implement Argon2id credential hashing, session tokens, and RBAC middleware in `/internal/auth`, paired with a guided 5-step setup wizard on `/setup` in Next.js (Language, Network, Storage, Features, Admin credentials) that transitions seamlessly to the operational dashboard at `/` with an `HttpOnly`, `SameSite=Strict` session cookie and zero reboot required.

## Boundaries & Constraints

**Always:**
- If the server is unconfigured, visits to `/` must redirect to `/setup`.
- The setup wizard must provide 5 distinct steps:
  - Step 1: Language selection (English, German, Spanish, French).
  - Step 2: Network verification and hostname confirmation (`sovereign.local`).
  - Step 3: Storage pool initialization (Local disk formatting or NAS NFS/SMB mount).
  - Step 4: Feature selection checklist (Video, Audio, DMS, Local AI).
  - Step 5: Admin password creation (hashed with Argon2id) and alert email address.
- Administrator passwords must be hashed using Argon2id with cryptographically secure random salts.
- Authenticated sessions must issue an `HttpOnly`, `SameSite=Strict` cookie (`sovereign_session`).
- Public LAN endpoints without authentication are strictly limited to `/` (dashboard), `/drop` (file upload), `/boot` (diagnostics), `/setup` (when unconfigured), and public status/telemetry endpoints.
- Clicking "Complete Setup" must immediately authenticate the session and navigate to `/` without requiring a system reboot.

**Never:**
- Never store administrator passwords in plaintext or using weak hash algorithms (MD5, SHA-1, plain SHA-256).
- Never allow unauthenticated users to access administrative mutation endpoints (`/api/v1/settings/*`, restart, rollback, or post-setup re-configuration).
- Never require a hardware or server reboot to finish the onboarding wizard.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| First Boot Unconfigured | GET `/api/v1/auth/setup-status` | Returns `{"is_configured": false}` | N/A |
| Step 1 Language Choice | User selects "DE" | Updates UI language instantly to German | Falls back to English if locale unsupported |
| Step 3 Storage Integration | User selects NAS in wizard | Calls storage API to test credentials and mount share | Displays inline error if NAS credentials invalid |
| Step 5 Submit Setup | POST `/api/v1/auth/setup` with password, email, features, lang | Hashes password with Argon2id, creates admin session, sets `is_configured: true`, sets cookie | Returns HTTP 400 Problem Details if password < 8 chars or email invalid |
| Access Dashboard Post-Setup | User visits `/` after setup | Renders central dashboard with session active | Unauthenticated users view read-only dashboard |
| Re-running Setup after Initialized | POST `/api/v1/auth/setup` when already configured | Rejects request with HTTP 403 / 409 Problem Details | Prevents unauthorized admin overwrite |

</frozen-after-approval>

## Code Map

- `/internal/auth/auth.go` -- Argon2id hashing, password verification, session token generation, session store in SQLite, and RBAC authentication middleware.
- `/internal/auth/auth_test.go` -- Unit tests for Argon2id hashing, verification, session issuance/expiry, and RBAC route protection.
- `/cmd/sovereign-daemon/server.go` -- Handlers for `GET /api/v1/auth/setup-status`, `POST /api/v1/auth/setup`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/session`, and RBAC middleware.
- `/api/openapi.yaml` -- Document auth and setup endpoints in OpenAPI contract.
- `/web/src/app/setup/page.tsx` -- 5-step guided First-Run Setup Wizard with inline validation and progress bar.
- `/web/src/app/setup/setup-wizard.test.tsx` -- Component tests verifying 5 steps, language switching, storage integration, and completion transition.
- `/web/src/app/page.tsx` -- Updated root page to redirect to `/setup` if unconfigured.
- `/web/src/lib/api/client.ts` -- Updated client methods: `getSetupStatus`, `completeSetup`, `login`, `logout`, `getSession`.

## Tasks & Acceptance

**Execution:**
- [x] `/internal/auth/auth.go` -- Implement Argon2id password hashing, session tokens, SQLite session persistence, and RBAC middleware -- Provides core security & auth substrate.
- [x] `/internal/auth/auth_test.go` -- Unit tests for password hashing, session validation, and RBAC authorization rules -- Verifies authentication invariants.
- [x] `/cmd/sovereign-daemon/server.go` -- Add setup and session REST endpoints with `HttpOnly`, `SameSite=Strict` cookie issuance -- Exposes auth APIs to frontend.
- [x] `/api/openapi.yaml` -- Add auth/setup endpoints and schemas to OpenAPI 3.1 specification -- Contract single source of truth.
- [x] `/web/src/app/setup/page.tsx` -- Implement 5-step guided setup wizard with progress indicator and seamless `/` transition -- Delivers onboarding UX per UX-DR4.
- [x] `/web/src/app/setup/setup-wizard.test.tsx` -- Component tests for setup wizard step navigation, validation, and completion -- Validates wizard flow.
- [x] `/web/src/app/page.tsx` -- Redirect to `/setup` on first boot if `is_configured === false` -- Ensures first-boot onboarding routing.
- [x] `/web/src/lib/api/` -- Regenerate TypeScript types and add auth/setup methods to `SovereignApiClient` -- Provides frontend client integration.

**Acceptance Criteria:**
- Given an unconfigured server on first boot, when the user visits `http://sovereign.local`, then the browser opens `/setup` with Step 1 (Language selection: EN, DE, ES, FR).
- Given Step 2, network settings and hostname are validated, and Step 3 executes storage pool initialization or NAS mount test.
- Given Step 4, selectable checkboxes are presented for Video, Audio, DMS, and Local AI features.
- Given Step 5, an administrator password hashed with Argon2id and an alert email address are recorded, issuing an authenticated session cookie (`HttpOnly`, `SameSite=Strict`).
- Given clicking "Complete Setup", the system transitions directly to the operational dashboard at `/` without requiring a server reboot.

## Implementation Notes

- Implemented Argon2id hashing parameters (`time=1`, `memory=64MB`, `threads=4`, `keyLen=32`, 16-byte random salt) conforming to standard PHC string formatting.
- Integrated SQLite session persistence with 30-day token expiration and automatic revocation.
- Added `HttpOnly`, `SameSite=Strict` cookie issuance (`sovereign_session`) and `Authorization: Bearer` extraction.
- Handled RBAC middleware enforcing `RoleAdmin` on destructive administrative routes (`/api/v1/settings`).
- Added setup and auth endpoints in `cmd/sovereign-daemon/server.go`: `GET /api/v1/auth/setup-status`, `POST /api/v1/auth/setup`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/session`.
- Documented all endpoints and schemas in `api/openapi.yaml` and regenerated frontend TypeScript contracts.
- Built 5-step guided wizard in `web/src/app/setup/page.tsx` supporting EN, DE, ES, and FR locale switching, inline NAS testing/local pool formatting, and seamless direct redirect to `/`.
- Updated `web/src/app/page.tsx` to automatically redirect unconfigured first boots to `/setup`.

## Spec Change Log

## Review Triage Log

- **Zero-length decoded hash and salt bypass in `VerifyPassword` (`internal/auth/auth.go`)**: Fixed by asserting `len(salt) > 0` and `len(expectedHash) > 0` before checking.
- **Zero cost parameters panic prevention in `VerifyPassword` (`internal/auth/auth.go`)**: Added check ensuring `memory > 0 && t > 0 && p > 0`.
- **Role hierarchy check in `RequireRole` (`internal/auth/auth.go`)**: Updated to properly enforce hierarchical roles (`RoleAdmin` > `RoleUser` > `RoleGuest`), rejecting `RoleGuest` sessions from `RoleUser` endpoints.
- **TOCTOU race condition in `CompleteSetup` (`internal/auth/auth.go`)**: Re-checked `setup_completed` inside the SQLite transaction before committing.
- **Argon2id DoS prevention via password length ceiling (`internal/auth/auth.go`)**: Added `MaxPasswordLength = 256` constant and rejection of oversized passwords.
- **User enumeration timing discrepancy in `Authenticate` (`internal/auth/auth.go`)**: Added dummy hash execution for non-existent users so query duration remains uniform.
- **Accumulation of expired sessions (`internal/auth/auth.go`, `cmd/sovereign-daemon/server.go`)**: Added `PruneExpiredSessions` and invoked it during daemon startup.
- **CORS Origin reflection and credentials security (`cmd/sovereign-daemon/server.go`)**: Added `isAllowedOrigin` validation restricting credentialed CORS reflection strictly to localhost, LAN IPs, and sovereign.local.
- **Post-setup storage mutation protection (`cmd/sovereign-daemon/server.go`)**: Added `requireSetupOrAdmin` middleware locking down `/api/v1/storage/initialize`, `/test`, and `/mount` to admin users once initial setup has completed.
- **Omission of `/api/v1/settings` in OpenAPI specification (`api/openapi.yaml`)**: Added `GET` and `POST` `/api/v1/settings` and `SettingsResponse` to `openapi.yaml`, regenerated TypeScript definitions with `codegen`, and added client methods to `SovereignApiClient`.
- **Language omitted from `handleSetupStatus` (`cmd/sovereign-daemon/server.go`)**: Updated `handleSetupStatus` to include `language` in response when configured.
- **Setup screen already-configured check (`web/src/app/setup/page.tsx`)**: Added `client.getSetupStatus()` on mount, redirecting to `/` if setup is already completed.
- **Step 3 storage configuration enforcement & disk change reset (`web/src/app/setup/page.tsx`)**: Reset `diskInitSuccess(false)` upon selecting another disk, and required disk initialization or NAS mounting before advancing from Step 3 to Step 4.
- **Step 2 hostname validation (`web/src/app/setup/page.tsx`)**: Enforced RFC 1123 hostname format check before advancing to Step 3.
- **Browser locale detection (`web/src/app/setup/page.tsx`)**: Initialized language state from `navigator.language` when supported.
- **Unhandled JSON parsing error on 204 No Content (`web/src/lib/api/client.ts`)**: Handled HTTP 204 and empty responses safely.
- **Verification gap in test suites**:
  - Added unit tests for CORS with Origin header and credentials in `cmd/sovereign-daemon/server_test.go`.
  - Added unit tests for `POST /api/v1/settings` (persisting and auth check) and post-setup storage protection in `cmd/sovereign-daemon/server_test.go`.
  - Added unit tests for zero-cost parameters, empty hash components, role hierarchy, and session pruning in `internal/auth/auth_test.go`.
  - Added unit tests for credentials header inclusion, 204 handling, and settings API in `web/src/lib/api/client.test.ts`.
  - Added unit tests for `checkBootAndSetup` component logic in `web/src/app/page.test.tsx`.
  - Added unit tests for already-configured check in `web/src/app/setup/setup-wizard.test.tsx`.

## Design Notes

- Argon2id Parameters: `time = 1`, `memory = 64 * 1024` (64 MB), `threads = 4`, `keyLen = 32`, cryptographically secure 16-byte salt via `crypto/rand`.
- Session Cookie: `Set-Cookie: sovereign_session=<token>; Path=/; HttpOnly; SameSite=Strict; Max-Age=2592000`.
- Setup Status: Saved in SQLite `settings` table as `setup_completed = "true"`.

## Verification

**Commands:**
- `go test -v ./...` -- expected: All unit tests pass, verifying Argon2id hashing, session auth, and setup endpoints.
- `npm --prefix web test` -- expected: All frontend tests pass, including setup wizard step progression.
- `npm --prefix web run build` -- expected: Next.js builds cleanly with `/setup` route.
