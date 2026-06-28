# The Dark Twenties

Telegram-first VPN-сервис с control plane на Go, управляемыми VPN-нодами,
подписками, лимитами устройств, smart routing для пользователей из РФ и
протокольно-независимым слоем туннелей.

Проект изначально проектируется как коммерческая VPN-платформа, а не как один
VPN-сервер с конфигами, которые вручную выдаются пользователям.

## Формат продукта

У сервиса три пользовательские поверхности:

- Telegram Bot: onboarding, trial, оплаты, конфиги, поддержка.
- Telegram Mini App: профиль, подписки, устройства, статистика, рефералы, инструкции.
- Admin Panel: пользователи, подписки, устройства, ноды, трафик, биллинг, поддержка.

Ключевой приоритет - VPN-инфраструктура:

- высокая скорость;
- стабильные соединения;
- минимальные потери пакетов;
- устойчивость к блокировкам;
- smart routing для российских сервисов;
- масштабируемое управление нодами.

## Базовая архитектура

Система разделяется на две плоскости.

```text
Control Plane = Go backend
Data Plane    = WireGuard / Xray / sing-box / Hysteria nodes
```

### Control Plane

Go backend не проксирует пользовательский трафик. Он управляет доступом.

Зоны ответственности:

- создание и управление пользователями;
- обработка оплат и подписок;
- применение trial-лимитов и лимитов устройств;
- создание, включение, отключение и удаление туннелей;
- генерация конфигов и QR-кодов;
- ротация и отзыв конфигов устройств;
- сбор статистики использования и состояния нод;
- применение версий routing-профилей;
- API для Bot, Mini App и Admin Panel.

```text
Telegram Bot / Mini App / Admin
        |
        v
API Gateway / BFF
        |
        +--> user-service
        +--> subscription-service
        +--> billing-service
        +--> device-service
        +--> tunnel-service
        +--> config-service
        +--> routing-service
        +--> node-manager-service
        +--> notification-service
        |
        v
PostgreSQL + Redis + RabbitMQ
```

### Data Plane

VPN-ноды принимают зашифрованный трафик, маршрутизируют его, делают NAT и отдают
операционные метрики.

```text
Client App
   |
   v
TUN interface
   |
   v
WireGuard / VLESS / VMess / Hysteria2 client
   |
   v
VPN Node
   |
   +--> direct internet exit
   +--> RU relay path
   +--> foreign exit path
```

## Tunnel Provider Interface

Не стоит создавать отдельный бизнес-сервис под каждый VPN-протокол. Подписки,
оплаты, лимиты устройств, учет трафика и отзыв доступа должны быть общими.

Backend должен зависеть от протокольно-независимого интерфейса туннелей.

```go
type TunnelProvider interface {
    CreateUserTunnel(ctx context.Context, req CreateTunnelRequest) (*TunnelConfig, error)
    EnableTunnel(ctx context.Context, tunnelID string) error
    DisableTunnel(ctx context.Context, tunnelID string) error
    DeleteTunnel(ctx context.Context, tunnelID string) error
    GetUsage(ctx context.Context, tunnelID string) (*TrafficUsage, error)
    HealthCheck(ctx context.Context) error
}
```

Конкретные провайдеры:

- `WireGuardProvider`: peers, ключи, выделенные IP, `AllowedIPs`, kernel WireGuard.
- `XrayProvider`: VLESS, VMess, Reality, TLS, WebSocket, gRPC, JSON/API config.
- `HysteriaProvider`: Hysteria2-профили для быстрых UDP/QUIC-сценариев.
- Позже опционально: `OutlineProvider`, `ShadowsocksProvider`, `TuicProvider`.

Бизнес-слой должен мыслить так:

```text
subscription -> device -> tunnel -> config
```

А не так:

```text
subscription -> wireguard peer
```

Это сохраняет готовность продукта к multi-protocol доступу:

```text
One subscription
  +-- WireGuard config
  +-- VLESS Reality config
  +-- Hysteria2 config
  +-- fallback profile
```

## Node Agent

Для MVP backend может provision-ить ноды через SSH. Для production на каждой
VPN-ноде лучше держать небольшой агент.

