-- +goose Up
INSERT INTO plans (
    id,
    code,
    name,
    description,
    price_amount,
    currency,
    duration_days,
    device_limit,
    traffic_limit_bytes,
    is_trial,
    is_active
) VALUES
    (
        '11111111-1111-4111-8111-111111111111',
        'trial_7d',
        'Trial 7 days',
        'Free trial access for one user. Can be issued only once.',
        0,
        'RUB',
        7,
        1,
        NULL,
        true,
        true
    ),
    (
        '22222222-2222-4222-8222-222222222222',
        'monthly',
        'Monthly',
        'Monthly access for regular users.',
        39900,
        'RUB',
        30,
        3,
        NULL,
        false,
        true
    ),
    (
        '33333333-3333-4333-8333-333333333333',
        'yearly',
        'Yearly',
        'Yearly access for regular users.',
        399000,
        'RUB',
        365,
        3,
        NULL,
        false,
        true
    )
ON CONFLICT (code) DO UPDATE
SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price_amount = EXCLUDED.price_amount,
    currency = EXCLUDED.currency,
    duration_days = EXCLUDED.duration_days,
    device_limit = EXCLUDED.device_limit,
    traffic_limit_bytes = EXCLUDED.traffic_limit_bytes,
    is_trial = EXCLUDED.is_trial,
    is_active = EXCLUDED.is_active,
    updated_at = now();

-- +goose Down
DELETE FROM plans
WHERE code IN ('trial_7d', 'monthly', 'yearly');
