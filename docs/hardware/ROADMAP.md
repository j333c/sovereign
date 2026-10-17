# Sovereign Turnkey Hardware Appliance Roadmap

While Sovereign is and will always remain 100% free, open-source software that anyone can install on commodity PC hardware, many families and non-technical households desire a **factory pre-configured, silent, elegant household appliance** that works immediately out of the box.

This roadmap details the three planned official Sovereign physical appliance editions and architectural pre-order hooks.

---

## Hardware Appliance Editions

### Phase 1: Sovereign Mini (Target: Q2 2027)
- **Form Factor**: Ultra-compact 0.6L CNC aluminum fanless chassis with engraved brand crest.
- **Hardware Profile**: Light Tier specification (Intel N100, 8 GB LPDDR5, 512 GB NVMe SSD).
- **Target Audience**: Families focused on private document archiving, optical disc music ripping, and lossless audio streaming with zero noise and sub-8W idle consumption.
- **Pre-installed Software**: Sovereign OS (hardened Linux kernel, pre-provisioned Caddy, Paperless-ngx, Navidrome).

### Phase 2: Sovereign Home (Target: Q4 2027)
- **Form Factor**: 4-bay brushed anodized aluminum mini-tower with hot-swap SATA drive caddies and integrated slot-loading Blu-ray drive.
- **Hardware Profile**: Media Tier specification (Intel Core i3/i5 with QuickSync, 16 GB DDR5, dual 2.5 GbE ports, optical disc auto-ingestion sensor).
- **Target Audience**: Media enthusiasts replacing proprietary streaming subscriptions and digitizing household physical media collections.

### Phase 3: Sovereign Pro Studio (Target: 2028)
- **Form Factor**: Compact desktop workstation with dedicated whisper-quiet chambered cooling and integrated battery-backed power outage protection.
- **Hardware Profile**: AI & Power Tier specification (8-core x86 CPU, 32 GB DDR5, dedicated 16 GB VRAM AI tensor accelerator, 8 TB enterprise storage).
- **Target Audience**: Power users, home offices, and multi-generational households demanding real-time local LLM intelligence, full-text RAG querying, and simultaneous 4K HDR transcoding.

---

## Architectural Reservation Hooks

The web interface includes an architectural reservation and notification hook (`/hardware` and `AppHeader`) enabling users to:
1. Register interest and reserve priority manufacturing slots for upcoming hardware batches.
2. Generate an appliance hardware identity token for zero-touch pairing during first boot.
3. Check hardware compatibility benchmarks directly against their current system telemetry.
