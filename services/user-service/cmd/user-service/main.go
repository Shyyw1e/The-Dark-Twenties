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
	grpcserver "github.com/Shyyw1e/The-Dark-Twenties/internal/transport/grpc"
	httpserver "github.com/Shyyw1e/The-Dark-Twenties/internal/transport/http"
	userv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/user/v1"
	userpostgres "github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/adapters/postgres"
	usergrpc "github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/transport/grpc"
	userhttp "github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/transport/http"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
	"google.golang.org/grpc"
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
	userGRPCServer := usergrpc.NewServer(userUsecase)

	mux := http.NewServeMux()
	httpserver.RegisterHealthHandlers(mux, rootpostgres.NewHealthCheck(db))
	userhttp.RegisterRoutes(mux, userHandler)

	grpcSrv := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMessageSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMessageSize),
	)
	userv1.RegisterUserServiceServer(grpcSrv, userGRPCServer)

	starter := lifecycle.NewStarter(log, cfg.HTTP.ShutdownTimeout)
	starter.Add(httpserver.NewService(cfg.HTTP, log, mux))
	starter.Add(grpcserver.NewService(cfg.GRPC, log, grpcSrv))
	starter.Add(runtimeobs.NewMonitor(cfg.Runtime, log))
	starter.Add(pprofservice.NewService(cfg.Runtime, log))

	if err := starter.Run(ctx); err != nil {
		log.Error("user-service stopped with error", "error", err)
		os.Exit(1)
	}
}
