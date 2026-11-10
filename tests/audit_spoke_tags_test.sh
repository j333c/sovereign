#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Spoke Dependency Governance Test Suite
# Tests scripts/audit-spoke-tags.sh under controlled mock environments.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AUDIT_SCRIPT="$REPO_ROOT/scripts/audit-spoke-tags.sh"

PASSED=0
FAILED=0

run_test() {
  local name="$1"
  shift
  echo -n "Running test: $name ... "
  if ( "$@" ); then
    echo "PASS"
    PASSED=$((PASSED + 1))
  else
    echo "FAIL"
    FAILED=$((FAILED + 1))
  fi
}

TEST_TMP="$(mktemp -d /tmp/sovereign-audit-test.XXXXXX)"
trap 'rm -rf "$TEST_TMP"' EXIT

test_current_repo_passes() {
  local output
  output="$("$AUDIT_SCRIPT" 2>&1)"
  echo "$output" | grep -q "Tag audit passed: all"
}

test_help_flag() {
  local output
  output="$("$AUDIT_SCRIPT" --help 2>&1)"
  echo "$output" | grep -q "Sovereign Spoke Dependency Governance & Tag Audit Suite"
}

test_floating_latest_rejected() {
  local dir="$TEST_TMP/floating_latest"
  mkdir -p "$dir/internal/supervisor"
  cat << 'EOF' > "$dir/docker-compose.yml"
services:
  caddy:
    image: caddy:latest
EOF
  cat << 'EOF' > "$dir/internal/supervisor/supervisor.go"
const (
  ImageJellyfin = "jellyfin/jellyfin:12.2.0"
)
EOF

  set +e
  local output
  output="$("$AUDIT_SCRIPT" -c "$dir/docker-compose.yml" -s "$dir/internal/supervisor" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "prohibited floating tag ':latest'"
}

test_floating_alpine_rejected() {
  local dir="$TEST_TMP/floating_alpine"
  mkdir -p "$dir/internal/supervisor"
  cat << 'EOF' > "$dir/docker-compose.yml"
services:
  caddy:
    image: caddy:2-alpine
EOF

  set +e
  local output
  output="$("$AUDIT_SCRIPT" -c "$dir/docker-compose.yml" -s "$dir/internal/supervisor" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "prohibited floating tag ':2-alpine'"
}

test_invalid_semver_spoke_rejected() {
  local dir="$TEST_TMP/invalid_semver"
  mkdir -p "$dir/internal/supervisor"
  cat << 'EOF' > "$dir/docker-compose.yml"
services:
  caddy:
    image: caddy:2.8.4-alpine
EOF
  cat << 'EOF' > "$dir/internal/supervisor/supervisor.go"
const (
  ImageJellyfin       = "jellyfin/jellyfin:12.2"
  ImageNavidrome      = "deluan/navidrome:0.54.5"
  ImageAudiobookshelf = "advplyr/audiobookshelf:2.19.5"
  ImagePaperless      = "paperlessngx/paperless-ngx:3.3.1"
)
EOF

  set +e
  local output
  output="$("$AUDIT_SCRIPT" -c "$dir/docker-compose.yml" -s "$dir/internal/supervisor" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "does not meet explicit MAJOR.MINOR.PATCH SemVer requirement"
}

test_invalid_llama_build_tag_rejected() {
  local dir="$TEST_TMP/invalid_llama"
  mkdir -p "$dir/internal/supervisor"
  cat << 'EOF' > "$dir/docker-compose.yml"
services:
  caddy:
    image: caddy:2.8.4-alpine
EOF
  cat << 'EOF' > "$dir/internal/supervisor/llama.go"
const (
  ImageLlama = "ghcr.io/ggml-org/llama.cpp:b12"
)
EOF

  set +e
  local output
  output="$("$AUDIT_SCRIPT" -c "$dir/docker-compose.yml" -s "$dir/internal/supervisor" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "does not meet llama.cpp build tag requirement"
}

test_valid_custom_fixture_passes() {
  local dir="$TEST_TMP/valid_fixture"
  mkdir -p "$dir/internal/supervisor"
  cat << 'EOF' > "$dir/docker-compose.yml"
services:
  caddy:
    image: caddy:2.8.4-alpine
EOF
  cat << 'EOF' > "$dir/internal/supervisor/supervisor.go"
const (
  ImageJellyfin       = "jellyfin/jellyfin:10.9.11"
  ImageNavidrome      = "deluan/navidrome:0.54.5"
  ImageAudiobookshelf = "advplyr/audiobookshelf:2.19.5"
  ImagePaperless      = "paperlessngx/paperless-ngx:2.14.7"
)
EOF
  cat << 'EOF' > "$dir/internal/supervisor/openwebui.go"
const (
  ImageOpenWebUI = "ghcr.io/open-webui/open-webui:0.5.20"
)
EOF
  cat << 'EOF' > "$dir/internal/supervisor/llama.go"
const (
  ImageLlama = "ghcr.io/ggml-org/llama.cpp:b4887"
)
EOF

  local output
  output="$("$AUDIT_SCRIPT" -c "$dir/docker-compose.yml" -s "$dir/internal/supervisor" 2>&1)"
  echo "$output" | grep -q "Tag audit passed: all 7 spoke and gateway image tags satisfy strict version pinning."
}

# Run tests
run_test "Current repo tags pass audit" test_current_repo_passes
run_test "Help output flag" test_help_flag
run_test "Floating :latest tag rejected" test_floating_latest_rejected
run_test "Floating :2-alpine tag rejected" test_floating_alpine_rejected
run_test "Invalid SemVer spoke tag rejected" test_invalid_semver_spoke_rejected
run_test "Invalid llama.cpp build tag rejected" test_invalid_llama_build_tag_rejected
run_test "Valid custom fixture passes" test_valid_custom_fixture_passes

echo "---"
echo "Tests passed: $PASSED, failed: $FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
