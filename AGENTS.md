# Repository Guidelines

## Project Structure & Module Organization

This Go repository contains two binaries: `cmd/api` for the public Kitchen API and `cmd/restaurant` for the internal partner simulator. Business capabilities live in `internal/catalog`, `internal/order`, and `internal/restaurant`; infrastructure packages are `internal/config`, `internal/httpserver`, and `internal/postgres`. Keep SQL changes as paired, ordered files in `migrations/`. Public and internal contracts are `api/kitchen-openapi.yaml` and `api/restaurant-openapi.yaml`. PlantUML sources and rendered diagrams live in `docs/diagrams`, while full-stack tests are in `tests/e2e`.

## Build, Test, and Development Commands

- `make up` builds and starts PostgreSQL, migrations, both services, and Swagger UI.
- `make check` runs formatting checks, `go vet`, race-enabled unit tests, and compilation.
- `make test-integration` tests repositories against the Compose PostgreSQL instance.
- `make coverage` runs the integration suite and enforces at least 60% statement coverage.
- `make test-e2e` verifies the Client → Kitchen → PostgreSQL → Restaurant flow.
- `make lint` runs the pinned golangci-lint Docker image; `make vuln` runs govulncheck.
- `make down` stops containers without deleting the database volume.

## Coding Style & Naming Conventions

Run `gofmt`; use tabs as produced by the formatter. Package and directory names are short lowercase nouns. Exported Go names use `PascalCase`; local names use `camelCase`. Keep Gin handlers limited to transport concerns, put business rules in services, and isolate SQL in repositories. Define narrow interfaces at the consuming layer, such as a handler’s service dependency. JSON fields and API parameters use `snake_case`.

## Testing Guidelines

Place unit tests beside production code and name them after observable behavior, for example `TestCreateRejectsDuplicateItems`. Use the `integration` build tag for PostgreSQL tests and `e2e` for full-stack tests. Every bug fix should include a regression test. Before opening a PR, run `make check`, `make coverage`, and `make test-e2e`.

## Commit & Pull Request Guidelines

History uses concise Conventional Commit subjects such as `feat: add customer order history endpoint`. Keep commits focused and use `feat:`, `fix:`, `test:`, `docs:`, or `chore:` consistently. PRs should explain behavior, schema or contract changes, migration rollback impact, and commands run. Update OpenAPI, README, and diagrams whenever externally visible behavior changes; attach screenshots only for Swagger or other UI changes.
