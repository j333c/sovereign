---
name: Sovereign
description: Appliance-grade home server platform. Clean Nordic/Braun minimalist aesthetic (Direction A) inheriting shadcn/ui and Tailwind CSS conventions.
status: final
created: 2026-10-15
updated: 2026-10-15
colors:
  canvas: '#F8F9FA'
  surface: '#FFFFFF'
  border: '#E5E7EB'
  border-subtle: '#F1F5F9'
  border-active: '#CBD5E1'
  text-primary: '#0F172A'
  text-muted: '#64748B'
  text-faint: '#475569'
  action-primary: '#0F172A'
  action-primary-hover: '#1E293B'
  action-primary-fg: '#FFFFFF'
  action-secondary: '#F8F9FA'
  action-secondary-hover: '#F1F5F9'
  action-secondary-fg: '#0F172A'
  status-green: '#10B981'
  status-green-bg: '#ECFDF5'
  status-green-fg: '#065F46'
  status-green-border: '#A7F3D0'
  status-amber: '#F59E0B'
  status-amber-bg: '#FFFBEB'
  status-amber-fg: '#92400E'
  status-amber-border: '#FDE68A'
  status-red: '#EF4444'
  status-red-bg: '#FEF2F2'
  status-red-fg: '#991B1B'
  status-red-border: '#FECACA'
  accent-blue: '#2563EB'
  focus-ring: '#2563EB'
  timeline-day: '#3B82F6'
  timeline-night: '#0F172A'
typography:
  display:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '24px'
    fontWeight: '600'
    lineHeight: '1.2'
    letterSpacing: '-0.02em'
  heading:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '18px'
    fontWeight: '600'
    lineHeight: '1.3'
    letterSpacing: '-0.01em'
  subheading:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '15px'
    fontWeight: '600'
    lineHeight: '1.4'
    letterSpacing: '-0.01em'
  body:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '14px'
    fontWeight: '400'
    lineHeight: '1.5'
  label:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '13px'
    fontWeight: '500'
    lineHeight: '1.4'
  small:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'
    fontSize: '12px'
    fontWeight: '500'
    lineHeight: '1.3'
  mono:
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace'
    fontSize: '12px'
    fontWeight: '400'
    lineHeight: '1.4'
rounded:
  sm: '4px'
  md: '6px'
  lg: '8px'
  xl: '12px'
  full: '9999px'
  DEFAULT: '8px'
spacing:
  '1': '4px'
  '2': '8px'
  '3': '12px'
  '4': '16px'
  '5': '20px'
  '6': '24px'
  '8': '32px'
  '12': '48px'
components:
  AppHeader:
    height: '64px'
    background: '{colors.surface}'
    borderBottom: '1px solid {colors.border}'
    paddingX: '{spacing.8}'
  ServiceCard:
    background: '{colors.surface}'
    border: '1px solid {colors.border}'
    borderRadius: '{rounded.lg}'
    padding: '{spacing.5}'
  StatusBadge:
    paddingY: '{spacing.1}'
    paddingX: '{spacing.2}'
    borderRadius: '{rounded.full}'
    fontSize: '{typography.small.fontSize}'
  OverflowMenu:
    width: '44px'
    height: '44px'
    borderRadius: '{rounded.md}'
  TelemetryBar:
    height: '56px'
    background: '{colors.surface}'
    borderTop: '1px solid {colors.border}'
    paddingX: '{spacing.8}'
  DropzoneCard:
    background: '{colors.surface}'
    border: '1px solid {colors.border}'
    borderRadius: '{rounded.xl}'
    padding: '{spacing.8}'
  SettingsNavigation:
    width: '240px'
    borderRadius: '{rounded.lg}'
    gap: '{spacing.1}'
  AIBackendCard:
    background: '{colors.surface}'
    border: '1px solid {colors.border}'
    borderRadius: '{rounded.lg}'
    padding: '{spacing.6}'
  RollbackModal:
    background: '{colors.surface}'
    borderRadius: '{rounded.xl}'
    border: '1px solid {colors.border}'
    maxWidth: '480px'
  BootProgressBar:
    height: '6px'
    background: '{colors.border-subtle}'
    fill: '{colors.status-green}'
    borderRadius: '{rounded.full}'
