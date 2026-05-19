# Confero — Claude Code Brief

> **You are picking up an in-flight project.** Four design documents
> in `docs/` have been written, reviewed, and locked. The repository
> has been bootstrapped with the design docs only. Your job is to
> implement Confero against those docs, milestone by milestone,
> with tests as a non-negotiable part of every milestone's
> Definition-of-Done.

---

## 1. What Confero is

Confero is a small internal tool for the Computer Science Education
chair at TU München. It tracks upcoming CS-Ed conferences, lets chair
members star the ones they intend to submit to, sends email
reminders for their deadlines, and exposes ICS calendar feeds (one
public, one per-user for starred conferences). The name comes from
Latin *cōnferō*, "to bring together / discuss / contribute" — the
etymological root of *conference*.

The architecture is deliberately small but production-quality: it
doubles as a reference implementation of the chair's preferred
stack.

---

## 2. Read this before anything else

Do **not** start writing code until you have read these four
documents in order:

1. `docs/REQUIREMENTS.md` — what the system must do, who can do
   what, the locked decisions log (D1–D28).
2. `docs/DATA_MODEL.md` — Postgres schema, indexes, migrations,
   sqlc conventions.
3. `docs/ARCHITECTURE.md` — repo layout, OpenAPI contract, server
   layering, auth, scheduler, containers, Helm, observability,
   testing strategy.
4. `docs/IMPLEMENTATION_PLAN.md` — the milestones (M0–M13) with
   testing pact, parallelism rules, and per-milestone
   Definitions-of-Done.

These four documents are the **source of truth**. Code is
downstream of them. If reality diverges from what they say, **stop
and update the relevant doc first** (and add an ADR in
`docs/adr/`).

---

## 3. Current state of the repository

- Branch: `main` (no other branches; we don't use feature branches
  in v1 per user preference).
- Commits so far:
  - `Initial commit` — empty `README.md` only.
  - `docs: import design docs from planning phase` — the four
    documents above.
- **Nothing else is implemented yet.** The next thing to do is
  M0 (Bootstrap) from `docs/IMPLEMENTATION_PLAN.md §4`.

---

## 4. How you work in this repo

### 4.1 Milestone discipline

- **Do exactly one milestone at a time, in order**, unless
  `docs/IMPLEMENTATION_PLAN.md §5` (Parallelism options) explicitly
  permits otherwise.
- A milestone is **done** only when every Definition-of-Done item
  in `docs/IMPLEMENTATION_PLAN.md` for that milestone is checked,
  including its tests.
- Never start milestone *N+1* while milestone *N*'s tests are red.

### 4.2 Testing pact (non-negotiable)

Repeating, with emphasis, the rules from
`docs/IMPLEMENTATION_PLAN.md §2`:

- Every service-layer function has a unit test (happy + at least
  one error path).
- Every sqlc query is covered by a repository test against a
  real Postgres via `testcontainers-go`.
- Every HTTP endpoint defined in `api/openapi.yaml` has an API
  test that runs the full server against testcontainers Postgres.
- `go test -race` is mandatory. No `t.Skip` to make CI green.
- Every page component has a render test (Vitest + MSW). Every
  form has empty-submit / valid-submit / server-error tests.
- Every operation in `api/openapi.yaml` has a corresponding
  Bruno request in `bruno/confero/`. CI fails on missing ones.
- The OpenAPI spec lints clean (`redocly lint`); generated code
  must not drift from the spec (CI runs `make generate` and
  fails on a dirty working tree).
- Helm chart: `helm lint` + `helm-unittest` + snapshot of
  `helm template` with the canonical values file.

### 4.3 Commit conventions

- Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`,
  `test:`, `refactor:`, `ci:`, `build:`. Scope is optional but
  encouraged (`feat(conferences):`, `fix(scheduler):`,
  `docs(adr):`).
- **One logical change per commit.** Examples:
  - `chore(repo): add Makefile, .editorconfig, .gitignore`
  - `chore(server): bootstrap Go module with version package`
  - `chore(web): bootstrap Vite + TypeScript skeleton`
  - `ci: add GitHub Actions workflow`
  - `feat(api): add OpenAPI 3.1 stub`
- Commits compile and pass the relevant tests on their own
  wherever feasible. The repo should be bisectable.
- Body explains *why*, not *what* — the diff already says what.

### 4.4 Branching

- `main` only. No feature branches, no PRs in v1.
- The user wants direct-to-`main` work with disciplined commits.
- That makes commits the unit of review — write good messages.

### 4.5 ADRs

Any architectural decision not already documented in
`docs/REQUIREMENTS.md`, `docs/DATA_MODEL.md`, or
`docs/ARCHITECTURE.md` gets a new ADR under `docs/adr/NNNN-title.md`
in the same commit that introduces it. Format: Michael Nygard's
short template (Status / Context / Decision / Consequences). The
implementation plan already enumerates which ADRs each milestone
must produce.

---

## 5. Repo layout (target)

This is the target shape from `docs/ARCHITECTURE.md §2`. Create
directories lazily as milestones need them. **Generated code paths
are gitignored and never edited by hand.**

```
confero/
├── README.md
├── AGENTS.md, CLAUDE.md          # this file is CLAUDE.md
├── Makefile
├── .editorconfig, .gitignore, .golangci.yml, commitlint.config.cjs
├── api/openapi.yaml              # SINGLE SOURCE OF TRUTH for the API
├── server/
│   ├── cmd/confero-server/main.go
│   ├── internal/
│   │   ├── api/                  # GENERATED (oapi-codegen) — do not edit
│   │   ├── audit/ auth/ calendar/ config/ database/
│   │   ├── http/ ical/ importer/ mail/
│   │   ├── repository/           # GENERATED (sqlc) — do not edit
│   │   ├── scheduler/ service/ version/
│   ├── db/{migrations,queries}/  # *.up.sql/*.down.sql and *.sql for sqlc
│   ├── sqlc.yaml, go.mod, go.sum, Dockerfile
│   └── tests/                    # integration & API tests
├── web/
│   ├── src/api/                  # GENERATED (@hey-api/openapi-ts) — do not edit
│   ├── src/{components,features,hooks,lib,pages}/
│   ├── package.json, vite.config.ts, tsconfig.json, Dockerfile
├── bruno/confero/                # API test collection (one .bru per operation)
├── deploy/
│   ├── compose/{docker-compose.yml,keycloak/realm-confero.json,...}
│   └── helm/confero/{Chart.yaml,values.yaml,values.schema.json,templates/}
├── docs/
│   ├── REQUIREMENTS.md, DATA_MODEL.md, ARCHITECTURE.md, IMPLEMENTATION_PLAN.md
│   ├── adr/NNNN-*.md
│   └── openapi/                  # generated Redoc site
└── .github/workflows/{ci.yaml,release.yaml}
```

---

## 6. Tech stack (locked)

| Area              | Choice                                            | Doc reference         |
| ----------------- | ------------------------------------------------- | --------------------- |
| Backend language  | Go 1.22+                                          | REQUIREMENTS D1       |
| HTTP router       | `go-chi/chi/v5`                                   | ARCHITECTURE §4.2     |
| OpenAPI codegen   | `oapi-codegen/oapi-codegen/v2` (server)           | ARCHITECTURE §3.2     |
| DB driver         | `jackc/pgx/v5` via `pgxpool`                      | DATA_MODEL §6         |
| Queries           | `sqlc-dev/sqlc` (pgx/v5 driver)                   | REQUIREMENTS NFR-8    |
| Migrations        | `golang-migrate/migrate/v4`                       | REQUIREMENTS NFR-8    |
| OIDC              | `coreos/go-oidc/v3` + `golang.org/x/oauth2`       | ARCHITECTURE §6.1     |
| JWT               | `golang-jwt/jwt/v5` (HS256)                       | ARCHITECTURE §6.3     |
| Logging           | stdlib `log/slog` (JSON handler)                  | ARCHITECTURE §9.1     |
| Metrics           | `prometheus/client_golang`                        | ARCHITECTURE §9.2     |
| Mail              | stdlib `net/smtp` + STARTTLS                      | ARCHITECTURE §7.1     |
| ICS               | in-repo encoder (`internal/ical`), no dep         | ARCHITECTURE §7.2     |
| Test container    | `testcontainers/testcontainers-go`                | IMPLEMENTATION §2.1   |
| Frontend          | React 18 + TypeScript + Vite                      | ARCHITECTURE §5.1     |
| Client codegen    | `@hey-api/openapi-ts`                             | ARCHITECTURE §3.2     |
| Routing           | `react-router` v6                                 | ARCHITECTURE §5.1     |
| Data fetching     | `@tanstack/react-query`                           | ARCHITECTURE §5.1     |
| Styling           | Tailwind CSS + `@radix-ui/*`                      | ARCHITECTURE §5.1     |
| Forms             | `react-hook-form` + `zod`                         | ARCHITECTURE §5.1     |
| Component testing | Vitest + React Testing Library + MSW              | IMPLEMENTATION §2.2   |
| API testing       | Bruno (`bruno-cli` for headless runs)             | IMPLEMENTATION §2.4   |
| DB in prod        | CloudNativePG-managed Postgres 15+                | REQUIREMENTS D15      |
| Identity provider | Keycloak (provider-agnostic OIDC code)            | REQUIREMENTS D3       |
| Deployment        | Helm chart with CNPG `Cluster` resource           | REQUIREMENTS D12      |
| Server image      | `gcr.io/distroless/static:nonroot`                | REQUIREMENTS D16      |
| Web image         | `nginxinc/nginx-unprivileged:1.27-alpine`         | REQUIREMENTS D16      |

---

## 7. Hard rules

These are the rules that protect the architecture from drift. Do
not break them silently; if you must, write an ADR.

1. **`api/openapi.yaml` is edited first.** Any API change starts
   there. Then `make generate`. Then handlers. Then tests. Then
   Bruno requests. Never the other way around.
2. **Never edit generated code.** `server/internal/api/`,
   `server/internal/repository/`, and `web/src/api/` are output
   of code generators. They are gitignored (committed-but-ignored
   in CI's drift check). Edits go to the source.
3. **Respect layer boundaries**: transport → service →
   repository. Handlers contain no business logic; services
   contain no SQL; repositories contain no business rules.
4. **OIDC claim *name* is build-time only.** Set via
   `-ldflags "-X confero/internal/auth.OIDCClaimName=<name>"`,
   default `groups`. The Dockerfile exposes it as `ARG
   OIDC_CLAIM_NAME`. The claim *values* (member, admin) are
   runtime env vars.
5. **Stateless auth.** The server does not read sessions from
   Postgres on every request. After the OIDC callback we issue
   a short-lived HS256 JWT in an HttpOnly cookie and verify it
   cryptographically per request. The only DB hit for auth is
   the user upsert at `/auth/callback`.
6. **Audit log is written by HTTP middleware on 2xx.** Service
   code does not write audit rows directly. `audit.For(entity,
   action)` wraps routes; handlers call `audit.MarkEntity(ctx,
   id)` when they know the id. PII-minimal: only actor
   name + OIDC subject + action + timestamp.
7. **Reminders are materialized.** Inserting/cancelling rows in
   `reminder_dispatch_log` happens *in the same transaction* as
   the underlying change (star, unstar, deadline edit, archive).
8. **Server runs single-replica in v1** because the scheduler is
   in-process. The Helm chart enforces `replicaCount: 1` with a
   `Recreate` strategy.
9. **Tests are non-negotiable.** See §4.2.
10. **Do not introduce dependencies without good reason.** Every
    new dep in `go.mod` or `package.json` adds to the security
    surface. Prefer stdlib where reasonable; in-repo small
    implementations (like `internal/ical`) over bringing in
    half-mature libraries.

---

## 8. Per-milestone workflow

For each milestone, in order:

1. Re-read its section in `docs/IMPLEMENTATION_PLAN.md`.
2. Open the relevant sections of `REQUIREMENTS.md`,
   `DATA_MODEL.md`, `ARCHITECTURE.md` for context.
3. **Plan**: write a brief task list (TodoWrite-style). Include
   the tests required by the testing pact and by the
   milestone's DoD.
4. **Implement** in small logical commits. Each commit:
   - compiles
   - passes its own tests (or doesn't worsen the suite)
   - has a Conventional-Commits subject and a body explaining
     *why*
5. **Tests**: write them alongside or before the code. Do not
   batch tests at the end.
6. **Lint and run the full suite** before the final commit of
   the milestone.
7. **Write the ADRs** the milestone requires.
8. **Verify the DoD** point by point. Don't claim "done" until
   all items are objectively true.
9. Tell the user the milestone is complete and what changed,
   then wait for sign-off before starting the next one.

---

## 9. Commands you'll use a lot

These targets live in the `Makefile` (which M0 creates):

```bash
make help            # list available targets
make generate        # regenerate OpenAPI server stubs + TS client
make lint            # golangci-lint + pnpm lint + redocly lint + helm lint
make test            # go test ./... -race + pnpm test + helm-unittest
make build           # build server & web images
make dev             # docker compose up everything for local dev
make dev-services    # bring up only Postgres + Keycloak + MailHog
make migrate-up      # apply all migrations
make migrate-down    # roll back one migration
make migrate-new name=add_something  # create paired up/down sql files
```

If a target you need doesn't exist yet, the relevant milestone
should add it.

---

## 10. Claude Code–specific guidance

### 10.1 Use subagents for genuinely parallel work

After M3 (auth), frontend (M11) and backend feature work can
proceed in parallel — they communicate only via
`api/openapi.yaml`. Delegate the frontend track to a subagent
with a clean prompt; keep the backend in your main context.

Don't fork before M3 — earlier milestones layer too tightly.

### 10.2 Prefer reading a whole file once

`docs/ARCHITECTURE.md` is long but tightly cross-referenced.
Read it end-to-end on first launch rather than skimming for
sections; you will need most of it eventually and re-reading
costs more turns.

### 10.3 Slash commands worth setting up

In `.claude/commands/` (create as you need them):

- `/lint` → `make lint`
- `/test` → `make test`
- `/generate` → `make generate && git status` (catch codegen drift early)
- `/milestone-status` → grep this file + `docs/IMPLEMENTATION_PLAN.md`
  for the current milestone

### 10.4 Hooks worth installing

A `pre-commit` hook (in `.git/hooks/` or via `husky`/`lefthook`
once we have JS tooling) that runs `make lint` and refuses
commits with codegen drift. Optional but saves CI cycles.

### 10.5 When you're unsure

If a requirement is ambiguous or a design decision needs to be
made:

1. Stop. Don't paper over it with a guess.
2. Re-read the relevant design doc section.
3. If still unclear, ask the user — don't invent.
4. When the decision is made, capture it as an ADR.

The user has been deliberate about decisions all the way through
Phase 1–4. Respect that by surfacing ambiguity rather than
resolving it silently.

---

## 11. What's next: start M0

Open `docs/IMPLEMENTATION_PLAN.md §4` → **M0 · Bootstrap**.
Deliverables include:

- Directory skeleton from `docs/ARCHITECTURE.md §2`.
- `README.md` (placeholder pointing at `docs/`).
- `AGENTS.md` (durable repo-wide agent guide; can re-use this
  CLAUDE.md as a starting point — see §12).
- `Makefile` with the targets in §9.
- `.editorconfig`, `.gitignore`, `.golangci.yml`,
  `commitlint.config.cjs`.
- `server/go.mod` (Go 1.22+).
- `web/package.json` (pnpm).
- `api/openapi.yaml` stub (`openapi: 3.1.0` + `info`).
- `.github/workflows/ci.yaml` running: Go lint+test, web
  lint+typecheck+build, OpenAPI lint, helm lint, commitlint.
- Two tests (one Go, one web) so the test pipelines fire.
- ADR-0001 (Go+Postgres+React) and ADR-0002 (OpenAPI-first).

The DoD: `make lint && make test && make build` is green on a
clean checkout, and CI is green on a no-op PR (we don't use PRs,
but the workflow must run cleanly on push).

When M0 is done, tell the user, then wait before starting M1.

---

## 12. About this file and AGENTS.md

Per `docs/ARCHITECTURE.md §13`, the long-term convention is:

- `AGENTS.md` — durable repo-wide conventions for **any** AI
  agent (Claude Code, Cursor, Aider, custom agents). **Already
  exists in the repo root** — read it after this file.
- `CLAUDE.md` (this file) — comprehensive Claude Code brief that
  doubles as the first-load context. Overlaps with `AGENTS.md`
  on purpose; the redundancy is cheap and the load order is
  reliable.

If the two ever drift, `AGENTS.md` is the durable doc and wins.
`CLAUDE.md` is allowed to be slightly more conversational and to
include Claude Code–specific tooling notes (slash commands,
subagent guidance, hooks) that don't belong in a general-agent
file.

---

Good luck. The hard thinking is already in `docs/`; your job is
disciplined execution against it.
