CREATE TABLE tracks (
    code         text        PRIMARY KEY,
    display_name text        NOT NULL,
    sort_order   int         NOT NULL DEFAULT 100,
    created_at   timestamptz NOT NULL DEFAULT now()
);
