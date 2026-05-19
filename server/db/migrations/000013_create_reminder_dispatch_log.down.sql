DROP TRIGGER IF EXISTS trg_reminder_dispatch_log_updated_at ON reminder_dispatch_log;
DROP INDEX IF EXISTS ix_reminder_dispatch_due;
DROP TABLE IF EXISTS reminder_dispatch_log;
