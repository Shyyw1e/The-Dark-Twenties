-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    language_code TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    is_admin BOOLEAN NOT NULL DEFAULT false,
    blocked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_status_check CHECK (status IN ('active', 'blocked', 'deleted'))
);

CREATE INDEX users_status_idx ON users (status);
CREATE INDEX users_created_at_idx ON users (created_at);

-- +goose Down
DROP INDEX IF EXISTS users_created_at_idx;
DROP INDEX IF EXISTS users_status_idx;
DROP TABLE IF EXISTS users;
