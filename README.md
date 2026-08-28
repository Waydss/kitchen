# Авито.Кухня

MVP backend-сервиса заказа еды. Kitchen API — единственная точка входа для web-клиента и источник истины по заказам. Example restaurant — внутренний сервис-партнёр: его адрес и контракт клиенту не передаются. Полный архитектурный замысел описан в [PLAN.md](PLAN.md).

## Быстрый запуск

Нужны Docker и Docker Compose. Для запуска Go-команд вне контейнеров используется Go `1.26.6`.

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

Название, версия, описание и контакт владельца API задаются в секции `info` файла `api/openapi.yaml`. Внутренний контракт и его контакт находятся в `api/restaurant-openapi.yaml`.

PostgreSQL публикуется только на loopback-интерфейсе и по умолчанию использует host-порт `55432`, чтобы не конфликтовать с локальным PostgreSQL на `5432`. Внутри Compose сервисы по-прежнему используют `postgres:5432`; порт можно изменить через `.env`.

## Публичный API Kitchen

- `GET /health/live` и `GET /health/ready` — liveness/readiness.
- `GET /api/v1/restaurants` — список заведений с обложкой.
- `GET /api/v1/restaurants/{restaurantId}` — карточка с упорядоченной галереей.
- `GET /api/v1/restaurants/{restaurantId}/menu` — меню, доступность и изображения блюд.
- `POST /api/v1/orders` — идемпотентное создание заказа.
- `GET /api/v1/orders?customer_id={uuid}` — все заказы пользователя, начиная с нового.
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

Kitchen проверяет заведение и позиции, рассчитывает цену из БД, сохраняет pending-заказ и снимки товаров, затем отправляет внутренний `order_id` и внешние SKU в restaurant service. Результат партнёра атомарно сохраняется как `confirmed` или `rejected`; для отказа API также возвращает `rejection_reason`. При сетевой ошибке клиент получает `202`, а заказ остаётся `restaurant_confirmation_pending`; повтор исходного запроса безопасно повторяет доставку того же `order_id`.

Идемпотентность клиента обеспечивается уникальным `(customer_id, idempotency_key)` в PostgreSQL. Тот же ключ с другим составом возвращает `409`. Example restaurant хранит отпечаток нормализованного payload для каждого Kitchen `order_id`: точный повтор безопасен, а изменённый payload возвращает `idempotency_conflict` и не списывает остаток.

Demo restaurant ID: `10000000-0000-0000-0000-000000000001`. Цена хранится в копейках: `59000` означает 590 ₽.

Историю заказов пользователя можно получить так:

```bash
curl 'http://localhost:8080/api/v1/orders?customer_id=50000000-0000-0000-0000-000000000001'
```

Ответ содержит `orders`, отсортированные от нового к старому. Для пользователя без заказов возвращается `{"orders":[]}`.

## Изображения каталога

PostgreSQL хранит только URL изображений. Для списка заведений поле `cover_image_url` берётся из единственной записи `restaurant_images` с `is_cover = true`; если фотографий нет, API возвращает `null`. Карточка возвращает все фотографии в `images`, отсортированные по `position`, либо пустой массив. У позиции меню `image_url` также может быть `null`. Загрузка файлов и внешнее object storage сознательно не входят в MVP.

## Архитектура и данные

Поток запроса: `Web Client → Kitchen HTTP handler → order service → repository/PostgreSQL → internal restaurant HTTP client → Example Restaurant`. Example restaurant не читает PostgreSQL Kitchen и не является источником пользовательского состояния.

Таблицы: `restaurants`, `restaurant_images`, `menu_categories`, `menu_items`, `orders`, `order_items`, `order_status_history`. `order_items` содержит снимки названия, внешнего SKU и цены на момент заказа; текущие изображения каталога в снимок заказа не входят. Составные FK не позволяют связать заказ или товар с чужим заведением. Migrations применяются до старта API.

## Команды разработки

```bash
make fmt              # gofmt
make check            # форматирование, go vet, unit-тесты и сборка
make test             # unit-тесты с race detector
make test-integration # repository и PostgreSQL
make test-e2e         # Client → Kitchen → PostgreSQL → Restaurant → Kitchen
make test-all         # все уровни тестов
make lint             # golangci-lint
make vuln             # govulncheck по актуальной базе Go
make build            # сборка обоих бинарников
make smoke            # запуск Compose и проверка публичного API
make swagger          # запуск стека и адрес Swagger UI
make down             # остановка без удаления данных
```

## Документация и ограничения

В `docs/` находятся CJM пользователя и заведения, C4 Container и ER-диаграмма в PlantUML. Конфигурация задаётся явно через environment; Compose автоматически читает `.env`, а значения по умолчанию находятся в `.env.example`. Дополнительная библиотека для `.env` не используется намеренно: бинарники не скрывают источник конфигурации, а реальные секреты и локальный `.env` не коммитятся. GitHub Actions запускает сборку, race-тесты, `go vet`, golangci-lint, govulncheck, integration/e2e и smoke-проверки.

Authentication, оплата, frontend и доставка вне scope MVP. Остатки и дедупликация example restaurant пока хранятся в памяти и сбрасываются при рестарте. Для production потребовались бы долговременное хранилище партнёра, transactional outbox/worker, authentication внутреннего API, метрики и трассировка.
