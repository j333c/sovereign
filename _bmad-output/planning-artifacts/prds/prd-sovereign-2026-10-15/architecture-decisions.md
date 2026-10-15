# Sovereign — Architecture Decision Records (ADRs)

> **Status**: Accepted & Binding  
> **Date**: 2026-10-15  
> **Authors**: Winston (System Architect), Amelia (Senior Software Engineer), John (Product Manager)  
> **Governing PRD**: [PRD Sovereign 2026-10-15](prd.md) · [Addendum](addendum.md)

---

## Index of Decisions

1. [ADR-001: Local LLM Inference Engine Runtime](#adr-001-local-llm-inference-engine-runtime)
2. [ADR-002: Document Knowledge (RAG) Retrieval Architecture](#adr-002-document-knowledge-rag-retrieval-architecture)
3. [ADR-003: GPU Resource Arbitration & Video Streaming Strategy](#adr-003-gpu-resource-arbitration--video-streaming-strategy)

---

## ADR-001: Local LLM Inference Engine Runtime

### Status
**ACCEPTED**

### Context
Sovereign provides a private, 100% offline local conversational AI assistant and document querying capability. The platform needs an inference engine that executes quantized open weights (such as Qwen 2.5 7B or Llama 3.2), exposes a standard OpenAI-compatible HTTP endpoint (`/v1`), and operates reliably across diverse consumer hardware (Nvidia CUDA GPUs, Intel/AMD integrated GPUs, and CPU-only mini-PCs).

### Options Considered
1. **Option A: Managed `llama.cpp` (`llama-server`)**:
   - Statically linked or minimal C++ binary container.
   - Native direct GGUF format loading via shared host storage (`/data/models`).
   - Explicit parameter control: FlashAttention (`-fa`), layer offload (`-ngl`), context length (`-c`), thread pinning (`-t`).
2. **Option B: Ollama Container Daemon**:
   - Broad consumer mindshare and automated model pulling.
   - Packages model weights into opaque internal content-addressed blobs (`~/.ollama/models/blobs`).
   - Background Go/Python runtime resident in memory.
3. **Option C: Python-based Inference Server (vLLM / Hugging Face TGI)**:
   - High enterprise batch throughput and continuous batching.
   - Requires heavy Python virtual environments, massive image sizes (10+ GB), and 1.5+ GB baseline idle memory before loading weights.
   - Poor consumer CPU-only and iGPU fallback.

### Decision
**Adopt Option A: Managed `llama.cpp` (`llama-server`)**.

### Consequences & Rationale
- **Zero Python Overhead**: The C++ binary consumes < 25 MB RAM when idle, avoiding runtime bloat on power-conscious home servers.
- **File Portability**: Model weights live in standard `/data/models/*.gguf` files accessible to the host and decoupled from container lifecycles.
- **Granular Hardware Tuning**: Direct control over `-ngl` allows precise VRAM budgeting to guarantee headroom for media transcoding.
- **Instant Wake via `mmap`**: When recovering from dynamic idle surrender, the model weights remain cached in the host OS page cache (`mmap`), reducing reload latency from 30 seconds down to < 2 seconds.
- **Predictable Core Pinning**: On CPU-only hosts, threads are capped to `max(1, physical_cores - 2)` to preserve host operating system and network responsiveness.

---

## ADR-002: Document Knowledge (RAG) Retrieval Architecture

### Status
**ACCEPTED**

### Context
Users require private conversational search and factual question-answering over their scanned invoices, insurance policies, contracts, and medical records stored in Paperless-ngx.

### Options Considered
1. **Option A: Direct SQLite FTS5 Full-Text Search Context Injection**:
   - Queries execute against Paperless-ngx's internal SQLite FTS5 full-text search index and document metadata.
   - Matching document chunks and page numbers are dynamically formatted into the LLM inference prompt context.
   - Zero additional database containers or background embedding workers.
2. **Option B: Dedicated Standalone Vector Database (Chroma / Qdrant / Milvus)**:
   - Embeds OCR text into dense vectors using a local embedding model.
   - Requires running an independent vector database daemon and background synchronization queue.
3. **Option C: Hybrid Sparse (BM25) + Dense Vector Pipeline**:
   - Multi-stage retrieval combining keyword matching and vector re-ranking.
   - Highest retrieval precision, but maximum complexity, memory consumption, and potential synchronization failure points.

### Decision
**Adopt Option A: Direct SQLite FTS5 Context Injection (Single-Source-of-Truth)**.

### Consequences & Rationale
- **Zero Redundant Storage**: Documents are not duplicated into a secondary database; Paperless-ngx remains the single source of truth.
- **Zero Index Drift**: Newly OCR'd documents are immediately searchable without background embedding queue lag or synchronization mismatch.
- **Appliance Simplicity**: Eliminates 500 MB–1 GB of persistent RAM consumption required by vector database daemons.
- **Factual Exactness**: Household document queries rely primarily on concrete identifiers (vendor names, policy numbers, dates, monetary totals) where BM25 full-text indexing excels over fuzzy vector embeddings.

---

## ADR-003: GPU Resource Arbitration & Video Streaming Strategy

### Status
**ACCEPTED**

### Context
In single-GPU or unified-memory consumer hardware setups, concurrent video streaming (Jellyfin real-time hardware transcoding) and LLM inference can compete for VRAM, triggering `CUDA out of memory` errors or playback stutter.

### Options Considered
1. **Option A: Ingestion Direct Play Pre-encoding + Daytime AI Priority + Dynamic Idle Surrender**:
   - Ingestion and disc ripping pre-transcode media during off-peak hours into standardized, universal streaming formats (H.264/H.265 + AAC in MP4/MKV) so modern Smart TVs direct-play streams without live transcoding.
   - AI has daytime priority; idle surrender releases VRAM after 5 minutes of inactivity.
   - Two-lane API priority governor ensures interactive human chat takes precedence over background document extraction.
2. **Option B: Real-Time Kernel/Driver Preemption**:
   - Dynamic GPU arbitration daemon intercepts ffmpeg processes and sends hot IPC signals to downscale `llama-server` layer allocation in real time.
3. **Option C: Strict Hardware Partitioning**:
   - Restrict local AI to CPU only, reserving the GPU exclusively for video transcoding.

### Decision
**Adopt Option A: Ingestion Direct Play Pre-encoding + Daytime AI Priority + Dynamic Idle Surrender**.

### Consequences & Rationale
- **Root Cause Elimination**: Modern Smart TVs, Apple TVs, and mobile clients natively support H.264/H.265 Direct Play. Standardizing media during ingestion eliminates 95%+ of on-the-fly live transcoding, preventing GPU contention entirely during media playback.
- **Full Daytime Performance**: Eliminating live transcoding contention allows the local AI model to utilize 100% of available GPU acceleration for sub-second conversational responses during daytime hours.
- **Appliance Robustness**: Avoids fragile, low-level driver hooking or runtime IPC signaling that frequently causes video playback freezes or driver crash-loops.
- **Energy Efficiency**: Dynamic idle surrender unloads weights after 5 minutes of inactivity, lowering idle system wattage and reducing thermal output.
