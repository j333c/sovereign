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

---

## Warranty & Support Disclaimers

### 1. Open-Source Software Disclaimer (DIY & Self-Hosted)
Sovereign software is distributed under the [Apache License, Version 2.0](../../LICENSE). In accordance with Sections 7 and 8 of the license:
- **"AS IS" Basis**: The software and documentation are provided on an "AS IS" basis, without warranties or conditions of any kind, either express or implied, including, without limitation, any warranties or conditions of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A PARTICULAR PURPOSE.
- **Limitation of Liability**: You are solely responsible for determining the appropriateness of using or redistributing the software and assume any risks associated with your exercise of permissions under this license. In no event and under no legal theory shall any contributor or maintainer be liable to you for damages, including any direct, indirect, special, incidental, or consequential damages arising out of the use or inability to use the software (including but not limited to loss of data, hardware malfunction, or system failure).
- **Community Support**: DIY and self-hosted installations are supported collaboratively via public GitHub discussions, issue trackers, and open documentation. No commercial service level agreements (SLAs) or guaranteed response times are provided for community deployments.

### 2. Pre-Assembled Turnkey Hardware Appliance Boundaries
For upcoming official factory pre-configured physical appliance editions (Sovereign Mini, Home, Pro Studio):
- **Physical Hardware Warranty**: Physical hardware components, chassis, power delivery, and cooling systems are covered by standard statutory consumer hardware warranties (minimum 24 months in the European Union, standard 12 months internationally) provided exclusively by authorized contract manufacturing partners and hardware distributors.
- **Software Boundary**: The pre-installed Sovereign operating system and software substrate remain 100% open source under Apache 2.0. Hardware warranty coverage protects against physical component defects and factory assembly failures, but does not constitute an ongoing commercial software maintenance contract or SLA for third-party application services.
- **Data Responsibility**: While Sovereign features automated local snapshotting and power-resilient SQLite WAL journaling, users retain ultimate responsibility for household data backup routines and off-site replication as outlined in [TIERS.md](./TIERS.md).

