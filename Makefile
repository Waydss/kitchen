.PHONY: env up down logs swagger build fmt fmt-check vet lint vuln test test-integration coverage test-e2e test-all check migrate smoke

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

fmt-check:
	@test -z "$$(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'))" || \
		(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'); exit 1)

vet:
	go vet ./...

lint:
	docker run --rm -v "$$(pwd):/app" -w /app golangci/golangci-lint:v2.13.0 golangci-lint run ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

test:
	go test ./... -race -count=1

test-integration:
	docker compose up -d --wait postgres
	docker compose run --rm migrate
	TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-kitchen}:$${POSTGRES_PASSWORD:-kitchen}@localhost:$${POSTGRES_PORT:-55432}/$${POSTGRES_DB:-kitchen}?sslmode=disable" go test -tags=integration ./... -count=1

coverage:
	docker compose up -d --wait postgres
	docker compose run --rm migrate
	TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-kitchen}:$${POSTGRES_PASSWORD:-kitchen}@localhost:$${POSTGRES_PORT:-55432}/$${POSTGRES_DB:-kitchen}?sslmode=disable" go test -tags=integration ./... -coverprofile=coverage.out -count=1
	@coverage=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$3); print $$3}'); \
		echo "Total coverage: $$coverage%"; \
		awk -v coverage="$$coverage" 'BEGIN { if (coverage < 60) exit 1 }'

test-e2e: up
	E2E_API_URL="http://localhost:$${API_PORT:-8080}" go test -tags=e2e ./tests/e2e -count=1

migrate:
	docker compose run --rm migrate

smoke: up
	./scripts/smoke.sh

test-all: test test-integration test-e2e

check: fmt-check vet test build
