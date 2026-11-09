---
title: 'Automated Containerized Installer Matrix Test'
type: 'feature'
created: '2026-11-09'
status: 'done'
route: 'oneshot'
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The turnkey one-line Linux host installer (`scripts/install.sh` and `install.sh`) must reliably validate prerequisites, detect OS distributions, and provision Sovereign dependencies across Debian, Ubuntu, and Fedora, but lacks an automated matrix test suite to verify execution behavior across these environments.

**Approach:** Implement `tests/installer_test.sh` to run automated syntax, dry-run, and containerized matrix tests verifying OS pre-flight validation, non-interactive execution flags (`-y`/`--yes`), upgrade, and uninstall flows across standard container images (Ubuntu, Debian, Fedora) with fallback mock simulation when Docker daemon is offline, and wire into `Makefile`.

</frozen-after-approval>

## Implementation Notes

- **Automated Matrix Test Suite (`tests/installer_test.sh`)**:
  - Implemented 12 test assertions covering:
    - Bash syntax verification (`bash -n`) for both `install.sh` and `scripts/install.sh`.
    - Strict file parity validation (`cmp`).
    - CLI options: `--help`, `-h`, invalid arguments, non-interactive execution (`-y` / `--yes`).
    - Non-root user privilege checking and error exit code enforcement.
    - Safe pre-flight dry-run validation (`--dry-run` / `--check`).
    - Uninstallation dry-run simulation (both standard and `--purge` modes).
    - Upgrade workflow dry-run simulation.
    - Mock OS distribution matrix (Ubuntu 22.04, Debian 12, Fedora 39, Arch Linux) via `OS_RELEASE_FILE` injection.
    - Live container matrix across `ubuntu:22.04`, `debian:12`, and `fedora:39` with non-blocking graceful fallback when Docker socket is unavailable.
- **Installer Enhancements (`scripts/install.sh` & `install.sh`)**:
  - Added `--dry-run` / `--check` flags allowing pre-flight inspection without modifying host filesystem or package managers.
  - Added support for `OS_RELEASE_FILE` override environment variable for mock test isolation.
  - Standardized disk space checks using POSIX `df -P -m /` to avoid multi-line formatting anomalies across non-GNU utilities.
  - Embedded dry-run checks into `uninstall_sovereign()` and `upgrade_sovereign()` so simulations cannot execute destructive host alterations.
  - Added `display_upgrade_banner()` for clean user feedback during upgrades.
- **Build System Integration**:
  - Added `test-installer` to `Makefile` and chained into `test-scripts` and overall `test` targets.
  - Verified `make test-scripts` passes seamlessly.

## Review Triage Log

- **Reviewer**: Blind Hunter (Iter 1)
  - Finding 1: Uninstallation and upgrade dry-run flags executed live actions if invoked before dry-run exit.
    - *Resolution*: Implemented dry-run simulation guards directly within `uninstall_sovereign` and `upgrade_sovereign` printing `[DRY-RUN]` steps without running commands.
  - Finding 2: Root privilege check prevented dry-run validation by non-root users.
    - *Resolution*: Permitted non-root execution when `--dry-run` or `--check` is passed, logging a warning while continuing prerequisite audits.
  - Finding 3: Containerized live tests caused test aborts if Docker daemon was offline or running in unprivileged container sandboxes.
    - *Resolution*: Added pre-flight Docker daemon check in test harness with graceful `SKIP` reporting and skip-count accounting.
  - Finding 4: Inconsistent disk space parsing on busybox/BSD `df`.
    - *Resolution*: Replaced `df -m /` with standard POSIX format `df -P -m / | awk 'NR==2 {print $4}'`.


