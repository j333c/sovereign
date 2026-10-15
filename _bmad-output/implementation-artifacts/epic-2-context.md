# Epic 2 Context: In-House Media Streaming & Staged Ingestion Dropzone

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Provide turnkey in-house media streaming and streamlined household file ingestion for Sovereign: deliver one-click on-demand activation for Jellyfin (video streaming), Navidrome (music and Subsonic API), and Audiobookshelf (audiobooks and chapter sync) pulled strictly from official upstream OCI registries without pre-download bloat; establish container lifecycle supervision with bounded memory limits, volume preservation, and watchdog dampening; integrate interactive ServiceCard and StatusBadge UI controls with accessible overflow actions; and implement a staged dropzone ingestion pipeline supporting both web drag-and-drop (`/drop`) and SMB network shares (`\\sovereign\drop`) with magic bytes validation, atomic file moves, and immediate media library refresh webhooks. This turns the server into an intuitive, high-performance home entertainment hub that runs reliably on consumer hardware without manual pathing or media server maintenance.

## Stories

- Story 2.1: On-Demand Container Lifecycle Engine & Health Probing
- Story 2.2: ServiceCard & StatusBadge Appliance UI Controls
- Story 2.3: In-House Video Streaming Integration (Jellyfin)
- Story 2.4: Music & Audiobook Streaming Integration (Navidrome & Audiobookshelf)
- Story 2.5: Staged Dropzone Ingestion Pipeline & Web Upload Interface

## Requirements & Constraints

- **On-Demand Container Deployment**: Container images must never be pre-downloaded during initial setup; images (`jellyfin/jellyfin`, `deluan/navidrome`, `advplyr/audiobookshelf`) are pulled strictly on-demand from official upstream OCI repositories only after explicit user activation. Image download progress streams to the UI via SSE.
- **Resource Limits & Runtime Boundaries**: Running containers must enforce bounded memory limits (`mem_limit`) and soft memory reservations to prevent media workloads from exhausting host RAM. Containers execute in isolated bridge networks without host Docker socket access.
- **Persistent Volume Preservation**: Deactivating or stopping a feature halts container runtime execution while strictly preserving all user media, configuration, and database volumes in `/data`.
- **Health Probing & Watchdog Dampening**: Feature health status transitions require watchdog debouncing: at least 3 consecutive failed healthcheck probes over a 60-second window before escalating to an Amber or Red degraded state or triggering automated container restart routines.
- **Video Streaming & Hardware Acceleration**: Jellyfin mounts pre-configured library paths at `/data/media/movies` and `/data/media/tv`. Direct play is the primary path to avoid GPU transcode overhead, with automatic fallback to hardware acceleration (Intel QuickSync, Nvidia NVENC, or VAAPI) for legacy formats and bandwidth-constrained remote streams. Standard DLNA and native Jellyfin API discovery must broadcast on the local network.
- **Audio & Audiobook Streaming**: Dual-container architecture isolates music from audiobooks. Navidrome mounts `/data/media/music`, providing Subsonic API support for mobile clients (e.g., Symfonium, Substreamer) and collaborative party playlist playback. Audiobookshelf mounts `/data/media/audiobooks`, preserving chapter markers, narration speed preferences, and multi-device playback position sync.
- **Dual-Path Ingress Routing**: Ingress to media services operates via dedicated ports (`:8096` Jellyfin, `:4533` Navidrome, `:13378` Audiobookshelf) and reverse-proxied subpaths (`/media`, `/music`, `/audiobooks`) managed dynamically via Caddy using path trimming (`handle_path`) and `X-Forwarded-Prefix` headers.
- **Multi-Channel Media Landing Zone**: Users can ingest media via public web drag-and-drop (`/drop`) or local network SMB file sharing (`\\sovereign\drop` / `smb://sovereign.local/drop`), with designated target folders for `movies`, `tv`, `music`, and `audiobooks`.
- **Staged Ingestion Safety & Integrity**: File uploads must stream into isolated staging paths (`/data/drop/incoming/:category/.uploading-:uuid-:filename`). Once upload streams close and `fsync()` completes, files are atomically renamed, inspected for file signature magic bytes to verify media validity, atomically moved (`os.Rename`) into the corresponding `/data/media/:category` directory, and staging files cleaned up. Non-media files are rejected with consumer-friendly error messaging.
- **Instant Library Webhooks**: Successful file promotions to `/data/media` must trigger immediate HTTP library scan webhooks to Jellyfin, Navidrome, or Audiobookshelf within 5 seconds (and at most 60 seconds for bulk network transfers).

## Technical Decisions

- **Upstream Images & Versions**: Strictly use official images and pinned tags: Jellyfin (`jellyfin/jellyfin:12.2.0`), Navidrome (`deluan/navidrome:0.54.5`), Audiobookshelf (`advplyr/audiobookshelf:2.19.5`), routed via Compose v2 templates in `/compose/templates/`.
- **Supervisor Engine & Lifecycle Control**: Handled in `/internal/supervisor` via Docker Compose v2 SDK. Service activation triggers on-demand compose synthesis and container start; deactivation stops containers without volume pruning.
- **Storage Subsystem & Hierarchy**: Handled in `/internal/storage`. Unified POSIX storage layout under `/data`:
  - Active libraries: `/data/media/movies`, `/data/media/tv`, `/data/media/music`, `/data/media/audiobooks`.
  - Ingestion staging: `/data/drop/incoming/{movies,tv,music,audiobooks}`.
  - Temporary upload files: `.uploading-:uuid-:filename`.
  - Zero-copy promotion via atomic filesystem `os.Rename` within the same `/data` mount.
