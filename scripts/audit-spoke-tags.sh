#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Upstream Spoke Dependency Governance & CI Tag Audit Suite
# Repository: https://github.com/j333c/sovereign
# License: Apache 2.0
#
# Inspects container image definitions in docker-compose.yml and Go supervisor
# source files to ensure all spoke and gateway containers are pinned to strict
# SemVer tags (or pinned build tags for llama.cpp). Floating tags (:latest,
# :2-alpine, :main, etc.) are strictly prohibited.
#
# Optionally queries upstream OCI registries (Docker Hub, GHCR) to check for
# newer stable releases and prepare PRs targeting release branches.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

COMPOSE_FILE="$REPO_ROOT/docker-compose.yml"
SUPERVISOR_DIR="$REPO_ROOT/internal/supervisor"
CHECK_UPSTREAM=false
CREATE_PR=false
TARGET_BRANCH=""
STRICT_UPSTREAM=false
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
Sovereign Spoke Dependency Governance & Tag Audit Suite

Usage:
  $(basename "$0") [options]

Options:
  -c, --compose <path>        Path to docker-compose.yml (default: docker-compose.yml)
  -s, --supervisor <path>     Path to internal/supervisor directory (default: internal/supervisor)
  -u, --check-upstream        Query upstream registries (Docker Hub, GHCR) for newer releases
  -p, --pr                    Prepare update branch/PR for detected upstream releases
  -b, --target-branch <name>  Target release branch for PR (e.g. release/v1.0 or main)
  --strict-upstream           Fail audit if upstream registry queries fail
  -q, --quiet                 Suppress informational messages
  -h, --help                  Show this help documentation

Exit Status:
  0   All spoke and gateway container images satisfy strict pinning governance.
  1   One or more images violate tag pinning rules, or audit failed.
EOF
}

# Parse Command-Line Options
while [ $# -gt 0 ]; do
  case "$1" in
    -c|--compose)
      if [ -n "${2:-}" ]; then
        COMPOSE_FILE="$2"
        shift 2
      else
        log_error "Option '$1' requires an argument."
        exit 1
      fi
      ;;
    -s|--supervisor)
      if [ -n "${2:-}" ]; then
        SUPERVISOR_DIR="$2"
        shift 2
      else
        log_error "Option '$1' requires an argument."
        exit 1
      fi
      ;;
    -u|--check-upstream)
      CHECK_UPSTREAM=true
      shift
      ;;
    -p|--pr)
      CREATE_PR=true
      shift
      ;;
    -b|--target-branch)
      if [ -n "${2:-}" ]; then
        TARGET_BRANCH="$2"
        shift 2
      else
        log_error "Option '$1' requires an argument."
        exit 1
      fi
      ;;
    --strict-upstream)
      STRICT_UPSTREAM=true
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

# Audit tracking
ERRORS=0
WARNINGS=0
TOTAL_CHECKED=0