```text
Backend node-manager
        |
        | mTLS HTTP/gRPC
        v
node-agent
        |
        +--> wg / wgctrl / systemd
        +--> xray-core / sing-box config API
        +--> nftables / iptables
        +--> traffic collector
        +--> health checks
```

Задачи node-agent:

- применять изменения peer/user;
- reload или hot-update VPN cores;
- собирать статистику трафика;
- отдавать health и capacity;
- сообщать protocol-specific ошибки;
- убрать вечную необходимость прямого shell-доступа центрального backend к нодам.

## Микросервисная архитектура

Проект сразу строится как микросервисная система с чистой архитектурой внутри
каждого сервиса. Деление идет по business capabilities, а не по техническим
деталям или отдельным VPN-протоколам.

Базовые сервисы:

- `api-gateway` / `bff`: единая входная точка для Bot, Mini App и Admin Panel.
- `telegram-service`: Telegram Bot commands, Mini App auth, Telegram-specific UX.
- `user-service`: пользователи, Telegram identity, роли, блокировки.
- `subscription-service`: тарифы, trial, подписки, expiration, лимиты.
- `billing-service`: инвойсы, платежи, webhooks, payment providers.
- `device-service`: устройства, revoke, лимиты устройств, статусы.
- `tunnel-service`: создание и управление туннелями через `TunnelProvider`.
- `config-service`: subscription endpoint, генерация конфигов, QR, динамические server profiles.
- `node-manager-service`: реестр нод, health, capacity, node assignment, failover.
- `routing-service`: smart routing profiles, RU domain/CIDR rules, rule versions.
- `notification-service`: Telegram-уведомления, reminders, service messages.
- `admin-service`: админские операции, audit log, ручное управление.

Протоколы не выносятся в отдельные микросервисы. Они остаются реализациями
внутри `tunnel-service`:

```text
tunnel-service
  +-- WireGuardProvider
  +-- XrayProvider
  +-- HysteriaProvider
  +-- TuicProvider
```

Так подписки, лимиты, отключения и anti-abuse правила не дублируются между
`wireguard-service`, `vless-service`, `vmess-service` и другими техническими
сервисами.

### Чистая архитектура внутри сервиса

Каждый сервис должен иметь одинаковую внутреннюю форму:

```text
cmd/
  service-name/

internal/
  domain/
  usecase/
  ports/
  adapters/
  transport/
  infrastructure/
```

Направление зависимостей:

```text
transport -> usecase -> domain
adapters  -> ports   -> usecase
```

`domain` и `usecase` не должны знать про PostgreSQL, Redis, RabbitMQ, Telegram,
HTTP, gRPC, WireGuard CLI или Xray JSON. Они работают через интерфейсы из `ports`.

## RabbitMQ

RabbitMQ используется как event bus и task queue для микросервисов. Он не заменяет
HTTP/gRPC полностью: синхронные запросы остаются для быстрых операций и чтения
текущего состояния, а RabbitMQ используется для событий, фоновых задач и надежных
асинхронных процессов.

Правило:

```text
HTTP/gRPC        = быстро запросить состояние или выполнить sync use case
RabbitMQ events  = сообщить, что факт уже произошел
RabbitMQ commands = поставить надежную задачу на выполнение
```

### Events и Commands

Event - это факт:

```text
payment.succeeded
subscription.activated
tunnel.provisioned
config.generated
node.became_degraded
```

Command - это просьба выполнить действие:

```text
tunnel.provision
tunnel.disable
config.generate
notification.telegram.send
subscription.expire
```

### Exchanges

Минимальная схема RabbitMQ:

```text
vpn.events.topic      # domain events
vpn.commands.direct   # task/command queues
vpn.retry             # retry queues
vpn.dlx               # dead-letter exchange
```

Рекомендуемые очереди на старте:

```text
subscription.payment-events
tunnel.provision
tunnel.disable
config.generate
notification.telegram
node.health
admin.audit-events
```

### Основной event flow

Первый сквозной поток, который стоит реализовать:

