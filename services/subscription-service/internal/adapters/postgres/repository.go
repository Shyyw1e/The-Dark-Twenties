package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindActivePlanByCode(ctx context.Context, code string) (*domain.Plan, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
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
	is_active,
	created_at,
	updated_at
FROM plans
WHERE code = $1 AND is_active = true
LIMIT 1`

	plan, err := scanPlan(r.db.QueryRowContext(ctx, query, strings.TrimSpace(code)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active plan by code: %w", err)
	}

	return plan, nil
}

func (r *Repository) FindActiveTrialPlan(ctx context.Context) (*domain.Plan, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
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
	is_active,
	created_at,
	updated_at
FROM plans
WHERE is_trial = true AND is_active = true
ORDER BY duration_days DESC, created_at ASC
LIMIT 1`

	plan, err := scanPlan(r.db.QueryRowContext(ctx, query))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active trial plan: %w", err)
	}

	return plan, nil
}

func (r *Repository) HasUsedTrial(ctx context.Context, userID string) (bool, error) {
	if err := r.validate(); err != nil {
		return false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT EXISTS (
	SELECT 1
	FROM subscriptions
	WHERE user_id = $1 AND trial = true
)`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(userID)).Scan(&exists); err != nil {
		return false, fmt.Errorf("check trial usage: %w", err)
	}

	return exists, nil
}

func (r *Repository) FindActiveSubscriptionByUserID(ctx context.Context, userID string, now time.Time) (*domain.Subscription, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const query = `
SELECT
	id,
	user_id,
	plan_id,
	status,
	starts_at,
	expires_at,
	auto_renew,
	trial,
	cancelled_at,
	created_at,
	updated_at
FROM subscriptions
WHERE user_id = $1
	AND status = 'active'
	AND starts_at <= $2
	AND expires_at > $2
ORDER BY expires_at DESC
LIMIT 1`

	subscription, err := scanSubscription(r.db.QueryRowContext(ctx, query, strings.TrimSpace(userID), now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active subscription by user_id: %w", err)
	}

	return subscription, nil
}

func (r *Repository) CreateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if subscription == nil {
		return errors.New("subscription is nil")
	}

	const query = `
INSERT INTO subscriptions (
	id,
	user_id,
	plan_id,
	status,
	starts_at,
	expires_at,
	auto_renew,
	trial,
	cancelled_at,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		subscription.ID,
		subscription.UserID,
		subscription.PlanID,
		string(subscription.Status),
		subscription.StartsAt,
		subscription.ExpiresAt,
		subscription.AutoRenew,
		subscription.Trial,
		subscription.CancelledAt,
		subscription.CreatedAt,
		subscription.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	return nil
}

func (r *Repository) UpdateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if subscription == nil {
		return errors.New("subscription is nil")
	}

	const query = `
UPDATE subscriptions
SET
	plan_id = $2,
	status = $3,
	starts_at = $4,
	expires_at = $5,
	auto_renew = $6,
	trial = $7,
	cancelled_at = $8,
	updated_at = $9
WHERE id = $1`

	result, err := r.db.ExecContext(
		ctx,
		query,
		subscription.ID,
		subscription.PlanID,
		string(subscription.Status),
		subscription.StartsAt,
		subscription.ExpiresAt,
		subscription.AutoRenew,
		subscription.Trial,
		subscription.CancelledAt,
		subscription.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update subscription rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrSubscriptionNotFound
	}

	return nil
}

func (r *Repository) validate() error {
	if r == nil || r.db == nil {
		return errors.New("subscription repository db is nil")
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPlan(row rowScanner) (*domain.Plan, error) {
	var plan domain.Plan
	var description sql.NullString
	var trafficLimitBytes sql.NullInt64

	if err := row.Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&description,
		&plan.PriceAmount,
		&plan.Currency,
		&plan.DurationDays,
		&plan.DeviceLimit,
		&trafficLimitBytes,
		&plan.IsTrial,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	); err != nil {
		return nil, err
	}

	plan.Description = description.String
	if trafficLimitBytes.Valid {
		value := trafficLimitBytes.Int64
		plan.TrafficLimitBytes = &value
	}

	return &plan, nil
}

func scanSubscription(row rowScanner) (*domain.Subscription, error) {
	var subscription domain.Subscription
	var status string
	var cancelledAt sql.NullTime

	if err := row.Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.PlanID,
		&status,
		&subscription.StartsAt,
		&subscription.ExpiresAt,
		&subscription.AutoRenew,
		&subscription.Trial,
		&cancelledAt,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	); err != nil {
		return nil, err
	}

	subscription.Status = domain.SubscriptionStatus(status)
	if cancelledAt.Valid {
		value := cancelledAt.Time
		subscription.CancelledAt = &value
	}

	return &subscription, nil
}
