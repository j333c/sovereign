---
title: Sovereign Home Server
status: final
created: 2026-10-15
updated: 2026-10-15
---

# PRD: Sovereign Home Server

## 0. Document Purpose
This Product Requirements Document (PRD) defines the functional, architectural, and operational requirements for **Sovereign**, an open-source, appliance-grade home server platform designed to bring privacy, simplicity, and self-hosted services to everyday households.

This document serves as the single source of truth for product direction, engineering implementation, UX workflows, and system architecture. Downstream technical specifications, container orchestration models, and hardware evaluations build directly upon this document and its companion technical addendum (`addendum.md`).

---

## 1. Vision
For most households, the promise of the "personal cloud" and "smart home" has resulted in vendor lock-in, recurring subscription fees, privacy compromises, and cloud dependencies that break when the internet disconnects. While powerful open-source self-hosted applications exist (such as Jellyfin, Paperless-ngx, and local LLMs), setting them up and keeping them running requires sysadmin skills, terminal commands, Docker management, and fragile configuration files that ordinary home users cannot maintain.

**Sovereign** bridges this divide by delivering a turnkey, appliance-like home server that is as straightforward to operate as a consumer smart speaker, yet fully sovereign, private, and local. It combines in-house media streaming, automated physical disc digitization, intelligent document archiving, and private local AI knowledge into a unified, resilient appliance. Sovereign protects data integrity through power-loss robustness, manages hardware resources (like GPU VRAM) automatically, and executes background maintenance with zero-touch rollback protection. Users retain complete ownership of their data without ever being forced to open a terminal.

---

## 2. Target User & Journeys

### 2.1 Jobs To Be Done (JTBD)
- **Functional**: Stream movies, TV shows, music, and audiobooks to any household screen or mobile device using existing, well-tested apps.
- **Functional**: Digitize household paperwork and physical mail effortlessly via smartphone or email, with zero manual filing.
- **Functional**: Chat with a private local AI and query household records without uploading sensitive personal documents to third-party cloud LLMs.
- **Emotional / Peace of Mind**: Feel confident that personal photos, financial records, and medical files are stored safely inside the home, immune to corporate terms-of-service changes, cloud leaks, or sudden pricing hikes.
- **Contextual / Maintenance**: Experience a home server that "just works"—resilient against accidental power unplugging, requiring zero command-line maintenance, and updating itself safely without breaking.

### 2.2 Non-Users (v1)
- **Enterprise / Multi-tenant Organizations**: Sovereign is designed specifically for single-household and private prosumer environments; it does not target multi-tenant corporate compliance or enterprise SSO federation.
- **Headless Cloud VPS Operators**: Sovereign is optimized as a local network physical/virtual appliance with direct storage and hardware acceleration; it is not a hosted multi-user SaaS.

### 2.3 Key User Journeys

#### UJ-1: Alex sets up Sovereign and launches Video Streaming
- **Persona & Context**: Alex, a homeowner who wants private family streaming and file archiving but does not want to interact with Linux terminals, SSH, or configuration files.
- **Entry State**: Plugs power and Ethernet cables into the server; sits on the living room sofa with a laptop connected to the home Wi-Fi.
- **Path**:
  1. Alex enters `http://sovereign.local` into a web browser.
  2. The First-Run Setup Wizard guides Alex through network verification, selecting storage (initializing local hard drives or mounting an existing home NAS), and selecting initial features via checkboxes.
  3. Alex enters an email address for important maintenance alerts and clicks "Complete Setup".
  4. The system provisions the selected containers in the background with pre-tuned, update-safe configurations.
- **Climax**: Within seconds, the central dashboard loads. The "Video Streaming" card displays a bright **Green** status indicator with a direct "Open Streaming UI" button.
- **Resolution**: Alex can immediately cast videos or drop movie files into the media folder without manual port mapping or library path tuning.
- **Edge Case**: If local network router settings block mDNS resolution for `sovereign.local`, clear fallback instructions guide Alex to the router's assigned DHCP IP address.

#### UJ-2: Sarah digitizes household paperwork with zero filing friction
- **Persona & Context**: Sarah, managing family bills, insurance policies, and tax receipts, wants paperwork organized without tedious manual tagging or paper filing cabinets.
- **Entry State**: Smartphone in hand while opening physical mail at the kitchen counter, or reviewing digital invoices received via personal email.
- **Path**:
  1. For paper mail, Sarah snaps photos of the insurance policy using a standard mobile document scanner app connected to Sovereign.
  2. For email bills, Sarah forwards the PDF to a dedicated home server email address (or Sovereign auto-polls a designated mailbox via IMAP).
  3. The Sovereign Document Management System (DMS) performs OCR, generates searchable text, and extracts metadata.
  4. When an urgent contract renewal or payment due date is detected, an event is dispatched to Home Assistant and sent to the household notification email.
