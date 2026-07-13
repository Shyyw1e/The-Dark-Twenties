package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
)

type fakeUserRepository struct {
	byTelegramID map[int64]*domain.User
	createErr    error
	updateErr    error
	findErr      error

	created []*domain.User
	updated []*domain.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byTelegramID: make(map[int64]*domain.User)}
}

func (r *fakeUserRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	user, ok := r.byTelegramID[telegramID]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (r *fakeUserRepository) Create(ctx context.Context, user *domain.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	copy := *user
	r.byTelegramID[user.TelegramID] = &copy
	r.created = append(r.created, &copy)
	return nil
}

func (r *fakeUserRepository) UpdateTelegramProfile(ctx context.Context, user *domain.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copy := *user
	r.byTelegramID[user.TelegramID] = &copy
	r.updated = append(r.updated, &copy)
	return nil
}

func TestGetOrCreateTelegramUserCreatesNewUser(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewService(repo)
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	user, err := service.GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{
		TelegramID:   1001,
		Username:     "  shyywie  ",
		FirstName:    "  Shyy  ",
		LastName:     "  Wie  ",
		LanguageCode: " ru ",
	})
	if err != nil {
		t.Fatalf("GetOrCreateTelegramUser returned error: %v", err)
	}

	if user.ID == "" {
		t.Fatal("created user id is empty")
	}
	if user.TelegramID != 1001 || user.Username != "shyywie" || user.FirstName != "Shyy" || user.LastName != "Wie" || user.LanguageCode != "ru" {
		t.Fatalf("created user = %+v", user)
	}
	if user.Status != domain.StatusActive {
		t.Fatalf("status = %q, want %q", user.Status, domain.StatusActive)
	}
	if !user.CreatedAt.Equal(now) || !user.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %v/%v, want %v", user.CreatedAt, user.UpdatedAt, now)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created count = %d, want 1", len(repo.created))
	}
}

func TestGetOrCreateTelegramUserUpdatesChangedProfile(t *testing.T) {
	repo := newFakeUserRepository()
	initialUpdatedAt := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	repo.byTelegramID[1001] = &domain.User{
		ID:           "user-1",
		TelegramID:   1001,
		Username:     "old",
		FirstName:    "Old",
		LastName:     "Name",
		LanguageCode: "en",
		Status:       domain.StatusActive,
		UpdatedAt:    initialUpdatedAt,
	}
	service := NewService(repo)
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	user, err := service.GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{
		TelegramID:   1001,
		Username:     "new",
		FirstName:    "New",
		LastName:     "Name",
		LanguageCode: "ru",
	})
	if err != nil {
		t.Fatalf("GetOrCreateTelegramUser returned error: %v", err)
	}

	if user.Username != "new" || user.FirstName != "New" || user.LanguageCode != "ru" {
		t.Fatalf("updated user = %+v", user)
	}
	if !user.UpdatedAt.Equal(now) {
		t.Fatalf("updated_at = %v, want %v", user.UpdatedAt, now)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("updated count = %d, want 1", len(repo.updated))
	}
}

func TestGetOrCreateTelegramUserDoesNotUpdateUnchangedProfile(t *testing.T) {
	repo := newFakeUserRepository()
	repo.byTelegramID[1001] = &domain.User{
		ID:           "user-1",
		TelegramID:   1001,
		Username:     "same",
		FirstName:    "Same",
		LastName:     "User",
		LanguageCode: "ru",
		Status:       domain.StatusActive,
	}
	service := NewService(repo)

	_, err := service.GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{
		TelegramID:   1001,
		Username:     "same",
		FirstName:    "Same",
		LastName:     "User",
		LanguageCode: "ru",
	})
	if err != nil {
		t.Fatalf("GetOrCreateTelegramUser returned error: %v", err)
	}
	if len(repo.updated) != 0 {
		t.Fatalf("updated count = %d, want 0", len(repo.updated))
	}
}

func TestGetOrCreateTelegramUserValidationAndErrors(t *testing.T) {
	t.Run("invalid telegram id", func(t *testing.T) {
		_, err := NewService(newFakeUserRepository()).GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("repository is nil", func(t *testing.T) {
		_, err := (*Service)(nil).GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{TelegramID: 1})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("find error is propagated", func(t *testing.T) {
		want := errors.New("db down")
		repo := newFakeUserRepository()
		repo.findErr = want
		_, err := NewService(repo).GetOrCreateTelegramUser(context.Background(), GetOrCreateTelegramUserInput{TelegramID: 1})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}
