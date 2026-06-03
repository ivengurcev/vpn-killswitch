#!/usr/bin/env bash
set -euo pipefail

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOTFS_DIR="${SCRIPT_DIR}/rootfs"
INSTALL_ROOT="${DESTDIR:-/}"

VPN_NAME_OVERRIDE="${VPN_LOCK_VPN_NAME:-}"
LOG_FILE_OVERRIDE="${VPN_LOCK_LOG_FILE:-}"
DOMAINS_OVERRIDE=()
MANAGE_HOSTS_OVERRIDE=""

INSTALL_DEPS=1
ENABLE_SERVICE=1
START_SERVICE=1
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage: sudo ./setup-vpn-lock.sh --vpn NAME --domain DOMAIN [--domain DOMAIN...]

Options:
  --vpn NAME             NetworkManager VPN connection name to watch.
  --domain DOMAIN        Protected domain. Can be repeated.
  --domains LIST         Comma or space separated protected domains.
  --log-file PATH        Log file path. Default: /var/log/vpn-lock.log.
  --destdir DIR          Stage files under DIR instead of installing to /.
  --no-install-deps      Do not install missing OS packages.
  --no-enable            Do not enable the systemd service.
  --no-start             Do not run the initial protection check.
  --manage-hosts         Add 0.0.0.0/:: entries for protected domains in /etc/hosts.
  --no-manage-hosts      Disable /etc/hosts management; rely only on nftables.
  --dry-run              Print the install plan (files, config, package and
                         systemd commands) and exit without changing anything.
  -h, --help             Show this help.

Environment:
  VPN_LOCK_VPN_NAME      Same as --vpn.
  VPN_LOCK_DOMAINS       Same as --domains.
  VPN_LOCK_LOG_FILE      Same as --log-file.
  DESTDIR                Same as --destdir.
EOF
}

die() {
  printf 'setup-vpn-lock: %s\n' "$*" >&2
  exit 1
}

target_path() {
  local path="${1#/}"
  if [ "$INSTALL_ROOT" = "/" ]; then
    printf '/%s\n' "$path"
  else
    printf '%s/%s\n' "${INSTALL_ROOT%/}" "$path"
  fi
}

split_domains() {
  local raw="${1//,/ }"
  local token
  for token in $raw; do
    printf '%s\n' "$token"
  done
}

add_domain_override() {
  local domain
  while IFS= read -r domain; do
    [ -n "$domain" ] && DOMAINS_OVERRIDE+=("$domain")
  done < <(split_domains "$1")
}

shell_quote() {
  printf '%q' "$1"
}

normalize_domain() {
  local domain="${1%.}"
  domain="${domain,,}"

  if [ -z "$domain" ]; then
    return 1
  fi

  if [[ "$domain" == *".."* ]] || [[ ! "$domain" =~ ^[a-z0-9][a-z0-9._-]*[a-z0-9]$|^[a-z0-9]$ ]]; then
    return 1
  fi

  printf '%s\n' "$domain"
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --vpn)
        [ "$#" -ge 2 ] || die "--vpn requires a value"
        VPN_NAME_OVERRIDE="$2"
        shift 2
        ;;
      --domain)
        [ "$#" -ge 2 ] || die "--domain requires a value"
        add_domain_override "$2"
        shift 2
        ;;
      --domains)
        [ "$#" -ge 2 ] || die "--domains requires a value"
        add_domain_override "$2"
        shift 2
        ;;
      --log-file)
        [ "$#" -ge 2 ] || die "--log-file requires a value"
        LOG_FILE_OVERRIDE="$2"
        shift 2
        ;;
      --destdir)
        [ "$#" -ge 2 ] || die "--destdir requires a value"
        INSTALL_ROOT="$2"
        shift 2
        ;;
      --no-install-deps)
        INSTALL_DEPS=0
        shift
        ;;
      --no-enable)
        ENABLE_SERVICE=0
        shift
        ;;
      --no-start)
        START_SERVICE=0
        shift
        ;;
      --manage-hosts)
        MANAGE_HOSTS_OVERRIDE="true"
        shift
        ;;
      --no-manage-hosts)
        MANAGE_HOSTS_OVERRIDE="false"
        shift
        ;;
      --dry-run)
        DRY_RUN=1
        shift
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

  if [ -n "${VPN_LOCK_DOMAINS:-}" ]; then
    add_domain_override "$VPN_LOCK_DOMAINS"
  fi
}

