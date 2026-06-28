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
	userpostgres "github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/adapters/postgres"
	userhttp "github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/transport/http"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
)

const serviceName = "user-service"

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

	userRepo := userpostgres.NewUserRepository(db)
	userUsecase := usecase.NewService(userRepo)
	userHandler := userhttp.NewHandler(userUsecase)

	mux := http.NewServeMux()
	httpserver.RegisterHealthHandlers(mux, rootpostgres.NewHealthCheck(db))
	userhttp.RegisterRoutes(mux, userHandler)

	starter := lifecycle.NewStarter(log, cfg.HTTP.ShutdownTimeout)
	starter.Add(httpserver.NewService(cfg.HTTP, log, mux))
	starter.Add(runtimeobs.NewMonitor(cfg.Runtime, log))
	starter.Add(pprofservice.NewService(cfg.Runtime, log))

	if err := starter.Run(ctx); err != nil {
		log.Error("user-service stopped with error", "error", err)
		os.Exit(1)
	}
}
