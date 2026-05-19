# Confero

> **Confero** - Latin *cōnferō*, "to bring together, to discuss, to contribute"
> - the etymological root of *conference*.

A small internal tool for the TUM CS-Education chair to track upcoming CS-Ed conferences,
let chair members star conferences they plan to submit to, receive email reminders for
deadlines, and subscribe to ICS calendar feeds.

**Designed as a reference implementation** of the chair's preferred stack:
Go + Postgres + React + Keycloak OIDC + Kubernetes via CloudNativePG + Helm.

---

## Features

- **Conference registry** - create, update, archive conferences with rich metadata
  (deadlines, rankings, acceptance rates, DBLP keys, tags, tracks)
- **Starring** - each member stars conferences they intend to submit to
- **Email reminders** - digest emails before upcoming deadlines (configurable schedule)
- **ICS feeds** - `/calendar/all.ics` for all active conferences; per-user feeds for
  starred conferences via secure token URLs
- **Bulk import** - upsert conferences from a YAML file (`POST /api/v1/import`)
- **Audit log** - every create/update/archive/delete is logged with actor info (admin view)
- **OpenAPI-first** - `api/openapi.yaml` is the single source of truth; client and server
  code is generated from it

---

## Quickstart (local dev)

**Prerequisites:** Docker Desktop (or OrbStack), `make`, Go 1.22+, pnpm.

```bash
# 1. Clone and enter
git clone https://github.com/your-org/confero
cd confero

# 2. Start Postgres + Keycloak + MailHog
make dev-services

# 3. Run migrations
make migrate-up

# 4. Start the server (hot-reload via `air` or just `go run`)
cd server && go run ./cmd/confero-server

# 5. Start the web app
cd web && pnpm install && pnpm dev
```

The app will be at `http://localhost:5173`.
Keycloak admin: `http://localhost:8080` (admin/admin).
MailHog: `http://localhost:8025`.

A test realm is pre-loaded from `deploy/compose/keycloak/realm-confero.json`.
Two test users: `member@example.org` / `admin@example.org` (password: `confero`).

---

## Running the full test suite

```bash
make test        # Go tests (race detector on) + Vitest + helm-unittest
make lint        # golangci-lint + eslint + redocly lint + helm lint
make build       # build Docker images
```

Tests use `testcontainers-go` for Go and Vitest + MSW for the frontend.
No external services required beyond Docker.

---

## Deploy to Kubernetes

Confero ships a Helm chart that manages:

- The Go server (single replica, Recreate strategy)
- The React SPA behind nginx
- A CloudNativePG `Cluster` (Postgres managed by the CNPG operator)
- A Kubernetes `Secret` (session key, Keycloak credentials, SMTP password)
- An `Ingress` with TLS

```bash
# Add the chart (from OCI registry once released)
helm install confero oci://ghcr.io/your-org/charts/confero \
  --namespace confero --create-namespace \
  --set ingress.host=confero.example.org \
  --set oidc.issuerURL=https://keycloak.example.org/realms/confero \
  --set oidc.clientID=confero \
  --set oidc.clientSecret=<secret> \
  --set server.sessionSecret=<32-byte-random> \
  --set postgres.password=<db-password>
```

See `deploy/helm/confero/values.yaml` for all knobs, and
`deploy/helm/confero/values.schema.json` for validation constraints.

---

## Bulk import

Conferences can be imported from YAML via `POST /api/v1/import` (member role required).
See [`docs/IMPORT_FORMAT.md`](./docs/IMPORT_FORMAT.md) for the schema.

```bash
curl -X POST https://confero.example.org/api/v1/import \
  -H "Content-Type: text/x-yaml" \
  --cookie "session=<token>" \
  --data-binary @conferences.yaml
```

---

## API

The OpenAPI 3.1 specification lives in [`api/openapi.yaml`](./api/openapi.yaml).
All API changes must start there. `make generate` re-generates server stubs and
the TypeScript client. Bruno requests for every operation live in `bruno/confero/`.

---

## Architecture

| Layer | Technology |
|-------|-----------|
| HTTP router | go-chi/chi v5 |
| OpenAPI server codegen | oapi-codegen v2 |
| Database driver | pgx/v5 via pgxpool |
| Query codegen | sqlc |
| Migrations | golang-migrate |
| Auth | Keycloak OIDC + HS256 JWT (HttpOnly cookie) |
| Logging | stdlib slog (JSON) |
| Metrics | Prometheus |
| Scheduler | in-process goroutine (single replica) |
| Frontend | React 18 + TypeScript + Vite + Tailwind + Radix UI |
| Client codegen | @hey-api/openapi-ts |

Full architecture rationale: [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md).

---

## Repo layout

```
api/        OpenAPI 3.1 spec (source of truth for the API)
server/     Go server - cmd/confero-server, internal/, db/
web/        React SPA - src/pages, src/features, src/components
bruno/      Bruno API collection (one .bru per operation)
deploy/     docker-compose (local dev) + Helm chart (k8s)
docs/       Design docs, ADRs, import format guide
```

---

## License

MIT. See `LICENSE`.
