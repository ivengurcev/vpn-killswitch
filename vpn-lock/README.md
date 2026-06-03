# vpn-lock

Набор скриптов для блокировки заданных доменов и их текущих IP-адресов, если указанное VPN-подключение NetworkManager не активно.

## Установка

```bash
sudo ./setup-vpn-lock.sh --vpn "VPN connection name" --domain example.com --domain api.example.com
```

Повторный запуск инсталлятора безопасен: файлы переустанавливаются, конфигурация пересобирается из переданных параметров или сохраняет существующие значения.

```bash
sudo ./setup-vpn-lock.sh --vpn "New VPN name" --domains "example.com,api.example.com"
```

Дополнительные флаги установщика:

- `--manage-hosts` / `--no-manage-hosts` — включить или отключить запись 0.0.0.0/:: для защищаемых доменов в `/etc/hosts`. При повторной установке без флага значение из существующего конфига сохраняется.
- `--dry-run` — напечатать план установки (список файлов, итоговый конфиг, команды package-manager и systemd) и завершиться, не меняя состояние системы. Не требует root.
- `--no-install-deps`, `--no-enable`, `--no-start` — не ставить пакеты, не включать или не запускать systemd-юниты.
- `--destdir DIR` — сложить файлы в staged-каталог вместо установки в `/`.

## Что устанавливается

- `/usr/local/sbin/vpn-lock` — основной скрипт.
- `/usr/local/sbin/uninstall-vpn-lock` — установленная копия деинсталлятора.
- `/etc/vpn-lock/vpn-lock.conf` — конфигурация.
- `/etc/systemd/system/vpn-lock.service` — проверка при старте системы.
- `/etc/systemd/system/vpn-lock-refresh.service` + `vpn-lock-refresh.timer` — периодический рефреш IP при неактивном VPN (по умолчанию раз в 5 минут).
- `/etc/systemd/system-sleep/vpn-lock` — проверка после выхода из сна.
- `/etc/NetworkManager/dispatcher.d/90-vpn-lock` — реакция на изменения сети и VPN с дебаунсом.
- `/usr/lib/tmpfiles.d/vpn-lock.conf` — гарантирует существование `/run/vpn-lock` после загрузки.
- `/etc/logrotate.d/vpn-lock` — ротация журнала работы.

## Команды

```bash
sudo vpn-lock enforce [reason]   # определить статус VPN и применить нужный режим защиты
sudo vpn-lock enable [reason]    # принудительно включить защиту
sudo vpn-lock disable [reason]   # принудительно снять защиту
sudo vpn-lock refresh [reason]   # обновить nftables-сеты (no-op если VPN активен)
sudo vpn-lock status             # короткий отчёт о состоянии
sudo vpn-lock resolve            # текущий список IP, в которые резолвятся защищаемые домены
sudo vpn-lock test               # диагностика: проверить, что состояние согласовано с VPN
sudo vpn-lock install-check      # проверить корректность конфигурации и наличие зависимостей
```

При активном VPN защита снимается. При неактивном или неизвестном статусе VPN включается fail-closed режим: managed-блок в `/etc/hosts` (если `MANAGE_HOSTS=true`) и `nftables`-правила для A/AAAA адресов защищаемых доменов.

## Что блокируется

В режиме защиты создаётся таблица `inet vpn_lock` со следующими сетами и цепочками:

- `set blocked_ipv4` — IPv4-адреса защищаемых доменов.
- `set blocked_ipv6` — IPv6-адреса защищаемых доменов.
- `chain vpn_lock_output` (hook output) и `chain vpn_lock_forward` (hook forward) — оба с правилом `reject` для пакетов с `daddr` в соответствующем сете.

Так как правило применяется в `inet`-таблице по адресу назначения, **блокируется любой исходящий и форвардящийся трафик** к этим IP, независимо от транспортного протокола: TCP, UDP (в том числе DNS-over-HTTPS/DoT, QUIC), ICMP/ICMPv6 и т.д. Цепочка `forward` покрывает трафик контейнеров и виртуальных машин, маршрутизируемый через хост.

Если включён `MANAGE_HOSTS=true`, дополнительно в `/etc/hosts` пишется managed-блок с записями `0.0.0.0 domain` и `:: domain`. Это срезает имена через NSS до сетевого слоя для приложений, использующих стандартный резолвер (см. также «Известные ограничения резолва»).

Список адресов, которые **не попадают** в сеты (фильтруются с записью `WARN` в журнал):

- `0.0.0.0`, `::`
- RFC1918: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
- link-local: `169.254.0.0/16`, `fe80::/10`
- loopback: `127.0.0.0/8`, `::1`

Это защищает от самоотравления резолва, когда `/etc/hosts` ещё содержит блок и `getent` возвращает `0.0.0.0`.

## Диагностика

