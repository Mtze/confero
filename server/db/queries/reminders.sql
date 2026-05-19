-- name: InsertReminderRow :exec
INSERT INTO reminder_dispatch_log
    (user_id, conference_id, deadline_kind, lead_time_days, scheduled_for)
VALUES (@user_id, @conference_id, @deadline_kind, @lead_time_days, @scheduled_for)
ON CONFLICT (user_id, conference_id, deadline_kind, lead_time_days) DO NOTHING;

-- name: CancelUserConferenceReminders :exec
-- Used when unstarring: preserves history, prevents re-send of the same lead_time.
UPDATE reminder_dispatch_log
SET status     = 'cancelled',
    updated_at = now()
WHERE status = 'pending'
  AND user_id       = @user_id
  AND conference_id = @conference_id;

-- name: DeleteConferencePendingReminders :exec
-- Used when re-materializing after a deadline change: deletes so fresh rows can be inserted.
DELETE FROM reminder_dispatch_log
WHERE status = 'pending'
  AND conference_id = @conference_id;

-- name: DeleteUserConferencePendingReminders :exec
-- Used when re-materializing a single (user, conference) pair after settings change.
DELETE FROM reminder_dispatch_log
WHERE status = 'pending'
  AND user_id       = @user_id
  AND conference_id = @conference_id;

-- name: CancelConferenceReminders :exec
-- Used when archiving: keeps history of planned-but-cancelled reminders.
UPDATE reminder_dispatch_log
SET status     = 'cancelled',
    updated_at = now()
WHERE status = 'pending'
  AND conference_id = @conference_id;

-- name: CountPendingRemindersForUser :one
SELECT COUNT(*) FROM reminder_dispatch_log
WHERE user_id = @user_id AND status = 'pending';

-- name: CountReminderRows :one
SELECT COUNT(*) FROM reminder_dispatch_log
WHERE user_id = @user_id AND conference_id = @conference_id;