```text
billing-service
  publishes: payment.succeeded

subscription-service
  consumes: payment.succeeded
  activates subscription
  publishes: subscription.activated

tunnel-service
  consumes: subscription.activated
  creates tunnel record
  publishes command: tunnel.provision

node-manager-service / tunnel worker
  consumes: tunnel.provision
  applies config on node
  publishes: tunnel.provisioned

config-service
  consumes: tunnel.provisioned
  generates subscription token, config and QR
  publishes: subscription_config.generated

notification-service
  consumes: subscription_config.generated
  sends Telegram message
```

### Envelope сообщений

Все события и команды должны иметь единый envelope:

```json
{
  "message_id": "uuid",
  "message_type": "subscription.activated",
  "message_version": 1,
  "occurred_at": "2026-05-28T12:00:00Z",
  "producer": "subscription-service",
  "correlation_id": "uuid",
  "causation_id": "uuid",
  "idempotency_key": "subscription:123:activated",
  "payload": {}
}
```

Обязательные поля:

- `message_id`;
- `message_type`;
- `message_version`;
- `occurred_at`;
- `producer`;
- `correlation_id`;
- `idempotency_key`;
- `payload`.

`correlation_id` должен проходить через HTTP/gRPC, RabbitMQ и логгер, чтобы один
пользовательский сценарий можно было собрать по логам между сервисами.

### Идемпотентность

RabbitMQ дает at-least-once delivery, поэтому consumer может получить одно и то же
сообщение больше одного раза.

У каждого consumer должна быть таблица:

```text
processed_messages:
  message_id
  consumer_name
  processed_at
  status
```

Правило обработки:

```text
если message_id уже обработан этим consumer:
  ack
иначе:
  выполнить use case
  записать processed_messages
  ack
```

Команды, которые меняют состояние внешней системы, например применяют peer на
VPN-ноде или активируют подписку, должны быть идемпотентными на уровне use case.

### Retry и DLQ

Не делать бесконечный immediate retry.

Рекомендуемая схема:

```text
main queue
  -> error
  -> retry queue with TTL
  -> back to main queue
  -> after N attempts
  -> DLQ
```

Пример retry-уровней:

```text
retry 1: 10 seconds
retry 2: 1 minute
retry 3: 5 minutes
retry 4: 30 minutes
then DLQ
```

DLQ должна быть видна в admin/ops интерфейсе, чтобы можно было вручную
переобработать или закрыть проблемные сообщения.

### Где RabbitMQ особенно важен

- billing webhooks: быстро принять webhook, сохранить событие и обработать дальше асинхронно;
- tunnel provisioning: применение конфига на ноду может быть долгим и падать;
- subscription expiration: worker публикует команды на отключение туннелей;
- config generation: QR/config можно генерировать отдельным consumer;
- subscription refresh: Happ/v2RayTun вручную обновляют подписку через `/sub/{token}`;
- notifications: Telegram API не должен блокировать billing или provisioning;
- traffic usage: ноды могут отправлять usage батчами;
- node health: изменения состояния нод рассылаются заинтересованным сервисам;
- routing updates: при смене routing profile можно массово пересоздавать конфиги.

## Стратегия VPN-стека

Практичный стек должен развиваться по этапам.

### Этап 1: WireGuard MVP

WireGuard - самый быстрый путь к первому надежному продукту.

Использовать для:

- простого VPN-доступа;
- быстрого onboarding на mobile/desktop;
- QR config flow;
- понятной модели peer/device;
- простой статистики usage.

Production-заметка: на Linux-серверах для производительности лучше использовать
kernel WireGuard, а не userspace `wireguard-go`.

### Этап 2: Xray / VLESS / Reality

Xray стоит добавлять, когда приоритетом становятся обход блокировок и маскировка
трафика.

Использовать для:

- VLESS Reality;
- VMess fallback, если нужен;
- TLS/WebSocket/gRPC transports;
- совместимости с клиентами вроде v2RayTun и похожими приложениями.

Для новых конфигов VLESS стоит предпочитать VMess, если только конкретный клиент
не требует VMess.

### Этап 3: Hysteria2 / TUIC

Добавлять после стабилизации базового продукта.

Использовать для:

- плохих мобильных сетей;
- сценариев с высоким packet loss;
- speed-focused premium profiles;
- альтернативного транспорта, когда TCP-профили деградируют.

### Роли протоколов

