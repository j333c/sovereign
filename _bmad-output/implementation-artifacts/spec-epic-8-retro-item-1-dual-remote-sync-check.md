---
title: 'Dual-Remote Git Sync Sanity Check Script'
type: 'feature'
created: '2026-11-09'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Developers and maintainers need an automated sanity check utility to verify that the internal authoritative repository (`origin/main`) and the public distribution repository (`github/main`) maintain exact commit parity and clean dual-remote synchronization before tagging releases or closing milestones.

**Approach:** Implement `scripts/verify-remotes.sh` to validate git remote URLs, compare commit SHAs between `origin/main` and `github/main` (supporting offline reference checks and optional remote fetching), output calm and reassuring colored status indicators, and exit with code 0 on parity and non-zero on divergence.

</frozen-after-approval>

## Implementation Notes

- Implemented `scripts/verify-remotes.sh` with flags `-b`/`--branch`, `-f`/`--fetch`, `-q`/`--quiet`, and `-h`/`--help`.
- Validates git environment, worktree, and presence of both `origin` and `github` remotes with distinct URL enforcement.
- Uses fully qualified tracking refs (`refs/remotes/origin/${BRANCH}`, `refs/remotes/github/${BRANCH}`) to prevent ambiguous local ref resolution.
- Compares commit SHAs, reports commit subjects and URLs, outputs reassuring colored status badges, and checks local HEAD parity (including detached HEAD environments).
- Exits code 0 on exact parity; exits code 1 on divergence, missing remotes, or failed fetch, routing diagnostic outputs and remediation steps to stderr.
- Added automated test suite `tests/verify_remotes_test.sh` covering 12 scenarios with subshell isolation and mock temporary git sandboxes.
- Integrated `verify-remotes` and `test-scripts` targets into `Makefile` and hooked `test-scripts` into `make test`.
- Updated `_bmad-output/implementation-artifacts/sprint-status.yaml` marking `epic-8-retro-item-1-dual-remote-sync-check` as `done`.

## Review Triage Log

- `medium` (`patch`): In `tests/verify_remotes_test.sh`, tests running inside `if "$@"` disabled `set -e` in bash. Fixed by invoking tests inside subshells `( "$@" )`.
- `medium` (`patch`): In `tests/verify_remotes_test.sh`, `cd "$repo"` mutated test runner working directory across tests. Fixed by subshell isolation per test.
- `medium` (`patch`): In `scripts/verify-remotes.sh`, failure of `--fetch` silently proceeded to verify stale local refs. Fixed by making fetch failure exit with code 1.
- `low` (`patch`): In `scripts/verify-remotes.sh`, remedy when GitHub is ahead was inaccurate. Fixed to instruct pushing GitHub's ref to origin.
- `low` (`patch`): In `scripts/verify-remotes.sh`, error diagnostics on divergence were written to stdout. Fixed to route diagnostics to stderr (`>&2`).
- `low` (`patch`): In `scripts/verify-remotes.sh`, "Local branch is clean" wording implied working tree file status. Clarified wording to "Local HEAD commit matches both remotes".
- `low` (`patch`): In `scripts/verify-remotes.sh`, `echo -e` with commit subject expanded escape sequences. Replaced with safe `printf`.
- `low` (`patch`): In `scripts/verify-remotes.sh`, ambiguous short refs `origin/${BRANCH}` replaced with fully qualified `refs/remotes/origin/${BRANCH}`.
- `low` (`patch`): In `Makefile`, `tests/verify_remotes_test.sh` was not hooked into `make test`. Added `test-scripts` and integrated into `test`.
- `low` (`patch`): In `scripts/verify-remotes.sh`, added check ensuring `origin` and `github` do not point to identical URLs.
- `low` (`patch`): In `scripts/verify-remotes.sh`, added support for detached HEAD commit comparison.
- `low` (`patch`): In `tests/verify_remotes_test.sh`, expanded test cases from 6 to 12 covering all edge cases.