- **Climax**: Sarah receives a gentle reminder on the family wall tablet: *"Car insurance renewal due in 10 days"*, with a direct link to the archived PDF.
- **Resolution**: Sarah drops the physical letter in the recycling bin; the document is archived, searchable, and secured.
- **Edge Case**: If mailbox credentials change or IMAP connection fails, the Sovereign admin dashboard displays a **Yellow** warning on the DMS card with a clear re-authentication link.

#### UJ-3: Alex inserts a movie disc, streams on TV, and downloads to an iPad for travel
- **Persona & Context**: Alex wants to digitize a collection of Blu-ray and DVD movies and make them available across all home devices and offline during trips.
- **Entry State**: Sovereign hardware equipped with an optical drive; household equipped with a living room Smart TV and an iPad for travel.
- **Path**:
  1. Alex slides a Blu-ray movie into the server's optical drive and walks away.
  2. Sovereign automatically detects the disc, initiates a background rip, matches the movie against external databases (TMDb/IMDb) for titles, cover art, and chapter markers, and automatically ejects the disc upon completion.
  3. In the early morning hours, the background transcoding queue processes the import, producing an in-house master and an optimized, space-efficient iPad mobile profile.
  4. Before heading to the airport, Alex opens the standard Jellyfin app on the iPad and taps "Download" for offline playback.
- **Climax**: Alex watches the downloaded movie smoothly on the airplane with zero buffering, while the rest of the family streams from the living room Smart TV app simultaneously.
- **Resolution**: 100% automated workflow without manual video encoding software or command-line tools.
- **Edge Case**: If an unidentifiable or scratched disc is inserted, Sovereign ejects the disc after three read attempts and logs an informational alert in the admin dashboard.

#### UJ-4: Family queries household paperwork with Local AI (RAG)
- **Persona & Context**: Alex needs to know the exact deductible on the household home insurance policy while discussing repairs with a contractor.
- **Entry State**: Alex opens a mobile browser on a smartphone connected to the home network.
- **Path**:
  1. Alex navigates to Sovereign's Local AI interface.
  2. Alex types: *"What is our current deductible for storm damage on our home insurance?"*
  3. The local LLM queries the DMS index directly via RAG—retrieving the relevant paragraphs from the policy document without any manual document uploading.
  4. The chat interface responds within seconds: *"Your storm damage deductible is $500, according to page 4 of Policy #98231 renewed last month."* A clickable citation link opens the exact PDF page.
- **Climax**: Alex obtains verified, instant answers derived directly from personal family records without sending any private data to cloud servers.
- **Resolution**: Zero redundant data storage; the DMS remains the authoritative repository.
- **Edge Case**: If the local AI model is unloaded from VRAM to make way for high-priority video transcoding, the interface displays a brief "Starting model..." status for 5 seconds before answering.

#### UJ-5: Power outage recovery and safe automated container updates
- **Persona & Context**: The household experiences an abrupt electrical power outage during a thunderstorm while media is streaming and background tasks are running.
- **Entry State**: The server experiences a sudden, dirty power cutoff. Power is restored 30 minutes later.
- **Path**:
  1. The server hardware reboots automatically upon power restoration.
  2. Sovereign's power-loss resilient storage layer mounts filesystems cleanly, verifies database journal integrity, and launches the central admin daemon.
  3. All enabled service containers initialize in their guarded environments and pass internal health checks.
  4. The admin interface becomes accessible with all services returning to **Green**.
  5. Later that week, an upstream container release is detected. Sovereign pulls the new image at the scheduled 3:00 AM window, snapshots the configuration volume, and starts the new container.
- **Climax**: If an updated container crashes or fails its health check within 3 minutes, Sovereign automatically rolls back to the previous image tag and configuration snapshot, alerting the user.
- **Resolution**: Zero database corruption, zero manual troubleshooting, and zero maintenance anxiety.
- **Edge Case**: If a user prefers to revert an update manually, a single click on "Roll back to previous version" in the admin dashboard restores the prior release instantly.

---

## 3. Glossary

- **Sovereign Server**: The physical or virtual machine running the Sovereign OS platform and its orchestrated service containers.
- **Admin Interface**: The lightweight, fast-booting central web application (`http://sovereign.local`) used for low-level system configuration, feature toggles, health monitoring, and updates.
- **Feature (Service)**: An isolated, containerized application component (e.g., Video Streaming, Audio Streaming, DMS, Local AI) managed by Sovereign.
- **Traffic-Light Indicator**: The standardized three-state status displayed on each feature card:
  - **Green**: Healthy, running, and all internal health checks passing.
  - **Yellow**: Degraded, attention required (e.g., pending setup, mailbox sync warning, optical drive read retry).
  - **Red**: Stopped or failed health checks.
