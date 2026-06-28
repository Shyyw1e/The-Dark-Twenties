package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(ctx context.Context, cfg *config.PostgresConfig, log logger.Logger) (*sql.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("postgres config is nil")
	}
	if log == nil {
		log = logger.FromContext(ctx)
	}

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		log.Error("failed to open db", "error", err)
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		log.Error("failed to ping db", "error", err)
		return nil, err
	}

	log.Info("connected to db successfully")
	return db, nil
}