require_root_for_real_install() {
  if [ "$INSTALL_ROOT" != "/" ] || [ "$DRY_RUN" -eq 1 ]; then
    return 0
  fi

  if [ "$(id -u)" -ne 0 ]; then
    die "run as root, for example: sudo ./setup-vpn-lock.sh --vpn NAME --domain example.com"
  fi
}

package_manager_install() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y nftables network-manager coreutils gawk logrotate
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y nftables NetworkManager coreutils gawk logrotate
  elif command -v yum >/dev/null 2>&1; then
    yum install -y nftables NetworkManager coreutils gawk logrotate
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --needed --noconfirm nftables networkmanager coreutils gawk logrotate
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install nftables NetworkManager coreutils gawk logrotate
  else
    return 1
  fi
}

emit_package_plan() {
  if command -v apt-get >/dev/null 2>&1; then
    printf '  apt-get update\n'
    printf '  DEBIAN_FRONTEND=noninteractive apt-get install -y nftables network-manager coreutils gawk logrotate\n'
  elif command -v dnf >/dev/null 2>&1; then
    printf '  dnf install -y nftables NetworkManager coreutils gawk logrotate\n'
  elif command -v yum >/dev/null 2>&1; then
    printf '  yum install -y nftables NetworkManager coreutils gawk logrotate\n'
  elif command -v pacman >/dev/null 2>&1; then
    printf '  pacman -Sy --needed --noconfirm nftables networkmanager coreutils gawk logrotate\n'
  elif command -v zypper >/dev/null 2>&1; then
    printf '  zypper --non-interactive install nftables NetworkManager coreutils gawk logrotate\n'
  else
    printf '  (no supported package manager detected; install nftables and NetworkManager manually)\n'
  fi
}

install_dependencies() {
  local missing=()
  local cmd

  if [ "$INSTALL_DEPS" -eq 0 ] || [ "$INSTALL_ROOT" != "/" ]; then
    return 0
  fi

  for cmd in nft nmcli getent awk mktemp stat systemctl logrotate flock; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done

  if [ "${#missing[@]}" -eq 0 ]; then
    return 0
  fi

  printf 'Missing commands: %s\n' "${missing[*]}"
  printf 'Installing required packages...\n'

  package_manager_install || die "cannot install dependencies automatically; install nftables and NetworkManager manually"

  missing=()
  for cmd in nft nmcli getent awk mktemp stat systemctl logrotate flock; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done

  [ "${#missing[@]}" -eq 0 ] || die "missing required commands after package install: ${missing[*]}"
}

load_existing_config() {
  local config_path="$1"

  VPN_CONNECTION_NAME=""
  PROTECTED_DOMAINS=()
  LOG_FILE="/var/log/vpn-lock.log"
  STATE_DIR="/var/lib/vpn-lock"
  RUN_DIR="/run/vpn-lock"
  HOSTS_FILE="/etc/hosts"
  NFT_TABLE_NAME="vpn_lock"
  RESOLVE_TIMEOUT_SECONDS="10"
  LOCK_TIMEOUT_SECONDS="10"
  DISPATCHER_DEBOUNCE_SECONDS="2"
  REFRESH_INTERVAL_MINUTES="5"
  MANAGE_HOSTS="true"
  VPN_CONNECTION_TYPES="vpn wireguard"

  if [ -r "$config_path" ]; then
    # shellcheck source=/etc/vpn-lock/vpn-lock.conf
    source "$config_path"
  elif [ -r "${ROOTFS_DIR}/etc/vpn-lock/vpn-lock.conf" ]; then
    # shellcheck source=rootfs/etc/vpn-lock/vpn-lock.conf
    source "${ROOTFS_DIR}/etc/vpn-lock/vpn-lock.conf"
  fi
}