- **Landing Zone**: A local network drop folder (SMB/NFS) and web upload component allowing users to drop media files for automated metadata enrichment and database ingestion.
- **Optical Disc Automation**: The background subsystem that detects inserted Blu-ray, DVD, or Audio CD media, executes unattended ripping and metadata tagging, and auto-ejects the disc.
- **Transcoding Queue**: The scheduled background worker that converts raw media imports into standardized in-house streamable files and optimized offline mobile profiles.
- **Local AI Subsystem**: The privacy-first, offline local artificial intelligence architecture comprising two distinct tiers: an end-user conversational application layer (Open WebUI) and an underlying high-performance LLM inference backend (candidate: `llama.cpp` / `llama-server`).
- **LLM Inference Backend**: The managed engine responsible for local GGUF model storage, CPU/GPU tensor layer offloading, dynamic VRAM lifecycle management, and serving internal OpenAI-compatible endpoints (`/v1`).
- **RAG (Retrieval-Augmented Generation)**: The architecture whereby the Local AI directly queries the DMS repository to answer user questions using verified private records.
- **GPU Arbitration**: The resource governor that balances GPU memory and compute between local LLM execution and real-time hardware-accelerated video transcoding.
- **Rollback Engine**: The automated and manual mechanism that preserves container image versions and volume snapshots, reverting immediately if an update fails health checks.

---

## 4. Features & Functional Requirements

### 4.1 Central Admin Platform & Lifecycle Management
**Description**: A fast-booting, low-overhead web application that serves as the single administrative pane of glass for the home user. Realizes UJ-1 and UJ-5. It provides a guided First-Run Wizard, visual feature toggles, system health indicators, and update controls. Power users can inspect advanced settings, but standard users never need to leave this interface.

#### FR-1: First-Run Setup Wizard
The system can guide a first-time user through initial configuration on first boot at `http://sovereign.local` (or DHCP IP). Realizes UJ-1. [ASSUMPTION: Standard consumer home routers allow mDNS multicast traffic (.local domain resolution); for routers with mDNS disabled, DHCP IP fallback is sufficient.]
- **Consequences (testable)**:
  - System advertises `sovereign.local` via mDNS on the local subnet within 30 seconds of network interface initialization.
  - Wizard prompts for network configuration (DHCP default, static IP option, hostname), storage selection (local disk initialization or NAS NFS/SMB mount), feature selection checkboxes, and notification email address.
  - Completing the wizard transitions the system into the operational dashboard without requiring a manual reboot.
- **Out of Scope**: Multi-tenant user permission hierarchies or corporate directory (Active Directory/LDAP) setup in the wizard.

#### FR-2: Feature Activation & Container Lifecycle
The user can activate, deactivate, and view the status of software features with a single click. Realizes UJ-1.
- **Consequences (testable)**:
  - Activating a feature pulls the designated official OCI container image in the background and starts it with guarded network and persistent storage bindings.
  - Deactivating a feature stops and removes the container runtime while preserving user data and configuration volumes.
  - Each feature card presents a direct link to the service's web UI once the status reaches **Green**.
  - Feature health state is displayed via a standardized traffic-light indicator (Green / Yellow with explanatory message / Red).

#### FR-3: Automated Maintenance & Rollback Engine
The system can execute scheduled background updates with automated failure rollback and provide manual one-click rollbacks. Realizes UJ-5.
- **Consequences (testable)**:
  - System checks for upstream image updates and executes unattended updates during a user-configured maintenance window (default: 3:00 AM).
  - Before an update is applied, the system snapshots container configuration files and records the previous image digest.
  - If a newly updated container fails health checks within 180 seconds or enters a crash-loop state, the system automatically terminates it, restores the prior image digest and configuration snapshot, sets the indicator to **Yellow**, and dispatches an alert email.
  - The Admin Interface displays a "Roll back to previous version" action button for any running feature.

#### FR-4: Power-Loss Resilience & Boot Diagnostics
The system can recover cleanly from abrupt power loss without file system corruption or data fragmentation. Realizes UJ-5.
- **Consequences (testable)**:
  - Storage filesystems utilize write-ahead journaling or transactional copy-on-write mechanisms ensuring clean remounts after ungraceful shutdowns.
  - Database containers utilize WAL (Write-Ahead Logging) mode to prevent SQLite or PostgreSQL corruption during dirty power cuts.
  - During boot, the Admin Interface initializes within 15 seconds of kernel boot and displays real-time startup progress and storage health checks if post-crash recovery routines are executing.

---

### 4.2 Storage, Ingestion & Media Hardware Subsystem
**Description**: The storage and hardware abstraction layer managing local storage pools, external NAS connections, media landing zones, optical drive automation, and scheduled transcoding. Realizes UJ-1, UJ-3.