- **Network Topology & Ingress Synchronization**: Containers join `sovereign-ingress` (for Caddy and client routing) and `sovereign-control` (for daemon supervision). `/internal/ingress` uses Caddy Admin API (`http://caddy:2019`) to dynamically register upstream route blocks and strip path prefixes.
- **Library Refresh Webhook Dispatch**: Daemon storage worker maintains authenticated API/webhook triggers for each upstream service: Jellyfin `/Library/Refresh`, Navidrome `/api/scan`, and Audiobookshelf `/api/libraries/:id/scan`.
- **API Contracts & SSE Telemetry**: Service states and lifecycle mutations adhere to OpenAPI 3.1 definitions under `/api/v1/services`. State transitions stream in real-time over SSE (`/api/events`) using `domain.action` naming (e.g., `service.health_changed`, `storage.media_imported`).
- **Conventions & Problem Details**: Service IDs are canonical lowercase slugs (`jellyfin`, `navidrome`, `audiobookshelf`). REST errors follow RFC 7807 Problem Details. Destructive actions (restart, rollback) require HTTP POST/PUT with authenticated session cookies.

## UX & Interaction Patterns

- **ServiceCard Anatomy**: Cards sit on canvas background with white surface (`#FFFFFF`), 1px hairline border (`#E5E7EB`), and 8px border-radius (`rounded-lg`). Each card displays service icon (40x40px), title, tech descriptor, `StatusBadge`, primary metric (e.g., "124 Movies", "Mobile Phone Sync Ready"), and a full-width "Open [Service] UI" action button (38px desktop, 44px mobile) opening in a new tab.
- **StatusBadge Triad**: Semantic indicator pairing color with distinct micro-shapes:
  - Green Circle (●): "Running Healthy" (all probes passing).
  - Amber Diamond (◆): "Attention Required" (degraded, network NAS disconnected, or warming up).
  - Red Triangle (▲): "Stopped / Error" (container failed, stopped, or continuous crash-loop).
- **Overflow Menu (`•••`)**: Accessible 44x44px touch target popover button on each card providing secondary administrative actions: *Restart Service*, *Roll Back to Previous Version*, and *Download Diagnostic Logs* (scrubbed text summary, zero personal data). Traps keyboard focus; closes on `Escape` or click-outside.
- **Standalone Dropzone Interface (`/drop`)**: Public unauthenticated surface accessible directly or via header link. Features a centered `DropzoneCard` (max-width 680px, 12px radius) with category selector pills (`Movies`, `TV`, `Music`, `Audiobooks`), 2px dashed border with active hover glow, multi-file concurrent upload progress bars, and copyable network UNC path (`\\sovereign\drop`).
- **Consumer-Grade Microcopy**: Strictly eliminates technical sysadmin jargon, stack traces, and exit codes:
  - OOM crash: *"Video Streaming paused due to high memory use. Automatically restarting..."*
  - Missing NAS: *"Home NAS storage is unreachable. Please verify that your NAS is powered on and connected to your Wi-Fi."*
  - File Dropzone: *"Drop files here or copy to \\sovereign\drop on your home network."*
- **Accessibility & Touch Standards**: WCAG 2.1 AA compliance across all components, minimum 4.5:1 text contrast, 44x44px mobile touch targets, 2px `#2563EB` visible focus rings, and `aria-live="polite"` for asynchronous upload and status updates.

## Cross-Story Dependencies

- **Story 2.1 (Container Lifecycle Engine)** establishes the on-demand OCI image pulling, Docker Compose v2 synthesis, and watchdog health probing required by Jellyfin (Story 2.3) and Navidrome/Audiobookshelf (Story 2.4).
- **Story 2.2 (ServiceCard & StatusBadge Controls)** delivers the dashboard UI components and SSE event listeners needed to display and control the media streaming services deployed in Stories 2.3 and 2.4.
- **Story 2.3 (Jellyfin Integration)** provisions the video server and storage paths (`/data/media/movies`, `/data/media/tv`) targeted by the dropzone ingestion pipeline (Story 2.5).
- **Story 2.4 (Navidrome & Audiobookshelf Integration)** provisions the audio servers and storage paths (`/data/media/music`, `/data/media/audiobooks`) targeted by the dropzone ingestion pipeline (Story 2.5).
- **Story 2.5 (Dropzone Ingestion Pipeline)** depends on the `/data/media` folder structure from Stories 2.3 and 2.4, and dispatches HTTP library scan webhooks to the running Jellyfin, Navidrome, and Audiobookshelf instances.
- **Upstream Foundation**: Depends on Epic 1 for the Go supervisor daemon, Caddy ingress gateway, `/data` storage pool initialization, Next.js dashboard shell (`AppHeader`), and SQLite WAL state store.
- **Downstream Capabilities**: Provides the media directory structures, direct-play foundation, and library endpoints required by Epic 3 (Physical Disc Automation & Off-Peak Transcoding Queue).