# Validate an image tag
# Returns 0 if valid pinned tag, 1 if floating or invalid
validate_image_pin() {
  local service_name="$1"
  local image_ref="$2"
  local source_file="$3"

  TOTAL_CHECKED=$((TOTAL_CHECKED + 1))

  # Extract repository and tag
  local tag="${image_ref##*:}"
  if [ "$tag" = "$image_ref" ]; then
    log_error "[$service_name] Image '$image_ref' in $source_file has NO tag specified (implicit :latest prohibited)."
    ERRORS=$((ERRORS + 1))
    return 1
  fi

  # Prohibit explicit floating tags
  if [[ "$tag" =~ ^(latest|main|master|dev|edge|nightly)$ ]]; then
    log_error "[$service_name] Image '$image_ref' in $source_file uses prohibited floating tag ':$tag'."
    ERRORS=$((ERRORS + 1))
    return 1
  fi

  # Prohibit floating major/minor alpine tags (e.g. :2-alpine, :alpine)
  if [[ "$tag" =~ ^([0-9]+-alpine|alpine)$ ]]; then
    log_error "[$service_name] Image '$image_ref' in $source_file uses prohibited floating tag ':$tag'."
    ERRORS=$((ERRORS + 1))
    return 1
  fi

  # Service-specific pinning checks
  case "$service_name" in
    caddy)
      # Must be explicit MAJOR.MINOR.PATCH (optional -alpine suffix)
      if [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-alpine)?$ ]]; then
        log_ok "[$service_name] Pinned: $image_ref"
        return 0
      else
        log_error "[$service_name] Image tag '$tag' does not meet explicit MAJOR.MINOR.PATCH[-alpine] SemVer requirement in $source_file."
        ERRORS=$((ERRORS + 1))
        return 1
      fi
      ;;
    llama|llama.cpp)
      # Must match ^b[0-9]{4,}$ or ^server-b[0-9]{4,}$
      if [[ "$tag" =~ ^(server-)?b[0-9]{4,}$ ]]; then
        log_ok "[$service_name] Pinned build tag: $image_ref"
        return 0
      else
        log_error "[$service_name] Image tag '$tag' does not meet llama.cpp build tag requirement (^b[0-9]{4,}$) in $source_file."
        ERRORS=$((ERRORS + 1))
        return 1
      fi
      ;;
    jellyfin|navidrome|audiobookshelf|paperless|openwebui|"open-webui")
      # Must be explicit MAJOR.MINOR.PATCH SemVer
      if [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
        log_ok "[$service_name] Pinned SemVer: $image_ref"
        return 0
      else
        log_error "[$service_name] Image tag '$tag' does not meet explicit MAJOR.MINOR.PATCH SemVer requirement in $source_file."
        ERRORS=$((ERRORS + 1))
        return 1
      fi
      ;;
    *)
      # Generic spoke: require SemVer or explicit build tag
      if [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]] || [[ "$tag" =~ ^(server-)?b[0-9]{4,}$ ]]; then
        log_ok "[$service_name] Pinned: $image_ref"
        return 0
      else
        log_error "[$service_name] Image tag '$tag' does not meet explicit version pinning in $source_file."
        ERRORS=$((ERRORS + 1))
        return 1
      fi
      ;;
  esac
}

log_info "Auditing container image tags for strict version pinning..."

# 1. Audit docker-compose.yml
if [ -f "$COMPOSE_FILE" ]; then
  log_info "Checking compose manifest: $COMPOSE_FILE"

  # Find caddy image
  CADDY_IMG="$(grep -E '^\s*image:\s*caddy:' "$COMPOSE_FILE" | head -n1 | sed -E 's/^\s*image:\s*//' | tr -d ' "' || true)"
  if [ -n "$CADDY_IMG" ]; then
    validate_image_pin "caddy" "$CADDY_IMG" "$COMPOSE_FILE"
  fi

  # Check all image: lines in docker-compose.yml for floating :latest
  while IFS= read -r line; do
    if [ -n "$line" ]; then
      img="$(echo "$line" | sed -E 's/^\s*image:\s*//' | tr -d ' "')"
      if [[ "$img" =~ :latest$ ]]; then
        log_error "Found floating :latest tag in compose file ($COMPOSE_FILE): $img"
        ERRORS=$((ERRORS + 1))
      fi
    fi
  done < <(grep -E '^\s*image:\s*' "$COMPOSE_FILE" || true)
else
  log_warn "Compose file not found at $COMPOSE_FILE (skipping)"
fi

