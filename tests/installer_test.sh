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

# Test 12: SELinux detection handling
test_selinux_detection() {
  local mock_bin="$TEST_TMP/mock_bin"
  mkdir -p "$mock_bin"

  # Case 1: SELinux active with getenforce Enforcing
  cat << 'EOF' > "$mock_bin/selinuxenabled"
#!/usr/bin/env bash
exit 0
EOF
  cat << 'EOF' > "$mock_bin/getenforce"
#!/usr/bin/env bash
echo "Enforcing"
EOF
  chmod +x "$mock_bin/selinuxenabled" "$mock_bin/getenforce"

  local output
  output="$(PATH="$mock_bin:$PATH" bash "$INSTALLER" --dry-run 2>&1)"
  if ! echo "$output" | grep -q "SELinux: Enforcing mode detected"; then
    echo "Expected SELinux enforcing detection output, got: $output" >&2
    return 1
  fi

  # Case 2: SELinux active in Permissive mode
  cat << 'EOF' > "$mock_bin/getenforce"
#!/usr/bin/env bash
echo "Permissive"
EOF
  chmod +x "$mock_bin/getenforce"
  output="$(PATH="$mock_bin:$PATH" bash "$INSTALLER" --dry-run 2>&1)"
  if ! echo "$output" | grep -q "SELinux: Permissive mode detected"; then
    echo "Expected SELinux permissive detection output, got: $output" >&2
    return 1
  fi

  # Case 3: SELinux disabled
  cat << 'EOF' > "$mock_bin/selinuxenabled"
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "$mock_bin/selinuxenabled"
  output="$(PATH="$mock_bin:$PATH" bash "$INSTALLER" --dry-run 2>&1)"
  if echo "$output" | grep -q "SELinux:"; then
    echo "Unexpected SELinux detection when disabled, got: $output" >&2
    return 1
  fi
}

# Test 13: Containerized matrix (runs live containers when Docker daemon is accessible)
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

# Test 14: Channel flag validation
test_channel_flag() {
  local output
  output="$(bash "$INSTALLER" --channel stable --dry-run 2>&1)"
  echo "$output" | grep -q "Pre-flight dry run completed"

  output="$(bash "$INSTALLER" --channel preview --dry-run 2>&1)"
  echo "$output" | grep -q "Pre-flight dry run completed"

  output="$(bash "$INSTALLER" --channel security-only --dry-run 2>&1)"
  echo "$output" | grep -q "Pre-flight dry run completed"

  set +e
  output="$(bash "$INSTALLER" --channel invalid 2>&1)"
  local status=$?
  set -e
  [ "$status" -eq 1 ] && echo "$output" | grep -q "Invalid release channel"

  set +e
  output="$(bash "$INSTALLER" --channel 2>&1)"
  local status_missing=$?
  set -e
  [ "$status_missing" -eq 1 ] && echo "$output" | grep -q "requires an argument"
}

# Test 15: Upgrade with channel flag dry-run simulation
test_upgrade_channel_dry_run() {
  local output
  output="$(bash "$INSTALLER" --upgrade --channel preview --dry-run 2>&1)"
  echo "$output" | grep -q "Would checkout qualifying tag for channel: preview" && echo "$output" | grep -q "Would persist channel preview"
}

# Test 16: Piped stdin execution (curl | bash) displays prompt and reads from /dev/tty
test_piped_tty_prompt() {
  local mock_bin="$TEST_TMP/mock_root_bin"
  mkdir -p "$mock_bin"
  cat << 'EOF' > "$mock_bin/id"
#!/usr/bin/env bash
echo "0"
EOF
  chmod +x "$mock_bin/id"

  # 1. File execution with piped stdin ("n")
  local file_out
  file_out="$(printf "n\n" | PATH="$mock_bin:$PATH" bash "$INSTALLER" 2>&1)"
  echo "$file_out" | grep -q "Do you wish to proceed? \[Y/n\]: "
  echo "$file_out" | grep -q "Installation aborted by user."

  # 2. Script piped via stdin (simulating curl -fsSL ... | sudo bash) with controlling PTY
  if command -v python3 >/dev/null 2>&1; then
    PATH="$mock_bin:$PATH" INSTALLER_PATH="$INSTALLER" python3 - << 'PYEOF'
import fcntl, os, pty, select, subprocess, sys, termios

installer_path = os.environ["INSTALLER_PATH"]
with open(installer_path, "rb") as f:
    script_bytes = f.read()

def run_piped(args, expected_prompt, reply, expected_msg):
    master, slave = pty.openpty()
    def set_ctty():
        os.setsid()
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)

    proc = subprocess.Popen(
        ["bash"] + args,
        stdin=subprocess.PIPE,
        stdout=slave,
        stderr=slave,
        preexec_fn=set_ctty,
        close_fds=True,
    )
    os.close(slave)
    proc.stdin.write(script_bytes)
    proc.stdin.close()

    out = b""
    replied = False
    while True:
        r, _, _ = select.select([master], [], [], 3.0)
        if not r:
            break
        try:
            chunk = os.read(master, 1024)
            if not chunk:
                break
            out += chunk
            if not replied and expected_prompt in out:
                os.write(master, reply)
                replied = True
        except OSError:
            break

    proc.wait(timeout=5)
    os.close(master)
    decoded = out.decode("utf-8", errors="replace")
    if not replied or expected_msg not in decoded or proc.returncode != 0:
        sys.stderr.write(f"PTY test failed (rc={proc.returncode}, replied={replied}):\n{decoded}\n")
        sys.exit(1)

run_piped(
    [],
    b"Do you wish to proceed? [Y/n]: ",
    b"n\n",
    "Installation aborted by user.",
)
run_piped(
    ["-s", "--", "--uninstall"],
    b"Are you sure you want to uninstall Sovereign from this machine? [y/N]: ",
    b"n\n",
    "Uninstallation cancelled by user.",
)
PYEOF
  fi
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
run_test "SELinux detection handling" test_selinux_detection
run_test "Release channel flag validation" test_channel_flag
run_test "Upgrade channel selection dry-run" test_upgrade_channel_dry_run
run_test "Piped stdin (curl | bash) interactive prompt" test_piped_tty_prompt
run_test "Containerized live matrix test" test_containerized_matrix

echo "---"
echo "Tests passed: $PASSED, skipped: $SKIPPED, failed: $FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
