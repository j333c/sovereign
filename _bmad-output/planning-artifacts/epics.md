---
status: final
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
  - step-04-final-validation
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/prd.md
  - _bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/addendum.md
  - _bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/architecture-decisions.md
  - _bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md
  - _bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md
---

# Sovereign - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for Sovereign, decomposing the requirements from the PRD, UX Design, and Architecture requirements into implementable stories.

## Requirements Inventory

### Functional Requirements

FR1: First-Run Setup Wizard — The system guides first-time users through initial configuration on first boot at `http://sovereign.local` (or DHCP IP) with mDNS advertising, network verification, storage initialization or NAS mount, feature selection checkboxes, notification email, and direct transition to dashboard without manual reboot.
FR2: Feature Activation & Container Lifecycle — The user can activate, deactivate, and view the status of software features with a single click. Required container components and images are downloaded strictly on-demand only after the user chooses to activate them (no pre-downloading of inactive features), pulled exclusively from the official OCI repositories and registries of the respective upstream projects (e.g., Docker Hub / GHCR official project tags). Containers are provisioned with bounded memory limits (`mem_limit`), soft reservations, persistent user data and configuration volume preservation on deactivation, direct web UI links, and traffic-light status monitoring with watchdog dampening (3 consecutive failures over 60s).
FR3: Automated Maintenance & Rollback Engine — The system executes scheduled background updates during a maintenance window (default 3:00 AM), snapshots configuration and records image digests pre-update, automatically rolls back within 180 seconds or crash loops while dispatching alert emails, and provides manual one-click rollbacks in the Admin UI.
FR4: Power-Loss Resilience & Boot Diagnostics — The system recovers cleanly from abrupt power loss with write-ahead journaling and SQLite/PostgreSQL WAL mode, initializes the Admin Interface within 15 seconds of kernel boot, and displays real-time startup progress and storage health checks if recovery routines are executing.
FR5: Storage Mount Management — The user can configure media and document storage targets using internal physical disks (formatting/mounting SATA/NVMe pools) or external network storage (NFS/SMB shares with credentials), gracefully handling NAS disconnections with an informative degraded status rather than crashing container runtimes.
FR6: Media Landing Zone Drop — The user can copy media files into the system via an SMB network share (`\\sovereign\drop`) or web drag-and-drop in the Admin Interface, featuring designated target folders (`drop/movies`, `drop/tv`, `drop/music`, `drop/audiobooks`), MIME/extension inspection, and automated media library scan trigger within 60 seconds.
FR7: Optical Disc Automated Ingestion — The system detects inserted optical media (Blu-ray, DVD, Audio CD) within 10 seconds, rips unencrypted discs to temporary staging, supports user-provided decryption keys (`KEYDB.cfg`/MakeMKV) for commercial Blu-ray, retrieves metadata/artwork from TMDb/IMDb/MusicBrainz, automatically ejects upon completion, and ejects after 3 failed retries with an alert.
FR8: Scheduled Transcoding Queue — The system executes scheduled background video transcoding during off-peak hours (default 01:00 to 07:00) to encode raw imports into standardized H.264/H.265 MP4 direct-play files and generate optimized 720p/1080p mobile offline profiles.
FR9: In-House Video Streaming — The system serves high-definition video streams across local and remote home devices using pre-configured Jellyfin container instances, supporting direct streaming, hardware-accelerated transcoding fallback (Intel QuickSync, Nvidia NVENC, VAAPI), DLNA discovery, and automatic storage path pre-configuration.
FR10: Audio Streaming, Party Playlists & Audiobooks — The system manages and streams personal music libraries, collaborative party playlists, and audiobook collections via a dual-container architecture: Navidrome for music and party playlists with Subsonic API support, and Audiobookshelf for audiobooks and podcasts with chapter markers, multi-device sync, and speed controls.
FR11: Centralized Document Management — The system stores, OCRs, categorizes, and indexes household documents via an integrated Paperless-ngx container, capping OCR worker concurrency to `max(1, physical_cores / 2)` at low priority (`nice -n 10`), generating full-text search indexes, and storing documents in standard filesystem structures.
FR12: Multi-Channel Document Ingestion — The user can ingest documents via mobile camera scanning companion apps and automated IMAP/SSL email polling with credentials to archive PDF attachments.
FR13: Smart Home & Notification Event Bridge — The system dispatches urgent alerts to Home Assistant (webhook/MQTT) and consolidated email digests when documents contain critical deadlines or renewal dates within a configurable threshold (e.g., 14 days), using deterministic regex parsing and nightly LLM extraction passes.
FR14: Browser-Based Local AI Chat Interface — The user can interact with local AI models through a pre-integrated Open WebUI interface via desktop and mobile web browsers, operating 100% offline with persona selection, chat history, and localized system customization prompts.
FR15: Managed LLM Inference Backend & Turnkey Default Setup — The system manages a dedicated high-performance `llama.cpp` inference engine backend with turnkey default setup. Inference engine containers and GGUF model weights are downloaded strictly on-demand when Local AI is activated or when a model is selected by the user, fetched directly from official upstream project repositories (official GitHub Container Registry images and official upstream model repositories/mirrors with SHA-256 validation). Features autodetected GPU/CPU acceleration, FlashAttention enabled, standardized `-np 2` parallel slots, air-gapped inference operation, decoupled `/data/models` storage with pre-flight GGUF magic check, safe HTTP 503 stub mode when models are missing, pre-flight storage reservation guard (model size + 5 GB reserve), generation intent presets (Precise 0.1, Balanced 0.7, Creative 0.85), two-lane priority request governor (user chat prioritized over background OCR/metadata tasks), GPU layer offloading slider (`-ngl`), context size presets (`-c`: 2048, 4096, 8192), CPU thread cap `max(1, physical_cores - 2)` on CPU-only hosts, and lifecycle status reporting with reload and log controls.
FR16: Single-Source-of-Truth RAG — The local AI queries Paperless-ngx directly via SQLite FTS5 full-text search and metadata matching without duplicate document uploads or external vector database bloat, utilizing a pre-flight Context Window Headroom Truncator with a 512-token response buffer, citing document titles and page numbers.
FR17: Standardized Shared AI Platform API — The system exposes an internal OpenAI-compatible `/v1` endpoint from the managed LLM backend and automatically configures installed features (Open WebUI, Paperless-ngx) with internal endpoint credentials.
FR18: Active GPU & VRAM Arbitration & Day/Night Scheduling — The system dynamically arbitrates GPU memory and compute across `llama.cpp` and transcoders: off-peak direct-play pre-encoding to avoid streaming transcode contention, daytime priority (07:00–01:00) for interactive AI, nighttime batch window (01:00–07:00) for transcoding and OCR, admin UI schedule configuration, dynamic idle surrender (timeout <= 5 min) releasing VRAM, daytime warm-up telemetry (`status: warming_up`) with UI notifications, and split iGPU/dGPU routing.
FR19: Multi-Language UI & Documentation — The system displays all admin interfaces, setup wizards, and user documentation in four primary launch languages (English, German, Spanish, French) with an instant language switcher and browser language autodetection.
FR20: Project Marketing Site & GitHub Presence — The project maintains a public marketing website and open-source GitHub repository detailing project architecture, BMad Method attribution, transparent AI-assistance notes, and FOSS license compliance.
FR21: Minimum Hardware Specification Guidance & Future Hardware Roadmap — The platform publishes concrete minimum hardware profiles across three tiers (Light Tier, Media Tier, AI & Power Tier) and reserves architectural hooks for turnkey hardware appliance sales.

