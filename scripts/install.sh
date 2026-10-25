#!/usr/bin/env bash
# ==============================================================================
# Sovereign Appliance: One-Line Linux Turnkey Host Installer
# Repository: https://github.com/j333c/sovereign
# License: Apache 2.0
# ==============================================================================

set -euo pipefail

# Configuration Defaults
REPO_URL="https://github.com/j333c/sovereign.git"
INSTALL_DIR="/opt/sovereign"
DATA_DIR="/data"
SYSTEMD_UNIT="/etc/systemd/system/sovereign.service"
NON_INTERACTIVE=false
SKIP_DOCKER=false
UNINSTALL=false
UPGRADE=false
PURGE_DATA=false

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
  echo -e "${BLUE}[INFO]${RESET} $1"
}

log_ok() {
  echo -e "${GREEN}[OK]${RESET} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${RESET} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${RESET} $1" >&2
}

print_banner() {
  echo -e "${CYAN}${BOLD}"
  echo "  ███████╗ ██████╗ ██╗   ██╗███████╗██████╗ ███████╗██╗ ██████╗ ███╗   ██╗"
  echo "  ██╔════╝██╔═══██╗██║   ██║██╔════╝██╔══██╗██╔════╝██║██╔════╝ ████╗  ██║"
  echo "  ███████╗██║   ██║██║   ██║█████╗  ██████╔╝█████╗  ██║██║  ███╗██╔██╗ ██║"
  echo "  ╚════██║██║   ██║╚██╗ ██╔╝██╔══╝  ██╔══██╗██╔══╝  ██║██║   ██║██║╚██╗██║"
  echo "  ███████║╚██████╔╝ ╚████╔╝ ███████╗██║  ██║███████╗██║╚██████╔╝██║ ╚████║"
  echo "  ╚══════╝ ╚═════╝   ╚═══╝  ╚══════╝╚═╝  ╚═╝╚══════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝"
  echo -e "${RESET}"
  echo -e "${BOLD}  The 100% Private Self-Hosted Home Appliance${RESET}"
  echo -e "  Repository: ${BLUE}https://github.com/j333c/sovereign${RESET}"
  echo "------------------------------------------------------------------------"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -y|--yes)
        NON_INTERACTIVE=true
        shift
        ;;
      --no-docker)
        SKIP_DOCKER=true
        shift
        ;;
      --uninstall)
        UNINSTALL=true
        shift
        ;;
      --upgrade)
        UPGRADE=true
        shift
        ;;
      --purge-data)
        PURGE_DATA=true
        shift
        ;;
      -h|--help)
        echo "Usage: curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  -y, --yes          Proceed without interactive confirmation prompts"
        echo "  --upgrade          Update Sovereign from https://github.com/j333c/sovereign and rebuild"
        echo "  --uninstall        Uninstall Sovereign and disable the systemd service"
        echo "  --purge-data       With --uninstall, delete persistent /data storage as well"
        echo "  --no-docker        Skip automated Docker CE and Compose plugin installation"
        echo "  -h, --help         Show this help message"
        exit 0
        ;;
      *)
        log_error "Unknown option: $1"
        exit 1
        ;;
    esac
  done
}

check_root() {
  if [ "$(id -u)" -ne 0 ]; then
    log_error "This script must be run as root or with sudo privileges."
    echo "Please re-run: curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash"
    exit 1
  fi
}

validate_environment() {
  log_info "Validating host environment and hardware profile..."

  # Architecture verification
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      log_ok "CPU architecture: $arch (x86_64 supported)"
      ;;
    aarch64|arm64)
      log_ok "CPU architecture: $arch (ARM64 supported)"
      ;;
    *)
      log_error "Unsupported architecture: $arch. Sovereign requires x86_64 or ARM64."
      exit 1
      ;;
  esac

  # OS distribution detection
  if [ -f /etc/os-release ]; then
    # shellcheck source=/dev/null
    . /etc/os-release
    log_ok "Operating system: ${NAME:-Linux} ${VERSION_ID:-}"
  else
    log_warn "Could not identify distribution via /etc/os-release. Proceeding with generic Linux checks."
  fi

  # Systemd verification
  if [ ! -d /run/systemd/system ]; then
    log_warn "Systemd does not appear to be active as PID 1. Automatic service startup requires systemd."
  else
    log_ok "Systemd init system detected."
  fi

  # RAM verification (target >= 4GB, minimum 2GB)
  if [ -f /proc/meminfo ]; then
    local total_ram_kb
    total_ram_kb="$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
    local total_ram_mb=$((total_ram_kb / 1024))
    if [ "$total_ram_mb" -lt 2000 ]; then
      log_warn "Low memory detected: ${total_ram_mb} MB RAM. Minimum recommended is 4 GB (4096 MB)."
    elif [ "$total_ram_mb" -lt 3800 ]; then
      log_ok "Memory: ${total_ram_mb} MB RAM (Light Tier suitable, 4 GB+ recommended for Local AI)"
    else
      log_ok "Memory: ${total_ram_mb} MB RAM (Meets appliance recommended specifications)"
    fi
  fi

  # Disk space check
  local free_disk_mb
  free_disk_mb="$(df -m / | awk 'NR==2 {print $4}')"
  if [ "$free_disk_mb" -lt 10000 ]; then
    log_error "Insufficient root disk space: ${free_disk_mb} MB available. Sovereign requires at least 10 GB (10000 MB) free space."
    exit 1
  elif [ "$free_disk_mb" -lt 20000 ]; then
    log_warn "Disk space: ${free_disk_mb} MB available on root filesystem. 20 GB+ recommended for container caching."
  else
    log_ok "Disk space: ${free_disk_mb} MB available on root filesystem."
  fi
}

