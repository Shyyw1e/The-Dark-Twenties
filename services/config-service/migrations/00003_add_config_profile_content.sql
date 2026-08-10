-- +goose Up
ALTER TABLE config_profiles
    ADD COLUMN content TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE config_profiles
    DROP COLUMN IF EXISTS content;
