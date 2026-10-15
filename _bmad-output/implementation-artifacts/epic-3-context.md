# Epic 3 Context: Physical Disc Automation & Off-Peak Transcoding Queue

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Provide end-to-end automated physical disc digitization and off-peak media transcoding for Sovereign: enable household users to slide physical Blu-ray, DVD, or Audio CD discs into an optical drive and walk away while the server detects media within 10 seconds, rips content to isolated staging, enforces commercial Blu-ray decryption key checks (`KEYDB.cfg`), automatically queries external databases (TMDb, IMDb, MusicBrainz) for metadata and high-resolution cover art, and automatically ejects the tray upon completion or after 3 failed retries with an alert. Furthermore, operate an automated background transcoding queue running during scheduled off-peak night hours (default: 01:00 to 07:00) that converts raw media and disc imports into standardized direct-play formats (H.264/H.265 MP4 with universal stereo/multichannel AAC audio) to eliminate real-time GPU transcoding during family viewing, and pre-generates space-efficient 720p/1080p mobile offline profiles ready for instant, transcode-free download to iPads and smartphones. This transforms physical media collections into streaming-ready, mobile-synchronized digital libraries without requiring desktop ripping software, terminal commands, or daytime GPU contention.

## Stories

- Story 3.1: Optical Drive Detection & Automated Disc Ripping Engine
- Story 3.2: Automated Media Metadata & Artwork Enrichment
- Story 3.3: Off-Peak Transcoding Queue & Universal Direct-Play Pre-Encoding
- Story 3.4: Mobile Offline Profile Generation & iPad Sync

## Requirements & Constraints

- **Optical Media Detection & Polling**: The supervisor daemon must continuously monitor connected SATA and USB optical drives, detecting media insertion (Blu-ray, DVD, Audio CD) within 10 seconds without manual user trigger.
- **Unattended Disc Ripping & Staging**: Unencrypted DVD and Audio CD discs must rip automatically to a staging directory (`/data/media/staging`) out of the box without requiring desktop tools or user intervention.
- **Blu-ray Decryption Key Compliance**: Commercial encrypted Blu-rays must use a user-provided `KEYDB.cfg` uploaded via Settings (or mapped to the ripping runtime) to maintain open-source licensing compliance. If decryption keys are missing or invalid, the disc must safely eject with an explanatory consumer notice (*"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."*) without system crashes or process deadlocks.
- **Auto-Ejection & Retry Safeguards**: The optical tray must automatically eject immediately upon rip completion. If disc reading fails due to scratches or read errors, the system must retry up to 3 times with exponential backoff before safely ejecting the disc, logging the failure, and dispatching a maintenance email alert to the admin email address.
- **Automated Metadata & Cover Art Matching**: Finished rips must be matched against external databases (TMDb, IMDb, MusicBrainz) using disc volume labels, track tables of contents (TOC), or audio fingerprints. Matched title, release year, genres, track names, and high-resolution cover art must be automatically retrieved and stored alongside the media files.
- **Atomic Library Promotion & Scan Triggers**: Once metadata enrichment completes, processed media files must be atomically moved (`os.Rename`) into their designated active library directories (`/data/media/movies`, `/data/media/tv`, `/data/media/music`) and trigger immediate HTTP library scan webhooks to Jellyfin or Navidrome within 5 seconds.
- **Off-Peak Nighttime Transcoding Window**: Heavy batch video transcoding and mobile profile generation must run strictly during scheduled off-peak nighttime hours (default: 01:00 to 07:00). During daytime hours (07:00 to 01:00), heavy batch video transcoding is paused or throttled to keep GPU compute and VRAM available for interactive AI chat and real-time operations.
- **Universal Direct-Play Pre-Encoding**: Ingestion and disc ripping pre-transcode media into standardized H.264/H.265 MP4 files with universal direct-play audio/video compatibility (AAC stereo and multichannel fallback). This eliminates runtime GPU transcoding contention on smart TVs during streaming playback.
- **Mobile Offline Profile Generation**: The encoder must generate an optimized, space-efficient 720p/1080p mobile direct-play profile alongside master video files, enabling instant offline downloads in the Jellyfin mobile app on iPads or phones without real-time on-the-fly server transcoding.
- **Storage Headroom Safety Floor**: All batch transcoding, disc ripping, and file promotion must enforce a 10% disk capacity threshold; operations pause gracefully if free disk space drops below 10% to protect SQLite WAL flushes and host filesystem stability.

## Technical Decisions

- **Containerization & Ripping Runtime**: Automatic Ripping Machine (ARM) pinned to official upstream release `2.24.4` (configured in `/data/state/compose/arm.yaml` via supervisor synthesis), configured with optical device passthrough (`/dev/sr0`, `/dev/sg*`) and volume mounts for `/data` and decryption keys (`/data/state/keys`).
- **Transcoding Engine & Hardware Acceleration**: Batch encoding pipeline built on hardware-accelerated ffmpeg / HandBrake CLI instances. Automatically detects and leverages Intel QuickSync (`/dev/dri`), Nvidia NVENC (`--gpus all`), or VAAPI.
- **Subsystem Architecture & Ownership**:
  - Optical drive detection, udev/sysfs monitoring, and disc lifecycle management live in `/internal/storage` and `/internal/supervisor`.
  - Scheduled off-peak transcoding queue runner and job queue state management live in `/internal/storage` (coordinated with `/internal/supervisor`).
  - Decryption key storage is isolated at `/data/state/keys/KEYDB.cfg`, accessible by ARM and managed via authenticated administrative settings endpoints (`/api/v1/admin/keys`).