install_dependencies() {
  log_info "Ensuring core prerequisite tools (git, curl, tar) are installed..."

  if command -v git >/dev/null 2>&1 && command -v curl >/dev/null 2>&1 && command -v tar >/dev/null 2>&1; then
    log_ok "Core prerequisites already installed."
    return 0
  fi

  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq
    apt-get install -y -qq git curl tar ca-certificates
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q git curl tar ca-certificates
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm git curl tar ca-certificates
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache git curl tar ca-certificates bash
  else
    log_warn "Unknown package manager. Please ensure git, curl, and tar are installed manually."
  fi
}

ensure_docker() {
  if [ "$SKIP_DOCKER" = true ]; then
    log_info "Skipping automated Docker check as requested (--no-docker)."
    return 0
  fi

  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    local d_ver
    d_ver="$(docker --version 2>/dev/null || echo 'installed')"
    local c_ver
    c_ver="$(docker compose version 2>/dev/null || echo 'installed')"
    log_ok "Docker runtime detected: $d_ver · $c_ver"
    return 0
  fi

  log_info "Docker CE or Compose plugin v2 missing. Installing official Docker runtime..."
  curl -fsSL https://get.docker.com | sh

  # Start and enable Docker
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable docker
    systemctl start docker
  fi

  # Add calling sudo user to docker group if applicable
  if [ -n "${SUDO_USER:-}" ] && [ "${SUDO_USER}" != "root" ]; then
    usermod -aG docker "${SUDO_USER}" 2>/dev/null || true
    log_info "Added user '${SUDO_USER}' to docker group."
  fi

  log_ok "Docker engine and compose plugin successfully configured."
}

deploy_sovereign() {
  log_info "Deploying Sovereign appliance to ${INSTALL_DIR} from ${REPO_URL}..."

  mkdir -p "$(dirname "$INSTALL_DIR")"

  if [ -d "${INSTALL_DIR}/.git" ]; then
    log_info "Existing git repository found at ${INSTALL_DIR}. Updating to latest release..."
    git -C "${INSTALL_DIR}" fetch origin
    git -C "${INSTALL_DIR}" checkout main
    git -C "${INSTALL_DIR}" pull origin main
  else
    log_info "Cloning Sovereign repository into ${INSTALL_DIR}..."
    git clone "${REPO_URL}" "${INSTALL_DIR}"
  fi

  # Initialize structured /data partition hierarchy
  log_info "Initializing structured ${DATA_DIR} hierarchy..."
  mkdir -p "${DATA_DIR}/state/compose"
  mkdir -p "${DATA_DIR}/state/backups"
  mkdir -p "${DATA_DIR}/media/movies"
  mkdir -p "${DATA_DIR}/media/tv"
  mkdir -p "${DATA_DIR}/media/music"
  mkdir -p "${DATA_DIR}/media/audiobooks"
  mkdir -p "${DATA_DIR}/documents"
  mkdir -p "${DATA_DIR}/models"
  mkdir -p "${DATA_DIR}/drop/incoming"
  mkdir -p "${DATA_DIR}/caddy/data"
  mkdir -p "${DATA_DIR}/caddy/config"

  chmod -R 775 "${DATA_DIR}" || true
  log_ok "Persistent storage directory hierarchy initialized."
}

configure_systemd() {
  log_info "Registering sovereign.service systemd unit..."

  cat << 'EOF' > "${SYSTEMD_UNIT}"
[Unit]
Description=Sovereign Home Server Appliance
Documentation=https://github.com/j333c/sovereign
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/sovereign
ExecStart=/usr/bin/docker compose -f /opt/sovereign/docker-compose.yml up -d --remove-orphans
ExecStop=/usr/bin/docker compose -f /opt/sovereign/docker-compose.yml down
TimeoutStartSec=600

[Install]
WantedBy=multi-user.target
EOF

  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    systemctl enable sovereign.service
    log_ok "Systemd service 'sovereign.service' enabled on machine boot."
  fi
}

start_services() {
  log_info "Building and launching Sovereign containers via Docker Compose..."
  cd "${INSTALL_DIR}"
  docker compose up -d --build

  log_ok "All appliance services launched successfully."
}

