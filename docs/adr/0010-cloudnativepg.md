# ADR-0010: Use CloudNativePG for managed Postgres in production

**Status:** Accepted

## Context

Confero requires a Postgres 15+ database in production.
Running a bare Postgres StatefulSet in Kubernetes is operationally complex:
failover, connection pooling, backups, and credential rotation all need custom
glue. The TUM CS-Ed chair already operates Kubernetes and needs a low-overhead
managed solution that integrates natively with Helm and standard Kubernetes
secrets.

CloudNativePG (CNPG) is a CNCF sandbox operator that manages Postgres
instances as first-class Kubernetes resources via the `postgresql.cnpg.io/v1`
CRD. It handles replication, failover, streaming backups, and
automatic secret generation (the `<cluster>-app` secret contains the full
connection URI).

This decision locks requirement D15 from `docs/REQUIREMENTS.md`.

## Decision

Use CloudNativePG to manage Postgres in production.
The Confero Helm chart instantiates a `postgresql.cnpg.io/v1` Cluster
resource when `postgres.enabled=true`, but does **not** install the CNPG
operator itself. Operator installation is a cluster-level prerequisite
documented in the deployment guide.

The generated `<release>-app` secret is consumed directly by the server
Deployment as `CONFERO_DATABASE_URL`.

For local development, plain Postgres via docker-compose is used; CNPG is
not involved.

## Consequences

- Deployers must install the CNPG operator before deploying the Confero chart.
  The chart will fail at the Cluster resource if the CRD is absent.
- Connection pooling (PgBouncer) is not configured in v1; CNPG supports it
  as a sidecar if needed in a future milestone.
- Backup configuration is not included in the chart stub in v1; it should be
  added when persistent data backup requirements are finalised.
- The `postgres.enabled=false` path allows deploying against an external
  Postgres (e.g. a pre-existing CNPG cluster) by supplying
  `CONFERO_DATABASE_URL` via a separate secret.
