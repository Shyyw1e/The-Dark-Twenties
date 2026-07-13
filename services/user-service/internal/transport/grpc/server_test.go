package grpc

import (
	"context"
	"testing"

	userv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/user/v1"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeUserRepository struct {
	user *domain.User
}

func (r *fakeUserRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	if r.user == nil {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeUserRepository) Create(ctx context.Context, user *domain.User) error {
	copy := *user
	r.user = &copy
	return nil
}

func (r *fakeUserRepository) UpdateTelegramProfile(ctx context.Context, user *domain.User) error {
	copy := *user
	r.user = &copy
	return nil
}

func TestGetOrCreateTelegramUser(t *testing.T) {
	server := NewServer(usecase.NewService(&fakeUserRepository{}))

	user, err := server.GetOrCreateTelegramUser(context.Background(), &userv1.GetOrCreateTelegramUserRequest{
		TelegramId:   1001,
		Username:     "shyywie",
		FirstName:    "Shyy",
		LastName:     "Wie",
		LanguageCode: "ru",
	})
	if err != nil {
		t.Fatalf("GetOrCreateTelegramUser returned error: %v", err)
	}

	if user.GetId() == "" || user.GetTelegramId() != 1001 || user.GetUsername() != "shyywie" || user.GetStatus() != string(domain.StatusActive) {
		t.Fatalf("user = %+v", user)
	}
	if user.GetCreatedAt() == nil || user.GetUpdatedAt() == nil {
		t.Fatalf("timestamps are nil: %+v", user)
	}
}

func TestGetOrCreateTelegramUserValidatesRequest(t *testing.T) {
	server := NewServer(usecase.NewService(&fakeUserRepository{}))

	_, err := server.GetOrCreateTelegramUser(context.Background(), nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil request code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	_, err = server.GetOrCreateTelegramUser(context.Background(), &userv1.GetOrCreateTelegramUserRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid id code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}
