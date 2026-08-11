package main

import (
	"context"
	"net/http"
	"os"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/lifecycle"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	pprofservice "github.com/Shyyw1e/The-Dark-Twenties/internal/observability/pprof"
	runtimeobs "github.com/Shyyw1e/The-Dark-Twenties/internal/observability/runtime"
	rootpostgres "github.com/Shyyw1e/The-Dark-Twenties/internal/storage/postgres"
	httpserver "github.com/Shyyw1e/The-Dark-Twenties/internal/transport/http"
	configpostgres "github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/adapters/postgres"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/adapters/staticnodes"
	subscriptionclient "github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/clients/subscription"
	confighttp "github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/transport/http"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/usecase"
)

const serviceName = "config-service"

func main() {
	ctx := context.Background()

	cfg := config.MustLoad(serviceName)
	log := logger.New(cfg.App.LogLevel, cfg.App.ServiceName)

	db, err := rootpostgres.Connect(ctx, &cfg.Postgres, log)
	if err != nil {
		log.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	subscriptionChecker, err := subscriptionclient.Dial(ctx, cfg.Telegram.SubscriptionServiceGRPCAddr)
	if err != nil {
		log.Error("failed to connect subscription-service", "error", err)
		os.Exit(1)
	}
	defer subscriptionChecker.Close()

	nodeProvider, err := staticnodes.NewProviderFromJSON(cfg.ConfigService.StaticNodesJSON, staticnodes.DefaultNodes())
	if err != nil {
		log.Error("failed to create node provider", "error", err)
		os.Exit(1)
	}

	configRepo := configpostgres.NewRepository(db)
	configUsecase := usecase.NewService(
		configRepo,
		subscriptionChecker,
		usecase.WithNodeProvider(nodeProvider),
		usecase.WithMaxProfileNodes(cfg.ConfigService.MaxProfileNodes),
	)
	configHandler := confighttp.NewHandler(configUsecase)

	mux := http.NewServeMux()
	httpserver.RegisterHealthHandlers(mux, rootpostgres.NewHealthCheck(db))
	confighttp.RegisterRoutes(mux, configHandler)

	starter := lifecycle.NewStarter(log, cfg.HTTP.ShutdownTimeout)
	starter.Add(httpserver.NewService(cfg.HTTP, log, mux))
	starter.Add(runtimeobs.NewMonitor(cfg.Runtime, log))
	starter.Add(pprofservice.NewService(cfg.Runtime, log))

	if err := starter.Run(ctx); err != nil {
		log.Error("config-service stopped with error", "error", err)
		os.Exit(1)
	}
}
