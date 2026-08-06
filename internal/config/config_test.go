package config

import (
	"testing"
	"time"
)

func TestEnvPrefix(t *testing.T) {
	if got := envPrefix("user-service.api"); got != "USER_SERVICE_API" {
		t.Fatalf("envPrefix = %q, want USER_SERVICE_API", got)
	}
}

func TestLoadUsesServiceSpecificEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("USER_SERVICE_HTTP_ADDR", ":18080")
	t.Setenv("USER_SERVICE_GRPC_ADDR", ":19090")
	t.Setenv("SUBSCRIPTION_SERVICE_GRPC_ADDR", ":19092")
	t.Setenv("USER_SERVICE_POSTGRES_DSN", "postgres://user:pass@localhost:5432/user?sslmode=disable")
	t.Setenv("USER_SERVICE_REDIS_DB", "2")
	t.Setenv("USER_SERVICE_RUNTIME_MONITOR_INTERVAL", "250ms")
	t.Setenv("USER_SERVICE_PPROF_ENABLED", "true")
	t.Setenv("USER_SERVICE_PPROF_ADDR", "127.0.0.1:6061")

	cfg, err := Load("user-service")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.App.ServiceName != "user-service" || cfg.HTTP.Addr != ":18080" || cfg.GRPC.Addr != ":19090" {
		t.Fatalf("config addresses = %+v %+v %+v", cfg.App, cfg.HTTP, cfg.GRPC)
	}
	if cfg.Postgres.DSN == "" {
		t.Fatal("postgres dsn is empty")
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("redis db = %d, want 2", cfg.Redis.DB)
	}
	if cfg.Runtime.MonitorInterval != 250*time.Millisecond {
		t.Fatalf("monitor interval = %v, want 250ms", cfg.Runtime.MonitorInterval)
	}
	if cfg.Telegram.SubscriptionServiceGRPCAddr != ":19092" {
		t.Fatalf("subscription grpc addr = %q, want :19092", cfg.Telegram.SubscriptionServiceGRPCAddr)
	}
	if !cfg.Runtime.PprofEnabled {
		t.Fatal("pprof should be enabled")
	}
}

func TestLoadValidatesLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load("user-service")
	if err == nil {
		t.Fatal("expected error")
	}
}
