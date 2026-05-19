# Confero — Requirements (v0.1 draft)

> **Confero** — Latin *cōnferō*, "to bring together, to discuss,
> to contribute" — the etymological root of *conference*.

**Owner:** Matthias Lin Huber, CS Education chair, TUM
**Status:** Draft for review — Phase 1 output
**Last updated:** 2026-05-19

A small internal tool for a CS-Education chair to keep an organized,
shared overview of relevant academic conferences, with starring,
deadline reminders, and a clean API-first architecture.

---

## 1. Purpose and context

The chair currently lacks a shared, structured view of upcoming
CS-Ed (and adjacent) conferences. Information lives in scattered
emails, calendars, and individuals' heads. The tool should:

- Be the single source of truth for "which conferences are coming up,
  when, where, and how do I submit".
- Make it trivial for members to flag conferences they intend to
  submit to and get email reminders for their deadlines.
- Be easy for any chair member (or an AI agent acting on their behalf)
  to add or update conference data.
- Serve as a small but well-engineered reference implementation of
  the chair's preferred stack (Go + sqlc + Postgres, OpenAPI-first,
  React+TS, containers, Helm).

## 2. Stakeholders and actors

| Actor                 | Description                                                                            |
| --------------------- | -------------------------------------------------------------------------------------- |
| **Anonymous visitor** | Anyone with the URL. Can view the public conference list (read-only).                  |
| **Chair member**      | Authenticated via OIDC with the configured chair-membership claim. Can create, edit, archive, star — but **not** delete conferences and **not** view the audit log. |
| **Chair admin**       | A chair member who additionally carries the configured admin claim value. Adds: hard-delete conferences, view the audit log. |
| **Service operator**  | The person deploying via Helm — sets env, secrets, SMTP, OIDC, schedules.              |
| **AI agent**          | A user-controlled LLM that creates/updates conferences via the API or bulk-import API. |

Trust model is **flat for create/edit/archive** — every
authenticated chair member can do those — and **gated for
destructive and observability actions**: hard-deleting a conference
and viewing the audit log require the admin claim value. Every
write produces an audit log entry so actions remain traceable.

## 3. Functional requirements

IDs are stable; we will refer to them in the data model and
implementation plan.

### 3.1 Conference catalog

- **FR-1** Anonymous visitors can list and view conferences (read-only),
  filtered by upcoming/past, tag, track, and free-text search.
- **FR-2** Authenticated chair members can **create, edit, and archive**
  any conference record. **Hard delete** is restricted to chair admins.
- **FR-2a** When adding or editing tags on a conference, the UI shows all
  existing tags as autocomplete suggestions so members can reuse them.
  Trust-based; no automated near-duplicate collapsing in v1.
- **FR-3** Each conference record holds:
  - **Core:** name, acronym, year, **location (free text — e.g.
    "Munich, Germany" or "Virtual")**, website URL, CFP URL, primary
    submission deadline (`timestamptz`), event start date, event end date.
  - **Extra deadlines (nullable):** abstract deadline, notification date,
    camera-ready date.
  - **Metadata (nullable):** CORE rank (A\*/A/B/C/...), h5-index,
    acceptance rate, DBLP / series key.
  - **Categorization:** tags (many), tracks accepted (e.g. full paper,
    short paper, workshop, doctoral consortium), free-text notes
    (markdown, rendered safely).
- **FR-4** Deadlines are stored as `timestamptz` (UTC under the hood)
  and displayed in the viewer's browser timezone. (Note: explicit
  "anywhere-on-earth" labeling is **not** preserved in v1; can be
  added later via an optional zone-hint column.)
- **FR-5** Conferences are uniquely identified by `(acronym, year)`;
  duplicates are rejected with a clear error.

### 3.2 Lifecycle

- **FR-6** A scheduled job sets `archived_at` on conferences whose
  `event_end_date` is older than a configurable grace period
  (default: 7 days).
- **FR-7** Archived conferences are excluded from the default list
  view but queryable via a `?archived=true` (or "show past") filter.
