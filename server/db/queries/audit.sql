-- name: InsertAuditEntry :exec
INSERT INTO audit_log (actor_user_id, actor_display_name, actor_oidc_subject, action, entity_type, entity_id)
VALUES (@actor_user_id, @actor_display_name, @actor_oidc_subject, @action, @entity_type, @entity_id);

-- name: ListAuditLog :many
SELECT id, actor_user_id, actor_display_name, actor_oidc_subject, action, entity_type, entity_id, created_at
FROM audit_log
WHERE (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type')::text)
  AND (sqlc.narg('entity_id')::uuid IS NULL OR entity_id = sqlc.narg('entity_id')::uuid)
  AND (sqlc.narg('actor_oidc_subject')::text IS NULL OR actor_oidc_subject = sqlc.narg('actor_oidc_subject')::text)
  AND (sqlc.narg('before')::timestamptz IS NULL OR created_at < sqlc.narg('before')::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.narg('limit_val')::int;