prompt_missing_config() {
  local raw

  if [ ! -t 0 ]; then
    return 0
  fi

  if [ -z "$VPN_CONNECTION_NAME" ]; then
    printf 'NetworkManager VPN connection name: '
    IFS= read -r VPN_CONNECTION_NAME
  fi

  if [ "${#PROTECTED_DOMAINS[@]}" -eq 0 ]; then
    printf 'Protected domains, separated by spaces or commas: '
    IFS= read -r raw
    add_domain_override "$raw"
  fi
}

build_config_values() {
  local config_path="$1"
  local -A seen=()
  local normalized=()
  local domain normalized_domain

  load_existing_config "$config_path"

  if [ -n "$VPN_NAME_OVERRIDE" ]; then
    VPN_CONNECTION_NAME="$VPN_NAME_OVERRIDE"
  fi

  if [ -n "$LOG_FILE_OVERRIDE" ]; then
    LOG_FILE="$LOG_FILE_OVERRIDE"
  fi

  if [ "${#DOMAINS_OVERRIDE[@]}" -gt 0 ]; then
    PROTECTED_DOMAINS=("${DOMAINS_OVERRIDE[@]}")
  fi

  if [ -n "$MANAGE_HOSTS_OVERRIDE" ]; then
    MANAGE_HOSTS="$MANAGE_HOSTS_OVERRIDE"
  fi

  prompt_missing_config

  if [ "${#DOMAINS_OVERRIDE[@]}" -gt 0 ]; then
    PROTECTED_DOMAINS=("${DOMAINS_OVERRIDE[@]}")
  fi

  [ -n "$VPN_CONNECTION_NAME" ] || die "VPN connection name is required"
  [ "${#PROTECTED_DOMAINS[@]}" -gt 0 ] || die "at least one protected domain is required"

  for domain in "${PROTECTED_DOMAINS[@]}"; do
    while IFS= read -r normalized_domain; do
      [ -n "$normalized_domain" ] || continue
      normalize_domain "$normalized_domain" >/dev/null || die "invalid domain: $normalized_domain"
      normalized_domain="$(normalize_domain "$normalized_domain")"
      if [ -z "${seen[$normalized_domain]+x}" ]; then
        seen[$normalized_domain]=1
        normalized+=("$normalized_domain")
      fi
    done < <(split_domains "$domain")
  done

  [ "${#normalized[@]}" -gt 0 ] || die "at least one valid protected domain is required"
  PROTECTED_DOMAINS=("${normalized[@]}")
}

install_files() {
  local source target mode

  [ -d "$ROOTFS_DIR" ] || die "rootfs directory not found: $ROOTFS_DIR"

  while IFS= read -r source; do
    target="$(target_path "${source#"$ROOTFS_DIR"}")"
    install -d -m 0755 "$(dirname "$target")"

    case "$source" in
      */usr/local/sbin/vpn-lock|*/etc/systemd/system-sleep/vpn-lock|*/etc/NetworkManager/dispatcher.d/90-vpn-lock)
        mode=0755
        ;;
      *)
        mode=0644
        ;;
    esac

    install -m "$mode" "$source" "$target"
  done < <(find "$ROOTFS_DIR" -type f ! -path '*/etc/vpn-lock/vpn-lock.conf' | sort)

  if [ -f "${SCRIPT_DIR}/uninstall-vpn-lock.sh" ]; then
    target="$(target_path /usr/local/sbin/uninstall-vpn-lock)"
    install -d -m 0755 "$(dirname "$target")"
    install -m 0755 "${SCRIPT_DIR}/uninstall-vpn-lock.sh" "$target"
  fi
}

