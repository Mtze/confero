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
- **Email reminders** - digest emails N days before each starred deadline
- **ICS feeds** - `/calendar/all.ics` for all active conferences; per-user feeds for
  starred conferences via secure token URLs
- **Bulk import** - upsert conferences from a YAML file (`POST /api/v1/import`)
- **Audit log** - every create/update/archive/delete is logged with actor info (admin view)
- **OpenAPI-first** - `api/openapi.yaml` is the single source of truth; client and server
  code is generated from it

---

## Local development (full stack via Docker)

The simplest way to run everything is with Docker Compose. All services start with a
single command and the app is accessible at `http://localhost:3000`.

**Prerequisites:** Docker Desktop or OrbStack, `make`.

```bash
git clone https://github.com/your-org/confero
cd confero
make dev          # builds images and starts all services
```

Open `http://localhost:3000` and click **Login**.

### Test credentials

| Username | Password | Role |
|----------|----------|------|
| `member` | `confero` | Can create/edit conferences, star, import |
| `admin`  | `confero` | All of the above + delete + audit log |

### Service URLs

| Service | URL | Notes |
|---------|-----|-------|
| Web app | `http://localhost:3000` | nginx serving the React SPA + proxying API |
| API server | `http://localhost:8080` | Go server (direct access, no auth UI) |
| Keycloak | `http://localhost:8180` | Admin console: admin / admin |
| MailHog | `http://localhost:8025` | Catches all outbound email |
| Postgres | `localhost:5432` | confero / confero / confero |

---

## Local development (services only, hot-reload)

Run only the backing services in Docker and the server/web natively for faster
iteration with hot-reload.

**Prerequisites:** Docker Desktop or OrbStack, Go 1.22+, pnpm 9+, `make`.

```bash
# 1. Start Postgres + Keycloak + MailHog
make dev-services

# 2. Apply migrations
make migrate-up

# 3. Start the Go server
cd server
CONFERO_DATABASE_URL=postgres://confero:confero@localhost:5432/confero?sslmode=disable \
CONFERO_OIDC_ISSUER_URL=http://host.docker.internal:8180/realms/confero \
CONFERO_OIDC_CLIENT_ID=confero \
CONFERO_OIDC_CLIENT_SECRET=confero-secret \
CONFERO_OIDC_REDIRECT_URL=http://localhost:8080/auth/callback \
CONFERO_OIDC_MEMBER_VALUE=cs-edu-chair \
CONFERO_OIDC_ADMIN_VALUE=cs-edu-chair-admin \
CONFERO_SESSION_SECRET=change-me-in-development-min32bytes! \
CONFERO_SMTP_ADDR=localhost:1025 \
CONFERO_SMTP_FROM=confero@example.org \
CONFERO_PUBLIC_BASE_URL=http://localhost:8080 \
go run ./cmd/confero-server

# 4. Start the Vite dev server (separate terminal)
cd web
pnpm install
pnpm dev
```

The app is at `http://localhost:5173` (Vite proxies `/api` and `/auth` to `:8080`).

---

## Running the test suite

```bash
make test     # Go tests (-race) + Vitest + helm-unittest
make lint     # golangci-lint + eslint + redocly lint + helm lint
make build    # build server and web Docker images
```

Go tests use `testcontainers-go` to spin up a real Postgres instance - no manual
setup needed beyond having Docker available. Frontend tests use Vitest + MSW (no
browser, no running server required).

### Running subsets

```bash
# Backend unit tests only (fast, no containers)
cd server && go test -race ./internal/...

# Backend API tests (spins up Postgres containers)
cd server && go test -race ./tests/...

# Frontend tests
cd web && pnpm test --run

# Single Go test
cd server && go test -race -run TestAuditLog_CreateConferenceWritesEntry ./tests/api/
```

---

## Code generation

The OpenAPI spec and sqlc queries are the sources of truth. Generated files are never
edited by hand.

```bash
make generate   # runs oapi-codegen + sqlc + @hey-api/openapi-ts
```

Run this after any change to `api/openapi.yaml` or `server/db/queries/*.sql`.
CI fails if the working tree is dirty after generation.

---

## Database migrations

```bash
make migrate-up                        # apply all pending migrations
make migrate-down                      # roll back one migration
make migrate-new name=add_something    # create a new up/down pair
```

Migration files live in `server/db/migrations/`. Each file is a plain SQL script.
The server runs `migrate-up` automatically on startup.

---

## Contributing

