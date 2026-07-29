package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/ports"
)

type Service struct {
	repository ports.Repository
	now        func() time.Time
}

type StartTrialInput struct {
	UserID string
}

type ActivateOrRenewInput struct {
	UserID    string
	PlanCode  string
	AutoRenew bool
}

func NewService(repository ports.Repository) *Service {
	return &Service{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) StartTrial(ctx context.Context, input StartTrialInput) (*domain.Subscription, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	used, err := s.repository.HasUsedTrial(ctx, userID)
	if err != nil {
		return nil, err
	}
	if used {
		return nil, domain.ErrTrialAlreadyUsed
	}

	plan, err := s.repository.FindActiveTrialPlan(ctx)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, domain.ErrPlanNotFound
	}

	subscription := newSubscription(userID, plan, s.now(), false)
	if err := s.repository.CreateSubscription(ctx, subscription); err != nil {
		return nil, err
	}

	return subscription, nil
}

func (s *Service) ActivateOrRenew(ctx context.Context, input ActivateOrRenewInput) (*domain.Subscription, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	planCode := strings.TrimSpace(input.PlanCode)
	if planCode == "" {
		return nil, errors.New("plan_code is required")
	}

	plan, err := s.repository.FindActivePlanByCode(ctx, planCode)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, domain.ErrPlanNotFound
	}
	if plan.IsTrial {
		return nil, errors.New("trial plan cannot be activated through paid renewal")
	}

	now := s.now()
	current, err := s.repository.FindActiveSubscriptionByUserID(ctx, userID, now)
	if err != nil {
		if !errors.Is(err, domain.ErrSubscriptionNotFound) {
			return nil, err
		}
		subscription := newSubscription(userID, plan, now, input.AutoRenew)
		if err := s.repository.CreateSubscription(ctx, subscription); err != nil {
			return nil, err
		}
		return subscription, nil
	}

	current.Renew(*plan, now, input.AutoRenew)
	if err := s.repository.UpdateSubscription(ctx, current); err != nil {
		return nil, err
	}

	return current, nil
}

func (s *Service) GetActive(ctx context.Context, userID string) (*domain.Subscription, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	return s.repository.FindActiveSubscriptionByUserID(ctx, userID, s.now())
}

func (s *Service) validate() error {
	if s == nil || s.repository == nil {
		return errors.New("subscription repository is nil")
	}
	return nil
}

func newSubscription(userID string, plan *domain.Plan, now time.Time, autoRenew bool) *domain.Subscription {
	return &domain.Subscription{
		ID:        uuid.NewString(),
		UserID:    userID,
		PlanID:    plan.ID,
		Status:    domain.SubscriptionStatusActive,
		StartsAt:  now,
		ExpiresAt: now.Add(plan.Duration()),
		AutoRenew: autoRenew,
		Trial:     plan.IsTrial,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
