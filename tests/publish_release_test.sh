#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Release Publishing Pipeline Test Suite
# Tests scripts/publish-release.sh under controlled mock environments.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PUBLISH_SCRIPT="$REPO_ROOT/scripts/publish-release.sh"

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

TEST_TMP="$(mktemp -d /tmp/sovereign-publish-test.XXXXXX)"
trap 'rm -rf "$TEST_TMP"' EXIT

test_help_flag() {
  local output
  output="$("$PUBLISH_SCRIPT" --help 2>&1)"
  echo "$output" | grep -q "Sovereign Release Publishing Pipeline"
}

test_missing_tag_argument() {
  set +e
  local output
  output="$("$PUBLISH_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Missing required release tag argument"
}

test_valid_stable_tag_dry_run() {
  local output
  output="$("$PUBLISH_SCRIPT" --dry-run --skip-tests --allow-dirty v1.0.0 2>&1)"
  echo "$output" | grep -q "Release Channel:.*Stable" && echo "$output" | grep -q "Dry-run validation successful"
}

test_valid_preview_tag_dry_run() {
  local output
  output="$("$PUBLISH_SCRIPT" --dry-run --skip-tests --allow-dirty v1.2.3-rc.1 2>&1)"
  echo "$output" | grep -q "Release Channel:.*Preview" && echo "$output" | grep -q "Dry-run validation successful"
}

test_invalid_tag_formats_rejected() {
  local invalid_tags=("1.0.0" "v1.0" "v1.0.0-beta" "latest" "v1.0.0.0" "v1.0.0-preview" "v1.0.0-rc")
  for tag in "${invalid_tags[@]}"; do
    set +e
    local output
    output="$("$PUBLISH_SCRIPT" --dry-run --skip-tests --allow-dirty "$tag" 2>&1)"
    local status=$?
    set -e

    if [ "$status" -ne 1 ] || ! echo "$output" | grep -q "is invalid"; then
      echo "Failed to reject invalid tag: $tag"
      return 1
    fi
  done
  return 0
}

test_dirty_working_tree_rejected() {
  local repo="$TEST_TMP/dirty_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"

  # Dirty working tree
  echo "dirty modification" >> file.txt

  set +e
  local output
  output="$("$PUBLISH_SCRIPT" --dry-run --skip-tests v1.0.0 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Working tree has uncommitted modifications"
}

test_github_actions_planned_in_dry_run() {
  local output
  output="$("$PUBLISH_SCRIPT" --dry-run --skip-tests --allow-dirty v1.0.0 2>&1)"
  echo "$output" | grep -q "GitHub Actions" && echo "$output" | grep -q "release.yml"
}

# Run tests
run_test "Help output flag" test_help_flag
run_test "Missing tag argument rejection" test_missing_tag_argument
run_test "Valid stable tag accepted (dry-run)" test_valid_stable_tag_dry_run
run_test "Valid preview tag accepted (dry-run)" test_valid_preview_tag_dry_run
run_test "Invalid tag formats rejected" test_invalid_tag_formats_rejected
run_test "Dirty working tree rejected" test_dirty_working_tree_rejected
run_test "GitHub Actions workflow planned in dry-run" test_github_actions_planned_in_dry_run

echo "---"
echo "Tests passed: $PASSED, failed: $FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi

