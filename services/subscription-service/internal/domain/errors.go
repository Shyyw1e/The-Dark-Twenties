package domain

import "errors"

var (
	ErrPlanNotFound         = errors.New("plan not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrTrialAlreadyUsed     = errors.New("trial already used")
)
