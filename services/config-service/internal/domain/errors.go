package domain

import "errors"

var (
	ErrSubscriptionTokenNotFound  = errors.New("subscription token not found")
	ErrSubscriptionTokenInactive  = errors.New("subscription token is inactive")
	ErrSubscriptionTokenExpired   = errors.New("subscription token is expired")
	ErrConfigProfileNotFound      = errors.New("config profile not found")
	ErrActiveSubscriptionNotFound = errors.New("active subscription not found")
)
