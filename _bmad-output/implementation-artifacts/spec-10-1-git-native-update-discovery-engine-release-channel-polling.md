---
title: 'Story 10.1: Git-Native Update Discovery Engine & Release Channel Polling'
type: 'feature'
created: '2026-11-10'
status: 'done'
baseline_commit: 'e04616d41a4a03429dd169c9ff42c26f00dbbe43'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/epics.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:**
Appliance update discovery previously lacked a rate-limit-free upstream update detection mechanism. Querying GitHub's REST API without credentials causes immediate failure or rate-limiting (HTTP 403) when multiple appliances share a public IP (CGNAT / home ISPs). Furthermore, the daemon container must discover updates natively via git over Smart HTTP without peeled tag duplicates, accurately parse SemVer 2.0.0 numeric `-rc.<N>` precedence, evaluate candidate tags against the configured channel (`stable`, `security-only`, or `preview`) with monotonic upgrade floor enforcement, and fetch changelog summaries via shallow tag fetches to emit SSE telemetry.

**Approach:**
1. **Container & Environment Substrate:**
   - Install `git` in `Dockerfile.daemon` and configure `git config --global --add safe.directory /opt/sovereign`.
   - Update `docker-compose.yml` daemon volume mounts and environment to support `/opt/sovereign` bind-mount and configurable `SOVEREIGN_REPO_DIR`.
2. **SemVer 2.0.0 Parser (`internal/supervisor/semver.go`):**
   - Parse `v<MAJOR>.<MINOR>.<PATCH>` and pre-release `-rc.<N>`.
   - Implement strict numeric `-rc.<N>` precedence (`v1.2.0-rc.1 < v1.2.0-rc.2 < v1.2.0-rc.10 < v1.2.0`).
   - Implement channel comparison logic for `stable`, `security-only`, and `preview`.
3. **Git-Native Update Discovery Engine (`internal/supervisor/discovery.go`):**
   - Query upstream tags via `git ls-remote --tags --refs origin` in repo directory.
   - Determine current version from `git -C <repo> describe --tags --exact-match` falling back to build-time `AppVersion`.
   - Filter candidate tags according to configured channel and monotonic upgrade floor (`candidate > current`).
   - When a newer tag is detected, execute shallow fetch `git -C <repo> fetch --depth=1 origin tag <target_tag>` and extract annotated subject via `git tag -l --format='%(contents:subject)' <target_tag>`.
   - Emit `system.update_available` SSE event with target tag, channel name, and changelog summary.
   - Run hourly background worker ticker. Handle network / DNS failures gracefully with quiet debug logs without failing health checks.

## Boundaries & Constraints

**Always:**
- Use `git ls-remote --tags --refs` to omit peeled `^{}` references.
- Enforce monotonic upgrade floor: `SemVer2(candidate) > SemVer2(current)`.
- Support `stable`, `security-only`, and `preview` channels.
- In `preview`, select newest pre-release or stable tag.
- In `security-only`, restrict updates to patch increments within the current `MAJOR.MINOR` series (or stable graduation for the current patch).
- In `stable`, require release tags without pre-release suffixes.
- Fall back gracefully to `AppVersion` when git tag describe fails.

**Never:**
- Never call the GitHub REST API (`api.github.com`).
- Never fail health checks or trigger alarm state due to network or DNS errors during update polling.

</frozen-after-approval>

## Code Map

- `Dockerfile.daemon` -- Add `git` package and `safe.directory /opt/sovereign`.
- `docker-compose.yml` -- Configure repo volume mount and `SOVEREIGN_REPO_DIR`.
- `internal/supervisor/semver.go` -- SemVer 2.0.0 parser, numeric rc comparator, and channel tag selectors.
- `internal/supervisor/semver_test.go` -- Unit tests for SemVer parsing, precedence, and channel selection.
- `internal/supervisor/discovery.go` -- Git-native update discovery engine, tag fetcher, and background poller.
- `internal/supervisor/discovery_test.go` -- Tests for update discovery engine with mock git repository and network error handling.
- `cmd/sovereign-daemon/server.go` -- Wire `UpdateDiscoveryEngine` into server lifecycle and SSE event dispatch.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- Track story status.

## Verification Plan

### Automated Tests
1. `go test -count=1 ./internal/supervisor/...`
   - Test SemVer 2.0.0 precedence (`v1.2.0-rc.1 < v1.2.0-rc.2 < v1.2.0-rc.10 < v1.2.0`).
   - Test channel filtering for `stable`, `security-only`, and `preview`.
   - Test update discovery against mock git repo and simulated network timeouts.
2. `make test-daemon`
   - Ensure all Go supervisor packages pass 100%.
3. `make test-web`
   - Ensure frontend tests remain 100% green.
