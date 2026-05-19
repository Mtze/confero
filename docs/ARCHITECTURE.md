# Confero — Architecture (v0.1 draft)

**Status:** Draft for review — Phase 3 output
**Reads with:** [`REQUIREMENTS.md`](./REQUIREMENTS.md), [`DATA_MODEL.md`](./DATA_MODEL.md)
**Last updated:** 2026-05-19

This document is the canonical architecture reference for Confero.
It is written to be useful for both humans and AI agents: every
component has a defined responsibility, a defined location in the
repo, and a defined contract with its neighbours.

If a section is unclear or out of date, fix it here first — the
code is downstream of this doc.

---

## 1. System overview

Confero is a small but real-world distributed system: a public web
client, a Go API server with an in-process worker, a Postgres
database, an SMTP relay, and an OIDC identity provider. The
diagram below shows the runtime topology in production (Kubernetes
via Helm); local development mirrors it with docker-compose.

```mermaid
flowchart LR
    user([Chair member / public visitor])
    browser([Browser])
    cal([Calendar client])

    user --> browser
    user --> cal

    subgraph k8s[Kubernetes cluster]
        ingress[/Ingress + TLS/]
        web[confero-web<br/>nginx-unprivileged]
        api[confero-server<br/>distroless static<br/>in-process scheduler]
        pg[(CloudNativePG<br/>Postgres 15)]
    end

    smtp[SMTP relay<br/>e.g. mailout.tum.de]
    idp[OIDC provider<br/>Keycloak]

    browser --> ingress
    cal     --> ingress
    ingress --> web
    ingress --> api
    api  -->|sqlc / pgx| pg
    api  -->|net/smtp| smtp
    api  <-->|OIDC code flow| idp
    browser <-->|redirect login| idp
```

Key properties:

- **Two container images.** `confero-server` (Go API + scheduler)
  ships as `gcr.io/distroless/static:nonroot`; `confero-web`
  (Vite-built SPA) ships as `nginxinc/nginx-unprivileged:alpine`.
  Both are built multi-stage.
- **One contract.** `api/openapi.yaml` is the single source of
  truth. Go server stubs and the TypeScript client are *both*
  generated from it. Bruno consumes the same spec.
- **Single API replica in v1** because the scheduler runs in-process.
  The chart's default `server.replicaCount` is `1` and the doc
  spells out why; extracting the scheduler is a documented
  evolution path (see §15).
- **Multiple web replicas** are safe; the SPA bundle is stateless.

---

## 2. Repository layout

A single repository with clear sub-trees per concern. AI agents
working in this repo should treat the boundaries between top-level
directories as hard contracts; cross-cutting changes get an ADR
(see §13).

```
confero/
├── README.md
├── AGENTS.md                   # repo-wide conventions for AI agents
├── CLAUDE.md                   # Claude-specific addendum (symlink-style)
├── LICENSE
├── Makefile                    # `make dev`, `make test`, `make generate`, ...
├── .editorconfig
├── .golangci.yml
├── .gitignore
│
├── api/
│   └── openapi.yaml            # single source of truth for the API
│
├── server/
│   ├── cmd/
│   │   └── confero-server/     # the only Go binary
│   │       └── main.go
│   ├── internal/
│   │   ├── api/                # generated server stubs (do not edit)
│   │   ├── audit/
│   │   ├── auth/               # OIDC + session middleware
│   │   ├── calendar/           # ICS feed assembly
│   │   ├── config/             # env-var parsing
│   │   ├── database/           # pgx pool, migration runner
│   │   ├── http/               # router, middleware, handler wiring
│   │   ├── ical/               # iCalendar encoder (tiny, no deps)
│   │   ├── importer/           # YAML/TOON bulk import parsers
│   │   ├── mail/               # Mailer interface + SMTP impl
│   │   ├── repository/         # sqlc-generated (do not edit)
│   │   ├── scheduler/          # in-process worker
│   │   ├── service/            # business logic
│   │   └── version/            # build-time injected metadata
│   ├── db/
│   │   ├── migrations/         # *.up.sql / *.down.sql (golang-migrate)
│   │   └── queries/            # *.sql for sqlc
│   ├── sqlc.yaml
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile              # builds confero-server image
│   └── tests/                  # integration & API tests
│
├── web/
│   ├── src/
│   │   ├── api/                # generated TS client (do not edit)
│   │   ├── components/
│   │   ├── features/           # feature-folders (conferences, stars, settings, ...)
│   │   ├── hooks/
│   │   ├── lib/                # shared utilities
│   │   ├── pages/              # route-level components
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── public/
│   ├── index.html
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── Dockerfile              # builds confero-web image
│
├── bruno/
│   └── confero/                # Bruno API collection (committed)
│
├── deploy/
│   ├── compose/
│   │   ├── docker-compose.yml
│   │   ├── keycloak/realm-confero.json   # seeded realm, test users
│   │   ├── postgres/init.sql             # only for compose; prod uses migrations
│   │   └── README.md
│   └── helm/
│       └── confero/
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── values.schema.json
│           └── templates/
│               ├── _helpers.tpl
│               ├── cnpg-cluster.yaml
│               ├── server-deployment.yaml
│               ├── server-service.yaml
│               ├── web-deployment.yaml
│               ├── web-service.yaml
│               ├── ingress.yaml
│               ├── configmap.yaml
│               └── secret.yaml
│
├── docs/
│   ├── ARCHITECTURE.md         # this file (moves here from /outputs)
│   ├── REQUIREMENTS.md
│   ├── DATA_MODEL.md
│   ├── adr/
│   │   ├── 0001-go-postgres-react.md
│   │   ├── 0002-openapi-first.md
│   │   ├── 0003-in-process-scheduler.md
│   │   ├── 0004-oidc-claim-name-at-build-time.md
│   │   ├── 0005-distroless-static-web-server.md
│   │   └── ...
│   └── openapi/                # generated Redoc static site
│
└── .github/
    └── workflows/
        ├── ci.yaml
        └── release.yaml
```