```bash
sudo vpn-lock status        # короткий отчёт: статус VPN, активность защиты, MANAGE_HOSTS
sudo vpn-lock test          # OK/FAIL по каждому ожидаемому состоянию
sudo tail -f /var/log/vpn-lock.log
```

`vpn-lock test` проверяет в одном запуске:

1. **Зависимости**: `nmcli`, `nft`, `getent`, `awk`, `mktemp`, `stat`, `flock` доступны в `PATH`.
2. **Целостность установки**: все ожидаемые файлы на месте — `/usr/local/sbin/vpn-lock`, `/etc/vpn-lock/vpn-lock.conf`, оба systemd-юнита (`vpn-lock.service`, `vpn-lock-refresh.{service,timer}`), `system-sleep`-хук, NM-диспетчер, `tmpfiles.d`-конфиг, `logrotate.d`-конфиг.
3. **Каталоги**: `STATE_DIR` (`/var/lib/vpn-lock`) и `RUN_DIR` (`/run/vpn-lock`) существуют — последний пересоздаётся `tmpfiles.d` при загрузке.
4. **Systemd-юниты**: `vpn-lock.service` и `vpn-lock-refresh.timer` `enabled`, таймер `active`.
5. **Конфигурация**: `install-check` (валидность параметров).
6. **Согласованность защиты**:
   - при неактивном или неизвестном VPN — наличие nft-таблицы, непустых сетов и (при `MANAGE_HOSTS=true`) managed-блока в `/etc/hosts`;
   - при активном VPN — отсутствие и nft-таблицы, и managed-блока.

Команда возвращает ненулевой код при наличии хотя бы одного `FAIL`. Запускать имеет смысл от root — иначе `nft list` и `is-active` могут вернуть неполные данные (об этом `WARN` в начале вывода).

Полезные ручные проверки:

```bash
sudo nft list table inet vpn_lock          # текущие сеты и правила
sudo nft list set inet vpn_lock blocked_ipv4
grep '# vpn-lock managed block' /etc/hosts # есть managed-блок?
systemctl status vpn-lock-refresh.timer    # таймер периодического рефреша
systemctl list-timers vpn-lock-refresh.timer
journalctl -u vpn-lock.service -n 50
```

## Ограничения

- **Только NetworkManager.** Программа определяет статус VPN через `nmcli -t -f NAME,TYPE connection show --active` и реагирует через `dispatcher.d`. Альтернативные детекторы (прямой `wg-quick`, `openvpn` без NM, мониторинг tun-интерфейса по имени) не поддерживаются. Для VPN, поднимаемых вне NetworkManager, потребуется внешний скрипт, дёргающий `vpn-lock enforce`.
- **VPN-типы.** По умолчанию VPN считается активным только если тип соединения NetworkManager — `vpn` или `wireguard`. Расширяется через `VPN_CONNECTION_TYPES` в конфиге (например, `"vpn wireguard openvpn"`).
- **Контейнеры и VM.** Цепочка `forward` блокирует трафик через bridge-сети (Docker, libvirt). Для трафика, не проходящего через хост (например, macvlan, отдельные сетевые namespace), правила не применяются.
- **Привязка по IP, а не по SNI.** Если защищаемый домен делит IP с другими сервисами (CDN, shared hosting), они тоже окажутся заблокированы. Если защищаемый домен использует адресный пул, не попавший в кэш, до следующего рефреша/события трафик к новым IP не блокируется.

## Известные ограничения резолва

Программа резолвит защищаемые домены через `getent ahostsv4` / `ahostsv6`, который проходит через NSS-цепочку `nsswitch.conf` — обычно `files dns`. Это значит:

- При `MANAGE_HOSTS=true` блок в `/etc/hosts` влияет на сам процесс резолва. Чтобы избежать самоотравления, домены резолвятся **до** записи блока, а служебные адреса (`0.0.0.0`, `::`, RFC1918, loopback, link-local) фильтруются из nft-сетов.
- Если в системе настроен локальный кэширующий резолвер (`systemd-resolved`, `dnsmasq`), результаты могут отличаться от того, что получит приложение, использующее свою DNS-логику (Chrome, Firefox с DoH, Go-программы). Защита по `nftables` срабатывает по фактическому destination IP пакета и не зависит от источника DNS-ответа, поэтому даже при «обходе» NSS пакет к правильному IP всё равно блокируется. Однако новый IP, известный приложению, но не попавший в кэш `vpn-lock`, не будет заблокирован до следующего рефреша.

Принудительный обход `/etc/hosts` через `dig`/`systemd-resolve` пока не реализован (см. п. 4.5 в `TECH_SPEC_IMPROVEMENTS.md`).

## Удаление

```bash
sudo ./uninstall-vpn-lock.sh
```

По умолчанию скрипт снимает активную защиту, отключает `vpn-lock.service` и `vpn-lock-refresh.timer`, удаляет установленные файлы и сохраняет конфигурацию, состояние и лог. Для полного удаления:

```bash
sudo ./uninstall-vpn-lock.sh --purge
```
