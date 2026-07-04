-- +goose Up
CREATE TABLE plans (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    price_amount BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB',
    duration_days INTEGER NOT NULL,
    device_limit INTEGER NOT NULL,
    traffic_limit_bytes BIGINT,
    is_trial BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT plans_price_amount_check CHECK (price_amount >= 0),
    CONSTRAINT plans_duration_days_check CHECK (duration_days > 0),
    CONSTRAINT plans_device_limit_check CHECK (device_limit > 0),
    CONSTRAINT plans_traffic_limit_bytes_check CHECK (traffic_limit_bytes IS NULL OR traffic_limit_bytes > 0)
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    plan_id UUID NOT NULL REFERENCES plans (id),
    status TEXT NOT NULL DEFAULT 'active',
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    auto_renew BOOLEAN NOT NULL DEFAULT false,
    trial BOOLEAN NOT NULL DEFAULT false,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT subscriptions_status_check CHECK (status IN ('active', 'expired', 'cancelled', 'suspended')),
    CONSTRAINT subscriptions_period_check CHECK (expires_at > starts_at)
);

CREATE INDEX subscriptions_user_id_idx ON subscriptions (user_id);
CREATE INDEX subscriptions_plan_id_idx ON subscriptions (plan_id);
CREATE INDEX subscriptions_status_idx ON subscriptions (status);
CREATE INDEX subscriptions_expires_at_idx ON subscriptions (expires_at);

-- +goose Down
DROP INDEX IF EXISTS subscriptions_expires_at_idx;
DROP INDEX IF EXISTS subscriptions_status_idx;
DROP INDEX IF EXISTS subscriptions_plan_id_idx;
DROP INDEX IF EXISTS subscriptions_user_id_idx;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