Notes:

- **One Go module.** `server/` holds one `go.mod` for the single
  `confero-server` binary. The web image uses nginx, not a Go
  binary, so there is no second `cmd/`.
- **`internal/`** is genuinely internal; nothing outside `server/`
  imports it.
- **Generated code lives next to consumers.** `server/internal/api/`
  and `web/src/api/` are gitignored as generated artifacts.

---

## 3. API contract & code generation

### 3.1 Single source of truth

`api/openapi.yaml` is OpenAPI 3.1.0. It is the only place where the
API is defined. Any change to the API begins with editing this file;
generation propagates to server stubs and the client.

Top-level resources:

| Path prefix                     | Auth     | Resource              |
| ------------------------------- | -------- | --------------------- |
| `/api/v1/conferences`           | public read, member write, **admin delete** | Conferences |
| `/api/v1/conferences/{id}/stars`| member   | Star / unstar         |
| `/api/v1/tags`                  | public   | Tag list (autocomplete) |
| `/api/v1/tracks`                | public   | Track list            |
| `/api/v1/me`                    | member   | Current user profile  |
| `/api/v1/me/settings`           | member   | Reminder settings     |
| `/api/v1/me/stars`              | member   | "My stars" view       |
| `/api/v1/me/calendar-tokens`    | member   | Per-user calendar tokens |
| `/api/v1/audit-log`             | **admin** | Audit history        |
| `/api/v1/import`                | member   | YAML/TOON bulk import |
| `/calendar/all.ics`             | public   | ICS: all deadlines    |
| `/calendar/u/{token}.ics`       | by token | ICS: starred only     |
| `/auth/login`                   | public   | OIDC login redirect   |
| `/auth/callback`                | public   | OIDC callback         |
| `/auth/logout`                  | member   | Session logout        |
| `/healthz`, `/readyz`, `/metrics` | infra | Liveness/readiness/Prometheus |

### 3.2 Generation

- **Server (Go):** `oapi-codegen` (`github.com/oapi-codegen/oapi-codegen/v2`)
  generates a strict server interface (`ServerInterface` + a
  `Strict<Method>` style) into `server/internal/api/`. Handlers
  implement the interface; routing is wired with chi via the
  generated `HandlerFromMux`.

- **Client (TypeScript):** `@hey-api/openapi-ts` generates fully
  typed types and a fetch-based SDK into `web/src/api/`. The SPA
  imports `import { ConferencesService } from "@/api"` and calls
  typed methods.

- **Bruno:** the collection in `bruno/confero/` is hand-edited but
  regenerable from the spec via `bruno-cli` import. CI verifies
  that every operation in the spec has a corresponding `.bru` file.

### 3.3 Versioning

- Path-prefix versioning: `/api/v1/...`. Breaking changes go to
  `/api/v2/...`. Non-breaking additions stay in `v1`.
- The spec is also tagged via `info.version` (semver). CI lints the
  spec with `redocly lint` and fails on undocumented endpoints.

---

## 4. Server architecture

### 4.1 Process model

`confero-server` is a single Go binary that runs:

1. An HTTP server (`net/http` + `chi`), serving the API, OIDC
   endpoints, health/metrics, and the calendar feeds.
2. An in-process scheduler goroutine for reminders and digests.

Both are managed by a single `errgroup.Group` so a fatal error in
either tears the process down cleanly.

### 4.2 Layering

Strict, one-way dependencies — transport → service → repository:

```
+------------------+
| HTTP handlers    |   internal/http, internal/api (generated)
| (transport)      |
+--------+---------+
         |
         v
+------------------+
| Services         |   internal/service/*
| (business logic) |
+--------+---------+
         |
         v
+------------------+
| Repositories     |   internal/repository (sqlc-generated)
| (data access)    |
+--------+---------+
         |
         v
+------------------+
| Postgres / SMTP  |
+------------------+
```

Rules enforced by code review and a `go vet`-style lint:

- Handlers contain **no** business logic — they validate the
  request shape, call a service, and translate the result into an
  HTTP response.
- Services contain **no** SQL — they call repository methods.
- Repositories contain **no** business rules — sqlc-generated
  queries plus the bare minimum of glue code.

### 4.3 Package responsibilities

