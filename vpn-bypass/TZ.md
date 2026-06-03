# Техническое задание: vpn-bypass

## 1. Назначение

`vpn-bypass` — локальная Linux-утилита для маршрутизации выбранных доменов мимо OpenVPN.

Основной сценарий:

```text
основной интернет-трафик    → через OpenVPN
выбранные сайты             → через обычный интернет-шлюз
NetBird / служебные VPN     → не трогать
```

Программа должна заменить текущую связку shell-скриптов:

```text
/etc/vpn-bypass-domains.txt
/usr/local/sbin/vpn-bypass-routes.sh
/etc/systemd/system/vpn-bypass-routes.service
/etc/systemd/system/vpn-bypass-routes.timer
/etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes
```

## 2. Целевая платформа

Поддерживаемая платформа MVP:

```text
Linux
systemd
NetworkManager
OpenVPN через NetworkManager
IPv4
```

Проверенная целевая схема:

```text
OpenVPN interface      tun0
NetBird interface      wt0
Wi-Fi interface        wlp3s0
обычный gateway        192.168.3.1
```

Программа не должна быть жёстко привязана к этим именам интерфейсов. Она должна уметь определять обычный шлюз автоматически.

## 3. Язык реализации

Основной язык:

```text
Go
```

Причины выбора:

```text
один статический бинарник
простая установка
удобный запуск из systemd
удобный запуск из NetworkManager dispatcher
подходит для системной CLI-утилиты
можно начать с вызова ip/systemctl/getent
позже можно перейти на netlink без изменения пользовательского интерфейса
```

## 4. Основной принцип работы

Программа читает список доменов из конфигурации, резолвит их в IPv4-адреса и добавляет для этих адресов `/32` маршруты через обычный интернет-шлюз.

Пример:

```text
ozon.ru → 185.73.193.68
```

Должен быть добавлен маршрут:

```bash
ip route replace 185.73.193.68/32 via 192.168.3.1 dev wlp3s0
```

В результате:

```bash
ip -4 route get 185.73.193.68
```

должен показывать:

```text
185.73.193.68 via 192.168.3.1 dev wlp3s0
```

а не:

```text
185.73.193.68 via 10.110.193.1 dev tun0
```

## 5. Что программа должна делать

### 5.1. Управление доменами

Программа должна поддерживать список доменов, которые идут мимо VPN.

Минимальный формат конфигурации:

```text
/etc/vpn-bypass/domains.txt
```

Пример:

```text
ozon.ru
www.ozon.ru
api.ozon.ru
ozone.ru
cdn1.ozone.ru

wildberries.ru
www.wildberries.ru

tbank.ru
www.tbank.ru

gosuslugi.ru
www.gosuslugi.ru
```

Требования:

```text
пустые строки игнорируются
строки с # игнорируются
домены читаются построчно
дубли удаляются
поддомены не добавляются автоматически
```

Пример комментария:

```text
# marketplaces
ozon.ru
www.ozon.ru
```

### 5.2. Резолвинг доменов

MVP:

```text
использовать системный DNS через getent ahostsv4
```

Требования:

```text
резолвить только IPv4
для каждого домена получать все уникальные IPv4
если домен не резолвится — писать warning, но не падать
если ни один домен не дал IP — завершаться с ненулевым кодом
```

Будущее улучшение:

```text
встроенный DNS resolver
поддержка TTL
поддержка IPv6
поддержка dnsmasq/ipset/nftset
```

### 5.3. Определение обычного шлюза

Программа должна найти default route, который не относится к VPN, Docker, bridge и служебным интерфейсам.

Нужно исключать интерфейсы:

```text
lo
tun*
tap*
wg*
wt*
docker*
br-*
virbr*
veth*
```

Пример исходных маршрутов:

```text
default via 10.110.193.1 dev tun0 metric 50
default via 192.168.3.1 dev wlp3s0 metric 600
```

Программа должна выбрать:

```text
192.168.3.1 dev wlp3s0
```

а не:

```text
10.110.193.1 dev tun0
```

MVP-реализация:

```text
парсить вывод ip -4 route show default
```

Будущее улучшение:

```text
использовать netlink
```

### 5.4. Применение маршрутов

Программа должна:

```text
прочитать предыдущие маршруты из state-файла
удалить старые маршруты, которые сама добавляла
получить актуальные IP
добавить новые маршруты
сохранить новое состояние
```

State-файл:

```text
/run/vpn-bypass/routes.v4
```

Формат state-файла MVP:

```text
185.73.193.68 192.168.3.1 wlp3s0 ozon.ru
185.73.194.82 192.168.3.1 wlp3s0 ozon.ru
```

Требования:

```text
удалять только маршруты из собственного state-файла
не удалять чужие маршруты
при ошибке удаления старого маршрута продолжать работу
при ошибке добавления нового маршрута записывать ошибку
итоговый exit code должен быть ненулевым, если часть маршрутов не добавилась
```

### 5.5. Не трогать NetBird

Программа не должна изменять маршруты NetBird.

Пример NetBird-маршрута:

```text
100.103.0.0/16 dev wt0
```

Интерфейсы `wt*` должны считаться VPN/служебными и не использоваться как обычный gateway.

### 5.6. Не менять OpenVPN

Программа не должна:

```text
редактировать .ovpn-файлы
менять NetworkManager VPN connection
менять OpenVPN service
отключать redirect-gateway
менять DNS OpenVPN
```

Она должна работать поверх уже существующей OpenVPN-конфигурации.

## 6. CLI-интерфейс

Имя программы:

```bash
vpn-bypass
```

### 6.1. Основные команды

```bash
vpn-bypass apply
```

Применить маршруты для доменов из конфига.

```bash
vpn-bypass status
```

Показать текущее состояние:

```text
обычный gateway
количество доменов
количество resolved IP
количество активных bypass routes
путь до config
путь до state
состояние systemd timer
наличие NetworkManager hook
```

```bash
vpn-bypass check ozon.ru
```

Проверить, куда сейчас маршрутизируется домен.

Пример хорошего вывода:

```text
Domain: ozon.ru
IP: 185.73.193.68
Route: via 192.168.3.1 dev wlp3s0
Status: bypassed
```

Пример плохого вывода:

```text
Domain: ozon.ru
IP: 185.73.193.68
Route: via 10.110.193.1 dev tun0
Status: vpn
```

```bash
vpn-bypass add ozon.ru
```

Добавить домен в список.

```bash
vpn-bypass remove ozon.ru
```

Удалить домен из списка.

```bash
vpn-bypass list
```

Показать список доменов.

```bash
vpn-bypass install
```

Установить systemd service, timer и NetworkManager hook.

```bash
vpn-bypass uninstall
```

Удалить systemd service, timer и NetworkManager hook.

```bash
vpn-bypass version
```

Показать версию.

### 6.2. Дополнительные флаги

```bash
--config /path/to/domains.txt
```

Путь до файла доменов.

```bash
--state /path/to/state
```

Путь до state-файла.

```bash
--dry-run
```

Показать, какие маршруты были бы добавлены/удалены, но ничего не менять.

```bash
--json
```

Вывести результат в JSON.

```bash
--verbose
```

Подробный вывод.

```bash
--quiet
```

Минимальный вывод.

## 7. Конфигурация

MVP использует простой файл доменов:

```text
/etc/vpn-bypass/domains.txt
```

Возможный будущий формат:

```toml
[network]
exclude_interfaces = [
    "lo",
    "tun*",
    "tap*",
    "wg*",
    "wt*",
    "docker*",
    "br-*",
    "virbr*",
    "veth*"
]

[routes]
refresh_interval = "10m"
ipv6 = false

[domains]
file = "/etc/vpn-bypass/domains.txt"
```

Для MVP TOML не обязателен.

## 8. Установка

Команда:

```bash
sudo vpn-bypass install
```

Должна создать:

