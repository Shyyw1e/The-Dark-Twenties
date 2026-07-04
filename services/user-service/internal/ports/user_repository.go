package ports

import (
	"context"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
)

type UserRepository interface {
	FindByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	UpdateTelegramProfile(ctx context.Context, user *domain.User) error
}