| Package                    | Role                                                                       |
| -------------------------- | -------------------------------------------------------------------------- |
| `cmd/confero-server`       | Wire dependencies, parse config, start HTTP + scheduler, shutdown.         |
| `internal/api`             | **Generated** OpenAPI types and server interface.                          |
| `internal/audit`           | Middleware + `audit.MarkEntity(ctx, kind, id)` helper; writes to `audit_log` after 2xx. |
| `internal/auth`            | OIDC discovery, login/callback handlers, JWT issue/verify, RBAC checks.    |
| `internal/calendar`        | ICS feed assembly using `internal/ical`.                                   |
| `internal/config`          | Env-var parsing into a `Config` struct; validation at startup.             |
| `internal/database`        | `pgxpool` setup, migration runner (`golang-migrate`).                      |
| `internal/http`            | Chi router, middleware (request ID, logger, auth, recovery), error mapping. |
| `internal/ical`            | Minimal iCalendar 2.0 encoder. No external deps.                           |
| `internal/importer`        | YAML and TOON parsers, both implementing an `Importer` interface.          |
| `internal/mail`            | `Mailer` interface; SMTP implementation; templates.                        |
| `internal/repository`      | **Generated** sqlc queries.                                                |
| `internal/scheduler`       | Periodic dispatch loop; reminder / digest pickup; archive sweeper.         |
| `internal/service`         | Domain services (`ConferenceService`, `StarService`, ...).                 |
| `internal/version`         | `version.Version`, `version.Commit`, set via `-ldflags`.                   |

### 4.4 Interfaces and seams

Where a thing has more than one plausible implementation, it gets
an interface so tests can swap it:

```go
// internal/mail/mailer.go
type Mailer interface {
    Send(ctx context.Context, msg Message) error
}

// internal/importer/importer.go
type Importer interface {
    Parse(r io.Reader) ([]ConferenceInput, error)
}

// internal/auth/tokens.go
type TokenIssuer interface {
    Issue(claims SessionClaims) (string, error)
}
type TokenVerifier interface {
    Verify(token string) (SessionClaims, error)
}
```

The repository layer is *not* abstracted behind interfaces — sqlc
gives us a concrete type per query group, and we test against a
real Postgres via testcontainers. Mocking the repository would buy
nothing and lose real-SQL coverage.

### 4.5 Error handling

- Internal: errors flow up as values, wrapped with `%w` where useful.
  Sentinel errors live next to the package that defines them
  (`service.ErrConferenceNotFound`, `auth.ErrUnauthorized`).
- Transport: a single error-mapping function in `internal/http`
  translates sentinel errors to HTTP status codes and produces
  RFC 7807 `application/problem+json` responses.

### 4.6 Configuration

Configuration is loaded once at startup from environment variables.
No config files in v1. The `Config` struct documents every field.

```go
type Config struct {
    HTTPAddr             string  // CONFERO_HTTP_ADDR (default ":8080")
    DatabaseURL          string  // CONFERO_DATABASE_URL (required)
    OIDCIssuerURL        string  // CONFERO_OIDC_ISSUER_URL (required)
    OIDCClientID         string  // CONFERO_OIDC_CLIENT_ID (required)
    OIDCClientSecret     string  // CONFERO_OIDC_CLIENT_SECRET (required, from Secret)
    OIDCRedirectURL      string  // CONFERO_OIDC_REDIRECT_URL (required)
    OIDCMemberValue      string  // CONFERO_OIDC_MEMBER_VALUE (required)
    OIDCAdminValue       string  // CONFERO_OIDC_ADMIN_VALUE (required)
    SessionSecret        string  // CONFERO_SESSION_SECRET (required, ≥32 bytes)
    SMTPAddr             string  // CONFERO_SMTP_ADDR (e.g. "mailout.tum.de:587")
    SMTPUsername         string  // CONFERO_SMTP_USERNAME (optional)
    SMTPPassword         string  // CONFERO_SMTP_PASSWORD (optional)
    SMTPFrom             string  // CONFERO_SMTP_FROM
    PublicBaseURL        string  // CONFERO_PUBLIC_BASE_URL — for links in emails / ICS
    ArchiveGraceDays     int     // CONFERO_ARCHIVE_GRACE_DAYS (default 7)
    DigestHorizonWeeks   int     // legacy; users override in their settings
    LogLevel             string  // CONFERO_LOG_LEVEL (default "info")
}
```

`Config.Validate()` runs at startup and refuses to boot on missing
required values. Build-time values (`auth.OIDCClaimName`,
`version.Version`) are NOT in `Config`; they are package-level
variables set via `-ldflags`.

---

## 5. Frontend architecture

### 5.1 Stack

- React 18 + TypeScript, built with Vite.
- Routing: `react-router` v6.
- Data fetching: `@tanstack/react-query` over the generated TS client.
- Styling: Tailwind CSS + a small set of headless UI primitives
  (`@radix-ui/*`) for accessible dialogs, menus, etc.
- Forms: `react-hook-form` + Zod for runtime validation that mirrors
  the OpenAPI shape.
- Markdown rendering: `marked` + `DOMPurify` (server already sanitizes
  on render, but defense in depth is cheap).

### 5.2 Layout

```
web/src/
├── api/                # generated SDK; do not edit
├── components/         # generic UI primitives (Button, Modal, ...)
├── features/
│   ├── conferences/    # list, detail, edit form, archive UI
│   ├── stars/          # star button, "my stars" page
│   ├── tags/           # autocomplete tag input
│   ├── settings/       # reminder lead times, digest, timezone
│   ├── calendar/       # calendar subscriptions UI, token regeneration
│   ├── audit/          # admin-only audit log view
│   └── auth/           # login button, "not authorized" page
├── hooks/
├── lib/
│   ├── auth.ts         # "is logged in?", "is admin?" helpers
│   ├── time.ts         # render UTC instants in user's local zone
│   └── query.ts        # configured QueryClient
├── pages/
│   ├── HomePage.tsx
│   ├── ConferenceDetailPage.tsx
│   ├── MyStarsPage.tsx
│   ├── SettingsPage.tsx
│   ├── AuditPage.tsx
│   └── NotAuthorizedPage.tsx
├── App.tsx
└── main.tsx
```

