# Confero — Data Model (v0.1 draft)

**Status:** Draft for review — Phase 2 output
**Source of truth:** Postgres 15+, deployed via CloudNativePG.
**Migrations:** `golang-migrate`, plain SQL files in `db/migrations/`.
**Queries:** `sqlc` against the same schema.
**Last updated:** 2026-05-19

This document defines the relational schema for Confero, plus the
conventions and indexing decisions that go with it. It is the source
of truth for migrations and `sqlc` query writing.

---

## 1. Conventions

These apply to every table unless explicitly noted.

### 1.1 Primary keys

- All primary keys are `uuid` populated by `gen_random_uuid()`
  (from the `pgcrypto` extension).
- Rationale: stable external IDs that don't leak row counts, safe to
  expose in URLs (e.g. `/api/v1/conferences/{id}`), no clashing across
  environments during data export/import. The minor index-size cost
  is fine at this scale.

### 1.2 Timestamps

- Every row has `created_at timestamptz NOT NULL DEFAULT now()`.
- Every mutable row also has `updated_at timestamptz NOT NULL DEFAULT
  now()`, maintained by a trigger
  (`set_updated_at()` reused across tables).
- All times stored in UTC; the app converts to the viewer's local
  zone for display.

### 1.3 Naming

- Tables: plural snake_case (`conferences`, `user_calendar_tokens`).
- Columns: singular snake_case.
- Foreign keys: `{referenced_table_singular}_id`
  (e.g. `conference_id`).
- Indexes: `ix_{table}_{columns}`; unique: `uq_{table}_{columns}`;
  partial: `ix_{table}_{columns}_where_{predicate}`.
- Check constraints: `ck_{table}_{description}`.
- Junction tables: alphabetical concatenation of the two singulars
  (`conference_tags`, `conference_tracks`).

### 1.4 Delete behavior

No soft deletes. Deletes are real. The `audit_log` table preserves
deletion events with the full last-known snapshot, so we get history
without dragging tombstoned rows through every query. The one
exception is `conferences`, which are *archived* (soft hide), not
deleted, when their event date passes — but a member can still hard
delete a conference if they want it gone entirely.

### 1.5 Extensions

Required Postgres extensions:

- `pgcrypto` — `gen_random_uuid()`.
- `citext` — case-insensitive comparisons (optional; we use
  functional unique indexes instead in v1, but `citext` is loaded so
  we can opt in later without a migration).

### 1.6 Triggers

A single shared function:

