---
title: 'Story 6.5: Full Four-Language Internationalization (i18n)'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: 'a41ea414199d57047e73d5b0ed10c4e347b8754a'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'web/src/lib/i18n.ts'
  - 'web/src/components/AppHeader.tsx'
  - 'web/src/app/page.tsx'
  - 'web/src/app/setup/page.tsx'
  - 'web/src/app/drop/page.tsx'
  - 'web/src/app/settings/page.tsx'
  - 'web/src/app/boot/page.tsx'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sovereign is designed as a household appliance for diverse international households across Europe and the Americas, primarily supporting English, German, Spanish, and French. If first-time users arrive and encounter language barriers, untranslated screens, lack of automatic browser locale detection, or missing native setup guides, the appliance fails its promise of consumer-grade turnkey simplicity.

**Approach:**
1. **Universal Browser Language Autodetection & Persistence (`web/src/lib/i18n.ts`)**:
   - Provide `detectBrowserLanguage` and `getInitialLanguage` helpers supporting `navigator.languages`, `navigator.language`, and `Accept-Language` headers.
   - Automatically map `"de-*"` &rarr; `"de"`, `"es-*"` &rarr; `"es"`, `"fr-*"` &rarr; `"fr"`, falling back to `"en"`.
   - Persist language changes to `localStorage` (`sovereign_lang`) and standard document cookies so all pages, tabs, and sessions maintain consistency across reloads.
2. **Instant Multi-Page Language Synchronization**:
   - Wire `AppHeader` language switcher across `/`, `/setup`, `/drop`, `/settings`, `/boot`, and `/guides` to update all UI copy instantly without full page reload.
   - Ensure Dropzone, Setup Wizard, Central Dashboard, Settings Hub, and Boot Screen all respect the persisted user language.
3. **Bundled Multilingual Guides & Help Center (`web/src/app/guides/page.tsx` & `docs/guides/`)**:
   - Provide comprehensive, user-friendly bundled guides available directly in the web UI and in repository documentation in English, German, Spanish, and French:
     - **Quickstart & Appliance Setup**: First boot, mDNS discovery (`http://sovereign.local`), and network verification.
     - **Storage & NAS Mounting**: Initializing internal drives, configuring SMB/NFS network shares, and capacity management.
     - **Media Ingestion & Dropzone**: Dropping video/audio files, SMB network folder copy (`\\sovereign\drop`), automated disc ripping.
     - **Private Local AI & Document Management**: Interacting with local models, Paperless OCR, document searching, and privacy guarantees.
     - **Resilience & Safe Rollback**: Power-loss recovery, automated 180s rollback safety, and one-click manual restores.
   - Include an accessible Guides viewer (`/guides` or Help modal) with 4-language toggle and clear navigation.
4. **Verification & Testing**:
   - Unit tests covering language detection edge cases, persistence, instantaneous header switching, and complete translation coverage across all 4 locales.
   - Verify 100% test pass rate across `go test ./...` and `npm --prefix web test`.

</frozen-after-approval>

## Acceptance Criteria

1. **Browser Language Autodetection**: On first visit without saved preferences, the application automatically detects the browser's language from `navigator.languages` / `navigator.language`, matching German (`de`), Spanish (`es`), French (`fr`), or falling back to English (`en`).
2. **Instant Language Switching**: Selecting a language from the `AppHeader` dropdown updates all UI text across the active screen immediately without a page refresh, and persists to `localStorage` and cookies.
3. **Consistent Multi-Page Language State**: All application pages (`/`, `/setup`, `/drop`, `/settings`, `/boot`, `/guides`) initialize from the persisted language preference.
4. **Bundled Multilingual Setup Guides**: Complete, bundled appliance documentation and setup guides are provided in the web interface and in `docs/guides/` in all four languages (EN, DE, ES, FR).
5. **No Missing Translations**: All translation dictionaries in `web/src/lib/i18n.ts` maintain complete 1:1 parity with zero missing keys or untranslated placeholders.
6. **Accessibility & WCAG AA Compliance**: Language toggles, guide navigation tabs, and content readers provide accessible `aria-label`, proper contrast, keyboard navigation, and minimum 44px touch targets.
7. **Automated Verification**: Complete automated test coverage verifying language autodetection, switching, guide rendering, and zero regressions across `go test ./...` and `npm --prefix web test`.