### 5.3 Auth in the SPA

- The browser hits `/auth/login` (server endpoint) which kicks off
  the OIDC code flow with PKCE. The callback sets a same-site
  cookie containing the session id.
- The SPA does not handle tokens directly. It calls API endpoints
  with `credentials: "include"`; the server's middleware reads
  the session cookie.
- A `GET /api/v1/me` endpoint returns the current user (and their
  role flags); the SPA caches this in React Query to drive UI
  affordances ("show admin badge", "show delete button", etc.).

### 5.4 Public vs. authenticated UI

The conference list page works without login. Member-only actions
(star, edit, delete, settings, calendar tokens) are gated by
`useCurrentUser()` returning a logged-in user; otherwise the
buttons either disappear or trigger the login redirect.

---

## 6. Authentication & authorization

### 6.1 OIDC flow

```mermaid
sequenceDiagram
    actor User
    participant Browser
    participant Web as confero-web (static)
    participant API as confero-server
    participant IdP as Keycloak

    User->>Browser: open https://confero.example
    Browser->>Web: GET /
    Web-->>Browser: SPA bundle
    Browser->>API: GET /api/v1/me
    API-->>Browser: 401 (no session)
    Browser->>API: GET /auth/login
    API-->>Browser: 302 → IdP authorize URL (with PKCE)
    Browser->>IdP: authorize
    User->>IdP: log in
    IdP-->>Browser: 302 → /auth/callback?code=...
    Browser->>API: GET /auth/callback?code=...
    API->>IdP: token exchange
    IdP-->>API: id_token + access_token
    API->>API: verify, extract claim, decide role
    API-->>Browser: Set-Cookie session=...; 302 → /
    Browser->>API: GET /api/v1/me (with cookie)
    API-->>Browser: 200 {id, name, email, roles:[member,admin?]}
```

Notes:

- PKCE on every flow, including confidential clients.
- `id_token` is verified against the IdP's JWKS (cached, refreshed
  by `go-oidc`).
- The session id is opaque; the server keeps the session record in
  Postgres (`sessions` table managed by `scs`).

### 6.2 Claim & role decoding

- Claim **name**: `auth.OIDCClaimName` (build-time var, default
  `groups`). Locked by `-ldflags -X`.
- Two runtime values:
  - `cfg.OIDCMemberValue` — member.
  - `cfg.OIDCAdminValue`  — admin.
- After verification, the server extracts the claim, expects a JSON
  array of strings, and computes the user's roles:

  ```go
  roles := auth.Roles{}
  for _, g := range groups {
      if g == cfg.OIDCMemberValue { roles.Member = true }
      if g == cfg.OIDCAdminValue  { roles.Member, roles.Admin = true, true }
  }
  if !roles.Member { return ErrNotAuthorized }
  ```

  Admin implies member (FR-22b).

### 6.3 Stateless token-based auth (no per-request DB)

We do **not** store sessions in Postgres. After a successful OIDC
callback, the server issues a short-lived, signed JWT and sets it
as an HttpOnly cookie. Every subsequent request is authenticated
by verifying the JWT cryptographically — **no database call**.

**Token format.**

- Algorithm: **HS256**, signed with `CONFERO_SESSION_SECRET`
  (≥32 random bytes, mounted from a Kubernetes Secret).
- Claims:
  ```json
  {
    "iss": "confero",
    "sub": "<users.id (UUID)>",
    "email": "user@example.org",
    "name": "Erika Mustermann",
    "oidc_sub": "<original OIDC subject>",
    "roles": ["member", "admin"],
    "exp": 1747700000,
    "iat": 1747680000
  }
  ```
- TTL: **1 hour** (configurable; the SPA silently re-runs the
  OIDC code flow when a 401 comes back).
- Cookie flags: `HttpOnly; Secure; SameSite=Lax; Path=/`.

**Why this works for us.**

- The chair is small; we don't need server-side revocation. If
  someone is offboarded, we remove them from the IdP group and
  their next OIDC flow fails — they get a 401 within the 1-hour
  window. We can also rotate `SESSION_SECRET` (which invalidates
  every token instantly) if we ever need an emergency kill switch.
- Verification is a HMAC + base64-decode; no allocations beyond the
  parsed claims, no I/O. Latency budget for auth on every request
  is ~5 µs.

**The one DB hit is at login.** When a valid OIDC token comes in
on `/auth/callback`, the server upserts the `users` row (so the
audit log and stars have an FK target) and bakes the resulting
`users.id` into the JWT's `sub` claim. From that point on, until
the JWT expires, the server never looks the user up by `users.id`
for authentication purposes.

**Logout.** `POST /auth/logout` clears the cookie. The JWT remains
technically valid until its `exp`, but no client can read it from
the cookie store; in practice this is the same security posture
that almost all JWT-backed apps run with. If you want
**hard** revocation we'd need a denylist (a single Postgres row
per logout, with a TTL); v1 ships without it.

### 6.4 Authorization middleware

Three chi middlewares stack:

1. `RequireToken` — parses + verifies the JWT, populates `ctx`
   with `SessionClaims` (id, email, name, roles). Returns 401 if
   missing/invalid/expired.
