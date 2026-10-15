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

> Behavioral specification for the Sovereign Home Server appliance layer. Defines information architecture, component behaviors, system states, accessibility floors, microcopy contrast tables, and key user flows. Paired with `DESIGN.md`. Spines win on conflict with all prototypes and mockups.
> Visual references: [Dashboard Mockup](mockups/dashboard.html) · [Standalone Dropzone Mockup](mockups/dropzone.html) · [Settings Mockup](mockups/settings.html).

---

## Foundation

Sovereign is a single-surface responsive web application operating as the **Appliance Management Layer** for the home server. 

### UI System & Architecture
- **Component Base**: `shadcn/ui` conventions on Next.js with Tailwind CSS semantics.
- **Visual Reference**: `DESIGN.md` governs all visual tokens, colors, shapes, and typography.
- **Scope Boundary**: Strict separation between the Sovereign Appliance Management Layer and installed third-party applications:
  - **Sovereign Owns**: First-run provisioning wizard, central admin dashboard, central settings hub (including **LLM Backend Administration**: active GGUF model switcher, turnkey default model status, context window sizing, GPU offload layer slider, dynamic idle surrender timeout, model reload, backend logs), standalone media dropzone, boot diagnostics screen, service lifecycle toggles, and safe rollback orchestration.
  - **Installed Applications Own**: Media streaming and playback UI (Jellyfin), document filing and OCR tagging UI (Paperless-ngx), interactive local AI chat UI (Open WebUI), music streaming UI (Navidrome/Audiobookshelf).
  - **Home Automation Owns**: Family wall tablet ambient cards and push notifications (Home Assistant via Sovereign webhooks/MQTT).

---

## Information Architecture

| Surface | Reached From | Purpose & Primary Actions | States Covered |
| :--- | :--- | :--- | :--- |
| **First-Run Setup Wizard** (`/setup`) | First visit or unconfigured state | 5-step guided wizard: Language &rarr; Network &rarr; Storage (Local format or NAS mount) &rarr; Feature checklist &rarr; Alert email. Completes with seamless transition to dashboard. | Cold-load, Empty, In-Progress, Testing NAS connection, Formatting, Complete. |
| **Central Admin Dashboard** (`/`) | Default home URL (`http://sovereign.local`) | Single-viewport appliance status dashboard: <br>• Extensible service cards (Jellyfin, Paperless, Audio, Local AI with Open WebUI + llama.cpp, + future add-ons).<br>• Primary "Open Service UI" buttons (open in new tab).<br>• Service overflow menus (`•••`) for restart, rollback, logs.<br>• Telemetry bar (Storage, NAS, Disc drive, GPU schedule). | Operational (Green), Attention Required (Amber), Error (Red), AI Warm-up (Amber), Offline/Reconnecting. |
| **Central Settings View** (`/settings`) | Dashboard header &rarr; Settings gear | Extensible modular system settings hub with tabbed navigation: Services & Add-ons, AI Backend & Models (`llama.cpp`), Smart Hardware Sharing, Storage & Pools, Maintenance & Rollback, Home Automation, System. | Read, Edit, Testing connection, Confirmation dialogs. |
| **Standalone Media Dropzone** (`/drop`) | Direct URL (`http://sovereign.local/drop`) or dashboard link | Household upload surface without admin privileges. Category selector (`Movies` \| `TV` \| `Music` \| `Audiobooks`), drag-and-drop target, copyable LAN path (`\\sovereign\drop`). | Idle, Drag-over, Uploading (Progress), Success, Duplicate/Error. |
| **Boot Diagnostics Screen** (`/boot`) | Automatic on startup post-power outage | Reassuring status screen displayed within 15s of kernel boot: *"Checking system safety after power outage... your files are secure."* Progress ring + auto-redirect. | Checking journals, Verifying database, Redirecting, Unrecoverable Hardware Alert. |

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
| **AI Card Metric** | `"Private RAG with Open WebUI & llama.cpp"` | `"Private Family Search"` |
| **AI Warm-up Subtitle**| `"Reloading model into GPU VRAM (<=5s)"` | `"Warming up private AI (ready in 5s)"` |
| **AI Backend Config** | `"llama-server --model qwen2.5-7b.gguf -ngl 33 -c 4096"` | `"Default Model: Qwen 2.5 7B · 100% GPU Offload"` |
| **Audio Sync Metric** | `"Subsonic API Active on port 4533"` | `"Mobile Phone Sync Ready"` |
| **DMS Sync Metric** | `"OCR Active · Mail Polling Connected"` | `"Automatic Text Search Active · Mailbox Connected"` |
| **Hardware Schedule**| `"GPU Arbitration Scheduler"` | `"Smart Hardware Sharing"` |
| **Missing Disc Key** | `"libaacs error: no matching processing key in KEYDB.cfg"` | `"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."` |
| **File Drop Zone** | `"Samba service listening on 0.0.0.0:445"` | `"Drop files here or copy to \\sovereign\drop on your home network."` |
| **Storage Metric** | `"4.2 TB NVMe Pool (btrfs raid1)"` | `"4.2 TB / 8.0 TB Internal Storage"` |
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
  - **Status Indicator**: Uses canonical three-state model (Green ●, Amber ◆, Red ▲) paired with explicit text.
  - **Overflow Menu (`•••`)**: Accessible popover providing three secondary actions:
    1. *Restart Service*: Gracefully stops container and restarts it.
    2. *Roll Back to Previous Version*: Opens `RollbackModal` for that specific container.
    3. *Download Diagnostic Logs*: Generates a scrubbed text summary of recent logs (zero personal file contents included).

