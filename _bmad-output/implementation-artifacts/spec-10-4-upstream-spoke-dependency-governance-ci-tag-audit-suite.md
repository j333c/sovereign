---
title: 'Story 10.4: Upstream Spoke Dependency Governance & CI Tag Audit Suite'
type: 'feature'
created: '2026-11-10'
status: 'done'
baseline_commit: '1f262c6407fcf8f0f878f7be2e684444558e4e75'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/epics.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:**
Appliance reliability and stability depend on deterministic, repeatable container dependencies. Floating tags like `:latest`, `:2-alpine`, or `:main` introduce unvetted third-party breaking changes into appliances without human oversight or automated testing. Additionally, maintainers need an automated release publishing pipeline (`scripts/publish-release.sh`) that strictly validates SemVer release tags (`v<MAJOR>.<MINOR>.<PATCH>` for stable, `v<MAJOR>.<MINOR>.<PATCH>-rc.<N>` for preview), enforces clean git working state, executes the verification test suite, tags and pushes to dual remotes (`origin` and `github`), and asserts tag ref parity via `scripts/verify-remotes.sh`.

**Approach:**
1. **Pin All Spoke and Gateway Container Images:**
   - In `docker-compose.yml`, pin Caddy to `caddy:2.8.4-alpine` and local services away from `:latest`.
   - In `internal/supervisor/` (`supervisor.go`, `openwebui.go`, `llama.go`, `catalog.go`), verify and enforce pinned SemVer tags for Jellyfin (`jellyfin/jellyfin:12.2.0`), Navidrome (`deluan/navidrome:0.54.5`), Audiobookshelf (`advplyr/audiobookshelf:2.19.5`), Paperless-ngx (`paperlessngx/paperless-ngx:3.3.1`), Open WebUI (`ghcr.io/open-webui/open-webui:0.12.0`), and explicit upstream build tag for llama.cpp (`ghcr.io/ggml-org/llama.cpp:b11647` matching `^b[0-9]{4,}$`).
2. **Spoke Tag Audit Suite (`scripts/audit-spoke-tags.sh`):**
   - Scans `docker-compose.yml` and Go supervisor constants for image references.
   - Enforces strict pinning rules: fails with exit code 1 if any floating tag (`:latest`, `:2-alpine`, `:main`, etc.) or non-pinned format is found.
   - Provides `--check-upstream` and `--pr` options to query upstream OCI registries / release endpoints (Docker Hub, GHCR/GitHub), detect newer stable versions, and generate branch updates targeting active `release/v<MAJOR>.<MINOR>` branches.
3. **Release Publishing Tool (`scripts/publish-release.sh`):**
   - Validates release tag against strict SemVer regex: `v<MAJOR>.<MINOR>.<PATCH>` for stable, `v<MAJOR>.<MINOR>.<PATCH>-rc.<N>` for preview.
   - Asserts git working directory is clean (`git status --porcelain`).
   - Runs verification suites (`test-daemon`, `test-web`, `test-scripts`, `audit-spoke-tags`).
   - Creates annotated git tag and pushes to both `origin` and `github`.
   - Runs `scripts/verify-remotes.sh` to confirm remote tag ref parity.
   - Provides `--dry-run` and `--skip-tests` flags for safe execution and test coverage.
4. **Enhanced Dual-Remote Tag Parity (`scripts/verify-remotes.sh`):**
   - Add support for `-t, --tag <tag>` and `--tags` to verify tag parity between `origin` and `github`.
5. **Testing & Makefile Integration:**
   - Add `tests/audit_spoke_tags_test.sh` and `tests/publish_release_test.sh`.
   - Add `audit-spoke-tags` target to `Makefile` and include new tests in `test-scripts`.

</frozen-after-approval>
