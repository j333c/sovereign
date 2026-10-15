# Epic 1 Context: Appliance Foundation, Setup Wizard & Power Resilience

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Establish the core appliance substrate and onboarding experience for Sovereign: bootstrap the decoupled Go supervisor daemon and Next.js presentation layers, configure local network mDNS discovery and dedicated Caddy ingress, ensure power-loss resilience with dirty-boot diagnostics and SQLite WAL durability, manage local disk partitions and external NAS mounts, guide first-time users through the 5-step First-Run Setup Wizard with RBAC session security, and deliver the responsive central admin dashboard with real-time SSE telemetry. This foundation ensures the home server operates as a trustworthy, turnkey consumer appliance without exposing host Docker sockets or requiring command-line intervention.

## Stories

- Story 1.1: Greenfield Monorepo Scaffold & Core Supervisor Daemon Boundary
- Story 1.2: Dedicated Caddy Ingress Gateway & Local mDNS Discovery
- Story 1.3: Power-Loss Resilience & Reassuring Boot Diagnostics Screen
- Story 1.4: Storage Pool Formatting & Network NAS Mount Management
- Story 1.5: Five-Step First-Run Setup Wizard & RBAC Authentication
- Story 1.6: Central Admin Dashboard & Real-Time SSE Telemetry Stream

## Requirements & Constraints

- **Control Plane Isolation**: The web presentation client must never mount the host Docker socket or execute host shell commands; all system telemetry, container orchestration, and hardware inspection execute strictly via HTTP REST and SSE APIs exposed by the privileged Go supervisor daemon.
- **Local Network Discovery & Ingress**: Broadcast `sovereign.local` via mDNS on the local subnet within 30 seconds of boot. Direct access via assigned DHCP IP address must function identically on routers where mDNS multicast is disabled. Dual-path ingress must route traffic seamlessly via dedicated ports and reverse-proxied subpaths.
- **Power-Loss Resilience & Boot Timing**: Cold boot to an operational administrative interface must complete within 60 seconds, with the core interface initializing within 15 seconds of kernel boot. All state and application databases must enforce write-ahead logging (WAL) and synchronous normal mode to survive sudden power cuts with zero corruption.
- **Unclean Shutdown Recovery**: A clean shutdown marker (`boot_state.json`) tracks system state. If missing or false after an ungraceful power loss, the daemon enters diagnostics mode, locks the browser interface to `/boot`, runs database integrity checks and validates filesystem journal mounts, and launches upstream application containers only after verification passes.
- **Storage Pool & External NAS Handling**: Support formatting and mounting internal SATA/NVMe storage partitions under a unified `/data` path, as well as mounting external NFS/SMB shares with credential testing. External NAS disconnections must transition affected services to an Amber degraded state without crashing container runtimes.
- **First-Run Onboarding**: First boot must route directly to `/setup`, guiding the user through language selection, network verification, storage pool or NAS configuration, initial feature selection, and admin credentials with an alert email. Completing the wizard must transition immediately to the operational dashboard without requiring a manual system reboot.
- **Access Control & Session Security**: Public unauthenticated LAN access is restricted strictly to the dashboard (`/`), media dropzone (`/drop`), and boot diagnostics (`/boot`). System settings, configuration changes, and service control actions require an authenticated session backed by an `HttpOnly`, `SameSite=Strict` cookie, using Argon2id password hashing.
- **Telemetry & Consumer Voice**: Dashboard Time-to-Interactive (TTI) must stay under 2.0 seconds on local gigabit LAN. Telemetry updates stream via Server-Sent Events without client-side polling. All user-facing screens, alerts, and tooltips must use reassuring consumer language, completely free of raw Linux terminal logs, stack traces, and error numbers.

## Technical Decisions

