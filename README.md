# Local Apps

Набор локальных Linux-утилит для управления VPN-сценариями.

Основной актуальный проект сейчас — [`vpn-killswitch`](./vpn-killswitch/README.md). Он объединяет идеи старых проектов:

- [`vpn-bypass`](./vpn-bypass/README.md) — маршрутизация выбранных доменов мимо VPN через обычный gateway.
- [`vpn-lock`](./vpn-lock/README.md) — блокировка выбранных доменов, если нужный VPN не активен.
- [`vpn-killswitch-tray`](./vpn-killswitch-tray/README.md) — планируемый tray companion для статуса, уведомлений и простого управления доменами.

## Проекты

### vpn-killswitch

Go CLI-приложение для Linux + NetworkManager + systemd + nftables.

Что умеет:

- `bypass`: маршрутизировать отдельные домены через обычный non-VPN gateway;
- `lock`: блокировать отдельные домены, если целевой NetworkManager VPN не активен;
- `enforce`: приводить оба режима к согласованному состоянию;
- `install` / `uninstall`: ставить systemd service/timer, NetworkManager dispatcher, sleep hook, tmpfiles и logrotate;
- `status`, `resolve`, `test`, `install-check`: диагностика;
- `config add-*` / `config remove-*`: управление списками доменов.

Быстрая сборка:

```bash
cd vpn-killswitch
go build -buildvcs=false -o vpn-killswitch ./cmd/vpn-killswitch
```

Безопасные проверки:

```bash
./vpn-killswitch help
./vpn-killswitch status --config ./config.example.toml --json
./vpn-killswitch install --dry-run --no-copy --no-enable --no-start
./vpn-killswitch uninstall --dry-run --purge
```

### vpn-bypass

Старое отдельное Go-приложение для bypass-маршрутов. Его логика перенесена в `vpn-killswitch`.

### vpn-lock

Старое shell/systemd/nftables-решение для fail-closed блокировки доменов. Его логика перенесена в `vpn-killswitch`.

### vpn-killswitch-tray

Планируемое пользовательское приложение для системного трея. Оно не заменяет `vpn-killswitch`, а вызывает установленный CLI, показывает состояние, отправляет уведомления и даёт простой способ добавлять/удалять домены.

## Миграция

Перед переходом на `vpn-killswitch` можно импортировать старые домены:

```bash
cd vpn-killswitch
./scripts/migrate-legacy-config.sh
sudo ./scripts/migrate-legacy-config.sh --apply
```

После проверки нового конфига старые установки можно очистить:

```bash
./scripts/cleanup-legacy.sh
sudo ./scripts/cleanup-legacy.sh --apply --purge
```

Оба скрипта по умолчанию работают в dry-run режиме и ничего не меняют без `--apply`.

## Текущий MVP

Ограничения MVP:

- только NetworkManager;
- одно целевое VPN-подключение;
- bypass только IPv4;
- legacy cleanup и config migration вынесены в отдельные shell-скрипты.

Отложено на потом:

- несколько VPN-имён;
- IPv6 bypass;
- VPN backends вне NetworkManager;
- дельта-обновление nft-сетов вместо пересоздания managed table.
