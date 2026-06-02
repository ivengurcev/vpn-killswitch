# Stage 1: Project Bootstrap

Stage 1 locks the MVP decisions before Go implementation starts.

## Done

- CLI shape is defined in `TZ.md`.
- TOML config format is selected.
- MVP defaults are defined.
- Bypass is IPv4-only for MVP.
- One target NetworkManager VPN connection is supported for MVP.
- Legacy config migration is out of scope.
- Legacy cleanup is a separate one-time shell script.

## Created Artifacts

- `TZ.md`: full technical specification.
- `README.md`: short project entry point.
- `config.example.toml`: MVP config example.
- `scripts/cleanup-legacy.sh`: dry-run-first cleanup script for old `vpn-lock` / `vpn-bypass` artifacts.

## Next Stage

Stage 2 starts core Go logic:

- config parser;
- command runner;
- resolver;
- gateway detector;
- NetworkManager VPN detector;
- bypass route manager;
- nft lock manager;
- hosts manager;
- lock-file manager.
