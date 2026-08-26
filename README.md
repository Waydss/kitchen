# Авито.Кухня

MVP backend-сервиса заказа еды. Kitchen API — единственная точка входа для web-клиента и источник истины по заказам. Example restaurant — внутренний сервис-партнёр: его адрес и контракт клиенту не передаются. Полный архитектурный замысел описан в [PLAN.md](PLAN.md).

## Быстрый запуск

Нужны Docker и Docker Compose.

```bash
cp .env.example .env  # необязательно
make up
make smoke
```

Публичный API доступен на `http://localhost:8080`. Example restaurant не публикует порт на host и доступен только Kitchen API внутри Compose-сети как `http://restaurant:8081`.

## Swagger UI

Swagger UI запускается вместе с Compose и доступен по адресу:

```text
http://localhost:8082
```

Также можно выполнить `make swagger`: команда поднимет стек и напечатает адрес интерфейса. Swagger UI загружает публичный контракт `api/openapi.yaml`. Кнопка `Try it out` отправляет запросы в Kitchen API на `http://localhost:8080`; для неё настроен ограниченный CORS только с origin Swagger UI.

Название, версия, описание и владелец API задаются в секции `info` файла `api/openapi.yaml`. Перед публикацией замените `TODO` и `your-email@example.com` на свои данные. Аналогичная секция внутреннего контракта находится в `api/restaurant-openapi.yaml`.

Если локальный PostgreSQL уже занимает `5432`, задайте свободный host-порт в `.env`, например `POSTGRES_PORT=55432`. Внутри Compose сервисы по-прежнему используют `postgres:5432`.

## Публичный API Kitchen

- `GET /health/live` и `GET /health/ready` — liveness/readiness.
- `GET /api/v1/restaurants` — список заведений.
- `GET /api/v1/restaurants/{restaurantId}` — карточка заведения.
- `GET /api/v1/restaurants/{restaurantId}/menu` — меню и доступность.
- `POST /api/v1/orders` — идемпотентное создание заказа.
- `GET /api/v1/orders/{orderId}` — заказ и история статусов из PostgreSQL Kitchen.

Полный публичный контракт находится в `api/openapi.yaml`. Внутренний контракт example restaurant описан отдельно в `api/restaurant-openapi.yaml` и не предназначен для web-клиента.

## Создание заказа

Клиент отправляет заказ только в Kitchen API:

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

Kitchen проверяет заведение и позиции, рассчитывает цену из БД, сохраняет pending-заказ и снимки товаров, затем отправляет внутренний `order_id` и внешние SKU в restaurant service. Результат партнёра атомарно сохраняется как `confirmed` или `rejected` и только после этого возвращается клиенту. При сетевой ошибке клиент получает `202`, а заказ остаётся `restaurant_confirmation_pending`; повтор исходного запроса безопасно повторяет доставку того же `order_id`.

Идемпотентность клиента обеспечивается уникальным `(customer_id, idempotency_key)` в PostgreSQL. Тот же ключ с другим составом возвращает `409`. Example restaurant дедуплицирует внутренние запросы по неизменному Kitchen `order_id`, поэтому повтор не списывает остаток и не создаёт второй внешний заказ.

Demo restaurant ID: `10000000-0000-0000-0000-000000000001`. Цена хранится в копейках: `59000` означает 590 ₽.

## Архитектура и данные

Поток запроса: `Web Client → Kitchen HTTP handler → order service → repository/PostgreSQL → internal restaurant HTTP client → Example Restaurant`. Example restaurant не читает PostgreSQL Kitchen и не является источником пользовательского состояния.

Таблицы: `restaurants`, `menu_categories`, `menu_items`, `orders`, `order_items`, `order_status_history`. `order_items` содержит снимки названия, внешнего SKU и цены на момент заказа. Составные FK не позволяют связать заказ или товар с чужим заведением. Migrations применяются до старта API.

## Команды разработки

```bash
make fmt              # gofmt
make test             # unit-тесты с race detector
make test-integration # repository и PostgreSQL
make test-e2e         # Client → Kitchen → PostgreSQL → Restaurant → Kitchen
make lint             # golangci-lint
make build            # сборка обоих бинарников
make smoke            # запуск Compose и проверка публичного API
make swagger          # запуск стека и адрес Swagger UI
make down             # остановка без удаления данных
```

## Документация и ограничения

В `docs/` находятся CJM пользователя и заведения, C4 Container и ER-диаграмма в PlantUML. Конфигурация задаётся через environment; реальные секреты и `.env` не коммитятся.

Authentication, оплата, frontend и доставка вне scope MVP. Остатки и дедупликация example restaurant пока хранятся в памяти и сбрасываются при рестарте. Для production потребовались бы долговременное хранилище партнёра, transactional outbox/worker, authentication внутреннего API, метрики и трассировка.