#### FR-5: Storage Mount Management
The user can configure media and document storage targets using internal physical disks or external network storage. Realizes UJ-1.
- **Consequences (testable)**:
  - Supports formatting and mounting internal SATA/NVMe disks.
  - Supports mounting external NFS and SMB/CIFS network shares with credentials.
  - Gracefully handles temporary NAS disconnections by setting affected feature cards to **Yellow** with an informative status (e.g., *"NAS storage unreachable"*) rather than crashing container runtimes.

#### FR-6: Media Landing Zone Drop
The user can copy media files into the system via a network shared folder or web-based drag-and-drop interface. Realizes UJ-1, UJ-3.
- **Consequences (testable)**:
  - System exposes an SMB network share (`\\sovereign\drop` or `smb://sovereign.local/drop`) and a drag-and-drop file upload target in the Admin Interface.
  - The Landing Zone provides designated target folders (`drop/movies`, `drop/tv`, `drop/music`, `drop/audiobooks`) and automated file extension / MIME-type inspection to route loose files to their designated media directories.
  - Triggers the corresponding media server library scan and metadata retrieval within 60 seconds of file transfer completion.

#### FR-7: Optical Disc Automated Ingestion
The system can detect inserted optical media (Blu-ray, DVD, Audio CD), rip media streams, retrieve external metadata, and eject the disc automatically. Realizes UJ-3. [ASSUMPTION: The server hardware contains a compatible SATA or USB optical drive with read support for CD, DVD, and Blu-ray media.]
- **Consequences (testable)**:
  - Optical drive polling detects disc insertion within 10 seconds.
  - Unencrypted DVD and Audio CD discs rip to a temporary staging volume out of the box without user intervention.
  - Commercial Blu-ray discs require a user-provided decryption key directory (e.g. `KEYDB.cfg` / MakeMKV integration) to remain strictly compliant with open-source licensing; unkeyed encrypted discs are gracefully rejected and ejected.
  - Queries external databases (TMDb, IMDb, MusicBrainz) using disc fingerprints or volume labels to populate title, year, track, and artwork metadata.
  - Ejects the optical tray upon completion and moves processed media to the active library folder.
  - If disc reading fails after 3 retries, ejects the disc, logs the failure, and triggers an email alert.

#### FR-8: Scheduled Transcoding Queue
The system can execute scheduled background video transcoding to generate standard in-house streaming versions and offline mobile profiles. Realizes UJ-3.
- **Consequences (testable)**:
  - Scheduled queue runs during off-peak hours (default: 01:00 to 07:00).
  - Encodes raw disc rips into standardized H.264/H.265 MP4 files with universal direct-play audio/video compatibility, ensuring modern Smart TVs direct-stream content without requiring on-the-fly live transcoding during streaming playback.
  - Automatically generates an optimized, space-efficient 720p/1080p mobile profile for offline synchronization to tablets and smartphones.

---

### 4.3 Media Streaming Services (Video & Audio)
**Description**: Out-of-the-box streaming services for movies, series, personal music libraries, party playlists, and audiobooks. Realizes UJ-1, UJ-3.

#### FR-9: In-House Video Streaming
The system can serve high-definition video streams across local and remote home devices using pre-configured Jellyfin container instances. Realizes UJ-1, UJ-3.
- **Consequences (testable)**:
  - Supports direct streaming (primary path for standardized library media, bypassing runtime GPU transcode overhead) and real-time hardware-accelerated transcoding fallback (Intel QuickSync, Nvidia NVENC, VAAPI) for legacy formats or bandwidth-constrained remote connections.
  - Exposes standard DLNA and Jellyfin API endpoints, enabling immediate discovery by Smart TV apps, Apple TV, Android TV, Roku, and mobile clients.
  - Pre-configures library paths to Sovereign storage locations without requiring manual path entry.
- **Out of Scope**: Building custom Sovereign video player apps; users rely entirely on existing open-source and ecosystem clients.

#### FR-10: Audio Streaming, Party Playlists & Audiobooks
The system can manage and stream personal music libraries, collaborative party playlists, and audiobook collections via a dual-container architecture. Realizes UJ-3.
- **Consequences (testable)**:
  - Deploys **Navidrome** as the primary music server, exposing a central web interface for party playlist creation and continuous playback, plus Subsonic API support for external clients.
  - Deploys **Audiobookshelf** as a dedicated audiobook and podcast backend supporting chapter markers, multi-device playback position sync, and narration speed adjustments.
  - Compatible with standard mobile audio apps (e.g., Symfonium, Substreamer, Plappa, Audiobookshelf mobile).

---

### 4.4 Document Management & Smart Alerts
**Description**: Secure, centralized document archiving with OCR, multi-channel intake, and automated alerting for critical contractual and financial obligations. Realizes UJ-2.