---

# Sovereign — Visual Design Spine

> Visual identity specification for the Sovereign Home Server appliance layer. Based on the Google Labs `design.md` spec. Paired with `EXPERIENCE.md`. Spines win on conflict with all mockups and prototypes.
> Visual references: [Dashboard Mockup](mockups/dashboard.html) · [Standalone Dropzone Mockup](mockups/dropzone.html) · [Settings Mockup](mockups/settings.html).

---

## Brand & Style

Sovereign embodies **quiet appliance confidence**. It is designed to feel like high-end consumer audio or refined domestic electronics (Braun, Dieter Rams, Bang & Olufsen)—never a dense Linux sysadmin dashboard or cluttered cloud infrastructure portal.

The design prioritizes clarity, spaciousness, and zero cognitive load. Data typography is crisp and legible; borders are hairline separators; status indicators are reassuring rather than alarming. The system communicates that home data is safe, private, and running smoothly without demanding constant user vigilance.

---

## Colors

The palette is rooted in clean Nordic light neutrality with decisive contrast and semantic traffic-light indicators:

- `{colors.canvas}` (`#F8F9FA`): The foundation background. A subtle, cool off-white that minimizes glare on 1080p+ screens and softens contrast compared to stark white.
- `{colors.surface}` (`#FFFFFF`): The card and panel elevation layer. Used for service cards, header, and settings panes to establish crisp hierarchy over the canvas.
- `{colors.border}` (`#E5E7EB`): Hairline structural division. Separates header, cards, and bottom telemetry bars without heavy drop-shadows.
- `{colors.text-primary}` (`#0F172A`): Deep slate obsidian for primary headings, service names, and active states. Delivers WCAG AAA contrast (14.2:1) against surface.
- `{colors.text-muted}` (`#64748B`): Secondary metrics, file metadata, and descriptive copy. Maintains WCAG AA compliance (4.6:1).
- `{colors.text-faint}` (`#475569`): Subtle secondary labels and tech descriptions, calibrated to 7.0:1 contrast for full WCAG AAA/AA compliance.
- `{colors.action-primary}` (`#0F172A`): Confident, tactile action buttons (such as "Open Service UI") with stark white text `{colors.action-primary-fg}`.
- `{colors.status-green}` (`#10B981`) & `{colors.status-green-bg}` (`#ECFDF5`): Healthy running state. Reassuring green dot (●) and soft background container with `{colors.status-green-fg}` text.
- `{colors.status-amber}` (`#F59E0B`) & `{colors.status-amber-bg}` (`#FFFBEB`): Attention required state (◆). The single canonical standard for warning/attention (replacing ambiguous "yellow").
- `{colors.status-red}` (`#EF4444`) & `{colors.status-red-bg}` (`#FEF2F2`): Service stopped or error state (▲) requiring action.
- `{colors.accent-blue}` (`#2563EB`) & `{colors.focus-ring}`: Text links, interactive paths, and high-visibility keyboard focus rings.

---

## Typography

Sovereign utilizes modern system sans-serif font stacks (`-apple-system`, `BlinkMacSystemFont`, `Segoe UI`, `Roboto`) to ensure zero network latency, immediate first render, and natural alignment with the user's host OS:

- **Display (`{typography.display}`)**: 24px/600. Reserved for page titles on standalone surfaces (Dropzone, First-Run Setup).
- **Heading (`{typography.heading}`)**: 18px/600. Section titles and appliance modal headers.
- **Subheading (`{typography.subheading}`)**: 15px/600. Service card titles and major telemetry headings.
- **Body (`{typography.body}`)**: 14px/400. Explanatory text, descriptions, and settings options.
- **Label (`{typography.label}`)**: 13px/500. Interactive buttons, table headers, and form labels.
- **Small (`{typography.small}`)**: 12px/500. Status pill labels, version numbers, and timestamps.
- **Mono (`{typography.mono}`)**: 12px/400. Network paths (`\\sovereign\drop`) and diagnostic digests.

