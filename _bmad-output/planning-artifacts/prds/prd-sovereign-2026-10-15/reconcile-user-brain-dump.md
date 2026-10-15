# Input Reconciliation: User Brain Dump & Requirements vs. Sovereign PRD

- **Input Source**: User Brain Dump & Comments
- **Target PRD**: `_bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/prd.md`
- **Target Addendum**: `_bmad-output/planning-artifacts/prds/prd-sovereign-2026-10-15/addendum.md`
- **Reconciliation Date**: 2026-10-15
- **Verdict**: **Full Match** (15 / 15 requirements completely addressed with high-rigor specifications)

---

## 1. Executive Summary

A comprehensive input reconciliation was conducted comparing the user's initial brain dump and design parameters against the drafted Sovereign Home Server PRD (`prd.md`) and Technical Addendum (`addendum.md`).

Every single requirement provided by the user—spanning media streaming, local AI, document management, appliance resilience, hardware tiering, multi-lingual support, and project governance—has been fully accommodated. Furthermore, each requirement is backed by concrete Functional Requirements (FRs), testable acceptance criteria, and specific User Journeys (UJs), with non-functional guardrails and competitive safeguards.

---

## 2. Requirement-by-Requirement Reconciliation Matrix

| # | User Requirement | PRD / Addendum Mapping | Coverage & Realization Analysis | Status |
|---|------------------|------------------------|---------------------------------|:------:|
| 1 | **In-house video streaming (movies, tv shows) via Jellyfin** | PRD: §2.3 (UJ-1, UJ-3), §4.3 (FR-9), §6.1<br>Addendum: §1 | Pre-configured Jellyfin container; direct play and hardware-accelerated transcoding (Intel QuickSync, Nvidia NVENC, VAAPI); DLNA and API exposure; automated internal storage path mapping. Out-of-scope clarifies zero custom player app development. | **Full Match** |
| 2 | **In-house audio streaming (music, audiobooks, party playlists) via open-source tools** | PRD: §4.3 (FR-10), §6.1<br>Addendum: §1 | Navidrome (Subsonic API) and Audiobookshelf containers; dedicated web interface for party playlist creation and continuous playback; multi-device sync, chapter marks, and playback speed adjustment for audiobooks; compatibility with open mobile players (Symfonium, Plappa, Audiobookshelf mobile). | **Full Match** |
| 3 | **Local AI model for chat-bot (Open WebUI) & RAG document knowledge (direct query against DMS, zero duplicate uploads)** | PRD: §2.3 (UJ-4), §4.5 (FR-14, FR-15, FR-16), §7 (SM-3)<br>Addendum: §1, §6, §7 | Open WebUI interface paired with local Ollama/vLLM backend operating 100% offline; single-source-of-truth RAG architecture querying Paperless-ngx index directly without file duplication; citations include document name and page number. Standardized OpenAI-compatible API exposed internally. | **Full Match** |
| 4 | **Document management system (Paperless-ngx, OCR, digitize paperwork via smartphone, mailbox IMAP import/sorting, nightly AI check for payment dues and expiring contracts with email/Home Assistant triggers)** | PRD: §2.3 (UJ-2), §4.4 (FR-11, FR-12, FR-13)<br>Addendum: §1, §2 | Paperless-ngx container providing OCR on images and PDFs; integration with Paperless mobile scanner apps; automated IMAP polling for incoming email attachments; scheduled analysis for due dates/contracts with event triggers dispatched via Home Assistant webhook/MQTT and email digest. | **Full Match** |
| 5 | **Hardware setup free to user with minimum specs by tier; later hardware sales via website** | PRD: §4.6 (FR-20), §6.1, §6.2<br>Addendum: §3 | Software remains 100% free, open-source, and self-installable; documentation defines 3 concrete hardware tiers (*Light*, *Media*, *AI & Power*); v1 focuses on DIY guidance while roadmap explicitly reserves architectural and commercial hooks for pre-assembled hardware sales via the marketing website. | **Full Match** |
| 6 | **Fast-booting low-level admin web interface (activate, deactivate, update, auto-update with rollback)** | PRD: §2.3 (UJ-1, UJ-5), §4.1 (FR-1, FR-2, FR-3, FR-4), §4.7 (NFR-2), §7 (SM-1, SM-2, SM-6)<br>Addendum: §2, §6 | Fast-booting admin UI (initializes in <= 15s, TTI <= 2.0s); first-run setup wizard via mDNS (`sovereign.local`); one-click feature activation/deactivation with traffic-light status; automated 3:00 AM updates with configuration snapshots and 180s health-check auto-rollback; manual one-click rollback in UI. | **Full Match** |
| 7 | **Invisible installation via official OCI containers; transparent update-safe configs; power-off robust (dirty shutdown recovery)** | PRD: §2.3 (UJ-5), §4.1 (FR-2, FR-3, FR-4), §4.7 (NFR-1, NFR-3), §7 (SM-4)<br>Addendum: §2, §4 | Official upstream OCI images deployed in isolated bridge networks; data and config mapped to persistent host volumes that survive image updates; journaled CoW filesystem and WAL mode for databases prevent corruption during dirty cuts; SM-4 requires 20 consecutive ungraceful power cuts with zero database corruption. | **Full Match** |
| 8 | **Internal storage or NAS support** | PRD: §2.3 (UJ-1), §4.2 (FR-5), §6.1<br>Addendum: §2 | Supports formatting and pooling internal SATA/NVMe drives; native support for mounting external NFS and SMB/CIFS network shares; handles temporary NAS dropouts with informative Yellow indicator without container crashes. | **Full Match** |
| 9 | **GPU concurrency handling (daytime AI priority, nightly/early morning transcoding window, visible admin schedule setting)** | PRD: §4.5 (FR-17), §7 (SM-C3)<br>Addendum: §2, §4 | Daytime GPU priority (07:00–01:00) reserves compute/VRAM for interactive AI; non-urgent background transcoding and batch OCR deferred to off-peak night window (01:00–07:00); visible user-facing scheduling controls in Admin UI; dynamic model unloading (`OLLAMA_KEEP_ALIVE`), VRAM overhead buffers, and split iGPU/dGPU routing. | **Full Match** |
| 10 | **Blu-ray/DVD auto-import (insert-and-walk-away, external metadata, auto-eject) + morning transcoding for in-house and iPad downloads** | PRD: §2.3 (UJ-3), §4.2 (FR-7, FR-8), §7 (SM-5)<br>Addendum: §2 | Optical drive detection within 10s; unattended ripping and automatic metadata retrieval from TMDb/IMDb/MusicBrainz; auto-eject on completion; scheduled morning transcoding queue produces in-house masters and optimized 720p/1080p offline mobile profiles downloadable via Jellyfin. | **Full Match** |
| 11 | **Multi-language UI and documentation in EN, DE, ES, FR** | PRD: §4.6 (FR-18), §6.1 | Instant language switcher in Admin UI supporting English (EN), German (DE), Spanish (ES), and French (FR); Setup Wizard auto-detects browser locale; bundled short guides and comprehensive documentation fully translated into all 4 languages. | **Full Match** |
| 12 | **Marketing website and GitHub README with BMad Method attribution, AI transparency, and FOSS license compliance** | PRD: §4.6 (FR-19), §5, §6.1<br>Addendum: §3 | Public marketing site directing users to guides and GitHub repo; GitHub README explicitly credits BMad Method, transparently discloses AI-assisted development, provides attribution to bundled upstream FOSS projects, and commits strictly to permissive open-source licensing. | **Full Match** |
| 13 | **AI standard customization prompt with locale language & country regulations/habits** | PRD: §4.5 (FR-14) | Default system prompt configured during deployment; automatically matches user's locale language and embeds country-specific legal/regulatory knowledge (e.g., statutory cancellation periods, standard billing cycles, regional contract conventions). | **Full Match** |
| 14 | **No custom client software (use existing client apps and web browsers)** | PRD: §2.1, §4.3 (FR-9, FR-10), §4.4 (FR-12), §4.5 (FR-14), §5 (Non-Goals) | Formally designated as primary Non-Goal #1; leverages official Jellyfin client apps (TV, mobile, desktop), Paperless mobile apps, Subsonic/Audiobookshelf audio players, and responsive web browsers; zero client app code to develop or maintain. | **Full Match** |
| 15 | **Media landing zone (SMB share & web upload)** | PRD: §3 (Glossary), §4.2 (FR-6), §6.1<br>Addendum: §7 | Dual-access landing zone via network SMB share (`\\sovereign\drop` / `smb://sovereign.local/drop`) and web drag-and-drop uploader in Admin UI; auto-sorting into target media directories (Movies, TV, Music, Audiobooks); automated library trigger within 60 seconds. | **Full Match** |

