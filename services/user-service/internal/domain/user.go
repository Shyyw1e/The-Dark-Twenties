package domain

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
	StatusDeleted Status = "deleted"
)

type User struct {
	ID           string
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	Status       Status
	IsAdmin      bool
	BlockedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u User) IsBlocked() bool {
	return u.Status == StatusBlocked
}

func (u User) IsDeleted() bool {
	return u.Status == StatusDeleted
}

func (u *User) Activate() {
	u.Status = StatusActive
	u.BlockedAt = nil
}

func (u *User) Block(now time.Time) {
	u.Status = StatusBlocked
	u.BlockedAt = &now
}

func (u *User) Delete() {
	u.Status = StatusDeleted
}
