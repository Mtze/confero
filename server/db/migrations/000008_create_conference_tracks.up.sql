CREATE TABLE conference_tracks (
    conference_id uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    track_code    text NOT NULL REFERENCES tracks(code)    ON DELETE RESTRICT,
    PRIMARY KEY (conference_id, track_code)
);

CREATE INDEX ix_conference_tracks_track ON conference_tracks (track_code);
