# Stage 5: Final MVP Polish And Tests

Stage 5 closes the planned MVP surface from the technical specification.

## Implemented

- `vpn-killswitch install-check`
- `vpn-killswitch config add-bypass DOMAIN`
- `vpn-killswitch config remove-bypass DOMAIN`
- `vpn-killswitch config add-lock DOMAIN`
- `vpn-killswitch config remove-lock DOMAIN`
- config editing with cross-list duplicate protection
- install diagnostics for generated files and systemd state
- one-time legacy config migration script

## Validation

- Unit tests cover config editing.
- Full package test suite passes.
- Binary builds successfully.
- CLI smoke tests pass on a local config.
- Installer dry-run and staged install were checked in Stage 4.

## Remaining For Real Machine Acceptance

- Run one-time legacy cleanup manually when ready.
- Run one-time legacy config migration manually if old domains should be imported.
- Install with real config and root privileges.
- Run `vpn-killswitch install-check`.
- Run `vpn-killswitch test`.
- Verify NetworkManager events trigger `enforce`.
- Verify suspend/resume hook triggers `enforce`.
