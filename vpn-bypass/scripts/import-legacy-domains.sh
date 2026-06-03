#!/bin/sh
set -eu

LEGACY_FILE="${LEGACY_FILE:-/etc/vpn-bypass-domains.txt}"
VPN_BYPASS="${VPN_BYPASS:-/usr/local/bin/vpn-bypass}"

if [ "${1:-}" = "--dry-run" ]; then
    DRY_RUN="--dry-run"
else
    DRY_RUN=""
fi

if [ ! -r "$LEGACY_FILE" ]; then
    echo "Legacy file is not readable: $LEGACY_FILE" >&2
    exit 1
fi

if [ ! -x "$VPN_BYPASS" ]; then
    echo "vpn-bypass binary is not executable: $VPN_BYPASS" >&2
    exit 1
fi

added=0
skipped=0

while IFS= read -r line || [ -n "$line" ]; do
    domain="${line%%#*}"
    domain="$(printf '%s' "$domain" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"

    if [ -z "$domain" ]; then
        skipped=$((skipped + 1))
        continue
    fi

    "$VPN_BYPASS" add "$domain" $DRY_RUN
    added=$((added + 1))
done < "$LEGACY_FILE"

echo "Processed domains: $added"
echo "Skipped lines: $skipped"