```sql
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

…attached as `BEFORE UPDATE` to every mutable table.

---

## 2. Entity-relationship overview

```mermaid
erDiagram
    users ||--o{ stars : "stars"
    users ||--|| user_settings : "configures"
    users ||--o{ user_calendar_tokens : "owns"
    users ||--o{ reminder_dispatch_log : "receives"
    users ||--o{ digest_dispatch_log : "receives"
    users ||--o{ audit_log : "performs"

    conferences ||--o{ stars : "is starred by"
    conferences ||--o{ conference_tags : "tagged with"
    conferences ||--o{ conference_tracks : "accepts"
    conferences ||--o{ reminder_dispatch_log : "drives"

    tags ||--o{ conference_tags : "applied to"
    tracks ||--o{ conference_tracks : "accepted by"

    users {
        uuid id PK
        text oidc_subject
        text oidc_issuer
        text email
        text display_name
        timestamptz last_login_at
    }
    conferences {
        uuid id PK
        text name
        text acronym
        int year
        text location
        text website_url
        text cfp_url
        timestamptz primary_deadline
        timestamptz abstract_deadline
        timestamptz notification_date
        timestamptz camera_ready_date
        date event_start_date
        date event_end_date
        text core_rank
        int h5_index
        numeric acceptance_rate_pct
        text dblp_key
        text notes
        timestamptz archived_at
        uuid created_by FK
        uuid updated_by FK
    }
    stars {
        uuid user_id PK_FK
        uuid conference_id PK_FK
        timestamptz created_at
    }
    user_settings {
        uuid user_id PK_FK
        text timezone
        int[] reminder_lead_days
        bool weekly_digest_enabled
        smallint weekly_digest_day
        smallint weekly_digest_hour
        smallint weekly_digest_horizon_weeks
    }
    user_calendar_tokens {
        uuid id PK
        uuid user_id FK
        text token
        text kind
        timestamptz last_used_at
        timestamptz revoked_at
    }
    tags {
        uuid id PK
        text slug
        text name
    }
    conference_tags {
        uuid conference_id PK_FK
        uuid tag_id PK_FK
    }
    tracks {
        text code PK
        text display_name
        int sort_order
    }
    conference_tracks {
        uuid conference_id PK_FK
        text track_code PK_FK
    }
    reminder_dispatch_log {
        uuid id PK
        uuid user_id FK
        uuid conference_id FK
        text deadline_kind
        int lead_time_days
        timestamptz scheduled_for
        text status
        timestamptz sent_at
        int attempts
        text last_error
    }
    digest_dispatch_log {
        uuid id PK
        uuid user_id FK
        date week_starting
        timestamptz scheduled_for
        text status
        timestamptz sent_at
        int attempts
        text last_error
    }
    audit_log {
        uuid id PK
        uuid actor_user_id FK
        text actor_display_name
        text actor_oidc_subject
        text action
        text entity_type
        uuid entity_id
        timestamptz created_at
    }
```

If your viewer doesn't render Mermaid, the same information lives in
the per-table sections below.

---

## 3. Tables

### 3.1 `users`

Chair members provisioned on first OIDC login.

```sql
CREATE TABLE users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_issuer     text        NOT NULL,
    oidc_subject    text        NOT NULL,
    email           text        NOT NULL,
    display_name    text        NOT NULL,
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_oidc UNIQUE (oidc_issuer, oidc_subject),
    CONSTRAINT uq_users_email UNIQUE (email)
);
```

Notes:

- `(oidc_issuer, oidc_subject)` is the true federated identity. We
  also enforce email uniqueness for sanity, but `oidc_subject` is
  the join key on login.
- `display_name` is mutable upstream; we update on each login.
- No password column — auth is delegated to the OIDC provider.

### 3.2 `conferences`

The central entity. Flat: one row per upcoming iteration.

```sql
CREATE TABLE conferences (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    -- core
    name                 text        NOT NULL,
    acronym              text        NOT NULL,
    year                 int         NOT NULL CHECK (year BETWEEN 2000 AND 2100),
    location             text        NOT NULL, -- free text, e.g. "Munich, Germany" or "Virtual"
    website_url          text,
    cfp_url              text,

    -- deadlines (all timestamptz, all nullable except primary while drafting)
    primary_deadline     timestamptz,
    abstract_deadline    timestamptz,
    notification_date    timestamptz,
    camera_ready_date    timestamptz,

    -- event dates (date, not timestamp — conferences are described in days)
    event_start_date     date,
    event_end_date       date,

    -- metadata (all optional)
    core_rank            text CHECK (core_rank IS NULL OR core_rank IN ('A*','A','B','C','Unranked')),
    h5_index             int  CHECK (h5_index IS NULL OR h5_index >= 0),
    acceptance_rate_pct  numeric(5,2) CHECK (acceptance_rate_pct IS NULL OR (acceptance_rate_pct >= 0 AND acceptance_rate_pct <= 100)),
    dblp_key             text,

    -- categorization
    notes                text, -- markdown, sanitized on render

    -- lifecycle
    archived_at          timestamptz,

    -- audit / ownership
    created_by           uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_by           uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_conferences_event_dates
        CHECK (event_start_date IS NULL OR event_end_date IS NULL OR event_end_date >= event_start_date),
    CONSTRAINT ck_conferences_deadline_order
        CHECK (
            (abstract_deadline   IS NULL OR primary_deadline    IS NULL OR abstract_deadline   <= primary_deadline) AND
            (primary_deadline    IS NULL OR notification_date   IS NULL OR primary_deadline    <= notification_date) AND
            (notification_date   IS NULL OR camera_ready_date   IS NULL OR notification_date   <= camera_ready_date)
        )
);

CREATE UNIQUE INDEX uq_conferences_acronym_year
    ON conferences (LOWER(acronym), year);

CREATE INDEX ix_conferences_primary_deadline_active
    ON conferences (primary_deadline)
    WHERE archived_at IS NULL;

CREATE INDEX ix_conferences_event_end_date
    ON conferences (event_end_date)
    WHERE archived_at IS NULL;

CREATE INDEX ix_conferences_archived_at
    ON conferences (archived_at);
```

Notes:

- `acronym` uniqueness is case-insensitive via the functional index;
  the column itself preserves user-entered case for display.
- All deadlines are nullable so a conference can be created when a
  CFP is announced but exact dates aren't known yet.
- The deadline-order check is best-effort; we don't want false
  rejections, so any field being NULL skips that comparison.
- `ix_conferences_primary_deadline_active` makes the default list
  view ("upcoming, not archived, sorted by next deadline") a cheap
  range scan.

### 3.3 `tags` and `conference_tags`

Open-ended categorization (e.g. `cs-education`, `programming-tools`).

```sql
CREATE TABLE tags (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL,
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_tags_slug UNIQUE (slug)
);

CREATE TABLE conference_tags (
    conference_id uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    tag_id        uuid NOT NULL REFERENCES tags(id)        ON DELETE CASCADE,
    PRIMARY KEY (conference_id, tag_id)
);

CREATE INDEX ix_conference_tags_tag ON conference_tags (tag_id);
```

Notes:

- Tags are created lazily: when a user types a new tag in the conference
  form, the API upserts a `tags` row by slug.
- `slug` is `lower(name)` with non-alphanumerics replaced by `-`,
  collapsed. Generated server-side.
- **UX contract:** the tag input on the conference form is an
  autocomplete that fetches all existing tags up front and shows
  them as suggestions as the user types. Users are trusted to reuse
  existing tags rather than create near-duplicates. The API exposes
  `GET /api/v1/tags` for this; the endpoint is cheap (small table,
  cache-friendly).

### 3.4 `tracks` and `conference_tracks`

Closed but expandable set of submission tracks a venue accepts.

```sql
CREATE TABLE tracks (
    code         text PRIMARY KEY,                -- e.g. 'full_paper'
    display_name text NOT NULL,                   -- e.g. 'Full paper'
    sort_order   int  NOT NULL DEFAULT 100,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE conference_tracks (
    conference_id uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    track_code    text NOT NULL REFERENCES tracks(code)    ON DELETE RESTRICT,
    PRIMARY KEY (conference_id, track_code)
);

CREATE INDEX ix_conference_tracks_track ON conference_tracks (track_code);
```

Seed (a separate migration):

```sql
INSERT INTO tracks (code, display_name, sort_order) VALUES
    ('full_paper',          'Full paper',          10),
    ('short_paper',         'Short paper',         20),
    ('workshop',            'Workshop',            30),
    ('doctoral_consortium', 'Doctoral consortium', 40),
    ('demo',                'Demo',                50),
    ('journal_first',       'Journal-first',       60),
    ('poster',              'Poster',              70);
```

Notes:

- `text` PK (vs `uuid`) because we want stable, human-readable codes
  in API responses and the seed is small.
- New tracks can be added by inserting rows; nothing in the code
  hard-codes the closed set.

### 3.5 `stars`

User × conference; cheap and dense.

```sql
CREATE TABLE stars (
    user_id        uuid NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    conference_id  uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, conference_id)
);

-- Reverse lookup: "who starred this conference?"
CREATE INDEX ix_stars_conference ON stars (conference_id);
```

Notes:

- The PK `(user_id, conference_id)` enforces "one star per user per
  conference" and also serves the forward lookup ("what did this
  user star?").
- ON DELETE CASCADE on both FKs: removing a user or a conference
  removes their stars too. We capture the stars in the audit log
  before deleting.

### 3.6 `user_settings`

Per-user reminder preferences. One row per user, created on first
login alongside the `users` row.

```sql
CREATE TABLE user_settings (
    user_id                      uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone                     text        NOT NULL DEFAULT 'Europe/Berlin',
    reminder_lead_days           int[]       NOT NULL DEFAULT ARRAY[28, 14, 7, 1],
    weekly_digest_enabled        bool        NOT NULL DEFAULT false,
    weekly_digest_day            smallint    NOT NULL DEFAULT 1   CHECK (weekly_digest_day BETWEEN 0 AND 6),     -- 0=Sun..6=Sat
    weekly_digest_hour           smallint    NOT NULL DEFAULT 8   CHECK (weekly_digest_hour BETWEEN 0 AND 23),
    weekly_digest_horizon_weeks  smallint    NOT NULL DEFAULT 6   CHECK (weekly_digest_horizon_weeks BETWEEN 1 AND 52),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_user_settings_lead_days_sane
        CHECK (array_length(reminder_lead_days, 1) IS NULL
               OR (array_length(reminder_lead_days, 1) <= 10
                   AND NOT EXISTS (SELECT 1 FROM unnest(reminder_lead_days) v WHERE v < 0 OR v > 365)))
);
```

Notes:

- Lead times stored as a Postgres int array. Capped at 10 entries
  per user with a check constraint to prevent abuse.
- `timezone` is an IANA name; we validate at the API layer using Go's
  `time.LoadLocation`.

### 3.7 `user_calendar_tokens`

Per-user, per-feed-kind tokens for ICS subscriptions.

```sql
CREATE TABLE user_calendar_tokens (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token         text        NOT NULL,
    kind          text        NOT NULL CHECK (kind IN ('personal_starred')),
    last_used_at  timestamptz,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_calendar_tokens_token UNIQUE (token)
);

-- At most one active token per (user, kind).
CREATE UNIQUE INDEX uq_user_calendar_tokens_active
    ON user_calendar_tokens (user_id, kind)
    WHERE revoked_at IS NULL;

CREATE INDEX ix_user_calendar_tokens_user
    ON user_calendar_tokens (user_id);
```

Notes:

- `token` is a 32-byte random value, base64url-encoded, generated
  server-side (`crypto/rand`). Never reused after revocation.
- The partial unique index models "one live token per kind per user"
  while keeping the revoked history as separate rows for forensics.
- Public feed (`/calendar/all.ics`) needs no token; not represented
  here.
- The `kind` CHECK constraint is intentionally extensible — we'll
  alter it to include future feed kinds (e.g. `'all_with_personal_view'`)
  without dropping the column.

### 3.8 `reminder_dispatch_log`

Materialized schedule of per-deadline reminder emails. The worker
inserts rows when stars/conferences change and updates them on send.

```sql
CREATE TABLE reminder_dispatch_log (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    conference_id   uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    deadline_kind   text NOT NULL CHECK (deadline_kind IN
                        ('submission','abstract','notification','camera_ready')),
    lead_time_days  int  NOT NULL CHECK (lead_time_days >= 0 AND lead_time_days <= 365),
    scheduled_for   timestamptz NOT NULL,
    status          text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','sent','failed','cancelled')),
    sent_at         timestamptz,
    attempts        int  NOT NULL DEFAULT 0,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_reminder_dispatch
        UNIQUE (user_id, conference_id, deadline_kind, lead_time_days)
);

CREATE INDEX ix_reminder_dispatch_due
    ON reminder_dispatch_log (scheduled_for)
    WHERE status = 'pending';
```

Notes:

- The `UNIQUE (user_id, conference_id, deadline_kind, lead_time_days)`
  constraint is the heart of FR-15: we cannot accidentally send the
  same reminder twice.
- The worker picks up due rows with
  `SELECT … FOR UPDATE SKIP LOCKED` against
  `ix_reminder_dispatch_due` to support a future move to multiple
  worker replicas without code change.
- When a deadline is changed on a conference, we recompute its
  related rows by `DELETE … WHERE status = 'pending'` followed by
  re-inserting the new schedule. `sent` rows are immutable.
- When a star is removed, we
  `UPDATE … SET status='cancelled' WHERE status='pending' AND
  user_id=? AND conference_id=?`.

### 3.9 `digest_dispatch_log`

Per-user weekly digest send log.

```sql
CREATE TABLE digest_dispatch_log (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_starting  date NOT NULL,                  -- Monday of the week, in user's timezone
    scheduled_for  timestamptz NOT NULL,
    status         text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','sent','failed','cancelled','skipped')),
    sent_at        timestamptz,
    attempts       int  NOT NULL DEFAULT 0,
    last_error     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_digest_dispatch UNIQUE (user_id, week_starting)
);

CREATE INDEX ix_digest_dispatch_due
    ON digest_dispatch_log (scheduled_for)
    WHERE status = 'pending';
```

Notes:

- `'skipped'` covers "user is opted in but had no upcoming deadlines
  this week" — we still log a row so the scheduler doesn't try again.

### 3.10 `audit_log`

A lean, PII-minimal action log: **who** did **what** to **which
entity** and **when**. No field-level diffs, no emails, no IPs, no
user agents.

Visibility: rows in this table are **only** exposed to users with
the admin claim (see Requirements §3.8 / FR-22b). Non-admin
authenticated users cannot read it, and anonymous visitors cannot
see it at all.

```sql
CREATE TABLE audit_log (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id        uuid REFERENCES users(id) ON DELETE SET NULL,
    actor_display_name   text NOT NULL,    -- snapshot at action time
    actor_oidc_subject   text NOT NULL,    -- snapshot at action time
    action               text NOT NULL CHECK (action IN ('create','update','delete','archive','unarchive')),
    entity_type          text NOT NULL,    -- e.g. 'conference', 'star'
    entity_id            uuid NOT NULL,    -- not a FK: entity may be gone after delete
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_audit_log_entity
    ON audit_log (entity_type, entity_id, created_at DESC);

CREATE INDEX ix_audit_log_actor
    ON audit_log (actor_user_id, created_at DESC);

CREATE INDEX ix_audit_log_created_at
    ON audit_log (created_at DESC);
```

Notes:

- **PII minimization** (per requirement update): we capture only what
  is needed for accountability — display name, OIDC subject, action,
  timestamp. No email, no IP address, no User-Agent, no diff body.
- `actor_display_name` and `actor_oidc_subject` are **denormalized
  snapshots** at audit-write time, so the log remains attributable
  even if the user record is later deleted (`actor_user_id` then
  becomes NULL, but the snapshot fields survive).
- Writes go through a service-layer helper that records
  `(actor, action, entity_type, entity_id)` in the same transaction
  as the underlying change, so we never have an action without an
  audit row or vice versa.
- `entity_id` is deliberately not a FK so deletion history survives
  cascades.
- If we later want richer "what changed" detail (currently locked as
  out of scope per PII minimization), we'd add a separate
  `audit_log_change_detail` table keyed by `audit_log.id` and gate
  it behind an explicit retention policy.

---

## 4. Index summary

| Table                   | Index                                            | Purpose                                                |
| ----------------------- | ------------------------------------------------ | ------------------------------------------------------ |
| `users`                 | `uq_users_oidc(oidc_issuer, oidc_subject)`       | Login lookup.                                          |
| `users`                 | `uq_users_email`                                 | Sanity uniqueness; email search.                       |
| `conferences`           | `uq_conferences_acronym_year`                    | Prevent duplicate `(acronym, year)`.                   |
| `conferences`           | `ix_conferences_primary_deadline_active`         | Default list query.                                    |
| `conferences`           | `ix_conferences_event_end_date`                  | Archive sweeper.                                       |
| `tags`                  | `uq_tags_slug`                                   | Lazy tag creation by slug.                             |
| `conference_tags`       | PK + `ix_conference_tags_tag`                    | Filter by tag; list tags of a conf.                    |
| `tracks`                | PK on `code`                                     | Stable lookups.                                        |
| `conference_tracks`     | PK + `ix_conference_tracks_track`                | Filter by track.                                       |
| `stars`                 | PK `(user_id, conference_id)`                    | Forward lookup + uniqueness.                           |
| `stars`                 | `ix_stars_conference`                            | "Who starred this?"                                    |
| `user_calendar_tokens`  | `uq_user_calendar_tokens_token`                  | Feed URL → user lookup.                                |
| `user_calendar_tokens`  | `uq_user_calendar_tokens_active` (partial)       | One live token per kind per user.                      |
| `reminder_dispatch_log` | `uq_reminder_dispatch`                           | Idempotency (FR-15).                                   |
| `reminder_dispatch_log` | `ix_reminder_dispatch_due` (partial)             | Worker pickup.                                         |
| `digest_dispatch_log`   | `uq_digest_dispatch`                             | One digest per user per week.                          |
| `digest_dispatch_log`   | `ix_digest_dispatch_due` (partial)               | Worker pickup.                                         |
| `audit_log`             | `ix_audit_log_entity`                            | "History of this conference".                          |
| `audit_log`             | `ix_audit_log_actor`                             | "What did this user do?".                              |

---

## 5. Migration plan (golang-migrate)

Files live in `db/migrations/` and are numbered sequentially. Each
migration has a paired `up.sql` and `down.sql`.

```
000001_create_extensions.{up,down}.sql        -- pgcrypto, citext
000002_create_set_updated_at_trigger.{up,down}.sql
000003_create_users.{up,down}.sql
000004_create_conferences.{up,down}.sql
000005_create_tags.{up,down}.sql
000006_create_conference_tags.{up,down}.sql
000007_create_tracks.{up,down}.sql
000008_create_conference_tracks.{up,down}.sql
000009_seed_tracks.{up,down}.sql
000010_create_stars.{up,down}.sql
000011_create_user_settings.{up,down}.sql
000012_create_user_calendar_tokens.{up,down}.sql
000013_create_reminder_dispatch_log.{up,down}.sql
000014_create_digest_dispatch_log.{up,down}.sql
000015_create_audit_log.{up,down}.sql
```

Conventions:

- One conceptual change per migration. No "fix that previous one".
- Migrations are **forward-only** in production (we don't run `down`
  against prod); but `down.sql` exists for local dev and tests.
- Schema changes that need data migration get their own numbered
  file with a guarded `INSERT … SELECT` or `UPDATE`.
- We add a `schema_migrations` lock to prevent two API replicas
  racing on startup (golang-migrate handles this by default).

---

## 6. sqlc considerations

- `sqlc.yaml` will use the `postgresql` engine, the `pgx/v5` driver,
  and `emit_pointers_for_null_types: true` so nullable columns map
  to `*time.Time`, `*string`, etc. (cleaner than the
  `pgtype.Timestamptz` ergonomics for our handler code).
- Type overrides:

  ```yaml
  overrides:
    - db_type: "uuid"
      go_type:
        import: "github.com/google/uuid"
        type: "UUID"
    - db_type: "int4[]"
      go_type: "[]int32"
  ```

- Queries live in `db/queries/*.sql`, one file per entity. Each
  query has a name like `-- name: CreateConference :one` and uses
  named parameters.
- We avoid `SELECT *`; every query enumerates the columns it needs,
  so adding a column doesn't accidentally break call sites.
- Joins involving many-to-many (tags, tracks) return per-row data
  and we re-assemble in Go, rather than aggregating with
  `array_agg` in SQL — easier to test, only one query path.

---

## 7. Open questions

Resolved during phase 2 review:

- ~~Tag normalization~~ — **resolved:** trust-based, UX shows
  existing tags as autocomplete suggestions when typing.
- ~~`location_country` validation~~ — **resolved:** single free-text
  `location` field.
- ~~PII in audit log~~ — **resolved:** audit log drops the JSON diff
  entirely; stores only actor name + OIDC subject + action +
  timestamp. Visibility restricted to admin role.

Resolved since phase 2 review:

- ~~Admin role implementation~~ — **resolved:** single OIDC claim
  (build-time name, default `groups`), two runtime-configured values
  (`CONFERO_OIDC_MEMBER_VALUE`, `CONFERO_OIDC_ADMIN_VALUE`). No
  schema impact; `users` table unchanged.

Parked for phase 3 (frontend / API concerns, no schema impact):

- **Delete confirmation UX.** When an admin deletes a conference,
  the UI should show "this will also remove N stars" before
  proceeding. Schema already cascades; this is a frontend concern.
- **Feed-kind expansion.** Will we want a second per-user feed kind
  later (e.g. an "all conferences" feed scoped to a user's
  timezone)? Schema already supports it via `kind` column.
