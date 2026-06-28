package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	ServiceName string
	Env         string
	Version     string
	LogLevel    string
}

type HTTPConfig struct {
	Addr              string
	RequestTimeout    time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type PostgresConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	DB       int
	Password string
}

type RabbitMQConfig struct {
	URL              string
	EventsExchange   string
	CommandsExchange string
	RetryExchange    string
	DLXExchange      string
}

type TelegramConfig struct {
	BotToken       string
	MiniAppSecret  string
	RequestTimeout time.Duration
}

type RuntimeConfig struct {
	MonitorEnabled           bool
	MonitorInterval          time.Duration
	GoroutineWarnThreshold   int
	GoroutineGrowthThreshold int
	PprofEnabled             bool
	PprofAddr                string
}

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	Telegram TelegramConfig
	Runtime  RuntimeConfig
}

func MustLoad(serviceName string) *Config {
	cfg, err := Load(serviceName)
	if err != nil {
		panic(fmt.Errorf("config load failed: %w", err))
	}
	return cfg
}

func Load(serviceName string) (*Config, error) {
	_ = godotenv.Load()

	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = getEnv("SERVICE_NAME", "")
	}

	cfg := &Config{
		App:      loadAppConfig(serviceName),
		HTTP:     loadHTTPConfig(serviceName),
		Postgres: loadPostgresConfig(serviceName),
		Redis:    loadRedisConfig(serviceName),
		RabbitMQ: loadRabbitMQConfig(serviceName),
		Telegram: loadTelegramConfig(serviceName),
		Runtime:  loadRuntimeConfig(serviceName),
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadAppConfig(serviceName string) AppConfig {
	return AppConfig{
		ServiceName: serviceName,
		Env:         getEnv("APP_ENV", "local"),
		Version:     getEnv("APP_VERSION", "dev"),
		LogLevel:    strings.ToLower(getEnv("LOG_LEVEL", "info")),
	}
}

func loadHTTPConfig(serviceName string) HTTPConfig {
	return HTTPConfig{
		Addr:              getServiceEnv(serviceName, "HTTP_ADDR", ":8080"),
		RequestTimeout:    getDurationEnv(serviceName, "REQUEST_TIMEOUT", 10*time.Second),
		ReadTimeout:       getDurationEnv(serviceName, "HTTP_READ_TIMEOUT", 5*time.Second),
		ReadHeaderTimeout: getDurationEnv(serviceName, "HTTP_READ_HEADER_TIMEOUT", 2*time.Second),
		WriteTimeout:      getDurationEnv(serviceName, "HTTP_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:       getDurationEnv(serviceName, "HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:   getDurationEnv(serviceName, "HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func loadPostgresConfig(serviceName string) PostgresConfig {
	dsn := getServiceEnv(serviceName, "POSTGRES_DSN", "")
	if dsn == "" {
		host := getServiceEnv(serviceName, "DB_HOST", "")
		port := getServiceEnv(serviceName, "DB_PORT", "5432")
		user := getServiceEnv(serviceName, "DB_USER", "")
		name := getServiceEnv(serviceName, "DB_NAME", "")
		pass := getServiceEnv(serviceName, "DB_PASSWORD", "")
		sslMode := getServiceEnv(serviceName, "DB_SSLMODE", "disable")

		if host != "" && user != "" && name != "" {
			dsn = fmt.Sprintf(
				"postgres://%s:%s@%s:%s/%s?sslmode=%s",
				user,
				pass,
				host,
				port,
				name,
				sslMode,
			)
		}
	}

	return PostgresConfig{
		DSN:             dsn,
		MaxOpenConns:    getIntEnv(serviceName, "POSTGRES_MAX_OPEN_CONNS", 20),
		MaxIdleConns:    getIntEnv(serviceName, "POSTGRES_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getDurationEnv(serviceName, "POSTGRES_CONN_MAX_LIFETIME", 30*time.Minute),
	}
}

func loadRedisConfig(serviceName string) RedisConfig {
	return RedisConfig{
		Addr:     getServiceEnv(serviceName, "REDIS_ADDR", "localhost:6379"),
		DB:       getIntEnv(serviceName, "REDIS_DB", 0),
		Password: getServiceEnv(serviceName, "REDIS_PASSWORD", ""),
	}
}

func loadRabbitMQConfig(serviceName string) RabbitMQConfig {
	return RabbitMQConfig{
		URL:              getServiceEnv(serviceName, "RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		EventsExchange:   getServiceEnv(serviceName, "RABBITMQ_EVENTS_EXCHANGE", "vpn.events.topic"),
		CommandsExchange: getServiceEnv(serviceName, "RABBITMQ_COMMANDS_EXCHANGE", "vpn.commands.direct"),
		RetryExchange:    getServiceEnv(serviceName, "RABBITMQ_RETRY_EXCHANGE", "vpn.retry"),
		DLXExchange:      getServiceEnv(serviceName, "RABBITMQ_DLX_EXCHANGE", "vpn.dlx"),
	}
}

func loadTelegramConfig(serviceName string) TelegramConfig {
	return TelegramConfig{
		BotToken:       getServiceEnv(serviceName, "TG_BOT_TOKEN", ""),
		MiniAppSecret:  getServiceEnv(serviceName, "TG_MINI_APP_SECRET", ""),
		RequestTimeout: getDurationEnv(serviceName, "TELEGRAM_REQUEST_TIMEOUT", 10*time.Second),
	}
}

func loadRuntimeConfig(serviceName string) RuntimeConfig {
	return RuntimeConfig{
		MonitorEnabled:           getBoolEnv(serviceName, "RUNTIME_MONITOR_ENABLED", true),
		MonitorInterval:          getDurationEnv(serviceName, "RUNTIME_MONITOR_INTERVAL", 30*time.Second),
		GoroutineWarnThreshold:   getIntEnv(serviceName, "RUNTIME_GOROUTINE_WARN_THRESHOLD", 1000),
		GoroutineGrowthThreshold: getIntEnv(serviceName, "RUNTIME_GOROUTINE_GROWTH_THRESHOLD", 100),
		PprofEnabled:             getBoolEnv(serviceName, "PPROF_ENABLED", false),
		PprofAddr:                getServiceEnv(serviceName, "PPROF_ADDR", "127.0.0.1:6060"),
	}
}

func validateConfig(cfg *Config) error {
	if cfg.App.ServiceName == "" {
		return errors.New("SERVICE_NAME is required")
	}

	switch cfg.App.LogLevel {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("unsupported LOG_LEVEL %q", cfg.App.LogLevel)
	}

	if cfg.HTTP.Addr == "" {
		return errors.New("HTTP_ADDR is required")
	}
	if cfg.HTTP.ShutdownTimeout <= 0 {
		return errors.New("HTTP_SHUTDOWN_TIMEOUT must be positive")
	}
	if cfg.Postgres.MaxOpenConns < 0 {
		return errors.New("POSTGRES_MAX_OPEN_CONNS must be non-negative")
	}
	if cfg.Postgres.MaxIdleConns < 0 {
		return errors.New("POSTGRES_MAX_IDLE_CONNS must be non-negative")
	}
	if cfg.Postgres.MaxOpenConns > 0 && cfg.Postgres.MaxIdleConns > cfg.Postgres.MaxOpenConns {
		return errors.New("POSTGRES_MAX_IDLE_CONNS must be <= POSTGRES_MAX_OPEN_CONNS")
	}
	if cfg.Redis.DB < 0 {
		return errors.New("REDIS_DB must be non-negative")
	}
	if cfg.RabbitMQ.URL == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	if cfg.Runtime.MonitorInterval <= 0 {
		return errors.New("RUNTIME_MONITOR_INTERVAL must be positive")
	}
	if cfg.Runtime.GoroutineWarnThreshold < 0 {
		return errors.New("RUNTIME_GOROUTINE_WARN_THRESHOLD must be non-negative")
	}
	if cfg.Runtime.GoroutineGrowthThreshold < 0 {
		return errors.New("RUNTIME_GOROUTINE_GROWTH_THRESHOLD must be non-negative")
	}
	if cfg.Runtime.PprofEnabled && cfg.Runtime.PprofAddr == "" {
		return errors.New("PPROF_ADDR is required when PPROF_ENABLED=true")
	}

	return nil
}

func getServiceEnv(serviceName, key, def string) string {
	if serviceName != "" {
		if v := getEnv(envPrefix(serviceName)+"_"+key, ""); v != "" {
			return v
		}
	}
	return getEnv(key, def)
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func getIntEnv(serviceName, key string, def int) int {
	raw := getServiceEnv(serviceName, key, "")
	if raw == "" {
		return def
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return value
}

func getDurationEnv(serviceName, key string, def time.Duration) time.Duration {
	raw := getServiceEnv(serviceName, key, "")
	if raw == "" {
		return def
	}

	duration, err := time.ParseDuration(raw)
	if err == nil {
		return duration
	}

	milliseconds, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func getBoolEnv(serviceName, key string, def bool) bool {
	raw := getServiceEnv(serviceName, key, "")
	if raw == "" {
		return def
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return value
}

func envPrefix(serviceName string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(serviceName)))
}
