# Авито.Кухня

MVP backend-сервиса заказа еды на Go. Web-клиент работает только с Kitchen API; отдельный Example Restaurant доступен лишь внутри Docker Compose. Источник истины по заказам — PostgreSQL Kitchen.

## Быстрый запуск

Нужны Docker, Docker Compose и, для локальных команд, Go версии из `go.mod`.

```bash
cp .env.example .env  # необязательно: Compose имеет безопасные defaults
make up
make smoke
```

- Kitchen API: `http://localhost:8080`
- Swagger UI: `http://localhost:8082`
- PostgreSQL: `127.0.0.1:55432`
- Example Restaurant: только `http://restaurant:8081` внутри Compose-сети

Порт PostgreSQL намеренно отличается от стандартного `5432`, чтобы не конфликтовать с локальной установкой. Значения можно изменить в `.env`; реальные секреты коммитить нельзя.

## API

Публичные маршруты Kitchen API:

| Метод | Маршрут | Назначение |
|---|---|---|
| GET | `/health/live` | процесс API работает |
| GET | `/health/ready` | API готов и PostgreSQL доступен |
| GET | `/api/v1/restaurants` | список заведений |
| GET | `/api/v1/restaurants/{restaurantId}` | карточка заведения |
| GET | `/api/v1/restaurants/{restaurantId}/menu` | актуальное меню |
| POST | `/api/v1/orders` | идемпотентное создание заказа |
| GET | `/api/v1/orders?customer_id={uuid}` | заказы пользователя от новых к старым |
| GET | `/api/v1/orders/{orderId}` | заказ и история статусов |

Внутренние маршруты Example Restaurant:

| Метод | Маршрут | Назначение |
|---|---|---|
| GET | `/health/live` | healthcheck контейнера |
| GET | `/api/v1/info` | идентификатор и название партнёра |
| POST | `/api/v1/orders` | принять заказ от Kitchen API |

Публичный контракт и данные владельца описаны в `api/openapi.yaml`, внутренний контракт — в `api/restaurant-openapi.yaml`. Swagger UI показывает только публичный API и поддерживает `Try it out`.

## Создание заказа и идемпотентность

```bash
curl -i http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: example-order-0001' \
  -d '{
    "customer_id":"50000000-0000-0000-0000-000000000001",
    "restaurant_id":"10000000-0000-0000-0000-000000000001",
    "items":[{
      "menu_item_id":"30000000-0000-0000-0000-000000000001",
      "quantity":1
    }]
  }'
```

Kitchen API валидирует запрос, блокирует и проверяет строки меню, фиксирует название и цену, создаёт `restaurant_confirmation_pending` в транзакции, затем отправляет тот же `order_id` партнёру. Решение сохраняется как `confirmed` либо `rejected`. При временной сетевой ошибке возвращается `202`, заказ остаётся pending, а повтор с теми же телом и `Idempotency-Key` безопасно повторяет доставку.

Уникальный `(customer_id, idempotency_key)` защищает Kitchen от дубликатов и гонок. Тот же ключ с другим телом даёт `409`. Example Restaurant хранит отпечаток нормализованного payload по `order_id`: точный повтор не списывает остаток второй раз, изменённый запрос даёт `idempotency_conflict`.

В MVP реализовано только начальное решение `pending → confirmed/rejected`. Значения `preparing`, `ready`, `delivering`, `delivered` и `cancelled` зарезервированы схемой для развития; callback и фоновый retry не реализованы.

`customer_id` передаётся клиентом, потому что регистрация и authentication не входят в тестовое задание. Demo UUID из примера не проверяется по таблице пользователей. Поэтому список заказов по `customer_id` — демонстрационный контракт, а не защищённый личный кабинет; в production идентификатор должен извлекаться из access token.

## Архитектура и данные

Поток создания: `Client → Kitchen handler → order service → repository/PostgreSQL → internal HTTP client → Example Restaurant → Kitchen → Client`. Внешний HTTP-вызов выполняется после commit транзакции БД. Клиенту никогда не передаётся адрес партнёра, а пользовательские `GET` читают только PostgreSQL Kitchen.

Код сгруппирован по бизнес-возможностям:

- `cmd/api`, `cmd/restaurant` — точки входа двух процессов;
- `internal/catalog`, `internal/orders` — handlers, services, interfaces и PostgreSQL repositories;
- `internal/examplerestaurant` — логика внутреннего партнёра;
- `internal/platform` — config, HTTP server и PostgreSQL infrastructure;
- `migrations` — versioned up/down migrations и demo seed;
- `tests/e2e` — полный клиентский сценарий;
- `api` и `docs` — OpenAPI, CJM, C4 и ER-диаграмма.

Цены хранятся целыми копейками (`59000` = 590 ₽). `order_items` содержит неизменяемые снимки названия, SKU и цены. FK/CHECK/UNIQUE защищают принадлежность меню заведению, положительные значения и идемпотентность. Если у заведения есть изображения, отложенный constraint требует ровно одну обложку.

## Разработка и проверки

```bash
make fmt              # применить gofmt
make check            # fmt-check, go vet, race-тесты, build
make test             # unit-тесты с race detector
make test-integration # repository-тесты с реальным PostgreSQL
make coverage         # integration-набор и порог покрытия 60%
make test-e2e         # Client → Kitchen → DB → Restaurant → Kitchen
make test-all         # unit, integration и e2e
make lint             # golangci-lint в закреплённом Docker image
make vuln             # govulncheck
make migrate          # применить новые migrations
make swagger          # поднять стек и показать адрес Swagger UI
make down             # остановить стек, сохранив volume БД
```

Unit-тесты лежат рядом с кодом и называются `Test<Behavior>`. Integration-файлы используют suffix `_integration_test.go` и build tag `integration`; e2e — tag `e2e`. CI проверяет форматирование, race, сборку, lint, уязвимости, Compose, PostgreSQL integration с порогом покрытия, e2e и smoke.

## Конфигурация и ограничения

Конфигурация читается из environment и валидируется при старте. Compose автоматически загружает `.env`; отдельная библиотека dotenv в бинарнике не нужна и не скрывает источник production-конфигурации. Все доступные параметры и безопасные локальные defaults перечислены в `.env.example`.

Authentication, оплата, frontend, доставка, загрузка изображений и административный API находятся вне scope. Остатки и idempotency state Example Restaurant хранятся в памяти и сбрасываются при рестарте. Для production потребуются persistent storage партнёра, authentication публичного и внутреннего API, transactional outbox/worker, метрики и tracing.

Архитектурные материалы: `docs/c4-container.puml`, `docs/database.puml`, `docs/cjm-user.puml` и `docs/cjm-restaurant.puml`. Они хранятся в text-based PlantUML, поэтому изменения хорошо видны в code review.
