# ADR 0004 - Database tooling: golang-migrate v4 + sqlc v2

**Status:** Accepted

## Context

ADR-0002 established the OpenAPI spec as the API source of truth. We
also need a source of truth for the database schema and a type-safe
query layer that does not introduce an ORM.

The data model (DATA_MODEL.md §5-6) specifies:
- 15 sequential migrations managed by golang-migrate.
- sqlc for generating typed Go query code from `.sql` files.
- pgx/v5 as the Postgres driver.

## Decision

**Migrations:** `github.com/golang-migrate/migrate/v4`

- SQL files live in `server/db/migrations/` as
  `NNNNNN_<name>.{up,down}.sql` pairs.
- The migrations FS is embedded at compile time via `//go:embed`
  in `server/db/embed.go` and passed to `iofs.New`.
- `database.RunMigrations(dsn)` is called from the server's
  startup path (no separate init container in v1).
- This makes the binary self-migrating and keeps the startup
  contract simple.

**Queries:** `github.com/sqlc-dev/sqlc` v2

- Config: `server/sqlc.yaml` (postgresql engine, pgx/v5 driver,
  `emit_pointers_for_null_types: true`, uuid and int4[] overrides).
- Generated output: `server/internal/repository/`.
- Invocation: `go run github.com/sqlc-dev/sqlc/cmd/sqlc generate`
  from `server/` — version pinned via `tools.go`.

**Postgres CHECK constraint workaround:**

DATA_MODEL.md §3.6 specifies a check constraint on
`user_settings.reminder_lead_days` that uses a subquery
(`NOT EXISTS (SELECT 1 FROM unnest(...))`). Postgres does not allow
subqueries inside CHECK constraints. We replace it with a call to
an immutable helper function `int_array_all_in_range(arr, lo, hi)`
defined in migration 000002, which wraps the same logic. Semantics
are identical; this is not a behavioral deviation.

## Consequences

- Contributors must run `make generate-sqlc` after changing query
  files. The `make generate` target already includes this.
- The generated directory `server/internal/repository/` is
  committed to git (same pattern as `server/internal/api/`).
- The server binary includes the migration SQL via embed, which
  adds a small amount to binary size (acceptable).
- golang-migrate handles advisory locking, so a future move to
  multiple replicas does not break startup ordering.