### NonFunctional Requirements

NFR1: Reliability & Power Resilience — Persistent container data and configuration state must survive simulated abrupt power loss with zero database corruption (WAL journaling enforced across all SQLite and PostgreSQL databases); boot routines must automatically perform filesystem checks and clean remounts without manual intervention.
NFR2: Performance & Responsiveness — Admin Interface web dashboard Time-to-Interactive (TTI) must be under 2.0 seconds on local gigabit LAN; cold boot to operational Admin Interface under 60 seconds; Local AI first-token latency for RAG queries under 3.5 seconds on recommended GPU hardware.
NFR3: Security, Sandboxing & Zero Telemetry — All features execute in isolated OCI containers with restricted privileges (read-only root where supported, non-root execution); zero telemetry transmitted externally; inter-container communication routed over dedicated internal bridge networks with minimal external ports exposed to LAN.
NFR4: Observability & Self-Healing — Continuous internal health checks per feature (process status, database responsiveness, API endpoints); automated recovery up to 3 restarts with watchdog dampening before flagging Yellow/Red and dispatching alert emails.

### Additional Requirements

AR1: Greenfield Monorepo Substrate (Epic 1 Story 1) — Scaffold initial monorepo structure separating Go supervisor daemon (`/cmd/sovereign-daemon`, `/internal/...`) and Next.js frontend (`/web` App Router, React 19, Tailwind CSS v4, shadcn/ui) with shared development tooling and Compose v2 execution substrate.
AR2: Decoupled Control Plane Boundary (AD-1) — `sovereign-web` must never mount `/var/run/docker.sock` or invoke host shell processes. All system mutations, telemetry reads, and container operations execute strictly via HTTP REST and SSE to `sovereign-daemon`.
AR3: Declarative Compose v2 & Atomic Rollback Engine (AD-2) — Managed services declared in versioned compose specifications (`/data/state/compose/rev-:rev.yaml`). Pre-upgrade atomic SQLite snapshot (`/data/state/backups/pre-upgrade-:rev.db`). 180s healthcheck verification window with watchdog debouncing (3 consecutive failures over 60s) triggering automatic rollback to previous revision and database restore.
AR4: Dedicated Caddy Gateway Dual-Path Ingress (AD-3) — Dedicated Caddy container handling ingress via dedicated ports (`:8096`, `:8000`, `:3001`, `:4533`, `:13378`) and standard subpaths (`/media`, `/docs`, `/chat`, `/music`, `/audiobooks`) using Caddy `handle_path` and `X-Forwarded-Prefix`. Daemon manages routes and SSL certificates via Caddy Admin API (`http://caddy:2019`).
AR5: Docker Network Segregation & Mediated AI Gateway (AD-4) — Three isolated bridge networks: `sovereign-ingress`, `sovereign-control`, and `sovereign-ai`. Raw `llama-server` never publishes host ports. All `/v1` AI traffic routes through `sovereign-daemon:8080/v1` AI Gateway with request prioritization, 300s dynamic idle surrender, safe HTTP 503 stub, and GGUF header validation.
AR6: Structured `/data` Hierarchy & SQLite WAL Durability (AD-5) — Partitioned storage under single `/data` mount (`/data/media`, `/data/documents`, `/data/models`, `/data/state`, `/data/drop`). SQLite enforces `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`.
AR7: Tiered Boot Lifecycle & Post-Crash Diagnostics (AD-6) — Clean shutdown marker in `/data/state/boot_state.json`. If missing, system enters `DiagnosticsMode`, locks UI to `/boot` via `/api/system/boot-status`, runs `PRAGMA integrity_check` on SQLite stores, and only starts Tier 2 app containers after verification passes.
AR8: Staged Dropzone Ingestion Pipeline (AD-7) — Dropzone uploads stream to `/data/drop/incoming/:category/.uploading-:uuid-:filename`. Upon `fsync()`, atomic rename to `:filename`, media signature magic bytes validation, atomic `os.Rename` to `/data/media/:category`, and immediate library refresh webhooks to Jellyfin, Navidrome, or Audiobookshelf.
AR9: OpenAPI 3.1 Contract & SSE Telemetry (AD-8) — All daemon REST endpoints defined in single OpenAPI 3.1 specification. Frontend TypeScript client strictly code-generated. System telemetry (CPU, GPU, thermal, disk, service states) streams via Server-Sent Events (`/api/events`).
AR10: Role-Based Access Control & Session Authentication (AD-9) — Argon2id password hash stored in `sovereign.db`. Public LAN endpoints strictly limited to `/` (dashboard), `/drop` (media upload), and `/boot` (diagnostics). All configuration and mutations require authenticated `HttpOnly`, `SameSite=Strict` session cookie.

### UX Design Requirements