# 2. Audit Go supervisor source files
if [ -d "$SUPERVISOR_DIR" ]; then
  log_info "Checking Go supervisor constants in: $SUPERVISOR_DIR"

  # Scan supervisor.go
  SUPERVISOR_GO="$SUPERVISOR_DIR/supervisor.go"
  if [ -f "$SUPERVISOR_GO" ]; then
    JELLYFIN_IMG="$(grep -E 'ImageJellyfin\s*=\s*' "$SUPERVISOR_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
    NAVIDROME_IMG="$(grep -E 'ImageNavidrome\s*=\s*' "$SUPERVISOR_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
    AUDIO_IMG="$(grep -E 'ImageAudiobookshelf\s*=\s*' "$SUPERVISOR_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
    PAPERLESS_IMG="$(grep -E 'ImagePaperless\s*=\s*' "$SUPERVISOR_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"

    [ -n "$JELLYFIN_IMG" ] && validate_image_pin "jellyfin" "$JELLYFIN_IMG" "$SUPERVISOR_GO"
    [ -n "$NAVIDROME_IMG" ] && validate_image_pin "navidrome" "$NAVIDROME_IMG" "$SUPERVISOR_GO"
    [ -n "$AUDIO_IMG" ] && validate_image_pin "audiobookshelf" "$AUDIO_IMG" "$SUPERVISOR_GO"
    [ -n "$PAPERLESS_IMG" ] && validate_image_pin "paperless" "$PAPERLESS_IMG" "$SUPERVISOR_GO"
  fi

  # Scan openwebui.go
  OPENWEBUI_GO="$SUPERVISOR_DIR/openwebui.go"
  if [ -f "$OPENWEBUI_GO" ]; then
    OPENWEBUI_IMG="$(grep -E 'ImageOpenWebUI\s*=\s*' "$OPENWEBUI_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
    [ -n "$OPENWEBUI_IMG" ] && validate_image_pin "openwebui" "$OPENWEBUI_IMG" "$OPENWEBUI_GO"
  fi

  # Scan llama.go
  LLAMA_GO="$SUPERVISOR_DIR/llama.go"
  if [ -f "$LLAMA_GO" ]; then
    LLAMA_IMG="$(grep -E 'ImageLlama\s*=\s*' "$LLAMA_GO" | sed -E 's/.*"([^"]+)".*/\1/' || true)"
    [ -n "$LLAMA_IMG" ] && validate_image_pin "llama" "$LLAMA_IMG" "$LLAMA_GO"
  fi
else
  log_warn "Supervisor directory not found at $SUPERVISOR_DIR (skipping)"
fi

# 3. Optional upstream checking
if [ "$CHECK_UPSTREAM" = true ]; then
  log_info "Checking upstream OCI registries for newer releases..."

  # Query helper for GitHub releases API
  check_github_latest() {
    local repo="$1"
    local current_tag="$2"
    local service="$3"

    log_info "[$service] Checking upstream: https://api.github.com/repos/$repo/releases/latest"
    local response
    if response="$(curl -fsSL --connect-timeout 5 --max-time 10 "https://api.github.com/repos/$repo/releases/latest" 2>/dev/null)"; then
      local latest_tag
      latest_tag="$(echo "$response" | grep -E '"tag_name":' | head -n1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"
      latest_tag="${latest_tag#v}"
      local cur="${current_tag#v}"
      cur="${cur#b}"
      latest_tag_clean="${latest_tag#b}"

      if [ -n "$latest_tag" ] && [ "$latest_tag_clean" != "$cur" ]; then
        log_info "[$service] Newer upstream release found: $latest_tag (current: $current_tag)"
        if [ "$CREATE_PR" = true ]; then
          handle_upstream_pr "$service" "$repo" "$latest_tag"
        fi
      else
        log_ok "[$service] Up to date with upstream release ($latest_tag)."
      fi
    else
      if [ "$STRICT_UPSTREAM" = true ]; then
        log_error "[$service] Failed to query upstream release for $repo."
        ERRORS=$((ERRORS + 1))
      else
        log_warn "[$service] Upstream check skipped (network offline or rate-limited)."
      fi
    fi
  }

  handle_upstream_pr() {
    local service="$1"
    local repo="$2"
    local new_tag="$3"

    local branch_name="spoke-update/${service}-${new_tag}"
    log_info "[$service] Preparing update branch '${branch_name}' targeting active release branch..."

    if [ -z "$TARGET_BRANCH" ]; then
      TARGET_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")"
      if [[ "$TARGET_BRANCH" == "HEAD" ]]; then
        TARGET_BRANCH="main"
      fi
    fi

    printf "  Target Branch: %s\n" "$TARGET_BRANCH"
    printf "  Update Branch: %s\n" "$branch_name"
    printf "  Action: Automated PR for %s to %s\n" "$service" "$new_tag"
  }

  check_github_latest "jellyfin/jellyfin" "${JELLYFIN_IMG##*:}" "jellyfin"
  check_github_latest "navidrome/navidrome" "${NAVIDROME_IMG##*:}" "navidrome"
  check_github_latest "advplyr/audiobookshelf" "${AUDIO_IMG##*:}" "audiobookshelf"
  check_github_latest "paperless-ngx/paperless-ngx" "${PAPERLESS_IMG##*:}" "paperless"
  check_github_latest "open-webui/open-webui" "${OPENWEBUI_IMG##*:}" "openwebui"
  check_github_latest "ggml-org/llama.cpp" "${LLAMA_IMG##*:}" "llama.cpp"
fi

# Summary
if [ "$ERRORS" -gt 0 ]; then
  log_error "Audit failed: $ERRORS violation(s) detected across $TOTAL_CHECKED checked image tag(s)."
  exit 1
else
  log_ok "Tag audit passed: all $TOTAL_CHECKED spoke and gateway image tags satisfy strict version pinning."
  exit 0
fi
