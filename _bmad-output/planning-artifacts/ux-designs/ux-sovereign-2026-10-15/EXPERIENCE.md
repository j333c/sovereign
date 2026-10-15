---
name: Sovereign
status: final
sources:
  - _bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/prd.md
  - _bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/addendum.md
created: 2026-10-15
updated: 2026-10-15
---

# Sovereign — Experience Spine

> Behavioral specification for the Sovereign Home Server appliance layer. Defines information architecture, component behaviors, system states, accessibility floors, and key user flows. Paired with `DESIGN.md`. Spines win on conflict with all prototypes and mockups.
> Visual references: [Dashboard Mockup](mockups/dashboard.html) · [Standalone Dropzone Mockup](mockups/dropzone.html) · [Settings Mockup](mockups/settings.html).

---

## Foundation

Sovereign is a single-surface responsive web application operating as the **Appliance Management Layer** for the home server. 

### UI System & Architecture
- **Component Base**: `shadcn/ui` conventions on Next.js with Tailwind CSS semantics.
- **Visual Reference**: `DESIGN.md` governs all visual tokens, colors, shapes, and typography.
- **Scope Boundary**: Strict separation between the Sovereign Appliance Management Layer and installed third-party applications:
  - **Sovereign Owns**: First-run provisioning wizard, central admin dashboard, central settings hub, standalone media dropzone, boot diagnostics screen, service lifecycle toggles, and safe rollback orchestration.
  - **Installed Applications Own**: Media streaming and playback UI (Jellyfin), document filing and OCR tagging UI (Paperless-ngx), interactive local AI chat UI (Open WebUI), music streaming UI (Navidrome/Audiobookshelf).
  - **Home Automation Owns**: Family wall tablet ambient cards and push notifications (Home Assistant via Sovereign webhooks/MQTT).

---

## Information Architecture

| Surface | Reached From | Purpose & Primary Actions | States Covered |
| :--- | :--- | :--- | :--- |
| **First-Run Setup Wizard** (`/setup`) | First visit or unconfigured state | 5-step guided wizard: Language &rarr; Network &rarr; Storage (Local format or NAS mount) &rarr; Feature checklist &rarr; Alert email. Completes with seamless transition to dashboard. | Empty, In-Progress, Testing NAS connection, Formatting, Complete. |
| **Central Admin Dashboard** (`/`) | Default home URL (`http://sovereign.local`) | Single-viewport appliance status dashboard: <br>• Extensible service cards (Jellyfin, Paperless, Audio, AI, + future add-ons).<br>• Primary "Open Service UI" buttons (open in new tab).<br>• Service overflow menus (`•••`) for restart, rollback, logs.<br>• Telemetry bar (Storage, NAS, Disc drive, GPU schedule). | Operational (Green), Degraded (Yellow), Error (Red), AI Warm-up (Amber). |
| **Central Settings View** (`/settings`) | Dashboard header &rarr; Settings gear | Extensible modular system settings hub with tabbed navigation: Services & Add-ons, Storage & Pools, Hardware & GPU schedule, Maintenance & Rollback, System. | Read, Edit, Testing connection, Confirmation dialogs. |
| **Standalone Media Dropzone** (`/drop`) | Direct URL (`http://sovereign.local/drop`) or dashboard link | Household upload surface without admin privileges. Category selector (`Movies` \| `TV` \| `Music` \| `Audiobooks`), drag-and-drop target, copyable LAN path (`\\sovereign\drop`). | Idle, Drag-over, Uploading (Progress), Success, Duplicate/Error. |
| **Boot Diagnostics Screen** (`/boot`) | Automatic on startup post-power outage | Reassuring status screen displayed within 15s of kernel boot: *"Checking system safety after power outage... your files are secure."* Progress ring + auto-redirect. | Checking journals, Verifying database, Redirecting. |

---

## Voice and Tone (Microcopy)

Sovereign speaks with **reassuring consumer clarity**. It protects family data and operates with appliance-like simplicity, completely eliminating Linux command-line jargon, stack traces, and unhelpful error numbers:

### Microcopy Contrast Table

