-- name: CreateStar :exec
INSERT INTO stars (user_id, conference_id)
VALUES (@user_id, @conference_id)
ON CONFLICT DO NOTHING;

-- name: DeleteStar :execrows
DELETE FROM stars
WHERE user_id = @user_id AND conference_id = @conference_id;

-- name: GetStar :one
SELECT user_id, conference_id, created_at
FROM stars
WHERE user_id = @user_id AND conference_id = @conference_id;

-- name: ListUserStarredConferences :many
SELECT c.*
FROM conferences c
INNER JOIN stars s ON s.conference_id = c.id
WHERE s.user_id = @user_id
ORDER BY c.primary_deadline ASC NULLS LAST, c.name ASC;

-- name: ListUsersStarringConferenceWithSettings :many
SELECT s.user_id, us.reminder_lead_days, us.timezone
FROM stars s
JOIN user_settings us ON us.user_id = s.user_id
WHERE s.conference_id = @conference_id;