---

## Layout & Spacing

### 1. Viewport Hierarchy & Breakpoints
Layout adheres to three standardized responsive tiers:
- **Desktop (Width $\ge$ 1024px)**: Default 4-column service grid. On 1080p through 4K screens with standard 4 services, the Central Admin Dashboard enforces a **single-viewport height (`100vh; overflow: hidden`)** with container `max-width: 1440px`. Header and Telemetry bar remain strictly pinned (`flex-shrink: 0`). When 5+ services are installed or when vertical height is constricted ($< 700\text{px}$), the service grid container gracefully unlocks internal scrolling (`overflow-y: auto`) while header and telemetry remain pinned.
- **Tablet (Width 768px – 1023px)**: 2-column service card grid (`repeat(2, 1fr)`) with fluid vertical scrolling.
- **Mobile (Width $< 768\text{px}$)**: Single-column layout (`repeat(1, 1fr)`) with fluid vertical flow and expanded touch targets ($44\text{px}+$ height).

### 2. Spacing Scale
Layouts adhere to an 8pt spatial grid:
- Card inner padding: `{spacing.5}` (20px)
- Gap between grid cards: `{spacing.4}` (16px)
- Outer viewport margin: `{spacing.6}` (24px) to `{spacing.8}` (32px)

---

## Elevation & Depth

Sovereign avoids deep or fuzzy shadows. Visual hierarchy is established through **borders and tonal layers**:
- **Default Elevation**: Cards sit flush on `{colors.canvas}` with a 1px border (`{colors.border}`).
- **Card Hover Elevation**: Soft ambient shadow `0 4px 12px rgba(15, 23, 42, 0.04)` and border shift to `{colors.border-active}`.
- **Modals / Dropdowns**: Ambient depth `0 10px 25px -5px rgba(15, 23, 42, 0.08)`, border `{colors.border}`.
- **Keyboard Focus Ring**: 2px solid `{colors.focus-ring}` with 2px offset (`outline: 2px solid #2563EB; outline-offset: 2px`).

---

## Shapes

- **Base Radius (`{rounded.DEFAULT}`)**: 8px (`rounded-lg`). Applied to service cards, action buttons, input fields, and dialogs for a clean, precision-engineered feel.
- **Pill Radius (`{rounded.full}`)**: 9999px. Used exclusively for status badges and schedule toggle pills.
- **Large Radius (`{rounded.xl}`)**: 12px. Applied to standalone dropzone containers and rollback modals.

---

## Components

### AppHeader
- Height: `{components.AppHeader.height}` (64px)
- Background: `{colors.surface}`
- Border Bottom: 1px solid `{colors.border}`
- Anatomy: Brand crest (32x32px `{colors.action-primary}` with white "S"), title text, "All Systems Operational" health pill, right-aligned utility actions (Dropzone shortcut, Settings gear, Language switcher).

### ServiceCard
- Background: `{colors.surface}`
- Border: 1px solid `{colors.border}`, Radius: `{rounded.lg}`
- Anatomy:
  1. Header: Service icon (40x40px, `{colors.canvas}` fill) + Service Title + Tech subtitle + `•••` overflow menu button.
  2. Status & Metric: Status pill (`{components.StatusBadge}`) + large metric (e.g. "124 Movies") + secondary descriptor.
  3. Action: Full-width button (`{colors.action-primary}` fill, white text, 38px desktop height, 44px mobile height) linking to the installed application in a new tab.

### StatusBadge
- Inline-flex pill: 3px top/bottom, 8px left/right. Radius: `{rounded.full}`.
- Color Independence: Pairs semantic color with distinct geometric micro-shapes:
  - Green state: `{colors.status-green-bg}` with `{colors.status-green-fg}` text and circle (●) icon.
  - Amber state: `{colors.status-amber-bg}` with `{colors.status-amber-fg}` text and diamond (◆) icon.
  - Red state: `{colors.status-red-bg}` with `{colors.status-red-fg}` text and triangle (▲) icon.

