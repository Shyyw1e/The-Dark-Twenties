package postgres

import (
	"context"
	"database/sql"
	"errors"
)

type PostgresHealthCheck struct {
	db *sql.DB
}

func (p *PostgresHealthCheck) Name() string {
	return "postgres"
}

func (p *PostgresHealthCheck) Check(ctx context.Context) error {
	if p == nil || p.db == nil {
		return errors.New("postgres db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return p.db.PingContext(ctx)
}

func NewHealthCheck(db *sql.DB) *PostgresHealthCheck {
	return &PostgresHealthCheck{
		db: db,
	}
}
