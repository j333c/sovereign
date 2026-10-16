# Epic 5 Context: Local AI Knowledge & Turnkey Inference Platform

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Provide a 100% private, turnkey local AI inference platform and document knowledge engine for Sovereign: deploy a managed high-performance C++ inference backend (`llama-server` from `llama.cpp`) operating inside an air-gapped `sovereign-ai` Docker network with no exposed host ports; govern all inbound requests through an internal AI Gateway reverse proxy in `sovereign-daemon` (`:8080/v1`) featuring a Two-Lane Priority Request Governor (prioritizing interactive user chat over automated background tasks like OCR parsing and metadata tagging); enforce dynamic GPU arbitration and Day/Night scheduling (releasing VRAM automatically after 300 seconds of inactivity to prevent media playback crashes, while waking with reassuring UI telemetry during daytime 07:00–01:00); provide an intuitive, responsive Open WebUI chat client at `/chat` (port `:3001`) with localized system prompts and persona selection; and deliver single-source-of-truth Retrieval-Augmented Generation (RAG) querying Paperless-ngx SQLite FTS5 directly with exact document titles and page citations, governed by a Context Window Headroom Truncator (reserving a 512-token response buffer).

## Stories

- Story 5.1: Managed `llama.cpp` Backend & Appliance Boundary Guard Suite
- Story 5.2: Mediated AI Gateway & Two-Lane Priority Request Governor
- Story 5.3: Dynamic GPU Arbitration & Day/Night Idle Surrender
- Story 5.4: Open WebUI Chat Interface & Localized Customization Prompts
- Story 5.5: Single-Source-of-Truth RAG & Context Window Headroom Truncator
- Story 5.6: AI Backend & Model Administration Interface in Settings

## Requirements & Constraints

- **Air-Gapped Container Networking**:
  - Raw `llama-server` runs inside the isolated `sovereign-ai` bridge network (`172.28.0.0/16` or internal bridge).
  - Raw `llama-server` must NEVER publish host ports or bind to public interfaces.
  - All communication to `llama-server` is mediated through the supervisor daemon's AI Gateway (`:8080/v1`).
- **Hardware Acceleration Autodetection & Flag Optimization**:
  - Autodetects host acceleration: CUDA (`/dev/nvidia*`), Vulkan (`/dev/dri/renderD128`), SYCL (Intel), or CPU fallback.
  - Automatically enables FlashAttention (`-fa`) and standardized `-np 2` parallel execution slots for simultaneous chat + background tasks.
  - On CPU-only hosts, threads are capped at `max(1, physical_cores - 2)` and UI indicates "CPU-Optimized Mode".
- **Pre-flight GGUF Magic Header Verification**:
  - GGUF files in `/data/models` must be verified by reading binary header bytes (`0x47, 0x47, 0x55, 0x46` - `GGUF`) before loading into `llama-server`.
  - Corrupted, incomplete, or non-GGUF files are rejected before container launch.
- **Safe Stub Mode**:
  - If `/data/models` contains no valid GGUF model, the backend does not crash; instead, the AI Gateway operates in Safe Stub Mode, returning RFC 7807 Problem Details (HTTP 503) explaining that no models are provisioned and directing the user to download a model in Settings.
- **Storage Reservation Guard**:
  - Model downloads require free space on `/data/models` greater than `model_size + 5.0 GB` buffer to prevent disk exhaustion.
- **Two-Lane Priority Request Governor**:
  - Lane 1 (Interactive Human Chat): High priority, zero wait, executes immediately.
  - Lane 2 (Background Tasks): Low priority, queued/yields when interactive queries arrive.
- **Generation Intent Presets**:
  - *Precise & Factual*: Temperature 0.1, top_p 0.9 (for document analysis, deadline extraction, invoice parsing).
  - *Balanced Assistant*: Temperature 0.7, top_p 0.95 (general household queries, writing).
  - *Creative Drafting*: Temperature 0.85, top_p 0.98 (brainstorming, creative drafting).
- **Dynamic Idle Surrender & Arbitration**:
  - After 300 seconds of inactivity, `sovereign-daemon` unloads model weights or stops the container, surrendering GPU memory.
  - Daytime (07:00 to 01:00): Incoming queries trigger wake-up with `status: "warming_up"` and an extended timeout buffer (>= 60s).
  - Nighttime (01:00 to 07:00): VRAM prioritized for batch transcoding and OCR jobs.
- **Single-Source-of-Truth RAG**:
  - Direct queries against Paperless-ngx SQLite FTS5 tables without external vector databases (no Chroma/Qdrant).
  - Context Window Headroom Truncator reserves 512 tokens for model response, retaining top-ranked chunks and appending warning if context is truncated.
  - Responses include citations with document titles and page numbers linked to `/docs/documents/:id`.
- **Open WebUI Integration**:
  - Pinned official image `ghcr.io/open-webui/open-webui:0.12.0` on port `:3001` and subpath `/chat` through Caddy.
  - Zero external telemetry or cloud pings emitted.

## Technical Decisions

- **Container Images**:
  - `llama-server`: Official `ghcr.io/ggml-org/llama.cpp:b11647` (or `:server`).
  - `open-webui`: Official `ghcr.io/open-webui/open-webui:0.12.0`.
- **Filesystem Layout**:
  - Models directory: `/data/models` (contains `.gguf` files, active model symlink, temporary `.downloading` files).
  - State directory: `/data/state/services/llama` and `/data/state/services/openwebui`.
- **Package Architecture**:
  - `internal/ai`: AI Gateway reverse proxy, request governor, generation presets, GGUF magic validator, and RAG orchestrator.
  - `internal/supervisor`: `llama-server` and `open-webui` container specifications, network isolation (`sovereign-ai`), GPU device allocation, and lifecycle controls.
  - `cmd/sovereign-daemon`: Router registration for `/v1/*`, `/api/v1/ai/*`, and SSE status broadcasting.
