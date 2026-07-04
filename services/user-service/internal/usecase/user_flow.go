package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/ports"
)

type Service struct {
	users ports.UserRepository
	now   func() time.Time
}

type GetOrCreateTelegramUserInput struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
}

func NewService(users ports.UserRepository) *Service {
	return &Service{
		users: users,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) GetOrCreateTelegramUser(ctx context.Context, input GetOrCreateTelegramUserInput) (*domain.User, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.users == nil {
		return nil, errors.New("user repository is nil")
	}
	if input.TelegramID <= 0 {
		return nil, errors.New("telegram_id must be positive")
	}

	input = normalizeTelegramInput(input)

	user, err := s.users.FindByTelegramID(ctx, input.TelegramID)
	if err == nil {
		if shouldUpdateTelegramProfile(user, input) {
			user.Username = input.Username
			user.FirstName = input.FirstName
			user.LastName = input.LastName
			user.LanguageCode = input.LanguageCode
			user.UpdatedAt = s.now()

			if err := s.users.UpdateTelegramProfile(ctx, user); err != nil {
				return nil, err
			}
		}

		return user, nil
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	now := s.now()
	user = &domain.User{
		ID:           uuid.NewString(),
		TelegramID:   input.TelegramID,
		Username:     input.Username,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		LanguageCode: input.LanguageCode,
		Status:       domain.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func normalizeTelegramInput(input GetOrCreateTelegramUserInput) GetOrCreateTelegramUserInput {
	input.Username = strings.TrimSpace(input.Username)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.LanguageCode = strings.TrimSpace(input.LanguageCode)
	return input
}

func shouldUpdateTelegramProfile(user *domain.User, input GetOrCreateTelegramUserInput) bool {
	if user == nil {
		return false
	}

	return user.Username != input.Username ||
		user.FirstName != input.FirstName ||
		user.LastName != input.LastName ||
		user.LanguageCode != input.LanguageCode
}