### StatusBadge
- **Visual reference**: `DESIGN.md.components.StatusBadge`.
- **Behavior**: Inline semantic indicator pairing color and geometric micro-shape:
  - `Green ●`: Service passing all internal liveness checks ("Running Healthy").
  - `Amber ◆`: Service active but running degraded or warming up ("Attention Required" or "Starting AI Model...").
  - `Red ▲`: Service stopped or restarting continuously ("Stopped / Error").

### OverflowMenu
- **Visual reference**: `DESIGN.md.components.OverflowMenu`.
- **Behavior**: Triggered by click or `Enter` on `•••` button. Contains `aria-label="Options for [Service Name]"`. Traps focus within menu; closes on `Escape` or click-outside. Touch target is $44\text{px} \times 44\text{px}$.

### TelemetryBar
- **Visual reference**: `DESIGN.md.components.TelemetryBar`.
- **Behavior**: Live hardware status updated via Server-Sent Events every 15s using `aria-live="polite"`.
  - *Storage bar*: Color shifts from green (`{colors.status-green}`) to amber if capacity exceeds 85%. Contains `role="progressbar"`, `aria-valuenow="52"`, `aria-valuemin="0"`, `aria-valuemax="100"`.
  - *Optical drive*: Transitions automatically: `Idle · Ready for disc` &rarr; `Disc detected` &rarr; `Ripping (X%)` &rarr; `Matching metadata` &rarr; `Auto-ejecting disc`.
  - *Smart Hardware Sharing Pill*: Clicking opens a quick modal linking to the 24h timeline scheduler.

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
  - Synchronizes active tab with browser URL hash (`#services`, `#ai-backend`, `#gpu`, `#storage`, `#maintenance`, `#automation`, `#system`).
  - Supports arrow key (`Up`/`Down`) keyboard navigation.

### AIBackendSettings
- **Visual reference**: `DESIGN.md.components.AIBackendCard`.
- **Behavior**:
  - Displays turnkey setup badge: *"Default Setup: Qwen 2.5 7B (Q4_K_M) · Autodetected GPU Offload Active"* (on zero-GPU hosts: *"CPU-Optimized Mode: Balanced Core Allocation Active"*).
  - **FlashAttention Status**: Pill badge displaying *"FlashAttention Active · KV Cache Optimized"*.
  - **Active Model Selector**: Dropdown showing available `.gguf` files in storage; switching model triggers graceful reload with progress spinner.
  - **Generation Intent Presets**: Segmented button pills (*"Precise & Factual"* · *"Balanced Assistant"* · *"Creative Drafting"*) providing intuitive consumer presets for temperature and sampling without technical dials.
  - **Model Download Lifecycle**: Integrated download progress indicator showing transfer rate, ETA, and automatic SHA-256 verification badge, with inline "Resume Download" action on network interruption. Pre-flight storage check blocks downloads exceeding the safety floor with an explicit banner (*"Insufficient storage: 4.7 GB model requires at least 9.7 GB free space (5 GB system safety reserve)"*).
  - **Context Window Sizing**: Segmented button pills for `2048`, `4096` (default), and `8192` tokens; displays estimated memory usage badge.
  - **GPU Acceleration Slider**: Range slider controlling transformer layer offload (`-ngl`) from `0% (CPU Only)` to `100% (Full GPU Offload)`; automatically locked to CPU-only on headless systems without dedicated hardware.
  - **Dynamic Idle Surrender Toggle**: Accessible switch with caption: *"Release GPU memory after 5 minutes of inactivity (reserves VRAM for video streaming)"*.
  - **Operational Actions**: Buttons for *"Reload Backend Engine"* and *"View Inference Logs"*, executing inline without page navigation.

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