### OverflowMenu
- Width / Height: `{components.OverflowMenu.width}` x `{components.OverflowMenu.height}` (44x44px touch target area with 28x28px visual button).
- Radius: `{rounded.md}`.
- Background: Transparent with hover `{colors.border-subtle}`.
- Behavior: Opens an accessible popover containing Restart, Rollback, and Logs actions. Focus trapped on open; closes on Escape.

### TelemetryBar
- Height: `{components.TelemetryBar.height}` (56px)
- Background: `{colors.surface}`, Border Top: 1px solid `{colors.border}`
- Items: Storage progress bar (64px width, 6px height, `{colors.status-green}` fill), NAS connection text, Optical Drive idle badge, Day/Night GPU schedule badge.

### DropzoneCard
- Max-width: 680px, centered on `{colors.canvas}`.
- Target: 2px dashed border (`{colors.border-active}`), padding `{spacing.12}`, hover transition to `{colors.action-primary}` border.
- Category Tabs: Pill group with active tab white background and subtle shadow.

### SettingsNavigation
- Width: `{components.SettingsNavigation.width}` (240px).
- Radius: `{rounded.lg}`.
- Active item: Background `{colors.surface}`, border 1px solid `{colors.border}`, bold text. Hover item: Background `{colors.border-subtle}`.

### AIBackendCard
- Background: `{components.AIBackendCard.background}`
- Border: 1px solid `{colors.border}`, Radius: `{rounded.lg}`
- Anatomy:
  1. Header: Section heading ("AI Backend & Inference Engine") + status badges:
     - Turnkey default badge (`{colors.status-green-bg}`, `{colors.status-green-fg}`, green ● dot) or CPU-Optimized Mode badge (`{colors.canvas}`, `{colors.text-primary}`, border `{colors.border}`).
     - FlashAttention indicator badge (`{colors.status-green-bg}`, `{colors.status-green-fg}`, "⚡ FlashAttention Active").
  2. Setting Rows:
     - Model selector dropdown with atomic download progress bar (ETA, transfer rate, SHA-256 verification status).
     - Context window segmented button pills (`2048`, `4096`, `8192`) with memory budget readout.
     - GPU layer offload slider with percentage readout (or capped CPU-thread allocation display `physical_cores - 2`).
     - Dynamic idle memory surrender switch.
  3. Action Footer: Secondary action buttons ("Reload Engine", "View Logs") with standard AA contrast and keyboard focus rings.

### RollbackModal
- Max-width: `{components.RollbackModal.maxWidth}` (480px).
- Radius: `{rounded.xl}`.
- Border: 1px solid `{colors.border}`, shadow `0 20px 25px -5px rgba(0, 0, 0, 0.1)`.
- Anatomy: Warning icon, version diff text, reassurance notice, and primary action button (`{colors.action-primary}`).

### BootProgressBar
- Height: `{components.BootProgressBar.height}` (6px).
- Background: `{components.BootProgressBar.background}`.
- Fill: `{components.BootProgressBar.fill}` with animated transition. Radius: `{rounded.full}`.

---

## Do's and Don'ts

### Do
- **Do** fit the 4-service Admin Dashboard on 1080p without page scrollbars, with internal scrolling only for 5+ services.
- **Do** pair every status color with its distinct micro-shape (●, ◆, ▲) and plain-language text label.
- **Do** provide high-visibility focus rings (`:focus-visible`) on all interactive buttons, cards, and switches.
- **Do** maintain strict visual separation between Sovereign's appliance chrome and the installed third-party apps.

### Don't
- **Don't** display terminal logs, stack traces, or Linux device paths (`/dev/sda1`) on the dashboard.
- **Don't** cram the media upload dropzone into the admin dashboard; keep it on its dedicated `/drop` surface.
- **Don't** use faint text colors below the 4.5:1 AA contrast threshold.
- **Don't** author custom sub-admin chat bubbles or media player controls; those belong to the installed software.
