# Changelog

All notable changes to Confero will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [0.1.0] - 2026-05-19

### Added

**Core backend (M0-M4)**
- Bootstrap: Go 1.22 module, Vite+TypeScript skeleton, OpenAPI 3.1 stub, Makefile,
  `.editorconfig`, `.golangci.yml`, `commitlint.config.cjs`, GitHub Actions CI
- Database: Postgres schema (conferences, users, user_settings, stars, tags, tracks,
  audit_log, reminder_dispatch_log, user_calendar_tokens), migrations via
  golang-migrate, sqlc query generation
- Auth: Keycloak OIDC callback, HS256 JWT in HttpOnly `session` cookie, `RequireMember`
  and `RequireAdmin` middleware, `/api/v1/me` endpoint
- Conference CRUD: create, read (list + get), update, archive/unarchive, delete with
  full pagination, tag/track filtering, and search

**Starring and settings (M5-M6)**
- Star/unstar conferences; `/api/v1/me/stars` list
- Per-user settings (email reminder opt-in, days-before-deadline preference)

**Scheduler and email (M7)**
- In-process cron scheduler: digest emails N days before each starred deadline
- `internal/mail` SMTP mailer with STARTTLS; `FakeMailer` for tests
- Automatic conference archival after configurable grace days

**ICS calendar feeds (M8)**
- `GET /calendar/all.ics` - public feed of all active conferences (ETag + 304 support)
- `GET /calendar/u/{token}.ics` - personal feed of starred conferences
- Calendar token management: `GET/POST/DELETE /api/v1/me/calendar-tokens`
- RFC 5545-compliant encoder (`internal/ical`) with line folding and text escaping

**Audit log (M9)**
- Audit middleware writes one row per 2xx mutation (create/update/archive/unarchive/delete)
- `GET /api/v1/audit-log` admin endpoint with entity-type, entity-id, actor, and
  cursor-based pagination
- `confero_audit_write_failures_total` Prometheus counter

**Bulk import (M10)**
- `POST /api/v1/import` accepts `text/x-yaml` (member role required)
- Upserts on `(LOWER(acronym), year)`; strict mode aborts on first error
- Unknown fields in YAML rejected at parse time
- See `docs/IMPORT_FORMAT.md` for the full schema

**React SPA (M11)**
- Pages: Home (conference list), Conference Detail, My Stars, Admin (conference
  management), Settings, Login
- hey-api generated client, tanstack/react-query v5, react-router-dom v7,
  Tailwind CSS v4, Radix UI primitives, react-hook-form + zod
- MSW mock service worker for tests; Vitest + React Testing Library; jest-axe a11y

**Helm chart (M12)**
- Helm chart at `deploy/helm/confero/` with CloudNativePG Cluster CRD
- `values.schema.json` enforces `server.replicaCount` <= 1
- Templates: cnpg-cluster, configmap, secret, server-deployment, server-service,
  web-deployment, web-service, ingress
- helm-unittest tests: TLS, CNPG, replica-count enforcement

**Docs and release (M13)**
- Real `README.md` with quickstart, deploy guide, architecture table
- `CHANGELOG.md` (this file)
- `docs/IMPORT_FORMAT.md` - YAML import schema reference
- ADRs 0001-0012 covering all architectural decisions

[0.1.0]: https://github.com/your-org/confero/releases/tag/v0.1.0