UX-DR1: Design Tokens & Theme Foundation — Implement Nordic/Braun minimalist design token system in Tailwind CSS v4 and CSS variables matching `DESIGN.md` (Canvas `#F8F9FA`, Surface `#FFFFFF`, Text Primary `#0F172A`, Status Green `#10B981`/`#ECFDF5`, Amber `#F59E0B`/`#FFFBEB`, Red `#EF4444`/`#FEF2F2`, Accent/Focus Blue `#2563EB`, Day `#3B82F6` and Night `#0F172A` timeline colors, typography scales, rounded corners `DEFAULT: 8px`).
UX-DR2: Persistent AppHeader & Status Pill — Implement `AppHeader` (height 64px, persistent top bar, crest/title linking to `/`, aggregate health pill showing overall system status with popover alert breakdown, quick links to `/drop` and `/settings`, and instant multilingual switcher).
UX-DR3: ServiceCard & StatusBadge Components — Implement standardized `ServiceCard` with semantic `StatusBadge` pairing color and geometric shapes (Green ● Running Healthy, Amber ◆ Attention Required / Warming up, Red ▲ Stopped / Error), primary "Open [Service] UI" action button (opens in new tab with pre-authenticated session), and accessible overflow menu (`•••`) with 44px touch target supporting keyboard navigation (`Enter`, `Escape`, focus trap) for Restart, Rollback, and Download Logs.
UX-DR4: First-Run Setup Wizard (`/setup`) — Implement 5-step guided wizard (Step 1: Language selection; Step 2: Network verification & hostname; Step 3: Storage selection [Local drive format or NAS NFS/SMB mount with credentials test]; Step 4: Feature selection checklist; Step 5: Admin password creation & alert email). Must provide animated transitions, inline validation, and seamless transition to dashboard without page reload or manual reboot.
UX-DR5: Central Admin Dashboard (`/`) — Implement single-viewport responsive appliance dashboard displaying telemetry bar (storage pool usage, NAS connection, optical drive status, Smart Hardware Sharing day/night schedule pill), responsive grid of Service Cards (Jellyfin, Paperless, Navidrome/Audiobookshelf, Local AI), and real-time SSE telemetry updates.
UX-DR6: Standalone Media Dropzone (`/drop`) — Implement public unauthenticated drag-and-drop file ingestion surface with category pills (`Movies`, `TV`, `Music`, `Audiobooks`), drag-over visual feedback, multi-file concurrent upload progress bars, copyable LAN path (`\\sovereign\drop`), and clear upload completion / duplicate handling feedback.
UX-DR7: Boot Diagnostics Screen (`/boot`) — Implement reassuring boot screen displayed within 15s of kernel boot post-power outage, adhering strictly to microcopy rules (*"Checking system safety after power restoration... your files are secure."*), with circular progress indicator, real-time stage descriptions, and automated redirect to `/` once diagnostics pass.
UX-DR8: Central Settings Hub (`/settings`) — Implement tabbed settings view (Services & Add-ons, AI Backend & Models, Smart Hardware Sharing, Storage & Pools, Maintenance & Rollback, Home Automation, System) with confirmation dialogs for destructive actions.
UX-DR9: AI Backend & Model Administration Interface — Inside Settings, implement dedicated AI management tab displaying active GGUF model card, memory footprint (RAM/VRAM), FlashAttention indicator, one-click model switcher from `/data/models`, Generation Intent Presets (Precise & Factual [0.1], Balanced Assistant [0.7], Creative Drafting [0.85]), GPU layer offload slider (`-ngl`), context length selector (`-c`), CPU-Optimized mode indicator, dynamic idle surrender timeout slider, and live inference logs view.
UX-DR10: Smart Hardware Sharing & Timeline Visualizer — In Settings and Dashboard telemetry, implement visual Day/Night timeline scheduler (07:00–01:00 Day AI Priority in blue; 01:00–07:00 Night Transcoding & Batch Window in slate), allowing users to adjust scheduling hours and task priorities.
UX-DR11: Consumer-Grade Microcopy & Error Prevention — Strictly enforce consumer voice (zero sysadmin jargon, stack traces, or raw error numbers) per the Microcopy Contrast Table across all UI error states, alerts, tooltips, and confirmation modals.
UX-DR12: Accessibility & Responsive Layout Floors — Enforce WCAG 2.1 AA accessibility (minimum 4.5:1 text contrast on light/dark surfaces, 3:1 non-text contrast, 44x44px touch targets, visible 2px `#2563EB` focus rings, full keyboard accessibility, ARIA live regions for telemetry), responsive breakpoints (Mobile 375px single column, Tablet 768px dual column, Desktop 1280px+ grid).

### FR Coverage Map

FR1: Epic 1 - First-Run Setup Wizard (mDNS discovery, storage setup/NAS mount, admin email, feature checkboxes)
FR2: Epic 2 - Feature Activation & Container Lifecycle (On-demand container downloads strictly from official upstream repos, memory limits, volume preservation)
FR3: Epic 6 - Automated Maintenance & Rollback Engine (Scheduled 3:00 AM updates, pre-update SQLite/compose snapshots, 180s healthcheck verification, one-click manual rollback)
FR4: Epic 1 - Power-Loss Resilience & Boot Diagnostics (SQLite WAL mode, <15s kernel-to-dashboard boot, post-crash `/boot` diagnostics)
FR5: Epic 1 - Storage Mount Management (Internal SATA/NVMe pool formatting/mounting, external NFS/SMB NAS shares with graceful disconnection status)
FR6: Epic 2 - Media Landing Zone Drop (SMB network share `\\sovereign\drop` + web drag-and-drop, category subfolders, staged atomic ingestion, library refresh trigger)
FR7: Epic 3 - Optical Disc Automated Ingestion (10s optical drive polling, automated disc rip staging, `KEYDB.cfg` Blu-ray decryption support, TMDb/IMDb/MusicBrainz metadata query, auto-eject)
FR8: Epic 3 - Scheduled Transcoding Queue (Off-peak 01:00–07:00 batch queue, standardized H.264/H.265 MP4 direct-play pre-encoding, 720p/1080p iPad mobile offline profiles)
FR9: Epic 2 - In-House Video Streaming (Pre-configured Jellyfin container, direct play primary path, hardware acceleration fallback QuickSync/NVENC/VAAPI, DLNA discovery)
FR10: Epic 2 - Audio Streaming, Party Playlists & Audiobooks (Dual container architecture: Navidrome for music/party playlists with Subsonic API, Audiobookshelf for audiobooks/podcasts with multi-device sync)
FR11: Epic 4 - Centralized Document Management (Integrated Paperless-ngx container, concurrency-capped OCR workers `max(1, physical_cores/2)` at `nice -n 10`, full-text search indexing, standard filesystem storage)
FR12: Epic 4 - Multi-Channel Document Ingestion (Paperless mobile app compatibility, automated IMAP/SSL email polling with credentials to archive PDF attachments)
FR13: Epic 4 - Smart Home & Notification Event Bridge (Deterministic regex date/amount parser + nightly LLM extraction, Home Assistant webhook/MQTT alerts, email digest notifications for impending deadlines)
FR14: Epic 5 - Browser-Based Local AI Chat Interface (Pre-integrated Open WebUI interface, 100% offline chat, persona selection, chat history, localized system prompt)
FR15: Epic 5 - Managed LLM Inference Backend & Turnkey Default Setup (Managed `llama.cpp` container backend, on-demand container and GGUF model download from official upstream sources with SHA-256 validation, FlashAttention enabled, `-np 2` parallel slots, air-gapped, decoupled `/data/models`, GGUF magic check, safe HTTP 503 stub mode, 5 GB storage reservation guard, intent presets, two-lane request governor, GPU layer offloading slider, context size presets, CPU thread cap on CPU-only hosts, reload/logs controls)
FR16: Epic 5 - Single-Source-of-Truth RAG (Direct Paperless-ngx SQLite FTS5 queries without duplicate vector DB, Context Window Headroom Truncator with 512-token response buffer, document title and page citations)
FR17: Epic 5 - Standardized Shared AI Platform API (Internal OpenAI-compatible `/v1` endpoint from managed LLM backend, automatic endpoint injection into features)
FR18: Epic 5 - Active GPU & VRAM Arbitration & Day/Night Scheduling (Daytime 07:00–01:00 AI priority, nighttime 01:00–07:00 batch transcoding/OCR, admin UI schedule configuration, dynamic idle surrender <= 5 min releasing VRAM, daytime warm-up telemetry `status: warming_up`, split iGPU/dGPU routing)
FR19: Epic 6 - Multi-Language UI & Documentation (English, German, Spanish, French language selector, browser language autodetection, translated guides)
FR20: Epic 7 - Project Marketing Site & GitHub Presence (Marketing landing page, GitHub README with BMad attribution, transparent AI-assistance notice, and open-source license compliance)
FR21: Epic 7 - Minimum Hardware Specification Guidance & Future Hardware Roadmap (Three-tier hardware profiles: Light, Media, AI & Power tiers; architectural hooks for future hardware sales)

