# Sovereign PRD Addendum: Technical Notes & Candidate Architecture

This addendum preserves technical depth, architectural ideas, candidate software components, and downstream operational details volunteered during discovery.

## 1. Candidate Software Stack

- **Media Streaming (Video & Audio)**:
  - Video Streaming candidate: [Jellyfin](https://jellyfin.org/) (open-source, official containers, DLNA, wide client ecosystem).
  - Audio Streaming / Audiobooks candidates: Navidrome (lightweight Subsonic API server for music/playlists), Audiobookshelf (dedicated audiobook & podcast server with multi-device sync), or Jellyfin music support.
- **Local AI & Knowledge (RAG)**:
  - LLM Interface candidate: [Open WebUI](https://github.com/open-webui/open-webui) (clean chat UI, multi-model support, built-in RAG / web search integration).
  - Local Inference Backend candidate: Ollama, vLLM, or llama.cpp container backend.
- **Document Management System (DMS)**:
  - Candidate: [Paperless-ngx](https://docs.paperless-ngx.com/) (OCR, tagging, email ingestion inbox, smartphone scan app support).
  - AI Document Analyzer: Custom cron/daemon or integration hook (e.g., querying local LLM on newly ingested documents to extract payment due dates, contract renewals, and dispatch alerts).
- **Home Automation Integration**:
  - Home Assistant webhook / MQTT integration to deliver urgent notifications (e.g. bills due, contract expiry).

## 2. Infrastructure & Runtime Mechanics

- **Containerization**:
  - Official OCI / Docker images from upstream projects.
  - Guarded, isolated network/runtime environments.
  - Zero-configuration auto-setup for standard users; transparent, update-safe configuration volumes for power-user tweaks.
- **Fast-Boot & Power Resilience**:
  - Central management web UI must load rapidly post-boot.
  - Power-loss resilience: robust filesystem journaling / read-only root OS with persistent data mounts to survive abrupt power cuts.
- **Storage Subsystems**:
  - Support for local internal storage (single or multi-drive pools) and networked storage (NFS / SMB / NAS mounts).
- **GPU Resource Management & Concurrency**:
  - Shared GPU arbitration between local AI inference (VRAM-heavy) and video transcoding (NVENC/VAAPI/QuickSync encoder sessions) to prevent OOM errors or performance collapse.
- **Automation & Background Queues**:
  - Optical drive disc import pipeline (auto-ripping when Blu-ray / DVD is inserted).
  - Nightly transcoding worker (generating in-house optimized streamable files and iPad-ready offline download files).
  - Nightly document scan worker (extracting contract dates and payment obligations).

## 3. Commercial & Go-To-Market Concepts

- **Hardware Tiering & Sales**:
  - Provide public minimum hardware specification guidelines by active feature set.
  - Future revenue model: Offer pre-assembled, turnkey Sovereign home server hardware appliances ordered directly via the project website.
- **Project Brand & Marketing**:
  - Public marketing website directing users to documentation and GitHub repository.
  - GitHub repository acknowledging the BMad methodology, transparent AI-assisted development, full open-source licensing compliance, and attribution for all bundled upstream tools.

## 4. Competitive Intelligence & GPU Arbitration Mechanics

### Landscape Highlights
- **Umbrel (umbrelOS)**: Polished consumer UX with hardware sales (Umbrel Home), but faced backlash over moving away from permissive FOSS licensing; storage management and GPU contention remain friction points.
- **CasaOS / ZimaOS**: Lightweight, strong iGPU support on Zima hardware, but shallow container isolation and weak dirty-shutdown recovery.
- **Cosmos Server**: Strong Docker security and reverse proxy, but lacks consumer-grade simplicity.

### Key Technical Pitfall: The GPU / VRAM Collision
- **Problem**: In consumer setups with a single shared GPU (or unified iGPU), local LLM backends (like Ollama) greedily allocate nearly 100% of available VRAM. When Jellyfin initiates an on-the-fly transcode session, the system throws `CUDA out of memory` errors or stalls playback.
- **Sovereign Day/Night Strategy & Mitigation Options**:
  1. **Daytime AI Reservation**: Keep GPU power and VRAM unconstrained during daytime hours for interactive local AI requests (chat, RAG queries). Avoid running non-urgent video transcoding or batch processing during the day.
  2. **Nighttime / Early Morning Batch Window**: Schedule heavy background tasks (disc rip encoding, multi-profile mobile exports, batch OCR) to execute during off-peak night/early morning hours.
  3. **Admin UI Visibility**: Provide a visible setting in the admin frontend that clearly explains GPU utilization policy and allows the user to configure when the GPU is reserved for AI and when batch background tasks are permitted.
  4. **Dynamic Model Unloading**: Configure `OLLAMA_KEEP_ALIVE=0` or short idle timeouts (e.g. 5 minutes) so VRAM is surrendered when chat is inactive.
  5. **VRAM Overhead Reservation**: Enforce `OLLAMA_GPU_OVERHEAD` reserves to protect encoder headroom.
  6. **Multi-Adapter / Hybrid Routing**: Route media transcoding to Intel QuickSync / iGPU and LLM inference to discrete Nvidia GPU (or CPU fallback).
  7. **Queue Prioritization**: Automatically pause background transcoding jobs during interactive LLM queries, or temporarily scale down LLM context when real-time streaming transcode sessions initiate.

### Dirty-Shutdown / Power-Outage Resilience
- Default ext4 setups risk corrupting container databases (SQLite in Jellyfin/Paperless, PostgreSQL).
- Recommended baseline: Journaled copy-on-write filesystem (e.g., Btrfs with automatic subvolume snapshots or transactional sync) and safe database write-ahead logging (WAL) modes.

## 5. Smart Home Integration & Zero-Conf Discovery Notes

- **Initial Contact**: Primary URL via mDNS (`http://sovereign.local`) and standard DHCP address.
- **Smart Home Discovery Idea**: Investigate Apple HomeKit (via Homebridge / Matter / HAP-NodeJS) or Google Home integration so the Sovereign server appears natively as an accessory or hub in consumer smart home apps during initial network discovery.
- **Status Reporting**: Standardized health model across all containers (Green / Yellow with reason string / Red).

## 6. RAG Single-Source-of-Truth & Update Rollback Mechanics

- **Zero Data Duplication for RAG**: The local LLM / Open WebUI instance must connect directly to Paperless-ngx / DMS search/API endpoints or vector index. Users do not manually re-upload documents to the AI interface.
- **Rollback Architecture**:
  - Image pinning and snapshotting of container configuration volumes prior to any update.
  - Automatic Rollback: If a newly pulled container fails its health check (e.g. within 3 minutes of startup or throws repeated crash loops), the previous image tag and volume state are restored automatically.
  - Manual Rollback: Admin UI exposes a "Roll back to previous version" button per feature.
- **Startup Diagnostics**: Admin interface provides clear startup progress and health reporting rather than a blank unresponsive screen during post-power-loss filesystem checks.

## 7. Media Landing Zone & Shared AI Platform API

- **Media Landing Zone**:
  - Expose a simple, local network shared folder (SMB/Samba or NFS) and/or a drag-and-drop web upload component in the admin dashboard.
  - Automatically routes uploaded videos, audio tracks, and audiobooks into their designated target directories and triggers library refresh/metadata indexing.
- **Shared Local AI Service (OpenAI-compatible Endpoint)**:
  - When the Local AI service is activated, it provides an internal OpenAI-compatible HTTP endpoint (e.g. `http://sovereign-ai:11434/v1` or via proxy).
  - Any installed feature that supports LLM integrations (such as Paperless-ngx automated workflows, transcription, or semantic search) is pre-configured during container provisioning with the internal API URL and mock token, making AI integration zero-touch.