| Протокол | Роль | Сильная сторона | Риск |
|---|---|---|---|
| WireGuard | MVP / speed baseline | Быстрый, простой, стабильный | UDP может блокироваться или fingerprint-иться |
| VLESS Reality | Основной anti-blocking профиль | Хорошая маскировка и экосистема | Больше операционной сложности |
| VMess | Compatibility fallback | Поддерживается многими клиентами | Лучше не делать основным |
| Hysteria2 | Быстрый QUIC-профиль | Хорош на сетях с потерями | UDP доступен не везде |
| TUIC | Experimental/premium | QUIC-based performance | Меньшая экосистема |
| Shadowsocks | Optional fallback | Простой и известный | Нужна аккуратная стратегия обфускации |

## Smart Routing для пользователей из РФ

Цель:

```text
RU services -> direct или RU relay
Other traffic -> foreign exit
```

Примеры RU-трафика:

- `.ru`, `.рф`, `.su`;
- VK;
- Госуслуги;
- Yandex;
- Sber;
- Ozon;
- Wildberries;
- Avito;
- экосистема Mail.ru/VK;
- российские банки и платежные сервисы;
- российские CDN/IP ranges.

### Где должен происходить routing

Где возможно, routing лучше делать на клиенте: клиент еще видит домены до того,
как они превратились в IP-соединения.

Слои маршрутизации:

- client TUN rules: лучший UX и минимальная нагрузка на серверы;
- DNS split routing: нужен, чтобы не было DNS leak;
- server-side routing: полезный fallback, но видимость доменов слабее;
- RU relay: нужен, когда direct-доступ ломается, потому что сервис ожидает российский IP.

### Routing modes

```text
Mode: Direct RU
  RU domains/IPs -> direct device connection
  everything else -> VPN

Mode: RU Relay
  RU domains/IPs -> Moscow/RU relay
  everything else -> foreign exit
```

### Источники routing rules

Нужно поддерживать версионированные наборы правил:

- RU domain suffixes;
- curated service domains;
- RU CIDR ranges;
- selected Russian ASN ranges;
- private/local network exclusions;
- optional blocked/malware lists.

`routing-service` должен генерировать rule sets для поддерживаемых клиентов:

- sing-box route rule sets;
- Xray routing rules;
- plain CIDR/domain lists для внутренних инструментов.

## Config Subscription для Happ/v2RayTun

Happ, v2RayTun и похожие клиенты работают не как "одноразовый config file", а как
оркестраторы подписки. Пользователь импортирует subscription link, а затем может
нажать "обновить подписку". В этот момент клиент снова запрашивает backend и
получает актуальный список серверов.

```text
Happ/v2RayTun
  -> GET /sub/{token}
  -> config-service
  -> validate token, device, user, subscription
  -> select healthy node profiles
  -> render client-specific subscription
  -> return config
```

Важно: backend не создает live-туннель при нажатии пользователем "подключить".
Live-туннель создает клиентское приложение на устройстве. Backend создает и
обновляет access credentials и subscription config.

Термины:

```text
Tunnel access = provisioned UUID/key/profile на backend и VPN-нодах
Client tunnel = live TUN/proxy connection внутри Happ/v2RayTun
Node session  = наблюдаемая активность на data plane
```

`config-service` отвечает за:

- стабильный subscription URL;
- хранение hash от subscription token;
- проверку active subscription/device;
- динамический список серверов;
- client-specific format: Happ, v2RayTun, sing-box, Clash, plain URI list;
- обновление списка серверов при degraded/down нодах;
- profile metadata: active_until, device_limit, update interval, server names;
- запись refresh events для аналитики и anti-abuse.

Когда пользователь нажимает "обновить подписку", config-service может вернуть
другой набор серверов:

- убрать degraded/down node;
- добавить новую foreign exit node;
- добавить RU relay;
- поменять transport/profile priority;
- обновить Reality/VLESS параметры;
- обновить routing profile version.

Ping серверов в Happ/v2RayTun выполняется самим клиентом. Это client-side latency
test, а не команда backend. Backend должен только не отдавать явно плохие ноды и
поддерживать собственный health score через `node-manager-service`.

Типы latency:

```text
client-measured ping = измерение из сети пользователя внутри Happ/v2RayTun
server-side health   = измерение node-manager/monitoring инфраструктурой
```

Для UX лучше не делать названия "Обход 1", "Обход 2" полностью случайными при
каждом refresh. Предпочтительнее стабильные profile slots:

```text
Обход 1 - Netherlands
Обход 2 - Germany
Обход 3 - Finland
Обход 4 - Fallback TCP
Обход 5 - RU Relay
```

Менять конкретную ноду внутри slot стоит при degraded/down состоянии, overload или
ручном drain.

## Backend-модули и репозитории

Проект ориентируется на микросервисную архитектуру. На старте можно держать код в
одном mono-repo, но каждый сервис должен иметь отдельную точку входа, отдельные
границы домена и собственные миграции.

```text
services/
  api-gateway/
  telegram-service/
  user-service/
  subscription-service/
  billing-service/
  device-service/
  tunnel-service/
  config-service/
  node-manager-service/
  routing-service/
  notification-service/
  admin-service/

shared/
  logger/
  messaging/
  observability/
  errors/
  contracts/
```

Внутри каждого сервиса:

```text
cmd/
  service-name/

internal/
  domain/
  usecase/
  ports/
  adapters/
  transport/
  infrastructure/

migrations/
```

`shared` не должен превращаться в скрытый монолит. В нем допустимы только
инфраструктурные библиотеки и стабильные контракты: logger, messaging envelope,
tracing, common errors, generated API/event contracts.

## Модель базы данных

Основные сущности:

```text
users
plans
subscriptions
payments
payment_events
devices
vpn_nodes
tunnels
tunnel_configs
traffic_usage
vpn_sessions
subscription_tokens
config_profiles
subscription_refresh_events
server_health_checks
routing_rulesets
admin_audit_log
support_tickets
```

Важные поля туннеля:

```text
tunnels:
  id
  user_id
  device_id
  node_id
  protocol
  status
  assigned_ip
  public_key
  private_key_encrypted
  external_ref
  expires_at
  traffic_limit
  traffic_used
  created_at
  updated_at
```

Поля конфига:

```text
tunnel_configs:
  id
  tunnel_id
  client_type
  config_json_encrypted
  config_uri_encrypted
  qr_payload_encrypted
  version
  expires_at
  created_at
```

Config subscription:

```text
subscription_tokens:
  id
  user_id
  device_id
  subscription_id
  token_hash
  status
  client_type
  format
  expires_at
  last_used_at
  refresh_count
  revoked_at
```

Observed sessions:

```text
vpn_sessions:
  id
  tunnel_id
  node_id
  protocol
  source_ip_hash
  status
  started_at
  last_seen_at
  rx_bytes
  tx_bytes
```

## Жизненный цикл подписки

```text
User starts bot
    |
    v
Trial or payment is created
    |
    v
Subscription becomes active
    |
    v
Backend selects node and protocol
    |
    v
TunnelProvider creates access credentials
    |
    v
ConfigService creates subscription token and returns QR/subscription link
    |
    v
Client refreshes /sub/{token} and receives actual server profiles
    |
    v
Worker checks expiration and limits
    |
    v
Expired tunnel is disabled
```

## Telegram Bot и Mini App UX

Bot должен быть оптимизирован под быстрое подключение.

Минимальный bot flow:

```text
/start
  -> get trial
  -> choose platform
  -> create device
  -> receive QR/config
  -> open setup guide
  -> check connection
```

Разделы Mini App:

- Profile: статус подписки, дата окончания, активные устройства.
- Plans: тарифы, продление, trial, промокоды.
- Devices: добавить, переименовать, отозвать, пересоздать конфиг.
- Configs: QR, subscription link, инструкции под конкретный клиент.
- Usage: трафик, текущая нода, статус подключения.
- Referrals: invite link и бонусные дни.
- Support: тикеты, FAQ, диагностика.

Telegram Mini App `initData` нужно валидировать на backend. Нельзя доверять
Telegram user data, которые пришли только с frontend.

## Billing

Архитектура платежей:

```text
Bot/Mini App
    |
    v
Backend creates invoice
    |
    v
Payment provider
    |
    v
Webhook
    |
    v
BillingService verifies event
    |
    v
SubscriptionService activates or renews subscription
```