- **GPU Resource Arbitration & Scheduling (ADR-003)**:
  - Day/Night scheduling: Interactive AI has daytime priority (07:00–01:00); batch video transcoding executes during the off-peak window (01:00–07:00).
  - Hybrid GPU topologies: In systems with both an integrated GPU (Intel QuickSync) and a discrete GPU (Nvidia), video transcoding is automatically pinned to the iGPU while AI inference routes to the dGPU.
  - Dynamic idle surrender: On single-GPU systems, if daytime playback transcode fallback occurs, the local LLM backend dynamic idle surrender (300s timeout) unloads VRAM to prevent CUDA out-of-memory errors.
- **Storage Layout & Staging Conventions**:
  - Disc ripping staging: `/data/media/staging/discs/:disc_id/`.
  - Active library targets: `/data/media/movies`, `/data/media/tv`, `/data/media/music`.
  - Zero-copy promotion via atomic filesystem `os.Rename` within the unified `/data` mount.
- **External Metadata Integration**:
  - Resilient API clients for TMDb and MusicBrainz with caching, exponential backoff, and local rate-limiting to gracefully handle intermittent network drops.
  - Fallback logic: If metadata match confidence is low, raw title and timestamp metadata are preserved so files remain accessible in Jellyfin/Navidrome for manual identification.
- **Telemetry & Event Dispatch**:
  - Real-time ripping and transcoding progress stream over SSE (`/api/events`) using standard domain action formats (`optical.disc_detected`, `optical.rip_progress`, `optical.disc_ejected`, `transcode.job_started`, `transcode.job_progress`, `transcode.job_completed`).
  - Read failures and missing decryption keys dispatch maintenance alert emails via `/internal/notify` (SMTP).

## UX & Interaction Patterns

- **TelemetryBar Optical Drive Widget**:
  - Pinned bottom TelemetryBar (height: 56px) displays real-time optical drive status:
    `Idle · Ready for disc` → `Disc detected` → `Ripping (X%)` → `Matching metadata` → `Auto-ejecting disc`.
  - Live progress updates delivered asynchronously via SSE every 15s using `aria-live="polite"` so screen readers are informed without disrupting active navigation.
- **Smart Hardware Sharing Schedule Pill**:
  - Dashboard TelemetryBar displays the active hardware schedule mode (e.g., `☀️ Daytime Mode: AI Priority (07:00–01:00)` or `🌙 Nighttime Mode: Batch Transcode (01:00–07:00)`). Clicking opens a direct shortcut to `/settings#gpu`.
- **Smart Hardware Sharing Scheduler Surface (`/settings#gpu`)**:
  - Interactive 24-hour visual timeline bar showing Day (`#3B82F6`) and Night (`#0F172A`) blocks.
  - User-configurable schedule controls allowing customization of the off-peak batch window and task priority allocation.
- **Decryption Key Upload in Settings**:
  - Administrative card in `/settings` allowing file upload of `KEYDB.cfg` for commercial Blu-ray playback with immediate format validation and status confirmation badge.
- **Consumer-Grade Microcopy & Error Handling**:
  - Strictly eliminates raw device paths, SCSI/udev errors, and ffmpeg terminal logs:
    - Missing Blu-ray key: *"This Blu-ray disc requires decryption keys to read. Please upload your key file in Settings, or use an unencrypted disc."*
    - Read retry failure: *"Disc reading failed after multiple attempts. The disc has been safely ejected."*
    - Low disk space: *"Transcoding paused: storage capacity is below 10%. Please free up space to continue."*
- **Accessibility & Interaction Standards**:
  - Full WCAG 2.1 AA compliance with minimum 4.5:1 text contrast and 3:1 heading contrast.
  - All status indications pair semantic colors with distinct micro-shapes (●, ◆, ▲) and descriptive text labels.
  - Interactive controls enforce minimum 44x44px touch targets and high-visibility `2px #2563EB` focus rings.

## Cross-Story Dependencies

- **Story 3.1 (Optical Drive Detection & Disc Ripping Engine)** provides the hardware polling substrate, disc detection, ripping execution, and temporary staging (`/data/media/staging`) required by Stories 3.2 and 3.3.
- **Story 3.2 (Automated Media Metadata & Artwork Enrichment)** processes the raw staged disc files from Story 3.1, enriches them with external metadata and cover art, moves them to `/data/media/{movies,tv,music}`, and triggers media library refresh webhooks.
- **Story 3.3 (Off-Peak Transcoding Queue & Universal Direct-Play Pre-Encoding)** implements the scheduled background batch runner, day/night hardware scheduler, and ffmpeg encoding pipeline that converts raw rips into standardized direct-play master files.
- **Story 3.4 (Mobile Offline Profile Generation & iPad Sync)** builds directly on the Story 3.3 transcoding engine to produce secondary 720p/1080p mobile profiles for instant download in the Jellyfin mobile app.
- **Upstream Foundation**: Depends on Epic 1 for the Go supervisor daemon, SQLite WAL state store, and `/data` storage hierarchy; and Epic 2 for active Jellyfin and Navidrome container deployments, library paths (`/data/media/*`), and library refresh webhook endpoints.
- **Downstream Capabilities**: Interacts with Epic 5 (Local AI & GPU Resource Arbitration) by adhering to daytime AI GPU priority and yielding compute/VRAM during daytime conversational queries.
