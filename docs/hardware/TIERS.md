# Sovereign Hardware Tiers Specification

This document provides exhaustive hardware specifications, benchmark baselines, and component guidelines for the three official Sovereign appliance hardware profiles.

---

## 1. Light Tier: Archival & Audio Hub

The **Light Tier** is tailored for households seeking paperless document archiving with automated OCR, personal cloud file storage, and lossless music streaming without heavy video transcoding or local generative AI.

### Target Workloads
- **Paperless-ngx**: Automated document ingestion, OCR text extraction, SQLite FTS5 search.
- **Navidrome**: Lossless FLAC/MP3 music streaming over Subsonic API.
- **Caddy Web Server**: TLS termination and local reverse proxy.
- **Sovereign Daemon**: Health supervision, automated daily snapshots, and telemetry streaming.

### Technical Specifications
- **CPU**: Dual-core or quad-core modern low-power CPU:
  - *x86-64*: Intel Processor N100 (4 cores, 6W TDP), Intel Celeron J4125, N5105.
  - *ARM64*: Rockchip RK3588, Broadcom BCM2712 (Raspberry Pi 5 4GB/8GB).
- **RAM**: 4 GB DDR4/DDR5 (minimum). 8 GB recommended for heavy simultaneous OCR batches.
- **System Storage**: 64 GB SSD (eMMC, SATA, or NVMe).
- **Data Storage**: 250 GB to 1 TB SSD or external USB 3.0 drive for archival storage.
- **Networking**: 1x Gigabit Ethernet (RJ-45). Wi-Fi 5/6 supported as fallback.
- **Power Profile**: 5W – 10W idle power consumption; passive or whisper-quiet cooling.
- **Backup Power & Redundancy**:
  - *UPS*: Compact 350VA – 500VA standby UPS or 12V DC inline mini-UPS provides 30–60 minutes of clean bridge power during household outages.
  - *Storage Redundancy*: Single SSD with automated daily encrypted snapshots replicated to an external USB drive or network SMB/NFS share. RAID 1 mirroring optional.

---

## 2. Media Tier: Family Entertainment & Optical Ingestion

The **Media Tier** adds multi-stream hardware-accelerated video transcoding, audiobook streaming with progress synchronization, and automatic optical media ingestion (CD, DVD, Blu-ray).

### Target Workloads
- **Everything in Light Tier** (Paperless-ngx + Navidrome).
- **Jellyfin**: 4K HEVC/H.264 video streaming with Intel QuickSync tone mapping and transcoding.
- **Audiobookshelf**: Audiobooks and podcasts streaming with multi-user chapter bookmarking.
- **Automatic Ripping Machine (ARM)**: Automated optical disc detection, ripping, and metadata tagging via `udev` triggers.

### Technical Specifications
- **CPU**: Quad-core x86 CPU with integrated GPU supporting Intel QuickSync Video (QSV):
  - *Recommended*: Intel Processor N100/N305, Intel Core i3-12100, Core i5-11400 / 12400.
  - *AMD Alternative*: AMD Ryzen 5 5600G / 7600 with VA-API support.
- **RAM**: 8 GB DDR4/DDR5 (minimum). 16 GB recommended when running simultaneous 4K transcodes.
- **Optical Drive**: Internal SATA or external USB 3.0 Blu-ray / DVD / CD drive with Tray-Eject support:
  - *LibreDrive & UHD Support*: For 4K UHD Blu-ray ingestion, choose drives with LibreDrive / UHD-friendly custom firmware (e.g. LG WH16NS40, Archgon MD-3102).
  - *Power Supply Requirement*: For external USB drives, use models with an independent AC 12V power supply to avoid USB bus brownouts during spin-up.
- **Data Storage**: 2 TB to 8 TB raw storage (NVMe cache + 3.5" SATA HDD pool or multi-terabyte SSDs).
- **Storage Redundancy**: 2-drive RAID 1 mirror or Btrfs / ZFS mirrored pool to protect against single-disk mechanical failure without service downtime.
- **Networking**: 1x or 2x Gigabit / 2.5 GbE Ethernet.
- **Power Profile**: 10W – 25W idle; 35W – 55W peak transcode load.
- **Backup Power (UPS)**: 500VA – 750VA UPS with USB HID/NUT signaling for automated graceful shutdown before battery exhaustion.

---

## 3. AI & Power Tier: Private Household Intelligence

The **AI & Power Tier** transforms Sovereign into a comprehensive private household assistant. In addition to full media streaming and document archiving, it runs local 7B–8B quantized large language models (llama.cpp) with zero cloud telemetry and instant retrieval-augmented generation (RAG) over your family's personal document archive.

### Target Workloads
- **Everything in Media Tier** (Jellyfin + Navidrome + Audiobookshelf + Paperless-ngx + ARM).
- **Managed llama.cpp Inference Backend**: Local GGUF execution with FlashAttention, parallel query slots, and dynamic GPU/CPU memory arbitration.
- **Open WebUI**: Household private chat interface with role personas (Household Assistant, Legal Advisor, Creative Writer, Tech Support).
- **Single-Source-of-Truth RAG**: Direct zero-vector database text retrieval querying Paperless-ngx FTS5 indices.

### Technical Specifications
- **CPU**: 6+ core modern high-efficiency CPU:
  - *Intel*: Core i5-12600K / 13500 / 14500 (with QuickSync for video transcoding).
  - *AMD*: Ryzen 7 5700X / 7700X, or AMD Ryzen 7 7840HS / 8840U mini-PCs with 780M iGPU.
- **RAM**: 16 GB to 32 GB DDR5 RAM.
- **GPU & VRAM**:
  - *Discrete NVIDIA*: GeForce RTX 3060 12GB, RTX 4060 8GB / 16GB, or RTX 4070 (8 GB+ VRAM required for full Q4_K_M layer offload).
  - *Unified Memory APU*: 32 GB shared LPDDR5X (e.g. Minisforum UM780 XTX, Framework Desktop).
- **Data Storage**: 4 TB to 16 TB storage pool (PCIe 4.0 NVMe for OS + models + database; SATA disks for bulk media).
- **Power Profile**: 25W – 45W idle; 90W – 180W full simultaneous AI query + 4K transcode load.
- **Uninterruptible Power Supply (UPS)**: 650VA – 1000VA UPS with USB NUT signalling for automated graceful shutdown.

---

## Architectural Hooks: Turnkey Pre-Assembled Hardware

For users who prefer a plug-and-play appliance rather than DIY PC building, Sovereign reserves architectural interfaces for factory-assembled turnkey appliances. Consult [ROADMAP.md](./ROADMAP.md) for pre-order announcements and hardware release phases.
