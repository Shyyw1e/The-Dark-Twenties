package redis

import (
	"context"
	"errors"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	goredis "github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, cfg *config.RedisConfig, log logger.Logger) (*goredis.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("redis config is nil")
	}
	if log == nil {
		log = logger.FromContext(ctx)
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		DB:       cfg.DB,
		Password: cfg.Password,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		log.Error("failed to ping redis", "addr", cfg.Addr, "db", cfg.DB, "error", err)
		return nil, err
	}

	log.Info("connected to redis successfully", "addr", cfg.Addr, "db", cfg.DB)
	return client, nil
}
