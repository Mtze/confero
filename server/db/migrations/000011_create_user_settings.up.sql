CREATE TABLE user_settings (
    user_id                     uuid        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone                    text        NOT NULL DEFAULT 'Europe/Berlin',
    reminder_lead_days          int[]       NOT NULL DEFAULT ARRAY[28, 14, 7, 1],
    weekly_digest_enabled       bool        NOT NULL DEFAULT false,
    weekly_digest_day           smallint    NOT NULL DEFAULT 1   CHECK (weekly_digest_day BETWEEN 0 AND 6),
    weekly_digest_hour          smallint    NOT NULL DEFAULT 8   CHECK (weekly_digest_hour BETWEEN 0 AND 23),
    weekly_digest_horizon_weeks smallint    NOT NULL DEFAULT 6   CHECK (weekly_digest_horizon_weeks BETWEEN 1 AND 52),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_user_settings_lead_days_sane
        CHECK (array_length(reminder_lead_days, 1) IS NULL
               OR (array_length(reminder_lead_days, 1) <= 10
                   AND int_array_all_in_range(reminder_lead_days, 0, 365)))
);

CREATE TRIGGER trg_user_settings_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
