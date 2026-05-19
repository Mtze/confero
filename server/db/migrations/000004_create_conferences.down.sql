DROP TRIGGER IF EXISTS trg_conferences_updated_at ON conferences;
DROP INDEX IF EXISTS ix_conferences_archived_at;
DROP INDEX IF EXISTS ix_conferences_event_end_date;
DROP INDEX IF EXISTS ix_conferences_primary_deadline_active;
DROP INDEX IF EXISTS uq_conferences_acronym_year;
DROP TABLE IF EXISTS conferences;
