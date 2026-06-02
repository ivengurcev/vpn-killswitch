# Stage 4: System Integration

Stage 4 adds installer/uninstaller plumbing and generated system integration files.

## Implemented

- `vpn-killswitch install`
- `vpn-killswitch uninstall`
- `--dry-run`
- `--no-copy`
- `--no-enable`
- `--no-start`
- `--purge`
- `--destdir DIR`

## Generated Assets

- `/usr/local/sbin/vpn-killswitch`
- `/etc/vpn-killswitch/config.toml`
- `/etc/systemd/system/vpn-killswitch.service`
- `/etc/systemd/system/vpn-killswitch-refresh.service`
- `/etc/systemd/system/vpn-killswitch-refresh.timer`
- `/etc/systemd/system-sleep/vpn-killswitch`
- `/etc/NetworkManager/dispatcher.d/90-vpn-killswitch`
- `/usr/lib/tmpfiles.d/vpn-killswitch.conf`
- `/etc/logrotate.d/vpn-killswitch`

## Behavior

- Installer is dry-run friendly.
- `--destdir` stages files under a root directory and skips systemctl actions.
- Installer creates example config only if it does not already exist.
- Installer enables service/timer by default.
- Installer starts initial reconcile by default.
- Uninstaller disables/stops units, removes generated files, disables lock, and removes bypass routes from state.
- Uninstaller keeps config/state/log by default.
- `--purge` removes config/state/log.

## Completed In Stage 5

- `vpn-killswitch install-check`
- `vpn-killswitch config add-bypass`
- `vpn-killswitch config remove-bypass`
- `vpn-killswitch config add-lock`
- `vpn-killswitch config remove-lock`

## Remaining Manual Check

- Full end-to-end installation test on the target machine.
