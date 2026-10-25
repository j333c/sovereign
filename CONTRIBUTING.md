# Contributing to Sovereign

Thank you for your interest in contributing to Sovereign! Sovereign is a community-driven, 100% private self-hosted home server appliance. We welcome code contributions, hardware compatibility reports, translation improvements, and documentation fixes.

---

## Code of Conduct

We are committed to providing a welcoming, respectful, and harassment-free environment for everyone. Please treat all contributors and community members with dignity, kindness, and constructive empathy.

---

## Core Architectural Invariants

All contributions must preserve Sovereign's foundational architectural guarantees:

1. **Zero External Telemetry**: No third-party tracking scripts, cloud analytics pings, external font downloads, or unapproved remote API calls. All assets must be bundled locally.
2. **Air-Gapped AI Isolation**: The `sovereign-ai` Docker bridge must remain without external internet routing.
3. **Zero Docker Socket Exposure**: Spoke application containers must never mount `/var/run/docker.sock`. All lifecycle management is mediated by the host Go daemon.
4. **CGO-Free WAL Durability**: The Go daemon must compile without CGO (`modernc.org/sqlite`) and configure SQLite with WAL mode (`PRAGMA journal_mode=WAL`) and synchronous disk durability.
5. **Reassuring Consumer Microcopy**: Error messages and UI notices must never present raw Linux stack traces or network exit codes. Microcopy must explain the situation calmly and outline clear next steps.
6. **WCAG AA Accessibility**: Web UI components must provide semantic HTML, WAI-ARIA roles, 44px minimum touch targets, visible keyboard focus rings, and high-contrast color pairings.
7. **Multilingual Parity**: Any new user-facing string added to `web/src/lib/i18n.ts` must be present across all four supported languages: English (`en`), German (`de`), Spanish (`es`), and French (`fr`).

---

## Development Workflow

### Repository & Upstream Remotes
Sovereign maintains a dual-remote repository architecture:
- **Public Community & Distribution Repo**: `https://github.com/j333c/sovereign` (for anonymous users, installation, documentation, issues, and public contributions)
- **Internal Development Repo**: `http://git.crins:3000/jens/sovereign` (internal team development remote)

Community contributors can clone directly from GitHub:
```bash
git clone https://github.com/j333c/sovereign.git
cd sovereign
```

### Prerequisites
- Go 1.27+
- Node.js 22+ and npm
- Docker Engine & Docker Compose v2

### Local Verification
Before submitting any pull request or commit, verify that both the Go and TypeScript test suites pass completely:

```bash
# Run all Go backend unit and integration tests
go test -count=1 ./...

# Run all Next.js frontend unit tests and typecheck
npm --prefix web test
```

### Git Commit Conventions & AI Disclosures
Follow conventional commit messaging with clear, descriptive prefixes:
- `feat(scope): ...`
- `fix(scope): ...`
- `docs(scope): ...`
- `test(scope): ...`
- `refactor(scope): ...`

If you use AI assistants (e.g. Gemini, Claude, ChatGPT, GitHub Copilot) to generate or refactor code, include an explicit Git commit trailer or pull request declaration:
```git
feat(optical): add audio CD metadata caching

Co-authored-by: AI Assistant <ai-assistant@sovereign.local>
```

---

## Reporting Issues

- **Bug Reports**: Open an issue detailing your appliance hardware (CPU, RAM, storage), browser/client OS, reproduction steps, and relevant log excerpts.
- **Hardware Profile Tests**: Tested Sovereign on mini-PCs, SBCs, or custom hardware? Share your idle power draw and transcode results in a Hardware Profile PR to `docs/hardware/`.
