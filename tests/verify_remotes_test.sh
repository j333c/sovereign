#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Dual-Remote Verification Test Suite
# Tests scripts/verify-remotes.sh under controlled mock git environments.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERIFY_SCRIPT="$REPO_ROOT/scripts/verify-remotes.sh"

PASSED=0
FAILED=0

run_test() {
  local name="$1"
  shift
  echo -n "Running test: $name ... "
  # Run in subshell so working directory mutations and errors are isolated
  if ( "$@" ); then
    echo "PASS"
    PASSED=$((PASSED + 1))
  else
    echo "FAIL"
    FAILED=$((FAILED + 1))
  fi
}

# Setup temporary sandbox directory
TEST_TMP="$(mktemp -d /tmp/sovereign-remotes-test.XXXXXX)"
trap 'rm -rf "$TEST_TMP"' EXIT

test_in_parity() {
  local repo="$TEST_TMP/parity_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "content" > file.txt
  git add file.txt
  git commit -q -m "initial commit"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"

  git update-ref refs/remotes/origin/main "$sha"
  git update-ref refs/remotes/github/main "$sha"

  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  echo "$output" | grep -q "Dual-remote synchronization verified: in parity."
}

test_detached_head_parity() {
  local repo="$TEST_TMP/detached_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "content" > file.txt
  git add file.txt
  git commit -q -m "initial commit"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"

  git update-ref refs/remotes/origin/main "$sha"
  git update-ref refs/remotes/github/main "$sha"

  git checkout -q "$sha" # enter detached HEAD

  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  echo "$output" | grep -q "Local HEAD commit matches both remotes."
}

test_diverged_origin_ahead() {
  local repo="$TEST_TMP/diverged_origin_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha1
  sha1="$(git rev-parse HEAD)"

  echo "c2" >> file.txt
  git add file.txt
  git commit -q -m "commit 2"
  local sha2
  sha2="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"

  git update-ref refs/remotes/origin/main "$sha2"
  git update-ref refs/remotes/github/main "$sha1"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Dual-remote divergence detected" && echo "$output" | grep -q "Push pending commits to the public distribution remote"
}

test_diverged_github_ahead() {
  local repo="$TEST_TMP/diverged_github_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha1
  sha1="$(git rev-parse HEAD)"

  echo "c2" >> file.txt
  git add file.txt
  git commit -q -m "commit 2"
  local sha2
  sha2="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"

  git update-ref refs/remotes/origin/main "$sha1"
  git update-ref refs/remotes/github/main "$sha2"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Dual-remote divergence detected" && echo "$output" | grep -q "Reconcile public distribution commits with internal origin"
}

test_missing_origin_remote() {
  local repo="$TEST_TMP/missing_origin_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add github "https://github.com/j333c/sovereign.git"
  git update-ref refs/remotes/github/main "$sha"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Remote 'origin' is not configured."
}

test_missing_github_remote() {
  local repo="$TEST_TMP/missing_github_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git update-ref refs/remotes/origin/main "$sha"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Remote 'github' is not configured."
}

test_identical_remotes_error() {
  local repo="$TEST_TMP/identical_remotes_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"

  git remote add origin "https://github.com/j333c/sovereign.git"
  git remote add github "https://github.com/j333c/sovereign.git"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "point to the identical URL"
}

test_not_a_git_repo() {
  local non_repo="$TEST_TMP/not_git_repo"
  mkdir -p "$non_repo"
  cd "$non_repo"

  set +e
  local output
  output="$("$VERIFY_SCRIPT" 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "not inside a git repository"
}

test_invalid_option() {
  set +e
  local output
  output="$("$VERIFY_SCRIPT" --invalid-arg 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Unknown option"
}

test_quiet_mode() {
  local repo="$TEST_TMP/quiet_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"
  git update-ref refs/remotes/origin/main "$sha"
  git update-ref refs/remotes/github/main "$sha"

  local output
  output="$("$VERIFY_SCRIPT" -q 2>&1)"
  ! echo "$output" | grep -q "\[INFO\]" && echo "$output" | grep -q "\[OK\]"
}