## Epic List

### Epic 1: Appliance Foundation, Setup Wizard & Power Resilience
The user can power on the Sovereign server, discover `sovereign.local` via mDNS on the local network, complete the 5-step First-Run Setup Wizard (storage pool / NAS mount, network, admin credentials, initial feature checklist), access the responsive central dashboard with live SSE telemetry, and rely on clean boot recovery with automated post-power-outage diagnostics.
**FRs covered:** FR1, FR4, FR5

### Epic 2: In-House Media Streaming & Staged Ingestion Dropzone
The user can activate video streaming (Jellyfin) and audio/audiobook streaming (Navidrome & Audiobookshelf) with one click—downloading official upstream container images strictly on-demand—and copy media files into the household dropzone (`\\sovereign\drop` or web UI) with staged atomic ingestion and instant library refresh.
**FRs covered:** FR2, FR6, FR9, FR10

### Epic 3: Physical Disc Automation & Off-Peak Transcoding Queue
The user can slide Blu-ray, DVD, or Audio CD discs into the server's optical drive to have them automatically ripped, tagged with cover art and metadata, and auto-ejected, while a scheduled background transcoding queue converts raw imports into standardized direct-play files and space-efficient iPad mobile profiles during off-peak hours.
**FRs covered:** FR7, FR8

### Epic 4: Private Document Archiving & Smart Home Alert Bridge
The user can ingest paper mail and invoices via smartphone scanner apps or email forwarding into a centralized Paperless-ngx archive with automated OCR, and receive urgent reminders on our home wall tablet or email when contractual deadlines or bills are approaching.
**FRs covered:** FR11, FR12, FR13

### Epic 5: Local AI Knowledge & Turnkey Inference Platform
The user can chat privately with a turnkey local AI (Open WebUI) to query household records (direct RAG from Paperless-ngx with page citations) without cloud leaks, manage GGUF models and sampling intent presets in the Admin settings, and rely on dynamic GPU arbitration to surrender VRAM automatically when media tasks run.
**FRs covered:** FR14, FR15, FR16, FR17, FR18

### Epic 6: Appliance Resilience, Safe Updates & System Administration
The user can rely on unattended scheduled container updates with automatic 180s health verification and atomic rollbacks, trigger manual one-click rollbacks for any service, manage system settings with confirmation protections, and operate the entire appliance in English, German, Spanish, or French.
**FRs covered:** FR3, FR19

### Epic 7: Ecosystem Governance, Hardware Profiles & Project Presence
The open-source user or hardware adopter can consult public hardware specification tiers to choose or assemble compatible equipment, access comprehensive multilingual documentation, and explore the transparent marketing site and GitHub repository with full attribution and licensing compliance.
**FRs covered:** FR20, FR21


## Epic 1: Appliance Foundation, Setup Wizard & Power Resilience

The user can power on the Sovereign server, discover `sovereign.local` via mDNS on the local network, complete the 5-step First-Run Setup Wizard (storage pool / NAS mount, network, admin credentials, initial feature checklist), access the responsive central dashboard with live SSE telemetry, and rely on clean boot recovery with automated post-power-outage diagnostics.

### Story 1.1: Greenfield Monorepo Scaffold & Core Supervisor Daemon Boundary

As an appliance developer,
I want to establish the monorepo structure with decoupled Go supervisor daemon (`/cmd/sovereign-daemon`), Next.js client (`/web`), structured `/data` directories, and SQLite WAL engine,
So that the appliance control plane runs securely without exposing host Docker sockets to presentation code.

**Acceptance Criteria:**

**Given** a clean development environment
**When** the monorepo is bootstrapped
**Then** directories `/cmd/sovereign-daemon`, `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}`, `/web`, and `/data/{media,documents,models,state,drop}` exist.
**And** `sovereign-daemon` initializes the SQLite state store at `/data/state/sovereign.db` enforcing `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`.
**And** `/var/run/docker.sock` is strictly mounted only by `sovereign-daemon` and absent from `sovereign-web`.
**And** all API contracts between web and daemon conform to a generated TypeScript client built from an OpenAPI 3.1 specification.
**And** Tailwind CSS v4 design tokens in `/web` match the Nordic/Braun design system from `DESIGN.md`.

### Story 1.2: Dedicated Caddy Ingress Gateway & Local mDNS Discovery

As a household user,
I want to reach my home server at `http://sovereign.local` on my home Wi-Fi network within 30 seconds of plugging it in,
So that I don't need to look up DHCP IP addresses in my router administration panel.

**Acceptance Criteria:**

**Given** server boot on a local network with mDNS multicast support
**When** the network interface initializes
**Then** `sovereign.local` is broadcast via mDNS within 30 seconds and resolves to the server's LAN IP.
**And** client HTTP requests on port 80/443 proxy `/` and `/drop` to `sovereign-web:3000` via Caddy.
**And** `sovereign-daemon` manages dynamic routes and certificates via `http://caddy:2019` over the isolated `sovereign-control` Docker bridge network.
**And** on routers where mDNS is disabled, direct DHCP IP access routes identically without errors.

### Story 1.3: Power-Loss Resilience & Reassuring Boot Diagnostics Screen

As a household owner,
I want the server to recover safely after an unexpected power outage and display an informative reassurance screen while checking database journals,
So that my files remain uncorrupted and I am confident the system is healthy without seeing scary Linux terminal error logs.

**Acceptance Criteria:**

