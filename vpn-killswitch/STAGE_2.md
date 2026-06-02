# Stage 2: Core Go Logic

Stage 2 starts the implementation without CLI orchestration or installer logic.

## Implemented

- Go module: `vpn-killswitch`.
- TOML config parser with defaults and validation.
- Command runner with timeout.
- Console logger.
- Lock-file helper.
- Normal non-VPN IPv4 gateway detector.
- NetworkManager VPN status parser/detector.
- Bypass route state and route apply core.
- Bypass NSS/getent IPv4 resolver.
- Lock DNS resolver that bypasses `/etc/hosts`.
- nftables script builder for the managed `inet` table.
- `/etc/hosts` managed block helper.
- Lock manager that connects DNS, nft and hosts helpers.

## Tests

Covered so far:

- TOML defaults and validation;
- domain normalization and cross-list conflict detection;
- gateway parsing and VPN interface exclusion;
- NetworkManager escaped field parsing;
- bypass route construction;
- DNS server parsing from `/etc/resolv.conf` and `resolvectl dns`;
- nft script generation;
- hosts managed block add/remove;
- lock DNS result deduplication.

## Not Yet Implemented

- Full CLI command tree.
- `enforce` orchestration.
- `status`, `resolve`, `test` user-facing output.
- Installer/uninstaller.
- systemd/NetworkManager/sleep hook generation.
- JSON output schemas.