- **Monorepo Layout**: Segregated Go daemon (`/cmd/sovereign-daemon`, `/internal/{supervisor,storage,store,auth,ingress,hardware,ai}`) and Next.js frontend (`/web` App Router, React 19, Tailwind CSS v4, shadcn/ui).
- **API Contract & Client Generation**: All daemon endpoints are defined in a unified OpenAPI 3.1 specification. The frontend TypeScript API client is strictly code-generated from this contract.
- **Network Architecture**: Docker bridge network segregation separating ingress (`sovereign-ingress` for Caddy, Web UI, public API) from daemon administration (`sovereign-control` for daemon, Docker socket, Caddy Admin API).
- **Ingress Gateway Integration**: Dedicated Caddy container routing external traffic, dynamically managed by the daemon via Caddy Admin API (`http://caddy:2019`). Supports path prefix stripping via `handle_path` and `X-Forwarded-Prefix` headers.
- **Storage Hierarchy & State Persistence**: Partitioned single mount at `/data` (`/data/media`, `/data/documents`, `/data/models`, `/data/state`, `/data/drop`). State store at `/data/state/sovereign.db` configured with `PRAGMA journal_mode = WAL;` and `PRAGMA synchronous = NORMAL;`.
- **Atomic State Writes**: All state configuration writes to `/data/state/` must write to a temporary file, execute `fsync()`, and perform an atomic rename.
- **Conventions & Error Format**: PascalCase for Go structs and UI components; kebab-case for files and route segments (`/api/v1/:resource`). SSE telemetry uses `domain.action` event naming. HTTP error responses adhere to the RFC 7807 Problem Details standard. Entity identifiers use UUIDv7.

## UX & Interaction Patterns

- **Visual Design Identity**: Nordic/Braun minimalist aesthetic with light neutral canvas (`#F8F9FA`), white card surface (`#FFFFFF`), hairline borders (`#E5E7EB`), obsidian text (`#0F172A`), and blue accent/focus ring (`#2563EB`).
- **Semantic Status Triad**: Accessibility-compliant status badges pairing color with geometric micro-shapes:
  - Green Circle (●): Running Healthy.
  - Amber Diamond (◆): Attention Required / Degraded / Warming Up.
  - Red Triangle (▲): Stopped / Error.
- **AppHeader (64px)**: Persistent top navigation bar displaying Sovereign crest, aggregate health pill with expandable alert popover, quick links to `/drop` and `/settings`, and immediate language switcher (EN, DE, ES, FR).
- **Admin Dashboard Layout**: Enforces a single-viewport layout (`100vh; overflow: hidden`) on desktop (1024px+) for standard services with pinned header and pinned 56px telemetry bar. Automatically unlocks internal vertical scrolling for 5+ cards or viewports under 700px. Responsive grid shifts to 2 columns on tablet (768px) and single column on mobile (<768px).
- **Telemetry Bar (56px)**: Pinned bottom bar displaying visual storage pool capacity with warning color shifts above 85%, NAS connection status, and hardware sharing badge, streaming real-time SSE updates using `aria-live="polite"`.
- **First-Run Setup Wizard (`/setup`)**: 5-step guided flow with progress indicator, inline validation, network checkmark, and zero-reboot transition to `/`.
- **Boot Diagnostics Screen (`/boot`)**: Renders within 15 seconds of boot after an unclean shutdown. Displays circular progress indicator, reassuring consumer copy (*"Checking system safety after power restoration... your files are secure."*), and auto-redirects to `/` within 2 seconds of passing integrity checks.
- **Interaction Standards**: Minimum 44x44px touch targets across all mobile/tablet views; 2px high-visibility focus rings (`:focus-visible`); full keyboard traversal (`Tab`, `Enter`, `Escape`).

## Cross-Story Dependencies

- **Story 1.1 (Scaffold & Daemon Boundary)** provides the foundation (monorepo directory tree, SQLite WAL engine, Docker Compose substrate, OpenAPI contract) needed by all subsequent stories.
- **Story 1.2 (Caddy Gateway & mDNS)** establishes the network ingress routing required to expose the web frontend, setup wizard, diagnostics screen, and API endpoints at `sovereign.local`.
- **Story 1.3 (Boot Diagnostics)** builds the boot lifecycle engine (`boot_state.json`) and `/boot` diagnostic UI that guards startup before routing to `/setup` or `/`.
- **Story 1.4 (Storage Management)** implements internal disk partitioning and NAS mount verification, which is called directly by Step 3 of the setup wizard (Story 1.5).
- **Story 1.5 (Setup Wizard & Auth)** implements the onboarding flow and RBAC session security required to configure the system and authorize access to administrative dashboard features.
- **Story 1.6 (Central Dashboard & Telemetry)** integrates the authenticated frontend shell, AppHeader, ServiceCard layout, and SSE telemetry feeds produced by the daemon and ingress layers.
- **Downstream Epic Impact**: All subsequent feature epics (Epic 2 Media, Epic 3 Optical/Transcoding, Epic 4 Documents, Epic 5 AI, Epic 6 Maintenance) depend directly on the daemon control plane, `/data` storage mounts, Caddy ingress gateway, and dashboard service cards established in Epic 1.
