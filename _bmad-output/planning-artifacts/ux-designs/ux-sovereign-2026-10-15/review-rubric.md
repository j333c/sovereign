# Spine Pair Review — Sovereign

## Overall verdict
The Sovereign UX spine pair (`DESIGN.md` and `EXPERIENCE.md`) establishes a clean, cohesive, appliance-grade design system with strong aesthetic posture, complete token resolution, and rigorous accessibility compliance. Pass 1 mechanical checks confirm that all 5 user journeys are detailed with named protagonists, climaxes, and failure paths, all YAML tokens resolve cleanly, and visual references are accounted for. Pass 2 identifies key downstream contracts requiring refinement: non-verbatim UJ titles that hinder automated story-dev traceability (High), 4 components missing visual spec breakdowns in `DESIGN.md` body (Medium), ambiguous Yellow vs Amber status naming divergence (Medium), and missing cold-load/offline states (Medium).

## 1. Flow coverage — strong
Checked all 5 Key User Journeys (UJ-1 through UJ-5) and 20 Functional Requirements (FR-1 through FR-20) from `prd.md` against `EXPERIENCE.md.Key Flows`. Verified that each flow defines a named protagonist, numbered chronological steps, an explicit bold Climax beat, and a realistic Failure Path.
### Findings
*(No misses. Note: Title naming discrepancies are logged under section 7 Inheritance discipline.)*

## 2. Token completeness — strong
Extracted all 29 color tokens, 7 typography ramps, 6 border radius tokens, 8 spacing levels, and 9 component mappings from `DESIGN.md` YAML frontmatter. Verified that every token has valid types, colors use valid 6-digit hex codes, and 100% of `{path.to.token}` references across both `DESIGN.md` and `EXPERIENCE.md` prose resolve without error. Verified WCAG AAA (14.2:1) and AA (4.6:1) contrast targets.
### Findings
- **low** Literal hex codes `#CBD5E1` and `#0F172A` hardcoded in Dropzone target prose (`DESIGN.md` § Components.DropzoneCard line 247) instead of referencing `{colors.border-active}` and `{colors.action-primary}`. *Fix:* Replace raw hex strings with standard token references `{colors.border-active}` and `{colors.action-primary}`.

## 3. Component coverage — adequate
Extracted all 9 components referenced across both documents: `AppHeader`, `ServiceCard`, `StatusBadge`, `OverflowMenu`, `TelemetryBar`, `DropzoneCard`, `SettingsNavigation`, `RollbackModal`, `BootProgressBar`. All 9 are defined in `DESIGN.md` frontmatter and have behavioral specs in `EXPERIENCE.md.Component Patterns`.
### Findings
- **medium** 4 of 9 components (`OverflowMenu`, `SettingsNavigation`, `RollbackModal`, `BootProgressBar`) are declared in `DESIGN.md` YAML frontmatter and specified in `EXPERIENCE.md.Component Patterns`, but lack dedicated visual specification H3 subsections under `DESIGN.md ## Components` (`DESIGN.md` § Components lines 217–250). *Fix:* Add dedicated visual specification blocks (anatomy, dimensions, borders, and interaction states) for the 4 missing components in `DESIGN.md`.

## 4. State coverage — adequate
Walked all 5 Information Architecture surfaces: Setup Wizard (`/setup`), Central Dashboard (`/`), Central Settings (`/settings`), Media Dropzone (`/drop`), and Boot Diagnostics (`/boot`). Evaluated coverage of empty, loading, operational, degraded, error, and recovery states.
### Findings
- **medium** Central Admin Dashboard lacks cold-load / initial loading skeleton state specification and offline/disconnected SSE reconnect behavior (`EXPERIENCE.md` § Information Architecture line 37 & State Patterns line 134). Downstream frontend implementers lack clear direction on how cards render before SSE data arrives or if network connectivity to the daemon drops. *Fix:* Document a skeleton loader pattern (matching `ServiceCard` dimensions) and a reconnecting toast/banner for lost SSE connection.
- **low** Boot Diagnostics Screen (`/boot`) lacks an unrecoverable failure state specification for scenarios where journal check fails or disk filesystem cannot be recovered automatically (`EXPERIENCE.md` § Information Architecture line 40 & State Patterns line 142). *Fix:* Add an unrecoverable error state with consumer-friendly guidance (e.g. "Hardware error detected. Please contact support or boot rescue USB").

