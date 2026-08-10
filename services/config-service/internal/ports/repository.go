package ports

import (
	"context"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
)

type Repository interface {
	FindTokenByHash(ctx context.Context, tokenHash string) (*domain.SubscriptionToken, error)
	FindActiveTokenByUserID(ctx context.Context, userID string, clientType string, format string, now time.Time) (*domain.SubscriptionToken, error)
	FindLatestProfileByTokenID(ctx context.Context, tokenID string) (*domain.ConfigProfile, error)
	CreateSubscriptionToken(ctx context.Context, token *domain.SubscriptionToken) error
	CreateConfigProfile(ctx context.Context, profile *domain.ConfigProfile) error
	MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time, userAgent string) error
	CreateRefreshEvent(ctx context.Context, event *domain.RefreshEvent) error
}

type SubscriptionChecker interface {
	HasActiveSubscription(ctx context.Context, userID string, at time.Time) error
}
