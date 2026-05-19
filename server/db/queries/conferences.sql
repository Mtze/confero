-- name: ListConferences :many
SELECT id, name, acronym, year, location, website_url, cfp_url,
       primary_deadline, abstract_deadline, notification_date, camera_ready_date,
       event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
       dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at
FROM conferences
WHERE archived_at IS NULL
ORDER BY primary_deadline ASC NULLS LAST, name ASC;

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
    @dblp_key, @notes, @created_by, @created_by
)
RETURNING id, name, acronym, year, location, website_url, cfp_url,
          primary_deadline, abstract_deadline, notification_date, camera_ready_date,
          event_start_date, event_end_date, core_rank, h5_index, acceptance_rate_pct,
          dblp_key, notes, archived_at, created_by, updated_by, created_at, updated_at;
