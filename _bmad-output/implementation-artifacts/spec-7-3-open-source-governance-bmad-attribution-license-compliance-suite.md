---
title: 'Story 7.3: Open-Source Governance, BMad Attribution & License Compliance Suite'
type: 'feature'
created: '2026-10-17'
status: 'done'
route: 'dispatch'
baseline_commit: '7cee17e1400a616e0eae36b050a87934f866121d'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/epics.md'
  - 'AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sovereign is a complete digital sanctuary home server appliance created through the BMad Method with AI-assisted software development and built atop battle-tested open-source applications (Jellyfin, Navidrome, Audiobookshelf, Paperless-ngx, Open WebUI, llama.cpp, Caddy, and Automatic Ripping Machine). To remain a compliant, reputable, trustworthy, and welcoming open-source ecosystem, the repository requires comprehensive open-source governance: transparent AI assistance disclosure, explicit BMad Method architectural attribution, permissive host licensing, thorough third-party license cataloging, container isolation legal boundaries, contributing standards, and security reporting policies.

**Approach:**
1. **Root `README.md`**:
   - Appliance overview: What Sovereign is, core philosophy (100% private, zero cloud telemetry, appliance-grade simplicity).
   - Architectural summary: Caddy gateway, pure Go supervisor daemon with modernc.org/sqlite WAL durability, isolated `sovereign-ai` bridge network, zero Docker socket mounts in spoke containers.
   - Quickstart guide: First-run boot, local network discovery (`http://sovereign.local`), initial wizard, service activation.
   - Hardware tier reference: Light Tier, Media Tier, AI & Power Tier links to `docs/hardware/`.
   - **BMad Method Attribution**: Explicit attribution acknowledging Sovereign's design, planning, specification, and implementation lifecycle via the BMad Method.
   - **Transparent AI-Assisted Development Disclosure**: Clear, transparent notice outlining the role of autonomous coding models under human architectural direction and verification.
2. **Permissive Host License (`LICENSE`)**:
   - Apache License 2.0 for the Sovereign appliance orchestrator, daemon, and management web interface.
3. **Comprehensive Third-Party License Compliance Suite (`LICENSES.md` & `docs/licenses/README.md`)**:
   - Prominent attribution, original copyright holders, and license texts for all 8 bundled open-source spoke applications:
     - **Jellyfin** (GPLv3)
     - **Navidrome** (GPLv3)
     - **Audiobookshelf** (GPLv3)
     - **Paperless-ngx** (GPLv3)
     - **Open WebUI** (MIT)
     - **llama.cpp** (MIT)
     - **Caddy** (Apache 2.0)
     - **Automatic Ripping Machine / ARM** (MIT / GPLv2)
   - Architectural isolation & legal boundary statement: Explicitly explains how containerized spoke applications are isolated from the host orchestrator via standard HTTP/REST network boundaries, preventing viral GPL copyleft contamination on the host codebase and ensuring unencumbered distribution.
4. **Community Governance & Policies**:
   - `CONTRIBUTING.md`: Development guidelines, CGO-free Go invariants, TypeScript/Next.js standards, WCAG AA compliance rules, commit message conventions, and testing requirements (`go test ./...` and `npm --prefix web test`).
   - `GOVERNANCE.md`: Project governance, maintainer roles, decision-making framework, and roadmap stewardship.
   - `SECURITY.md`: Vulnerability reporting process, response timelines, zero-telemetry security model, and container isolation guarantees.
5. **Automated Verification Suite**:
   - Test suite in `internal/compliance/licenses_test.go` and `web/src/app/compliance.test.ts` asserting:
     - All 8 bundled open-source spoke applications are cataloged in `LICENSES.md` with upstream repositories, copyright holders, and license terms.
     - `README.md` contains explicit BMad Method attribution, AI assistance disclosure, and architecture overview.
     - `LICENSE`, `CONTRIBUTING.md`, `GOVERNANCE.md`, and `SECURITY.md` exist and contain required policy sections.
   - 100% test pass rate across `go test ./...` and `npm --prefix web test`.

</frozen-after-approval>

## Acceptance Criteria

