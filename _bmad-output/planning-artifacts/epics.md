---
status: draft
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
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