### 1. Universal Appliance States
1. **Healthy / Running (Green ●)**: All service cards display primary metrics.
2. **Attention Required / Degraded (Amber ◆)**: Service active with temporary degradation; explanatory banner visible directly inside card (e.g. NAS unreachable, email sync paused).
3. **Transient AI Warm-up (Amber ◆)**: In daytime mode when model wakes from dynamic idle surrender, badge reads: *"Waking local AI model into memory..."* (reverse proxy protected with >= 60s timeout guard to prevent gateway timeouts).
4. **Error / Stopped (Red ▲)**: Card provides immediate "Restart Service" and "Roll Back" options.
5. **Background Updating**: Status badge displays subtle rotating indicator: *"Updating to latest release..."*.
6. **Outage Recovery**: Boot diagnostics screen renders within 15 seconds of power restoration with reassuring status message.
7. **Watchdog Debounced State**: Transient I/O or swap spikes do not immediately trigger amber/red alarms; the system requires 3 consecutive failed health probes over 60 seconds before altering display status or initiating automated container recovery.

### 2. Edge & Lifecycle States
1. **Cold-Load / First-Run State**: On initial unprovisioned boot, root URL `/` automatically routes to `/setup`. Services are unconfigured; wizard presents step 1 with browser language pre-selected.
2. **Offline / LAN Reconnecting State**: If local network connectivity drops, header displays persistent banner: *"Reconnecting to Sovereign appliance..."* with automatic reconnection polling every 3 seconds.
3. **Unrecoverable Hardware Alert State**: If storage pool reports hardware block failures post-outage, screen displays: *"Hardware attention needed: A hard drive is reporting errors. Your existing files are locked in read-only safety mode."* Includes direct email notification dispatch.
4. **Storage Headroom Warning State**: When free host disk space falls below 10%, the telemetry bar displays an Amber warning (*"Storage low: Background imports paused to protect system databases"*), preventing SQLite database locks.

---

## Interaction Primitives

- **Touch Targets**: Minimum $44\text{px} \times 44\text{px}$ touch targets across all mobile and tablet viewports for all buttons, menus, and switches.
- **Keyboard Navigation**:
  - `Tab` / `Shift+Tab`: Full logical focus ring traversal across all interactive buttons, cards, and tabs with visible focus rings (`:focus-visible`).
  - `Enter` / `Space`: Activates buttons, toggles switches, opens overflow menus.
  - `Esc`: Closes modals, dropdowns, and overflow menus immediately.
- **Drag & Drop**: Native HTML5 Drag and Drop API with visual dropzone hover feedback.
- **One-Click Actions**: Zero multi-step confirmation dialogs for non-destructive actions; single-click launch for all installed services.

---

## Accessibility Floor

- **Contrast**: Text elements meet WCAG 2.1 AA standards ($4.5:1$ for body text, $3:1$ for large headings). Primary obsidian text meets WCAG AAA ($14.2:1$). Secondary text uses `{colors.text-faint}` ($7.0:1$ AAA).
- **Color Independence**: Status states are never conveyed by color alone. Every badge pairs a semantic colored dot and distinct micro-shape (●, ◆, ▲) with an explicit text label ("Running Healthy", "Starting AI Model...", "Attention Required").
- **Live Regions**: Asynchronous hardware updates (such as disc ripping progress or AI model warm-up) use `aria-live="polite"` so screen readers are notified without interrupting active reading.
- **Screen Reader Support**: All inline SVGs carry `aria-hidden="true"`, accompanied by descriptive `aria-label` or `.sr-only` text for assistive technologies.

---

## Key Flows

### UJ-1: Alex sets up Sovereign and launches Video Streaming
- **Protagonist**: Alex (Household Tech Lead).
- **Context**: Alex plugs power and Ethernet into the Sovereign server; sits on the sofa with a laptop.
- **Steps**:
  1. Alex opens `http://sovereign.local` in a web browser.
  2. Setup Wizard Step 1: Language auto-detected as English; clicks "Continue".
  3. Step 2: Network check displays green checkmark for DHCP Ethernet connection.
  4. Step 3: Storage picker shows internal storage pool. Alex clicks "Initialize Local Storage" (confirmation modal verifies zero data risk).
  5. Step 4: Feature checklist has Video, Audio, DMS, and AI pre-checked. Alex keeps defaults.
  6. Step 5: Alex enters email `alex@family.net` for system notifications and clicks "Complete Setup".
  7. **Climax**: Progress bar transitions seamlessly into the Central Admin Dashboard without reboot. The "Video Streaming" card displays **Green ●** with an active "Open Streaming UI" button.
  8. Alex clicks "Open Streaming UI"; Jellyfin opens in a new tab ready for playback.
- **Failure Path**: If router blocks `sovereign.local` mDNS, the initial connection instructions display a clean fallback card guiding Alex to find the DHCP IP address via router client list.

