DROP TRIGGER IF EXISTS trg_digest_dispatch_log_updated_at ON digest_dispatch_log;
DROP INDEX IF EXISTS ix_digest_dispatch_due;
DROP TABLE IF EXISTS digest_dispatch_log;
