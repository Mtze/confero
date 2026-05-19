CREATE TABLE user_calendar_tokens (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        text        NOT NULL,
    kind         text        NOT NULL CHECK (kind IN ('personal_starred')),
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_calendar_tokens_token UNIQUE (token)
);

CREATE UNIQUE INDEX uq_user_calendar_tokens_active
    ON user_calendar_tokens (user_id, kind)
    WHERE revoked_at IS NULL;

CREATE INDEX ix_user_calendar_tokens_user
    ON user_calendar_tokens (user_id);