### UJ-2: Sarah digitizes household paperwork with zero filing friction
- **Protagonist**: Sarah (Household Records Manager).
- **Context**: Sarah reviewing mail at the kitchen counter; smartphone in hand.
- **Steps**:
  1. Sarah captures photos of an auto insurance renewal policy using a mobile scanner app synced to Sovereign's inbox.
  2. The document is delivered in the background to Sovereign.
  3. Sovereign's Paperless-ngx engine runs OCR, extracts metadata, and tags the renewal deadline (due in 10 days).
  4. An event is dispatched to Home Assistant.
  5. **Climax**: A proactive notification card appears on the kitchen wall tablet: *"Car insurance renewal due in 10 days"*, with a direct tap-to-view link.
  6. Sarah drops the paper envelope into the recycling bin, confident it is safely filed and indexed.
- **Failure Path**: If email ingestion credentials fail, the Document Archive card on the dashboard turns Amber ◆ with a direct "Re-authenticate Mailbox" button.

### UJ-3: Alex inserts a movie disc, streams on TV, and downloads to an iPad for travel
- **Protagonist**: Alex.
- **Context**: Preparing for family travel; digitizing Blu-ray collection.
- **Steps**:
  1. Alex inserts a Blu-ray movie into the server's optical drive and walks away.
  2. Optical drive widget on the dashboard transitions from `Idle` to `Disc Detected` &rarr; `Ripping`.
  3. Disc completes ripping and auto-ejects.
  4. At night (01:00 AM), the Smart Hardware Sharing scheduler shifts to Nighttime Mode, transcoding the movie into a space-efficient mobile format.
  5. The next morning, Alex opens the Jellyfin app on an iPad and taps "Download".
  6. **Climax**: On the flight, the movie plays smoothly with zero buffering while family streams at home.
- **Failure Path**: If the Blu-ray is scratched, the drive retries 3 times, auto-ejects, and logs an informational alert banner without halting server operations.

### UJ-4: Family queries household paperwork with Local AI (RAG)
- **Protagonist**: Alex.
- **Context**: Speaking with a home repair contractor about storm damage.
- **Steps**:
  1. Alex opens the Local AI Chat interface on a smartphone browser via bookmark.
  2. Local AI card on dashboard indicates transient amber state: *"Starting AI model..."* ($\le 5\text{s}$ warm-up from idle).
  3. Alex types: *"What is our current deductible for storm damage on our home insurance?"*.
  4. Local LLM queries Paperless-ngx document database securely within the local network.
  5. **Climax**: LLM responds within seconds: *"Your deductible is $500, according to page 4 of Policy #98231 renewed last month"*, with a clickable citation tag that opens the exact PDF page.
- **Failure Path**: If model takes $>10\text{s}$ to load, UI displays: *"Allocating memory, response will begin shortly..."*.

### UJ-5: Power outage recovery and safe automated container updates
- **Protagonist**: Alex & Sarah.
- **Context**: Thunderstorm causes an unexpected power outage; server shuts down dirty.
- **Steps**:
  1. Power is restored; Sovereign server reboots automatically.
  2. Boot Diagnostics screen renders within 15 seconds: *"Checking system safety after power restoration... your files are secure."*
  3. Filesystem journals verify cleanly; dashboard loads with all service cards returning to Green ●.
  4. At 03:00 AM, background update pulls new container images after taking a configuration snapshot.
  5. If an image fails its health check within 180 seconds, the engine automatically rolls back to the prior snapshot and sends an alert email.
  6. **Climax**: In the morning, Alex glances at the dashboard; an amber informational banner notes: *"Update rolled back automatically to ensure stability."* All media and documents remained uninterrupted.
- **Failure Path**: Alex can also click "•••" &rarr; "Roll back to previous version" on any card to trigger an immediate manual 1-click restore.

---

## Responsive & Platform

- **Desktop (1024px - 4K)**: 4-column service grid. Standard 4 services fit in a single viewport on 1080p (`height: 100vh; overflow: hidden`), with container max-width 1440px. Grid unlocks internal scrolling (`overflow-y: auto`) if 5+ services are installed.
- **Tablet (768px - 1023px)**: 2-column service card grid with fluid vertical scrolling.
- **Mobile (360px - 767px)**: Fluid single-column layout with vertical scrolling unlocked (`overflow-y: auto`). Touch targets sized at $\ge 44\text{px}$.
- **Kitchen Wall Tablet (Home Assistant)**: Ambient notification display; Sovereign dispatches clean webhooks for high-visibility glanceable cards.
- **Smart TV (10-foot UI)**: Consumed via native Jellyfin apps (Apple TV, Roku, Android TV); zero custom TV app needed.