#### FR-11: Centralized Document Management
The system can store, OCR, categorize, and index household documents via an integrated Paperless-ngx container. Realizes UJ-2.
- **Consequences (testable)**:
  - Performs automated OCR on incoming PDF, PNG, JPEG, and TIFF files.
  - Generates full-text search indexes allowing queries across text content, tags, correspondents, and dates.
  - Stores all documents in standard, non-proprietary filesystem directory structures.

#### FR-12: Multi-Channel Document Ingestion
The user can ingest documents via mobile camera scanning apps and automated email polling. Realizes UJ-2. [ASSUMPTION: The household has an active email account supporting standard IMAP/SSL connections for automated invoice retrieval.]
- **Consequences (testable)**:
  - Compatible with standard Paperless mobile companion apps on iOS and Android.
  - Supports IMAP mailbox polling with user-supplied credentials, automatically downloading and archiving PDF attachments and marking emails as processed.

#### FR-13: Smart Home & Notification Event Bridge
The system can dispatch urgent alerts to Home Assistant and email when documents contain critical deadlines or renewal dates. Realizes UJ-2.
- **Consequences (testable)**:
  - Scans document metadata and OCR text using a lightweight deterministic regex parser (for dates and payment totals), supplemented by a nightly off-peak LLM extraction pass during the nighttime batch window.
  - Exposes a Home Assistant webhook and MQTT event publisher that fires when an actionable deadline is within a configurable threshold (e.g., 14 days).
  - Sends a consolidated email digest to the admin email address detailing impending deadlines.

---

### 4.5 Local AI & Document Knowledge (RAG)
**Description**: Fully local, private conversational AI interface with direct document retrieval (RAG), dedicated managed LLM backend administration, and active hardware resource arbitration. Realizes UJ-4.

#### FR-14: Browser-Based Local AI Chat Interface
The user can interact with local AI models through a pre-integrated Open WebUI interface accessible via standard web browsers, served by the managed LLM inference backend. Realizes UJ-4.
- **Consequences (testable)**:
  - Provides conversational chat, persona selection, and chat history.
  - Accessible on desktop and mobile web browsers without installing custom native software.
  - Connects out-of-the-box to the internal managed `llama.cpp` backend container operating 100% offline.
  - Provides a pre-configured standard system customization prompt that sets the response language matching the user's configured locale and embeds country-specific regulations, legal frameworks, and cultural habits (e.g., standard billing periods, cancellation notice windows, document formatting conventions).

#### FR-15: Managed LLM Inference Backend & Turnkey Default Setup
The system can manage a dedicated high-performance LLM inference engine backend (candidate: `llama.cpp` / `llama-server`) with turnkey default configuration, automated hardware acceleration detection, and appliance-grade administrative controls. Realizes UJ-4.
- **Consequences (testable)**:
  - **Turnkey Default Setup**: On initial provisioning of the Local AI subsystem, Sovereign automatically deploys a pre-configured `llama.cpp` backend service with a verified, balanced default quantized GGUF model (e.g., standard Q4_K_M model such as Qwen 2.5 7B Instruct or Llama 3.2), autodetects hardware acceleration capabilities (Nvidia CUDA, Vulkan, Intel SYCL, or CPU fallback), enables FlashAttention (`-fa`) by default to reduce KV cache memory overhead by up to 50%, and establishes internal connectivity with zero manual configuration.
  - **Model Storage & Administration**: Dedicated local volume (`/data/models`) stores GGUF models. The Admin Interface provides an appliance administration surface to:
    - View the currently loaded active model, FlashAttention status, and memory footprint (RAM/VRAM).
    - Select and switch active models from local storage with one click.
    - Trigger background downloads of recommended default or alternative curated GGUF models with progress reporting. Model acquisition delegates to backend-native mechanisms where available; otherwise the system enforces atomic staging (`.downloading` staging file, SHA-256 validation, and atomic rename to `.gguf`) with resumable HTTP range downloads and UI retry controls.
  - **Backend Resource & Execution Tuning**: Admin interface allows users to adjust:
    - GPU Layer Offloading (`-ngl` / `--n-gpu-layers`): Auto-calculated default with manual override slider (0 to 100% offload).
    - Context Window Size (`-c`): Configurable presets (2048, 4096, 8192 tokens) balanced against memory constraints and protected by FlashAttention.
    - CPU Thread Allocation (`-t`): Auto-pinned to optimal physical core count; on zero-GPU / CPU-only hosts, threads are automatically capped to `max(1, physical_cores - 2)` to guarantee uninterrupted OS, DNS, and storage operations, accompanied by a dedicated "CPU-Optimized Mode" UI indicator and 3B model recommendation.
  - **Operational Lifecycle & Health**:
    - Reports granular backend status: Operational/Ready, Warm-up/Loading weights into memory, Inferencing, Idle/Unloaded, Error.
    - Exposes "Reload Backend" and "View Inference Logs" directly in the appliance settings without terminal access.
    - Supports configurable dynamic idle surrender timeout to unload model weights when idle and free VRAM for video transcoding.