---

## 3. Top Qualitative Nuances & Architectural Enforcements

1. **Active GPU Arbitration vs. The VRAM Collision Pitfall**:
   - Rather than treating GPU access as an unmanaged shared resource, the PRD tackles the known `CUDA out of memory` failure mode seen in competing appliances (e.g. Umbrel/CasaOS). It formalizes daytime interactive AI priority, nighttime batch encoding, `OLLAMA_KEEP_ALIVE` limits, minimum VRAM headroom reserves, and hybrid iGPU/dGPU routing into FR-17 and Addendum §4.
2. **Zero-Duplication Single Source of Truth for RAG**:
   - A common pitfall in local AI setups is duplicating files into Open WebUI's local database or standalone vector stores. FR-15 strictly binds Open WebUI directly to Paperless-ngx, eliminating redundant disk usage and ensuring that document deletions or metadata updates are reflected immediately in AI answers.
3. **Dirty-Shutdown Resilience Prioritized Over Sub-10s Boot**:
   - Counter-metric SM-C2 strictly prohibits optimizing boot time below 10 seconds if it requires bypassing disk journal verification or SQLite/Postgres WAL checks. This directly serves the household appliance philosophy: sudden power cuts must never corrupt family records or media databases.
4. **Appliance Simplicity via Ecosystem Clients**:
   - Explicitly declining to build proprietary client apps (Section 5 Non-Goals) protects the project from maintenance overhead and fragmentation, ensuring users get world-class apps on Apple TV, Android TV, iOS, and Android on day one.
5. **Commercial Phasing & FOSS Integrity**:
   - FR-20 and Addendum §3 cleanly separate open-source software availability (free DIY installation across 3 clear hardware tiers) from future commercial hardware appliance sales, learning from open-source community backlash against restrictive relicensing.

---

## 4. Conclusion & Downstream Implementation Readiness

The PRD and Technical Addendum provide a complete, robust, and faithful realization of the user's requirements. No missing features, unhandled edge cases, or divergent directions were identified. The specifications are ready for downstream architectural design, container compose definitions, and sprint planning.
