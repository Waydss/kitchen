# Repository Guidelines

## Project Structure & Module Organization

The repository currently contains the task specification in `backend-trainee-assignment-autumn-2026.md`; application code has not yet been added. Keep the future layout easy to navigate:

- `cmd/` — executable entry points for the main API and example establishment service.
- `internal/` — domain, application, transport, and persistence code.
- `migrations/` — ordered database migrations such as `0001_create_orders.up.sql`.
- `api/openapi.yaml` — the public and establishment-facing API contract.
- `docs/` — CJM, C4, and database diagrams stored as PlantUML or another text format.
- `tests/` — integration and end-to-end tests; keep unit tests beside their source when the chosen language convention supports it.

Both services and their database must start through the root `compose.yaml` (or `docker-compose.yml`).

## Build, Test, and Development Commands

Expose routine workflows through a root `Makefile`, regardless of implementation language:

- `make up` — build and start the complete Docker Compose stack.
- `make down` — stop the local stack without deleting persistent data by default.
- `make test` — run all unit and integration tests.
- `make lint` — run the configured formatter, linter, and static analysis.
- `make migrate` — apply database migrations to the local database.

Document any required environment variables and language-specific commands in `README.md`. Keep `.env.example` safe to commit and never place real credentials in it.

## Coding Style & Naming Conventions

Use the standard formatter for the selected language (for example, `gofmt` for Go or Ruff for Python) and commit its configuration. Prefer small packages/modules grouped by business capability (`orders`, `menu`, `establishments`) rather than generic utility folders. Use descriptive names, lowercase directory names, and consistent API field naming. Keep HTTP handlers thin; business rules belong in domain or application services.

## Testing Guidelines

Cover business scenarios from both customer and establishment journeys. Name tests after observable behavior, such as `TestCreateOrderRejectsUnavailableItem`. Unit-test domain rules and add integration tests for database migrations, API contracts, and service-to-service communication. `make test` must pass from a clean checkout; add regression tests with every bug fix.

## Commit & Pull Request Guidelines

History currently establishes only a short, imperative style (`Initial commit`). Continue with focused subjects such as `Add order creation endpoint`; optional prefixes like `feat:` and `fix:` are acceptable if used consistently. Pull requests should explain the user scenario, architecture or schema changes, migration/rollback impact, and verification commands. Link relevant issues and update OpenAPI, diagrams, and README whenever behavior changes.