#### FR-16: Single-Source-of-Truth RAG
The local AI system can query the document management system directly to answer user questions without requiring duplicate document uploads. Realizes UJ-4. [ASSUMPTION: Paperless-ngx API and metadata provide adequate search and text extraction performance for direct local LLM retrieval without requiring a separate standalone vector database for small-to-medium household libraries (< 10,000 documents).]
- **Consequences (testable)**:
  - Queries execute directly against the DMS search API / vector embeddings.
  - System enforces zero redundant data storage: documents are neither duplicated into the chat UI database nor stored in multiple locations.
  - Chat responses cite the specific document title and page number used to generate the answer.

#### FR-17: Standardized Shared AI Platform API
The system can expose an internal OpenAI-compatible API endpoint directly from the managed LLM backend and automatically configure installed features to leverage local AI. Realizes UJ-4.
- **Consequences (testable)**:
  - Exposes an internal HTTP endpoint (e.g., `http://sovereign-ai-backend:8080/v1` or reverse-proxied internally) accessible to Open WebUI, Paperless-ngx, and other Sovereign feature containers.
  - When activating a feature that supports AI integrations (e.g., DMS auto-tagging or document analysis), Sovereign automatically provisions the internal endpoint URL and credentials into the feature container's configuration.

#### FR-18: Active GPU & VRAM Arbitration & Day/Night Scheduling
The system can dynamically manage GPU memory and compute resources across the managed LLM backend (llama.cpp) and media transcoders to prevent inference from starving or crashing video playback sessions, reserving daytime GPU availability for interactive user requests. Realizes UJ-3, UJ-4. [ASSUMPTION: Intel QuickSync (via 8th gen+ Core CPUs) or Nvidia GPUs with NVENC support are the primary hardware acceleration targets for video transcoding.]
- **Consequences (testable)**:
  - **Direct Play Pre-encoding as Primary Isolation**: Ripped media and dropzone video files are pre-transcoded during off-peak queue processing into universal Direct Play formats (H.264/H.265 + AAC), ensuring Smart TVs direct-stream without triggering runtime GPU transcoding, thereby avoiding streaming/AI resource contention during normal playback.
  - **Daytime Priority for AI**: During daytime hours (default: 07:00 to 01:00), the system actively reserves GPU compute and VRAM for interactive user AI queries. Heavy background video transcoding, disc rips, and non-urgent batch tasks are withheld or throttled.
  - **Nighttime Batch Window**: Heavy batch video transcoding and background OCR processing run during scheduled off-peak nighttime/early morning hours (default: 01:00 to 07:00).
  - **Visible Admin UI Setting**: The Admin Interface provides a clear, user-facing schedule setting where users can inspect and customize daytime vs. nighttime GPU allocation and task priorities.
  - **Resource Arbitration with llama.cpp**:
    - If a single GPU is shared between local LLMs and real-time streaming:
      - LLM backend enforces dynamic idle model unloading / process surrender (timeout configurable to 5 minutes or less) to release VRAM back to the system.
      - In daytime mode, when waking the model from idle surrender, the backend and reverse proxy return `status: "warming_up"` with extended timeout guards (>= 60s) and emit an in-UI notification (*"Waking local AI model into memory..."*), preventing gateway timeouts and user confusion.
      - System reserves minimum VRAM overhead ensuring video encoder buffers remain available during concurrent usage, adjusting `-ngl` offload layer budget accordingly.
    - In systems equipped with hybrid GPUs (e.g., Intel iGPU with QuickSync + discrete Nvidia GPU), video transcoding is automatically pinned to the iGPU while LLM inference is pinned to the dGPU.

---

### 4.6 Localization, Governance & Web Presence
**Description**: Multilingual accessibility, public documentation, project marketing, and compliance.

#### FR-19: Multi-Language UI & Documentation
The system can display all admin interfaces, setup wizards, and user documentation in four primary launch languages.
- **Consequences (testable)**:
  - Admin UI provides an instant language selector for English (EN), German (DE), Spanish (ES), and French (FR).
  - Setup Wizard automatically detects browser language preferences.
  - Bundled short guides and comprehensive documentation are fully translated across all four languages.

#### FR-20: Project Marketing Site & GitHub Presence
The project can maintain a public marketing website and an open-source GitHub repository detailing project creation and attribution.
- **Consequences (testable)**:
  - Marketing website highlights feature capabilities, minimum hardware guidelines, and directs users to the GitHub repository.
  - GitHub README documents project architecture, acknowledges creation via the BMad Method, transparently notes AI assistance, and gives prominent attribution and license compliance notices for all bundled open-source projects.

