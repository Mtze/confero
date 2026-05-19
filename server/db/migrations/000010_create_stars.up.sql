CREATE TABLE stars (
    user_id       uuid        NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    conference_id uuid        NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, conference_id)
);

CREATE INDEX ix_stars_conference ON stars (conference_id);
