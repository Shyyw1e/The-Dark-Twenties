package domain

import "time"

type Plan struct {
	ID                string
	Code              string
	Name              string
	Description       string
	PriceAmount       int64
	Currency          string
	DurationDays      int
	DeviceLimit       int
	TrafficLimitBytes *int64
	IsTrial           bool
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (p Plan) Duration() time.Duration {
	return time.Duration(p.DurationDays) * 24 * time.Hour
}