test_custom_branch() {
  local repo="$TEST_TMP/custom_branch_repo"
  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  git checkout -q -b release/1.0
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "http://git.crins:3000/jens/sovereign"
  git remote add github "https://github.com/j333c/sovereign.git"
  git update-ref refs/remotes/origin/release/1.0 "$sha"
  git update-ref refs/remotes/github/release/1.0 "$sha"

  local output
  output="$("$VERIFY_SCRIPT" -b release/1.0 2>&1)"
  echo "$output" | grep -q "Branch:       release/1.0"
}

test_help_flag() {
  local output
  output="$("$VERIFY_SCRIPT" --help 2>&1)"
  echo "$output" | grep -q "Sovereign Dual-Remote Git Synchronization Verifier"
}

test_tag_parity() {
  local repo="$TEST_TMP/tag_parity_repo"
  local mock_origin="$TEST_TMP/tag_parity_origin.git"
  local mock_github="$TEST_TMP/tag_parity_github.git"
  git init -q --bare "$mock_origin"
  git init -q --bare "$mock_github"

  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"
  local sha
  sha="$(git rev-parse HEAD)"

  git remote add origin "$mock_origin"
  git remote add github "$mock_github"
  git push -q origin main
  git push -q github main

  git tag -a "v1.0.0" -m "Release v1.0.0" "$sha"
  git push -q origin v1.0.0
  git push -q github v1.0.0

  local output
  output="$("$VERIFY_SCRIPT" -t v1.0.0 2>&1)"
  echo "$output" | grep -q "Dual-remote tag parity verified"
}

test_tag_divergence() {
  local repo="$TEST_TMP/tag_divergence_repo"
  local mock_origin="$TEST_TMP/tag_divergence_origin.git"
  local mock_github="$TEST_TMP/tag_divergence_github.git"
  git init -q --bare "$mock_origin"
  git init -q --bare "$mock_github"

  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"

  git remote add origin "$mock_origin"
  git remote add github "$mock_github"

  git tag -a "v1.0.0" -m "v1.0.0 commit 1"
  git push -q origin v1.0.0

  echo "c2" >> file.txt
  git add file.txt
  git commit -q -m "commit 2"
  git tag -f -a "v1.0.0" -m "v1.0.0 commit 2"
  git push -f -q github v1.0.0

  set +e
  local output
  output="$("$VERIFY_SCRIPT" -t v1.0.0 2>&1)"
  local status=$?
  set -e

  [ "$status" -eq 1 ] && echo "$output" | grep -q "Dual-remote tag disparity detected"
}

test_all_tags_parity() {
  local repo="$TEST_TMP/all_tags_parity_repo"
  local mock_origin="$TEST_TMP/all_tags_parity_origin.git"
  local mock_github="$TEST_TMP/all_tags_parity_github.git"
  git init -q --bare "$mock_origin"
  git init -q --bare "$mock_github"

  mkdir -p "$repo"
  cd "$repo"
  git init -q -b main
  git config user.name "Test User"
  git config user.email "test@example.com"
  echo "c1" > file.txt
  git add file.txt
  git commit -q -m "commit 1"

  git remote add origin "$mock_origin"
  git remote add github "$mock_github"
  git tag -a "v1.0.0" -m "v1.0.0"
  git tag -a "v1.1.0-rc.1" -m "v1.1.0-rc.1"
  git push -q origin --tags
  git push -q github --tags

  local output
  output="$("$VERIFY_SCRIPT" --tags 2>&1)"
  echo "$output" | grep -q "Dual-remote tag parity verified: all"
}

# Run tests
run_test "Exact parity scenario" test_in_parity
run_test "Detached HEAD parity scenario" test_detached_head_parity
run_test "Diverged remotes (origin ahead)" test_diverged_origin_ahead
run_test "Diverged remotes (github ahead)" test_diverged_github_ahead
run_test "Missing origin remote scenario" test_missing_origin_remote
run_test "Missing github remote scenario" test_missing_github_remote
run_test "Identical remote URLs check" test_identical_remotes_error
run_test "Outside of git repository check" test_not_a_git_repo
run_test "Invalid command line option check" test_invalid_option
run_test "Quiet mode suppresses INFO logs" test_quiet_mode
run_test "Custom branch verification" test_custom_branch
run_test "Tag parity verification" test_tag_parity
run_test "Tag disparity detection" test_tag_divergence
run_test "All tags parity verification" test_all_tags_parity
run_test "Help output" test_help_flag

echo "---"
echo "Tests passed: $PASSED, failed: $FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