| Context | Do NOT Say (Sysadmin / Developer Jargon) | DO Say (Sovereign Consumer Voice) |
| :--- | :--- | :--- |
| **Dirty Boot Recovery** | `"Running fsck on /dev/sda1; clean 41829/244320 files"` | `"Checking system safety after power restoration... your files are secure."` |
| **Out-of-Memory Halt** | `"Container jellyfin exited with code 137 (OOMKilled)"` | `"Video Streaming paused due to high memory use. Automatically restarting..."` |
| **Update Failed Check** | `"Docker healthcheck failed: image rolled back to sha256:7b1e..."` | `"Update couldn't start properly. We've safely restored your previous working version."` |
| **NAS Disconnected** | `"mount.nfs: Connection refused (errno 111) on 192.168.1.50"` | `"Home NAS storage is unreachable. Please verify that your NAS is powered on and connected to your Wi-Fi."` |
| **AI Model Loading** | `"Loading quantized GGUF weights into CUDA VRAM..."` | `"Starting AI model..."` |
| **Missing Disc Key** | `"libaacs error: no matching processing key in KEYDB.cfg"` | `"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."` |
| **File Drop Zone** | `"Samba service listening on 0.0.0.0:445"` | `"Drop files here or copy to \\sovereign\drop on your home network."` |
| **Privacy Guarantee** | `"Enforcing zero-telemetry egress iptables rule"` | `"100% Local & Private: Your files and personal data never leave your home."` |

---

## Component Patterns

### AppHeader
- **Visual reference**: `DESIGN.md.components.AppHeader`.
- **Behavior**: Persistent top bar across dashboard views.
  - Crest & Title link to `/`.
  - "All Systems Operational" health pill aggregates status: turns Amber if any service is degraded, Red if any service has halted. Clicking opens quick alert popover.
  - "Drop Media" button links to `/drop`.
  - Settings gear links to `/settings`.
  - Language toggle immediately switches UI strings without page reload.

### ServiceCard
- **Visual reference**: `DESIGN.md.components.ServiceCard`.
- **Behavior**:
  - **Primary Action ("Open [Service] UI")**: Enabled when status is Green. Opens the installed web interface in a new browser tab with SSO authentication headers pre-populated.
  - **Status Dot & Pill**: Reflects container healthcheck queries (polled every 10s via Server-Sent Events).
  - **Overflow Menu (`•••`)**: Accessible popover providing three secondary actions:
    1. *Restart Service*: Gracefully stops container and restarts it.
    2. *Roll Back to Previous Version*: Opens `RollbackModal` for that specific container.
    3. *Download Diagnostic Logs*: Generates a scrubbed text summary of recent logs (zero personal file contents included).

### StatusBadge
- **Visual reference**: `DESIGN.md.components.StatusBadge`.
- **Behavior**: Inline semantic indicator.
  - `Green`: Service passing all internal liveness checks.
  - `Amber`: Service active but running degraded or warming up. Displays explicit explanatory text on card.
  - `Red`: Service stopped or restarting continuously.

### OverflowMenu
- **Visual reference**: `DESIGN.md.components.OverflowMenu`.
- **Behavior**: Triggered by click or `Enter` on `•••` button. Traps focus within menu; closes on `Escape` or click-outside.

### TelemetryBar
- **Visual reference**: `DESIGN.md.components.TelemetryBar`.
- **Behavior**: Live hardware status updated via SSE every 15s.
  - *Storage bar*: Color shifts from green (`{colors.status-green}`) to amber if capacity exceeds 85%.
  - *Optical drive*: Transitions automatically: `Idle · Ready for disc` &rarr; `Disc detected` &rarr; `Ripping (X%)` &rarr; `Matching metadata` &rarr; `Auto-ejecting disc`.
  - *GPU Day/Night Pill*: Clicking opens a quick modal linking to the 24h timeline scheduler.

### DropzoneCard
- **Visual reference**: `DESIGN.md.components.DropzoneCard`.
- **Behavior**:
  - Category tabs (`Movies`, `TV Shows`, `Music`, `Audiobooks`) set the active deposit directory (`drop/movies`, `drop/tv`, etc.).
  - Dragging files over the box highlights the border to `{colors.action-primary}` with active pulse.
  - Supports multiple file drops simultaneously; displays individual progress bars.
  - Auto-triggers background library scan in Jellyfin/Audiobookshelf once upload completes.
  - Displays copy-to-clipboard button for the network UNC path (`\\sovereign\drop`).

### SettingsNavigation
- **Visual reference**: `DESIGN.md.components.SettingsNavigation`.
- **Behavior**: Sidebar navigation menu.
  - Synchronizes active tab with browser URL hash (`#services`, `#gpu`, `#storage`, `#maintenance`).
  - Supports arrow key (`Up`/`Down`) keyboard navigation.