```text
/etc/vpn-bypass/domains.txt
/etc/systemd/system/vpn-bypass-routes.service
/etc/systemd/system/vpn-bypass-routes.timer
/etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes
```

И выполнить:

```bash
systemctl daemon-reload
systemctl enable --now vpn-bypass-routes.timer
systemctl start vpn-bypass-routes.service
```

Файл доменов не должен перезаписываться, если он уже существует.

## 9. systemd service

Файл:

```text
/etc/systemd/system/vpn-bypass-routes.service
```

Содержимое:

```ini
[Unit]
Description=Update routes for domains bypassing OpenVPN
Wants=network-online.target
After=network-online.target NetworkManager.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/vpn-bypass apply
```

## 10. systemd timer

Файл:

```text
/etc/systemd/system/vpn-bypass-routes.timer
```

Содержимое:

```ini
[Unit]
Description=Periodically update routes for domains bypassing OpenVPN

[Timer]
OnBootSec=30
OnUnitActiveSec=10min
Unit=vpn-bypass-routes.service

[Install]
WantedBy=timers.target
```

## 11. NetworkManager dispatcher hook

Файл:

```text
/etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes
```

Содержимое:

```sh
#!/bin/sh

ACTION="$2"

case "$ACTION" in
    vpn-up|vpn-down)
        /usr/bin/systemctl start vpn-bypass-routes.service
        ;;
esac

exit 0
```

Права:

```bash
chmod +x /etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes
```

## 12. Удаление

Команда:

```bash
sudo vpn-bypass uninstall
```

Должна:

```text
остановить timer
отключить timer
удалить systemd service
удалить systemd timer
удалить NetworkManager dispatcher hook
выполнить systemctl daemon-reload
```

Команды:

```bash
systemctl disable --now vpn-bypass-routes.timer
rm -f /etc/systemd/system/vpn-bypass-routes.service
rm -f /etc/systemd/system/vpn-bypass-routes.timer
rm -f /etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes
systemctl daemon-reload
```

Файл доменов по умолчанию не удалять:

```text
/etc/vpn-bypass/domains.txt
```

Для полного удаления можно добавить:

```bash
sudo vpn-bypass uninstall --purge
```

Тогда дополнительно удалять:

```text
/etc/vpn-bypass/
```

## 13. Права

Команды, которые меняют маршруты или systemd-файлы, требуют root.

Должны требовать root:

```text
apply
install
uninstall
add
remove
```

Могут работать без root:

```text
list
status
check
version
```

Но `status` без root может показывать неполную информацию, если не хватает прав.

## 14. Логирование

MVP:

```text
stdout/stderr
```

Так как запуск идёт через systemd, вывод попадёт в journald.

Просмотр логов:

```bash
journalctl -u vpn-bypass-routes.service -n 50 --no-pager
```

Уровни сообщений:

```text
INFO      обычные действия
WARN      домен не зарезолвился, маршрут не найден
ERROR     критическая ошибка
```

Пример:

```text
INFO  normal gateway: 192.168.3.1 dev wlp3s0
INFO  route add: ozon.ru 185.73.193.68 via 192.168.3.1 dev wlp3s0
WARN  resolve failed: example.invalid
ERROR no normal gateway found
```

## 15. Exit codes

```text
0    успешно
1    общая ошибка
2    нет root-прав
3    не найден обычный gateway
4    ошибка чтения config
5    ошибка применения части маршрутов
```

## 16. Проверочные сценарии

### 16.1. Базовое применение

Условие:

```text
OpenVPN подключён
есть default route через tun0
есть обычный default route через wlp3s0
```

Команда:

```bash
sudo vpn-bypass apply
```

Ожидаемый результат:

```text
маршруты для доменов добавлены через wlp3s0
exit code 0
```

### 16.2. Проверка домена

Команда:

```bash
vpn-bypass check ozon.ru
```

Ожидаемый результат:

```text
Status: bypassed
```

### 16.3. Переподключение VPN

Команды:

```bash
nmcli connection down "work-germany"
sleep 5
nmcli connection up "work-germany"
sleep 5
vpn-bypass check ozon.ru
```

Ожидаемый результат:

```text
домен идёт через обычный gateway
```

### 16.4. Изменение domains.txt

Действия:

```text
добавить новый домен в /etc/vpn-bypass/domains.txt
подождать до 10 минут
```

Ожидаемый результат:

```text
timer применил новый маршрут
```

### 16.5. Домен не резолвится

Конфиг:

```text
invalid-domain.example
```

Ожидаемый результат:

```text
warning в логах
программа продолжает обработку остальных доменов
```

### 16.6. Нет обычного gateway

Условие:

```text
есть только default route через tun0
обычного Wi-Fi/Ethernet gateway нет
```

Ожидаемый результат:

```text
ошибка
маршруты не добавляются
exit code 3
```

## 17. Ограничения MVP

```text
только Linux
только IPv4
только маршрутизация по IP
домены резолвятся периодически, не по DNS-событиям
поддомены не добавляются автоматически
CDN могут отдавать новые IP между запусками timer
NetworkManager hook рассчитан на VPN-подключения через NetworkManager
```

## 18. Не входит в MVP

```text
GUI
TUI
автоматический wildcard для *.domain.ru
IPv6
nftables
ipset
dnsmasq-интеграция
собственный DNS proxy
поддержка Windows/macOS
поддержка OpenWrt
```

## 19. Возможные улучшения после MVP

```text
перейти с вызова ip на netlink
добавить config.toml
добавить IPv6
добавить режим watch
добавить nftables/ipset backend
добавить DNS backend через dnsmasq
добавить автоматическое определение CDN-поддоменов
добавить импорт доменов из файла
добавить export/import конфигурации
добавить deb-пакет
добавить shell completion
```

## 20. Приёмка MVP

MVP считается готовым, если выполнены условия:

```text
программа собирается в один бинарник vpn-bypass
команда sudo vpn-bypass install создаёт нужные systemd/NM файлы
команда sudo vpn-bypass apply добавляет маршруты мимо VPN
команда vpn-bypass check ozon.ru показывает bypassed
после переподключения VPN маршруты восстанавливаются
после изменения domains.txt timer применяет изменения максимум через 10 минут
NetBird-маршруты не ломаются
uninstall удаляет systemd/NM-интеграцию
```

## 21. Предлагаемая структура проекта

```text
vpn-bypass/
    cmd/
        vpn-bypass/
            main.go

    internal/
        cli/
            cli.go

        config/
            domains.go

        resolver/
            getent.go

        gateway/
            detect.go

        routes/
            apply.go
            state.go
            check.go

        systemd/
            install.go
            uninstall.go

        networkmanager/
            dispatcher.go

        log/
            log.go

    scripts/
        install.sh

    README.md
    SPEC.md
    go.mod
```

## 22. Минимальный набор команд MVP

Обязательные:

```bash
vpn-bypass apply
vpn-bypass check <domain>
vpn-bypass list
vpn-bypass install
vpn-bypass uninstall
vpn-bypass status
vpn-bypass version
```

Желательные:

```bash
vpn-bypass add <domain>
vpn-bypass remove <domain>
```

## 23. Критичные требования безопасности

```text
не удалять маршруты, которых нет в state-файле программы
удалять старые маршруты только строго по ip + gateway + dev из state-файла
не перезаписывать существующий domains.txt при install
не менять OpenVPN-конфигурацию
не менять NetBird-конфигурацию
не использовать wt*, tun*, wg* как обычный gateway
не выполнять shell-команды через неэкранированные строки
не подставлять домены напрямую в shell
```

## 24. Уточнения перед реализацией

### 24.1. Удаление старых маршрутов из state

При удалении старых маршрутов программа должна удалять маршрут строго с теми параметрами, которые сама ранее сохранила в state-файле.

Правильно:

```bash
ip route del <ip>/32 via <old_gateway> dev <old_dev>
```