Правила:

- сохранять каждый webhook в `payment_events`;
- обрабатывать webhooks идемпотентно;
- проверять signatures/secrets провайдера;
- никогда не активировать подписку только по frontend callback;
- ручные изменения из админки писать в audit log;
- поддерживать замену платежного провайдера через `PaymentProvider` interface.

## Безопасность

Критичные требования:

- не логировать конфиги, private keys, subscription tokens или payment secrets;
- шифровать private keys и сгенерированные конфиги at rest;
- хранить только hashes от long-lived config/subscription tokens;
- использовать короткоживущие one-time links для показа конфигов;
- защищать admin API через RBAC и 2FA;
- использовать HTTPS везде;
- использовать mTLS между backend и node-agent;
- проверять payment webhooks;
- rate-limit для bot, API и config endpoints;
- разделять права node-подсистемы и billing-подсистемы;
- поддерживать revoke на уровне отдельного устройства;
- детектить подозрительный sharing конфигов, но избегать агрессивных false-positive банов.

Anti-sharing сигналы:

- слишком много одновременных IP на одно устройство;
- невозможные geo changes;
- трафик сильно выше нормального профиля тарифа;
- частые повторные скачивания конфига;
- много созданий и удалений устройств;
- много failed auth attempts.

## Observability

Нужно отслеживать и бизнес-метрики, и сетевые метрики.

Network:

- node online/offline;
- CPU/RAM/disk;
- bandwidth;
- packet loss probes;
- p50/p95 latency;
- active tunnels;
- handshake или auth failures;
- traffic per node/protocol.

Business:

- trial starts;
- trial to paid conversion;
- payment success/failure;
- churn;
- active subscriptions;
- active devices;
- support tickets;
- config generation failures.

Рекомендуемый стек:

- Prometheus;
- Grafana;
- Alertmanager;
- Loki или другой log store;
- optional ClickHouse для long-term traffic и product analytics.

### Runtime diagnostics

Так как сервис будет активно использовать HTTP servers, RabbitMQ consumers,
workers, node pollers и фоновые задачи, нужно с первого дня следить за тихими
утечками goroutine.

Минимальные runtime-сигналы:

- `runtime.NumGoroutine()` не должен монотонно расти без причины;
- pprof goroutine profile должен быть доступен в dev/internal окружении;
- отсутствие crash не считается признаком здоровья сервиса.

Для этого есть общие lifecycle-компоненты:

```text
internal/observability/runtime
  runtime monitor: периодически логирует goroutine count и warn при пороге/скачке

internal/observability/pprof
  pprof HTTP server: /debug/pprof/*
```

Конфиг:

```text
RUNTIME_MONITOR_ENABLED=true
RUNTIME_MONITOR_INTERVAL=30s
RUNTIME_GOROUTINE_WARN_THRESHOLD=1000
RUNTIME_GOROUTINE_GROWTH_THRESHOLD=100

PPROF_ENABLED=false
PPROF_ADDR=127.0.0.1:6060
```

В production pprof нельзя открывать наружу. Только localhost, internal network,
VPN/admin доступ или временное включение на время диагностики.

## MVP Scope

MVP должен доказать, что пользователь может оплатить, получить рабочий VPN-конфиг
и продолжать пользоваться сервисом без ручной работы оператора.

MVP включает:

- Go microservices;
- PostgreSQL;
- Redis;
- RabbitMQ;
- Telegram Bot;
- базовый Mini App или простой web cabinet;
- WireGuardProvider;
- одну или две WireGuard-ноды;
- интеграцию платежного провайдера;
- лимиты устройств;
- генерацию конфига и QR;
- expiration worker;
- базовую админку;
- Prometheus + Grafana;
- начальный план smart routing, даже если он еще не полностью автоматизирован.

MVP не должен включать:

- собственный VPN-протокол;
- собственную криптографию;
- full mesh между нодами;
- сложный autoscaling;
- слишком много протоколов сразу;
- идеальный smart routing в первый день.

## Roadmap

### Stage 0: Product and Network Design

Outcome: архитектура достаточно стабильна, чтобы начинать реализацию.

