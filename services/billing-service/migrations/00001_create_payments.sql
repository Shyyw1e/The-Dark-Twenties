-- +goose Up
CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    plan_id UUID,
    provider TEXT NOT NULL,
    provider_invoice_id TEXT,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB',
    status TEXT NOT NULL DEFAULT 'pending',
    payment_url TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT invoices_amount_check CHECK (amount >= 0),
    CONSTRAINT invoices_status_check CHECK (status IN ('pending', 'paid', 'failed', 'cancelled', 'expired'))
);

CREATE TABLE payments (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices (id),
    user_id UUID NOT NULL,
    provider TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB',
    status TEXT NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT payments_provider_payment_unique UNIQUE (provider, provider_payment_id),
    CONSTRAINT payments_amount_check CHECK (amount >= 0),
    CONSTRAINT payments_status_check CHECK (status IN ('succeeded', 'failed', 'cancelled', 'refunded'))
);

CREATE TABLE payment_events (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL,
    event_type TEXT NOT NULL,
    external_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT payment_events_external_unique UNIQUE (provider, event_type, external_id)
);

CREATE INDEX invoices_user_id_idx ON invoices (user_id);
CREATE INDEX invoices_status_idx ON invoices (status);
CREATE INDEX invoices_provider_invoice_id_idx ON invoices (provider, provider_invoice_id);
CREATE INDEX payments_invoice_id_idx ON payments (invoice_id);
CREATE INDEX payments_user_id_idx ON payments (user_id);
CREATE INDEX payment_events_processed_at_idx ON payment_events (processed_at);

-- +goose Down
DROP INDEX IF EXISTS payment_events_processed_at_idx;
DROP INDEX IF EXISTS payments_user_id_idx;
DROP INDEX IF EXISTS payments_invoice_id_idx;
DROP INDEX IF EXISTS invoices_provider_invoice_id_idx;
DROP INDEX IF EXISTS invoices_status_idx;
DROP INDEX IF EXISTS invoices_user_id_idx;
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS invoices;
