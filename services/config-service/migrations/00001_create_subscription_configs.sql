-- +goose Up
CREATE TABLE subscription_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    device_id UUID NOT NULL,
    subscription_id UUID,
    token_hash TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active',
    client_type TEXT NOT NULL DEFAULT 'generic',
    format TEXT NOT NULL DEFAULT 'uri-list',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    last_user_agent TEXT,
    refresh_count BIGINT NOT NULL DEFAULT 0,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT subscription_tokens_status_check CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT subscription_tokens_refresh_count_check CHECK (refresh_count >= 0)
);

CREATE TABLE config_profiles (
    id UUID PRIMARY KEY,
    subscription_token_id UUID NOT NULL REFERENCES subscription_tokens (id) ON DELETE CASCADE,
    profile_version INTEGER NOT NULL,
    client_type TEXT NOT NULL,
    format TEXT NOT NULL,
    server_count INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL,
    node_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,

    CONSTRAINT config_profiles_version_check CHECK (profile_version > 0),
    CONSTRAINT config_profiles_server_count_check CHECK (server_count >= 0)
);

CREATE TABLE subscription_refresh_events (
    id UUID PRIMARY KEY,
    subscription_token_id UUID NOT NULL REFERENCES subscription_tokens (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    device_id UUID NOT NULL,
    client_type TEXT,
    user_agent TEXT,
    source_ip_hash TEXT,
    returned_profile_version INTEGER,
    returned_server_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT subscription_refresh_events_server_count_check CHECK (returned_server_count >= 0)
);

CREATE INDEX subscription_tokens_user_id_idx ON subscription_tokens (user_id);
CREATE INDEX subscription_tokens_device_id_idx ON subscription_tokens (device_id);
CREATE INDEX subscription_tokens_status_idx ON subscription_tokens (status);
CREATE INDEX subscription_tokens_expires_at_idx ON subscription_tokens (expires_at);
CREATE INDEX config_profiles_token_version_idx ON config_profiles (subscription_token_id, profile_version DESC);
CREATE INDEX subscription_refresh_events_token_created_idx ON subscription_refresh_events (subscription_token_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS subscription_refresh_events_token_created_idx;
DROP INDEX IF EXISTS config_profiles_token_version_idx;
DROP INDEX IF EXISTS subscription_tokens_expires_at_idx;
DROP INDEX IF EXISTS subscription_tokens_status_idx;
DROP INDEX IF EXISTS subscription_tokens_device_id_idx;
DROP INDEX IF EXISTS subscription_tokens_user_id_idx;
DROP TABLE IF EXISTS subscription_refresh_events;
DROP TABLE IF EXISTS config_profiles;
DROP TABLE IF EXISTS subscription_tokens;
