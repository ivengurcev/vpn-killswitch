#!/usr/bin/env bash
set -euo pipefail

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

INSTALL_ROOT="${DESTDIR:-/}"
PURGE=0

VPN_CONNECTION_NAME=""
PROTECTED_DOMAINS=()
LOG_FILE="/var/log/vpn-lock.log"
STATE_DIR="/var/lib/vpn-lock"
RUN_DIR="/run/vpn-lock"
HOSTS_FILE="/etc/hosts"
NFT_TABLE_NAME="vpn_lock"

HOSTS_BEGIN="# vpn-lock managed block: begin"
HOSTS_END="# vpn-lock managed block: end"

usage() {
  cat <<'EOF'
Usage: sudo ./uninstall-vpn-lock.sh [--purge]

Options:
  --purge        Also remove /etc/vpn-lock, /var/lib/vpn-lock and the log file.
  --destdir DIR  Remove files from a staged root instead of /.
  -h, --help     Show this help.

Environment:
  DESTDIR        Same as --destdir.
EOF
}

die() {
  printf 'uninstall-vpn-lock: %s\n' "$*" >&2
  exit 1
}

info() {
  printf 'uninstall-vpn-lock: %s\n' "$*"
}

target_path() {
  local path="${1#/}"
  if [ "$INSTALL_ROOT" = "/" ]; then
    printf '/%s\n' "$path"
  else
    printf '%s/%s\n' "${INSTALL_ROOT%/}" "$path"
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --purge)
        PURGE=1
        shift
        ;;
      --destdir)
        [ "$#" -ge 2 ] || die "--destdir requires a value"
        INSTALL_ROOT="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown option: $1"
        ;;
    esac
  done
}

require_root_for_real_uninstall() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  if [ "$(id -u)" -ne 0 ]; then
    die "run as root, for example: sudo ./uninstall-vpn-lock.sh"
  fi
}

load_config() {
  local config_path

  config_path="$(target_path /etc/vpn-lock/vpn-lock.conf)"
  if [ -r "$config_path" ]; then
    # shellcheck source=/etc/vpn-lock/vpn-lock.conf
    source "$config_path"
  fi

  if [[ ! "$NFT_TABLE_NAME" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
    NFT_TABLE_NAME="vpn_lock"
  fi
}

disable_service() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now vpn-lock-refresh.timer >/dev/null 2>&1 || true
    systemctl stop vpn-lock-refresh.service >/dev/null 2>&1 || true
    systemctl disable --now vpn-lock.service >/dev/null 2>&1 || true
  fi
}

reload_systemd() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi
}

remove_hosts_block_fallback() {
  local file="$HOSTS_FILE"
  local dir tmp perms uid gid

  if [ "$INSTALL_ROOT" != "/" ] || [ ! -e "$file" ]; then
    return 0
  fi

  dir="$(dirname "$file")"
  tmp="$(mktemp "${dir%/}/.vpn-lock.hosts.XXXXXX")" || return 1

  perms="$(stat -c '%a' "$file" 2>/dev/null || printf '%s\n' 644)"
  uid="$(stat -c '%u' "$file" 2>/dev/null || printf '%s\n' 0)"
  gid="$(stat -c '%g' "$file" 2>/dev/null || printf '%s\n' 0)"

  if ! awk -v begin="$HOSTS_BEGIN" -v end="$HOSTS_END" '
    $0 == begin { skip = 1; next }
    $0 == end { skip = 0; next }
    !skip { print }
  ' "$file" >"$tmp"; then
    rm -f "$tmp"
    return 1
  fi

  chown "$uid:$gid" "$tmp" 2>/dev/null || true
  chmod "$perms" "$tmp" 2>/dev/null || true
  mv "$tmp" "$file"
}

remove_nft_table_fallback() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  if command -v nft >/dev/null 2>&1; then
    nft delete table inet "$NFT_TABLE_NAME" >/dev/null 2>&1 || true
  fi
}

remove_active_protection() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  if [ -x /usr/local/sbin/vpn-lock ]; then
    /usr/local/sbin/vpn-lock disable uninstall >/dev/null 2>&1 || true
  fi

  remove_hosts_block_fallback || info "warning: could not remove managed hosts block"
  remove_nft_table_fallback
}

remove_installed_files() {
  rm -f \
    "$(target_path /usr/local/sbin/vpn-lock)" \
    "$(target_path /usr/local/sbin/uninstall-vpn-lock)" \
    "$(target_path /etc/systemd/system/vpn-lock.service)" \
    "$(target_path /etc/systemd/system/vpn-lock-refresh.service)" \
    "$(target_path /etc/systemd/system/vpn-lock-refresh.timer)" \
    "$(target_path /etc/systemd/system-sleep/vpn-lock)" \
    "$(target_path /usr/lib/systemd/system-sleep/vpn-lock)" \
    "$(target_path /etc/NetworkManager/dispatcher.d/90-vpn-lock)" \
    "$(target_path /etc/logrotate.d/vpn-lock)" \
    "$(target_path /usr/lib/tmpfiles.d/vpn-lock.conf)"
}

is_vpn_lock_path() {
  local path="$1"
  [[ "$path" = /* && "$path" == *vpn-lock* && "$path" != "/" ]]
}

safe_remove_tree() {
  local path="$1"

  if is_vpn_lock_path "$path"; then
    rm -rf "$(target_path "$path")"
  else
    info "warning: refusing to purge unsafe directory path: $path"
  fi
}

safe_remove_file() {
  local path="$1"

  if is_vpn_lock_path "$path"; then
    rm -f "$(target_path "$path")"
  else
    info "warning: refusing to purge unsafe file path: $path"
  fi
}

purge_data() {
  if [ "$PURGE" -ne 1 ]; then
    return 0
  fi

  safe_remove_tree /etc/vpn-lock
  safe_remove_tree "$STATE_DIR"
  safe_remove_tree "$RUN_DIR"
  safe_remove_file "$LOG_FILE"
}

main() {
  parse_args "$@"
  require_root_for_real_uninstall
  load_config

  remove_active_protection
  disable_service
  remove_installed_files
  purge_data
  reload_systemd

  if [ "$PURGE" -eq 1 ]; then
    info "uninstalled and purged"
  else
    info "uninstalled; configuration, state and log files were kept"
  fi
}

main "$@"
