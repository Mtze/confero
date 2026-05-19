-- name: UpsertUserSettings :exec
INSERT INTO user_settings (user_id)
VALUES (@user_id)
ON CONFLICT (user_id) DO NOTHING;

-- name: GetUserSettings :one
SELECT user_id, timezone, reminder_lead_days,
       weekly_digest_enabled, weekly_digest_day, weekly_digest_hour,
       weekly_digest_horizon_weeks, updated_at
FROM user_settings
WHERE user_id = @user_id;

-- name: UpdateUserSettings :one
UPDATE user_settings
SET timezone                    = @timezone,
    reminder_lead_days          = @reminder_lead_days,
    weekly_digest_enabled       = @weekly_digest_enabled,
    weekly_digest_day           = @weekly_digest_day,
    weekly_digest_hour          = @weekly_digest_hour,
    weekly_digest_horizon_weeks = @weekly_digest_horizon_weeks,
    updated_at                  = now()
WHERE user_id = @user_id
RETURNING user_id, timezone, reminder_lead_days,
          weekly_digest_enabled, weekly_digest_day, weekly_digest_hour,
          weekly_digest_horizon_weeks, updated_at;
