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
Go Backend API
        |
        +--> users
        +--> subscriptions
        +--> billing
        +--> devices
        +--> tunnel-service
        +--> config-service
        +--> routing-service
        +--> node-manager
        |
        v
PostgreSQL + Redis
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

## Backend-модули

Начинать лучше с modular monolith. Разделять на сервисы стоит только тогда, когда
к этому принуждает масштаб или границы команды.

```text
cmd/
  api/
  bot/
  worker/
  node-agent/

internal/
  users/
  auth/
  billing/
  subscriptions/
  plans/
  devices/
  tunnels/
  configs/
  nodes/
  routing/
  traffic/
  support/
  admin/
  telegram/
  observability/

pkg/
  tunnelprovider/
  wireguard/
  xray/
  hysteria/
  payments/
```

Возможные service boundaries на будущее:

- `api-gateway`;
- `bot-service`;
- `billing-service`;
- `subscription-service`;
- `tunnel-service`;
- `node-manager`;
- `config-service`;
- `routing-service`;
- `admin-api`;
- `worker`.

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
TunnelProvider creates tunnel
    |
    v
ConfigService returns QR/config/subscription link
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

## MVP Scope

MVP должен доказать, что пользователь может оплатить, получить рабочий VPN-конфиг
и продолжать пользоваться сервисом без ручной работы оператора.

MVP включает:

- Go backend;
- PostgreSQL;
- Redis;
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

Outcome: базовый control plane умеет регистрировать пользователей и управлять подписками.

- Создать структуру Go-проекта.
- Добавить config, logging, migrations и health endpoint.
- Реализовать users и Telegram identity.
- Реализовать plans и subscriptions.
- Добавить Redis-backed rate limiting.
- Добавить worker process для scheduled jobs.
- Добавить основу admin audit log.

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

- Разделить modular monolith, если это действительно нужно.
- Перенести analytics в ClickHouse.
- Добавить multi-region node pools.
- Добавить canary rollout для routing rules и node configs.
- Добавить provider/IP reputation tracking.
- Добавить automated capacity planning.
- Добавить несколько payment providers.
- Добавить Hysteria2/TUIC как premium или fallback profiles.

## Начальный порядок реализации

Самый безопасный порядок разработки:

1. Go backend skeleton.
2. PostgreSQL migrations.
3. Users и subscriptions.
4. `TunnelProvider` interface.
5. `WireGuardProvider`.
6. Telegram Bot trial flow.
7. Config и QR generation.
8. Expiration worker.
9. Billing webhooks.
10. Admin panel basics.
11. Node agent.
12. Xray/VLESS Reality.
13. Smart routing.
14. Mini App polish и referrals.

## Главное инженерное правило

Не привязывать бизнес-логику к одному протоколу.

WireGuard - лучший путь для MVP, но архитектура должна продавать и управлять
доступом к "туннелю", а не к "WireGuard peer". Это разница между быстрым скриптом
и масштабируемым VPN-продуктом.