#### FR-21: Minimum Hardware Specification Guidance & Future Hardware Roadmap
The platform can publish concrete minimum hardware profiles by feature tier and prepare for future hardware appliance sales.
- **Consequences (testable)**:
  - Documentation and marketing site specify 3 distinct hardware tiers:
    1. *Light Tier* (DMS + Audio Streaming): Dual-core x86/ARM64, 4 GB RAM, 64 GB storage.
    2. *Media Tier* (Video Streaming + DMS + Audio + Disc Ingestion): Quad-core x86 with Intel QuickSync / iGPU, 8 GB RAM, 2 TB+ storage.
    3. *AI & Power Tier* (Media + DMS + Local 7B/8B LLM RAG): 6+ core x86, 16–32 GB RAM, discrete GPU (8 GB+ VRAM) or modern unified memory APU, 4 TB+ storage.
  - Roadmap reserves architecture hooks for turnkey hardware appliance ordering via the marketing website.

---

### 4.7 Cross-Cutting Non-Functional Requirements (NFRs)

#### NFR-1: Reliability & Power Resilience
- All persistent container data and configuration state must survive simulated abrupt power loss with zero database corruption (WAL journaling enforced).
- System must automatically perform filesystem integrity checks during boot and remount storage pools without requiring manual intervention.

#### NFR-2: Performance & Responsiveness
- The Admin Interface web dashboard must achieve Time-to-Interactive (TTI) within 2.0 seconds of request on a local gigabit network.
- Cold boot to operational Admin Interface must complete within 60 seconds on recommended hardware.
- Local AI first-token latency for RAG queries must remain under 3.5 seconds on recommended GPU hardware.

#### NFR-3: Security, Sandboxing & Zero Telemetry
- All features must execute in isolated OCI containers with restricted host privileges (read-only root where supported, non-root user execution where supported by upstream images).
- Sovereign must enforce a strict zero-telemetry policy: no usage data, IP addresses, or document statistics are transmitted to external servers.
- All inter-container communication must be routed over dedicated internal bridge networks; only required external ports (e.g., HTTP 80/443, DLNA, SMB) are exposed to the LAN.

#### NFR-4: Observability & Self-Healing
- Each feature must implement continuous internal health checks (evaluating process status, database responsiveness, and API endpoints).
- System must support automated recovery: if a container becomes unresponsive, the runtime attempts up to 3 automatic restarts before flagging the feature as **Yellow** or **Red** and dispatching an alert email.

---

## 5. Non-Goals (Explicit)

- **No Custom Client Applications**: Sovereign will not build, publish, or maintain native client applications for iOS, Android, Smart TVs, or desktop OS. All client interactions rely exclusively on existing open-source and ecosystem clients (e.g., Jellyfin client apps, Paperless mobile apps, Subsonic apps, and modern web browsers).
- **No Proprietary Cloud Dependencies or Mandatory Accounts**: The server must never require an external cloud account, central server ping, or subscription license to function. Remote access (if desired) is handled via standard VPN/reverse proxy protocols, not a proprietary relay service.
- **No Manual Configuration File Editing for Core Workflows**: Standard users must never be forced to edit `.yaml`, `.json`, `.toml`, or `.env` files via command line to install, update, or configure core services.
- **No Proprietary Source-Available Re-Licensing**: Unlike platforms that moved from open-source to restrictive source-available licenses, Sovereign commits to remaining strictly open source under permissive/standard FOSS licensing.

---

## 6. MVP Scope

### 6.1 In Scope (v1)
- Core Admin Web Interface with First-Run Wizard and mDNS discovery (`sovereign.local`).
- One-click container lifecycle management for Jellyfin (Video), Navidrome/Audiobookshelf (Audio), Paperless-ngx (DMS), and Local AI (Open WebUI user interface + managed `llama.cpp` backend).
- Turnkey default setup for LLM backend with pre-configured default GGUF model, autodetected hardware acceleration, and internal OpenAI-compatible API.
- Appliance Admin UI controls for LLM backend administration (model selection, GPU layer offloading, context length, idle surrender).
- Traffic-light status monitoring (Green/Yellow/Red) with automated failure rollback and one-click manual rollback.
- Storage setup: internal drive pool or external NAS mount.
- Media Landing Zone drop folder (SMB + Web upload).
- Automated Optical Disc Ingestion pipeline (insert-and-walk-away rip, metadata query, auto-eject).
- Scheduled transcoding queue for in-house and iPad mobile profiles.
- Direct RAG integration between Local AI and DMS (single source of truth).
- Internal shared OpenAI-compatible API endpoint for container features.
- Basic GPU arbitration rules (VRAM reserve, keep-alive timeouts, split iGPU/dGPU routing).
- Full localization in English, German, Spanish, and French.
- Public marketing website, comprehensive documentation, and GitHub repository with full BMad and FOSS attribution.