### RollbackModal
- **Visual reference**: `DESIGN.md.components.RollbackModal`.
- **Behavior**:
  - Non-destructive confirmation dialog: displays the currently running container version vs the last known stable snapshot.
  - Prompts: *"Roll back to version [Date/Digest]?"*.
  - Explicit guarantee note: *"Your media, files, and documents are preserved and will not be touched."*
  - Action executes in background with visual spinner; auto-closes and updates card status.

### BootProgressBar
- **Visual reference**: `DESIGN.md.components.BootProgressBar`.
- **Behavior**:
  - Active during boot diagnostics screen.
  - Fills progressively from 0% to 100% as journal integrity checks complete.
  - Smoothly redirects to `/` as soon as the web server reports operational status.

---

## State Patterns

### Universal Appliance States
1. **Healthy / Running**: Status Green dot. All service cards display primary metrics.
2. **Attention Required / Degraded**: Status Amber dot. Explanatory banner visible directly inside card (e.g. NAS unreachable, email sync paused).
3. **Transient AI Warm-up**: Amber badge on Local AI card reading: *"Starting AI model..."* (maximum 5 seconds when reloading model into GPU VRAM from idle state).
4. **Error / Stopped**: Status Red dot. Card provides immediate "Restart Service" and "Roll Back" options.
5. **Background Updating**: Status badge displays subtle rotating indicator: *"Updating to latest release..."*.
6. **Outage Recovery**: Boot diagnostics screen renders within 15 seconds of power restoration with reassuring status message.

---

## Interaction Primitives

- **Touch Targets**: Minimum $44\text{px} \times 44\text{px}$ touch targets across all mobile and tablet viewports.
- **Keyboard Navigation**:
  - `Tab` / `Shift+Tab`: Full logical focus ring traversal across all interactive buttons, cards, and tabs.
  - `Enter` / `Space`: Activates buttons, toggles switches, opens overflow menus.
  - `Esc`: Closes modals, dropdowns, and overflow menus immediately.
- **Drag & Drop**: Native HTML5 Drag and Drop API with visual dropzone hover feedback.
- **One-Click Actions**: Zero multi-step confirmation dialogs for non-destructive actions; single-click launch for all installed services.

---

## Accessibility Floor

- **Contrast**: Text elements meet WCAG 2.1 AA standards ($4.5:1$ for body text, $3:1$ for large headings). Primary obsidian text meets WCAG AAA ($14.2:1$).
- **Color Independence**: Status states are never conveyed by color alone. Every badge pairs a semantic colored dot with an explicit text label ("Running Healthy", "Starting AI Model...", "Attention Required").
- **Live Regions**: Asynchronous hardware updates (such as disc ripping progress or AI model warm-up) use `aria-live="polite"` so screen readers are notified without interrupting active reading.
- **Screen Reader Support**: All inline SVGs carry `aria-hidden="true"`, accompanied by descriptive `.sr-only` text for assistive technologies.

---

## Key Flows

### UJ-1: Setup & Media Launch
- **Protagonist**: Alex (Household Tech Lead).
- **Context**: Alex plugs power and Ethernet into the Sovereign server; sits on the sofa with a laptop.
- **Steps**:
  1. Alex opens `http://sovereign.local` in a web browser.
  2. Setup Wizard Step 1: Language auto-detected as English; clicks "Continue".
  3. Step 2: Network check displays green checkmark for DHCP Ethernet connection.
  4. Step 3: Storage picker shows internal NVMe pool. Alex clicks "Initialize Local Storage" (confirmation modal verifies zero data risk).
  5. Step 4: Feature checklist has Video, Audio, DMS, and AI pre-checked. Alex keeps defaults.
  6. Step 5: Alex enters email `alex@family.net` for system notifications and clicks "Complete Setup".
  7. **Climax**: Progress bar transitions seamlessly into the Central Admin Dashboard without reboot. The "Video Streaming" card displays **Green** with an active "Open Streaming UI" button.
  8. Alex clicks "Open Streaming UI"; Jellyfin opens in a new tab ready for playback.
- **Failure Path**: If router blocks `sovereign.local` mDNS, the initial connection instructions display a clean fallback card guiding Alex to find the DHCP IP address via router client list.