## Review Triage Log

### 1. Blind Hunter Review Findings
- **Finding**: Untracked guides, documentation, and test files omitted from partial diff.
  - *Triage*: Fixed; all files (`docs/guides/`, `web/src/app/guides/`, `web/src/lib/i18n.test.ts`) created and staged for git commit.
- **Finding**: `BootScreen` (`web/src/app/boot/page.tsx`) lacked localization and persisted language initialization.
  - *Triage*: Fixed; integrated `getInitialLanguage`, `translations[lang]`, and localized boot diagnostics microcopy across EN, DE, ES, and FR.
- **Finding**: Hardcoded English microcopy in Dropzone and Settings sections (GPU sharing schedule, maintenance window, destructive controls header).
  - *Triage*: Fixed; added dedicated translation keys across all four languages in `DashboardTranslations` and replaced hardcoded strings.
- **Finding**: `detectBrowserLanguage` using loose `startsWith` could match unrelated ISO language codes (e.g., `et` for Estonian or `el` for Greek matching `es`).
  - *Triage*: Fixed; updated subtag parsing to split on hyphens/underscores (`clean.split(/[-_]/)[0]`), matching only valid primary language tags.
- **Finding**: Regional language codes (e.g. `de-DE`, `es-MX`) rejected by `getInitialLanguage`.
  - *Triage*: Fixed; introduced `normalizeLanguageTag` helper that normalizes regional subtags to the primary supported language.
- **Finding**: `document.documentElement.lang` was not synchronized on language change, violating WCAG 3.1.1.
  - *Triage*: Fixed; `persistLanguage` updates `document.documentElement.lang = lang` in browser context.
- **Finding**: Language dropdown options used only 2-letter codes without accessible endonyms.
  - *Triage*: Fixed; updated `<select>` options to `English (EN)`, `Deutsch (DE)`, `Español (ES)`, `Français (FR)`.

### 2. Edge Case Hunter Review Findings
- **Finding**: `initialLanguage = "en"` default parameter in `HomeScreenContent` and `SettingsPage` caused `getInitialLanguage("en")` to be evaluated immediately, bypassing `localStorage`, cookies, and browser detection.
  - *Triage*: Fixed; removed default `initialLanguage = "en"` from both components, allowing `props.initialLanguage` to remain `undefined` when not passed so storage and browser autodetection cascades execute properly.
- **Finding**: Accept-Language header string support claimed in Intent was not implemented in `detectBrowserLanguage`.
  - *Triage*: Fixed; `detectBrowserLanguage` now accepts `readonly string[] | string` and parses RFC 2616 quality-weighted `Accept-Language` headers (e.g. `de-DE,de;q=0.9,en;q=0.8`).

### 3. Verification Gap Reviewer Findings
- **Finding**: Missing unit test coverage verifying `persistLanguage` storage and cookie writes.
  - *Triage*: Fixed; added unit test mocking `localStorage`, `document.cookie`, and `document.documentElement.lang`.
- **Finding**: Missing unit test coverage for `getInitialLanguage` resolution cascade (localStorage -> cookie -> navigator -> default en).
  - *Triage*: Fixed; added comprehensive priority cascade unit test in `web/src/lib/i18n.test.ts`.
- **Finding**: Missing unit test assertion for `AppHeader` `/guides` navigation link.
  - *Triage*: Fixed; added assertions in `web/src/app/page.test.tsx` verifying `/guides` URL and `data-testid="guides-nav-link"`.
- **Finding**: Missing unit test for `DropzonePage` localized rendering with non-English language props.
  - *Triage*: Fixed; added unit test in `web/src/app/drop/page.test.tsx` rendering German and French locales.
- **Finding**: Missing unit test for `BootScreen` localized rendering.
  - *Triage*: Fixed; added unit test in `web/src/app/boot/boot-screen.test.tsx` verifying German, Spanish, and French rendering.

