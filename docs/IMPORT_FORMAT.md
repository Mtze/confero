# Confero YAML Import Format

The `/api/v1/import` endpoint accepts a YAML document describing one or more conferences.
Only members (and admins) may call it. The import is idempotent: a conference is matched
by `(LOWER(acronym), year)`; matching records are updated, new ones are created.

## Top-level structure

```yaml
conferences:
  - <ConferenceEntry>
  - <ConferenceEntry>
  ...
```

The document must have a `conferences` key at the root. Unknown top-level keys or unknown
fields inside a `ConferenceEntry` are rejected with HTTP 400.

## ConferenceEntry fields

| Field                 | Type    | Required | Notes |
|-----------------------|---------|----------|-------|
| `name`                | string  | yes      | 1-255 chars |
| `acronym`             | string  | yes      | 1-50 chars, matched case-insensitively |
| `year`                | integer | yes      | 2000-2100 |
| `location`            | string  | yes      | |
| `website_url`         | string  | no       | |
| `cfp_url`             | string  | no       | |
| `dblp_key`            | string  | no       | |
| `notes`               | string  | no       | |
| `core_rank`           | string  | no       | e.g. `A*`, `A`, `B` |
| `h5_index`            | integer | no       | |
| `acceptance_rate_pct` | number  | no       | 0-100 |
| `primary_deadline`    | string  | no       | RFC 3339 datetime or `YYYY-MM-DD` |
| `abstract_deadline`   | string  | no       | same formats |
| `notification_date`   | string  | no       | same formats |
| `camera_ready_date`   | string  | no       | same formats |
| `event_start_date`    | string  | no       | `YYYY-MM-DD` |
| `event_end_date`      | string  | no       | `YYYY-MM-DD` |
| `tags`                | list    | no       | strings |
| `tracks`              | list    | no       | strings |

## Behavior

- **Strict mode** (default): the first validation error aborts the entire import. The
  response is HTTP 200 with `errors` containing the first error message and
  `created`/`updated`/`skipped` all zero.
- **Idempotent**: submitting the same document twice creates on the first call and
  updates (no-op update) on the second. The conference data is fully replaced on update.
- **Upsert key**: `(LOWER(acronym), year)`. Two entries with the same acronym+year in
  one batch are not allowed - the second will overwrite the first.

## Response

HTTP 200:

```json
{
  "created": 3,
  "updated": 1,
  "skipped": 0,
  "errors": []
}
```

HTTP 400 is returned only for YAML parse errors (malformed YAML, unknown fields).

## Example

```yaml
conferences:
  - name: SIGCSE Technical Symposium
    acronym: SIGCSE
    year: 2027
    location: Pittsburgh, PA
    website_url: https://sigcse2027.sigcse.org
    core_rank: A
    primary_deadline: 2026-09-15T23:59:00Z
    abstract_deadline: 2026-09-08T23:59:00Z
    event_start_date: 2027-03-08
    event_end_date: 2027-03-12
    tags: [CS education, pedagogy]
    tracks: []
```