### UJ-2: Zero-Friction Paperwork Ingestion
- **Protagonist**: Sarah (Household Records Manager).
- **Context**: Sarah reviewing mail at the kitchen counter; smartphone in hand.
- **Steps**:
  1. Sarah captures photos of an auto insurance renewal policy using a mobile scanner app synced to Sovereign's inbox.
  2. The document is delivered in the background to Sovereign.
  3. Sovereign's Paperless-ngx engine runs OCR, extracts metadata, and tags the renewal deadline (due in 10 days).
  4. An event is dispatched to Home Assistant.
  5. **Climax**: A proactive notification card appears on the kitchen wall tablet: *"Car insurance renewal due in 10 days"*, with a direct tap-to-view link.
  6. Sarah drops the paper envelope into the recycling bin, confident it is safely filed and indexed.
- **Failure Path**: If email ingestion credentials fail, the Document Archive card on the dashboard turns Yellow with a direct "Re-authenticate Mailbox" button.

### UJ-3: Optical Disc Ingestion & Travel Sync
- **Protagonist**: Alex.
- **Context**: Preparing for family travel; digitizing Blu-ray collection.
- **Steps**:
  1. Alex inserts a Blu-ray movie into the server's optical drive and walks away.
  2. Optical drive widget on the dashboard transitions from `Idle` to `Disc Detected` &rarr; `Ripping`.
  3. Disc completes ripping and auto-ejects.
  4. At night (01:00 AM), the GPU scheduler shifts to Nighttime Mode, transcoding the movie into a space-efficient mobile format.
  5. The next morning, Alex opens the Jellyfin app on an iPad and taps "Download".
  6. **Climax**: On the flight, the movie plays smoothly with zero buffering while family streams at home.
- **Failure Path**: If the Blu-ray is scratched, the drive retries 3 times, auto-ejects, and logs an informational alert banner without halting server operations.

### UJ-4: Local AI Document Query (RAG)
- **Protagonist**: Alex.
- **Context**: Speaking with a home repair contractor about storm damage.
- **Steps**:
  1. Alex opens the Local AI Chat interface on a smartphone browser via bookmark.
  2. Local AI card on dashboard indicates transient amber state: *"Starting AI model..."* ($\le 5\text{s}$ warm-up from idle).
  3. Alex types: *"What is our current deductible for storm damage on our home insurance?"*.
  4. Local LLM queries Paperless-ngx vector embeddings securely within the local network.
  5. **Climax**: LLM responds within seconds: *"Your deductible is $500, according to page 4 of Policy #98231 renewed last month"*, with a clickable citation tag that opens the exact PDF page.
- **Failure Path**: If model takes $>10\text{s}$ to load, UI displays: *"Allocating GPU memory, response will begin shortly..."*.

### UJ-5: Outage Recovery & Safe Auto-Updates
- **Protagonist**: Alex & Sarah.
- **Context**: Thunderstorm causes an unexpected power outage; server shuts down dirty.
- **Steps**:
  1. Power is restored; Sovereign server reboots automatically.
  2. Boot Diagnostics screen renders within 15 seconds: *"Checking system safety after power restoration... your files are secure."*
  3. Filesystem journals verify cleanly; dashboard loads with all service cards returning to Green.
  4. At 03:00 AM, background update pulls new container images after taking a configuration snapshot.
  5. If an image fails its health check within 180 seconds, the engine automatically rolls back to the prior snapshot and sends an alert email.
  6. **Climax**: In the morning, Alex glances at the dashboard; an amber informational banner notes: *"Update rolled back automatically to ensure stability."* All media and documents remained uninterrupted.
- **Failure Path**: Alex can also click "•••" &rarr; "Roll back to previous version" on any card to trigger an immediate manual 1-click restore.

---

## Responsive & Platform

- **Desktop (1080p - 4K)**: Single-viewport layout (`height: 100vh; overflow: hidden`). Zero vertical scrolling on the central dashboard.
- **Mobile (360px - 430px)**: Fluid single-column layout with vertical scrolling unlocked (`overflow-y: auto`). Touch targets sized at $48\text{px}$.
- **Kitchen Wall Tablet (Home Assistant)**: Ambient notification display; Sovereign dispatches clean webhooks for high-visibility glanceable cards.
- **Smart TV (10-foot UI)**: Consumed via native Jellyfin apps (Apple TV, Roku, Android TV); zero custom TV app needed.
