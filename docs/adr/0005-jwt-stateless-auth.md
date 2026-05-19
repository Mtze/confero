# ADR 0005 - Stateless JWT-based session auth (no Postgres per-request)

**Status:** Accepted

## Context

Confero needs to authenticate API requests without the overhead of a
per-request database lookup. The chair is small; we don't need server-
side session revocation for normal operation. We also need to avoid
introducing an external session store (Redis, etc.) to keep the
deployment simple.

D27 in REQUIREMENTS.md locks this as a decision: "stateless auth".

## Decision

After a successful OIDC callback, the server issues a short-lived
HS256-signed JWT (TTL 1 hour) and sets it as an HttpOnly cookie.

Every subsequent request is authenticated by:
1. Parsing the `session` cookie.
2. Verifying the HMAC signature and expiry (no I/O).
3. Extracting `SessionClaims` (sub = users.id, email, name, roles)
   into the request context.

No session rows are stored or read from Postgres after login.

**Revocation:** rotating `CONFERO_SESSION_SECRET` invalidates all
tokens immediately. For individual user revocation, removing the user
from the IdP group means their next OIDC flow returns a 403 (not a
chair member), and their current token expires within 1 hour. This
is acceptable for v1.

**Cookie flags:** `HttpOnly; Secure; SameSite=Lax; Path=/`.
SameSite=Lax (not Strict) allows the IdP redirect to set the cookie
after the OIDC callback (cross-site top-level navigation).

## Consequences

- Auth verification is O(1) CPU — no DB reads per request.
- Token lifetime leaks are bounded to 1 hour.
- Logout is best-effort (cookie cleared on the client; token remains
  technically valid until expiry).
- `CONFERO_SESSION_SECRET` must be ≥32 bytes and treated as a secret.
  It is mounted from a Kubernetes Secret in production.
- If we ever need hard revocation (e.g., GDPR erasure), we would add
  a denylist table checked only when revocation is needed — not on
  every request.
