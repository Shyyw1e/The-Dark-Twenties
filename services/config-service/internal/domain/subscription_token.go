package domain

import "time"

type TokenStatus string

const (
	TokenStatusActive  TokenStatus = "active"
	TokenStatusRevoked TokenStatus = "revoked"
	TokenStatusExpired TokenStatus = "expired"
)

type SubscriptionToken struct {
	ID             string
	UserID         string
	DeviceID       string
	SubscriptionID string
	TokenHash      string
	Status         TokenStatus
	ClientType     string
	Format         string
	ExpiresAt      *time.Time
	LastUsedAt     *time.Time
	LastUserAgent  string
	RefreshCount   int64
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (t SubscriptionToken) IsActiveAt(now time.Time) bool {
	if t.Status != TokenStatusActive {
		return false
	}
	return t.ExpiresAt == nil || now.Before(*t.ExpiresAt)
}
