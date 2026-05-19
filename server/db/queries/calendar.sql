-- name: GetCalendarTokenByValue :one
UPDATE user_calendar_tokens
SET last_used_at = now()
WHERE token = @token AND revoked_at IS NULL
RETURNING id, user_id, token, kind, last_used_at, revoked_at, created_at;

-- name: ListCalendarTokensByUser :many
SELECT id, user_id, token, kind, last_used_at, revoked_at, created_at
FROM user_calendar_tokens
WHERE user_id = @user_id AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: CreateCalendarToken :one
INSERT INTO user_calendar_tokens (user_id, token, kind)
VALUES (@user_id, @token, @kind)
RETURNING id, user_id, token, kind, last_used_at, revoked_at, created_at;

-- name: RevokeCalendarTokensByUser :exec
UPDATE user_calendar_tokens
SET revoked_at = now()
WHERE user_id = @user_id AND kind = @kind AND revoked_at IS NULL;
