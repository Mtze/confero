-- name: ListConferences :many
SELECT c.id, c.name, c.acronym, c.year, c.location, c.website_url, c.cfp_url,
       c.primary_deadline, c.abstract_deadline, c.notification_date, c.camera_ready_date,
       c.event_start_date, c.event_end_date, c.core_rank, c.h5_index, c.acceptance_rate_pct,
       c.dblp_key, c.notes, c.archived_at, c.created_by, c.updated_by, c.created_at, c.updated_at
FROM conferences c
WHERE (sqlc.narg('include_archived')::boolean IS TRUE OR c.archived_at IS NULL)
  AND (sqlc.narg('tag_slug')::text IS NULL
       OR EXISTS (
           SELECT 1 FROM conference_tags ct
           JOIN tags t ON t.id = ct.tag_id
           WHERE ct.conference_id = c.id AND t.slug = sqlc.narg('tag_slug')::text
       ))
  AND (sqlc.narg('track_code')::text IS NULL
       OR EXISTS (
           SELECT 1 FROM conference_tracks ctr
           WHERE ctr.conference_id = c.id AND ctr.track_code = sqlc.narg('track_code')::text
       ))
  AND (sqlc.narg('search')::text IS NULL
       OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
       OR c.acronym ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY c.primary_deadline ASC NULLS LAST, c.name ASC;

-- name: GetConference :one
SELECT id, name, acronym, year, location, website_url, cfp_url,
       primary_deadline, abstract_deadline, notification_date, camera_ready_date,
       event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
       dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at
FROM conferences
WHERE id = @id;

-- name: CreateConference :one
INSERT INTO conferences (
    name, acronym, year, location, website_url, cfp_url,
    primary_deadline, abstract_deadline, notification_date, camera_ready_date,
    event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
    dblp_key, notes, created_by, updated_by
) VALUES (
    @name, @acronym, @year, @location, @website_url, @cfp_url,
    @primary_deadline, @abstract_deadline, @notification_date, @camera_ready_date,
    @event_start_date, @event_end_date, @core_rank, @h5_index, @acceptance_rate_pct,
    @dblp_key, @notes, @created_by, @updated_by
)
RETURNING id, name, acronym, year, location, website_url, cfp_url,
          primary_deadline, abstract_deadline, notification_date, camera_ready_date,
          event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
          dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at;

-- name: UpdateConference :one
UPDATE conferences SET
    name              = @name,
    acronym           = @acronym,
    year              = @year,
    location          = @location,
    website_url       = @website_url,
    cfp_url           = @cfp_url,
    primary_deadline  = @primary_deadline,
    abstract_deadline = @abstract_deadline,
    notification_date = @notification_date,
    camera_ready_date = @camera_ready_date,
    event_start_date  = @event_start_date,
    event_end_date    = @event_end_date,
    core_rank         = @core_rank,
    h5_index          = @h5_index,
    acceptance_rate_pct = @acceptance_rate_pct,
    dblp_key          = @dblp_key,
    notes             = @notes,
    updated_by        = @updated_by
WHERE id = @id
RETURNING id, name, acronym, year, location, website_url, cfp_url,
          primary_deadline, abstract_deadline, notification_date, camera_ready_date,
          event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
          dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at;

-- name: DeleteConference :execrows
DELETE FROM conferences WHERE id = @id;

-- name: ArchiveConference :one
-- Idempotent: if already archived, keeps the original archived_at timestamp.
UPDATE conferences SET archived_at = COALESCE(archived_at, now()) WHERE id = @id
RETURNING id, name, acronym, year, location, website_url, cfp_url,
          primary_deadline, abstract_deadline, notification_date, camera_ready_date,
          event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
          dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at;

-- name: UnarchiveConference :one
-- Idempotent: clears archived_at whether it was set or not.
UPDATE conferences SET archived_at = NULL WHERE id = @id
RETURNING id, name, acronym, year, location, website_url, cfp_url,
          primary_deadline, abstract_deadline, notification_date, camera_ready_date,
          event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
          dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at;

-- name: GetConferenceTags :many
SELECT t.id, t.slug, t.name
FROM tags t
JOIN conference_tags ct ON ct.tag_id = t.id
WHERE ct.conference_id = @conference_id
ORDER BY t.slug;

-- name: GetConferenceTracks :many
SELECT tr.code, tr.display_name, tr.sort_order
FROM tracks tr
JOIN conference_tracks ct ON ct.track_code = tr.code
WHERE ct.conference_id = @conference_id
ORDER BY tr.sort_order, tr.code;

-- name: DeleteAllConferenceTags :exec
DELETE FROM conference_tags WHERE conference_id = @conference_id;

-- name: AddConferenceTag :exec
INSERT INTO conference_tags (conference_id, tag_id) VALUES (@conference_id, @tag_id)
ON CONFLICT DO NOTHING;

-- name: DeleteAllConferenceTracks :exec
DELETE FROM conference_tracks WHERE conference_id = @conference_id;

-- name: AddConferenceTrack :exec
INSERT INTO conference_tracks (conference_id, track_code) VALUES (@conference_id, @track_code)
ON CONFLICT DO NOTHING;
