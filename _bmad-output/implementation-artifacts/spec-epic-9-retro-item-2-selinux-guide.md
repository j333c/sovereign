---
title: 'SELinux Context Configuration and Troubleshooting Guide'
type: 'documentation'
created: '2026-11-09'
status: 'done'
route: 'oneshot'
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On Red Hat Enterprise Linux, Fedora, CentOS Stream, AlmaLinux, and Rocky Linux, SELinux runs in Enforcing mode by default. Standard container volume mounts under `/data/` and `/opt/sovereign` are blocked by default access control policies with `Permission denied` errors unless explicitly labeled with `container_file_t` or configured with container SELinux volume flags (`:z` / `:Z`). There is currently no documentation in the Sovereign guides explaining SELinux context configuration, troubleshooting, or persistence rules for users running Fedora/RHEL, and `install.sh` / `scripts/install.sh` does not proactively configure or verify SELinux contexts when SELinux is detected.

**Approach:**
1. Document SELinux context configuration for Red Hat / Fedora distributions in `docs/guides/en/GUIDES.md` and keep all native language guides (`docs/guides/de/ANLEITUNGEN.md`, `docs/guides/es/GUIAS.md`, `docs/guides/fr/GUIDES.md`, and `docs/guides/README.md`) synchronized with Section 6 covering SELinux container volume labels (`container_file_t`), temporary `chcon` labeling, permanent `semanage fcontext` policies, volume `:z` flags, and AVC denial audit troubleshooting.
2. Enhance `install.sh` and `scripts/install.sh` during `initialize_storage` to check `selinuxenabled` and automatically label `/data` and `/opt/sovereign` with `container_file_t` when running on SELinux-enforcing hosts, with calm, reassuring feedback.
3. Add automated test coverage in `tests/installer_test.sh` for SELinux detection handling.

</frozen-after-approval>

## Implementation Notes

- **Documentation Updates across 4 Locales (`docs/guides/`)**:
  - Added Section 6 to `docs/guides/en/GUIDES.md`, `docs/guides/de/ANLEITUNGEN.md`, `docs/guides/es/GUIAS.md`, and `docs/guides/fr/GUIDES.md`, with index reference in `docs/guides/README.md`.
  - Documented container volume contexts (`container_file_t`) required for `/data/` and `/opt/sovereign/`.
  - Explained temporary `chcon -Rt container_file_t` and persistent `semanage fcontext` policies surviving relabels and upgrades.
  - Cautioned against `:Z` (exclusive private volume) and mandated `:z` (shared volume) for multi-service data sharing across Jellyfin, Navidrome, and Paperless.
  - Documented external NAS mounts (SMB / NFS) with SELinux context mount options (`-o context="system_u:object_r:container_file_t:s0"`) and booleans (`virt_use_nfs`, `virt_use_samba`).
  - Added AVC denial troubleshooting with `ausearch -m avc -ts recent`, `journalctl -g AVC`, and `/var/log/audit/audit.log`.
- **Installer Automation (`scripts/install.sh` & `install.sh`)**:
  - Enhanced environment validation to inspect SELinux active status and mode via `getenforce`.
  - Added `policycoreutils` to `dnf` dependencies on Fedora/RHEL.
  - Configured automated persistent `semanage fcontext` registration (falling back safely to verified `chcon`) during storage deployment.
  - Preserved SELinux context integrity during `upgrade_sovereign`.
- **Verification**:
  - Enhanced `tests/installer_test.sh` with `test_selinux_detection` covering Enforcing, Permissive, and Disabled states.
  - All 12 test assertions passing cleanly in `make test-scripts`.
  - All 181 frontend unit and integration tests passing in `npm --prefix web test`.

## Review Triage Log

- **Reviewer**: Blind Hunter (Iter 1)
  - Finding 1: `selinuxenabled` unconditionally reported "Enforcing mode" even on Permissive systems.
    - *Verdict*: medium. Fixed by querying `getenforce` to report the true mode (Enforcing, Permissive, active).
  - Finding 2: `chcon` error suppression masked failures and logged false success.
    - *Verdict*: medium. Fixed by checking exit code and issuing `log_warn` on failure.
  - Finding 3: Installer lacked permanent `semanage fcontext` rules.
    - *Verdict*: medium. Fixed by adding automated `semanage fcontext` rule registration when present before falling back to `chcon`.
  - Finding 4: `upgrade_sovereign` did not maintain context on updated repository files.
    - *Verdict*: medium. Fixed by adding context check and re-application during upgrade before container restart.
  - Finding 5: `policycoreutils` missing from `dnf install` package list.
    - *Verdict*: low. Added `policycoreutils` to `dnf` dependency command.
  - Finding 6: Omission of SELinux section from in-app Help Center at `web/src/app/guides/page.tsx`.
    - *Verdict*: rejected (false/low by design). The web UI is targeted at non-technical household media/document consumers; host OS SELinux policy administration belongs in the developer/admin guides (`docs/guides/`).
  - Finding 7: Missing warning regarding `:Z` breaking multi-container volume sharing in `/data`.
    - *Verdict*: medium. Fixed across all 4 guides, explicitly recommending `:z` and warning against `:Z`.
  - Finding 8: Missing guidance for external NAS storage (NFS/SMB) under SELinux.
    - *Verdict*: medium. Fixed across all 4 guides, documenting mount `-o context=...` and booleans `virt_use_nfs` / `virt_use_samba`.
  - Finding 9: Test suite lacked permissive/disabled coverage and `chmod +x` assurance.
    - *Verdict*: low. Fixed in `tests/installer_test.sh` covering Enforcing, Permissive, and Disabled cases.
  - Finding 10: Spec and sprint status files need completion.
    - *Verdict*: high. Finalized with implementation notes and triage log.
  - Finding 11: `setroubleshoot` rarely present on minimal servers.
    - *Verdict*: medium. Added standard `journalctl -g AVC` and `grep AVC /var/log/audit/audit.log` across all 4 guides.


