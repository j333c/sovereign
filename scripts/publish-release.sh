#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Release Publishing Pipeline
# Repository: https://github.com/j333c/sovereign
# License: Apache 2.0
#
# Enforces SemVer tagging conventions:
#   - Stable:  v<MAJOR>.<MINOR>.<PATCH>
#   - Preview: v<MAJOR>.<MINOR>.<PATCH>-rc.<N>
#
# Validates clean working state, runs the comprehensive test and spoke audit
# suites, creates an annotated git tag, pushes to dual remotes ('origin' and
# 'github'), and verifies tag ref parity via scripts/verify-remotes.sh.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TAG=""
DRY_RUN=false
SKIP_TESTS=false
ALLOW_DIRTY=false
QUIET=false

# Styling & Colors
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  BOLD="\033[1m"
  GREEN="\033[0;32m"
  BLUE="\033[0;34m"
  YELLOW="\033[0;33m"
  RED="\033[0;31m"
  CYAN="\033[0;36m"
  RESET="\033[0m"
else
  BOLD=""
  GREEN=""
  BLUE=""
  YELLOW=""
  RED=""
  CYAN=""
  RESET=""
fi

log_info() {
  if [ "$QUIET" = false ]; then
    printf "${BLUE}[INFO]${RESET} %s\n" "$1"
  fi
}

log_ok() {
  printf "${GREEN}[OK]${RESET} %s\n" "$1"
}

log_warn() {
  printf "${YELLOW}[WARN]${RESET} %s\n" "$1"
}

log_error() {
  printf "${RED}[ERROR]${RESET} %s\n" "$1" >&2
}

show_help() {
  cat << EOF
Sovereign Release Publishing Pipeline

Usage:
  $(basename "$0") [options] <tag>

Tag Format:
  Stable:  v<MAJOR>.<MINOR>.<PATCH>       (e.g. v1.0.0)
  Preview: v<MAJOR>.<MINOR>.<PATCH>-rc.<N> (e.g. v1.0.0-rc.1)

Options:
  -d, --dry-run        Validate tag format, working tree, and test suites without creating or pushing tag
  --skip-tests         Skip running full verification test suites (for dry-run/mock testing)
  --allow-dirty        Bypass clean working tree assertion (for testing only)
  -q, --quiet          Suppress informational messages
  -h, --help           Show this help documentation

Exit Status:
  0   Release tag published and verified in parity on dual remotes.
  1   Validation failure, dirty working tree, failing test, or remote divergence.

Examples:
  $(basename "$0") v1.0.0                 # Publish stable release v1.0.0
  $(basename "$0") v1.1.0-rc.1            # Publish preview release candidate
  $(basename "$0") --dry-run v1.0.0       # Validate release readiness without tagging/pushing
EOF
}

# Parse Arguments
while [ $# -gt 0 ]; do
  case "$1" in
    -d|--dry-run)
      DRY_RUN=true
      shift
      ;;
    --skip-tests)
      SKIP_TESTS=true
      shift
      ;;
    --allow-dirty)
      ALLOW_DIRTY=true
      shift
      ;;
    -q|--quiet)
      QUIET=true
      shift
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    -*)
      log_error "Unknown option: $1"
      echo "Run '$(basename "$0") --help' for usage." >&2
      exit 1
      ;;
    *)
      if [ -z "$TAG" ]; then
        TAG="$1"
        shift
      else
        log_error "Unexpected additional argument: $1"
        exit 1
      fi
      ;;
  esac
done

if [ -z "$TAG" ]; then
  log_error "Missing required release tag argument."
  echo "Run '$(basename "$0") --help' for usage." >&2
  exit 1
fi

cd "$REPO_ROOT"

# 1. Enforce SemVer Tagging Convention
STABLE_PATTERN='^v[0-9]+\.[0-9]+\.[0-9]+$'
PREVIEW_PATTERN='^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+$'

IS_STABLE=false
IS_PREVIEW=false

if [[ "$TAG" =~ $STABLE_PATTERN ]]; then
  IS_STABLE=true
  log_info "Release Channel: ${GREEN}${BOLD}Stable${RESET} (${TAG})"
elif [[ "$TAG" =~ $PREVIEW_PATTERN ]]; then
  IS_PREVIEW=true
  log_info "Release Channel: ${YELLOW}${BOLD}Preview${RESET} (${TAG})"
else
  log_error "Release tag '${TAG}' is invalid."
  echo "Tag must strictly follow SemVer:" >&2
  echo "  Stable:  v<MAJOR>.<MINOR>.<PATCH>       (e.g. v1.0.0)" >&2
  echo "  Preview: v<MAJOR>.<MINOR>.<PATCH>-rc.<N> (e.g. v1.0.0-rc.1)" >&2
  exit 1
fi

# 2. Assert Clean Working Tree
if [ "$ALLOW_DIRTY" = false ]; then
  log_info "Checking working tree cleanliness..."
  DIRTY_FILES="$(git status --porcelain)"
  if [ -n "$DIRTY_FILES" ]; then
    log_error "Working tree has uncommitted modifications or untracked files:"
    echo "$DIRTY_FILES" >&2
    echo "Commit, stash, or clean working tree before publishing a release." >&2
    exit 1
  fi
  log_ok "Working tree is clean."
