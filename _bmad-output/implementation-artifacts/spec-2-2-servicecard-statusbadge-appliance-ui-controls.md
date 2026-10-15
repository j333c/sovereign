---
title: 'Story 2.2: ServiceCard & StatusBadge Appliance UI Controls'
type: 'feature'
created: '2026-10-15'
status: 'review'
route: 'dispatch'
baseline_commit: '5c0e3ca55b21f4545a0aebab1d6d25b761de67e3'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Household users need clear, non-technical visual status for each appliance service, one-click access in a new browser tab when healthy, accessible management controls (restart, rollback, logs), and reassuring consumer microcopy during memory spikes or restarts without exposing Linux exit codes or raw terminal logs.

**Approach:** Upgrade `ServiceCard`, `StatusBadge`, and `HomePage` to bind to real-time service lifecycle and health SSE events (`service.status_changed`, `service.health_changed`), enable live state transitions (Green ●, Amber ◆, Red ▲), execute real backend restart calls via `SovereignApiClient.restartService()` with auth gating, provide scrubbed diagnostic log downloads, and display localized consumer-grade reassurance microcopy during crashes or restarts.

## Boundaries & Constraints

**Always:**
- Use the canonical geometric triad for status indicators: Green ● ("Running Healthy"), Amber ◆ ("Attention Required" or "Starting..."), Red ▲ ("Stopped / Error").
- Accessible 44x44px touch targets on mobile/tablet for all interactive buttons, including overflow menus (`•••`).
- When Green, "Open [Service] UI" opens the application in a new tab (`target="_blank" rel="noopener noreferrer"`) routed through Caddy.
- When starting or degraded, button state clearly communicates active state ("Starting..." or disabled).
- Destructive actions (Restart, Rollback) require admin authentication via `AdminLoginModal` if unauthenticated.
- Service crashes or memory pressure events display consumer reassurance microcopy (*"Video Streaming paused due to high memory use. Automatically restarting..."*) and strictly eliminate raw Linux error messages, exit codes, and stack traces.
- Keyboard accessible with full focus rings (`focus-visible:ring-2 focus-visible:ring-blue-600`), and overflow menus close on `Escape` or click-outside.
- All copy localized across all 4 supported languages (EN, DE, ES, FR).

**Never:**
- Never display raw container exit codes (e.g. `exit 137`) or terminal stack traces to users.
- Never use generic JavaScript `alert()` dialogs in production components.
- Never mount Docker sockets or run privileged operations on the client.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Healthy Service | Status `healthy`, running | Green circle ● `Running Healthy`, primary action button enabled "Open [Service] UI" opening in new tab | N/A |
| Starting / Warming Up | Status `starting`, activating | Amber diamond ◆ `Starting...`, primary action button displays loading state | N/A |
| Memory Crash / Degraded | Container OOM killed or sustained probe failure | Amber diamond ◆ or Red triangle ▲ with consumer microcopy *"Paused due to high memory use. Automatically restarting..."* | Graceful reassurance banner |
| Restart Action (Auth) | User clicks Restart in overflow menu while authenticated | Calls `apiClient.restartService(id)`, shows temporary starting indicator, updates on SSE response | ProblemDetails modal on failure |
| Restart Action (Unauth) | User clicks Restart in overflow menu while unauthenticated | Opens `AdminLoginModal`; executes restart upon successful login | Resets on cancel |
| Download Scrubbed Logs | User clicks Download Diagnostic Logs in overflow menu | Generates and downloads sanitized text log with timestamp, service name, status; zero personal data | Clean file download |

</frozen-after-approval>

## Code Map

- `web/src/components/ServiceCard.tsx` -- Update ServiceCard with live action handlers, state-aware primary action button, consumer reassurance alert banner, and sanitized log generation.
- `web/src/components/StatusBadge.tsx` -- Ensure full semantic geometry mapping for running, starting, degraded, stopped, and error.
- `web/src/lib/i18n.ts` -- Add translation keys for reassurance crash microcopy, starting/restarting states, and action results across EN, DE, ES, FR.
- `web/src/app/page.tsx` -- Subscribe to service lifecycle SSE events (`service.health_changed`, `service.status_changed`), wire real `apiClient.restartService()` calls, and handle pending admin restarts.
- `web/src/app/page.test.tsx` -- Component tests for ServiceCard interaction, status badges, reassurance microcopy, and overflow menus across all locales.

## Tasks & Acceptance

**Execution:**
- [x] `web/src/lib/i18n.ts` -- Add localized microcopy for memory pauses, starting, restarting, and service actions -- Deliver reassuring consumer copy in EN, DE, ES, FR.
- [x] `web/src/components/ServiceCard.tsx` -- Wire live restart/rollback actions, state-aware action buttons, and memory pressure reassurance alert -- Eliminate alert() and enhance UX.
- [x] `web/src/components/StatusBadge.tsx` -- Verify geometric pairing (●/◆/▲) and color contrast -- Meet accessibility standards.
- [x] `web/src/app/page.tsx` -- Integrate service SSE events and real backend restart calls with auth modal -- Connect frontend to daemon container lifecycle.
- [x] `web/src/app/page.test.tsx` -- Add comprehensive tests for ServiceCard controls, overflow menu, restart actions, and crash microcopy -- Verify acceptance criteria.

**Acceptance Criteria:**
- Given the central dashboard, each service card renders a geometric `StatusBadge` (●/◆/▲).
- When healthy, clicking "Open [Service] UI" opens a new tab directed to the service URL.
- When unauthenticated, triggering restart prompts for admin credentials via `AdminLoginModal` and executes on success.
- High memory or degraded status displays reassuring microcopy without raw system errors.
- Accessible 44px touch targets and full keyboard navigation.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

- Reassurance microcopy: *"Video Streaming paused due to high memory use. Automatically restarting..."*
- Colors: Running `#10B981` (Green ●), Degraded/Starting `#F59E0B` (Amber ◆), Error/Stopped `#EF4444` (Red ▲).
