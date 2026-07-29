package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRepository struct {
	plansByCode       map[string]*domain.Plan
	trialPlan         *domain.Plan
	activeByUserID    map[string]*domain.Subscription
	usedTrialByUserID map[string]bool
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		plansByCode:       make(map[string]*domain.Plan),
		activeByUserID:    make(map[string]*domain.Subscription),
		usedTrialByUserID: make(map[string]bool),
	}
}

func (r *fakeRepository) FindActivePlanByCode(ctx context.Context, code string) (*domain.Plan, error) {
	plan, ok := r.plansByCode[code]
	if !ok {
		return nil, domain.ErrPlanNotFound
	}
	copy := *plan
	return &copy, nil
}

func (r *fakeRepository) FindActiveTrialPlan(ctx context.Context) (*domain.Plan, error) {
	if r.trialPlan == nil {
		return nil, domain.ErrPlanNotFound
	}
	copy := *r.trialPlan
	return &copy, nil
}

func (r *fakeRepository) HasUsedTrial(ctx context.Context, userID string) (bool, error) {
	return r.usedTrialByUserID[userID], nil
}

func (r *fakeRepository) FindActiveSubscriptionByUserID(ctx context.Context, userID string, now time.Time) (*domain.Subscription, error) {
	subscription, ok := r.activeByUserID[userID]
	if !ok {
		return nil, domain.ErrSubscriptionNotFound
	}
	copy := *subscription
	return &copy, nil
}

func (r *fakeRepository) CreateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	copy := *subscription
	r.activeByUserID[subscription.UserID] = &copy
	if subscription.Trial {
		r.usedTrialByUserID[subscription.UserID] = true
	}
	return nil
}

func (r *fakeRepository) UpdateSubscription(ctx context.Context, subscription *domain.Subscription) error {
	copy := *subscription
	r.activeByUserID[subscription.UserID] = &copy
	return nil
}

func TestStartTrial(t *testing.T) {
	repo := newFakeRepository()
	repo.trialPlan = &domain.Plan{ID: "trial-plan", Code: "trial", DurationDays: 7, IsTrial: true}
	server := NewServer(usecase.NewService(repo))

	subscription, err := server.StartTrial(context.Background(), &subscriptionv1.StartTrialRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("StartTrial returned error: %v", err)
	}

	if subscription.GetId() == "" || subscription.GetUserId() != "user-1" || subscription.GetPlanId() != "trial-plan" || !subscription.GetTrial() {
		t.Fatalf("subscription = %+v", subscription)
	}
	if subscription.GetStartsAt() == nil || subscription.GetExpiresAt() == nil {
		t.Fatalf("timestamps are nil: %+v", subscription)
	}
}

func TestActivateOrRenew(t *testing.T) {
	repo := newFakeRepository()
	repo.plansByCode["monthly"] = &domain.Plan{ID: "monthly-plan", Code: "monthly", DurationDays: 30}
	server := NewServer(usecase.NewService(repo))

	subscription, err := server.ActivateOrRenew(context.Background(), &subscriptionv1.ActivateOrRenewRequest{
		UserId:    "user-1",
		PlanCode:  "monthly",
		AutoRenew: true,
	})
	if err != nil {
		t.Fatalf("ActivateOrRenew returned error: %v", err)
	}

	if subscription.GetPlanId() != "monthly-plan" || subscription.GetTrial() || !subscription.GetAutoRenew() {
		t.Fatalf("subscription = %+v", subscription)
	}
}

func TestGetActive(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.activeByUserID["user-1"] = &domain.Subscription{
		ID:        "subscription-1",
		UserID:    "user-1",
		PlanID:    "monthly-plan",
		Status:    domain.SubscriptionStatusActive,
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
		UpdatedAt: now,
	}
	server := NewServer(usecase.NewService(repo))

	subscription, err := server.GetActive(context.Background(), &subscriptionv1.GetActiveRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetActive returned error: %v", err)
	}
	if subscription.GetId() != "subscription-1" || subscription.GetStatus() != string(domain.SubscriptionStatusActive) {
		t.Fatalf("subscription = %+v", subscription)
	}
}

func TestValidatesRequests(t *testing.T) {
	server := NewServer(usecase.NewService(newFakeRepository()))

	_, err := server.StartTrial(context.Background(), nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("StartTrial nil request code = %v", status.Code(err))
	}

	_, err = server.ActivateOrRenew(context.Background(), &subscriptionv1.ActivateOrRenewRequest{UserId: "user-1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ActivateOrRenew invalid request code = %v", status.Code(err))
	}

	_, err = server.GetActive(context.Background(), &subscriptionv1.GetActiveRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GetActive invalid request code = %v", status.Code(err))
	}
}

func TestMapsDomainErrors(t *testing.T) {
	err := toStatusError(domain.ErrTrialAlreadyUsed)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("trial error code = %v", status.Code(err))
	}

	err = toStatusError(domain.ErrSubscriptionNotFound)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("not found code = %v", status.Code(err))
	}

	err = toStatusError(errors.New("db down"))
	if status.Code(err) != codes.Internal {
		t.Fatalf("internal code = %v", status.Code(err))
	}
}