2. `RequireMember` — 403 if `!roles.Member`. Implied by
   `RequireToken` since the token issuer rejects non-members.
3. `RequireAdmin` — 403 if `!roles.Admin`.

Public routes (`GET /api/v1/conferences`, `/calendar/all.ics`,
`/auth/login`, `/healthz`) opt out by not mounting these middlewares.

### 6.5 Audit middleware

Audit log writes are handled by **HTTP middleware**, not by the
service layer. Rationale: it keeps service code free of
audit-write boilerplate, and the action/timestamp/actor are
naturally bound to the HTTP request — middleware is the right
seam.

**Wiring.**

```go
r.Route("/api/v1/conferences", func(r chi.Router) {
    r.Use(auth.RequireToken, auth.RequireMember)
    r.With(audit.For("conference", "create")).
        Post("/", h.CreateConference)
    r.With(audit.For("conference", "update")).
        Put("/{id}", h.UpdateConference)
    r.With(audit.For("conference", "archive")).
        Post("/{id}/archive", h.ArchiveConference)
    r.With(audit.For("conference", "unarchive")).
        Post("/{id}/unarchive", h.UnarchiveConference)
    r.With(auth.RequireAdmin, audit.For("conference", "delete")).
        Delete("/{id}", h.DeleteConference)
})
```

**How `audit.For` works.**

1. Before the handler runs, it stamps the request context with
   `entity_type` and `action`.
2. The handler runs. If it operated on an entity, it calls
   `audit.MarkEntity(ctx, entityID)` to record the specific id.
   (For `POST` creates, the handler always knows the id it just
   produced; for `PUT/DELETE`, it usually comes from the URL.)
3. After the handler returns, the middleware wraps the
   `ResponseWriter` to capture the status code.
4. **On 2xx only**, the middleware writes one `audit_log` row
   with `actor_user_id`, the denormalized `actor_display_name` +
   `actor_oidc_subject` snapshots (from `SessionClaims`),
   `action`, `entity_type`, `entity_id`, and `created_at = now()`.
5. On non-2xx, nothing is written.

**Trade-off.** The audit row is written *after* the underlying
change has been committed. In the unhappy path where the change
commits but the audit insert fails (e.g., Postgres goes away in
that millisecond), the action is real but unrecorded. This is
acceptable at chair scale: such failures will be visible in logs
and metrics (`confero_audit_write_failures_total`), and the
operational fix is "investigate and reconcile manually" —
something we'd need to do anyway for any unhappy-path scenario.
A transactional alternative would re-tangle the service layer
with audit concerns, which the user has explicitly chosen against.

**Best-effort retry.** The middleware retries the insert up to 3
times with a 50 ms backoff on transient errors, then logs at WARN
and continues. The response is **not** held up by audit failures.

---

## 7. Mail & calendar

### 7.1 Mailer

Interface lives in `internal/mail`; the SMTP implementation uses
`net/smtp` with STARTTLS. Templates are Go `html/template`
plus a sibling `text/template` for plain-text alternatives.

Templates and their data shapes:

| Template            | Trigger                                       |
| ------------------- | --------------------------------------------- |
| `reminder.html.tmpl`| Per-deadline reminder for one starred conf.   |
| `digest.html.tmpl`  | Weekly digest of all upcoming deadlines.      |

The `Mailer` interface is the seam where future providers (SES,
Postmark) plug in.

### 7.2 Calendar (ICS)

The `internal/calendar` package builds RFC 5545 iCalendar streams.
`internal/ical` is a tiny in-repo encoder so we don't take a
dependency for a few hundred lines of formatting.

Two feeds:

- `GET /calendar/all.ics` — every non-archived conference.
- `GET /calendar/u/{token}.ics` — token lookup, then starred
  conferences for that user.

Each conference emits up to five `VEVENT`s:

| Event                  | When                                    |
| ---------------------- | --------------------------------------- |
| Conference dates       | `event_start_date` → `event_end_date`   |
| Submission deadline    | `primary_deadline` (point-in-time, 15min duration for visibility) |
| Abstract deadline      | `abstract_deadline` if set              |
| Notification           | `notification_date` if set              |
| Camera-ready           | `camera_ready_date` if set              |

`UID` is deterministic: `{conference_id}:{deadline_kind}@confero`.
Calendar clients use it to update existing entries instead of
duplicating when fields change.

Response headers:

```
Content-Type: text/calendar; charset=utf-8
Cache-Control: public, max-age=300
ETag: "<sha256 of body, first 16 hex chars>"
```

Conditional GETs (`If-None-Match`) return 304 cheaply.

---

## 8. Scheduler (in-process worker)

### 8.1 Architecture

A single goroutine started from `cmd/confero-server/main.go`:

```go
sched := scheduler.New(scheduler.Config{
    Tick:         60 * time.Second,
    Mailer:       mailer,
    DB:           dbpool,
    PublicBaseURL: cfg.PublicBaseURL,
})
g.Go(func() error { return sched.Run(ctx) })
```

Each tick the scheduler does, in order:

1. **Dispatch due reminders.** `SELECT ... FOR UPDATE SKIP LOCKED
   LIMIT 50 WHERE status='pending' AND scheduled_for <= now()`
   on `reminder_dispatch_log`, then for each row: render the
   template, call `mailer.Send`, and either set `status='sent',
   sent_at=now()` or bump `attempts` and set
   `last_error`. After `MaxAttempts`, set `status='failed'`.