- **FR-8** Archive is reversible: members can un-archive a conference
  by clearing `archived_at`.

### 3.3 Starring (submission intent)

- **FR-9** Authenticated members can star and un-star any conference.
- **FR-10** Stars are visible to other chair members (collaboration
  signal): the conference detail view shows which members have starred it.
- **FR-11** Each member has a private "my conferences" view of their
  starred items, sorted by next upcoming deadline.

### 3.4 Reminders

- **FR-12** Each member has a per-user reminder configuration:
  - A list of lead times (defaults: 28d, 14d, 7d, 1d) applied to the
    primary submission deadline of each starred conference.
  - An opt-in weekly digest (default: Monday 08:00 in user's configured
    timezone) listing all starred conferences with deadlines in the next
    N weeks (configurable, default 6).
- **FR-13** Reminder emails are sent only for **starred** conferences.
- **FR-14** Reminder emails contain: conference name + acronym + year,
  deadline kind (submission), absolute and relative time, conference
  link, deep link back to the conference detail page in the app.
- **FR-15** A reminder is sent at most once per
  `(user, conference, deadline, lead_time)` tuple, even if the worker
  retries or is restarted.
- **FR-16** Failed sends are retried with exponential backoff up to a
  configurable cap; persistent failures are logged and surfaced in an
  admin-readable health endpoint.

### 3.5 Editing UX

- **FR-17** A web UI lets members create or edit a conference via a
  single form covering all fields in FR-3.
- **FR-18** A bulk-import endpoint accepts a structured payload
  (YAML primary; pluggable parser so TOON / JSON can be added without
  a rewrite) and creates or updates conferences in one transaction.
  Designed to be friendly for an LLM to produce.
- **FR-19** All write operations validate input server-side and return
  field-level error messages.

### 3.6 Auditing

- **FR-20** Every create / update / archive / unarchive / delete on a
  conference produces an audit row containing the **actor's display
  name**, the **actor's OIDC subject**, the **action**, the
  **entity type and id**, and the **timestamp**. **No field-level
  diffs, no email, no IP address, no User-Agent** are stored
  (PII minimization).
- **FR-21** Audit history is visible **only to chair admins** (via
  the audit claim). Non-admin authenticated users and anonymous
  visitors cannot read it.

### 3.7 Calendar subscriptions (ICS feeds)

Calendar feeds let members add the chair's conferences to their
calendar app (Apple Calendar, Google Calendar, Outlook, Thunderbird,
etc.) and have the entries update automatically as conferences are
added, edited, or starred. Updates happen because calendar clients
re-fetch the feed on their own polling schedule (typically every
few hours to once a day, client-controlled). The server simply
returns the current state on every request — no push needed.

- **FR-25** A **public, unauthenticated** ICS feed at a stable URL
  (e.g., `/calendar/all.ics`) returns every non-archived
  conference's deadlines and event dates as iCalendar `VEVENT`
  entries.
- **FR-26** Each authenticated chair member has a **personal** ICS
  feed at an unguessable, stable URL containing **only their starred
  conferences** (e.g., `/calendar/u/{calendar_token}.ics`). The
  token is a high-entropy random string stored per user.
- **FR-27** Members can view, copy, and **regenerate** their personal
  calendar token from their settings page. Regenerating invalidates
  the old URL (all previously subscribed clients stop receiving
  updates and need the new URL).
- **FR-28** Both feeds include, for each conference: the conference
  event itself as a multi-day VEVENT (`event_start_date` →
  `event_end_date`), and each set deadline as its own point-in-time
  VEVENT titled `{acronym} {year}: {deadline kind}` (Submission,
  Abstract, Notification, Camera-ready). Each VEVENT carries a
  stable `UID` (so calendar apps can update existing entries rather
  than duplicate them when fields change), a `DESCRIPTION` with the
  CFP/website links, and a `URL` pointing to the conference detail
  page in the app.
