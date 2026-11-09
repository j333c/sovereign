---
title: 'Story 8.4: Multilingual Modal Token Validation & Hardware Calculator Baseline Fallback'
type: 'feature'
created: '2026-11-09'
status: 'done'
baseline_commit: '3ed4709cf26e60ac3d16b9e7207406e607e8e899'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/implementation-artifacts/epic-8-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:**
1. International administrators operating Sovereign in German, Spanish, or French encounter a broken user experience when performing factory reset operations in `DestructiveModal`: the modal strictly requires the exact English uppercase string `"RESET"`. Typing localized confirmation words (such as "LÖSCHEN", "ELIMINAR", "SUPPRIMER", or "DELETE") or lowercase/mixed-case input is rejected, leaving the confirmation button permanently disabled.
2. In the interactive Hardware Tier Calculator (both on `/hardware` and the `/landing` showcase), if a user unchecks all feature workloads, the recommendation box must not collapse into a broken or empty state; it must reliably present the foundational Light Tier baseline (Dual-core x86/ARM64, 4 GB RAM, 64 GB storage, 5W–10W idle power).
3. The turnkey hardware roadmap (`docs/hardware/ROADMAP.md`) and specification (`docs/hardware/TIERS.md`) reserve architectural hooks for physical appliances without explicitly articulating open-source warranty boundaries (Apache 2.0 §§ 7 & 8) versus physical hardware manufacturing partner warranties, leaving potential legal and support ambiguity.

**Approach:**
1. **Multilingual Confirmation Token Validation:**
   - Implement `isValidConfirmationToken(input: string, lang: Language): boolean` in `DestructiveModal.tsx`.
   - Support case-insensitive validation with trimmed whitespace across all four supported locales:
     - English (`en`): `"RESET"`, `"DELETE"`.
     - German (`de`): `"LÖSCHEN"`, `"ZURÜCKSETZEN"`, `"RESET"`, `"DELETE"`.
     - Spanish (`es`): `"ELIMINAR"`, `"RESTABLECER"`, `"RESET"`, `"DELETE"`.
     - French (`fr`): `"SUPPRIMER"`, `"RÉINITIALISER"`, `"REINITIALISER"`, `"RESET"`, `"DELETE"`.
   - Update input placeholder and accessible labels dynamically to display the primary localized confirmation token alongside universal `"RESET"`.
2. **Hardware Calculator Baseline Fallback:**
   - In `web/src/app/hardware/page.tsx` and `web/src/app/landing/page.tsx`, guarantee that `calculateHardwareTier` returns `"light"` when all features are unselected or false.
   - Surface an explicit baseline indicator badge (`data-testid="calc-baseline-notice"`) in the recommendation box when zero features are active, confirming the minimum foundational hardware requirements (Dual-core x86/ARM64, 4 GB RAM, 64 GB storage).
   - Add automated test assertions verifying the baseline state when all checkboxes are false.
3. **Hardware Warranty & Support Disclaimers:**
   - Add dedicated `Warranty & Support Disclaimers` sections to `docs/hardware/ROADMAP.md` and `docs/hardware/TIERS.md`.
   - Explicitly cite Apache 2.0 disclaimer of warranty and limitation of liability for self-hosted software, while delineating statutory hardware component warranties for turnkey manufacturing partners.
4. **Retrospective Action Item Resolution:**
   - Fulfill and close `epic-6-retro-item-4-i18n-destructive-modal-tokens`, `epic-7-retro-item-2-hardware-warranty-disclaimer`, and `epic-7-retro-item-3-hardware-calculator-baseline` in `sprint-status.yaml`.

## Boundaries & Constraints

**Always:**
- Trim whitespace and ignore casing when validating destructive confirmation tokens.
- Accept universal tokens `"RESET"` and `"DELETE"` across all locales, in addition to localized tokens (`"LÖSCHEN"`, `"ELIMINAR"`, `"SUPPRIMER"`).
- For French, accept both accented (`"réinitialiser"`) and unaccented (`"reinitialiser"`) inputs.
- Ensure the hardware calculator recommendation result box always renders a valid tier (defaulting to Light Tier) with CPU, RAM, storage, and idle power specs, even when zero feature checkboxes are selected.
- Maintain 100% key parity across `en`, `de`, `es`, and `fr` dictionaries in `web/src/lib/i18n.ts`.
- Ensure all interactive elements adhere to WCAG AA touch target (44x44px minimum) and keyboard focus guidelines.