fi

# 3. Check tag collision
if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  CURRENT_TAG_SHA="$(git rev-parse "refs/tags/${TAG}^{commit}")"
  HEAD_SHA="$(git rev-parse HEAD)"
  if [ "$CURRENT_TAG_SHA" != "$HEAD_SHA" ]; then
    log_error "Tag '${TAG}' already exists locally and points to different commit (${CURRENT_TAG_SHA:0:7} vs HEAD ${HEAD_SHA:0:7})."
    echo "Never move or delete published SemVer release tags." >&2
    exit 1
  fi
  log_warn "Tag '${TAG}' already exists on current HEAD."
fi

# 4. Run Verification Test Suites
if [ "$SKIP_TESTS" = false ]; then
  log_info "Running Spoke Tag Audit Suite..."
  if ! "$REPO_ROOT/scripts/audit-spoke-tags.sh"; then
    log_error "Spoke tag audit suite failed. Resolve image pinning violations before releasing."
    exit 1
  fi

  log_info "Running Go Daemon & Supervisor Test Suite..."
  if ! go test -count=1 ./...; then
    log_error "Go test suite failed."
    exit 1
  fi

  log_info "Running Web Presentation Client Test Suite..."
  if ! npm --prefix web test; then
    log_error "Web test suite failed."
    exit 1
  fi

  log_info "Running Shell & Installer Test Suites..."
  if ! bash "$REPO_ROOT/tests/installer_test.sh"; then
    log_error "Installer test suite failed."
    exit 1
  fi

  if ! bash "$REPO_ROOT/tests/verify_remotes_test.sh"; then
    log_error "Dual-remote verification test suite failed."
    exit 1
  fi
  log_ok "All test and audit verification suites passed successfully."
else
  log_warn "Test verification suites skipped (--skip-tests)."
fi

# 5. Handle Dry-Run Mode
if [ "$DRY_RUN" = true ]; then
  log_ok "Dry-run validation successful for release tag ${BOLD}${TAG}${RESET}."
  printf "\n${BOLD}Planned Actions (skipped due to --dry-run):${RESET}\n"
  printf "  1. git tag -a '%s' -m 'Release %s'\n" "$TAG" "$TAG"
  printf "  2. git push origin '%s'\n" "$TAG"
  printf "  3. git push github '%s'\n" "$TAG"
  printf "  4. %s/scripts/verify-remotes.sh -t '%s'\n" "$REPO_ROOT" "$TAG"
  printf "  5. GitHub Actions '.github/workflows/release.yml' automatically publishes GitHub Release with compiled binaries\n"
  exit 0
fi

# 6. Create Annotated Tag
log_info "Creating annotated git tag: ${BOLD}${TAG}${RESET}..."
if ! git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  git tag -a "$TAG" -m "Release $TAG"
  log_ok "Created git tag: $TAG"
fi

# 7. Push to Dual Remotes
log_info "Pushing tag '${TAG}' to internal remote 'origin'..."
if ! git push origin "$TAG"; then
  log_error "Failed to push tag '${TAG}' to 'origin'."
  exit 1
fi

log_info "Pushing tag '${TAG}' to public remote 'github'..."
if ! git push github "$TAG"; then
  log_error "Failed to push tag '${TAG}' to 'github'."
  exit 1
fi
log_ok "Pushed tag '${TAG}' to both 'origin' and 'github'."

# 8. Verify Dual-Remote Tag Parity
log_info "Verifying remote tag parity via verify-remotes.sh..."
if ! "$REPO_ROOT/scripts/verify-remotes.sh" -t "$TAG"; then
  log_error "Dual-remote tag parity verification failed."
  exit 1
fi

log_ok "Release ${BOLD}${TAG}${RESET} successfully published and verified across both remotes!"

printf "\n${BOLD}${GREEN}==============================================================================${RESET}\n"
printf "${BOLD}${GREEN}  Release %s Successfully Published & Verified!${RESET}\n" "$TAG"
printf "${BOLD}${GREEN}==============================================================================${RESET}\n"
printf "  • Dual-Remote Tag Parity: ${GREEN}VERIFIED${RESET} (origin & github)\n"
printf "  • GitHub Actions Pipeline: ${BLUE}TRIGGERED${RESET}\n"
printf "    ↳ Workflow: https://github.com/j333c/sovereign/actions?query=workflow%%3A%%22Release+Pipeline%%22\n"
printf "    ↳ Automated on GitHub:\n"
printf "      - Full verification test suite execution\n"
printf "      - Multi-architecture daemon cross-compilation (linux/amd64, linux/arm64)\n"
printf "      - SHA256SUMS generation\n"
printf "      - Official GitHub Release creation with release notes & assets\n"
printf "  • Appliance Discovery: Active appliances will discover %s over Smart HTTP\n" "$TAG"
printf "${BOLD}${GREEN}==============================================================================${RESET}\n\n"

