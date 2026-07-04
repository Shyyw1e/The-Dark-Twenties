package grpcserver

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
	"google.golang.org/grpc"
)

const defaultServiceName = "grpc-server"

type Service struct {
	name            string
	addr            string
	shutdownTimeout time.Duration
	server          *grpc.Server
	log             logger.Logger
}

func NewService(cfg config.GRPCConfig, log logger.Logger, server *grpc.Server) *Service {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	return &Service{
		name:            defaultServiceName,
		addr:            cfg.Addr,
		shutdownTimeout: cfg.ShutdownTimeout,
		server:          server,
		log:             log,
	}
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Start(ctx context.Context) error {
	if s.server == nil {
		return errors.New("grpc server is nil")
	}
	if s.addr == "" {
		return errors.New("grpc server addr is empty")
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.log.Info("grpc server listening", "addr", s.addr)

	go func() {
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.log.Error("grpc server failed", "addr", s.addr, "error", err)
		}
	}()

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.log.Info("grpc server shutdown completed", "addr", s.addr)
		return nil
	case <-ctx.Done():
		s.server.Stop()
		return ctx.Err()
	}
}
