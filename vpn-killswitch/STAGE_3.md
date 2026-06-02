# Stage 3: CLI Orchestration

Stage 3 wires the core packages into a runnable CLI. Installer/systemd generation remains for the next stage.

## Implemented

- `vpn-killswitch enforce [reason]`
- `vpn-killswitch bypass apply`
- `vpn-killswitch lock enable [reason]`
- `vpn-killswitch lock disable [reason]`
- `vpn-killswitch lock refresh [reason]`
- `vpn-killswitch status`
- `vpn-killswitch resolve`
- `vpn-killswitch test`
- `vpn-killswitch version`
- `vpn-killswitch help`

## Flags

- `--config PATH`
- `--dry-run`
- `--json`
- `--verbose`
- `--quiet`

## Behavior

- Mutating commands require root unless `--dry-run` is used.
- Mutating commands acquire `/run/vpn-killswitch/lock`.
- `enforce` applies bypass first, then checks NetworkManager VPN state.
- If bypass fails, `enforce` tries to enable lock fail-closed.
- `lock enable` and `lock disable` are force commands.
- `lock refresh` is no-op when the target VPN is active.
- `status`, `resolve` and `test` support JSON output.

## Not Yet Implemented In Stage 3

- `vpn-killswitch install-check`
- `vpn-killswitch config add-*` helpers
