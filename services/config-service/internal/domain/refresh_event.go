package domain

import "time"

type RefreshEvent struct {
	ID                     string
	SubscriptionTokenID    string
	UserID                 string
	DeviceID               string
	ClientType             string
	UserAgent              string
	SourceIPHash           string
	ReturnedProfileVersion int
	ReturnedServerCount    int
	CreatedAt              time.Time
}
