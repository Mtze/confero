# ADR-0011: Helm chart layout and single-replica enforcement

**Status:** Accepted

## Context

Confero needs a production deployment artifact. The architecture document
(ARCHITECTURE.md §11) specifies a Helm chart as the delivery mechanism.
Key constraints from the design:

- The server runs in a single replica in v1 because the reminder scheduler
  is in-process (ADR-0008). Running multiple replicas would trigger
  duplicate reminder dispatch.
- Configuration is split between a ConfigMap (non-secret values) and
  Secret references (OIDC client secret, session secret, SMTP credentials).
- Postgres is managed by CNPG (ADR-0010), which generates its own connection
  secret.
- The ingress routes `/api`, `/auth`, `/calendar`, and `/healthz` to the
  server, and `/` to the web (nginx) container.

## Decision

The Helm chart at `deploy/helm/confero/` uses the following template layout:

| Template | Purpose |
|---|---|
| `_helpers.tpl` | `confero.fullname`, `confero.name`, `confero.labels`, `confero.selectorLabels`, `confero.serviceAccountName` |
| `cnpg-cluster.yaml` | `postgresql.cnpg.io/v1` Cluster, conditional on `postgres.enabled` |
| `configmap.yaml` | Non-secret environment variables for the server |
| `secret.yaml` | Derived secret holding the OIDC redirect URL (computed from `ingress.host`) |
| `server-deployment.yaml` | Server Deployment with envFrom the ConfigMap plus secretKeyRef env entries |
| `server-service.yaml` | ClusterIP Service for the server on port 8080 |
| `web-deployment.yaml` | Web (nginx) Deployment |
| `web-service.yaml` | ClusterIP Service for the web on port 8080 |
| `ingress.yaml` | Ingress with path-based routing, conditional on `ingress.enabled` |
| `NOTES.txt` | Post-install usage notes |

`server.replicaCount` is constrained to `minimum: 1, maximum: 1` in
`values.schema.json` so Helm rejects misconfigured installs at validation
time rather than at runtime.

helm-unittest tests live under `tests/unit/` and are run with
`helm unittest -f 'tests/unit/*_test.yaml'`. The test path is non-default
because `tests/snapshots/` holds snapshot files that should not be picked
up as test suites.

## Consequences

- Any operator who attempts `helm install --set server.replicaCount=2`
  will get a schema validation error immediately.
- Adding HorizontalPodAutoscaler support for the server in a future version
  requires removing the `maximum: 1` constraint from the schema and
  migrating the scheduler to a distributed lock (e.g. Postgres advisory
  locks). This is a deliberate future break point.
- The web Deployment has no `strategy` override; it defaults to
  `RollingUpdate`, which is appropriate because the web container is
  stateless nginx serving static assets.
- NetworkPolicy support is scaffolded (`networkPolicy.enabled`) but not
  implemented in v1; the value exists so chart consumers can enable it
  without a values file change when templates are added.
