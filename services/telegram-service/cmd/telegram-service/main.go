package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/lifecycle"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	pprofservice "github.com/Shyyw1e/The-Dark-Twenties/internal/observability/pprof"
	runtimeobs "github.com/Shyyw1e/The-Dark-Twenties/internal/observability/runtime"
	subscriptionclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/subscription"
	userclient "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/clients/user"
	bottransport "github.com/Shyyw1e/The-Dark-Twenties/services/telegram-service/internal/transport/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const serviceName = "telegram-service"

func main() {
	ctx := context.Background()

	cfg := config.MustLoad(serviceName)
	log := logger.New(cfg.App.LogLevel, cfg.App.ServiceName)

	if cfg.Telegram.BotToken == "" {
		log.Error("telegram bot token is required")
		os.Exit(1)
	}

	botAPI, err := tgbotapi.NewBotAPIWithClient(
		cfg.Telegram.BotToken,
		tgbotapi.APIEndpoint,
		&http.Client{Timeout: cfg.Telegram.RequestTimeout},
	)
	if err != nil {
		log.Error("failed to create telegram bot api", "error", redactToken(err.Error(), cfg.Telegram.BotToken))
		os.Exit(1)
	}

	users, err := userclient.Dial(ctx, cfg.Telegram.UserServiceGRPCAddr)
	if err != nil {
		log.Error("failed to connect user-service grpc", "addr", cfg.Telegram.UserServiceGRPCAddr, "error", err)
		os.Exit(1)
	}
	defer users.Close()

	subscriptions, err := subscriptionclient.Dial(ctx, cfg.Telegram.SubscriptionServiceGRPCAddr)
	if err != nil {
		log.Error("failed to connect subscription-service grpc", "addr", cfg.Telegram.SubscriptionServiceGRPCAddr, "error", err)
		os.Exit(1)
	}
	defer subscriptions.Close()

	starter := lifecycle.NewStarter(log, cfg.HTTP.ShutdownTimeout)
	starter.Add(bottransport.NewService(botAPI, users, subscriptions, cfg.Telegram, log))
	starter.Add(runtimeobs.NewMonitor(cfg.Runtime, log))
	starter.Add(pprofservice.NewService(cfg.Runtime, log))

	if err := starter.Run(ctx); err != nil {
		log.Error("telegram-service stopped with error", "error", err)
		os.Exit(1)
	}
}

func redactToken(value, token string) string {
	if token == "" {
		return value
	}
	return strings.ReplaceAll(value, token, "[REDACTED]")
}
