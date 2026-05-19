# ADR 0008 - In-process scheduler

**Status:** Accepted

## Context

Confero must dispatch reminder emails and weekly digests on a schedule,
auto-archive expired conferences, and maintain Prometheus metrics for
pending work. The question is where to run this periodic logic.

Options considered:

1. **In-process goroutine** (chosen): a single background goroutine
   starts alongside the HTTP server, ticks at a configured interval,
   and shuts down via the same `context.Context` as the server.
2. **Separate process / Kubernetes CronJob**: would require a shared
   database schema for coordination, a separate Docker image, and
   additional Helm complexity.
3. **External job queue** (e.g., pg-boss, River): operational
   overhead and an additional dependency that is not justified for v1
   volume.

## Decision

Run the scheduler in-process inside `cmd/confero-server`. Key design
choices:

- `SELECT ... FOR UPDATE SKIP LOCKED` on `reminder_dispatch_log` and
  `digest_dispatch_log` ensures that even if two replicas exist, each
  row is picked up exactly once. The UNIQUE constraint on
  `(user_id, conference_id, deadline_kind, lead_time_days)` is the
  second line of defence against double-send.
- The server is locked to `replicaCount: 1` in the Helm chart with a
  `Recreate` deployment strategy (no concurrent replicas). This avoids
  wasted work from two schedulers racing, while the locking ensures
  correctness if the constraint is ever lifted.
- The `Mailer` interface is injected at construction time. In dev and
  in v1 production, a `FakeMailer` (which logs "would send") is used
  until the real SMTP mailer lands in M7.
- The `Now` function is injectable for clock-faking in tests.
- Tick, MaxAttempts, GraceDays are runtime-configurable via `Config`.
- `Tick()` and `SweepArchive()` are exported for white-box testing
  without starting the tick loop.

## Consequences

- Simple deployment: one process, one image.
- The extraction path to a separate scheduler process is documented in
  `ARCHITECTURE.md §15`. No code change is required for extraction
  because the FOR UPDATE SKIP LOCKED locking is already in place.
- If the server crashes mid-tick, at-most-once delivery is preserved:
  the uncommitted transaction is rolled back, and the row stays
  'pending' for the next tick.
- `confero_scheduler_pending_reminders` gauge is registered on the
  default Prometheus registry and exposed at `/metrics`.
