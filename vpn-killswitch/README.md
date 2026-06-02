# vpn-killswitch

CLI application for Linux that combines two domain-based VPN helpers:

- **lock**: block selected domains unless the configured NetworkManager VPN is active;
- **bypass**: route selected domains through the normal non-VPN gateway.

The MVP implementation is in place. Core logic, CLI orchestration, installer/systemd asset generation, diagnostics and config helpers exist. See [TZ.md](TZ.md) for the full technical specification.

## MVP Shape

- Linux + NetworkManager + systemd + nftables.
- One target VPN connection for MVP.
- Bypass is IPv4-only for MVP.
- Lock resolves domains for nftables without relying on `/etc/hosts`.
- Config format is TOML.
- Installer is built into the Go binary.
- Legacy `vpn-lock` / `vpn-bypass` cleanup is a separate one-time shell script.

## Config Preview

```toml
[vpn]
connection_name = "VPN connection name"
connection_types = ["vpn", "wireguard"]

[bypass]
domains = ["ozon.ru", "wildberries.ru"]
state_path = "/run/vpn-killswitch/bypass-routes.v4"

[killswitch]
domains = ["example.com", "api.example.com"]
manage_hosts = true
hosts_file = "/etc/hosts"
nft_table_name = "vpn_killswitch_lock"
state_dir = "/var/lib/vpn-killswitch"
run_dir = "/run/vpn-killswitch"
```

## CLI

```bash
vpn-killswitch enforce [reason]
vpn-killswitch bypass apply
vpn-killswitch lock enable [reason]
vpn-killswitch lock disable [reason]
vpn-killswitch lock refresh [reason]
vpn-killswitch status
vpn-killswitch resolve
vpn-killswitch test
vpn-killswitch install-check
vpn-killswitch install
vpn-killswitch uninstall
vpn-killswitch config add-bypass DOMAIN
vpn-killswitch config remove-bypass DOMAIN
vpn-killswitch config add-lock DOMAIN
vpn-killswitch config remove-lock DOMAIN
```

## Legacy Cleanup

Old `vpn-lock` and `vpn-bypass` config can be imported with a separate one-time script:

```bash
./scripts/migrate-legacy-config.sh
sudo ./scripts/migrate-legacy-config.sh --apply
```

The old installations can then be cleaned with:

```bash
./scripts/cleanup-legacy.sh
sudo ./scripts/cleanup-legacy.sh --apply --purge
```

The default mode is dry-run. Use `--apply` to make changes.
