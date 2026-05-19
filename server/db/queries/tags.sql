-- name: ListTags :many
SELECT id, slug, name FROM tags ORDER BY slug;

-- name: GetTagBySlug :one
SELECT id, slug, name FROM tags WHERE slug = @slug;

-- name: UpsertTagBySlug :one
INSERT INTO tags (slug, name)
VALUES (@slug, @name)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
RETURNING id, slug, name;
