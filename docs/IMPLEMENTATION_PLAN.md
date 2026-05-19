# Confero — Implementation Plan (v0.1 draft)

**Status:** Draft for review — Phase 4 output
**Repository:** <https://github.com/Mtze/confero>
**Reads with:** [`REQUIREMENTS.md`](./REQUIREMENTS.md), [`DATA_MODEL.md`](./DATA_MODEL.md), [`ARCHITECTURE.md`](./ARCHITECTURE.md)
**Last updated:** 2026-05-19

This plan turns the design into a sequenced list of milestones with
explicit Definitions-of-Done. It is written to be executed by a
human or an AI coding agent. Tests are not an afterthought; they are
a non-negotiable part of each milestone's DoD.

If you are an agent reading this: **complete one milestone at a
time, in order, unless the parallelism notes in §5 explicitly let
you fork**. Never start milestone *N+1* while milestone *N*'s tests
are red.

---

## 1. How to use this plan

- Milestones are numbered M0 → M13 and sized XS/S/M/L.
- Each milestone has: a one-line goal, prerequisites, deliverables,
  required tests, a Definition-of-Done checklist, and the ADRs that
  must be written during it.
- The **Testing pact (§2)** lists invariants every milestone must
  uphold. They are repeated, by reference, in each DoD.
- Cross-cutting concerns (logging, configuration, errors) get the
  same treatment as features: they are explicit milestones with
  their own tests.

A milestone is **done** only when:

1. All listed deliverables exist.
2. All listed tests exist and are passing in CI.
3. The testing pact invariants still hold across the repo.
4. The required ADRs are merged.

If a milestone uncovers an architectural surprise: stop, write an
ADR, update `ARCHITECTURE.md`, then continue. Do not silently
diverge from the design docs.

---

## 2. Testing pact (non-negotiable)

These rules apply across every milestone. Violating them is a
regression even if the new feature works.

### 2.1 Backend (Go)

- **Every service-layer function has at least one unit test** that
  exercises the happy path and at least one error path.
- **Every repository (sqlc-generated query)** is covered by a
  test in `server/tests/repository_test.go` that runs against a
  real Postgres via `testcontainers-go`.
- **Every HTTP endpoint defined in `api/openapi.yaml`** has an
  API test in `server/tests/api/` that spins the full server
  against testcontainers Postgres and exercises:
  - a 2xx happy path,
  - the most plausible 4xx (auth, validation, not-found),
  - one 5xx scenario where feasible (e.g., simulated DB outage).
- **Race detector on** for the full test suite (`go test -race`).
- **No skipping** with `t.Skip` to make CI green; either fix the
  test or delete it.

### 2.2 Frontend (TypeScript)

- **Every page component** has at least one render test verifying
  the happy-path UI given a mocked API response (via MSW).
- **Every form** has a test for: empty submit, valid submit,
  and one server-side error response.
- **Every custom hook** that contains non-trivial logic has a
  test.

### 2.3 API contract