### 6.2 Out of Scope for MVP (Deferred to v2 / Future)
- Direct e-commerce purchasing of pre-assembled Sovereign hardware appliances via the website (marketing site links to self-build hardware specs in v1).
- Native Matter / Apple HomeKit zero-conf bridge for smart home hub pairing [NOTE FOR PM: Revisit once Matter smart home standards stabilize].
- Automated multi-node clustering or distributed storage across multiple Sovereign physical servers.
- Complex multi-user quota and bandwidth throttling management.

---

## 7. Success Metrics & Counter-Metrics

> **Measurement Methodology Note**: In strict adherence to **NFR-3 (Zero Telemetry)**, Sovereign transmits zero diagnostic, usage, or completion pings to external servers. All primary and secondary success metrics (SM-1 through SM-6) are measured and validated via synthetic end-to-end test harnesses, automated CI fault-injection suites, and structured, opt-in local usability lab sessions.

### Primary Success Metrics
- **SM-1 (Setup Completion Rate)**: $\ge 90\%$ of first-time home users complete the First-Run Wizard from power-on to their first active **Green** feature in under 10 minutes without consulting external command-line guides. Validates FR-1, FR-2.
- **SM-2 (Zero Terminal Operation)**: $100\%$ of standard feature installations, updates, and rollbacks execute cleanly through the Admin Interface with zero required shell or terminal commands. Validates FR-2, FR-3.
- **SM-3 (RAG Answer Accuracy & Integrity)**: $\ge 95\%$ of test queries against the DMS via Local AI cite correct document sources and page numbers with zero cross-document hallucinations. Validates FR-14, FR-15, FR-16.
- **SM-4 (Dirty-Shutdown Recovery)**: $100\%$ clean boot and database integrity recovery after 20 consecutive simulated ungraceful power disconnections during active transcode/write operations. Validates FR-4.

### Secondary Success Metrics
- **SM-5 (Optical Ingestion Success)**: $\ge 85\%$ of unencrypted/keyed commercial discs ripped, matched with metadata, and ejected without manual intervention. Validates FR-7.
- **SM-6 (Update Rollback Reliability)**: $100\%$ of simulated failed container updates trigger automatic rollback within 180 seconds, restoring the system to operational state. Validates FR-3.

### Counter-Metrics (Do NOT Optimize)
- **SM-C1: App Store Catalog Sprawl**: Do not optimize for the total number of installable apps. A catalog of 100 poorly maintained, unvetted containers degrades the appliance experience. Prioritize deep, robust integration of the core curated stack over raw quantity. Counterbalances feature count growth.
- **SM-C2: Aggressive Cold Boot Time at the Expense of Disk Verification**: Do not optimize boot time below 10 seconds if it requires skipping filesystem journal verification or database consistency checks. Power-loss resilience always trumps raw boot speed. Counterbalances FR-4.
- **SM-C3: Maximum Continuous LLM VRAM Retention**: Do not optimize for instant LLM response latency by keeping model weights permanently pinned in VRAM if doing so starves video transcoding streams. Counterbalances FR-15, FR-18.

---

## 8. Open Questions

1. **Optical Drive Encryption & Keys**: Commercial Blu-ray discs often require updated AACS/BD+ keyrings (e.g. via MakeMKV or libaacs). How should Sovereign handle initial decryption key setup while remaining strictly compliant with open-source licensing?
2. **Dynamic DNS & Remote Access Strategy**: While local network operation is the primary design target, users frequently desire streaming away from home. Should Sovereign bundle a zero-config WireGuard / Tailscale container, or leave remote access entirely to user router configuration in v1?
3. **Smart Home Auto-Discovery**: What is the most seamless protocol to enable Apple Home and Google Home discovery without requiring Apple MFi certification or complex bridge maintenance?

---

## 9. Assumptions Index
- `[ASSUMPTION: §4.1 FR-1]`: Standard consumer home routers allow mDNS multicast traffic (`.local` domain resolution); for routers with mDNS disabled, DHCP IP fallback is sufficient.
- `[ASSUMPTION: §4.2 FR-7]`: The server hardware contains a compatible SATA or USB optical drive with read support for CD, DVD, and Blu-ray media.
- `[ASSUMPTION: §4.4 FR-12]`: The household has an active email account supporting standard IMAP/SSL connections for automated invoice retrieval.
- `[ASSUMPTION: §4.5 FR-16]`: Paperless-ngx API and metadata provide adequate search and text extraction performance for direct local LLM retrieval without requiring a separate standalone vector database for small-to-medium household libraries (< 10,000 documents).
- `[ASSUMPTION: §4.5 FR-18]`: Intel QuickSync (via 8th gen+ Core CPUs) or Nvidia GPUs with NVENC support are the primary hardware acceleration targets for video transcoding.