- Определить тарифную модель: trial, monthly, yearly, device limits.
- Выбрать первые регионы: один RU relay, один или два foreign exits.
- Определить supported clients для MVP.
- Решить порядок payment providers: YooKassa, CloudPayments или оба.
- Подготовить legal/privacy docs и support policy.
- Финализировать начальную database schema.

### Stage 1: Backend Foundation

Outcome: базовый микросервисный control plane умеет регистрировать пользователей
и управлять подписками.

- Создать mono-repo структуру `services/*` и `shared/*`.
- Добавить общий logger, config, errors, health endpoint и graceful shutdown.
- Поднять локальный Docker Compose: PostgreSQL, Redis, RabbitMQ.
- Описать messaging envelope и базовые RabbitMQ adapters.
- Реализовать `user-service` и Telegram identity.
- Реализовать `subscription-service`: plans, subscriptions, trial.
- Добавить Redis-backed rate limiting.
- Добавить worker process для scheduled jobs.
- Добавить основу admin audit log.

### Stage 1.5: RabbitMQ Backbone

Outcome: сервисы умеют безопасно обмениваться событиями и командами.

- Настроить exchanges: `vpn.events.topic`, `vpn.commands.direct`, `vpn.retry`, `vpn.dlx`.
- Описать стандарт event/command envelope.
- Добавить publisher confirms.
- Добавить manual ack/nack в consumers.
- Добавить `processed_messages` для идемпотентности consumers.
- Добавить retry queues с TTL.
- Добавить DLQ и базовый ops-view для проблемных сообщений.
- Протащить `correlation_id` через HTTP/gRPC, RabbitMQ и logger.

### Stage 2: WireGuard MVP

Outcome: пользователь может получить настоящий рабочий WireGuard config.

- Реализовать `TunnelProvider` interface.
- Реализовать `WireGuardProvider`.
- Генерировать WireGuard keys и assigned IPs.
- Добавлять peer на ноду через SSH или local node adapter.
- Генерировать `.conf` и QR-код.
- Хранить config material в зашифрованном виде.
- Добавить expiration worker, который отключает expired peers.
- Добавить базовый usage collection.

### Stage 2.5: Config Subscription MVP

Outcome: Happ/v2RayTun могут импортировать стабильную subscription link и
обновлять список серверов кнопкой refresh.

- Реализовать `config-service`.
- Создать `subscription_tokens` и хранить только `token_hash`.
- Реализовать `GET /sub/{token}`.
- Проверять active token, user, device и subscription.
- Генерировать plain URI list для VLESS/WireGuard-compatible profiles.
- Логировать `subscription_refresh_events`.
- Добавить `client_type` и `format`.
- Обновлять `last_used_at` и `refresh_count`.
- Возвращать только healthy/degraded-acceptable node profiles.

### Stage 3: Telegram Bot MVP

Outcome: первый пользовательский flow полностью работает внутри Telegram.

- Реализовать `/start`.
- Реализовать trial activation.
- Реализовать "my subscription".
- Реализовать "add device".
- Реализовать "get config".
- Реализовать platform-specific setup guides.
- Реализовать support contact flow.
- Добавить Telegram notifications перед окончанием подписки.

### Stage 4: Billing

Outcome: paid subscriptions работают безопасно.

- Добавить `PaymentProvider` interface.
- Интегрировать первого payment provider.
- Создать invoice flow из Bot/Mini App.
- Реализовать webhook verification.
- Добавить idempotent payment event processing.
- Публиковать `payment.succeeded` и `payment.failed` через RabbitMQ.
- Активировать и продлевать subscriptions только через server-side webhooks.
- Добавить payment history в admin panel.

### Stage 5: Admin Panel

Outcome: оператор может поддерживать пользователей без прямого доступа к базе.

- Поиск users по Telegram ID и username.
- Subscription view и manual extension.
- Manual trial issuance.
- Device list и revoke.
- Tunnel status и assigned node.
- Payment и webhook history.
- Node health page.
- Audit log.

### Stage 6: Node Agent

Outcome: provisioning нод больше не зависит от ручных SSH-операций.

