# ADR 0006 - OIDC group claim name set at build time via -ldflags

**Status:** Accepted

## Context

Confero uses an OIDC claim (by default `groups`) to determine a user's
role within the chair. Different deployments may map roles to different
claim names (e.g., Keycloak may be configured to use `roles`,
`realm_access`, or a custom claim).

We want the claim name to be configurable without introducing a
runtime config value that can drift or be misconfigured silently.
REQUIREMENTS.md decision D3a / FR-22a locks this approach.

## Decision

`auth.OIDCClaimName` is a package-level variable in
`confero/internal/auth`, defaulting to `"groups"`. It is set at
build time via:

```
-ldflags "-X confero/internal/auth.OIDCClaimName=<name>"
```

The `server/Dockerfile` exposes this as `ARG OIDC_CLAIM_NAME=groups`
and passes it through the build command. The Helm chart exposes it as
`server.oidcClaimName` in `values.yaml`.

The two claim *values* (`CONFERO_OIDC_MEMBER_VALUE`,
`CONFERO_OIDC_ADMIN_VALUE`) remain runtime environment variables,
because they represent data (group names) rather than code structure.

## Consequences

- Changing the claim name requires a rebuild and redeploy. This is
  intentional: the claim name is a structural contract with the IdP,
  not a runtime tunable.
- The default (`groups`) works out of the box with Keycloak's
  standard group membership mapper.
- A wrong claim name produces `!roles.Member` → 403 for all users,
  which is immediately visible and easy to diagnose.
- Tests that exercise role decoding work without any ldflags because
  the default `"groups"` is always set correctly for the test realm.
