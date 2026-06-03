#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="$PREFIX/bin"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
AUTOSTART_DIR="$CONFIG_HOME/autostart"
BIN_PATH="$BIN_DIR/vpn-killswitch-tray"
DESKTOP_PATH="$AUTOSTART_DIR/vpn-killswitch-tray.desktop"

usage() {
  cat <<'USAGE'
Использование:
  scripts/install-user.sh [--no-autostart]

Собирает vpn-killswitch-tray и устанавливает для текущего пользователя:
  ~/.local/bin/vpn-killswitch-tray
  ~/.config/autostart/vpn-killswitch-tray.desktop

Переменные окружения:
  PREFIX=/custom/prefix  Переопределить ~/.local
USAGE
}

autostart=1
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-autostart)
      autostart=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Неизвестный аргумент: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

command -v go >/dev/null 2>&1 || {
  echo "Нужен go" >&2
  exit 1
}

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

cd "$ROOT_DIR"
go build -buildvcs=false -o "$tmp_dir/vpn-killswitch-tray" ./cmd/vpn-killswitch-tray

install -d "$BIN_DIR"
install -m 0755 "$tmp_dir/vpn-killswitch-tray" "$BIN_PATH"

if [[ "$autostart" -eq 1 ]]; then
  install -d "$AUTOSTART_DIR"
  cat > "$DESKTOP_PATH" <<DESKTOP
[Desktop Entry]
Type=Application
Name=vpn-killswitch Tray
Comment=Приложение в трее для vpn-killswitch
Exec=$BIN_PATH
Icon=network-vpn
Terminal=false
X-GNOME-Autostart-enabled=true
DESKTOP
  chmod 0644 "$DESKTOP_PATH"
fi

echo "Установлено: $BIN_PATH"
if [[ "$autostart" -eq 1 ]]; then
  echo "Автозапуск: $DESKTOP_PATH"
else
  echo "Автозапуск пропущен"
fi
