#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Dual-Remote Git Sync Sanity Check
# Repository: https://github.com/j333c/sovereign
# License: Apache 2.0
#
# Verifies commit parity and synchronization between the authoritative internal
# repository ('origin') and the public distribution repository ('github').
# ==============================================================================

set -euo pipefail

# Configuration Defaults
DEFAULT_BRANCH="main"
BRANCH="$DEFAULT_BRANCH"
DO_FETCH=false
QUIET=false
CHECK_TAGS=false
TAG_NAME=""

# Styling & Colors (disabled if not terminal or NO_COLOR set)
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
Sovereign Dual-Remote Git Synchronization Verifier

Usage:
  $(basename "$0") [options]

Options:
  -b, --branch <name>   Branch to verify (default: ${DEFAULT_BRANCH})
  -t, --tag <name>      Release tag to verify across remotes
  --tags                Verify all release tag refs across remotes
  -f, --fetch           Fetch latest references from remotes before verifying
  -q, --quiet           Suppress informational messages; only print results/errors
  -h, --help            Show this help documentation

Exit Status:
  0   Remotes are configured and in exact commit parity.
  1   Remotes are diverged, missing, or refs could not be verified.

Examples:
  $(basename "$0")                 # Check local tracking refs for 'main'
  $(basename "$0") --fetch         # Fetch latest remote refs before checking
  $(basename "$0") -b release/1.0  # Check a specific release branch
  $(basename "$0") -t v1.0.0       # Check specific release tag parity
  $(basename "$0") --tags          # Check all tag refs parity
EOF
}

# Parse Command-Line Options
while [ $# -gt 0 ]; do
  case "$1" in
    -b|--branch)
      if [ -n "${2:-}" ]; then
        BRANCH="$2"
        shift 2
      else
        log_error "Option '$1' requires an argument."
        exit 1
      fi
      ;;
    -t|--tag)
      if [ -n "${2:-}" ]; then
        TAG_NAME="$2"
        shift 2
      else
        log_error "Option '$1' requires an argument."
        exit 1
      fi
      ;;
    --tags)
      CHECK_TAGS=true
      shift
      ;;
    -f|--fetch)
      DO_FETCH=true
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
    *)
      log_error "Unknown option: $1"
      echo "Run '$(basename "$0") --help' for usage." >&2
      exit 1
      ;;
  esac
done

# Pre-flight: Check git environment
if ! command -v git >/dev/null 2>&1; then
  log_error "git is not installed or not in PATH."
  exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  log_error "Current working directory is not inside a git repository."
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

log_info "Verifying dual-remote configuration for branch: ${BOLD}${BRANCH}${RESET}..."

# Verify 'origin' remote exists
if ! git remote get-url origin >/dev/null 2>&1; then
  log_error "Remote 'origin' is not configured."
  echo "Expected internal development remote (e.g. http://git.crins:3000/jens/sovereign)." >&2
  exit 1
fi
ORIGIN_URL="$(git remote get-url origin)"

# Verify 'github' remote exists
if ! git remote get-url github >/dev/null 2>&1; then
  log_error "Remote 'github' is not configured."
  echo "Expected public distribution remote (e.g. https://github.com/j333c/sovereign.git)." >&2
  exit 1
fi
GITHUB_URL="$(git remote get-url github)"

# Verify dual distinct remotes
if [ "$ORIGIN_URL" = "$GITHUB_URL" ]; then
  log_error "Remotes 'origin' and 'github' point to the identical URL: $ORIGIN_URL"
  echo "Sovereign requires dual distinct remotes for internal dev and public distribution." >&2
  exit 1
fi

log_info "Origin remote: ${CYAN}${ORIGIN_URL}${RESET}"
log_info "GitHub remote: ${CYAN}${GITHUB_URL}${RESET}"