**Given** an ungraceful shutdown without a clean shutdown record in `/data/state/boot_state.json`
**When** the system powers on
**Then** `sovereign-daemon` enters `DiagnosticsMode` and client visits to `http://sovereign.local` redirect to `/boot`.
**And** `/boot` displays the consumer reassurance message *"Checking system safety after power restoration... your files are secure."* with a circular progress animation.
**And** `sovereign-daemon` executes `PRAGMA integrity_check` on `/data/state/sovereign.db` and validates filesystem journal mounts within 15 seconds.
**And** upon successful integrity verification, the screen automatically redirects to `/` within 2 seconds.
**And** a graceful reboot or shutdown writes `clean_shutdown: true` and renders `/` directly on next boot within 15 seconds.

### Story 1.4: Storage Pool Formatting & Network NAS Mount Management

As a home server administrator,
I want to format local SATA/NVMe disks or connect an external NAS NFS/SMB share,
So that media and documents have dedicated, resilient storage partitions.

**Acceptance Criteria:**

**Given** unformatted internal SATA/NVMe disks
**When** the user selects local storage initialization in the storage wizard
**Then** `sovereign-daemon` partitions and mounts the disk under `/data` with write-ahead journaling enabled.
**And** when an existing NAS is selected, NFS or SMB share credentials undergo a non-destructive mount verification test before being saved.
**And** if a mounted NAS becomes temporarily disconnected, the daemon transitions affected feature cards to Amber status with a clear explanatory notice without crashing service runtimes.

### Story 1.5: Five-Step First-Run Setup Wizard & RBAC Authentication

As Alex setting up the server for the first time,
I want a 5-step guided wizard that configures language, network, storage, initial features, and admin credentials,
So that the appliance is tailored to my household and secured against unauthorized modifications.

**Acceptance Criteria:**

**Given** an unconfigured server on first boot
**When** the user visits `http://sovereign.local`
**Then** the browser opens `/setup` with Step 1 (Language selection: EN, DE, ES, FR).
**And** Step 2 validates network settings and hostname, while Step 3 executes the storage pool or NAS mount test.
**And** Step 4 presents selectable checkboxes for Video, Audio, DMS, and Local AI features.
**And** Step 5 records an administrator password hashed with Argon2id and an alert email address, issuing an authenticated session cookie (`HttpOnly`, `SameSite=Strict`).
**And** clicking "Complete Setup" transitions the user directly to the operational dashboard at `/` without requiring a server reboot.

### Story 1.6: Central Admin Dashboard & Real-Time SSE Telemetry Stream

As a household user,
I want to view a single-viewport appliance dashboard with persistent header and live telemetry,
So that I can see the server's overall health and resource consumption at a glance.

**Acceptance Criteria:**

**Given** a user viewing the central dashboard at `/`
**When** the page renders
**Then** the persistent 64px `AppHeader` displays the Sovereign crest, aggregate health pill, quick links to `/drop` and `/settings`, and an instant language switcher.
**And** real-time SSE telemetry from `/api/events` updates CPU, RAM, storage usage (`X.X TB / Y.Y TB`), and NAS connection status without page polling.
**And** unauthenticated LAN users can view read-only dashboard cards, while clicking `/settings` or destructive overflow actions prompts for the administrator password.
**And** layout adapts responsively across mobile (375px), tablet (768px), and desktop (1280px+) viewports with accessible 44px touch targets and high-contrast focus rings.


## Epic 2: In-House Media Streaming & Staged Ingestion Dropzone

The user can activate video streaming (Jellyfin) and audio/audiobook streaming (Navidrome & Audiobookshelf) with one click—downloading official upstream container images strictly on-demand—and copy media files into the household dropzone (`\\sovereign\drop` or web UI) with staged atomic ingestion and instant library refresh.

### Story 2.1: On-Demand Container Lifecycle Engine & Health Probing

As an appliance administrator,
I want features to download official container images only when I activate them and monitor their health with watchdog debouncing,
So that inactive services don't consume disk space or network bandwidth, and transient I/O spikes don't trigger restart loops.

**Acceptance Criteria:**

**Given** an inactive feature (e.g., Video Streaming)
**When** checking local container images
**Then** the image is not pre-downloaded on initial setup.
**And** when an administrator clicks "Activate", `sovereign-daemon` pulls the image strictly from official project registries (`jellyfin/jellyfin`, `deluan/navidrome`, `advplyr/audiobookshelf`) while streaming download progress over SSE.
**And** running containers enforce bounded memory limits (`mem_limit`) and soft reservation limits.
**And** health probes enforce watchdog dampening requiring 3 consecutive failures over 60 seconds before flagging Amber or attempting restart.
**And** deactivating a feature stops the runtime while preserving user data and configuration volumes in `/data`.

### Story 2.2: ServiceCard & StatusBadge Appliance UI Controls

As a household user,
I want interactive feature cards on the dashboard showing clear traffic-light status and an "Open UI" launcher button,
So that I can launch my services in a new tab or perform administrative restarts from an accessible menu.

**Acceptance Criteria:**

**Given** the dashboard at `/`
**When** rendered
**Then** each feature card displays a `StatusBadge` pairing color and geometry: Green ● ("Running Healthy"), Amber ◆ ("Attention Required" or "Starting..."), Red ▲ ("Stopped / Error").
**And** clicking "Open [Service] UI" on a Green card opens the application in a new browser tab routed through Caddy.
**And** the accessible overflow menu (`•••`) provides *Restart Service*, *Roll Back to Previous Version*, and *Download Diagnostic Logs* with a 44px touch target.
**And** service crashes or high memory events display consumer-friendly microcopy (*"Video Streaming paused due to high memory use. Automatically restarting..."*).

### Story 2.3: In-House Video Streaming Integration (Jellyfin)

As Alex streaming movies in the living room,
I want Jellyfin pre-configured to stream high-definition video from my media library with hardware transcoding fallback,
So that my Smart TV and mobile devices can stream movies directly without manual library configuration.

**Acceptance Criteria:**

**Given** Video Streaming activation
**When** the Jellyfin container starts
**Then** `/data/media/movies` and `/data/media/tv` are automatically mounted as pre-configured library paths.
**And** client playback defaults to direct-streaming without GPU transcode overhead whenever media formats are compatible.
**And** bandwidth-constrained or legacy format streams automatically fall back to hardware acceleration (Intel QuickSync, Nvidia NVENC, or VAAPI).
**And** DLNA and native Jellyfin API discovery broadcast on the local network.
**And** Jellyfin is accessible via dedicated port `:8096` and subpath `/media` through Caddy.

### Story 2.4: Music & Audiobook Streaming Integration (Navidrome & Audiobookshelf)

As a household member listening to music and audiobooks,
I want dedicated audio streaming backends for music playlists and audiobook chapter syncing,
So that our family can use standard mobile apps like Substreamer, Symfonium, or Audiobookshelf on our phones.

**Acceptance Criteria:**