emit_config() {
  local domain
  printf '# Generated by setup-vpn-lock.sh. Edit carefully; this file is sourced by bash.\n'
  printf 'VPN_CONNECTION_NAME=%s\n' "$(shell_quote "$VPN_CONNECTION_NAME")"
  printf 'VPN_CONNECTION_TYPES=%s\n' "$(shell_quote "$VPN_CONNECTION_TYPES")"
  printf 'PROTECTED_DOMAINS=(\n'
  for domain in "${PROTECTED_DOMAINS[@]}"; do
    printf '  %s\n' "$(shell_quote "$domain")"
  done
  printf ')\n'
  printf 'LOG_FILE=%s\n' "$(shell_quote "$LOG_FILE")"
  printf 'STATE_DIR=%s\n' "$(shell_quote "$STATE_DIR")"
  printf 'RUN_DIR=%s\n' "$(shell_quote "$RUN_DIR")"
  printf 'HOSTS_FILE=%s\n' "$(shell_quote "$HOSTS_FILE")"
  printf 'NFT_TABLE_NAME=%s\n' "$(shell_quote "$NFT_TABLE_NAME")"
  printf 'RESOLVE_TIMEOUT_SECONDS=%s\n' "$(shell_quote "$RESOLVE_TIMEOUT_SECONDS")"
  printf 'LOCK_TIMEOUT_SECONDS=%s\n' "$(shell_quote "$LOCK_TIMEOUT_SECONDS")"
  printf 'DISPATCHER_DEBOUNCE_SECONDS=%s\n' "$(shell_quote "$DISPATCHER_DEBOUNCE_SECONDS")"
  printf 'REFRESH_INTERVAL_MINUTES=%s\n' "$(shell_quote "$REFRESH_INTERVAL_MINUTES")"
  printf 'MANAGE_HOSTS=%s\n' "$(shell_quote "$MANAGE_HOSTS")"
}

write_config() {
  local config_path="$1"
  local tmp

  install -d -m 0755 "$(dirname "$config_path")"
  tmp="$(mktemp "$(dirname "$config_path")/.vpn-lock.conf.XXXXXX")"

  emit_config >"$tmp"

  chmod 0644 "$tmp"
  mv "$tmp" "$config_path"
}

emit_logrotate() {
  printf '%s {\n' "$LOG_FILE"
  printf '    weekly\n'
  printf '    rotate 8\n'
  printf '    compress\n'
  printf '    missingok\n'
  printf '    notifempty\n'
  printf '    create 0644 root root\n'
  printf '}\n'
}

write_logrotate_config() {
  local logrotate_path tmp

  logrotate_path="$(target_path /etc/logrotate.d/vpn-lock)"
  install -d -m 0755 "$(dirname "$logrotate_path")"
  tmp="$(mktemp "$(dirname "$logrotate_path")/.vpn-lock.logrotate.XXXXXX")"

  emit_logrotate >"$tmp"

  chmod 0644 "$tmp"
  mv "$tmp" "$logrotate_path"
}

emit_refresh_timer() {
  local interval="$REFRESH_INTERVAL_MINUTES"
  [[ "$interval" =~ ^[0-9]+$ ]] && [ "$interval" -ge 1 ] || interval=5

  printf '[Unit]\n'
  printf 'Description=Periodic IP refresh for VPN Lock\n'
  printf '\n'
  printf '[Timer]\n'
  printf 'OnBootSec=1min\n'
  printf 'OnUnitActiveSec=%dmin\n' "$interval"
  printf 'AccuracySec=15s\n'
  printf 'Unit=vpn-lock-refresh.service\n'
  printf '\n'
  printf '[Install]\n'
  printf 'WantedBy=timers.target\n'
}

write_refresh_timer() {
  local timer_path tmp

  timer_path="$(target_path /etc/systemd/system/vpn-lock-refresh.timer)"
  install -d -m 0755 "$(dirname "$timer_path")"
  tmp="$(mktemp "$(dirname "$timer_path")/.vpn-lock-refresh.timer.XXXXXX")"

  emit_refresh_timer >"$tmp"

  chmod 0644 "$tmp"
  mv "$tmp" "$timer_path"
}

enable_and_start() {
  if [ "$INSTALL_ROOT" != "/" ]; then
    printf 'Staged vpn-lock under %s; systemd enable/start skipped.\n' "$INSTALL_ROOT"
    return 0
  fi

  systemctl daemon-reload || true

  if [ "$ENABLE_SERVICE" -eq 1 ]; then
    systemctl enable vpn-lock.service || true
    systemctl enable vpn-lock-refresh.timer || true
  fi

  if [ "$START_SERVICE" -eq 1 ]; then
    if ! systemctl start vpn-lock.service; then
      printf 'systemd start failed; running direct initial check...\n' >&2
      /usr/local/sbin/vpn-lock enforce installer
    fi
    systemctl start vpn-lock-refresh.timer || true
  fi
}