## 5. Visual reference coverage — adequate
Listed all files in `mockups/`: `dashboard.html`, `dropzone.html`, `settings.html` (with `imports/` empty and `wireframes/` absent). Checked links and the "Spines win on conflict" clause in both spine files.
### Findings
- **low** Mockups are linked only in the top header blockquote (`DESIGN.md` line 141, `EXPERIENCE.md` line 14) and are not contextualized inline at their respective surface or component subsections (`EXPERIENCE.md` § Information Architecture / Component Patterns, `DESIGN.md` § Components). *Fix:* Place inline links (`[Mockup: mockups/dashboard.html]`, `[Mockup: mockups/dropzone.html]`, `[Mockup: mockups/settings.html]`) directly inside their corresponding IA table rows and component headings.

## 6. Bloat & overspecification — strong
Evaluated visual specifications, source restatements, tabular vs prose formatting, and editorial voice boundaries. `DESIGN.md` maintains an appropriate high-level editorial tone while `EXPERIENCE.md` remains dry, factual, and behavioral.
### Findings
- **low** Sizing discrepancies: Touch target floor is defined as minimum 44px × 44px in `EXPERIENCE.md` line 148 but as 48px in line 235; StatusBadge vertical padding is specified as `{spacing.1}` (4px) in `DESIGN.md` frontmatter line 104 but as `3px top/bottom` in prose line 234. *Fix:* Align touch target minimums to 44px across all sections, and update StatusBadge prose to 4px (`{spacing.1}`).

## 7. Inheritance discipline — adequate
Verified `sources` frontmatter resolution against filesystem paths (`prd.md` and `addendum.md`). Checked cross-file token references, component naming, and domain terminology alignment with sources.
### Findings
- **high** All 5 User Journey titles in `EXPERIENCE.md` were rephrased instead of using the verbatim names from `prd.md` (`EXPERIENCE.md` § Key Flows lines 169, 183, 195, 207, 218). For example, `UJ-1: Alex sets up Sovereign and launches Video Streaming` became `UJ-1: Setup & Media Launch`. Downstream story slicing and acceptance tracking require exact verbatim matching. *Fix:* Restore verbatim UJ titles from `prd.md` in `EXPERIENCE.md` (e.g. `### UJ-1: Alex sets up Sovereign and launches Video Streaming`).
- **medium** Inconsistent status terminology: `EXPERIENCE.md` alternates between "Amber" (matching `DESIGN.md.colors.status-amber`) and "Yellow" (from `prd.md` Traffic-Light Indicator) across lines 37, 69, 88, 193, and 227. *Fix:* Standardize on "Amber" across all spine copy, explicitly noting in the IA table that "Amber" implements the PRD "Yellow" degraded tier.

## 8. Shape fit — strong
Checked `DESIGN.md` canonical section sequence: Brand & Style → Colors → Typography → Layout & Spacing → Elevation & Depth → Shapes → Components → Do's and Don'ts. Checked `EXPERIENCE.md` for all 8 required default sections plus the triggered required-when-applicable section `Responsive & Platform`.
### Findings
*(No misses. Both documents strictly maintain their canonical section sequences.)*

## Mechanical notes
- **Frontmatter completeness**: `DESIGN.md` and `EXPERIENCE.md` frontmatter blocks are valid YAML, properly delimited, and contain all required metadata keys (`name`, `status`, `sources`, tokens).
- **Cross-reference resolution**: All 18 `{colors.*}`, 8 `{spacing.*}`, 6 `{rounded.*}`, 7 `{typography.*}`, and 3 `{components.*}` token references across both documents resolve cleanly.
- **Name consistency**: Component names are 100% consistent PascalCase across frontmatter and headings (`AppHeader`, `ServiceCard`, `StatusBadge`, `OverflowMenu`, `TelemetryBar`, `DropzoneCard`, `SettingsNavigation`, `RollbackModal`, `BootProgressBar`). Status indicator naming should be standardized to "Amber" throughout.
- **Mermaid syntax**: No Mermaid diagrams are present in the spine pair; zero syntax errors detected.