- **FR-29** Feeds are returned with `Content-Type: text/calendar;
  charset=utf-8`, a short `Cache-Control: max-age` (default: 300s,
  configurable) so updates propagate reasonably quickly, and an
  `ETag` to support conditional GETs.
- **FR-30** Personal feed URLs are treated as bearer credentials:
  served only over HTTPS, never logged in full (only a prefix),
  and revoked immediately on token regeneration.

### 3.8 Authentication

- **FR-22** Auth uses OIDC. Implementation is **provider-agnostic**:
  issuer URL, client id, client secret, and scopes are configurable
  at runtime via env vars. The deployment target is a Keycloak
  instance, but switching providers requires only config changes.
- **FR-22a** The **name** of the OIDC claim that carries group /
  role memberships is a **single string baked at build time** via
  `go build -ldflags "-X confero/internal/auth.OIDCClaimName=<name>"`.
  The default is `groups` (Keycloak's default; works for most
  providers). The Dockerfile exposes this as a build ARG so a
  different deployment can rebuild the image with one flag —
  no allowlist, no priority order, no runtime claim-name override.
  The **values** to look for inside that claim
  (`CONFERO_OIDC_MEMBER_VALUE`, `CONFERO_OIDC_ADMIN_VALUE`) are
  configured at runtime via env vars.
  Rationale: keeps the binary opinionated about the upstream schema
  (one claim, one source of truth) while letting the same image be
  reused across different member/admin group names. Simpler than an
  allowlist, and matches how every major OIDC provider documents
  group claims.
- **FR-22b** A second runtime-configured value
  (`CONFERO_OIDC_ADMIN_VALUE`) on the **same** claim identifies
  chair admins. Presence of this value in a user's token grants
  the additional capabilities listed for the admin role
  (delete conferences, view audit log). Admin implies membership;
  a user with only the admin value but not the member value is
  still admitted.
- **FR-23** A user is admitted iff the OIDC ID token (or userinfo)
  contains the configured chair-membership claim **and** at least
  one of the configured values (member or admin) is present in it.
  Otherwise the user is shown a clear "not authorized" page.
- **FR-24** On first successful login, a `users` row is provisioned
  from token claims (`sub`, `email`, `name`). Subsequent logins update
  email/name if they changed upstream.

## 4. Non-functional requirements

- **NFR-1 Architecture clarity.** Layered Go backend (transport →
  service → repository); single OpenAPI document is the contract;
  no business logic in handlers.
- **NFR-2 API-driven.** OpenAPI 3.1 spec is the source of truth.
  Server handlers and TypeScript client are **both** generated from
  it (server: `oapi-codegen`; client: `openapi-typescript-codegen`
  or `orval`). Bruno collection consumes the same spec.
- **NFR-3 Containerized.** Server and worker ship as OCI images
  built from a single multi-stage `Dockerfile`. Frontend ships as a
  static bundle served by an `nginx`-based image (or by the Go
  server in a single-binary mode — TBD in architecture phase).
- **NFR-4 Helm deployment.** A first-class Helm chart with values for
  image tags, replicas, ingress, OIDC config, SMTP config, Postgres
  connection, and reminder schedules. Chart lints cleanly; `helm
  template` output is committed as a snapshot test.
- **NFR-5 Expandable & configurable.** New conference fields, new
  reminder cadences, new import formats, and new mailer backends are
  all addable behind interfaces with minimal blast radius.
- **NFR-6 Documentation.** README, ARCHITECTURE.md, ADRs (Architecture
  Decision Records), this REQUIREMENTS.md, an OpenAPI reference site
  (Redoc/Stoplight Elements), and AI-facing files (`AGENTS.md` and
  `CLAUDE.md`) with explicit conventions and pitfalls.
- **NFR-7 Testing.**
  - Backend: unit tests for service/repository layers; integration
    tests against a real Postgres (Testcontainers); API tests against
    the OpenAPI spec to detect drift.
  - Frontend: component tests (Vitest + React Testing Library) for
    critical UI; type-checking enforced in CI.
  - Bruno collection covers the full happy-path API surface manually.