**Given** Audio Streaming activation
**When** containers deploy
**Then** Navidrome mounts `/data/media/music` and Audiobookshelf mounts `/data/media/audiobooks`.
**And** Navidrome exposes its web player and Subsonic API on dedicated port `:4533` and subpath `/music`.
**And** Audiobookshelf exposes port `:13378` and subpath `/audiobooks`, preserving chapter markers, narration speed settings, and multi-device sync.

### Story 2.5: Staged Dropzone Ingestion Pipeline & Web Upload Interface

As Sarah adding new family videos and music to the server,
I want to drag files into `http://sovereign.local/drop` or copy them to `\\sovereign\drop` on my computer,
So that new media is safely verified, placed in the right library folder, and immediately visible in Jellyfin and Navidrome.

**Acceptance Criteria:**

**Given** a user accessing `http://sovereign.local/drop` or `\\sovereign\drop`
**When** viewing the interface
**Then** category selectors for `Movies`, `TV`, `Music`, and `Audiobooks` are available with drag-over visual feedback and copyable network path.
**And** upload streams write to `/data/drop/incoming/:category/.uploading-:uuid-:filename`.
**And** upon upload completion and `fsync()`, the daemon validates media magic bytes, atomically moves the file (`os.Rename`) to `/data/media/:category`, and cleans staging files.
**And** successful moves trigger immediate HTTP library refresh webhooks to Jellyfin, Navidrome, or Audiobookshelf within 5 seconds.
**And** invalid or non-media files are rejected with in-UI explanations and never indexed into media libraries.


## Epic 3: Physical Disc Automation & Off-Peak Transcoding Queue

The user can slide Blu-ray, DVD, or Audio CD discs into the server's optical drive to have them automatically ripped, tagged with cover art and metadata, and auto-ejected, while a scheduled background transcoding queue converts raw imports into standardized direct-play files and space-efficient iPad mobile profiles during off-peak hours.

### Story 3.1: Optical Drive Detection & Automated Disc Ripping Engine

As Alex with a box of physical DVDs and Blu-rays,
I want to insert a movie disc into the server's optical drive and walk away while it automatically rips and ejects,
So that I don't have to fiddle with desktop ripping software or command-line tools.

**Acceptance Criteria:**

**Given** an optical drive connected to the server
**When** a disc (Blu-ray, DVD, or Audio CD) is inserted
**Then** `sovereign-daemon` detects media insertion within 10 seconds.
**And** unencrypted DVDs and CDs automatically rip to `/data/media/staging` without user intervention.
**And** commercial encrypted Blu-rays utilize a user-provided `KEYDB.cfg` uploaded in Settings, or safely eject with an explanatory notice (*"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."*).
**And** upon rip completion, the optical drive tray automatically ejects.
**And** if reading fails after 3 retries, the disc ejects and dispatches an alert email.

### Story 3.2: Automated Media Metadata & Artwork Enrichment

As a movie and music collector,
I want ripped discs automatically matched against movie and music databases for titles, cover art, and track names,
So that my media libraries look clean and professionally organized in Jellyfin and Navidrome.

**Acceptance Criteria:**

**Given** a finished disc rip in `/data/media/staging`
**When** the metadata enrichment worker runs
**Then** it queries TMDb, IMDb, or MusicBrainz using volume labels and disc fingerprints.
**And** matched cover art posters, release year, genres, and track tags are downloaded and saved.
**And** processed media is atomically moved to `/data/media/{movies,tv,music}` and triggers an immediate library refresh webhook.

### Story 3.3: Off-Peak Transcoding Queue & Universal Direct-Play Pre-Encoding

As a household user streaming movies on our smart TVs,
I want large raw disc rips and uploaded media pre-encoded into universal direct-play formats during off-peak night hours,
So that our TV streams instantly without buffering and without hogging the GPU during the day.

**Acceptance Criteria:**

**Given** raw media waiting in the transcoding queue
**When** the scheduled off-peak window arrives (default: 01:00 to 07:00)
**Then** the background transcoding worker starts.
**And** raw videos encode into standardized H.264/H.265 MP4 files with universal stereo/multichannel AAC audio compatibility.
**And** during daytime hours (07:00 to 01:00), heavy batch video transcoding is throttled or paused to keep GPU compute and VRAM available for interactive AI chat.

### Story 3.4: Mobile Offline Profile Generation & iPad Sync

As Alex packing an iPad for a family flight,
I want the server to automatically generate space-efficient 720p/1080p mobile video profiles,
So that I can download movies to the iPad in seconds and watch offline with zero playback hiccups.

**Acceptance Criteria:**

**Given** an active video title in the library
**When** the off-peak queue runs
**Then** the encoder generates an optimized mobile direct-play profile (720p/1080p, bounded bitrate) alongside the master file.
**And** when tapping "Download" in the Jellyfin mobile app on an iPad or phone, the pre-encoded mobile profile transfers directly without requiring real-time on-the-fly server transcoding.


## Epic 4: Private Document Archiving & Smart Home Alert Bridge

The user can ingest paper mail and invoices via smartphone scanner apps or email forwarding into a centralized Paperless-ngx archive with automated OCR, and receive urgent reminders on our home wall tablet or email when contractual deadlines or bills are approaching.

### Story 4.1: Centralized Document Management Integration (Paperless-ngx)

As Sarah archiving family insurance policies and medical bills,
I want Paperless-ngx deployed on-demand with OCR worker resource bounds,
So that incoming documents become fully text-searchable without slowing down other server features.

**Acceptance Criteria:**

**Given** Document Management activation
**When** Paperless-ngx deploys
**Then** it pulls official `paperlessngx/paperless-ngx` on-demand and mounts persistent storage at `/data/documents`.
**And** OCR worker concurrency is capped to `max(1, physical_cores / 2)` and scheduled at low priority (`nice -n 10`), preventing CPU starvation of interactive services.
**And** incoming documents are OCR'd and indexed into SQLite FTS5 full-text search tables.
**And** Caddy routes traffic via dedicated port `:8000` and subpath `/docs`.

### Story 4.2: Multi-Channel Ingestion (Mobile Scanner & Automated IMAP Email Polling)

As Sarah opening paper mail in the kitchen or receiving PDF bills via email,
I want to snap photos with a mobile scanner app or auto-forward emails to the server,
So that documents are ingested into the archive with zero manual filing.

**Acceptance Criteria:**

**Given** a user with a Paperless-compatible mobile scanner app on iOS or Android
**When** scanning physical mail
**Then** documents upload directly over the local network into the ingestion pipeline.
**And** when IMAP credentials are configured in Settings, `sovereign-daemon` polls the mailbox over SSL, downloading and archiving PDF attachments.
**And** if mailbox polling fails, the DMS card transitions to Amber status (*"Automatic Text Search Active · Mailbox Disconnected"*) without crashing container runtimes.