configure_firewall() {
  if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
    log_info "UFW active. Permitting ports 80/tcp, 443/tcp, and 5353/udp..."
    ufw allow 80/tcp comment 'Sovereign HTTP Gateway' >/dev/null 2>&1 || true
    ufw allow 443/tcp comment 'Sovereign HTTPS Gateway' >/dev/null 2>&1 || true
    ufw allow 5353/udp comment 'Sovereign mDNS Multicast' >/dev/null 2>&1 || true
  elif command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
    log_info "Firewalld active. Opening http, https, and mdns services..."
    firewall-cmd --permanent --add-service=http --add-service=https --add-service=mdns >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
  fi
}

get_primary_ip() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [ -z "$ip" ]; then
    ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{print $7}')"
  fi
  echo "${ip:-127.0.0.1}"
}

display_completion_banner() {
  local ip
  ip="$(get_primary_ip)"

  echo ""
  echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════════════════${RESET}"
  echo -e "${GREEN}${BOLD}   🎉 Sovereign Home Appliance Installation Completed Successfully!    ${RESET}"
  echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════════════════${RESET}"
  echo ""
  echo -e "Your private server is now running and reachable across your home network:"
  echo ""
  echo -e "  ${BOLD}► Local mDNS Link:${RESET}  ${CYAN}${BOLD}http://sovereign.local${RESET}"
  echo -e "  ${BOLD}► Direct IP Link:${RESET}   ${CYAN}${BOLD}http://${ip}${RESET}"
  echo ""
  echo -e "Next Steps:"
  echo -e "  1. Open ${BOLD}http://sovereign.local${RESET} (or ${BOLD}http://${ip}${RESET}) in any web browser."
  echo -e "  2. Follow the 5-step First-Run Wizard to configure your storage and select spokes."
  echo -e "  3. Service logs can be inspected anytime with: ${BOLD}docker compose -f ${INSTALL_DIR}/docker-compose.yml logs -f${RESET}"
  echo ""
  echo -e "${BLUE}Repository:${RESET} https://github.com/j333c/sovereign"
  echo "------------------------------------------------------------------------"
}

uninstall_sovereign() {
  log_warn "Initiating uninstallation of Sovereign Appliance..."

  if [ "$NON_INTERACTIVE" = false ]; then
    read -r -p "Are you sure you want to uninstall Sovereign from this machine? [y/N]: " confirm
    if [[ ! "$confirm" =~ ^[yY]([eE][sS])?$ ]]; then
      log_info "Uninstallation cancelled by user."
      exit 0
    fi
  fi

  # Stop and disable systemd
  if [ -f "${SYSTEMD_UNIT}" ]; then
    log_info "Stopping and disabling sovereign.service..."
    systemctl stop sovereign.service 2>/dev/null || true
    systemctl disable sovereign.service 2>/dev/null || true
    rm -f "${SYSTEMD_UNIT}"
    systemctl daemon-reload 2>/dev/null || true
  fi

  # Stop and remove containers
  if [ -d "${INSTALL_DIR}" ]; then
    log_info "Stopping and removing Sovereign containers..."
    docker compose -f "${INSTALL_DIR}/docker-compose.yml" down 2>/dev/null || true
    log_info "Removing ${INSTALL_DIR}..."
    rm -rf "${INSTALL_DIR}"
  fi

  # Prompt or purge data
  if [ "$PURGE_DATA" = true ]; then
    log_warn "Purging persistent storage at ${DATA_DIR}..."
    rm -rf "${DATA_DIR}"
  else
    log_info "Persistent data at ${DATA_DIR} preserved."
  fi

  log_ok "Sovereign has been cleanly uninstalled."
  exit 0
}

upgrade_sovereign() {
  log_info "Upgrading Sovereign from ${REPO_URL}..."

  if [ ! -d "${INSTALL_DIR}/.git" ]; then
    log_error "No existing git installation found at ${INSTALL_DIR}. Run standard install instead."
    exit 1
  fi

  log_info "Pulling latest code changes..."
  git -C "${INSTALL_DIR}" fetch origin
  git -C "${INSTALL_DIR}" checkout main
  git -C "${INSTALL_DIR}" pull origin main

  log_info "Rebuilding and restarting updated containers..."
  docker compose -f "${INSTALL_DIR}/docker-compose.yml" up -d --build

  log_ok "Sovereign successfully upgraded to the latest version."
  display_completion_banner
  exit 0
}

main() {
  parse_args "$@"
  print_banner
  check_root

  if [ "$UNINSTALL" = true ]; then
    uninstall_sovereign
  fi

  if [ "$UPGRADE" = true ]; then
    upgrade_sovereign
  fi

  if [ "$NON_INTERACTIVE" = false ]; then
    echo -e "This script will install Sovereign and its prerequisites to ${BOLD}${INSTALL_DIR}${RESET}."
    echo -e "Data will be safely stored under ${BOLD}${DATA_DIR}${RESET}."
    echo ""
    read -r -p "Do you wish to proceed? [Y/n]: " proceed
    if [[ "$proceed" =~ ^[nN] ]]; then
      log_info "Installation aborted by user."
      exit 0
    fi
  fi

  validate_environment
  install_dependencies
  ensure_docker
  deploy_sovereign
  configure_systemd
  start_services
  configure_firewall
  display_completion_banner
}

main "$@"