- **NFR-8 Database.** Postgres 15+, schema managed by versioned
  migrations using **`golang-migrate`**, queries via `sqlc`.
- **NFR-9 Observability (lightweight).** Structured JSON logs
  (`slog`), `/healthz` and `/readyz` endpoints, Prometheus metrics
  endpoint, request IDs propagated through logs.
- **NFR-10 Security.**
  - HTTPS terminated at ingress.
  - OIDC code flow with PKCE.
  - CSRF protection on cookie-authenticated endpoints.
  - Server-side input validation on every write endpoint.
  - Markdown notes rendered with safelist HTML sanitization.
  - Secrets only via env vars / Kubernetes Secrets, never in chart
    values committed to git.

## 5. Out of scope for v1

Listed explicitly so we don't accidentally creep:

- Multi-tenancy (multiple chairs in one instance).
- Per-track or per-submission-type deadlines (only the primary
  submission deadline + the three extra date fields are modeled).
- Submission status tracking (planning → submitted → accepted → rejected).
- Per-star private notes ("submitting with student X on topic Y").
- Scraping from WikiCFP / conf-deadlines.org / ICS feeds.
- Slack/Teams/Mattermost notification channels.
- Internationalization beyond English.

## 6. Decisions log (locked so far)

| #   | Decision                                                                  |
| --- | ------------------------------------------------------------------------- |
| D1  | Backend in Go, Postgres + sqlc + `golang-migrate`.                        |
| D2  | Frontend in React + TypeScript + Vite, consuming OpenAPI-generated client.|
| D3  | OIDC, provider-agnostic; Keycloak is the deployment target.               |
| D3a | OIDC claim **name** baked at build time via `-ldflags`; **value** runtime.|
| D4  | Flat trust for create/edit/archive; admin claim required for delete + audit-log view. |
| D5  | Flat conference model — one row per upcoming iteration.                   |
| D6  | Public read of conference list; auth required for everything else.        |
| D7  | Per-user configurable reminder lead times + opt-in weekly digest.         |
| D8  | Email via configurable SMTP relay.                                        |
| D9  | Full change history via separate audit table with JSON diffs.             |
| D10 | Bulk import accepts YAML; parser is pluggable (TOON, JSON later).         |
| D11 | Auto-archive conferences after `event_end + grace_days`.                  |
| D12 | Containerized + Helm-deployed.                                            |
| D13 | Public ICS feed for all deadlines + per-user ICS feed for starred ones.   |
| D14 | Personal feed authenticated by an unguessable, user-regeneratable token.  |
| D15 | Postgres deployed via CloudNativePG operator inside the cluster.          |
| D16 | Server ships as a distroless image; web ships as nginx. Both multi-stage. |
| D17 | `docker-compose.yml` for local dev with Postgres, Keycloak, MailHog, app. |
| D18 | Notifications run in-process in the Go server (v1); extractable later.    |
| D19 | Calendar tokens live in a dedicated `user_calendar_tokens` table.         |
| D20 | API is versioned under `/api/v1/...`.                                     |
| D21 | Stars are visible to all chair members (confirmed FR-10).                 |
| D22 | Conference deletion restricted to chair admins; archive available to all. |
| D23 | Location is a single free-text field (e.g. "Munich, Germany", "Virtual"). |
| D24 | Audit log is PII-minimal (name, OIDC subject, action, timestamp) and admin-only. |
| D25 | Tag input shows existing tags as autocomplete; users trusted to reuse.    |
| D26 | OIDC claim name: single default `groups`, build-time override via Docker ARG.|
| D27 | Stateless auth via signed JWT in HttpOnly cookie — no DB lookup per request. |
| D28 | Audit log is written by HTTP middleware after a successful (2xx) response.|

## 7. Open questions

None. All phase 1–2 decisions are resolved; remaining choices belong
to phase 3 (architecture) and will be tracked in
`ARCHITECTURE.md`.
