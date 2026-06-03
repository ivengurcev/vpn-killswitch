# ТЗ: vpn-killswitch

## 1. Назначение

`vpn-killswitch` — CLI-приложение на Go для управления двумя связанными сетевыми режимами:

1. Маршрутизация выбранных доменов в обход основного VPN через обычный интернет-шлюз.
2. Блокировка выбранных доменов, если заданное VPN-подключение не активно.

Приложение должно объединить логику двух существующих проектов:

- killswitch-логика берётся из `vpn-lock`;
- bypass-логика берётся из `vpn-bypass`.

На первом этапе приложение работает только с VPN-подключениями, управляемыми через NetworkManager.

## 2. Основная идея

После установки приложение отслеживает состояние целевого VPN через уже проверенную event-driven схему: systemd service, NetworkManager dispatcher, sleep hook и timer-refresh. При каждом событии сети, старте системы, выходе из сна, timer-refresh или ручном запуске оно приводит систему к согласованному состоянию:

- bypass-домены маршрутизируются через обычный non-VPN gateway;
- killswitch-домены доступны только когда целевое VPN-подключение активно;
- если статус VPN неизвестен, приложение работает fail-closed и включает блокировку killswitch-доменов.

## 3. Платформа

Целевая платформа:

- Linux;
- NetworkManager;
- systemd;
- nftables;
- Go binary без runtime-зависимостей кроме системных утилит.

На первом этапе другие источники VPN-статуса не поддерживаются: `wg-quick`, standalone OpenVPN, прямой мониторинг `tun`/`wg` интерфейсов и сторонние VPN-клиенты вне NetworkManager остаются за рамками MVP.

На этапе MVP поддерживается одно целевое VPN-подключение. Поддержка нескольких VPN-имён откладывается на следующий этап.

Обязательные внешние команды:

- `ip`;
- `nmcli`;
- `nft`;
- `getent`;
- `systemctl` для установки/диагностики systemd-интеграции.

## 4. Конфигурация

Основной конфиг:

```text
/etc/vpn-killswitch/config.toml
```

Формат конфига: TOML.

Обоснование:

- хорошо отображается на Go-структуры;
- поддерживает комментарии и человекочитаемые списки;
- строже и удобнее для системного конфига, чем shell-like `source`;
- удобнее JSON для ручного редактирования.

Для парсинга TOML нужно использовать простую популярную Go-библиотеку без лишней магии. Конкретный пакет выбирается при реализации, но структура конфига должна напрямую мапиться на Go-структуры.

Минимальные параметры:

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

Требования к конфигу:

- повторная установка не должна терять существующие значения;
- домены должны нормализоваться к lowercase без завершающей точки;
- дубликаты доменов должны удаляться;
- один и тот же домен не может одновременно находиться в bypass-списке и killswitch-списке;
- команды добавления домена должны отказывать, если домен уже есть в любом из двух списков;
- `killswitch.manage_hosts` по умолчанию включён (`true`), как в текущем `vpn-lock`;
- пустые списки допустимы только если явно разрешены логикой команды;
- ошибочный конфиг должен давать понятный exit code и сообщение.

## 5. Команды CLI

Обязательные команды:

```bash
vpn-killswitch enforce [reason]
vpn-killswitch bypass apply
vpn-killswitch lock enable [reason]
vpn-killswitch lock disable [reason]
vpn-killswitch lock refresh [reason]
vpn-killswitch status
vpn-killswitch resolve
vpn-killswitch test
vpn-killswitch install
vpn-killswitch uninstall
vpn-killswitch version
vpn-killswitch help
```

Желательные команды:

```bash
vpn-killswitch config add-bypass DOMAIN
vpn-killswitch config remove-bypass DOMAIN
vpn-killswitch config add-lock DOMAIN
vpn-killswitch config remove-lock DOMAIN
vpn-killswitch install-check
```

Общие флаги:

```bash
--config PATH
--dry-run
--json
--verbose
--quiet
```

## 6. Команда enforce

`vpn-killswitch enforce` — главная команда.

Алгоритм:

1. Загрузить и проверить конфигурацию.
2. Захватить lock-файл, чтобы не было параллельных применений.
3. Определить обычный non-VPN gateway.
4. Применить bypass-маршруты для `bypass.domains`.
5. Определить статус `vpn.connection_name` через NetworkManager.
6. Если VPN активен — снять killswitch-блокировку.
7. Если VPN неактивен или статус неизвестен — включить killswitch-блокировку.
8. Записать состояние и журнал.

Fail-closed правило:

- любая ошибка определения VPN должна считаться неизвестным статусом;
- неизвестный статус должен включать защиту killswitch-доменов.
- если обычный non-VPN gateway для bypass не найден, приложение должно включить killswitch, записать ошибку и завершиться с ненулевым exit code.

Ручные команды:

- `vpn-killswitch lock enable` принудительно включает killswitch без проверки статуса VPN;
- `vpn-killswitch lock disable` принудительно снимает killswitch без проверки статуса VPN;
- проверка VPN-статуса относится к `enforce` и `lock refresh`.

## 7. Bypass-логика

Назначение:

- выбранные домены должны ходить мимо основного VPN;
- маршруты должны указывать на обычный gateway домашней/рабочей сети.

Требования:

- резолвить `bypass.domains` в IPv4 через NSS/getent;
- удалять старые маршруты из state-файла перед применением новых;
- добавлять маршруты вида `ip route replace IP/32 via GATEWAY dev DEV`;
- исключать VPN/виртуальные интерфейсы при выборе gateway: `lo`, `tun*`, `tap*`, `wg*`, `docker*`, `br-*`, `virbr*`, `veth*`;
- хранить state-файл, чтобы можно было удалить ранее добавленные маршруты;
- поддерживать `--dry-run`.

Решение для MVP:

- IPv6 для bypass не входит в MVP. На первом этапе bypass работает только с IPv4, как текущий `vpn-bypass`; IPv6-поддержка планируется отдельным улучшением.

## 8. Killswitch-логика

Назначение:

- killswitch-домены должны блокироваться, если нужный VPN не активен;
- блокировка должна работать по фактическому destination IP, а не только через `/etc/hosts`.

Источник логики: текущий проект `vpn-lock`.

Требования:

- резолвить `killswitch.domains` в IPv4 и IPv6;
- создавать nftables table `inet`;
- блокировать трафик в цепочках `output` и `forward`;
- использовать reject для адресов из сетов;
- фильтровать служебные адреса из nft-сетов:
  - `0.0.0.0`, `::`;
  - loopback;
  - private/RFC1918;
  - link-local;
- при `MANAGE_HOSTS=true` добавлять managed-блок в `/etc/hosts`;
- при снятии защиты удалять nft-таблицу и managed-блок hosts;
- при активном hosts-блоке избегать самоотравления резолва.

Обновление nft:

- в MVP допустимо пересоздавать managed nft-таблицу через единый `nft -f` script;
- script должен быть атомарным насколько это позволяет nftables;
- приложение управляет только собственной таблицей `killswitch.nft_table_name`;
- дельта-обновление элементов сетов можно добавить позже как оптимизацию.

Резолв killswitch-доменов:

- нельзя полагаться только на NSS/getent, если `MANAGE_HOSTS=true`, потому что `/etc/hosts` может вернуть `0.0.0.0` / `::`;
- для наполнения nft-сетов нужно резолвить домены в обход managed-блока `/etc/hosts`;
- MVP должен реализовать DNS-resolve для killswitch через Go DNS-клиент без учёта `/etc/hosts`;
- DNS-серверы берутся из `/etc/resolv.conf`;
- если `/etc/resolv.conf` указывает на локальный stub, например `127.0.0.53`, DNS-серверы нужно получать через `resolvectl dns`;
- служебные адреса всё равно фильтруются как дополнительная защита;
- NSS/getent допустим только для диагностического сравнения или fallback с явным предупреждением.

Совместимость и старые программы:

- миграция старых конфигов `vpn-lock` и `vpn-bypass` выполняется отдельным одноразовым shell-скриптом `scripts/migrate-legacy-config.sh`;
- cleanup старых программ не входит в Go-приложение;
- для удаления старых `vpn-lock`/`vpn-bypass` артефактов используется отдельный одноразовый shell-скрипт `scripts/cleanup-legacy.sh`, который пользователь запускает вручную перед установкой или сразу после проверки нового инструмента;
- новый основной конфиг хранится в TOML и использует `bypass.domains` / `killswitch.domains`.

## 9. Интеграция с системой

Установщик должен поставить:

```text
/usr/local/sbin/vpn-killswitch
/etc/vpn-killswitch/config.toml
/etc/systemd/system/vpn-killswitch.service
/etc/systemd/system/vpn-killswitch-refresh.service
/etc/systemd/system/vpn-killswitch-refresh.timer
/etc/systemd/system-sleep/vpn-killswitch
/etc/NetworkManager/dispatcher.d/90-vpn-killswitch
/usr/lib/tmpfiles.d/vpn-killswitch.conf
/etc/logrotate.d/vpn-killswitch
```

События, которые должны вызывать `vpn-killswitch enforce`:

- старт системы;
- изменение сетевого подключения NetworkManager;
- подключение/отключение VPN;
- выход из сна;
- периодический refresh по timer.

Поведение `vpn-killswitch lock refresh`:

- если целевой VPN активен, команда должна быть no-op и не включать блокировку;
- если VPN неактивен или статус неизвестен, команда обновляет DNS-результаты и nft-сеты для lock-доменов;
- если lock уже активен, обновление должно быть максимально бесшовным.

Поведение `vpn-killswitch resolve`:

- команда показывает результаты резолва для обоих списков;
- секция `bypass` показывает IPv4-адреса, которые будут использоваться для маршрутов;
- секция `lock` показывает IPv4/IPv6-адреса, которые будут использоваться для nft-сетов;
- при `--json` команда отдаёт стабильную структуру с секциями `bypass` и `lock`.

## 10. Диагностика

`vpn-killswitch status` должен показывать:

- имя целевого VPN;
- активен ли VPN;
- выбранный обычный gateway;
- количество bypass-доменов и текущих bypass-маршрутов;
- количество killswitch-доменов;
- активна ли nft-блокировка;
- есть ли managed-блок в hosts;
- путь к конфигу и state-файлам.

Для автоматизации `vpn-killswitch status --json` должен отдавать тот же набор данных в стабильной JSON-структуре.

Минимальная JSON-структура `status --json`:

- `vpn`: имя, типы, статус;
- `gateway`: адрес gateway, interface, ошибка определения если есть;
- `bypass`: количество доменов, количество маршрутов, путь state-файла;
- `lock`: количество доменов, активна ли nft-таблица, есть ли hosts-блок;
- `config`: путь к конфигу;
- `errors`: список ошибок диагностики текущего состояния.

Контракт `status --json` для внешних клиентов, включая `vpn-killswitch-tray`:

```json
{
  "vpn": {
    "name": "Work VPN",
    "types": ["vpn", "wireguard"],
    "status": "active"
  },
  "gateway": {
    "via": "192.168.3.1",
    "dev": "wlp3s0",
    "error": ""
  },
  "bypass": {
    "domain_count": 13,
    "route_count": 27,
    "state_path": "/run/vpn-killswitch/bypass-routes.v4"
  },
  "lock": {
    "domain_count": 6,
    "nft_active": false,
    "hosts_block": false,
    "error": ""
  },
  "config": {
    "path": "/etc/vpn-killswitch/config.toml"
  },
  "errors": []
}
```

Стабильные значения `vpn.status`:

- `active`;
- `inactive`;
- `unknown`.

Внешние клиенты должны считать наличие элементов в `errors` предупреждением/ошибкой состояния и не пытаться самостоятельно исправлять систему в обход CLI.

`vpn-killswitch test` должен проверять:

- наличие зависимостей;
- валидность конфига;
- корректность systemd-установки;
- активность timer/service;
- наличие runtime/state-директорий;
- согласованность состояния с VPN:
  - VPN active: killswitch должен быть снят;
  - VPN inactive/unknown: killswitch должен быть включён.

Для автоматизации `vpn-killswitch test --json` должен отдавать список проверок, их статус, сообщения об ошибках и общий итог.

Минимальная JSON-структура `test --json`:

- `ok`: общий boolean-итог;
- `checks`: список объектов с `name`, `status`, `message`;
- `errors`: список критичных ошибок.

## 11. Безопасность и устойчивость

Требования:

- команды, меняющие систему, требуют root;
- `--dry-run` не требует root там, где это возможно;
- параллельные запуски сериализуются lock-файлом;
- временные файлы записываются атомарно;
- приложение не должно удалять чужие nft-таблицы, hosts-блоки или маршруты;
- при ошибке частичного применения должен быть понятный отчёт;
- при ошибках применения приложение выполняет fail-closed действия по возможности и возвращает ненулевой exit code;
- uninstall должен перед удалением файлов снять nft/hosts lock и удалить bypass-маршруты из state-файла;
- uninstall должен уметь сохранять конфиг/state/log или удалять всё через `--purge`.

## 12. Логирование

Лог:

```text
/var/log/vpn-killswitch.log
```

Требования:

- писать причину запуска `reason`, если передана;
- логировать изменения маршрутов;
- логировать включение/выключение killswitch;
- логировать ошибки резолва;
- поддержать logrotate.

## 13. Exit Codes

Предлагаемые коды:

```text
0  OK
1  general error
2  root required
3  gateway not found
4  config error
5  apply error
6  lock acquisition error
7  install error
8  uninstall error
9  bad arguments
10 diagnostics failed
```

## 14. MVP Defaults

Конфиг:

- путь по умолчанию: `/etc/vpn-killswitch/config.toml`;
- `vpn.connection_name` обязателен;
- `vpn.connection_types`: `["vpn", "wireguard"]`;
- `bypass.domains`: пустой список по умолчанию;
- `bypass.state_path`: `/run/vpn-killswitch/bypass-routes.v4`;
- `killswitch.domains`: пустой список по умолчанию;
- `killswitch.manage_hosts`: `true`;
- `killswitch.hosts_file`: `/etc/hosts`;
- `killswitch.nft_table_name`: `vpn_killswitch_lock`;
- `killswitch.state_dir`: `/var/lib/vpn-killswitch`;
- `killswitch.run_dir`: `/run/vpn-killswitch`;
- минимум один из списков `bypass.domains` или `killswitch.domains` должен быть непустым.

Runtime:

- lock-файл: `/run/vpn-killswitch/lock`;
- лог: `/var/log/vpn-killswitch.log`;
- refresh interval: 5 минут;
- NetworkManager dispatcher debounce: 2 секунды;
- command timeout: 2 минуты;
- DNS timeout для одного запроса: 10 секунд;
- lock acquisition timeout: 10 секунд.

Bypass:

- MVP поддерживает только IPv4;
- gateway выбирается из IPv4 default routes;
- исключаемые интерфейсы: `lo`, `tun*`, `tap*`, `wg*`, `wt*`, `docker*`, `br-*`, `virbr*`, `veth*`;
- state-файл хранит только маршруты, созданные `vpn-killswitch`;
- старые bypass-маршруты из state удаляются перед записью новых.

Lock:

- nft family: `inet`;
- set IPv4: `blocked_ipv4`;
- set IPv6: `blocked_ipv6`;
- chain output: `vpn_killswitch_output`;
- chain forward: `vpn_killswitch_forward`;
- действие блокировки: `reject`;
- приложение управляет только собственной nft-таблицей;
- managed hosts block markers:
  - `# vpn-killswitch managed block: begin`;
  - `# vpn-killswitch managed block: end`.

Installer:

- встроенная команда: `vpn-killswitch install`;
- installer поддерживает `--dry-run`;
- installer по умолчанию копирует текущий бинарь в `/usr/local/sbin/vpn-killswitch`;
- installer по умолчанию включает systemd service/timer;
- installer по умолчанию запускает первичный `vpn-killswitch enforce`;
- дополнительные флаги MVP: `--no-copy`, `--no-enable`, `--no-start`.

Uninstall:

- встроенная команда: `vpn-killswitch uninstall`;
- перед удалением файлов выполняет снятие lock и удаление bypass-маршрутов из state;
- по умолчанию сохраняет config/state/log;
- `--purge` удаляет config/state/log;
- поддерживает `--dry-run`.

## 15. План работ

Этап 1: проектирование

- утвердить CLI;
- утвердить формат конфига;
- утвердить MVP defaults;
- зафиксировать IPv4-only bypass для MVP;
- описать и подготовить отдельный one-time cleanup shell-скрипт для старых `vpn-bypass`/`vpn-lock` установок.

Этап 2: core Go logic

- config parser;
- command runner;
- resolver;
- gateway detector;
- NetworkManager VPN detector;
- bypass route manager;
- nft killswitch manager;
- hosts manager;
- lock-file manager.

Этап 3: CLI

- `enforce`;
- `status`;
- `resolve`;
- `test`;
- ручные bypass/lock команды;
- JSON output.

Этап 4: system integration

- встроенный installer на Go: `vpn-killswitch install`;
- uninstaller;
- systemd service/timer по логике текущего `vpn-lock`;
- NetworkManager dispatcher по логике текущего `vpn-lock`;
- sleep hook по логике текущего `vpn-lock`;
- tmpfiles/logrotate.

Этап 5: тестирование

- unit-тесты парсинга;
- unit-тесты gateway/VPN/status parsing;
- fake runner tests для маршрутов и nft;
- dry-run golden tests;
- ручная проверка на целевой системе.

## 16. Критерии готовности

Проект можно считать готовым к замене старых приложений, когда:

- `vpn-killswitch enforce` полностью заменяет синхронный запуск `vpn-bypass apply` и `vpn-lock enforce`;
- рядом с проектом есть отдельный one-time shell-скрипт `scripts/cleanup-legacy.sh` для cleanup старых `vpn-bypass`/`vpn-lock` артефактов;
- установка создаёт все нужные systemd/NM hooks;
- `vpn-killswitch test` проходит на целевой машине;
- uninstall корректно снимает маршруты, nft-блокировку и hosts-блок до удаления файлов;
- README содержит понятные команды установки, диагностики и удаления.