# Tag parity verification mode
if [ -n "$TAG_NAME" ]; then
  log_info "Verifying tag '${TAG_NAME}' parity across dual remotes..."
  if [ "$DO_FETCH" = true ]; then
    git fetch origin tag "$TAG_NAME" --quiet 2>/dev/null || true
    git fetch github tag "$TAG_NAME" --quiet 2>/dev/null || true
  fi

  ORIGIN_TAG_SHA="$(git ls-remote --tags --refs origin "refs/tags/${TAG_NAME}" 2>/dev/null | awk '{print $1}')"
  GITHUB_TAG_SHA="$(git ls-remote --tags --refs github "refs/tags/${TAG_NAME}" 2>/dev/null | awk '{print $1}')"

  if [ -z "$ORIGIN_TAG_SHA" ]; then
    ORIGIN_TAG_SHA="$(git rev-parse --verify "refs/remotes/origin/tags/${TAG_NAME}" 2>/dev/null || git rev-parse --verify "refs/tags/${TAG_NAME}^{commit}" 2>/dev/null || git rev-parse --verify "refs/tags/${TAG_NAME}" 2>/dev/null || true)"
  fi
  if [ -z "$GITHUB_TAG_SHA" ]; then
    GITHUB_TAG_SHA="$(git rev-parse --verify "refs/remotes/github/tags/${TAG_NAME}" 2>/dev/null || git rev-parse --verify "refs/tags/${TAG_NAME}^{commit}" 2>/dev/null || git rev-parse --verify "refs/tags/${TAG_NAME}" 2>/dev/null || true)"
  fi

  if [ -z "$ORIGIN_TAG_SHA" ]; then
    log_error "Tag '${TAG_NAME}' could not be resolved on 'origin'."
    exit 1
  fi
  if [ -z "$GITHUB_TAG_SHA" ]; then
    log_error "Tag '${TAG_NAME}' could not be resolved on 'github'."
    exit 1
  fi

  if [ "$ORIGIN_TAG_SHA" = "$GITHUB_TAG_SHA" ]; then
    log_ok "Dual-remote tag parity verified: '${TAG_NAME}' points to ${BOLD}${ORIGIN_TAG_SHA:0:7}${RESET} on both remotes."
    exit 0
  else
    log_error "Dual-remote tag disparity detected for tag '${TAG_NAME}'."
    printf "  Origin: %s\n  GitHub: %s\n" "$ORIGIN_TAG_SHA" "$GITHUB_TAG_SHA" >&2
    exit 1
  fi
fi

if [ "$CHECK_TAGS" = true ]; then
  log_info "Verifying all release tag refs across dual remotes..."
  if [ "$DO_FETCH" = true ]; then
    git fetch origin --tags --quiet 2>/dev/null || true
    git fetch github --tags --quiet 2>/dev/null || true
  fi

  ORIGIN_TAGS="$(git ls-remote --tags --refs origin 2>/dev/null | awk '{print $1, $2}' | sort)"
  GITHUB_TAGS="$(git ls-remote --tags --refs github 2>/dev/null | awk '{print $1, $2}' | sort)"

  if [ -z "$ORIGIN_TAGS" ] && [ -z "$GITHUB_TAGS" ]; then
    ORIGIN_TAGS="$(git for-each-ref --format='%(objectname) %(refname)' refs/tags 2>/dev/null | sort)"
    GITHUB_TAGS="$(git for-each-ref --format='%(objectname) %(refname)' refs/tags 2>/dev/null | sort)"
  fi

  if [ "$ORIGIN_TAGS" = "$GITHUB_TAGS" ]; then
    TAG_COUNT="$(echo "$ORIGIN_TAGS" | grep -c "refs/tags" || echo "0")"
    log_ok "Dual-remote tag parity verified: all ${TAG_COUNT} tag(s) identical across remotes."
    exit 0
  else
    log_error "Dual-remote tag disparity detected between origin and github."
    diff -u <(echo "$ORIGIN_TAGS") <(echo "$GITHUB_TAGS") >&2 || true
    exit 1
  fi
fi

# Optional fetch from both remotes
if [ "$DO_FETCH" = true ]; then
  log_info "Fetching ref '${BRANCH}' from 'origin'..."
  if ! git fetch origin "$BRANCH" --quiet 2>/dev/null; then
    log_error "Failed to fetch '${BRANCH}' from origin (network unreachable or ref absent)."
    exit 1
  fi

  log_info "Fetching ref '${BRANCH}' from 'github'..."
  if ! git fetch github "$BRANCH" --quiet 2>/dev/null; then
    log_error "Failed to fetch '${BRANCH}' from github (network unreachable or ref absent)."
    exit 1
  fi
fi

# Resolve commit SHAs using fully qualified remote tracking refs
ORIGIN_REF="refs/remotes/origin/${BRANCH}"
GITHUB_REF="refs/remotes/github/${BRANCH}"

ORIGIN_COMMIT="$(git rev-parse --verify "$ORIGIN_REF" 2>/dev/null || true)"
GITHUB_COMMIT="$(git rev-parse --verify "$GITHUB_REF" 2>/dev/null || true)"

