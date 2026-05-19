---
Status: Accepted
Date: 2026-05-19
---

# ADR-0012: Audit middleware uses mutable pointer in context

## Context

The audit middleware needs to know the entity ID that a handler operated on, so it can
write a row to `audit_log` after the handler returns a 2xx response.

`oapi-codegen`'s strict handler adapter calls the Go method with a `context.Context`
derived from `r.Context()` at the time the adapter runs. If the method calls
`context.WithValue(ctx, key, id)` the returned context is local to the method and never
reflected in the original `r.Context()`. The middleware calls `next.ServeHTTP(rr, r)`
and then reads from `r.Context()` - so any child context the handler creates is invisible
to it.

## Decision

Inject a pointer to a mutable `entityHolder` struct into the request context *before*
calling `next.ServeHTTP`. Handlers call `audit.MarkEntity(ctx, id)` which writes through
the pointer. The middleware reads from the same pointer via the original `r.Context()`
after the handler returns.

```
middleware:
  r = r.WithContext(initContext(r.Context()))  // injects *entityHolder
  next.ServeHTTP(rr, r)
  entityID, ok := entityFromContext(r.Context())  // reads *entityHolder

handler:
  audit.MarkEntity(ctx, id)  // writes to *entityHolder (no new context needed)
```

## Consequences

- `audit.MarkEntity` does not return a context (no assignment needed in handlers).
- The entity holder is write-once by convention; the last call wins if a handler calls
  MarkEntity more than once (not expected in practice).
- The pattern is an intentional exception to the Go "never store mutable values in
  context" guideline; it is isolated to the audit package and documented here.
