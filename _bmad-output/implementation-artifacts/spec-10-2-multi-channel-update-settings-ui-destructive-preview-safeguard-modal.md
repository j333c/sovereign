---
title: 'Story 10.2: Multi-Channel Update Settings UI & Destructive Preview Safeguard Modal'
type: 'feature'
created: '2026-11-10'
status: 'done'
baseline_commit: '31322659e98e82ef7d9d06b537be4d6da2bb21d6'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/epics.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:**
Administrators need to select their update cadence (`Stable`, `Security-only`, or `Preview`) from the Settings UI with immediate visual feedback. Switching to the experimental `Preview` channel introduces forward database migrations and unfinished features that cannot be downgraded without a snapshot restore. Administrators must be protected by an explicit amber destructive confirmation modal requiring a typed confirmation token before applying the change. Furthermore, when `Preview` is active, the top header must display an amber badge indicating that preview builds are in use, and switching from `Preview` back to `Stable` or `Security-only` must enter a safe "Graduating to Stable" hold that never downgrades to an older release.

**Approach:**
1. **OpenAPI & Backend REST Endpoints:**
   - Update `api/openapi.yaml` to include `update_channel` in `SettingsResponse`, `TelemetryUpdate`, and `BootStatusResponse`.
   - Register `PUT /api/v1/settings` in `api/openapi.yaml` and `routes.go`.
   - Run `npm --prefix web run codegen`.
   - In `handleGetSettings`, return `update_channel` (defaulting to `"stable"`).
   - In `handleUpdateSettings`, validate `update_channel` and require confirmation token (`"PREVIEW"`, `"VORSCHAU"`, `"PRELIMINAR"`, `"VISTA PREVIA"`, `"APERÇU"`, `"APERCU"`) when switching to `preview`.
   - Include `update_channel` in `handleBootStatus` and telemetry SSE broadcasts.
2. **I18n Key Parity (`web/src/lib/i18n/`):**
   - Add keys across English, German, Spanish, and French for channel selection, reassurance text, "Graduating to Stable" hold notices, preview modal copy, and header badge.
3. **DestructiveModal Enhancement (`web/src/components/DestructiveModal.tsx`):**
   - Add `actionType="preview-channel"`.
   - Implement localized confirmation token validation for preview across all four languages.
   - Display consumer reassurance copy: *"Preview releases contain experimental features and forward database migrations that cannot be downgraded without a snapshot restore."*
4. **Settings UI Integration (`web/src/components/settings/SystemSettingsTab.tsx`):**
   - Render Update Channel selector loaded from `apiClient.getSettings`.
   - Immediately persist `stable` or `security-only` changes with reassuring toast.
   - Trigger `DestructiveModal` when `preview` is chosen, reverting dropdown on cancel/dismiss.
   - Render "Graduating to Stable" banner when running a preview version and switching back to `stable` or `security-only`.
5. **AppHeader Preview Badge (`web/src/components/AppHeader.tsx`):**
   - Render secondary amber pill (`data-testid="preview-channel-pill"`, *"Preview Channel Active"*) when `updateChannel === "preview"`.

## Boundaries & Constraints

**Always:**
- Enforce typed confirmation token for preview channel on both client and server.
- Maintain 100% key parity across `en`, `de`, `es`, `fr` dictionaries.
- Revert dropdown state if destructive confirmation modal is cancelled.
- Show "Graduating to Stable" notice when moving from preview build to stable/security-only.
- Never hand-edit `web/src/lib/api/schema.d.ts`.

## Code Map

- `api/openapi.yaml` -- Declare `update_channel` in schemas and `PUT /api/v1/settings`.
- `cmd/sovereign-daemon/routes.go` -- Register `PUT /api/v1/settings`.
- `cmd/sovereign-daemon/handlers_auth_settings.go` -- Handle `update_channel` in GET and PUT/POST with preview token guard.
- `cmd/sovereign-daemon/handlers_system.go` -- Include `update_channel` in boot status.
- `cmd/sovereign-daemon/telemetry_sse.go` -- Include `update_channel` in telemetry updates.
- `web/src/lib/i18n/types.ts`, `en.ts`, `de.ts`, `es.ts`, `fr.ts` -- Localized strings.
- `web/src/components/DestructiveModal.tsx` -- Support `actionType="preview-channel"` and localized tokens.
- `web/src/components/settings/SystemSettingsTab.tsx` -- Update channel dropdown and graduation hold banner.
- `web/src/components/AppHeader.tsx` -- Secondary amber preview pill.
- `web/src/components/DestructiveModal.test.tsx` -- Tests for preview token validation.
- `web/src/components/settings/SystemSettingsTab.test.tsx` -- Tests for channel switching, modal trigger, and graduation state.
- `web/src/components/AppHeader.test.tsx` -- Tests for preview channel badge.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- Track story status.

## Verification Plan

### Automated Tests
1. `go test -count=1 ./cmd/sovereign-daemon/...`
   - Test `GET /api/v1/settings` returns `update_channel`.
   - Test `POST/PUT /api/v1/settings` changing to preview requires `"PREVIEW"` token.
   - Test `POST/PUT /api/v1/settings` changing to stable or security-only succeeds without token.
2. `npm --prefix web test`
   - Verify DestructiveModal preview validation across EN/DE/ES/FR.
   - Verify SystemSettingsTab channel dropdown, toast, modal trigger, and graduation notice.
   - Verify AppHeader preview pill rendering.
   - Verify i18n key parity test.
