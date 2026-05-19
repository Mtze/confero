# ADR 0001 - Go, Postgres, React as the core stack

**Status:** Accepted

## Context

Confero is a small internal tool for the TUM CS Education chair.
We need a backend language, a database, and a frontend framework.
The tool doubles as a reference implementation of the chair's preferred stack,
so the choices carry weight beyond the immediate project.

Key constraints:
- Small team; single-binary deployment preferred.
- Strong typing end-to-end, from database to API to UI.
- API-first: the OpenAPI spec must be the contract; both server and client
  code should be generated from it.
- Low operational overhead: one process, one database.

## Decision

- **Backend language:** Go 1.22+ (`server/`, `go.mod`).
- **Database:** PostgreSQL 15+, managed by CloudNativePG in production
  and by a docker-compose postgres service in development.
- **Frontend:** React 18 + TypeScript, built with Vite.

## Consequences

- Go gives us a single statically-linked binary, excellent concurrency for the
  in-process scheduler, and good tooling for OpenAPI codegen (`oapi-codegen`).
- Postgres is the canonical RDBMS at TUM; CloudNativePG provides
  production-grade HA and backup without bespoke operator code.
- React + TypeScript is familiar to the chair and has mature tooling.
  `@hey-api/openapi-ts` generates a typed client from the same OpenAPI spec
  that drives the Go server stubs, so the two sides stay in sync.
- This combination rules out embedding a different DB (e.g. SQLite) and
  rules out a Go-only SPA approach (e.g. `templ`). Both trade-offs are
  acceptable given the reference-implementation goal.

Locks: REQUIREMENTS D1, D2.