2. **Dispatch due digests.** Same pattern on `digest_dispatch_log`.
3. **Archive sweeper** (once per hour): set `archived_at` on any
   conference whose `event_end_date < (today - grace_days)`.
4. **Materialize new schedule entries** if needed (see §8.2).

### 8.2 Materialization

Per FR-15, every send is keyed by
`(user, conference, deadline_kind, lead_time_days)`. To enforce
"send-at-most-once" cheaply, we *materialize* the schedule when
inputs change, not at send time:

| Trigger                          | Action                                                     |
| -------------------------------- | ---------------------------------------------------------- |
| User stars a conference          | Insert one row per (set deadline × user's lead times).     |
| User un-stars a conference       | Set `status='cancelled'` on pending rows.                  |
| User changes lead times          | Re-materialize all their pending rows.                     |
| Conference deadline edited       | Re-materialize all pending rows for that conference.       |
| Conference archived              | Cancel all related pending rows.                           |
| New user logs in (first time)    | No-op (no stars yet).                                      |

All of the above happen *transactionally* in the service layer
alongside the underlying change, so a star + its reminder schedule
either both land or both don't.

Re-materialization uses a two-step pattern:

```sql
-- 1. Cancel anything still pending for this slice
UPDATE reminder_dispatch_log
SET status = 'cancelled', updated_at = now()
WHERE status = 'pending'
  AND user_id = $1 AND conference_id = $2;

-- 2. Insert fresh schedule (UNIQUE constraint protects against double-fire)
INSERT INTO reminder_dispatch_log (...) VALUES (...)
ON CONFLICT (user_id, conference_id, deadline_kind, lead_time_days) DO NOTHING;
```

### 8.3 Why single-replica

The scheduler is in-process; running two API replicas with two
schedulers would not double-send (the UNIQUE constraint catches it)
but it would race on locks and waste work. v1 ships with
`server.replicaCount: 1` and a `Recreate` deployment strategy.

`SELECT ... FOR UPDATE SKIP LOCKED` is used anyway, so when we
later extract the scheduler into its own Deployment (or move to
N>1 server replicas with a leader election lib), no code change is
required (see §15).

### 8.4 Digest scheduling

A daily probe (once per UTC hour) finds users whose configured
digest hour matches the current hour *in their timezone* and have
not yet received this week's digest. It inserts a row in
`digest_dispatch_log (user_id, week_starting, scheduled_for=now())`
which the main dispatch loop then picks up. The
`UNIQUE (user_id, week_starting)` constraint prevents duplicates.

---

## 9. Observability

### 9.1 Logging

- `log/slog` with the JSON handler. Every log line carries
  `request_id`, `route`, `user_id` (if authenticated).
- `internal/http/middleware.RequestID` injects a request id from
  the `X-Request-Id` header (or generates one) and propagates it
  via `context.Context`.
- Errors propagating to the transport are logged once at the
  boundary, never twice.

### 9.2 Metrics

Prometheus via `prometheus/client_golang`. Initial metrics:

| Metric                                   | Type      | Labels                |
| ---------------------------------------- | --------- | --------------------- |
| `http_requests_total`                    | counter   | method, route, status |
| `http_request_duration_seconds`          | histogram | method, route         |
| `confero_emails_sent_total`              | counter   | kind (reminder/digest), result (sent/failed) |
| `confero_scheduler_pending_reminders`    | gauge     | —                     |
| `confero_scheduler_pending_digests`      | gauge     | —                     |
| `confero_db_connections_in_use`          | gauge     | —                     |

`/metrics` is exposed unauthenticated on the same port; the chart
restricts it to a `ClusterIP`-only service.

### 9.3 Health endpoints

- `GET /healthz` — always 200 if the process is up. Liveness probe.
- `GET /readyz` — runs `db.Ping(ctx)` with a 1s timeout, returns 503
  on failure. Readiness probe.

### 9.4 Tracing

Out of scope for v1. The logging + metrics surface is enough at
chair scale. If we ever add it, `otel-go` slots in via middleware.

---

## 10. Containers & local development

### 10.1 Server image (`confero-server`)

```dockerfile
# server/Dockerfile
FROM golang:1.22 AS builder
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY api/ ../api/
ARG OIDC_CLAIM_NAME=groups
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w \
      -X confero/internal/auth.OIDCClaimName=${OIDC_CLAIM_NAME} \
      -X confero/internal/version.Version=${VERSION} \
      -X confero/internal/version.Commit=${COMMIT}" \
    -o /out/confero-server ./cmd/confero-server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/confero-server /confero-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/confero-server"]
```

Target image size: ~20 MB.

### 10.2 Web image (`confero-web`)

The web image is **plain nginx** — boring on purpose. We accept
that the runtime base is not distroless in exchange for using the
most battle-tested static-file server in existence. The base
image is `nginxinc/nginx-unprivileged:alpine` so the container
runs as non-root on port 8080 (Kubernetes-friendly out of the box).

```dockerfile
# web/Dockerfile
FROM node:20 AS web-builder
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
COPY api/ ../api/
RUN pnpm run build              # → /web/dist

FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY web/deploy/nginx.conf       /etc/nginx/conf.d/default.conf
COPY --from=web-builder /web/dist /usr/share/nginx/html
EXPOSE 8080
# Default ENTRYPOINT/CMD from the base image stay.
```

`web/deploy/nginx.conf` is intentionally tiny:

```nginx
server {
    listen 8080;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # Hashed assets: long cache.
    location ~* \.(?:js|css|woff2?|svg|png|jpg|webp|avif)$ {
        access_log off;
        expires 1y;
        add_header Cache-Control "public, immutable";
        try_files $uri =404;
    }

    # SPA fallback: anything that doesn't match a file goes to index.html.
    location / {
        try_files $uri /index.html;
        add_header Cache-Control "no-cache";
    }
}
```

Target image size: ~25 MB + bundle. No API proxying lives here —
Ingress routes `/api/*`, `/auth/*`, `/calendar/*`, `/healthz`,
`/readyz`, `/metrics` to the `confero-server` Service directly.

### 10.3 docker-compose for local dev

```yaml
# deploy/compose/docker-compose.yml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: confero
      POSTGRES_PASSWORD: confero
      POSTGRES_DB: confero
    ports: ["5432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U confero"]
      interval: 5s
    volumes: [pgdata:/var/lib/postgresql/data]

  keycloak:
    image: quay.io/keycloak/keycloak:24.0
    command: ["start-dev", "--import-realm"]
    environment:
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
    ports: ["8081:8080"]
    volumes:
      - ./keycloak:/opt/keycloak/data/import:ro

  mailhog:
    image: mailhog/mailhog
    ports: ["1025:1025", "8025:8025"]

  server:
    build:
      context: ../..
      dockerfile: server/Dockerfile
      args:
        OIDC_CLAIM_NAME: groups
        VERSION: dev
    environment:
      CONFERO_DATABASE_URL: postgres://confero:confero@postgres:5432/confero?sslmode=disable
      CONFERO_OIDC_ISSUER_URL: http://keycloak:8080/realms/confero
      CONFERO_OIDC_CLIENT_ID: confero
      CONFERO_OIDC_CLIENT_SECRET: dev-secret
      CONFERO_OIDC_REDIRECT_URL: http://localhost:8080/auth/callback
      CONFERO_OIDC_MEMBER_VALUE: cs-edu-chair
      CONFERO_OIDC_ADMIN_VALUE: cs-edu-chair-admin
      CONFERO_SESSION_SECRET: dev-only-not-a-real-secret-please-change
      CONFERO_SMTP_ADDR: mailhog:1025
      CONFERO_SMTP_FROM: confero@example.org
      CONFERO_PUBLIC_BASE_URL: http://localhost:5173
    depends_on:
      postgres: { condition: service_healthy }
      keycloak: { condition: service_started }
      mailhog:  { condition: service_started }
    ports: ["8080:8080"]

  web:
    build:
      context: ../..
      dockerfile: web/Dockerfile
    ports: ["5173:8080"]
    depends_on: [server]

volumes:
  pgdata:
```

The `keycloak/realm-confero.json` realm is committed and includes:

- A `confero` client (confidential, with PKCE).
- Two groups: `cs-edu-chair` and `cs-edu-chair-admin`.
- Two test users (`member@example.org`, `admin@example.org`,
  password `confero`) with the respective groups.

`make dev` runs `docker compose -f deploy/compose/docker-compose.yml
up --build`.

### 10.4 Local code edit loop

For day-to-day work, devs run the server locally (`go run ./cmd/confero-server`)
against compose-managed Postgres + Keycloak + MailHog, and the web
locally with `pnpm dev`. The `make dev-services` target brings up
only the dependencies, leaving the server and web to run on the
host with hot reload.

---

## 11. Helm chart

### 11.1 Layout

```
deploy/helm/confero/
├── Chart.yaml
├── values.yaml
├── values.schema.json
└── templates/
    ├── _helpers.tpl
    ├── cnpg-cluster.yaml        # spec.kind: Cluster (CloudNativePG)
    ├── server-deployment.yaml
    ├── server-service.yaml
    ├── web-deployment.yaml
    ├── web-service.yaml
    ├── ingress.yaml
    ├── configmap.yaml
    └── secret.yaml
```

### 11.2 CloudNativePG integration

The chart **does not** install the CNPG operator — that is a
cluster-wide prerequisite. The chart **does** instantiate a
`postgresql.cnpg.io/v1 Cluster` resource sized via values:

```yaml
postgres:
  enabled: true
  instances: 1
  storage:
    size: 10Gi
    storageClass: ""
  resources:
    requests: { cpu: 100m, memory: 256Mi }
    limits:   { cpu: 1,    memory: 1Gi }
  superuserSecret: confero-pg-superuser
```

CNPG generates the application credentials Secret automatically;
the server Deployment mounts it.

### 11.3 Key `values.yaml` excerpt

```yaml
image:
  server:
    repository: ghcr.io/<org>/confero-server
    tag: "0.1.0"
  web:
    repository: ghcr.io/<org>/confero-web
    tag: "0.1.0"

server:
  replicaCount: 1                  # locked at 1 in v1 (in-process scheduler)
  strategy: { type: Recreate }
  resources:
    requests: { cpu: 100m, memory: 128Mi }
    limits:   { cpu: 1,    memory: 512Mi }

web:
  replicaCount: 2
  resources:
    requests: { cpu: 25m, memory: 32Mi }
    limits:   { cpu: 200m, memory: 128Mi }

ingress:
  enabled: true
  className: nginx
  host: confero.example.org
  tls:
    enabled: true
    secretName: confero-tls
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod

oidc:
  issuerURL: https://keycloak.example.org/realms/confero
  clientID: confero
  clientSecretRef: { name: confero-oidc, key: client-secret }
  memberValue: cs-edu-chair
  adminValue: cs-edu-chair-admin

smtp:
  addr: smtp.example.org:587
  from: confero@example.org
  usernameRef: { name: confero-smtp, key: username }
  passwordRef: { name: confero-smtp, key: password }
```

### 11.4 Snapshot tests

`helm template` output is rendered with a fixed values file under
`deploy/helm/confero/tests/snapshots/` and CI fails if rendering
drifts unexpectedly. We use `helm-unittest` for value-driven test
cases (e.g. "TLS disabled produces no `tls:` block in the Ingress").

---

## 12. Testing strategy

| Layer                       | Tooling                                       | Runs where           |
| --------------------------- | --------------------------------------------- | -------------------- |
| Unit (Go)                   | stdlib `testing` + `testify/require`          | every PR             |
| Repository (Go)             | `testcontainers-go` + real Postgres image     | every PR             |
| Service (Go)                | unit tests with real repository or mocks      | every PR             |
| HTTP / API (Go)             | end-to-end against a spawned server          | every PR             |
| OpenAPI conformance         | `oapi-codegen --generate types` + diff in CI | every PR             |
| Frontend unit               | Vitest + React Testing Library + MSW          | every PR             |
| Type check                  | `tsc --noEmit`, `golangci-lint run`           | every PR             |
| Bruno collection            | `bru run` against compose stack               | nightly + on release |
| Helm template               | `helm-unittest`, `helm lint`                  | every PR             |

Coverage targets are not hard-set in v1; the rule is "every public
function in `service/` has at least one test, every handler has a
happy path and one error path".

The Bruno collection is the **manual** test artifact — humans
opening Bruno see a curated, ordered list of operations they can
click through, with sample bodies pre-filled. CI runs the same
collection headlessly to catch regressions in the deployed stack.

---

## 13. Documentation strategy

The repo carries five tiers of documentation:

1. **`README.md`** — the front door. Quickstart for running locally,
   how to deploy, where to find the rest.
2. **`AGENTS.md`** (root) — repo-wide conventions for AI coding
   agents. Hard rules: never edit generated code; always edit
   `api/openapi.yaml` first; run `make generate` before
   committing; respect layer boundaries; etc. Includes a "how
   to add a feature" recipe.
3. **`CLAUDE.md`** (root) — Claude-specific addendum that points
   at AGENTS.md plus any model-specific quirks.
4. **`docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`,
   `docs/REQUIREMENTS.md`** — the three reference docs.
5. **`docs/adr/NNNN-*.md`** — Architecture Decision Records. Every
   decision in the decisions logs of REQUIREMENTS / DATA_MODEL /
   ARCHITECTURE gets an ADR over time. Format: Michael Nygard's
   short ADR template (`Status`, `Context`, `Decision`,
   `Consequences`).
6. **`docs/openapi/`** — Redoc-generated static reference site,
   published to GitHub Pages from CI.

The `docs/` directory is the source of truth; the README links
into it but does not duplicate it.

---

## 14. CI/CD outline

GitHub Actions workflows:

- **`ci.yaml`** runs on every push and PR:
  - `golangci-lint run`
  - `go test ./... -race -cover` with Testcontainers
  - `pnpm lint`, `pnpm test`, `tsc --noEmit`, `pnpm build`
  - `redocly lint api/openapi.yaml`
  - `helm lint deploy/helm/confero`
  - `helm-unittest deploy/helm/confero`
  - Build server + web images (no push) to verify they build.

- **`release.yaml`** runs on tag push (`v*`):
  - Build and push server + web images to GHCR with the tag and
    `:latest`.
  - Package the Helm chart and push to a chart repo.
  - Publish the Redoc site to GitHub Pages.
  - Attach a `CHANGELOG.md` snippet to the GitHub Release.

---

## 15. Future evolution

These are paths v1 leaves open so we don't paint ourselves into a
corner; none are work for v1.

- **Extract the scheduler.** Move `internal/scheduler` into a
  `cmd/confero-worker` binary; same Go module, separate Deployment
  with `replicaCount: 1` (or N + leader election). API server then
  scales horizontally. No data-model change required.
- **Multi-tenancy.** Add a `tenants` table and `tenant_id` to
  every entity. The OIDC layer already abstracts the membership
  decision; we'd extend it to map claim values to tenants.
- **External transactional mail.** Replace the SMTP `Mailer`
  implementation with an SES/Postmark adapter. Interface unchanged.
- **Calendar feed variants.** Add `kind` values to
  `user_calendar_tokens` and corresponding handlers.
- **External imports.** Add WikiCFP / conf-deadlines.org scrapers
  as additional `Importer` implementations.

---

## 16. Open questions

- **CNPG operator install responsibility.** v1 chart assumes the
  operator is pre-installed. Confirm this is acceptable for the
  TUM cluster, or we'll add a `dependencies:` block in
  `Chart.yaml`.
- **`docs/openapi/` publication.** GitHub Pages from CI is the
  proposed default; confirm or pick an alternative (in-cluster
  Redoc served by the web image at `/docs`, for instance).
- **Frontend session bootstrap.** Should the SPA call
  `GET /api/v1/me` on first paint and show a skeleton, or render
  the public list immediately and resolve auth lazily? Default
  proposal: resolve eagerly — it's one cheap request and avoids
  flicker on member-only affordances.
