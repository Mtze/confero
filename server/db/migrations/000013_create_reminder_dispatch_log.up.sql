CREATE TABLE reminder_dispatch_log (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid        NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    conference_id  uuid        NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    deadline_kind  text        NOT NULL CHECK (deadline_kind IN ('submission','abstract','notification','camera_ready')),
    lead_time_days int         NOT NULL CHECK (lead_time_days >= 0 AND lead_time_days <= 365),
    scheduled_for  timestamptz NOT NULL,
    status         text        NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending','sent','failed','cancelled')),
    sent_at        timestamptz,
    attempts       int         NOT NULL DEFAULT 0,
    last_error     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_reminder_dispatch
        UNIQUE (user_id, conference_id, deadline_kind, lead_time_days)
);

CREATE INDEX ix_reminder_dispatch_due
    ON reminder_dispatch_log (scheduled_for)
    WHERE status = 'pending';

CREATE TRIGGER trg_reminder_dispatch_log_updated_at
    BEFORE UPDATE ON reminder_dispatch_log
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