### Story 4.3: Deadline Extraction Parser & Obligation Scanner

As a household administrator tracking family contracts,
I want the server to inspect incoming documents for renewal dates and payment due dates,
So that critical deadlines are flagged before penalties or auto-renewals occur.

**Acceptance Criteria:**

**Given** newly indexed document text in Paperless
**When** the daemon analyzer runs
**Then** a deterministic regex parser inspects dates, payment totals, and contract terms.
**And** an off-peak nightly LLM extraction pass validates and refines detected deadlines.
**And** detected deadlines within the threshold (default: 14 days) are queued in `sovereign.db` with document title, due date, and excerpt citation.

### Story 4.4: Home Assistant Webhook, MQTT & Email Alert Dispatcher

As a family member checking our kitchen wall tablet,
I want impending document deadlines to appear on our Home Assistant dashboard and in an email digest,
So that our family never misses an insurance renewal or utility payment.

**Acceptance Criteria:**

**Given** a flagged deadline event
**When** dispatched
**Then** `sovereign-daemon` publishes the alert to Home Assistant via webhook and MQTT topic `sovereign/alerts/documents`.
**And** a consolidated email digest is dispatched to the admin notification email detailing upcoming obligations with direct links to the archived PDFs.


## Epic 5: Local AI Knowledge & Turnkey Inference Platform

The user can chat privately with a turnkey local AI (Open WebUI) to query household records (direct RAG from Paperless-ngx with page citations) without cloud leaks, manage GGUF models and sampling intent presets in the Admin settings, and rely on dynamic GPU arbitration to surrender VRAM automatically when media tasks run.

### Story 5.1: Managed `llama.cpp` Backend & Appliance Boundary Guard Suite

As an appliance administrator,
I want a high-performance C++ LLM inference engine (`llama.cpp`) operating in an isolated network with strict boundary guards,
So that inference runs with low overhead, autodetects hardware acceleration, and enters a safe stub mode if models are missing rather than crashing.

**Acceptance Criteria:**

**Given** Local AI activation
**When** the inference engine starts
**Then** `sovereign-daemon` pulls official `ghcr.io/ggml-org/llama.cpp` on-demand and places it on the isolated `sovereign-ai` network with no exposed host ports.
**And** it autodetects hardware acceleration (CUDA, Vulkan, SYCL, or CPU), enabling FlashAttention (`-fa`) and standardized `-np 2` parallel execution slots.
**And** a Pre-flight GGUF Magic Header check validates binary header bytes (`GGUF`) before attempting model load.
**And** if `/data/models` contains no valid model, the backend enters Safe Stub Mode (listening on the internal port and returning HTTP 503 with instructions to download a model).
**And** model downloads reject immediately if free space on `/data/models` is less than `model_size + 5.0 GB`.

### Story 5.2: Mediated AI Gateway & Two-Lane Priority Request Governor

As a household user,
I want an internal AI gateway that routes all AI requests, prioritizes interactive chat over background tasks, and provides simple intent presets,
So that my chat queries never stall behind batch document processing and I don't need machine learning degrees to calibrate settings.

**Acceptance Criteria:**

**Given** internal AI requests to `sovereign-daemon:8080/v1`
**When** arriving from Open WebUI or Paperless
**Then** the daemon acts as the AI Gateway reverse proxy to `llama-server`.
**And** the Two-Lane Priority Request Governor prioritizes interactive human chat over automated background tasks (OCR extraction, metadata tagging).
**And** inference requests enforce pre-calibrated Generation Intent Presets (*Precise & Factual* at temp 0.1; *Balanced Assistant* at temp 0.7; *Creative Drafting* at temp 0.85).

### Story 5.3: Dynamic GPU Arbitration & Day/Night Idle Surrender

As a household member streaming movies or chatting with AI,
I want GPU memory automatically released when the AI is idle and scheduled around family day/night habits,
So that video playback never crashes with Out-of-Memory errors and the AI wakes smoothly during the day.

**Acceptance Criteria:**

**Given** 300 seconds of AI inactivity
**When** the idle ticker expires
**Then** `sovereign-daemon` unloads model weights from VRAM, surrendering GPU memory to active media tasks.
**And** during daytime hours (07:00 to 01:00), waking the model returns `status: "warming_up"` with an extended >=60s timeout buffer and an in-UI notification (*"Waking up private AI (ready in 5s)..."*).
**And** hybrid GPU hardware automatically pins video transcoding to the iGPU and LLM inference to the dGPU.
**And** CPU-only hosts automatically cap threads to `max(1, physical_cores - 2)` and display a "CPU-Optimized Mode" indicator.

### Story 5.4: Open WebUI Chat Interface & Localized Customization Prompts

As Alex chatting with the local AI on a smartphone or laptop,
I want a clean, responsive chat interface that operates 100% offline and understands local cultural and contractual norms,
So that our family can brainstorm, draft emails, and ask questions with complete privacy.

**Acceptance Criteria:**

**Given** Local AI activation
**When** deployed
**Then** Open WebUI starts on-demand from `ghcr.io/open-webui/open-webui` connected to the internal AI Gateway.
**And** Open WebUI is reachable via dedicated port `:3001` and subpath `/chat` through Caddy.
**And** the interface provides persona selection, chat history, and applies a pre-configured localized system prompt matching user language and regional legal/contractual norms.
**And** zero external telemetry calls or third-party cloud API pings are emitted.

### Story 5.5: Single-Source-of-Truth RAG & Context Window Headroom Truncator

As Alex asking the local AI about an insurance deductible or household bill,
I want the AI to query our Paperless document archive directly and cite exact document titles and page numbers,
So that I get verified, accurate answers without uploading files to multiple places or running a heavy vector database.

**Acceptance Criteria:**

**Given** a document query in Open WebUI
**When** RAG executes
**Then** it queries Paperless-ngx SQLite FTS5 full-text search directly without requiring an external vector database (Chroma/Qdrant).
**And** the Context Window Headroom Truncator reserves a dedicated 512-token response buffer, retaining top-ranked chunks and appending an explanatory notice if excerpts exceed headroom.
**And** responses cite specific document titles and page numbers with clickable links to the archived PDF.

### Story 5.6: AI Backend & Model Administration Interface in Settings

As an appliance administrator,
I want an administrative panel in Settings to manage GGUF models, view memory usage, and monitor inference logs,
So that I can switch models or inspect performance without using command-line commands.

**Acceptance Criteria:**

**Given** the Settings view at `/settings`
**When** opening the AI tab
**Then** the active model card displays RAM/VRAM footprint, tokens/sec inference speed, and FlashAttention status.
**And** new models download atomically (`.downloading` suffix, SHA-256 verification, atomic rename to `.gguf`) with resumable range downloads and UI retry controls.
**And** clicking a downloaded GGUF model switches the active backend model after passing the GGUF header magic check.
**And** the user can adjust the GPU offload layer slider (`-ngl`), context length selector (`-c`), and dynamic idle surrender timeout slider with immediate effect.


