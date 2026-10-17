# Sovereign Hardware Specification & Tier Guidance

Welcome to the Sovereign Home Appliance Hardware Guide. Sovereign is designed to run efficiently on standard, readily available consumer computing hardware — from fanless energy-efficient mini-PCs to high-performance workstations with discrete AI accelerators.

## Hardware Philosophy

1. **Zero Proprietary Lock-In**: Run Sovereign on standard x86-64 or modern 64-bit ARM hardware.
2. **Energy Efficiency First**: Home servers run 24/7. Idling at 5–15W saves substantial electricity compared to enterprise server racks.
3. **Decoupled Architecture**: Workloads are strictly modular. If you don't run AI or transcode video, a low-cost \$120 mini-PC is more than sufficient.
4. **Data Durability**: Storage architecture uses Linux kernel ext4/btrfs with SQLite WAL mode and optional automated network NAS replication.

## Hardware Tiers at a Glance

| Feature Tier | Target Workloads | Minimum CPU | Minimum RAM | Recommended Storage | Idle Power |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Light Tier** | Document OCR (Paperless-ngx) + Lossless Music (Navidrome) | Dual-core x86 / ARM64 (Intel N100, Celeron, RPi 5) | 4 GB | 64 GB – 500 GB SSD | 5W – 10W |
| **Media Tier** | 4K Video Streaming (Jellyfin QuickSync) + Audiobookshelf + Optical Disc Ingestion | Quad-core x86 with iGPU (Intel N100/N305, Core i3/i5 10th+ Gen) | 8 GB | 2 TB – 8 TB SATA/NVMe | 10W – 25W |
| **AI & Power Tier** | Local 7B/8B LLM Chat (llama.cpp) + Instant RAG + Transcoding + Archiving | 6+ core x86 + NVIDIA GPU (8–16 GB VRAM) or unified memory APU | 16 GB – 32 GB | 4 TB – 16 TB NVMe/SATA | 25W – 45W |

For comprehensive component breakdowns and benchmark recommendations, consult [TIERS.md](./TIERS.md).

For the turnkey pre-assembled Sovereign appliance roadmap, consult [ROADMAP.md](./ROADMAP.md).