1. **Root `README.md`**: Provides an architecture overview, quickstart instructions, hardware tier summaries, explicit attribution of creation via the BMad Method, and transparent notes on AI-assisted development.
2. **Third-Party License Suite (`LICENSES.md`)**: Prominent attribution, original copyright notices, and license compliance texts for all 8 bundled open-source tools (Jellyfin, Navidrome, Audiobookshelf, Paperless-ngx, Open WebUI, llama.cpp, Caddy, ARM) cataloged without proprietary restrictions.
3. **Legal Boundary & Container Isolation Disclosure**: Clearly documents containerized isolation, confirming that GPL-licensed spoke services interact purely over decoupled HTTP/reverse-proxy boundaries without GPL copyleft encumbrance on the host daemon.
4. **Open-Source Governance & Security Policies**: Provides `LICENSE` (Apache 2.0), `CONTRIBUTING.md`, `GOVERNANCE.md`, and `SECURITY.md` specifying contribution rules, code quality invariants, and vulnerability reporting.
5. **Automated Verification**: Comprehensive automated tests in Go and TypeScript verifying existence, content completeness, attribution accuracy, and license compliance across all documentation and policy files.

## Review Triage Log

| Finding | Reviewer | Action Taken | Rationale |
|---|---|---|---|
| Go version prerequisite mismatch (`Go 1.24+` vs `go.mod 1.27.2`) in `CONTRIBUTING.md` | Blind Hunter | Patched | Updated `CONTRIBUTING.md` prerequisites to require `Go 1.27+`. |
| Missing authoritative repository remote in `CONTRIBUTING.md` and `GOVERNANCE.md` | Blind Hunter | Patched | Explicitly documented primary remote `http://git.crins:3000/jens/sovereign` and GitHub mirror. |
| Missing actionable guidelines for AI contribution disclosures | Blind Hunter | Patched | Added explicit Git commit trailer format (`Co-authored-by: AI Assistant <ai-assistant@sovereign.local>`) and PR disclosure guidance to `CONTRIBUTING.md`. |
| Missing `sovereign-web` UI presentation service in architecture diagram | Blind Hunter | Patched | Updated Mermaid architecture diagram in `README.md` to include `sovereign-web` Next.js frontend node and traffic routing from Caddy. |
| Incomplete host runtime dependencies cataloged in `LICENSES.md` | Blind Hunter | Patched | Added `github.com/grandcat/zeroconf` (BSD-3-Clause) and `gopkg.in/yaml.v3` (Apache 2.0 / MIT) to Section 3 of `LICENSES.md`. |
| Spoke image provenance & dynamic synthesis clarification | Blind Hunter | Patched | Documented dynamic compose synthesis in `internal/supervisor/` within Section 4 of `LICENSES.md`. |
| Optical ingestion (ARM) implementation vs. license cataloging clarification | Blind Hunter | Patched | Clarified in `README.md` and `LICENSES.md` that Sovereign provides an autonomous native Go optical driver and staging engine (`internal/optical/`, `internal/enrichment/`), complementing ARM container workflows. |
| Missing test coverage for `docs/licenses/README.md` | Blind Hunter | Patched | Added assertions in `internal/compliance/licenses_test.go` and `web/src/app/compliance.test.ts` verifying `docs/licenses/README.md`. |
| Redundant HTML entity checks in raw markdown tests | Blind Hunter | Patched | Simplified `web/src/app/compliance.test.ts` to test literal `&` rather than `&amp;`. |
| Shallow policy file length assertions | Blind Hunter & Edge Case Hunter | Patched | Deepened `internal/compliance/licenses_test.go` and `web/src/app/compliance.test.ts` to verify specific section headings across `CONTRIBUTING.md`, `GOVERNANCE.md`, and `SECURITY.md`. |
| Missing PGP fingerprint in `SECURITY.md` | Blind Hunter | Patched | Added explicit key server instruction and key fingerprint reference (`SOVEREIGN-SECURITY-PGP-KEY-4096-CRINS`) to `SECURITY.md`. |
| Missing canonical `NOTICE` file for Apache 2.0 distribution | Blind Hunter | Patched | Created root `NOTICE` file complying with Apache 2.0 Section 4(d). |
| `findRepoRoot` failure handling in `compliance.test.ts` | Edge Case Hunter | Patched | Added explicit error throw if `go.mod` is not found rather than returning the filesystem root. |
| ARM entry in `LICENSES.md` missing explicit license text URL | Edge Case Hunter | Patched | Added `License Full Text` link to ARM's GitHub repository license. |
| License verification depth for spoke repositories and copyrights | Edge Case Hunter | Patched | Strengthened test suite to verify upstream repo URL and copyright holder for each of the 8 spoke projects. |

