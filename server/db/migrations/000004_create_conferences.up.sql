CREATE TABLE conferences (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),

    name                 text        NOT NULL,
    acronym              text        NOT NULL,
    year                 int         NOT NULL CHECK (year BETWEEN 2000 AND 2100),
    location             text        NOT NULL,
    website_url          text,
    cfp_url              text,

    primary_deadline     timestamptz,
    abstract_deadline    timestamptz,
    notification_date    timestamptz,
    camera_ready_date    timestamptz,

    event_start_date     date,
    event_end_date       date,

    core_rank            text CHECK (core_rank IS NULL OR core_rank IN ('A*','A','B','C','Unranked')),
    h5_index             int  CHECK (h5_index IS NULL OR h5_index >= 0),
    acceptance_rate_pct  numeric(5,2) CHECK (acceptance_rate_pct IS NULL OR (acceptance_rate_pct >= 0 AND acceptance_rate_pct <= 100)),
    dblp_key             text,

    notes                text,

    archived_at          timestamptz,

    created_by           uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_by           uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_conferences_event_dates
        CHECK (event_start_date IS NULL OR event_end_date IS NULL OR event_end_date >= event_start_date),
    CONSTRAINT ck_conferences_deadline_order
        CHECK (
            (abstract_deadline IS NULL OR primary_deadline IS NULL OR abstract_deadline <= primary_deadline) AND
            (primary_deadline IS NULL OR notification_date IS NULL OR primary_deadline <= notification_date) AND
            (notification_date IS NULL OR camera_ready_date IS NULL OR notification_date <= camera_ready_date)
        )
);

CREATE UNIQUE INDEX uq_conferences_acronym_year
    ON conferences (LOWER(acronym), year);

CREATE INDEX ix_conferences_primary_deadline_active
    ON conferences (primary_deadline)
    WHERE archived_at IS NULL;

CREATE INDEX ix_conferences_event_end_date
    ON conferences (event_end_date)
    WHERE archived_at IS NULL;

CREATE INDEX ix_conferences_archived_at
    ON conferences (archived_at);

CREATE TRIGGER trg_conferences_updated_at
    BEFORE UPDATE ON conferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
