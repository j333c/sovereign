---
title: 'Story 6.3: One-Click Manual Rollback & Rollback Confirmation Modal'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: '1b994d87287f4f2f66522c9f2c71e9b3cf3e05e9'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'web/src/components/ServiceCard.tsx'
  - 'web/src/components/RollbackModal.tsx'
  - 'web/src/app/page.tsx'
  - 'web/src/app/settings/page.tsx'
  - 'web/src/lib/i18n.ts'
  - 'web/src/lib/api/client.ts'
  - 'cmd/sovereign-daemon/server.go'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** While Story 6.2 automated rollbacks for unattended healthcheck failures during overnight maintenance, users who manually upgrade or test new service versions during the day (e.g., Jellyfin, Paperless, Navidrome, Open WebUI) may notice visual glitches, broken plugins, or subtle regressions that do not trigger automated crash loop detectors. In an appliance environment, family users must not be expected to use terminal SSH or Docker commands to revert to a prior working version.

**Approach:**
1. **Dedicated `RollbackModal` Component (`web/src/components/RollbackModal.tsx`)**:
   - Provide a WCAG AA/AAA accessible dialog (`role="dialog"`, `aria-modal="true"`, focus trapping, Escape key dismiss) that clearly displays:
     - The service name and human-readable icon
     - Current Version / Image tag
     - Target Restored Version / Prior Image tag
     - Pre-Upgrade Backup Snapshot Timestamp (e.g. formatted local date/time)
     - Reassurance note: *"Your media, files, and documents are preserved and will not be touched."* (localized across EN, DE, ES, and FR)
     - Explicit Cancel button (44px touch target) and Confirm Rollback button (44px touch target) with inline loading spinner during execution.
2. **Dashboard Integration (`web/src/app/page.tsx` & `web/src/components/ServiceCard.tsx`)**:
   - When the user selects "Roll Back to Previous Version" from the service card overflow menu (`•••`):
     - If unauthenticated, prompt administrator password login via `AdminLoginModal` before opening the rollback confirmation dialog.
     - Fetch the latest revision metadata for the service via `apiClient.getRevisions()`.
     - Open `RollbackModal` populated with actual current vs previous version tags and snapshot timestamp.
     - On user confirmation, execute `apiClient.rollbackService(backendId)`.
     - Upon successful rollback, update the service card status to Green (● Running Healthy) or restored state, clear any lingering error/degraded microcopy, and display a reassuring success toast notification (*"Service rolled back successfully"* or localized equivalent).
3. **Settings Hub Integration (`web/src/app/settings/page.tsx`)**:
   - Under the Services/Maintenance tab in Settings, display revision history and provide a one-click "Roll Back" action for each installed service, reusing `RollbackModal` to maintain a consistent experience.
4. **Localization (`web/src/lib/i18n.ts`)**:
   - Ensure complete 4-language support (English, German, Spanish, French) for modal title, current version label, previous version label, snapshot timestamp label, reassurance copy, cancel/confirm buttons, and rollback success/failure toasts.
5. **Robust Error Handling**:
   - If rollback fails or no prior revision exists, display an RFC 7807 Problem Details dialog without throwing unhandled exceptions or breaking the UI.

</frozen-after-approval>

## Acceptance Criteria

1. **Service Card Overflow Action**: Clicking "Roll Back to Previous Version" in the ServiceCard overflow menu opens `RollbackModal` with admin authentication enforcement.
2. **Rollback Confirmation Details**: `RollbackModal` displays current version tag, previous version tag, backup snapshot timestamp, and the reassurance note: *"Your media, files, and documents are preserved and will not be touched."*
3. **One-Click Atomic Rollback Execution**: Confirming rollback invokes `POST /api/v1/services/{id}/rollback`, restoring compose revision, configuration volumes, and pre-upgrade SQLite snapshot within 30 seconds.
4. **Dashboard State Update & Success Toast**: Following successful rollback, the modal closes, the service card reflects operational status, and a localized success notification is displayed.
5. **Settings Hub Availability**: Users can also trigger service rollback from the Settings hub with full confirmation protection.
6. **Accessibility & i18n**: The modal meets WCAG touch target (≥44px) and dialog accessibility standards, and copy renders accurately across EN, DE, ES, and FR.
7. **Automated Verification**: Comprehensive unit and integration test suites pass with 100% success (`go test ./...` and `npm --prefix web test`).

## Review Triage Log

### Iteration 1 Review Findings
- **Blind Hunter Reviewer**:
  - Dashboard revision state disconnect when already authenticated: `ServiceCard` sets `localRollbackOpen(true)` without notifying `page.tsx` (`onRollbackOpen`). [Action: Patched — Added `onRollbackOpen` callback to `ServiceCardProps` and wired to `setActiveRollbackService` in `page.tsx`].
  - Incomplete modal accessibility and lack of focus trap (`Tab`/`Shift+Tab`) in `RollbackModal.tsx`. [Action: Patched — Implemented bidirectional focus trapping and restored focus to triggering element on dialog dismiss].
  - Escape key conflict during active rollback in `ServiceCard.tsx`. [Action: Patched — Removed redundant window keydown handler from `ServiceCard` in favor of `RollbackModal` with `!isRollingBack` guard].
  - Confirm rollback button enabled when no prior revision exists. [Action: Patched — Disabled confirm button when `!previousVersion || previousVersion === currentVersion`].
  - Misleading placeholder copy for missing current version. [Action: Patched — Fixed `formatImageTag` to show neutral placeholder `—` for current version].
  - Unsafe revision lookup matching `r.service_id === ""`. [Action: Patched — Added `&& r.service_id !== ""` check in both `page.tsx` and `settings/page.tsx`].
  - Untranslated English badge "SQLite WAL + Config". [Action: Patched — Added `walAndConfig` to `i18n.ts` across EN, DE, ES, FR].
  - Toast timer leak on unmount in `page.tsx` and `settings/page.tsx`. [Action: Patched — Added `rollbackToastTimerRef` and cleanup `useEffect`].
  - Backdrop overlay click dismiss. [Action: Patched — Added backdrop click handler in `RollbackModal`].
- **Edge Case Hunter Reviewer**:
  - RFC 7807 problem details modal on rollback failure in settings page. [Action: Patched — Rendered global `role="alertdialog"` for `actionError` in `settings/page.tsx`].
  - Buttons touch target in Settings Services tab. [Action: Patched — Upgraded button min-height to `min-h-[44px]`].
- **Verification Gap Reviewer**:
  - Missing integration test for HomeScreen rollback execution, state update, and toast. [Action: Patched — Added comprehensive integration test in `web/src/app/page.test.tsx`].
  - Missing integration test for Settings page service rollback workflow and failure dialog. [Action: Patched — Added interactive test in `web/src/app/settings/ai-settings.test.tsx`].

### Automated Verification
- `npm --prefix web test`: 124/124 tests pass (100%).
- `go test -count=1 ./...`: 100% pass across all packages.
