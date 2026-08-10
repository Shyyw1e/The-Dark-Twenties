package domain

import "time"

type ConfigProfile struct {
	ID                  string
	SubscriptionTokenID string
	ProfileVersion      int
	ClientType          string
	Format              string
	ServerCount         int
	Content             string
	ContentHash         string
	NodeRefs            string
	Metadata            string
	GeneratedAt         time.Time
	ExpiresAt           *time.Time
}
