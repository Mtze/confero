# ADR 0003 - Codegen tools: oapi-codegen v2 + @hey-api/openapi-ts

**Status:** Accepted

## Context

ADR-0002 established that `api/openapi.yaml` is the single source of truth.
Both the Go server and the TypeScript SPA must derive their API types and
calling code from the spec automatically so that drift is caught at PR time,
not in production.

We need to pick concrete tools for each side.

## Decision

**Go server stubs:** `github.com/oapi-codegen/oapi-codegen/v2`

- Generates `server/internal/api/api.gen.go` from the spec.
- Configuration: `server/oapi-codegen.yaml` (chi-server + strict-server + models).
- Invocation: `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen` — no
  separate binary install, version pinned in `go.mod` via `server/tools.go`.
- The strict server mode (`NewStrictHandler`) produces typed request/response
  structs per operation, which enforces correct handler signatures at compile
  time and prevents forgetting to write a response.

**TypeScript client:** `@hey-api/openapi-ts` (v0.97.x with `@hey-api/client-fetch`)

- Generates `web/src/api/` (types, SDK, client plumbing).
- Configuration: `web/openapi-ts.config.ts`.
- Invocation: `node_modules/.bin/openapi-ts --config openapi-ts.config.ts` from
  the `web/` directory — pinned via pnpm lock file.
- The generated SDK is strongly typed end-to-end; every operation returns a
  typed response object, not `any`.

**Drift check in CI:** the `codegen-drift` job regenerates both outputs on every
push and fails if `git diff` is non-empty. This makes stale generated code a
hard CI failure, not a soft reminder.

## Consequences

- Contributors must run `make generate` after changing `api/openapi.yaml`.
- The generated directories (`server/internal/api/`, `web/src/api/`) are
  committed to git — this makes generated changes visible in PRs, enables
  codegen drift detection, and prevents having to regenerate code just to read
  it in CI.
- oapi-codegen emits a warning for OpenAPI 3.1.x (not yet fully supported);
  the generated code works correctly for our current endpoint set. We accept
  this trade-off and will revisit when full 3.1 support is released.
- Go minimum version bumped to 1.24 (required by oapi-codegen v2.7.0). This is
  within the "Go 1.22+" intent from ADR-0001, and Go 1.24 is backward
  compatible for all code we write.