print_plan() {
  local config_path="$1"
  local source target mode

  printf '=== vpn-lock dry-run: install plan ===\n'
  printf 'INSTALL_ROOT=%s\n' "$INSTALL_ROOT"
  printf 'INSTALL_DEPS=%s ENABLE_SERVICE=%s START_SERVICE=%s\n' \
    "$INSTALL_DEPS" "$ENABLE_SERVICE" "$START_SERVICE"

  printf '\n--- Files to install ---\n'
  while IFS= read -r source; do
    target="$(target_path "${source#"$ROOTFS_DIR"}")"
    case "$source" in
      */usr/local/sbin/vpn-lock|*/etc/systemd/system-sleep/vpn-lock|*/etc/NetworkManager/dispatcher.d/90-vpn-lock)
        mode=0755
        ;;
      *)
        mode=0644
        ;;
    esac
    printf '  %s -> %s (mode=%s)\n' "$source" "$target" "$mode"
  done < <(find "$ROOTFS_DIR" -type f ! -path '*/etc/vpn-lock/vpn-lock.conf' | sort)

  if [ -f "${SCRIPT_DIR}/uninstall-vpn-lock.sh" ]; then
    printf '  %s/uninstall-vpn-lock.sh -> %s (mode=0755)\n' \
      "$SCRIPT_DIR" "$(target_path /usr/local/sbin/uninstall-vpn-lock)"
  fi

  printf '\n--- Generated %s ---\n' "$config_path"
  emit_config | sed 's/^/  /'

  printf '\n--- Generated %s ---\n' "$(target_path /etc/logrotate.d/vpn-lock)"
  emit_logrotate | sed 's/^/  /'

  printf '\n--- Generated %s ---\n' "$(target_path /etc/systemd/system/vpn-lock-refresh.timer)"
  emit_refresh_timer | sed 's/^/  /'

  if [ "$INSTALL_DEPS" -eq 1 ] && [ "$INSTALL_ROOT" = "/" ]; then
    printf '\n--- Package manager actions (would run if commands are missing) ---\n'
    emit_package_plan
  else
    printf '\n--- Package manager actions: skipped (--no-install-deps or staged install) ---\n'
  fi

  printf '\n--- Runtime / systemd actions ---\n'
  if [ "$INSTALL_ROOT" = "/" ]; then
    printf '  install -d -m 0755 %s %s\n' "$STATE_DIR" "$RUN_DIR"
    printf '  systemd-tmpfiles --create /usr/lib/tmpfiles.d/vpn-lock.conf\n'
    printf '  /usr/local/sbin/vpn-lock install-check\n'
    printf '  systemctl daemon-reload\n'
    if [ "$ENABLE_SERVICE" -eq 1 ]; then
      printf '  systemctl enable vpn-lock.service\n'
      printf '  systemctl enable vpn-lock-refresh.timer\n'
    fi
    if [ "$START_SERVICE" -eq 1 ]; then
      printf '  systemctl start vpn-lock.service\n'
      printf '  systemctl start vpn-lock-refresh.timer\n'
    fi
  else
    printf '  (staged under %s; systemd actions skipped)\n' "$INSTALL_ROOT"
  fi

  printf '\n=== dry-run: nothing was changed on disk ===\n'
}

main() {
  local config_path

  parse_args "$@"
  require_root_for_real_install

  config_path="$(target_path /etc/vpn-lock/vpn-lock.conf)"

  if [ "$DRY_RUN" -eq 1 ]; then
    build_config_values "$config_path"
    print_plan "$config_path"
    return 0
  fi

  install_dependencies
  build_config_values "$config_path"
  install_files
  write_config "$config_path"
  write_logrotate_config
  write_refresh_timer

  if [ "$INSTALL_ROOT" = "/" ]; then
    install -d -m 0755 "$STATE_DIR" "$RUN_DIR"
    if command -v systemd-tmpfiles >/dev/null 2>&1; then
      systemd-tmpfiles --create /usr/lib/tmpfiles.d/vpn-lock.conf || true
    fi
    /usr/local/sbin/vpn-lock install-check
  fi

  enable_and_start

  printf 'vpn-lock installed for VPN "%s"; protected domains: %s\n' \
    "$VPN_CONNECTION_NAME" "${PROTECTED_DOMAINS[*]}"
}

main "$@"