if [ -z "$ORIGIN_COMMIT" ]; then
  log_error "Ref '${ORIGIN_REF}' could not be resolved. Have you fetched from 'origin'?"
  exit 1
fi

if [ -z "$GITHUB_COMMIT" ]; then
  log_error "Ref '${GITHUB_REF}' could not be resolved. Have you fetched from 'github'?"
  exit 1
fi

# Compare commit hashes
if [ "$ORIGIN_COMMIT" = "$GITHUB_COMMIT" ]; then
  COMMIT_SHORT="${ORIGIN_COMMIT:0:7}"
  COMMIT_SUBJECT="$(git log -1 --format="%s" "$ORIGIN_COMMIT" 2>/dev/null || echo "Unknown commit")"

  log_ok "Dual-remote synchronization verified: ${BOLD}in parity${RESET}."
  printf "  Branch:       ${CYAN}%s${RESET}\n" "$BRANCH"
  printf "  Commit:       ${BOLD}%s${RESET} (%s)\n" "$COMMIT_SHORT" "$ORIGIN_COMMIT"
  printf "  Subject:      %s\n" "$COMMIT_SUBJECT"
  printf "  Origin URL:   %s\n" "$ORIGIN_URL"
  printf "  GitHub URL:   %s\n" "$GITHUB_URL"

  # Check local HEAD or local branch status
  CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  LOCAL_COMMIT="$(git rev-parse --verify HEAD 2>/dev/null || true)"

  if [ "$CURRENT_BRANCH" = "$BRANCH" ] || [ "$CURRENT_BRANCH" = "HEAD" ]; then
    if [ "$LOCAL_COMMIT" = "$ORIGIN_COMMIT" ]; then
      log_ok "Local HEAD commit matches both remotes."
    else
      LOCAL_SHORT="${LOCAL_COMMIT:0:7}"
      AHEAD="$(git rev-list --count "${ORIGIN_COMMIT}..${LOCAL_COMMIT}" 2>/dev/null || echo "0")"
      BEHIND="$(git rev-list --count "${LOCAL_COMMIT}..${ORIGIN_COMMIT}" 2>/dev/null || echo "0")"
      log_warn "Local HEAD (${LOCAL_SHORT}) differs from remotes: ${AHEAD} commit(s) ahead, ${BEHIND} commit(s) behind."
    fi
  fi

  exit 0
else
  # Remotes are out of sync
  ORIGIN_SHORT="${ORIGIN_COMMIT:0:7}"
  GITHUB_SHORT="${GITHUB_COMMIT:0:7}"

  ORIGIN_AHEAD="$(git rev-list --count "${GITHUB_COMMIT}..${ORIGIN_COMMIT}" 2>/dev/null || echo "unknown")"
  GITHUB_AHEAD="$(git rev-list --count "${ORIGIN_COMMIT}..${GITHUB_COMMIT}" 2>/dev/null || echo "unknown")"

  log_error "Dual-remote divergence detected for branch '${BRANCH}'."
  {
    printf "  Origin:       ${BOLD}%s${RESET} (%s)\n" "$ORIGIN_SHORT" "$ORIGIN_COMMIT"
    printf "  GitHub:       ${BOLD}%s${RESET} (%s)\n" "$GITHUB_SHORT" "$GITHUB_COMMIT"
    printf "  Origin ahead: ${YELLOW}%s${RESET} commit(s)\n" "$ORIGIN_AHEAD"
    printf "  GitHub ahead: ${YELLOW}%s${RESET} commit(s)\n" "$GITHUB_AHEAD"

    if [ "$ORIGIN_AHEAD" != "0" ] && [ "$GITHUB_AHEAD" = "0" ]; then
      printf "\n${BOLD}Remedy:${RESET} Push pending commits to the public distribution remote:\n"
      printf "  git push github %s\n" "$BRANCH"
    elif [ "$GITHUB_AHEAD" != "0" ] && [ "$ORIGIN_AHEAD" = "0" ]; then
      printf "\n${BOLD}Remedy:${RESET} Reconcile public distribution commits with internal origin:\n"
      printf "  git push origin refs/remotes/github/%s:%s\n" "$BRANCH" "$BRANCH"
    else
      printf "\n${BOLD}Remedy:${RESET} Remotes have diverged. Inspect git log between 'refs/remotes/origin/%s' and 'refs/remotes/github/%s'.\n" "$BRANCH" "$BRANCH"
    fi
  } >&2

  exit 1
fi