## Epic 6: Appliance Resilience, Safe Updates & System Administration

The user can rely on unattended scheduled container updates with automatic 180s health verification and atomic rollbacks, trigger manual one-click rollbacks for any service, manage system settings with confirmation protections, and operate the entire appliance in English, German, Spanish, or French.

### Story 6.1: Declarative Compose Revisioning & Pre-Upgrade Snapshot Engine

As an appliance administrator,
I want service configurations managed through versioned Compose specifications with pre-upgrade SQLite database snapshots,
So that every change is tracked and downgrades never leave orphaned or incompatible database state.

**Acceptance Criteria:**

**Given** any service configuration or image tag change
**When** applied
**Then** `sovereign-daemon` generates a declarative revision file at `/data/state/compose/rev-:rev.yaml`.
**And** `sovereign-daemon` creates an atomic SQLite snapshot at `/data/state/backups/pre-upgrade-:rev.db` and snapshots configuration volumes before pulling new images.
**And** the active revision number is tracked and persisted in `sovereign.db`.

### Story 6.2: Automated 180-Second Health Verification & Atomic Rollback Execution

As a homeowner sleeping at night,
I want background container updates at 3:00 AM to automatically roll back if the new version fails to boot or healthcheck,
So that our family server never wakes up broken after an upstream container release.

**Acceptance Criteria:**

**Given** scheduled updates in the maintenance window (default: 3:00 AM)
**When** a new container image is started
**Then** the 180-second health verification window begins.
**And** if health checks fail across 3 consecutive retries or enter a crash loop, the daemon terminates the container, restores `rev-:prev_rev.yaml`, and restores `/data/state/backups/pre-upgrade-:rev.db`.
**And** the affected feature card displays Amber with a reassuring message (*"Update couldn't start properly. We've safely restored your previous working version."*) and dispatches an alert email.

### Story 6.3: One-Click Manual Rollback & Rollback Confirmation Modal

As a user who noticed an issue with a newly updated service,
I want to click "Roll back to previous version" on the service card or in Settings,
So that I can immediately restore the working release without typing command-line rollback commands.

**Acceptance Criteria:**

**Given** a running feature card
**When** clicking "Roll Back to Previous Version" in the overflow menu (`•••`)
**Then** `RollbackModal` displays current version, previous version tag, and backup snapshot timestamp.
**And** confirming rollback restores the prior Compose revision and database snapshot within 30 seconds.
**And** the feature card returns to Green status with a success notification.

### Story 6.4: Modular Central Settings Hub & Destructive Protection Dialogs

As an appliance administrator,
I want an authenticated, tabbed Settings hub with connection tests and confirmation dialogs for destructive actions,
So that I can customize services, network, and storage safely without accidental data loss.

**Acceptance Criteria:**

**Given** an administrator navigating to `/settings`
**When** accessing the page
**Then** a valid `HttpOnly` session cookie is verified (or prompts for password authentication).
**And** tabs for Services & Add-ons, Storage & Pools, Maintenance & Rollback, and System provide clean form controls and inline test buttons.
**And** destructive actions (server reboot, factory reset, service uninstall) require an explicit confirmation dialog before execution.

### Story 6.5: Full Four-Language Internationalization (i18n)

As a user speaking German, Spanish, or French,
I want the entire appliance interface, setup wizard, and bundled guides available in my native language,
So that our household can manage the server comfortably without language barriers.

**Acceptance Criteria:**

**Given** first visit to the server
**When** loading the browser
**Then** the interface automatically detects browser language headers, defaulting to English, German (DE), Spanish (ES), or French (FR).
**And** clicking a different language in the `AppHeader` toggle updates all UI strings instantly without a page reload.
**And** bundled documentation and setup guides are fully translated and readable across all four launch languages.


## Epic 7: Ecosystem Governance, Hardware Profiles & Project Presence

The open-source user or hardware adopter can consult public hardware specification tiers to choose or assemble compatible equipment, access comprehensive multilingual documentation, and explore the transparent marketing site and GitHub repository with full attribution and licensing compliance.

### Story 7.1: Minimum Hardware Specification Guides & Hardware Tier Roadmap

As an open-source enthusiast planning my home server hardware build,
I want clear minimum hardware specifications categorized by feature tier and a roadmap for turnkey hardware appliances,
So that I purchase or reuse the right hardware without guessing resource requirements.

**Acceptance Criteria:**

**Given** hardware documentation and guidance
**When** published
**Then** three distinct hardware profiles are detailed:
  1. *Light Tier* (DMS + Audio): Dual-core x86/ARM64, 4 GB RAM, 64 GB storage.
  2. *Media Tier* (Video + DMS + Audio + Disc Ingestion): Quad-core x86 with Intel QuickSync / iGPU, 8 GB RAM, 2 TB+ storage.
  3. *AI & Power Tier* (Media + DMS + Local 7B/8B LLM RAG): 6+ core x86, 16–32 GB RAM, discrete GPU (8 GB+ VRAM) or unified memory APU, 4 TB+ storage.
**And** architectural hooks are reserved for ordering turnkey pre-assembled Sovereign home server hardware directly from the project website.

### Story 7.2: Public Marketing Landing Website & Feature Showcase

As a prospective user discovering Sovereign,
I want an informative, polished marketing website showcasing the appliance features, privacy guarantees, and hardware compatibility,
So that I understand how Sovereign solves household privacy and can get started immediately.

**Acceptance Criteria:**

**Given** the public marketing website
**When** accessed
**Then** it highlights core capabilities (Video, Audio, Paperless, Local AI, Disc Ingestion), zero-telemetry privacy guarantees, and appliance-grade simplicity.
**And** a hardware tier calculator helps users pick the right tier and provides direct links to the GitHub repository and installation documentation.

### Story 7.3: Open-Source Governance, BMad Attribution & License Compliance Suite

As an open-source community member or contributor,
I want a comprehensive GitHub repository with transparent AI assistance attribution, BMad Method acknowledgment, and strict FOSS licensing notices,
So that the project remains a compliant, reputable, and welcoming open-source ecosystem.

**Acceptance Criteria:**

**Given** the project GitHub repository
**When** reviewing `README.md`
**Then** it provides an architecture overview, quickstart instructions, explicit attribution of creation via the BMad Method, and transparent notes on AI-assisted development.
**And** prominent attribution, original copyright notices, and license compliance texts for all bundled open-source tools (Jellyfin, Navidrome, Audiobookshelf, Paperless-ngx, Open WebUI, llama.cpp, Caddy, ARM) are cataloged without proprietary restrictions.