1. Read [`AGENTS.md`](./AGENTS.md) for the repo conventions (applies to humans too).
2. Read the relevant section of [`docs/IMPLEMENTATION_PLAN.md`](./docs/IMPLEMENTATION_PLAN.md).
3. All API changes start in `api/openapi.yaml`, then `make generate`.
4. All DB changes start with `make migrate-new`, then update `server/db/queries/`.
5. Tests are non-negotiable - see the [testing pact](./docs/IMPLEMENTATION_PLAN.md#2-testing-pact-non-negotiable).
6. Commit with [Conventional Commits](https://www.conventionalcommits.org/):
   `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`, `ci:`, `build:`.
7. Run `make lint && make test` before pushing.

Architecture decisions are recorded as ADRs in [`docs/adr/`](./docs/adr/).
If you make a decision not already captured there, add one.

---

## Deploying to Kubernetes

### Prerequisites

- Kubernetes cluster with the [CloudNativePG operator](https://cloudnative-pg.io/) installed
- An OIDC provider (Keycloak or any OpenID Connect-compatible IdP)
- An SMTP server for email reminders
- An ingress controller with TLS (e.g. cert-manager + nginx-ingress)

### Install with Helm

```bash
helm install confero oci://ghcr.io/your-org/charts/confero \
  --namespace confero --create-namespace \
  --set ingress.host=confero.example.org \
  --set ingress.tls.enabled=true \
  --set ingress.tls.secretName=confero-tls \
  --set oidc.issuerURL=https://keycloak.example.org/realms/confero \
  --set oidc.clientID=confero \
  --set oidc.clientSecret=<client-secret> \
  --set oidc.memberValue=cs-edu-chair \
  --set oidc.adminValue=cs-edu-chair-admin \
  --set server.sessionSecret=<32-byte-random-string> \
  --set server.publicBaseURL=https://confero.example.org \
  --set smtp.addr=smtp.example.org:587 \
  --set smtp.from=confero@example.org \
  --set smtp.username=<smtp-user> \
  --set smtp.password=<smtp-password> \
  --set postgres.password=<db-password>
```

### Key constraints

- `server.replicaCount` is locked to 1 - the reminder scheduler runs in-process.
  Scale-out requires extracting the scheduler first (see `docs/ARCHITECTURE.md §15`).
- The Helm chart provisions a CloudNativePG `Cluster` resource. The CNPG operator
  must be installed cluster-wide before deploying.

### Upgrading

```bash
helm upgrade confero oci://ghcr.io/your-org/charts/confero \
  --namespace confero \
  --reuse-values \
  --set image.tag=v0.2.0
```

The server applies database migrations automatically on startup. The deployment
strategy is `Recreate` (not `RollingUpdate`) to avoid two server instances running
migrations concurrently.

### Configuration reference

All available values and their defaults are documented in
[`deploy/helm/confero/values.yaml`](./deploy/helm/confero/values.yaml).
Validation constraints (required fields, value ranges) are in
[`deploy/helm/confero/values.schema.json`](./deploy/helm/confero/values.schema.json).

---

## Bulk import

Conferences can be imported from YAML via `POST /api/v1/import` (member role required).
See [`docs/IMPORT_FORMAT.md`](./docs/IMPORT_FORMAT.md) for the full schema.

```bash
curl -X POST https://confero.example.org/api/v1/import \
  -H "Content-Type: text/x-yaml" \
  --cookie "session=<token>" \
  --data-binary @conferences.yaml
```

---

## API

The OpenAPI 3.1 specification is at [`api/openapi.yaml`](./api/openapi.yaml).
Bruno requests for every operation are in [`bruno/confero/`](./bruno/confero/).

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
| Metrics | Prometheus (`/metrics`) |
| Scheduler | in-process goroutine (single replica only) |
| Frontend | React 18 + TypeScript + Vite + Tailwind CSS + Radix UI |
| Client codegen | @hey-api/openapi-ts |
| Deployment | Helm + CloudNativePG on Kubernetes |

Full rationale: [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md).

---

## Repo layout

```
api/        OpenAPI 3.1 spec (source of truth for the HTTP API)
server/     Go server - cmd/confero-server, internal/, db/
web/        React SPA - src/pages, src/features, src/components
bruno/      Bruno API collection (one .bru per operation)
deploy/     docker-compose (local dev) + Helm chart (Kubernetes)
docs/       Design docs, ADRs, import format guide
```

---

## License

MIT. See `LICENSE`.
