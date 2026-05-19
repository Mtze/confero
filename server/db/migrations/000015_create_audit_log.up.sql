CREATE TABLE audit_log (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id      uuid        REFERENCES users(id) ON DELETE SET NULL,
    actor_display_name text        NOT NULL,
    actor_oidc_subject text        NOT NULL,
    action             text        NOT NULL CHECK (action IN ('create','update','delete','archive','unarchive')),
    entity_type        text        NOT NULL,
    entity_id          uuid        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_audit_log_entity
    ON audit_log (entity_type, entity_id, created_at DESC);

CREATE INDEX ix_audit_log_actor
    ON audit_log (actor_user_id, created_at DESC);

CREATE INDEX ix_audit_log_created_at
    ON audit_log (created_at DESC);
