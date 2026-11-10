# Changelog

All notable changes to the Sovereign home server appliance monorepo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Automated changelog & release notes generation integrated into release pipeline and annotated git tags.
- Extractor utility `scripts/extract-changelog.py` for parsing release notes directly into GitHub Releases and appliance update discovery summaries.

---

## [v1.0.0-rc.2] - 2026-11-10

### Added
- Official GitHub Actions release pipeline (`.github/workflows/release.yml`) cross-compiling static Go supervisor daemon binaries for `linux/amd64` and `linux/arm64`.
- Automated generation and publication of `SHA256SUMS.txt` with release assets on GitHub Releases.
- Continuous Integration workflow (`.github/workflows/ci.yml`) for pull requests and branch builds.
- Upstream Spoke Dependency Governance daily CI audit (`.github/workflows/audit-spoke-tags.yml`).

### Fixed
- Test suite runner isolation in `tests/publish_release_test.sh` to correctly detect dirty repository states in clean cloud CI environments via `SOVEREIGN_DIR`.
- Network independence in `tests/verify_remotes_test.sh` by replacing external remote URLs with local bare git fixtures, eliminating offline/air-gapped CI test failures.

---

## [v1.0.0-rc.1] - 2026-11-10

Initial Preview Release Candidate of the Sovereign Home Server Appliance Monorepo, completing Epics 1 through 10.

### Added
- **Turnkey Linux Host Installer & Host Provisioning Engine (Epic 9)**
  - One-line host provisioner (`install.sh` / `scripts/install.sh`) supporting Debian, Ubuntu, Fedora, and Arch Linux.
  - Multi-channel install flags (`--channel stable|security-only|preview`, defaulting to `stable`).
  - Automated Docker CE substrate installation, `/data` directory tree provisioning, and systemd service registration.
  - Unattended upgrade maintenance pipeline and safe uninstaller (`--uninstall`, `--purge`).
  - Automated SELinux context configuration for Red Hat / Fedora distributions.

- **Multi-Channel Release Distribution & Update Discovery Engine (Epic 10)**
  - Three distinct update channels (`Stable`, `Security-only`, and `Preview`).
  - Git-native update discovery engine polling `origin` over Smart HTTP (`git ls-remote --tags --refs origin`) throttled to at most once per hour.
  - SemVer 2.0.0 comparator enforcing monotonic upgrade floors, with "Graduating to Stable" holding pattern for preview appliances.
  - Amber `DestructiveModal` requiring localized confirmation token entry (`PREVIEW`, `VORSCHAU`, `PRELIMINAR`, `APERÇU`) for preview channel activation.
  - Morning dashboard banner notifying operators of unattended overnight update outcomes.
  - OCI spoke container image tagging governance enforcing explicit SemVer (`MAJOR.MINOR.PATCH`) or build numbers (`^b[0-9]{4,}$` for `llama.cpp`).
  - Dual-remote release publishing script (`scripts/publish-release.sh`) and parity verifier (`scripts/verify-remotes.sh`).

- **Operational Hardening & Resilience Safety Net (Epic 8)**
  - Database write-ahead log (WAL) checkpointing before snapshot restoration and rollback circuit breaker (max 3 restarts).
  - Pre-flight storage capacity guards on dropzone uploads and AI model downloads (+5 GB safety reserve).
  - Peripheral disconnect error recovery for optical disc drives and custom TLS certificate support for IMAP connections.
  - Critical-path fault injection test suite and persistent retry alert queue.

- **Appliance Foundation, Setup Wizard & Power Resilience (Epic 1)**
  - Go supervisor daemon architecture (`cmd/sovereign-daemon` and `internal/`) with SQLite WAL durability and zero CGO dependencies.
  - Dedicated Caddy ingress gateway handling ports `:8096`, `:8000`, `:3001`, `:4533`, `:13378` and subpaths (`/media`, `/docs`, `/chat`, `/music`, `/audiobooks`).
  - Local network discovery via mDNS / Zeroconf (`http://sovereign.local`).
  - 5-step guided first-run setup wizard (`/setup`) with animated transitions and RBAC password creation.
  - Post-power outage boot diagnostics screen (`/boot`) running filesystem integrity checks within 15 seconds.
  - Central administration dashboard (`/`) streaming real-time system telemetry via Server-Sent Events (SSE).

- **In-House Media Streaming & Staged Ingestion Dropzone (Epic 2)**
  - On-demand container lifecycle engine pulling official upstream OCI images strictly when features are enabled.
  - Standalone media dropzone (`/drop` and `\\sovereign\drop`) with atomic staging and category inspection.
  - Pre-configured Jellyfin video streaming with hardware transcoding fallback (Intel QuickSync, Nvidia NVENC, VAAPI).
  - Music and party playlists with Subsonic API via Navidrome, and audiobooks with multi-device sync via Audiobookshelf.

- **Physical Disc Automation & Off-Peak Transcoding Queue (Epic 3)**
  - Optical drive detection with automated CD, DVD, and Blu-ray disc ripping.
  - Automated media metadata and artwork enrichment via TMDb, IMDb, and MusicBrainz.
  - Scheduled off-peak video transcoding queue (01:00–07:00) with mobile offline profile generation.

- **Private Document Archiving & Smart Home Alert Bridge (Epic 4)**
  - Centralized document management and OCR indexing via Paperless-ngx.
  - Multi-channel document ingestion via mobile companion scanner and automated IMAP email attachment polling.
  - Deterministic regex and AI deadline extraction for contracts, warranties, and obligations.
  - Home Assistant (webhook/MQTT) and SMTP email alert dispatcher.

- **Local AI Knowledge & Turnkey Inference Platform (Epic 5)**
  - Managed `llama.cpp` inference engine backend with GGUF model administration in Settings.
  - Single-Source-of-Truth RAG querying Paperless-ngx directly via SQLite FTS5 without external vector database bloat.
  - Context Window Headroom Truncator reserving a 512-token response buffer.
  - Local browser chat interface via Open WebUI operating 100% offline.
  - Smart Hardware Sharing & dynamic GPU arbitration with Day AI Priority (07:00–01:00) and Night Batch Window (01:00–07:00).

- **Appliance Resilience, Safe Updates & System Administration (Epic 6)**
  - Declarative Compose v2 revisioning (`/data/state/compose/rev-:rev.yaml`) with pre-upgrade atomic database snapshots.
  - 180-second automated health verification with watchdog dampening and automatic rollback.
  - One-click manual rollback in Admin UI with version comparison diffs.
  - Full 4-language internationalization (English, German, Spanish, French) across all interfaces, errors, and reassurance dialogs.

- **Ecosystem Governance & Hardware Guidance (Epic 7)**
  - Turnkey hardware specifications across three tiers (Light Tier, Media Tier, AI & Power Tier).
  - Hardware calculator UI baseline fallback.
  - Public marketing site and open-source governance documentation with BMad Method attribution.