- Реализовать node-agent service.
- Добавить mTLS между backend и agents.
- Добавить agent registration.
- Реализовать WireGuard peer apply/remove.
- Добавить traffic и health reporting.
- Публиковать `node.health_changed` и `node.became_degraded`.
- Добавить node capacity и load score.
- Заменить SSH provisioning path.

### Stage 7: Xray / VLESS Reality

Outcome: anti-blocking профиль доступен без изменения subscription logic.

- Реализовать `XrayProvider`.
- Добавить генерацию VLESS Reality configs.
- Добавить VMess только как compatibility fallback, если нужно.
- Поддержать применение xray-core или sing-box config через node-agent.
- Добавить multi-protocol config на устройство или подписку.
- Добавить protocol selector в Bot/Mini App.
- Добавить operational metrics per protocol.
- Добавить Happ/v2RayTun-specific renderers для subscription profiles.

### Stage 8: Smart Routing v1

Outcome: российские сервисы маршрутизируются напрямую или через RU relay.

- Создать `routing-service`.
- Поддерживать RU domain и RU CIDR rule sets.
- Добавить curated service groups: банки, marketplaces, Yandex, VK, Gosuslugi.
- Генерировать sing-box/Xray-compatible rules.
- Добавить routing profile versions.
- Добавить config regeneration при изменении routing profile.
- Добавить режимы "Direct RU" и "RU Relay".
- Добавить проверки DNS leak и IPv6 leak.

### Stage 9: Reliability and Anti-Abuse

Outcome: сервис переживает типовые production-проблемы.

- Добавить node failover logic.
- Добавить degraded node state.
- Добавить config migration на другую ноду.
- Добавить suspicious sharing detection.
- Добавить soft lock и manual review flows.
- Добавить backup и restore procedures.
- Добавить alerts на node loss, payment failures и config errors.
- Добавить replay/retry tooling для сообщений из DLQ.

### Stage 10: Mini App

Outcome: Telegram получает полноценный продуктовый интерфейс.

- Profile screen.
- Subscription и tariffs.
- Devices management.
- Config screen с QR и deep links.
- Usage stats.
- Referral system.
- Support tickets.
- Diagnostics screen.

### Stage 11: Scale to 1k Users

Outcome: система готова к реальному платному трафику.

- Разнести backend и monitoring hosts.
- Добавить несколько VPN-нод.
- Добавить node pool assignment.
- Добавить Prometheus alerts.
- Добавить log aggregation.
- Добавить staging environment.
- Добавить automated deploys.
- Добавить smoke tests для config generation и node apply.

### Stage 12: Scale to 10k+ Users

Outcome: архитектура может расти без переписывания продукта.

- Выносить отдельные сервисы в разные репозитории, если mono-repo начнет мешать.
- Перенести analytics в ClickHouse.
- Добавить multi-region node pools.
- Добавить canary rollout для routing rules и node configs.
- Добавить provider/IP reputation tracking.
- Добавить automated capacity planning.
- Добавить несколько payment providers.
- Добавить Hysteria2/TUIC как premium или fallback profiles.

## Начальный порядок реализации

Самый безопасный порядок разработки:

1. Mono-repo skeleton для `services/*` и `shared/*`.
2. Docker Compose: PostgreSQL, Redis, RabbitMQ.
3. Общие logger, config, health, graceful shutdown.
4. Messaging envelope, RabbitMQ publisher/consumer adapters.
5. PostgreSQL migrations по сервисам.
6. `user-service` и `subscription-service`.
7. `TunnelProvider` interface внутри `tunnel-service`.
8. `WireGuardProvider`.
9. Telegram Bot trial flow.
10. Config и QR generation.
11. Subscription endpoint `/sub/{token}` для Happ/v2RayTun.
12. Expiration worker через RabbitMQ command.
13. Billing webhooks и payment events.
14. Admin panel basics.
15. Node agent.
16. Xray/VLESS Reality.
17. Smart routing.
18. Mini App polish и referrals.

## Главное инженерное правило

Не привязывать бизнес-логику к одному протоколу.

WireGuard - лучший путь для MVP, но архитектура должна продавать и управлять
доступом к "туннелю", а не к "WireGuard peer". Это разница между быстрым скриптом
и масштабируемым VPN-продуктом.
