-- name: UpsertUser :one
INSERT INTO users (oidc_issuer, oidc_subject, email, display_name, last_login_at)
VALUES (@oidc_issuer, @oidc_subject, @email, @display_name, now())
ON CONFLICT (oidc_issuer, oidc_subject) DO UPDATE
    SET email         = EXCLUDED.email,
        display_name  = EXCLUDED.display_name,
        last_login_at = now()
RETURNING id, oidc_issuer, oidc_subject, email, display_name, last_login_at, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, oidc_issuer, oidc_subject, email, display_name, last_login_at, created_at, updated_at
FROM users
WHERE id = @id;
