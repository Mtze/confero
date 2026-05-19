CREATE TABLE conference_tags (
    conference_id uuid NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
    tag_id        uuid NOT NULL REFERENCES tags(id)        ON DELETE CASCADE,
    PRIMARY KEY (conference_id, tag_id)
);

CREATE INDEX ix_conference_tags_tag ON conference_tags (tag_id);