- The OpenAPI spec **lints clean** under `redocly lint`.
- Server stubs and TS client are **regenerated in CI** and the
  diff against committed artifacts must be empty (no "stale
  generated code" merges).

### 2.4 Bruno collection

- Every operation in `api/openapi.yaml` has a corresponding
  `.bru` request in `bruno/confero/`. CI fails if any operation
  is missing.
- The Bruno collection runs headless in CI (`bru run`) against a
  compose-managed stack and must pass before a release tag.

### 2.5 Helm

- `helm lint` and `helm-unittest` pass for every change to the
  chart.
- A snapshot of `helm template` with the canonical values file is
  committed; CI fails on unexpected diffs.

### 2.6 CI gates

- A push to `main` or a PR runs: Go tests, Go race, Go lint, web
  tests, web lint, web typecheck, OpenAPI lint, codegen drift
  check, helm lint, helm-unittest. All must be green.

---

## 3. Repository starting state

Repo: `github.com/Mtze/confero` (empty as of M0 kickoff).

The first commit comes from M0; the repo layout target is the one
documented in [`ARCHITECTURE.md` §2](./ARCHITECTURE.md#2-repository-layout).
Branch model: `main` is always green and deployable; feature
branches are PR-merged with squash.

Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`,
`test:`, `refactor:`) — CI lints commit messages on PRs.

---

## 4. Milestones

### Group A — Foundation

#### M0 · Bootstrap (S)

- **Goal:** Empty repo becomes a buildable, lint-clean monorepo
  with CI passing on a no-op change.
- **Prereqs:** none.
- **Deliverables:**
  - Directory skeleton from `ARCHITECTURE.md §2` (empty packages
    are fine as long as paths exist).
  - `README.md` (quickstart placeholder pointing at `docs/`).
  - `AGENTS.md` and `CLAUDE.md` at the repo root, with the
    repo-wide conventions copied from `ARCHITECTURE.md §13`.
  - `Makefile` with at least: `make help`, `make generate`,
    `make lint`, `make test`, `make build`, `make dev`,
    `make dev-services`.
  - `.editorconfig`, `.gitignore`, `.golangci.yml`,
    `commitlint.config.cjs`.
  - `server/go.mod` (Go 1.22+), `web/package.json` (pnpm),
    empty `api/openapi.yaml` with `openapi: 3.1.0` and a stub
    `info`.
  - `.github/workflows/ci.yaml` running: Go lint, Go test,
    web lint, web typecheck, web build, OpenAPI lint,
    helm lint (no-op on empty chart), commit lint.
  - `docs/` directory containing the four design docs
    (`REQUIREMENTS.md`, `DATA_MODEL.md`, `ARCHITECTURE.md`,
    `IMPLEMENTATION_PLAN.md`).
- **Tests required:**
  - One Go test under `server/internal/version/version_test.go`
    asserting that `version.Version != ""` once `-ldflags` is set
    (use a tiny `t.Skip` fallback only when `Version == ""` in
    local `go test`; CI builds with ldflags).
  - One web test (`web/src/lib/__tests__/sanity.test.ts`)
    asserting `1 + 1 === 2`. Yes, really — proves the Vitest
    pipeline runs.
- **ADRs to write:**
  - `0001-go-postgres-react.md` (locks D1+D2).
  - `0002-openapi-first.md` (locks D2 / NFR-2).
- **DoD:**
  - `make lint && make test && make build` is green on a clean
    checkout.
  - CI is green on a no-op PR.
  - Repo URL is set as `Mtze/confero` (already done).

#### M1 · OpenAPI contract + codegen pipeline (M)

- **Goal:** One round-trippable trivial endpoint
  (`GET /healthz`) is defined in OpenAPI, the server generates
  stubs and serves it, the TS client consumes it.
- **Prereqs:** M0.
- **Deliverables:**
  - `api/openapi.yaml` defining `/healthz` (200 → `Status` object).
  - `server/Makefile` target `generate` running `oapi-codegen` to
    produce `server/internal/api/`.
  - `web/package.json` scripts running `@hey-api/openapi-ts` into
    `web/src/api/`.
  - `cmd/confero-server/main.go` wires chi, mounts the generated
    handler, serves `/healthz`.
  - `Dockerfile` for the server (multi-stage → distroless).
  - GitHub Action job that runs `make generate` and fails if the
    working tree is dirty afterwards (the "no stale generated
    code" gate).
- **Tests required:**
  - **API test** (`server/tests/api/health_test.go`) starts the
    server, hits `/healthz`, asserts the response body matches
    the spec.
  - **TS test** (`web/src/api/__tests__/health.test.ts`) calls
    the generated client against an MSW stub and asserts types.
  - OpenAPI lint passes in CI.
- **ADRs to write:**
  - `0003-codegen-tools.md` (oapi-codegen + hey-api).
- **DoD:**
  - `curl http://localhost:8080/healthz` returns the documented
    body when running the built image.
  - CI's codegen-drift check passes.

#### M2 · Database layer (M)

- **Goal:** Postgres connectivity, migrations, and sqlc are wired,
  with the full schema from `DATA_MODEL.md` applied via
  `golang-migrate`.
- **Prereqs:** M0.
- **Deliverables:**
  - `server/db/migrations/000001..000015` per
    `DATA_MODEL.md §5`. Each migration has matching
    `up.sql`/`down.sql`.
  - `server/db/queries/*.sql` with the minimum query set
    needed for the next milestones (just `users.upsert`,
    `conferences.list`, `conferences.get`, `conferences.create`
    are fine here; more come per feature).
  - `server/sqlc.yaml` with the type overrides from
    `DATA_MODEL.md §6`.
  - `server/internal/database/pool.go` exposing a configured
    `*pgxpool.Pool` and the migration runner (`Run(ctx,
    dsn)` calls `migrate.New(...).Up()`).
  - `make migrate-up`, `make migrate-down`, `make migrate-new
    name=...` targets.
  - The server runs migrations on startup (no separate init
    container).
- **Tests required:**
  - **Migration round-trip test**
    (`server/tests/migrations_test.go`): boots a testcontainers
    Postgres, runs every migration up, then every migration
    down, asserts the schema is empty at the end. Catches any
    `down.sql` that doesn't actually undo its `up.sql`.
  - **Repository smoke test**: creates a conference row via the
    generated sqlc code, reads it back, asserts equality.
  - Race detector still green.
- **ADRs to write:**
  - `0004-golang-migrate-and-sqlc.md` (locks NFR-8).
- **DoD:**
  - Migrations and sqlc-generated code compile.
  - `make test` with `TESTCONTAINERS_RYUK_DISABLED=true` for CI
    runs green.
  - `make migrate-up` against a docker-compose Postgres
    successfully creates every table and index in
    `DATA_MODEL.md §4`.

#### M3 · Auth: OIDC + JWT + middleware (L)

- **Goal:** Users can log in via Keycloak (in docker-compose),
  receive a JWT cookie, and authenticated/authorized endpoints
  work.
- **Prereqs:** M1, M2.
- **Deliverables:**
  - `deploy/compose/docker-compose.yml` with Postgres, Keycloak,
    MailHog, the server, the web (built from current state).
  - `deploy/compose/keycloak/realm-confero.json` seeded realm
    with two groups (`cs-edu-chair`, `cs-edu-chair-admin`),
    two users (`member@example.org`, `admin@example.org`,
    password `confero`).
  - `internal/auth/oidc.go` — provider discovery, code flow
    with PKCE, callback handler, JWT issuance.
  - `internal/auth/tokens.go` — HS256 sign/verify, claim struct
    with `sub`, `email`, `name`, `oidc_sub`, `roles`.
  - `internal/auth/middleware.go` — `RequireToken`,
    `RequireMember`, `RequireAdmin`.
  - `internal/auth.OIDCClaimName` package-level var set via
    `-ldflags`; the Dockerfile exposes `OIDC_CLAIM_NAME` as ARG.
  - `GET /api/v1/me` returning the current user (proves the
    middleware works).
  - User upsert on `/auth/callback`.
- **Tests required:**
  - **Unit tests:** token sign/verify round-trip; expired token
    rejected; tampered token rejected; missing-claim handling.
  - **Unit tests:** roles decoder for each combination of
    member/admin/neither.
  - **API test:** `/api/v1/me` returns 401 without cookie, 200
    with a freshly-issued cookie, 401 with an expired one.
  - **Integration test (compose):** end-to-end OIDC flow against
    real Keycloak. Lives under `server/tests/e2e_auth_test.go`,
    behind a `//go:build compose` tag so it only runs in the
    nightly compose job.
- **ADRs to write:**
  - `0005-jwt-stateless-auth.md` (locks D27).
  - `0006-oidc-claim-name-at-build-time.md` (locks D3a / FR-22a).
- **DoD:**
  - From a fresh `make dev`, logging in as
    `member@example.org` lands on the SPA and `GET /api/v1/me`
    returns `roles: ["member"]`.
  - Same for `admin@example.org` returning
    `roles: ["member","admin"]`.

---

### Group B — Core features

#### M4 · Conferences CRUD + tags + tracks + archive (L)

- **Goal:** Full conference catalog feature: read for everyone,
  create/edit/archive for members, delete for admins. Tag
  autocomplete and tracks are wired.
- **Prereqs:** M2, M3.
- **Deliverables:**
  - OpenAPI: `GET/POST /api/v1/conferences`,
    `GET/PUT/DELETE /api/v1/conferences/{id}`,
    `POST /api/v1/conferences/{id}/archive`,
    `POST /api/v1/conferences/{id}/unarchive`,
    `GET /api/v1/tags`, `GET /api/v1/tracks`.
  - sqlc queries + repositories.
  - `internal/service/conferences.go` with input validation
    (deadline ordering, year sanity, etc.).
  - Tag upsert-by-slug helper.
  - Archive endpoint is idempotent.
  - Bruno requests for every operation.
- **Tests required:**
  - Service-layer unit tests for all validation cases.
  - Repo tests for: insert/get round-trip, list filters
    (archived flag, tag, track, free-text search), uniqueness
    violation on `(LOWER(acronym), year)`.
  - API tests for every endpoint × every auth state (public,
    member, admin, anonymous).
  - **One important test:** `DELETE` as a member returns 403;
    as an admin returns 204 and cascades stars (assert via DB
    query).
  - Bruno collection requests with example bodies committed.
- **ADRs to write:**
  - `0007-flat-trust-with-admin-delete.md` (locks D4 / D22).
- **DoD:**
  - From the SPA running on `make dev`, an admin can create
    SIGCSE 2027, tag it, archive it, unarchive it, then delete
    it. A member can do everything except delete.

#### M5 · Stars + reminder materialization (DB side only) (M)

- **Goal:** Members can star/unstar conferences; pending
  reminder rows get materialized correctly. Emails are *not*
  sent yet — that's M7.
- **Prereqs:** M4.
- **Deliverables:**
  - OpenAPI: `POST/DELETE /api/v1/conferences/{id}/stars`,
    `GET /api/v1/me/stars`,
    `GET /api/v1/me/settings`, `PUT /api/v1/me/settings`.
  - sqlc queries + repositories.
  - `internal/service/stars.go` performs the star write **and**
    the `reminder_dispatch_log` materialization in the same
    transaction.
  - `internal/service/settings.go` updates `user_settings` and
    triggers re-materialization of all the user's pending
    reminders.
  - Recompute logic for conference deadline edits in M4's
    update path (add a hook now).
- **Tests required:**
  - Unit test: given a conference with three set deadlines
    and a user with two lead times, materialization produces
    six rows with the correct `scheduled_for` values.
  - Unit test: un-starring sets `status='cancelled'` on
    pending rows, not on sent ones.
  - Unit test: editing a deadline date causes the user's
    pending row to be cancelled and re-inserted at the new
    time.
  - API tests: star + unstar happy + error paths.
- **DoD:**
  - Toggling a star in the SPA results in the expected number
    of `reminder_dispatch_log` rows (verified via a small
    debug endpoint in dev mode only — never exposed in
    production).

#### M6 · In-process scheduler with a fake mailer (M)

- **Goal:** The scheduler picks up due rows and "sends" them via
  a fake mailer (logs to slog); idempotency and retry behavior
  are testable.
- **Prereqs:** M5.
- **Deliverables:**
  - `internal/scheduler/scheduler.go` with the tick loop,
    `SELECT ... FOR UPDATE SKIP LOCKED`, retry budget, archive
    sweeper.
  - `internal/mail/fake.go` — captures sent messages in memory
    for tests.
  - Wiring in `main.go`: scheduler runs in an `errgroup` and
    shuts down cleanly on SIGTERM.
  - `confero_scheduler_pending_reminders` gauge metric.
- **Tests required:**
  - Test that two scheduler instances racing for the same row
    don't double-send (skip-locked behavior).
  - Test that a transient mailer error increments `attempts`
    and keeps `status='pending'`; a permanent error after N
    attempts sets `status='failed'`.
  - Test that the archive sweeper sets `archived_at` after
    `event_end + grace_days` and cancels pending reminders.
  - Test that a `SIGTERM` triggers a graceful shutdown
    inside ~5 seconds.
- **ADRs to write:**
  - `0008-in-process-scheduler.md` (locks D18, documents the
    extraction path from `ARCHITECTURE.md §15`).
- **DoD:**
  - With clock-faking in tests, all reminder/digest scenarios
    pass.
  - In `make dev`, the scheduler logs "would send" messages
    for due reminders.

---

### Group C — Notifications

#### M7 · SMTP mailer + reminder & digest templates (M)

- **Goal:** Real emails leave the server via SMTP (MailHog in
  dev), with both per-deadline reminder and weekly digest
  formats.
- **Prereqs:** M6.
- **Deliverables:**
  - `internal/mail/smtp.go` — `net/smtp` + STARTTLS.
  - `internal/mail/templates/reminder.{html,txt}.tmpl`.
  - `internal/mail/templates/digest.{html,txt}.tmpl`.
  - `internal/scheduler/digest.go` — weekly digest scheduling
    logic per `ARCHITECTURE.md §8.4`.
  - `digest_dispatch_log` rows scheduled per user-tz/hour.
- **Tests required:**
  - Template render tests with golden files
    (`server/internal/mail/testdata/`).
  - Unit test for the digest scheduler: a user in
    `Europe/Berlin` with `weekly_digest_day=1, hour=8` gets one
    row inserted on Monday at 06:00 UTC (DST sanity).
  - Integration test (compose tag): MailHog receives the
    expected count of messages with the expected subjects after
    triggering the scheduler.
- **DoD:**
  - From `make dev` with the test users having stars,
    triggering the scheduler results in visible mails in
    MailHog with sensible content.

---

### Group D — Calendar feeds

#### M8 · ICS feeds (public + per-user) (M)

- **Goal:** Both calendar feeds are live and stable.
- **Prereqs:** M5.
- **Deliverables:**
  - OpenAPI: `GET /calendar/all.ics`,
    `GET /calendar/u/{token}.ics`,
    `GET/POST/DELETE /api/v1/me/calendar-tokens`.
  - `internal/ical/encoder.go` — minimal RFC 5545 emitter.
  - `internal/calendar/feeds.go` — assembly logic for both feed
    kinds, deterministic `UID`s.
  - Token generation (`crypto/rand`, 32 bytes, base64url).
  - Token regeneration revokes prior live token via the partial
    unique index.
  - Response headers per `ARCHITECTURE.md §7.2`.
- **Tests required:**
  - Unit tests for the ICS encoder: line folding at 75 octets,
    DTSTART formatting, CRLF terminators, escaping of `,`/`;`/`\\`
    in summaries.
  - Snapshot test for both feed payloads with a fixed dataset
    (`server/tests/calendar/testdata/`).
  - Test that calling the token endpoint twice invalidates the
    first URL (404 / 410).
  - Test that an archived conference is **not** in the public
    feed.
  - Test that `If-None-Match` against the current ETag returns
    304.
- **DoD:**
  - Adding the public feed URL to Apple Calendar / Google
    Calendar / Thunderbird shows the events. Adding the
    personal feed URL shows only the test user's starred
    conferences. (Manual verification documented in the
    milestone PR.)

---

### Group E — Admin & extras

#### M9 · Audit middleware + admin endpoints (M)

- **Goal:** Every write produces an audit row; admins can query
  it; non-admins cannot.
- **Prereqs:** M4, M3.
- **Deliverables:**
  - `internal/audit/middleware.go` — `audit.For(entityType,
    action)` middleware per `ARCHITECTURE.md §6.5`.
  - `internal/audit/context.go` — `MarkEntity(ctx, id)`.
  - Routes for `POST`, `PUT`, `DELETE`, archive, unarchive on
    conferences are wrapped.
  - OpenAPI: `GET /api/v1/audit-log?entity_type=...&entity_id=...&actor=...&limit=...&before=...`.
  - Admin-only handler reads from `audit_log` and joins with
    user metadata snapshot fields.
  - `confero_audit_write_failures_total` counter metric.
- **Tests required:**
  - Test that a 2xx PUT writes one audit row with the expected
    actor and action.
  - Test that a 4xx PUT writes **zero** audit rows.
  - Test that a 5xx (forced via a fault-injection middleware
    in tests) writes zero rows.
  - Test that audit-log retry logic retries 3× on transient
    error and gives up with a metric increment.
  - Test that `GET /api/v1/audit-log` returns 403 for a member,
    200 for an admin.
- **ADRs to write:**
  - `0009-audit-via-http-middleware.md` (locks D28).
- **DoD:**
  - A member edits a conference; an admin can see the audit
    row with the member's name and OIDC subject.

#### M10 · Bulk import (YAML, pluggable parser) (S)

- **Goal:** Members can paste a YAML document of conferences and
  have them upserted in one transaction.
- **Prereqs:** M4.
- **Deliverables:**
  - OpenAPI: `POST /api/v1/import` accepting
    `text/x-yaml` (or `application/yaml`); response lists
    `created`, `updated`, `skipped`.
  - `internal/importer/importer.go` — `Importer` interface.
  - `internal/importer/yaml.go` — first implementation.
  - Service-layer upsert keyed by `(LOWER(acronym), year)`.
  - Strict mode (default): any malformed entry aborts the whole
    import; soft mode reports errors per entry without rolling
    back.
- **Tests required:**
  - Parser tests with several YAML fixtures: valid, partial,
    invalid.
  - End-to-end: import 10 conferences in strict mode, verify
    row count.
  - End-to-end: import same payload twice, verify upsert (no
    duplicates).
- **DoD:**
  - A documented YAML schema in `docs/IMPORT_FORMAT.md`.
  - `bru run` of the import collection passes in CI.

---

### Group F — Frontend

#### M11 · React SPA feature parity (XL)

- **Goal:** The SPA implements every member-facing capability
  the API exposes.
- **Prereqs:** M1 (codegen). Strictly, can begin in parallel
  with M3+ as long as a mock API is used; final acceptance
  needs M3..M9.
- **Deliverables:**
  - Pages: `Home` (public list with filters, search, sort,
    show-past toggle), `ConferenceDetail`, `MyStars`,
    `Settings`, `CalendarTokens`, `Audit` (admin-only),
    `NotAuthorized`.
  - Components: Conference card, tag autocomplete using
    `GET /api/v1/tags`, lead-time editor, deadline countdown.
  - React Query setup with the generated SDK + cookie-credentials.
  - Tailwind + Radix UI; tasteful, restrained design.
- **Tests required:**
  - Component tests for every page with MSW-stubbed APIs.
  - Form tests for: create conference, edit settings, regenerate
    calendar token.
  - Snapshot test for the conference card's "deadline countdown"
    component covering: future-far, future-near, past, AoE-edge.
  - Accessibility test (`@testing-library/jest-axe`) for at
    least the Home and ConferenceDetail pages.
- **DoD:**
  - A new member can: log in, see the public list, star a
    conference, configure reminder lead times, subscribe to
    their personal calendar feed, see their stars in "My
    Stars".
  - An admin can additionally: delete a conference and view
    the audit log.

---

### Group G — Deployment

#### M12 · Helm chart + CNPG deployment (L)

- **Goal:** A single `helm install confero ./deploy/helm/confero
  -f values.yaml` produces a working stack in a Kubernetes
  cluster with the CNPG operator pre-installed.
- **Prereqs:** M4 (functioning server + web images).
- **Deliverables:**
  - `deploy/helm/confero/Chart.yaml` (`appVersion`,
    `version`).
  - `templates/` per `ARCHITECTURE.md §11`.
  - `values.yaml` with every knob from `ARCHITECTURE.md §11.3`.
  - `values.schema.json` validating types.
  - Default `server.replicaCount: 1` with a `Recreate`
    strategy and a `NOTES.txt` warning that bumping it will
    cause duplicate scheduling work (the unique constraint
    still guards correctness).
  - CNPG `Cluster` resource templated.
  - Probes: `/healthz` (liveness), `/readyz` (readiness).
  - Network policy templates (optional toggle).
- **Tests required:**
  - `helm lint` clean.
  - `helm-unittest` cases for: TLS on/off, OIDC values
    required, SMTP optional, replicaCount=1 enforced.
  - Snapshot of `helm template` with the canonical values
    file is committed; CI fails on unexpected diffs.
  - Smoke test on a kind/minikube CI runner (optional;
    nightly): install the chart, wait for readiness, hit
    `/healthz`.
- **ADRs to write:**
  - `0010-cloudnativepg.md` (locks D15).
  - `0011-helm-chart-layout.md`.
- **DoD:**
  - Documented `helm install` instructions in
    `deploy/helm/confero/README.md` work end-to-end on a
    fresh cluster.

---

### Group H — Polish

#### M13 · Docs, ADRs, OpenAPI site, release 0.1.0 (M)

- **Goal:** The project is genuinely usable by a stranger.
- **Prereqs:** M0..M12.
- **Deliverables:**
  - `README.md` (real, not placeholder): pitch, screenshot,
    quickstart, deploy, contribute.
  - `AGENTS.md` reviewed and tightened with concrete recipes
    ("how to add a field to Conference", "how to add a new
    endpoint", "how to add a migration").
  - All ADRs from earlier milestones merged.
  - `docs/openapi/` Redoc site generation in CI, published to
    GitHub Pages.
  - `CHANGELOG.md` seeded.
  - Tagged release `v0.1.0` triggers `release.yaml` (image
    push, chart push, docs publish).
- **Tests required:**
  - Spell-check the docs (`make spellcheck` via `cspell` or
    similar — optional but nice).
  - Manual test: walk through the README from scratch on a new
    machine and produce no errors.
- **DoD:**
  - A reader who has never seen the repo can get the dev stack
    running by following the README in under 10 minutes.

---

## 5. Parallelism options

You can fan out work across milestones if you keep dependencies
honest:

| Track   | Can start after | Independent of  |
| ------- | --------------- | --------------- |
| Frontend (M11) | M1                | Backend feature ordering, as long as you mock APIs |
| Helm chart (M12) | M4              | M5..M10 (chart only needs images) |
| Bulk import (M10) | M4             | M5..M9          |
| Calendar (M8)    | M5 (stars exist) | M6..M7 (no scheduler needed) |

In practice an agent should do M0 → M3 strictly sequentially
(they layer too tightly to parallelize), then fan out from M4.

---

## 6. Cross-cutting work that is not its own milestone

Some chores must happen *during* the milestones above, not at the
end:

- **Logging.** Every new package adds at least one `slog` line at
  the right level on its public entry points. Sensitive values
  (tokens, secrets, full JWTs) never appear in logs.
- **Metrics.** Each milestone that adds an externally-visible
  operation must add or update Prometheus metrics (per
  `ARCHITECTURE.md §9.2`).
- **Error envelope.** All HTTP errors use the RFC 7807 envelope
  introduced in M1; new endpoints must conform.
- **ADRs.** Any decision not already captured in the design docs
  gets an ADR in the same PR.

---

## 7. After v1

The decision logs in `REQUIREMENTS.md` already list what's out of
scope (§5 there). The two most-likely follow-up tracks:

- **Extract the scheduler** into its own Deployment so the API
  server can scale horizontally. Touches M6's package and the
  Helm chart only.
- **TOON import format.** Drop a second `Importer` impl in
  `internal/importer/toon.go`; no other changes required.

These get their own plans when they come due.

---

## 8. Open questions in this plan

- **Nightly E2E smoke tests** on kind/minikube — proposed in M12;
  confirm whether the chair's CI budget covers a daily 5-minute
  cluster spin.
- **OpenAPI docs hosting** — GitHub Pages from `docs/openapi/`
  is proposed in M13; confirm or switch to serving from the
  web image at `/docs`.
- **Frontend a11y depth** — `jest-axe` on two pages is the
  current target; if you want a higher bar (e.g., axe on every
  page), I'll bump it in the testing pact.
