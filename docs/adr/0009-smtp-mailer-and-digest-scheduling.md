# ADR 0009 - SMTP mailer and digest scheduling

**Status:** Accepted

## Context

M7 introduces real email delivery via SMTP and a weekly digest feature.
Several design choices needed to be made explicit.

## Decision

### 1. Pre-rendered bodies in `mail.Message`

`mail.Message` carries `BodyText` and `BodyHTML` (pre-rendered strings)
rather than template data. Rendering happens in the scheduler before
calling `Mailer.Send`. This keeps the `Mailer` interface minimal (any
transport provider only needs to deliver opaque strings) and makes it
easy to test rendering independently of transport.

### 2. Embedded templates via `embed.FS`

Templates live in `internal/mail/templates/` and are embedded at
compile time with `//go:embed`. This avoids runtime file-path concerns
and allows golden-file tests to render against exactly the same
templates the binary uses.

### 3. SMTP mailer: one connection per message, STARTTLS

`SMTPMailer.Send` opens a new TCP connection per message. At TU chair
scale (single-digit emails per tick) this is simpler than maintaining a
persistent SMTP session and avoids reconnection logic. STARTTLS is
attempted if the server advertises the extension; AUTH is skipped if
username is empty.

### 4. Digest probe: SQL timezone comparison with `timezone()` function

The digest probe (`scheduleDigests`) must match the current hour in each
user's IANA timezone. The SQL uses:

```sql
EXTRACT(DOW  FROM timezone(us.timezone, @now)) = us.weekly_digest_day
EXTRACT(HOUR FROM timezone(us.timezone, @now)) = us.weekly_digest_hour
```

`timezone(zone, ts)` is preferred over `ts AT TIME ZONE zone` because
sqlc's parser cannot handle the named-parameter + `AT TIME ZONE`
combination. The two forms are semantically equivalent in PostgreSQL.

The clock is injected via `Config.Now`, so tests can drive the probe to
any point in time without sleeping.

### 5. `week_starting` = Monday of the UTC week

Per DATA_MODEL §3.9, `week_starting` is "Monday of the week, in the
user's timezone". In v1 all chair members are in Europe/Berlin, so UTC
Monday midnight is used as a pragmatic simplification. The UNIQUE
constraint `(user_id, week_starting)` ensures at-most-one digest per
user per week regardless of how many times the probe fires.

### 6. Digest dispatch: 'skipped' status for empty weeks

If a user has no upcoming deadlines within their configured horizon,
the dispatch marks the row `status='skipped'` rather than `status='sent'`.
This prevents the probe from re-inserting the row on the next tick while
also clearly distinguishing "sent a real email" from "nothing to send".

## Consequences

- SMTP credentials are optional at startup; if `CONFERO_SMTP_ADDR` is
  unset, the `FakeMailer` is used and emails are logged but not sent.
- Template golden files in `internal/mail/testdata/` lock rendering
  output. Update them with `UPDATE_GOLDEN=1 go test ./internal/mail/...`
  when templates change intentionally.
- The digest probe runs every scheduler tick (60 s by default). The SQL
  `UNIQUE` constraint and `ON CONFLICT DO NOTHING` ensure correctness
  even if the probe fires multiple times within the same hour.
