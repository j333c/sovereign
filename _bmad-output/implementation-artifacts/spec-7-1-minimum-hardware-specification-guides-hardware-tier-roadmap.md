---
title: 'Story 7.1: Minimum Hardware Specification Guides & Hardware Tier Roadmap'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: '5f97c95e3f6388cbfddf6d0270edf26209f374df'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'web/src/app/guides/page.tsx'
  - 'web/src/app/settings/page.tsx'
  - 'web/src/lib/i18n.ts'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Enthusiasts and families building or repurposing hardware for Sovereign need precise, clear hardware specifications tailored to their specific workload (e.g. lightweight document archiving vs. 4K transcode vs. local 7B LLM inference). Without categorized guidance and calculators, users either overspend on oversized server racks or encounter resource starvation. Additionally, prospective consumers who prefer turnkey, pre-configured appliances require a transparent hardware roadmap and architectural hooks for pre-assembled hardware ordering.

**Approach:**
1. **Repository Hardware Tier Documentation (`docs/hardware/`)**:
   - Provide comprehensive, practical hardware specifications:
     - **Light Tier** (DMS + Audio): Dual-core x86/ARM64 (Intel N100, Raspberry Pi 5 / RK3588, Celeron), 4 GB RAM, 64 GB storage. Low-power (6–10W idle), silent, fanless.
     - **Media Tier** (Video + DMS + Audio + Optical Disc Ingestion): Quad-core x86 with Intel QuickSync / iGPU (Intel N100/N305, 12th+ Gen Core i3/i5), 8 GB RAM, 2 TB+ storage pool, USB/SATA optical drive support.
     - **AI & Power Tier** (Media + DMS + Local 7B/8B LLM RAG): 6+ core x86, 16–32 GB RAM, discrete GPU (NVIDIA RTX 3060/4060 with 8–16 GB VRAM) or unified memory APU (AMD 7840HS/8840U, Apple Silicon), 4 TB+ storage pool.
   - Detail power consumption targets, noise ratings, backup power / UPS guidelines, and disk redundancy recommendations.
2. **Interactive Hardware Calculator & Specification Viewer (`web/src/app/hardware/page.tsx`)**:
   - User-friendly interactive calculator: select intended household features (Streaming, Music, OCR, AI Assistant, Disc Ripping) and receive immediate hardware tier recommendation, CPU/RAM/VRAM targets, and estimated idle wattage.
   - Architectural hooks for turnkey pre-assembled Sovereign home server hardware (e.g., pre-order waitlist modal, hardware reservation token, and specs overview).
   - Fully localized in English, German, Spanish, and French.
3. **Integration with Guides & Settings**:
   - Link to hardware tier guides from `/guides` and `/settings` System & Hardware tab.
4. **Automated Verification**:
   - Unit tests covering hardware tier calculation logic, interactive tier selection, localized strings, and accessibility.
   - 100% test pass rate across `go test ./...` and `npm --prefix web test`.

</frozen-after-approval>

## Acceptance Criteria

1. **Three Distinct Hardware Profiles**: Hardware documentation and interactive interfaces explicitly define:
   - *Light Tier* (DMS + Audio): Dual-core x86/ARM64, 4 GB RAM, 64 GB storage.
   - *Media Tier* (Video + DMS + Audio + Disc Ingestion): Quad-core x86 with Intel QuickSync / iGPU, 8 GB RAM, 2 TB+ storage.
   - *AI & Power Tier* (Media + DMS + Local 7B/8B LLM RAG): 6+ core x86, 16–32 GB RAM, discrete GPU (8 GB+ VRAM) or unified memory APU, 4 TB+ storage.
2. **Interactive Hardware Tier Calculator**: An accessible web interface allows users to check feature checkboxes and dynamically see recommended hardware tier, minimum RAM, CPU class, storage recommendation, and estimated power draw.
3. **Turnkey Hardware Appliance Roadmap & Architectural Hooks**: The interface provides a turnkey appliance roadmap and architectural preorder/reservation hook for factory pre-assembled Sovereign hardware.
4. **Multilingual Support & Parity**: Hardware tiers and calculator copy are fully localized across English, German, Spanish, and French.
5. **Accessibility & WCAG AA Compliance**: Keyboard-navigable tabs, accessible `role="radiogroup"` / `role="region"`, clear focus indicators, and 44px minimum touch targets.
6. **Automated Verification**: Full test suite coverage in `web/src/app/hardware/page.test.tsx` verifying tier recommendations, component rendering, localized microcopy, and zero regressions across existing test suites.

## Review Triage Log

### Review Loop 1 Findings & Remediation

1. **Edge Case Hunter**:
   - `calculateHardwareTier`: Guard against undefined/null arguments (`if (!features) return "light";`). Remediated in `web/src/app/hardware/page.tsx` and verified with unit tests.
   - Reservation token formatting: Pad random base-36 string with `padEnd(6, '0')` to guarantee 6 characters. Remediated in `web/src/app/hardware/page.tsx`.
   - Inbound navigation links: Specification approach called for discoverable links to `/hardware` from `/guides` and `/settings` (System & Hardware tab). Remediated by adding cross-links in `web/src/app/guides/page.tsx` and `web/src/app/settings/page.tsx`.

2. **Blind Hunter**:
   - Calculator result box: Detailed hardware targets (CPU, RAM, Storage) added inside `data-testid="calc-result"`. Remediated in `web/src/app/hardware/page.tsx`.
   - Pre-order reservation dialog: Display selected appliance model edition name (`Sovereign Mini`, `Sovereign Home`, `Sovereign Pro Studio`) in modal header and confirmation notice. Remediated.
   - Token clipboard copy button: Added accessible "Copy Token" button to modal confirmation screen. Remediated.
   - Modal focus trapping: Added Tab / Shift+Tab focus trap cycling within dialog and Escape key listener. Remediated.
   - Accessible section landmarks: Added `role="region"` across all major sections in `web/src/app/hardware/page.tsx`. Remediated.
   - AppHeader icon alignment: Adjusted `mr-1.5` to `lg:mr-1.5` to ensure centered alignment when text is hidden on smaller viewports. Remediated.
   - Documentation clarifications: Added UPS/battery guidance and disk redundancy / RAID topology details for Light and Media tiers, optical drive LibreDrive firmware and power recommendations in `docs/hardware/TIERS.md`, and harmonized idle power draw across documentation. Remediated.

3. **Verification Gap Reviewer**:
   - AppHeader `/hardware` navigation link regression test: Added assertions to `web/src/app/page.test.tsx`.
   - Pre-order reservation modal test coverage: Added tests verifying modal rendering when open, model badge display, and reservation confirmation with token copy button in `web/src/app/hardware/page.test.tsx`.
   - Inbound link test coverage: Added assertions to `web/src/app/guides/page.test.tsx`.

