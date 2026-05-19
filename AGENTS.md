# AGENTS.md — repo-wide conventions for AI coding agents

> Audience: any LLM or autonomous coding agent working in this repo.
> Humans should also read this — the rules are not LLM-specific, they
> are just written down explicitly so an agent can follow them
> without inferring them from style.

If you are reading this because you have been asked to make a
change in this repo: **read this whole file first, then
[`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md), then the
[`docs/IMPLEMENTATION_PLAN.md`](./docs/IMPLEMENTATION_PLAN.md)
section for the current milestone, before touching any code.**

---

## 1. The hard rules

These are non-negotiable. CI enforces most of them; reviewers
enforce the rest.

1. **The spec is the contract.** `api/openapi.yaml` is the single
   source of truth for the HTTP API. Any change to the API begins
   with editing that file. Server stubs (`server/internal/api/`)
   and the TypeScript client (`web/src/api/`) are *generated*
   from it.
2. **Never edit generated code.** Files under
   `server/internal/api/`, `server/internal/repository/`, and
   `web/src/api/` are produced by code generators. Edit the
   inputs (the OpenAPI spec, the SQL queries, the sqlc config)
   and re-run `make generate`.
3. **Run `make generate` before committing.** CI fails on a dirty
   working tree after generation. Stale generated code is not
   merged.
4. **Respect layer boundaries** (see ARCHITECTURE §4.2):
   `transport (handlers) → service → repository`. Handlers do
   not write SQL. Services do not write HTTP responses.
   Repositories contain no business rules.
5. **No skipping tests to make CI green.** `t.Skip` is reserved
   for "this environment cannot run this test at all" (e.g., a
   compose-only integration test in a unit-test run). If a test
   is broken, fix it or delete it — do not silently bypass it.
6. **Every milestone's DoD is binding.** Do not start milestone
   *N+1* while milestone *N*'s tests are red. The plan in
   `docs/IMPLEMENTATION_PLAN.md` is sequenced for a reason.
7. **Decisions get ADRs.** If you encounter an architectural
   surprise that isn't already captured in
   `docs/ARCHITECTURE.md` or in an existing ADR, stop and write
   one (`docs/adr/NNNN-title.md`, Michael Nygard format). Do
   not silently diverge from the design docs.
8. **Conventional Commits.** Every commit message starts with
   `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`,
   `ci:`, or `build:`, optionally scoped (e.g.
   `feat(server): ...`). `commitlint` runs in CI.
9. **No secrets in git.** Not in code, not in chart values, not
   in test fixtures. Use env vars or Kubernetes Secrets. The
   local dev compose has obviously-fake values.
10. **PII minimization in audit/logging.** The audit log stores
    name + OIDC subject + action + timestamp only — no email,
    no IP, no User-Agent, no field-level diffs. Logs never
    contain full tokens, full session JWTs, or SMTP passwords.

---

## 2. Repository layout — where things live

The full tree is in
[`docs/ARCHITECTURE.md §2`](./docs/ARCHITECTURE.md#2-repository-layout).
The shortest possible map:

| Path                                | Contains                                              |
| ----------------------------------- | ----------------------------------------------------- |
| `api/openapi.yaml`                  | The HTTP API. Edit here first, always.                |
| `server/cmd/confero-server/`        | The only Go binary. Wires everything up.              |
| `server/internal/api/`              | **Generated.** OpenAPI server stubs. Do not edit.     |
| `server/internal/repository/`       | **Generated.** sqlc query types. Do not edit.         |
| `server/internal/service/`          | Business logic. Where features actually live.         |
| `server/internal/http/`             | Routing, middleware, error mapping.                   |
| `server/internal/auth/`             | OIDC, JWT, RBAC middleware.                           |
| `server/internal/scheduler/`        | The in-process worker (reminders, digests, archive).  |
| `server/internal/mail/`             | Mailer interface + SMTP impl + templates.             |
| `server/internal/ical/`             | Tiny RFC 5545 encoder, no external deps.              |
| `server/internal/calendar/`         | Feed assembly using `ical/`.                          |
| `server/internal/audit/`            | Audit middleware + context helper.                    |
| `server/internal/importer/`         | YAML/TOON bulk-import parsers.                        |
| `server/internal/version/`          | `Version`, `Commit` — set via `-ldflags`.             |
| `server/db/migrations/`             | `*.up.sql` / `*.down.sql` for golang-migrate.         |
| `server/db/queries/`                | sqlc input. Edit here; run `make generate`.           |
| `web/src/api/`                      | **Generated.** TS SDK from OpenAPI. Do not edit.      |
| `web/src/features/`                 | Feature folders (conferences, stars, settings, ...).  |
| `web/src/pages/`                    | Route-level components.                               |
| `web/src/components/`               | Generic UI primitives.                                |
| `bruno/confero/`                    | Bruno API collection (one `.bru` per operation).      |
| `deploy/compose/`                   | docker-compose for local dev (Postgres, Keycloak, MailHog, ...). |
| `deploy/helm/confero/`              | The production Helm chart.                            |
| `docs/`                             | All design docs and ADRs.                             |
| `.github/workflows/`                | CI / release workflows.                               |

`internal/` is genuinely internal — nothing outside `server/` imports it.

---

## 3. The four design docs

These are the source of truth. The code is downstream.

1. [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) — *what* the
   system does. Stakeholders, functional + non-functional
   requirements, scope, locked decisions log.
2. [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — *how* the
   system is built. Components, package responsibilities,
   interfaces, deployment topology.
3. [`docs/DATA_MODEL.md`](./docs/DATA_MODEL.md) — the database
   schema and the rationale for it.
4. [`docs/IMPLEMENTATION_PLAN.md`](./docs/IMPLEMENTATION_PLAN.md)
   — the milestone-by-milestone build order, each with a
   Definition-of-Done.

ADRs in `docs/adr/` lock specific decisions in Michael Nygard
format (`Status`, `Context`, `Decision`, `Consequences`).

---

## 4. The standard workflow

For any non-trivial change:

1. **Read** the relevant design-doc section and the matching
   milestone in `IMPLEMENTATION_PLAN.md`.
2. **If the API changes**, edit `api/openapi.yaml` first. Run
   `make generate`. Commit the generated drift as part of the
   same PR.
3. **If the DB changes**, add a new migration pair in
   `server/db/migrations/` and (if needed) update queries in
   `server/db/queries/`. Run `make generate` for sqlc.
4. **Implement** in the service layer, then wire the handler.
5. **Add tests** at every layer the change touches (see §5).
6. **Update the Bruno collection** for any new or changed
   endpoint.
7. **Add or update an ADR** if you made a decision the design
   docs didn't already cover.
8. **Update logs/metrics** per ARCHITECTURE §9 (every new
   externally-visible operation gets at least one slog line and
   the relevant Prometheus metric).
9. **Run `make lint && make test && make build`** locally before
   pushing.

---

## 5. Testing pact (recap)

See [`docs/IMPLEMENTATION_PLAN.md §2`](./docs/IMPLEMENTATION_PLAN.md#2-testing-pact-non-negotiable)
for the full pact. The summary you must internalize:

- Backend tests run with `-race`.
- Every service function: at least one happy + one error path.
- Every repository query: a real-Postgres test via
  testcontainers.
- Every HTTP endpoint: an API test covering 2xx + a 4xx + (where
  feasible) a 5xx.
- Every page and form on the frontend: a Vitest test, MSW for
  mocked API responses.
- The OpenAPI spec lints clean (`redocly lint`).
- `make generate` produces no diff in CI.

`t.Skip` is reserved for "this test cannot physically run in this
context" (e.g., `//go:build compose` integration tests). It is
never used as a workaround for "the test is flaky" or "the test
is broken".

---

## 6. How to add a feature (recipe)

A common shape that fits almost every feature in this repo:

1. Read the requirement (FR-* / NFR-*) and the affected ADR.
2. Update `api/openapi.yaml` — paths, schemas, responses,
   security.
3. Add a migration if the DB shape changes.
4. Add or update queries in `server/db/queries/*.sql`.
5. Run `make generate` (oapi-codegen + sqlc +
   `@hey-api/openapi-ts`).
6. Implement a `Service` method with validation + business
   logic.
7. Implement the generated `ServerInterface` method in
   `internal/http/handlers/...` — translate request → service
   call → response.
8. Add middleware on the route if it is auth- or audit-gated:
   `auth.RequireMember`, `auth.RequireAdmin`,
   `audit.For("entity", "action")`.
9. Add tests:
   - service unit test (happy + error),
   - repo test (testcontainers Postgres),
   - API test (`server/tests/api/`).
10. Add a Bruno request under `bruno/confero/`.
11. Wire it into the SPA: a `features/<feature>/` folder, a
    page or page-section, a form with `react-hook-form` + zod,
    component + form tests with MSW.
12. Update logs/metrics, update docs if any contract widened.

---

## 7. Quirks and pitfalls

- **The OIDC claim name is baked at build time** via
  `-ldflags -X confero/internal/auth.OIDCClaimName=...`. The
  default is `groups`. You can't override it at runtime. The
  Dockerfile exposes `OIDC_CLAIM_NAME` as an ARG for
  rebuilds. See ADR 0006 (when written in M3) and FR-22a.
- **Stars + reminders are transactional.** When you change the
  star service or the conference update path, you also change
  reminder materialization (see ARCHITECTURE §8.2). They must
  commit together.
- **Audit writes happen after a 2xx response.** A 4xx/5xx
  produces zero audit rows. If your handler intentionally
  returns 2xx for a no-op (e.g., archive-already-archived),
  decide whether you want that to audit; the default is "yes,
  it audits".
- **Calendar tokens are bearer credentials.** Don't log them in
  full — log a prefix. The handler that creates them is the
  only code path that ever returns the plaintext.
- **`server.replicaCount` is locked at 1 in v1** because the
  scheduler runs in-process. The Helm chart's `NOTES.txt` warns
  about this. Extracting the scheduler is a documented
  evolution path (see ARCHITECTURE §15).
- **One Go module.** Don't add a second `go.mod`. The web image
  uses nginx, not a Go binary.
- **`testcontainers-go`** is the only sanctioned way to test
  against Postgres. Don't introduce a mock repository — sqlc
  gives us a concrete type per query group; mocking it would
  lose real-SQL coverage (ARCHITECTURE §4.4).

---

## 8. When in doubt

Default to the design docs, not to "what's conventional
elsewhere". If the docs are unclear or out of date, fix the docs
first — the code is downstream of them.

If a decision the docs don't cover comes up, write an ADR in the
same PR that introduces the decision. Don't bury new direction in
implementation.
