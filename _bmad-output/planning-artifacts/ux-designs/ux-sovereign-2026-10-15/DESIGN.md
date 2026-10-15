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
  text-faint: '#94A3B8'
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
    width: '28px'
    height: '28px'
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

Sovereign embodies **quiet appliance confidence**. It is designed to feel like high-end consumer audio or refined domestic hardware (Braun, Dieter Rams, Teenage Engineering, Bang & Olufsen)—never a dense Linux sysadmin dashboard or cluttered cloud infrastructure portal.

The design prioritizes clarity, spaciousness, and zero cognitive load. Data typography is crisp and legible; borders are hairline separators; status indicators are reassuring rather than alarming. The system communicates that home data is safe, private, and running smoothly without demanding constant user vigilance.

---

## Colors

The palette is rooted in clean Nordic light neutrality with decisive contrast and semantic traffic-light indicators:

- `{colors.canvas}` (`#F8F9FA`): The foundation background. A subtle, cool off-white that minimizes glare on 1080p+ screens and softens contrast compared to stark white.
- `{colors.surface}` (`#FFFFFF`): The card and panel elevation layer. Used for service cards, header, and settings panes to establish crisp hierarchy over the canvas.
- `{colors.border}` (`#E5E7EB`): Hairline structural division. Separates header, cards, and bottom telemetry bars without heavy drop-shadows.
- `{colors.text-primary}` (`#0F172A`): Deep slate obsidian for primary headings, service names, and active states. Delivers WCAG AAA contrast (14.2:1) against surface.
- `{colors.text-muted}` (`#64748B`): Secondary metrics, file metadata, and descriptive copy. Maintains WCAG AA compliance (4.6:1).
- `{colors.action-primary}` (`#0F172A`): Confident, tactile action buttons (such as "Open Service UI") with stark white text `{colors.action-primary-fg}`.
- `{colors.status-green}` (`#10B981`) & `{colors.status-green-bg}` (`#ECFDF5`): Healthy running state. Reassuring green dot and soft background container.
- `{colors.status-amber}` (`#F59E0B`) & `{colors.status-amber-bg}` (`#FFFBEB`): Attention required (e.g., transient AI warm-up, NAS reconnection check).
- `{colors.status-red}` (`#EF4444`) & `{colors.status-red-bg}` (`#FEF2F2`): Service stopped or error state requiring action.
- `{colors.accent-blue}` (`#2563EB`): Text links, interactive paths, and focus rings.

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

### 1. Single-Viewport Desktop Rule (1080p+)
On desktop viewports (width $\ge$ 900px, 1080p through 4K), the Central Admin Dashboard enforces a strict **single-viewport height (`100vh; overflow: hidden`)**. The entire appliance interface—Header (`{components.AppHeader.height}`), Service Card Grid, and Telemetry Bar (`{components.TelemetryBar.height}`)—must fit completely without vertical page scrolling.

### 2. Spacing Scale
Layouts adhere to an 8pt spatial grid:
- Card inner padding: `{spacing.5}` (20px)
- Gap between grid cards: `{spacing.4}` (16px)
- Outer viewport margin: `{spacing.6}` (24px) to `{spacing.8}` (32px)

### 3. Mobile Viewport Fluidity
On mobile screens ($< 900\text{px}$), the single-viewport restriction unlocks (`overflow-y: auto`). Service cards stack into a single column with full touch targets ($44\text{px}+$ height).

---

## Elevation & Depth

Sovereign avoids deep or fuzzy shadows. Visual hierarchy is established through **borders and tonal layers**:
- **Default Elevation**: Cards sit flush on `{colors.canvas}` with a 1px border (`{colors.border}`).
- **Card Hover Elevation**: Soft ambient shadow `0 4px 12px rgba(15, 23, 42, 0.04)` and border shift to `{colors.border-active}`.
- **Modals / Dropdowns**: Ambient depth `0 10px 25px -5px rgba(15, 23, 42, 0.08)`, border `{colors.border}`.

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
  3. Action: Full-width button (`{colors.action-primary}` fill, white text, 38px height) linking to the installed application in a new tab.

### StatusBadge
- Inline-flex pill: 3px top/bottom, 8px left/right. Radius: `{rounded.full}`.
- Dot: 6x6px rounded circle.
- Green state: `{colors.status-green-bg}` with `{colors.status-green-fg}` text and `{colors.status-green}` dot.
- Amber state: `{colors.status-amber-bg}` with `{colors.status-amber-fg}` text and `{colors.status-amber}` dot.
- Red state: `{colors.status-red-bg}` with `{colors.status-red-fg}` text and `{colors.status-red}` dot.

### TelemetryBar
- Height: `{components.TelemetryBar.height}` (56px)
- Background: `{colors.surface}`, Border Top: 1px solid `{colors.border}`
- Items: Storage progress bar (64px width, 6px height, `{colors.status-green}` fill), NAS connection text, Optical Drive idle badge, Day/Night GPU schedule badge.

### DropzoneCard
- Max-width: 680px, centered on `{colors.canvas}`.
- Target: 2px dashed border (`#CBD5E1`), padding `{spacing.12}`, hover transition to `#0F172A` border.
- Category Tabs: Pill group with active tab white background and subtle shadow.

---

## Do's and Don'ts

### Do
- **Do** fit the entire Admin Dashboard on 1080p without page scrollbars.
- **Do** use plain, reassuring consumer language for all states and tooltips.
- **Do** maintain strict visual separation between Sovereign's appliance chrome and the installed third-party apps.
- **Do** use the standard 8px corner radius across all components.

### Don't
- **Don't** display terminal logs, stack traces, or Linux device paths (`/dev/sda1`) on the dashboard.
- **Don't** cram the media upload dropzone into the admin dashboard; keep it on its dedicated `/drop` surface.
- **Don't** use decorative gradients, glow effects, or non-semantic saturated accents.
- **Don't** author custom sub-admin chat bubbles or media player controls; those belong to the installed software.
