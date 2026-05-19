-- name: SelectDueReminders :many
SELECT r.id,
       r.user_id,
       r.conference_id,
       r.deadline_kind,
       r.lead_time_days,
       r.scheduled_for,
       r.attempts,
       u.email              AS user_email,
       u.display_name       AS user_name,
       c.name               AS conference_name,
       c.acronym            AS conference_acronym,
       c.primary_deadline,
       c.abstract_deadline,
       c.notification_date,
       c.camera_ready_date
FROM reminder_dispatch_log r
JOIN users       u ON u.id = r.user_id
JOIN conferences c ON c.id = r.conference_id
WHERE r.status = 'pending'
  AND r.scheduled_for <= @now
ORDER BY r.scheduled_for
LIMIT 50
FOR UPDATE OF r SKIP LOCKED;

-- name: MarkReminderSent :exec
UPDATE reminder_dispatch_log
SET status     = 'sent',
    sent_at    = now(),
    updated_at = now()
WHERE id = @id;

-- name: IncrementReminderAttempt :exec
UPDATE reminder_dispatch_log
SET attempts   = attempts + 1,
    last_error = @last_error,
    updated_at = now()
WHERE id = @id;

-- name: MarkReminderFailed :exec
UPDATE reminder_dispatch_log
SET status     = 'failed',
    last_error = @last_error,
    updated_at = now()
WHERE id = @id;

-- name: CountPendingReminders :one
SELECT COUNT(*) FROM reminder_dispatch_log WHERE status = 'pending';

-- name: SelectExpiredConferences :many
SELECT id FROM conferences
WHERE archived_at IS NULL
  AND event_end_date IS NOT NULL
  AND event_end_date < (CURRENT_DATE - @grace_days::int)
FOR UPDATE SKIP LOCKED;

-- name: SelectDigestDueUsers :many
SELECT u.id AS user_id
FROM users u
JOIN user_settings us ON us.user_id = u.id
WHERE us.weekly_digest_enabled = true
  AND EXTRACT(DOW  FROM timezone(us.timezone, @now))::smallint = us.weekly_digest_day
  AND EXTRACT(HOUR FROM timezone(us.timezone, @now))::smallint = us.weekly_digest_hour
  AND NOT EXISTS (
      SELECT 1 FROM digest_dispatch_log d
      WHERE d.user_id = u.id AND d.week_starting = @week_starting
  );

-- name: InsertDigestRow :exec
INSERT INTO digest_dispatch_log (user_id, week_starting, scheduled_for)
VALUES (@user_id, @week_starting, @scheduled_for)
ON CONFLICT (user_id, week_starting) DO NOTHING;

-- name: SelectDueDigests :many
SELECT d.id,
       d.user_id,
       d.week_starting,
       d.scheduled_for,
       d.attempts,
       u.email        AS user_email,
       u.display_name AS user_name,
       us.weekly_digest_horizon_weeks
FROM digest_dispatch_log d
JOIN users        u  ON u.id  = d.user_id
JOIN user_settings us ON us.user_id = d.user_id
WHERE d.status = 'pending'
  AND d.scheduled_for <= @now
ORDER BY d.scheduled_for
LIMIT 50
FOR UPDATE OF d SKIP LOCKED;

-- name: MarkDigestSent :exec
UPDATE digest_dispatch_log
SET status     = 'sent',
    sent_at    = now(),
    updated_at = now()
WHERE id = @id;

-- name: MarkDigestSkipped :exec
UPDATE digest_dispatch_log
SET status     = 'skipped',
    updated_at = now()
WHERE id = @id;

-- name: IncrementDigestAttempt :exec
UPDATE digest_dispatch_log
SET attempts   = attempts + 1,
    last_error = @last_error,
    updated_at = now()
WHERE id = @id;

-- name: MarkDigestFailed :exec
UPDATE digest_dispatch_log
SET status     = 'failed',
    last_error = @last_error,
    updated_at = now()
WHERE id = @id;

-- name: CountPendingDigests :one
SELECT COUNT(*) FROM digest_dispatch_log WHERE status = 'pending';
