package ports

import (
	"context"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
)

type Repository interface {
	FindActivePlanByCode(ctx context.Context, code string) (*domain.Plan, error)
	FindActiveTrialPlan(ctx context.Context) (*domain.Plan, error)
	HasUsedTrial(ctx context.Context, userID string) (bool, error)
	FindActiveSubscriptionByUserID(ctx context.Context, userID string, now time.Time) (*domain.Subscription, error)
	CreateSubscription(ctx context.Context, subscription *domain.Subscription) error
	UpdateSubscription(ctx context.Context, subscription *domain.Subscription) error
}
