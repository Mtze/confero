# ADR 0007 - Flat trust model: members write, admins delete

**Status:** Accepted

## Context

Confero needs an authorization model for the conference catalog. The
chair is a small, high-trust group: everyone who is an OIDC member is
known to the chair leadership. The design document (D4 / D22) calls for
a simple two-level model rather than per-resource ownership checks.

REQUIREMENTS.md FR-2 says: authenticated chair members can create, edit,
and archive conferences. FR-3a implied that any member should be able to
edit any conference (no ownership concept). D22 says delete is admin-only
to protect against accidental permanent loss.

## Decision

- **Read** (list, get): public, no authentication required (D6).
- **Write** (create, update, archive, unarchive): any `member` role can
  perform these operations on any conference. There is no per-resource
  ownership check - any chair member can edit any conference.
- **Delete**: admin role required (`RequireAdmin` middleware). A member
  attempting DELETE receives 403.

The `RequireToken` → `RequireMember` → `RequireAdmin` middleware chain is
applied at the chi router level, before the strict handler, so role
enforcement is always enforced regardless of handler implementation.

## Consequences

- Simple to reason about: one middleware layer per role, checked once.
- No ownership column on conferences (no `owner_id`); `created_by` and
  `updated_by` are for audit only, not for authorization.
- A member who creates a conference can have it edited or archived by
  any other member. This is intentional for a collaborative tool.
- If per-resource ownership is needed later, it can be added by reading
  the `created_by` field in the handler and comparing with the session
  subject — without a schema change.
- Delete cascades are handled by ON DELETE CASCADE in the DB schema
  (stars, tags, tracks, reminder rows all cascade on conference delete).
