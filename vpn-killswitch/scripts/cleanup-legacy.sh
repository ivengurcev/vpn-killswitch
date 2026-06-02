#!/usr/bin/env bash
set -euo pipefail

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

APPLY=0
PURGE=0

usage() {
  cat <<'EOF'
Usage: ./scripts/cleanup-legacy.sh [--apply] [--purge]

One-time cleanup for old vpn-lock and vpn-bypass installations.

Default mode is dry-run. Nothing is changed unless --apply is passed.

Options:
  --apply   Actually remove files and state.
  --purge   Also remove old config/state/log directories where known.
  -h, --help
EOF
}

log() {
  printf 'cleanup-legacy: %s\n' "$*"
}

die() {
  printf 'cleanup-legacy: %s\n' "$*" >&2
  exit 1
}

run() {
  if [ "$APPLY" -eq 1 ]; then
    "$@"
  else
    printf 'DRY-RUN'
    printf ' %q' "$@"
    printf '\n'
  fi
}

remove_file() {
  local path="$1"
  if [ -e "$path" ] || [ -L "$path" ]; then
    run rm -f "$path"
  fi
}

remove_dir() {
  local path="$1"
  if [ -d "$path" ]; then
    run rm -rf "$path"
  fi
}

remove_hosts_block() {
  local file="$1"
  local begin="$2"
  local end="$3"
  local tmp

  [ -f "$file" ] || return 0
  if ! grep -Fq "$begin" "$file"; then
    return 0
  fi

  if [ "$APPLY" -ne 1 ]; then
    log "DRY-RUN remove hosts block from $file: $begin ... $end"
    return 0
  fi

  tmp="$(mktemp "${file}.legacy-cleanup.XXXXXX")"
  awk -v begin="$begin" -v end="$end" '
    $0 == begin { skip = 1; next }
    $0 == end { skip = 0; next }
    !skip { print }
  ' "$file" >"$tmp"
  chmod --reference="$file" "$tmp" 2>/dev/null || chmod 0644 "$tmp"
  chown --reference="$file" "$tmp" 2>/dev/null || true
  mv "$tmp" "$file"
}

remove_route_state() {
  local state_file="$1"
  local ip gateway dev domain

  [ -r "$state_file" ] || return 0

  while read -r ip gateway dev domain _; do
    [ -n "${ip:-}" ] || continue
    [ -n "${gateway:-}" ] || continue
    [ -n "${dev:-}" ] || continue
    if command -v ip >/dev/null 2>&1; then
      run ip route del "${ip}/32" via "$gateway" dev "$dev"
    fi
  done <"$state_file"
}

systemctl_if_present() {
  if command -v systemctl >/dev/null 2>&1; then
    run systemctl "$@"
  fi
}

cleanup_vpn_lock() {
  local nft_table="vpn_lock"

  log "checking vpn-lock artifacts"

  systemctl_if_present disable --now vpn-lock-refresh.timer
  systemctl_if_present stop vpn-lock-refresh.service
  systemctl_if_present disable --now vpn-lock.service

  if [ -x /usr/local/sbin/vpn-lock ]; then
    run /usr/local/sbin/vpn-lock disable legacy-cleanup
  fi

  if command -v nft >/dev/null 2>&1; then
    run nft delete table inet "$nft_table"
  fi

  remove_hosts_block /etc/hosts "# vpn-lock managed block: begin" "# vpn-lock managed block: end"

  remove_file /usr/local/sbin/vpn-lock
  remove_file /usr/local/sbin/uninstall-vpn-lock
  remove_file /etc/systemd/system/vpn-lock.service
  remove_file /etc/systemd/system/vpn-lock-refresh.service
  remove_file /etc/systemd/system/vpn-lock-refresh.timer
  remove_file /etc/systemd/system-sleep/vpn-lock
  remove_file /usr/lib/systemd/system-sleep/vpn-lock
  remove_file /etc/NetworkManager/dispatcher.d/90-vpn-lock
  remove_file /etc/logrotate.d/vpn-lock
  remove_file /usr/lib/tmpfiles.d/vpn-lock.conf

  if [ "$PURGE" -eq 1 ]; then
    remove_dir /etc/vpn-lock
    remove_dir /var/lib/vpn-lock
    remove_dir /run/vpn-lock
    remove_file /var/log/vpn-lock.log
  fi
}

cleanup_vpn_bypass() {
  log "checking vpn-bypass artifacts"

  systemctl_if_present disable --now vpn-bypass-routes.timer
  systemctl_if_present stop vpn-bypass-routes.service

  remove_route_state /run/vpn-bypass/routes.v4

  remove_file /usr/local/bin/vpn-bypass
  remove_file /usr/local/sbin/vpn-bypass
  remove_file /etc/systemd/system/vpn-bypass-routes.service
  remove_file /etc/systemd/system/vpn-bypass-routes.timer
  remove_file /etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes

  if [ "$PURGE" -eq 1 ]; then
    remove_dir /etc/vpn-bypass
    remove_dir /run/vpn-bypass
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --apply)
        APPLY=1
        shift
        ;;
      --purge)
        PURGE=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown option: $1"
        ;;
    esac
  done
}

main() {
  parse_args "$@"

  if [ "$APPLY" -eq 1 ] && [ "$(id -u)" -ne 0 ]; then
    die "run with sudo when using --apply"
  fi

  cleanup_vpn_lock
  cleanup_vpn_bypass

  systemctl_if_present daemon-reload

  if [ "$APPLY" -eq 1 ]; then
    log "legacy cleanup finished"
  else
    log "dry-run finished; pass --apply to make changes"
  fi
}

main "$@"
