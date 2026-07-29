package domain

import "time"

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusSuspended SubscriptionStatus = "suspended"
)

type Subscription struct {
	ID          string
	UserID      string
	PlanID      string
	Status      SubscriptionStatus
	StartsAt    time.Time
	ExpiresAt   time.Time
	AutoRenew   bool
	Trial       bool
	CancelledAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s Subscription) IsActiveAt(now time.Time) bool {
	return s.Status == SubscriptionStatusActive && !now.Before(s.StartsAt) && now.Before(s.ExpiresAt)
}

func (s *Subscription) Renew(plan Plan, now time.Time, autoRenew bool) {
	base := now
	if s.ExpiresAt.After(now) {
		base = s.ExpiresAt
	}

	s.PlanID = plan.ID
	s.Status = SubscriptionStatusActive
	s.ExpiresAt = base.Add(plan.Duration())
	s.AutoRenew = autoRenew
	s.Trial = plan.IsTrial
	s.CancelledAt = nil
	s.UpdatedAt = now
}

func (s *Subscription) Cancel(now time.Time) {
	s.Status = SubscriptionStatusCancelled
	s.CancelledAt = &now
	s.UpdatedAt = now
}

func (s *Subscription) Expire(now time.Time) {
	s.Status = SubscriptionStatusExpired
	s.UpdatedAt = now
}
