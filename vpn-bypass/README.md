# vpn-bypass

Local Linux CLI utility for routing selected domains outside OpenVPN while leaving other VPNs, including NetBird, untouched.

## Build

```bash
go build -buildvcs=false -o vpn-bypass ./cmd/vpn-bypass
```

`-buildvcs=false` is useful while this directory is not a git repository.

## Examples

```bash
vpn-bypass version
vpn-bypass list
vpn-bypass status
vpn-bypass check ozon.ru
```

Commands that modify the system require root:

```bash
sudo vpn-bypass apply
sudo vpn-bypass install
sudo vpn-bypass uninstall
sudo vpn-bypass add ozon.ru
sudo vpn-bypass remove ozon.ru
```

Safe previews:

```bash
vpn-bypass apply --dry-run --config /path/to/domains.txt --state /tmp/routes.v4
vpn-bypass install --dry-run --no-copy
vpn-bypass uninstall --dry-run
vpn-bypass add ozon.ru --dry-run
vpn-bypass remove ozon.ru --dry-run
```
