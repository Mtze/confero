-- name: SelectDueReminders :many
SELECT r.id,
       r.user_id,
       r.conference_id,
       r.deadline_kind,
       r.lead_time_days,
       r.scheduled_for,
       r.attempts,
       u.email        AS user_email,
       u.display_name AS user_name,
       c.name         AS conference_name,
       c.acronym      AS conference_acronym
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
