.PHONY: env up down logs swagger build fmt lint test test-integration test-e2e migrate smoke

env:
	@test -f .env || cp .env.example .env

up: env
	docker compose up --build -d --wait

down:
	docker compose down

logs:
	docker compose logs -f

swagger: up
	@echo "Swagger UI: http://localhost:$${SWAGGER_PORT:-8082}"

build:
	go build ./...

fmt:
	gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')

lint:
	docker run --rm -v "$$(pwd):/app" -w /app golangci/golangci-lint:v2.4.0 golangci-lint run ./...

test:
	go test ./... -race -count=1

test-integration:
	docker compose up -d --wait postgres migrate
	TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-kitchen}:$${POSTGRES_PASSWORD:-kitchen}@localhost:$${POSTGRES_PORT:-5432}/$${POSTGRES_DB:-kitchen}?sslmode=disable" go test -tags=integration ./... -count=1

test-e2e: up
	E2E_API_URL="http://localhost:$${API_PORT:-8080}" go test -tags=e2e ./tests/e2e -count=1

migrate:
	docker compose run --rm migrate

smoke: up
	./scripts/smoke.sh
