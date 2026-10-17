---
title: 'Story 6.4: Modular Central Settings Hub & Destructive Protection Dialogs'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: 'd4b9eb2989554ef8f3df01f093fd296bdb59efea'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'web/src/app/settings/page.tsx'
  - 'web/src/components/DestructiveModal.tsx'
  - 'web/src/lib/api/client.ts'
  - 'web/src/lib/i18n.ts'
  - 'cmd/sovereign-daemon/server.go'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Appliance administrators need a single, authenticated control center to configure services, inspect physical disks, mount external network storage (NAS), adjust maintenance windows, and manage hardware arbitration. However, performing irreversible or disruptive operations (such as appliance reboot, factory data reset, or service uninstallation) without stringent safeguards risks catastrophic user data loss or disruption to family media and document streaming.

**Approach:**
1. **Modular Central Settings Hub (`web/src/app/settings/page.tsx`)**:
   - Provide an authenticated tabbed interface requiring valid administrator credentials:
     - **Services & Add-ons (`services`)**: Enable/disable sovereign services (Jellyfin, Navidrome, Paperless, AI), view health status indicators, and trigger one-click manual rollbacks.
     - **Storage & Pools (`storage`)**: Display detected storage disks (`GET /api/v1/storage/disks`), pool status (`GET /api/v1/storage/status`), and NAS configuration with inline connection testing (`POST /api/v1/storage/nas/test`) and mount controls (`POST /api/v1/storage/nas/mount`).
     - **AI Engine & Models (`ai`)**: Complete local inference engine management, active model switcher, GGUF model downloader with background progress tracking, inference logs, and generation parameter presets.
     - **Smart Hardware Sharing (`gpu`)**: GPU day/night arbitration schedule visualization and interactive vs batch resource allocation controls.
     - **System & Maintenance (`system`)**: Daily maintenance window (default 03:00 AM), rollback health verification timer (180s), network verification hostname check, and destructive appliance controls.
2. **Dedicated Destructive Protection Modal (`web/src/components/DestructiveModal.tsx`)**:
   - Provide a WCAG AA/AAA accessible dialog (`role="dialog"`, `aria-modal="true"`, focus trapping, Escape dismiss, 44px min touch targets):
     - **Appliance Reboot (`reboot`)**: Informs user the server will temporarily go offline for ~60 seconds with Cancel and Confirm Reboot buttons.
     - **Factory Reset (`factory-reset`)**: High-stakes destructive guard requiring the administrator to explicitly type `"RESET"` into a validation input before the destructive button activates.
     - **Service Deactivation (`deactivate`)**: Informs user that the service will stop running while all media, files, and documents in `/data/` are safely preserved.
3. **Backend Endpoints & API Client**:
   - Add authenticated endpoints in `cmd/sovereign-daemon/server.go`:
     - `POST /api/v1/system/reboot`: Dispatches reboot sequence with safe shutdown of running containers.
     - `POST /api/v1/system/factory-reset`: Validates confirmation token (`"RESET"`), resets application state database, and dispatches reset event.
   - Extend `SovereignApiClient` in `web/src/lib/api/client.ts` with `rebootSystem()` and `factoryReset(confirmation: string)`.
4. **Internationalization & Reassuring Microcopy**:
   - Full 4-language coverage (EN, DE, ES, FR) for all tabs, storage pool actions, NAS testing banners, and destructive dialog warnings.

</frozen-after-approval>

## Acceptance Criteria

1. **Authenticated Access**: Navigating to `/settings` validates the active administrator session; unauthenticated visits prompt administrator login before granting access to configuration tabs.
2. **Tabbed Modular Hub**: Clean tabbed navigation supports Services (`services`), AI Engine (`ai`), Storage & Pools (`storage`), Smart GPU Sharing (`gpu`), and System & Maintenance (`system`).
3. **Storage & NAS Management**: The Storage tab displays disk details, storage capacity status, and provides working inline tests for NAS credentials and mount configuration.
4. **Destructive Protection Confirmation**: Destructive operations (Reboot, Factory Reset, Service Deactivation) trigger `DestructiveModal`; Factory Reset strictly requires typing confirmation `"RESET"` before execution.
5. **Backend Execution**: `POST /api/v1/system/reboot` and `POST /api/v1/system/factory-reset` require admin authentication, return typed RFC 7807 problem details on failure, and execute safely without crashing the daemon.
6. **Accessibility & i18n**: All dialogs, forms, and touch targets satisfy WCAG AA/AAA (≥44px min targets) with full localization across English, German, Spanish, and French.
7. **Automated Verification**: 100% test pass rate across `go test ./...` and `npm --prefix web test`.

## Review Triage Log

| # | Lens | Finding | Resolution |
|---|------|---------|------------|
| 1 | Blind Hunter | `RecordCleanShutdown` needed timeout context in `server.go` | Patched: provided 5-second `context.WithTimeout` context |
| 2 | Blind Hunter | `SetSetting("setup_completed", "false")` error checking and session cookie clearing | Patched: added robust error handling and cookie invalidation |
| 3 | Edge Case Hunter | `isProcessing` dependency triggered premature focus restoration in `DestructiveModal.tsx` | Patched: isolated focus restoration from `isProcessing` via ref |
| 4 | Edge Case Hunter | Zero-byte capacity/usage values formatted as "—" rather than "0 B" | Patched: used `!= null` checks on `used_bytes` / `available_bytes` |
| 5 | Edge Case Hunter | Factory reset input missing Enter key shortcut | Patched: added `onKeyDown` with Enter to submit when valid |
| 6 | Verification Gap | Interactive callback testing missing in `DestructiveModal.test.tsx` | Patched: added button click tests verifying `onClose`, `onConfirm`, and `RESET` validation |
| 7 | Verification Gap | Settings tab click executions needed in `ai-settings.test.tsx` | Patched: added click tests for NAS test/mount and destructive triggers |
| 8 | Blind Hunter | Storage capacity table labels were hardcoded English | Patched: added `totalCapacity`, `usedSpace`, `freeSpace`, `noDisksDetected` across all 4 locales (EN, DE, ES, FR) |
