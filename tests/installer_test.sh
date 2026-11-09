#!/usr/bin/env bash
# ==============================================================================
# Sovereign: Containerized & Mock Host Installer Matrix Test Suite
# Tests scripts/install.sh and install.sh across supported distributions
# (Ubuntu, Debian, Fedora, Arch) and execution flags.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALLER="$REPO_ROOT/install.sh"
SCRIPTS_INSTALLER="$REPO_ROOT/scripts/install.sh"

PASSED=0
FAILED=0
SKIPPED=0

run_test() {
  local name="$1"
  shift
  echo -n "Running test: $name ... "
  # Run in isolated subshell
  local result=0
  output="$(( "$@" ) 2>&1)" || result=$?

  if [ "$result" -eq 0 ]; then
    if echo "$output" | grep -q "^SKIP:"; then
      local skip_reason
      skip_reason="$(echo "$output" | grep "^SKIP:" | head -n1)"
      echo "$skip_reason"
      SKIPPED=$((SKIPPED + 1))
    else
      echo "PASS"
      PASSED=$((PASSED + 1))
    fi
  else
    echo "FAIL"
    echo "$output" >&2
    FAILED=$((FAILED + 1))
  fi
}

TEST_TMP="$(mktemp -d /tmp/sovereign-installer-test.XXXXXX)"
trap 'rm -rf "$TEST_TMP"' EXIT

# Test 1: Bash syntax validation
test_syntax() {
  bash -n "$INSTALLER"
  bash -n "$SCRIPTS_INSTALLER"
}

# Test 2: File parity between root install.sh and scripts/install.sh
test_file_parity() {
  cmp "$INSTALLER" "$SCRIPTS_INSTALLER"
}

# Test 3: Help option output
test_help_flag() {
  local output
  output="$(bash "$INSTALLER" --help 2>&1)"
  echo "$output" | grep -q "Usage: curl" && echo "$output" | grep -q -- "--dry-run"
}

# Test 4: Unknown command-line option
test_unknown_flag() {
  set +e
  local output
  output="$(bash "$INSTALLER" --unknown-option 2>&1)"
  local status=$?
  set -e
  [ "$status" -eq 1 ] && echo "$output" | grep -q "Unknown option"
}

# Test 5: Root permission requirement
test_root_guard() {
  if [ "$(id -u)" -eq 0 ]; then
    echo "SKIP: test running as root"
    return 0
  fi
  set +e
  local output
  output="$(bash "$INSTALLER" 2>&1)"
  local status=$?
  set -e
  [ "$status" -eq 1 ] && echo "$output" | grep -q "must be run as root"
}

# Test 6: Pre-flight dry run
test_dry_run() {
  local output
  output="$(bash "$INSTALLER" --dry-run 2>&1)"
  echo "$output" | grep -q "Pre-flight dry run completed"
}

# Test 7: Non-interactive dry run
test_non_interactive_dry_run() {
  local output
  output="$(bash "$INSTALLER" -y --dry-run 2>&1)"
  echo "$output" | grep -q "Pre-flight dry run completed"
}

# Test 8: Uninstallation dry run
test_uninstall_dry_run() {
  local output
  output="$(bash "$INSTALLER" --uninstall --dry-run 2>&1)"
  echo "$output" | grep -q "Sovereign uninstallation dry-run completed"
  echo "$output" | grep -q "Would preserve persistent storage"
}

# Test 9: Uninstallation with purge dry run
test_uninstall_purge_dry_run() {
  local output
  output="$(bash "$INSTALLER" --uninstall --purge-data --dry-run 2>&1)"
  echo "$output" | grep -q "Sovereign uninstallation dry-run completed"
  echo "$output" | grep -q "Would purge persistent storage"
}

# Test 10: Upgrade dry run
test_upgrade_dry_run() {
  local output
  output="$(bash "$INSTALLER" --upgrade --dry-run 2>&1)"
  echo "$output" | grep -q "Sovereign upgrade dry-run completed"
}

# Test 11: Mock OS Distribution Matrix (Ubuntu / Debian / Fedora / Arch)
test_mock_os_matrix() {
  local mock_dir="$TEST_TMP/os_matrix"
  mkdir -p "$mock_dir"

  # Distro definitions
  local distros=("ubuntu" "debian" "fedora" "arch")
  for distro in "${distros[@]}"; do
    local os_file="$mock_dir/os-release-$distro"
    local expected_str=""
    case "$distro" in
      ubuntu)
        cat << 'EOF' > "$os_file"
NAME="Ubuntu"
VERSION_ID="22.04"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.4 LTS"
EOF
        expected_str="Operating system: Ubuntu 22.04"
        ;;
      debian)
        cat << 'EOF' > "$os_file"
NAME="Debian GNU/Linux"
VERSION_ID="12"
ID=debian
PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
EOF
        expected_str="Operating system: Debian GNU/Linux 12"
        ;;
      fedora)
        cat << 'EOF' > "$os_file"
NAME="Fedora Linux"
VERSION_ID="39"
ID=fedora
PRETTY_NAME="Fedora Linux 39 (Server Edition)"
EOF
        expected_str="Operating system: Fedora Linux 39"
        ;;
      arch)
        cat << 'EOF' > "$os_file"
NAME="Arch Linux"
ID=arch
PRETTY_NAME="Arch Linux"
EOF
        expected_str="Operating system: Arch Linux"
        ;;
    esac

    # Test execution injecting OS_RELEASE_FILE without mutating script
    local output
    output="$(OS_RELEASE_FILE="$os_file" bash "$INSTALLER" --dry-run 2>&1)"
    if ! echo "$output" | grep -q "$expected_str"; then
      echo "Failed OS detection for $distro: expected '$expected_str', got output:\n$output" >&2
      return 1
    fi
  done
}

# Test 12: Containerized matrix (runs live containers when Docker daemon is accessible)
test_containerized_matrix() {
  if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "SKIP: Docker daemon not running or socket unavailable"
    return 0
  fi

  local images=("ubuntu:22.04" "debian:12-slim" "fedora:latest")
  for img in "${images[@]}"; do
    local output
    output="$(docker run --rm -v "$INSTALLER:/install.sh:ro" "$img" bash /install.sh -y --dry-run 2>&1)"
    if ! echo "$output" | grep -q "Pre-flight dry run completed"; then
      echo "Container execution failed on $img: $output" >&2
      return 1
    fi
  done
}

# Execute tests
run_test "Installer bash syntax" test_syntax
run_test "Root and scripts installer parity" test_file_parity
run_test "Help documentation flag" test_help_flag
run_test "Unknown command-line argument handling" test_unknown_flag
run_test "Root privileges guard" test_root_guard
run_test "Pre-flight dry-run validation" test_dry_run
run_test "Non-interactive (-y) dry-run validation" test_non_interactive_dry_run
run_test "Uninstallation dry-run simulation" test_uninstall_dry_run
run_test "Uninstallation with purge dry-run simulation" test_uninstall_purge_dry_run
run_test "Upgrade dry-run simulation" test_upgrade_dry_run
run_test "Mock OS distribution matrix (Ubuntu/Debian/Fedora/Arch)" test_mock_os_matrix
run_test "Containerized live matrix test" test_containerized_matrix

echo "---"
echo "Tests passed: $PASSED, skipped: $SKIPPED, failed: $FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
