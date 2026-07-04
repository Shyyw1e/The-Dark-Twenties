package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type RedisHealthCheck struct {
	client *redis.Client
}

func (r *RedisHealthCheck) Name() string {
	return "redis"
}

func (r *RedisHealthCheck) Check(ctx context.Context) error {
	if r == nil || r.client == nil {
		return errors.New("redis client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return r.client.Ping(ctx).Err()
}

func NewRedisHealthCheck(client *redis.Client) *RedisHealthCheck {
	return &RedisHealthCheck{client: client}
}
