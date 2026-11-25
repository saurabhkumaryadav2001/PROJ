-- Day 19: persistent authentication tables. Never store plaintext passwords/tokens.
BEGIN;

CREATE TABLE IF NOT EXISTS app_users (
    id UUID PRIMARY KEY,
    email VARCHAR(254) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    disabled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_sessions (
    id UUID PRIMARY KEY,
    family_id UUID NOT NULL,
    token_hash CHAR(43) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    replaced_by_id UUID NULL REFERENCES refresh_sessions(id)
);

CREATE INDEX IF NOT EXISTS ix_refresh_sessions_user_id ON refresh_sessions(user_id);
CREATE INDEX IF NOT EXISTS ix_refresh_sessions_family_id ON refresh_sessions(family_id);
CREATE INDEX IF NOT EXISTS ix_refresh_sessions_expiry ON refresh_sessions(expires_at);
CREATE INDEX IF NOT EXISTS ix_refresh_sessions_active_family
    ON refresh_sessions(family_id) WHERE revoked_at IS NULL;

COMMIT;
