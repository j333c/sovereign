# Sovereign PRD Addendum: Technical Notes & Candidate Architecture

This addendum preserves technical depth, architectural ideas, candidate software components, and downstream operational details volunteered during discovery.

## 1. Candidate Software Stack

- **Media Streaming (Video & Audio)**:
  - Video Streaming candidate: [Jellyfin](https://jellyfin.org/) (open-source, official containers, DLNA, wide client ecosystem).
  - Audio Streaming / Audiobooks candidates: Navidrome (lightweight Subsonic API server for music/playlists), Audiobookshelf (dedicated audiobook & podcast server with multi-device sync), or Jellyfin music support.
- **Local AI & Knowledge (RAG)**:
  - User-Facing AI Interface candidate: [Open WebUI](https://github.com/open-webui/open-webui) (clean chat UI, multi-model support, built-in RAG / web search integration, mobile-responsive).
  - LLM Inference Backend candidate: [llama.cpp](https://github.com/ggerganov/llama.cpp) (`llama-server`) container backend.
    - *Why llama.cpp*: Extremely lightweight C/C++ engine, native GGUF model format, granular layer-by-layer GPU offloading (`-ngl`), zero heavyweight Python runtime overhead, built-in OpenAI-compatible HTTP server (`/v1`), explicit context memory limits (`-c`), and multi-backend support (CUDA, Vulkan, Intel SYCL, Metal, CPU).
    - *Default Setup*: Pre-configured default model (e.g., Qwen 2.5 7B Instruct or Llama 3.2 3B/8B GGUF Q4_K_M) with autodetected hardware acceleration and pre-tuned context length (4096 tokens).
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
- **Problem**: In consumer setups with a single shared GPU (or unified iGPU), local LLM inference engines greedily allocate available VRAM. When Jellyfin initiates an on-the-fly hardware transcode session, the system throws `CUDA out of memory` errors or stalls playback.
- **Sovereign Day/Night Strategy & llama.cpp Arbitration Mechanics**:
  1. **Deterministic Layer Offloading (`-ngl`)**: Unlike opaque runtimes, `llama.cpp` allows precise control over how many transformer layers are offloaded to VRAM (`--n-gpu-layers`). Sovereign calculates and caps VRAM allocation, guaranteeing a dedicated buffer (e.g. 1.5–2 GB) is always preserved for video transcode sessions.
  2. **Daytime AI Reservation**: Keep GPU power and VRAM unconstrained during daytime hours for interactive local AI requests (chat, RAG queries). Avoid running non-urgent video transcoding or batch processing during the day.
  3. **Nighttime / Early Morning Batch Window**: Schedule heavy background tasks (disc rip encoding, multi-profile mobile exports, batch OCR) to execute during off-peak night/early morning hours.
  4. **Dynamic Idle Surrender & Timeout**: Enforce an idle timeout (default: 5 minutes) where `llama-server` releases VRAM or unloads model weights when inactive, surrendering 100% of GPU compute and memory to active media transcoding.
  5. **Admin UI Visibility**: Provide visible settings in the admin frontend that clearly explain GPU utilization policy and allow the user to adjust daytime vs. nighttime priorities, active model, and offload layers.
  6. **Multi-Adapter / Hybrid Routing**: Route media transcoding to Intel QuickSync / iGPU and LLM inference to discrete Nvidia GPU (or high-efficiency CPU fallback with AVX2/AVX-512).
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
- **Managed LLM Backend Administration & Shared API Endpoint**:
  - **Candidate Runtime**: `llama.cpp` (`llama-server`) running as an independent, lightweight C++ container on internal bridge network (`http://sovereign-ai-backend:8080/v1`).
  - **Turnkey Default Setup**: First boot provisions a recommended default quantized GGUF model (e.g., Qwen 2.5 7B Instruct or Llama 3.2 Q4_K_M), autodetects available GPU acceleration (CUDA, Vulkan, SYCL, or CPU), and wires the internal OpenAI-compatible endpoint to Open WebUI and Paperless-ngx.
  - **Admin Surface in Sovereign**: Dedicated management controls in the Admin Interface allow the user to:
    - View active model, memory footprint (RAM/VRAM), and inference speed (tokens/sec).
    - Select from downloaded GGUF models in `/data/models`.
    - Adjust GPU offload layer slider (`-ngl`), context window presets (`-c`: 2048, 4096, 8192), and idle surrender timeout.
    - Reload backend model or restart container with a single click.
    - Inspect backend inference and error logs without CLI access.
  - **Zero-Touch Feature Integration**: Any installed feature supporting AI integration automatically inherits the internal API URL and mock token during container provisioning.
