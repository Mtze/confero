CREATE TABLE users (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_issuer   text        NOT NULL,
    oidc_subject  text        NOT NULL,
    email         text        NOT NULL,
    display_name  text        NOT NULL,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_oidc  UNIQUE (oidc_issuer, oidc_subject),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
