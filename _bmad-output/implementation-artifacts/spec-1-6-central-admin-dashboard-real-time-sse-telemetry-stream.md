---
title: 'Story 1.6: Central Admin Dashboard & Real-Time SSE Telemetry Stream'
type: 'feature'
created: '2026-10-15'
status: 'done'
baseline_commit: 'b8eb1fbaba434b3f8c34be8325822fe4e905b201'
route: 'dispatch'
review_loop_iteration: 0

context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Household users need a single-viewport appliance dashboard with persistent navigation and live hardware telemetry, without page polling or manual refreshes, while keeping public LAN read-only access secure and prompting for administrator authentication on privileged actions.

**Acceptance Criteria:**
- **Given** a user viewing the central dashboard at `/`
- **When** the page renders
- **Then** the persistent 64px `AppHeader` displays the Sovereign crest, aggregate health pill, quick links to `/drop` and `/settings`, and an instant language switcher (EN, DE, ES, FR).
- **And** real-time SSE telemetry from `/api/events` (and `/api/v1/system/events`) updates CPU, RAM, storage usage (`X.X TB / Y.Y TB`), and NAS connection status without page polling (`aria-live="polite"`).
- **And** unauthenticated LAN users can view read-only dashboard cards, while clicking `/settings` or destructive overflow actions prompts for the administrator password via an accessible login modal.
- **And** layout adapts responsively across mobile (375px), tablet (768px), and desktop (1280px+) viewports with accessible 44px touch targets and high-contrast focus rings.

</frozen-after-approval>

## Implementation Notes

- Daemon exposes `GET /api/v1/system/events` and `GET /api/events` with `Content-Type: text/event-stream`.
- Initial telemetry event is pushed immediately on connection and periodically every 5 seconds (or on status change), formatted as `event: telemetry.update\ndata: <json>\n\n`.
- Telemetry payload contains: `cpu_percent`, `memory_used_bytes`, `memory_total_bytes`, `storage_used_bytes`, `storage_total_bytes`, `storage_percent`, `nas_status`, `nas_message`, `overall_status`, `optical_status`, `gpu_status`, `timestamp`.
- Open to unauthenticated LAN callers for read-only telemetry.
- Updated `api/openapi.yaml` and generated frontend contracts via `npm --prefix web run codegen`.
- Built `AppHeader` (64px, brand crest, health pill, `/drop`, `/settings`, language dropdown).
- Built `TelemetryBar` (56px pinned, storage progress bar with >85% amber threshold, NAS status, CPU/RAM metrics).
- Built `ServiceCard` grid displaying configured features (Video, Audio, DMS, AI) with status badge (●/◆/▲), metric readout, and accessible `•••` overflow menu.
- Built accessible `AdminLoginModal` prompting for admin password when an unauthenticated user attempts to access `/settings` or destructive service actions.

## Spec Change Log

## Review Triage Log

- **Status Ordering & Storage Fallback (`cmd/sovereign-daemon/server.go`)**: Applied error precedence over degraded in telemetry calculation, mapped disconnected NAS to degraded overall state, and added fallback to hardware collector storage metric when storage engine reports 0 total.
- **CPU Collector Protection (`internal/hardware/hardware.go`)**: Guarded against `/proc/stat` counter reset/wrap and included `iowait` in idle calculations.
- **SSE Connection Termination on Daemon Shutdown (`cmd/sovereign-daemon/server.go`)**: Added `stopChan` and `closeOnce` to terminate SSE loops gracefully upon `Server.Close()`.
- **Localization & Microcopy (`web/src/components/TelemetryBar.tsx`, `AppHeader.tsx`, `i18n.ts`)**: Added `healthPopoverHealthy`, `healthPopoverAttention`, `systemHealthDetails` keys across all 4 locales (EN, DE, ES, FR), mapped optical and GPU statuses to localized strings, and added click-outside/Escape dismissal and 44px touch targets.
- **Dashboard Interaction & Auth Gating (`web/src/app/page.tsx`, `AdminLoginModal.tsx`)**: Added focus trap and submission guards to `AdminLoginModal`, enabled feature filtering based on `getSettings()`, persisted selected language in `localStorage`, and added `{ withCredentials: true }` to EventSource.
- **Test Coverage Expansion**: Added unit tests in `page.test.tsx`, `useTelemetry.test.ts`, `server_test.go`, and `hardware_test.go`. All 61 frontend tests and all Go tests pass.

## Design Notes

- Colors: Canvas (`#F8F9FA`), Surface (`#FFFFFF`), Border (`#E5E7EB`), Text Primary (`#0F172A`), Text Muted (`#64748B`), Status Green (`#10B981`), Status Amber (`#F59E0B`), Status Red (`#EF4444`), Focus Ring (`#2563EB`).
- Layout: 64px pinned `AppHeader`, 56px pinned `TelemetryBar`, main grid with `h-screen overflow-hidden` on desktop ($\ge 1024$px), unlocking internal scroll when cards exceed viewport.
- Touch Targets: Minimum 44x44px touch target on mobile/tablet.

## Verification

**Commands:**
- `go test -v ./...` -- expected: All Go daemon tests pass including SSE telemetry streaming.
- `npm --prefix web test` -- expected: All frontend unit and component tests pass.
- `npm --prefix web run build` -- expected: Clean production build of Next.js frontend.
