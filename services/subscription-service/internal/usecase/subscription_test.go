package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
)

type fakeRepository struct {
	plansByCode       map[string]*domain.Plan
	trialPlan         *domain.Plan
	activeByUserID    map[string]*domain.Subscription
	usedTrialByUserID map[string]bool
	findPlanErr       error
	findTrialPlanErr  error
	hasUsedTrialErr   error
	findActiveErr     error
	createErr         error
	updateErr         error
	created           []*domain.Subscription
	updated           []*domain.Subscription
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		plansByCode:       make(map[string]*domain.Plan),
		activeByUserID:    make(map[string]*domain.Subscription),
		usedTrialByUserID: make(map[string]bool),
	}
}

func (r *fakeRepository) FindActivePlanByCode(ctx context.Context, code string) (*domain.Plan, error) {
	if r.findPlanErr != nil {
		return nil, r.findPlanErr
	}
	plan, ok := r.plansByCode[code]
	if !ok {
		return nil, domain.ErrPlanNotFound
	}
	copy := *plan
	return &copy, nil
}

func (r *fakeRepository) FindActiveTrialPlan(ctx context.Context) (*domain.Plan, error) {
	if r.findTrialPlanErr != nil {
		return nil, r.findTrialPlanErr
	}
	if r.trialPlan == nil {
		return nil, domain.ErrPlanNotFound
	}
	copy := *r.trialPlan
	return &copy, nil
}

func (r *fakeRepository) HasUsedTrial(ctx context.Context, userID string) (bool, error) {
	if r.hasUsedTrialErr != nil {
		return false, r.hasUsedTrialErr
	}
	return r.usedTrialByUserID[userID], nil
}

func (r *fakeRepository) FindActiveSubscriptionByUserID(ctx context.Context, userID string, now time.Time) (*domain.Subscription, error) {
	if r.findActiveErr != nil {
		return nil, r.findActiveErr
	}
	subscription, ok := r.activeByUserID[userID]
	if !ok {
		return nil, domain.ErrSubscriptionNotFound
	}
	copy := *subscription
	return &copy, nil
}

func (r *fakeRepository) CreateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	if r.createErr != nil {
		return r.createErr
	}
	copy := *subscription
	r.created = append(r.created, &copy)
	r.activeByUserID[subscription.UserID] = &copy
	if subscription.Trial {
		r.usedTrialByUserID[subscription.UserID] = true
	}
	return nil
}

func (r *fakeRepository) UpdateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copy := *subscription
	r.updated = append(r.updated, &copy)
	r.activeByUserID[subscription.UserID] = &copy
	return nil
}

func TestStartTrialCreatesTrialSubscription(t *testing.T) {
	repo := newFakeRepository()
	repo.trialPlan = &domain.Plan{ID: "trial-plan", Code: "trial", DurationDays: 7, IsTrial: true}
	service := NewService(repo)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	subscription, err := service.StartTrial(context.Background(), StartTrialInput{UserID: " user-1 "})
	if err != nil {
		t.Fatalf("StartTrial returned error: %v", err)
	}

	if subscription.ID == "" || subscription.UserID != "user-1" || subscription.PlanID != "trial-plan" || !subscription.Trial {
		t.Fatalf("subscription = %+v", subscription)
	}
	if !subscription.StartsAt.Equal(now) || !subscription.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("period = %v/%v", subscription.StartsAt, subscription.ExpiresAt)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created count = %d, want 1", len(repo.created))
	}
}

func TestStartTrialRejectsRepeatedTrial(t *testing.T) {
	repo := newFakeRepository()
	repo.usedTrialByUserID["user-1"] = true

	_, err := NewService(repo).StartTrial(context.Background(), StartTrialInput{UserID: "user-1"})
	if !errors.Is(err, domain.ErrTrialAlreadyUsed) {
		t.Fatalf("error = %v, want %v", err, domain.ErrTrialAlreadyUsed)
	}
}

func TestActivateOrRenewCreatesPaidSubscription(t *testing.T) {
	repo := newFakeRepository()
	repo.plansByCode["monthly"] = &domain.Plan{ID: "monthly-plan", Code: "monthly", DurationDays: 30}
	service := NewService(repo)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	subscription, err := service.ActivateOrRenew(context.Background(), ActivateOrRenewInput{
		UserID:    "user-1",
		PlanCode:  "monthly",
		AutoRenew: true,
	})
	if err != nil {
		t.Fatalf("ActivateOrRenew returned error: %v", err)
	}

	if subscription.PlanID != "monthly-plan" || subscription.Trial || !subscription.AutoRenew {
		t.Fatalf("subscription = %+v", subscription)
	}
	if !subscription.ExpiresAt.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %v", subscription.ExpiresAt)
	}
	if len(repo.created) != 1 || len(repo.updated) != 0 {
		t.Fatalf("created/updated = %d/%d", len(repo.created), len(repo.updated))
	}
}

func TestActivateOrRenewExtendsActiveSubscription(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	currentExpiresAt := now.Add(5 * 24 * time.Hour)
	repo.plansByCode["monthly"] = &domain.Plan{ID: "monthly-plan", Code: "monthly", DurationDays: 30}
	repo.activeByUserID["user-1"] = &domain.Subscription{
		ID:        "subscription-1",
		UserID:    "user-1",
		PlanID:    "trial-plan",
		Status:    domain.SubscriptionStatusActive,
		StartsAt:  now.Add(-24 * time.Hour),
		ExpiresAt: currentExpiresAt,
		Trial:     true,
	}
	service := NewService(repo)
	service.now = func() time.Time { return now }

	subscription, err := service.ActivateOrRenew(context.Background(), ActivateOrRenewInput{
		UserID:   "user-1",
		PlanCode: "monthly",
	})
	if err != nil {
		t.Fatalf("ActivateOrRenew returned error: %v", err)
	}

	if subscription.ID != "subscription-1" || subscription.PlanID != "monthly-plan" || subscription.Trial {
		t.Fatalf("subscription = %+v", subscription)
	}
	if !subscription.ExpiresAt.Equal(currentExpiresAt.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %v", subscription.ExpiresAt)
	}
	if len(repo.created) != 0 || len(repo.updated) != 1 {
		t.Fatalf("created/updated = %d/%d", len(repo.created), len(repo.updated))
	}
}

func TestActivateOrRenewRejectsTrialPlan(t *testing.T) {
	repo := newFakeRepository()
	repo.plansByCode["trial"] = &domain.Plan{ID: "trial-plan", Code: "trial", DurationDays: 7, IsTrial: true}

	_, err := NewService(repo).ActivateOrRenew(context.Background(), ActivateOrRenewInput{
		UserID:   "user-1",
		PlanCode: "trial",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
