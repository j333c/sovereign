---
title: 'Story 7.2: Public Marketing Landing Website & Feature Showcase'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: 'ef4e47c7e3e14b519ccc9eb0ae69ff46e53c1931'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'web/src/app/hardware/page.tsx'
  - 'web/src/app/guides/page.tsx'
  - 'web/src/lib/i18n.ts'
  - 'web/src/components/AppHeader.tsx'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Prospective users, self-hosting enthusiasts, and privacy-focused households discovering Sovereign need a polished, informative, and accessible public marketing landing website and feature showcase. The page must clearly communicate what Sovereign is, showcase the full suite of integrated spoke services (Jellyfin 4K transcoding, Navidrome & Audiobookshelf, Paperless-ngx OCR archival, local private AI with instant document RAG, and ARM optical disc ingestion), highlight the strict zero-telemetry architectural invariants, feature an interactive hardware tier selector, and provide direct links to the open-source repository and installation documentation.

**Approach:**
1. **Public Marketing Showcase Page (`web/src/app/landing/page.tsx`)**:
   - **Hero Section**: Value proposition (*"Your Personal Digital Sanctuary: 100% Private, Zero Cloud Telemetry"*), live status badge, quick links to GitHub repository, hardware guidance, and live appliance dashboard.
   - **Core Feature Showcase (Interactive & Tabbed)**:
     - *4K Video Streaming & Transcoding*: Jellyfin with Intel QuickSync tone mapping and hardware acceleration.
     - *Lossless Music & Audiobooks*: Navidrome FLAC/MP3 streaming and Audiobookshelf multi-user chapter sync.
     - *Intelligent Document Management*: Paperless-ngx OCR text indexing, multi-channel IMAP polling, and automated deadline detection.
     - *Private Local AI Assistant & RAG*: llama.cpp 7B/8B inference, isolated bridge network without internet routing, zero-vector FTS5 document search, and household personas.
     - *Automatic Optical Disc Ingestion*: Automatic CD/DVD/Blu-ray rip, metadata query, and auto-ejection.
   - **Privacy & Architectural Invariants Section**:
     - Strict zero-telemetry boundary (air-gapped internal Docker network, no analytics or cloud pings).
     - High durability CGO-free Go SQLite WAL mode storage.
     - Pre-upgrade snapshots and automated 180s rollback safety.
     - Zero Docker socket mounts in spoke containers.
   - **Interactive Hardware Tier Calculator**:
     - Direct tier calculator integration recommending Light Tier (5–10W), Media Tier (10–25W), or AI & Power Tier (25–45W) based on chosen workloads.
   - **Open-Source Call to Action & Navigation**:
     - Links to GitHub repository, documentation guides (`/guides`), hardware tiers (`/hardware`), and appliance dashboard (`/`).
   - **Full Internationalization (i18n)**:
     - 100% key parity across English, German (DE), Spanish (ES), and French (FR) in `web/src/lib/i18n.ts`.
   - **WCAG AA Accessibility**:
     - Accessible landmarks (`role="region"`), tab panels (`role="tablist"` / `role="tab"` / `role="tabpanel"`), 44px minimum touch targets, and visible focus rings.

2. **Navigation & Inbound Links**:
   - Add link to Landing Showcase from `AppHeader.tsx`, `/guides`, and `/hardware`.

3. **Automated Verification**:
   - Unit tests in `web/src/app/landing/page.test.tsx` verifying hero rendering, feature tabs, interactive hardware tier recommendation, localized microcopy across all 4 languages, and WCAG AA accessibility compliance.
   - 100% test pass rate across `go test ./...` and `npm --prefix web test`.

</frozen-after-approval>

## Acceptance Criteria

1. **Feature Showcase**: Highlights core appliance capabilities:
   - 4K video transcoding with Intel QuickSync (Jellyfin).
   - Lossless music and audiobooks (Navidrome & Audiobookshelf).
   - Document archival with OCR and deadline detection (Paperless-ngx).
   - Private local AI chat with instant document RAG (llama.cpp 7B/8B).
   - Automated optical disc ingestion (ARM).
2. **Zero-Telemetry Privacy Invariants**: Explicitly showcases strict air-gapped network boundaries, zero Docker socket mounts in spokes, and database durability.
3. **Integrated Hardware Calculator**: Includes an interactive tier picker/calculator providing CPU, RAM, storage, and idle power draw estimates.
4. **Project & Documentation Links**: Provides clear links to GitHub repository, bundled guides, hardware roadmap, and appliance dashboard.
5. **Multilingual Parity**: 100% translation key parity across English (EN), German (DE), Spanish (ES), and French (FR).
6. **WCAG AA Compliance**: 44px minimum touch targets, visible keyboard focus indicators, accessible tablist/tab navigation, and semantic landmarks.
7. **Automated Verification**: Full test suite coverage in `web/src/app/landing/page.test.tsx` with zero regressions across existing test suites.

## Review Triage Log

| Finding | Reviewer | Action Taken | Rationale |
|---|---|---|---|
| Inbound navigation links to `/landing` missing in `/guides` and `/hardware` | Blind Hunter | Patched | Added responsive showcase quick-links in sidebar of `GuidesPage` and top header area of `HardwarePage`. |
| Unused `landingTitle` translation key & missing document title | Blind Hunter | Patched | Added `useEffect` in `LandingPage` setting `document.title = t.landingTitle`. |
| Hardcoded English text in trust badge, stats, badges, and footer | Blind Hunter | Patched | Added 17 translation keys to `DashboardTranslations` across EN, DE, ES, and FR with 100% parity (`landingHeroBadge`, `landingStatPings`, `landingStatStorage`, etc.). |
| Broken ARIA `aria-controls` references on unselected tabs | Blind Hunter | Patched | Rendered all 5 tabpanels in DOM with `hidden={activeTab !== tab.id}` and `id={`panel-${tab.id}`}` so every `aria-controls` target always exists. |
| Incomplete WAI-ARIA tab keyboard navigation | Blind Hunter | Patched | Added roving `tabIndex={isSelected ? 0 : -1}` to tab buttons, `onKeyDown` handlers for ArrowLeft/Right/Home/End, and `tabIndex={0}` on active tabpanel. |
| WCAG AA color contrast on Section 5 subtitle | Blind Hunter | Patched | Changed text color to `text-[#1D4ED8]` (blue-700) over `bg-[#EFF6FF]`, guaranteeing >4.5:1 contrast. |
| Feature panel guide links lack topic anchor deep-links | Blind Hunter | Patched | Updated guide links to point to `/guides#media`, `/guides#storage`, `/guides#ai` rather than root `/guides`. |
| Responsive icon margin misalignment in `AppHeader.tsx` | Blind Hunter | Patched | Changed `lg:mr-1.5` to `xl:mr-1.5` to match `hidden xl:inline` text visibility. |
| Hardware calculator spec rendering & non-default tier branches unverified | Verification Gap Reviewer | Patched | Added `initialFeatures?: HardwareCalculatorState` prop to `LandingPageProps` and comprehensive tests in `web/src/app/landing/page.test.tsx` verifying Light, Media, and AI tier outputs with exact spec assertions. |
| Missing touch target sizing & focus ring styling on footer links | Blind Hunter | Patched | Added `min-h-[44px]` and `focus-visible:ring-2` to all footer links and in-panel guide links. |

