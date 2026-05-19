CREATE TABLE digest_dispatch_log (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_starting date        NOT NULL,
    scheduled_for timestamptz NOT NULL,
    status        text        NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending','sent','failed','cancelled','skipped')),
    sent_at       timestamptz,
    attempts      int         NOT NULL DEFAULT 0,
    last_error    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_digest_dispatch UNIQUE (user_id, week_starting)
);

CREATE INDEX ix_digest_dispatch_due
    ON digest_dispatch_log (scheduled_for)
    WHERE status = 'pending';

CREATE TRIGGER trg_digest_dispatch_log_updated_at
    BEFORE UPDATE ON digest_dispatch_log
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
