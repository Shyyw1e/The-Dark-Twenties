package postgres

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
)

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *int64:
			*target = value.(int64)
		case *bool:
			*target = value.(bool)
		case *time.Time:
			*target = value.(time.Time)
		case *sql.NullString:
			*target = value.(sql.NullString)
		case *sql.NullInt64:
			*target = value.(sql.NullInt64)
		case *sql.NullTime:
			*target = value.(sql.NullTime)
		default:
			panic("unsupported scan target")
		}
	}
	return nil
}

func TestScanPlan(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	trafficLimit := int64(1_000_000)

	plan, err := scanPlan(fakeRow{values: []any{
		"plan-1",
		"monthly",
		"Monthly",
		sql.NullString{String: "30 days", Valid: true},
		int64(39900),
		"RUB",
		30,
		3,
		sql.NullInt64{Int64: trafficLimit, Valid: true},
		false,
		true,
		now,
		now,
	}})
	if err != nil {
		t.Fatalf("scanPlan returned error: %v", err)
	}

	if plan.ID != "plan-1" || plan.Code != "monthly" || plan.PriceAmount != 39900 || plan.DurationDays != 30 || plan.DeviceLimit != 3 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.TrafficLimitBytes == nil || *plan.TrafficLimitBytes != trafficLimit {
		t.Fatalf("traffic limit = %v", plan.TrafficLimitBytes)
	}
}

func TestScanSubscription(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	cancelledAt := now.Add(time.Hour)

	subscription, err := scanSubscription(fakeRow{values: []any{
		"subscription-1",
		"user-1",
		"plan-1",
		string(domain.SubscriptionStatusCancelled),
		now,
		now.Add(30 * 24 * time.Hour),
		false,
		true,
		sql.NullTime{Time: cancelledAt, Valid: true},
		now,
		cancelledAt,
	}})
	if err != nil {
		t.Fatalf("scanSubscription returned error: %v", err)
	}

	if subscription.ID != "subscription-1" || subscription.UserID != "user-1" || subscription.Status != domain.SubscriptionStatusCancelled || !subscription.Trial {
		t.Fatalf("subscription = %+v", subscription)
	}
	if subscription.CancelledAt == nil || !subscription.CancelledAt.Equal(cancelledAt) {
		t.Fatalf("cancelled_at = %v", subscription.CancelledAt)
	}
}