Неправильно:

```bash
ip route del <ip>/32
```

Причина:

```text
короткая команда может удалить чужой маршрут,
если такой же /32 был добавлен вручную или другой программой.
```

State-файл должен хранить минимум:

```text
ip gateway dev domain
```

Пример:

```text
185.73.193.68 192.168.3.1 wlp3s0 ozon.ru
```

При удалении использовать именно:

```bash
ip route del 185.73.193.68/32 via 192.168.3.1 dev wlp3s0
```

Если удаление не удалось, программа должна вывести warning и продолжить работу.

### 24.2. Поведение check <domain>

Если домен резолвится в несколько IPv4, команда `check` должна показывать все IP.

Пример:

```bash
vpn-bypass check ozon.ru
```

Пример вывода:

```text
Domain: ozon.ru

185.73.193.68
    Route: via 192.168.3.1 dev wlp3s0
    Status: bypassed

185.73.194.82
    Route: via 192.168.3.1 dev wlp3s0
    Status: bypassed
```

Если часть IP идёт мимо VPN, а часть через VPN, статус должен быть смешанным:

```text
Summary: partial
```

Возможные итоговые статусы:

```text
bypassed    все IP идут через обычный gateway
vpn         все IP идут через VPN
partial     часть IP идёт мимо VPN, часть нет
unresolved  домен не удалось зарезолвить
error       ошибка проверки маршрута
```

### 24.3. Поведение status

Команда `status` должна быть read-only.

Она не должна:

```text
резолвить домены заново
добавлять маршруты
удалять маршруты
изменять state
запускать apply
```

Она должна читать только:

```text
config-файл
state-файл
текущую таблицу маршрутов
наличие systemd service
наличие systemd timer
наличие NetworkManager dispatcher hook
состояние timer
обычный gateway
```

Пример:

```bash
vpn-bypass status
```

Пример вывода:

```text
Config: /etc/vpn-bypass/domains.txt
State: /run/vpn-bypass/routes.v4

Domains in config: 12
Routes in state: 18
Active bypass routes: 18

Normal gateway: 192.168.3.1 dev wlp3s0

Systemd service: installed
Systemd timer: active
NetworkManager hook: installed
```

Если нужна диагностика с новым DNS-резолвом, для этого должна быть отдельная команда:

```bash
vpn-bypass check <domain>
```

или будущая команда:

```bash
vpn-bypass diagnose
```

### 24.4. Поведение add/remove

Команды:

```bash
vpn-bypass add <domain>
vpn-bypass remove <domain>
```

в MVP должны только изменять файл:

```text
/etc/vpn-bypass/domains.txt
```

Они не должны автоматически запускать `apply`.

Причина:

```text
поведение команды должно быть предсказуемым,
изменение config и применение маршрутов лучше разделить.
```

После успешного `add` или `remove` программа должна выводить подсказку:

```text
Domain added: ozon.ru

Routes are not applied automatically.
Run:

    sudo vpn-bypass apply
```

Или:

```text
Changes will be applied automatically by systemd timer.
```

Команды `add/remove` требуют root, потому что пишут в `/etc`.

### 24.5. install и путь к бинарнику

Команда:

```bash
sudo vpn-bypass install
```

должна установить сам бинарник в:

```text
/usr/local/bin/vpn-bypass
```

Правила:

```text
если текущий бинарник уже /usr/local/bin/vpn-bypass — ничего не копировать
если текущий бинарник запущен из другого места — скопировать его в /usr/local/bin/vpn-bypass
если /usr/local/bin/vpn-bypass уже существует — заменить атомарно
права бинарника выставить 0755
владелец root:root, если возможно
```

Service должен всегда ссылаться на стабильный путь:

```ini
ExecStart=/usr/local/bin/vpn-bypass apply
```

Установка должна делать примерно следующее:

```text
определить путь текущего исполняемого файла
скопировать его во временный файл рядом с target
выставить chmod 0755
атомарно переименовать в /usr/local/bin/vpn-bypass
создать systemd service
создать systemd timer
создать NetworkManager dispatcher hook
systemctl daemon-reload
systemctl enable --now vpn-bypass-routes.timer
systemctl start vpn-bypass-routes.service
```

Нужно предусмотреть флаг:

```bash
vpn-bypass install --no-copy
```

Он нужен для разработки, если бинарник уже установлен вручную или запускается из нестандартного места.

### 24.6. Конкурентный запуск

Нужно добавить lock.

Файл lock:

```text
/run/vpn-bypass/lock
```

Команды, которые должны брать exclusive lock:

```text
apply
install
uninstall
add
remove
```

Причина:

```text
timer может запустить apply
NetworkManager hook может запустить apply
пользователь может вручную запустить sudo vpn-bypass apply
без lock возможна гонка при удалении/добавлении маршрутов и записи state
```

Поведение MVP:

```text
если lock свободен — команда выполняется
если lock занят — ждать до 30 секунд
если через 30 секунд lock не получен — завершиться с ошибкой
```

Новый exit code:

```text
6    lock timeout / another instance is running
```

Пример ошибки:

```text
ERROR another vpn-bypass instance is running
```

Технически в Go можно использовать `flock` через syscall.

Важно:

```text
systemd защищает от параллельного запуска одного и того же unit,
но не защищает от ручного запуска vpn-bypass apply.
Поэтому lock нужен внутри программы.
```

### 24.7. dry-run

В MVP флаг `--dry-run` должен поддерживаться только для команд, которые могут изменять систему:

```text
apply
install
uninstall
add
remove
```

Поведение:

```bash
vpn-bypass apply --dry-run
```

Показывает:

```text
какие старые маршруты были бы удалены
какие новые маршруты были бы добавлены
какой state-файл был бы записан
```

Но не меняет:

```text
route table
state-файл
config
systemd
NetworkManager hook
```

```bash
vpn-bypass install --dry-run
```

Показывает:

```text
куда был бы скопирован бинарник
какие systemd-файлы были бы созданы
какой NetworkManager hook был бы создан
какие systemctl-команды были бы выполнены
```

Но ничего не создаёт.

```bash
vpn-bypass uninstall --dry-run
```

Показывает, что было бы удалено.

```bash
vpn-bypass add ozon.ru --dry-run
```

Показывает, что домен был бы добавлен, но файл не меняет.

```bash
vpn-bypass remove ozon.ru --dry-run
```

Показывает, что домен был бы удалён, но файл не меняет.

Команды read-only не нуждаются в `--dry-run`:

```text
status
check
list
version
```

Для них `--dry-run` в MVP считается ошибкой, чтобы не было неоднозначности.

### 24.8. Обновлённые exit codes

```text
0    успешно
1    общая ошибка
2    нет root-прав
3    не найден обычный gateway
4    ошибка чтения config
5    ошибка применения части маршрутов
6    lock timeout / another instance is running
7    ошибка установки
8    ошибка удаления
9    некорректные аргументы CLI
```

### 24.9. Уточнённая приёмка MVP

MVP считается готовым, если:

```text
apply удаляет старые маршруты строго по ip + gateway + dev
apply не удаляет чужие маршруты
check показывает все IPv4 домена
status не резолвит домены и не меняет систему
add/remove только меняют config и не запускают apply
install копирует бинарник в /usr/local/bin/vpn-bypass
systemd service использует /usr/local/bin/vpn-bypass apply
есть lock от параллельных запусков
--dry-run работает для apply/install/uninstall/add/remove
после переподключения VPN маршруты восстанавливаются
после изменения domains.txt timer применяет изменения максимум через 10 минут
NetBird-маршруты не ломаются
```

## 25. Итог

`vpn-bypass` должен стать аккуратной системной CLI-утилитой, которая заменяет набор shell-скриптов и даёт управляемый способ отправлять выбранные домены мимо OpenVPN, не ломая NetBird и остальную маршрутизацию.
