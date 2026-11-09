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
