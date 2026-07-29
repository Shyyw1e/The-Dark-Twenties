package domain

import (
	"testing"
	"time"
)

func TestSubscriptionIsActiveAt(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	subscription := Subscription{
		Status:    SubscriptionStatusActive,
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
	}

	if !subscription.IsActiveAt(now) {
		t.Fatal("subscription must be active")
	}

	subscription.Status = SubscriptionStatusCancelled
	if subscription.IsActiveAt(now) {
		t.Fatal("cancelled subscription must not be active")
	}
}

func TestSubscriptionRenewExtendsFromCurrentExpiration(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	currentExpiresAt := now.Add(10 * 24 * time.Hour)
	subscription := Subscription{
		PlanID:    "trial-plan",
		Status:    SubscriptionStatusActive,
		StartsAt:  now.Add(-24 * time.Hour),
		ExpiresAt: currentExpiresAt,
		Trial:     true,
	}
	plan := Plan{ID: "monthly-plan", DurationDays: 30}

	subscription.Renew(plan, now, true)

	if subscription.PlanID != plan.ID || subscription.Trial {
		t.Fatalf("subscription plan/trial = %q/%t", subscription.PlanID, subscription.Trial)
	}
	if !subscription.ExpiresAt.Equal(currentExpiresAt.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %v", subscription.ExpiresAt)
	}
	if !subscription.AutoRenew || subscription.Status != SubscriptionStatusActive {
		t.Fatalf("auto/status = %t/%q", subscription.AutoRenew, subscription.Status)
	}
}