**Never:**
- Never reject valid localized confirmation tokens ("LÖSCHEN", "ELIMINAR", "SUPPRIMER", "DELETE") or lowercase inputs in `DestructiveModal`.
- Never render an empty or broken recommendation card when all calculator checkboxes are deselected.
- Never hand-edit `web/src/lib/api/schema.d.ts`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| English Factory Reset | `lang: 'en'`, input: `'reset'` or `'DELETE'` | Confirm button enabled; Enter triggers `onConfirm` | Rejects arbitrary strings (e.g. `'confirm'`) |
| German Factory Reset | `lang: 'de'`, input: `'löschen'`, `'LÖSCHEN'`, or `'RESET'` | Confirm button enabled; Enter triggers `onConfirm` | Rejects non-tokens; allows leading/trailing whitespace |
| Spanish Factory Reset | `lang: 'es'`, input: `'eliminar'`, `'ELIMINAR'`, or `'restablecer'` | Confirm button enabled; Enter triggers `onConfirm` | Rejects non-tokens |
| French Factory Reset | `lang: 'fr'`, input: `'supprimer'`, `'réinitialiser'`, or `'reinitialiser'` | Confirm button enabled; Enter triggers `onConfirm` | Handles Unicode accents seamlessly |
| Hardware Calculator All Deselected | `{ dms: false, audio: false, video: false, disc: false, ai: false }` | Renders Light Tier baseline card with CPU, RAM, storage, power, and baseline badge | Never renders empty/undefined card |
| Hardware Calculator AI Selected | `{ ai: true, ... }` | Renders AI & Power Tier recommendation | Overrides lower tiers |
| Turnkey Roadmap Review | User reads `docs/hardware/ROADMAP.md` | Clear disclaimer regarding Apache 2.0 "AS IS" software vs physical hardware partner warranty | Clear legal boundary |

</frozen-after-approval>

## Code Map

- `web/src/components/DestructiveModal.tsx` -- Localized confirmation token validation helper `isValidConfirmationToken` and dynamic placeholder/prompt.
- `web/src/components/DestructiveModal.test.tsx` -- Multilingual token tests across EN, DE, ES, FR verifying case-insensitivity, trim, and localized words.
- `web/src/app/hardware/page.tsx` -- Baseline fallback and indicator when zero features are selected in hardware calculator.
- `web/src/app/hardware/page.test.tsx` -- Test coverage for zero-selection baseline state.
- `web/src/app/landing/page.tsx` -- Baseline fallback and indicator when zero features are selected in landing page calculator.
- `web/src/app/landing/page.test.tsx` -- Test coverage for landing page calculator baseline fallback.
- `docs/hardware/ROADMAP.md` -- Open-source warranty and turnkey hardware disclaimer section.
- `docs/hardware/TIERS.md` -- Hardware warranty boundaries and Apache 2.0 cross-reference.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- Sprint status and retrospective action item updates.

## Verification Plan

### Automated Tests
1. `npm --prefix web test -- --run`
   - Verify `DestructiveModal.test.tsx` passes with all localized token cases.
   - Verify `HardwarePage.test.tsx` passes with zero-feature baseline fallback.
   - Verify `LandingPage.test.tsx` passes with zero-feature baseline fallback.
   - Verify `i18n.test.ts` confirms key parity.
2. `go test -count=1 ./...`
   - Verify all Go packages continue to pass 100%.

### Manual Probes
1. Verify `DestructiveModal` with German (`de`) accepts `"löschen"` and `"RESET"`.
2. Verify `DestructiveModal` with Spanish (`es`) accepts `"eliminar"` and `"RESET"`.
3. Verify `DestructiveModal` with French (`fr`) accepts `"supprimer"` and `"reinitialiser"`.
4. Verify `/hardware` and `/landing` render Light Tier baseline with all checkboxes unselected.
