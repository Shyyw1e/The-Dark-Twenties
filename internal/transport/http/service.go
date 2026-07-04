package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
)

const defaultServiceName = "http-server"

type Service struct {
	name string
	srv  *http.Server
	log  logger.Logger
}

func NewService(cfg config.HTTPConfig, log logger.Logger, handler http.Handler) *Service {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if handler == nil {
		handler = http.NewServeMux()
	}

	return &Service{
		name: defaultServiceName,
		log:  log,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

func NewHealthService(cfg config.HTTPConfig, log logger.Logger, checks ...HealthCheck) *Service {
	mux := http.NewServeMux()
	RegisterHealthHandlers(mux, checks...)

	return NewService(cfg, log, mux)
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Start(ctx context.Context) error {
	if s.srv == nil {
		return errors.New("http server is nil")
	}
	if s.srv.Addr == "" {
		return errors.New("http server address is empty")
	}

	s.log.Info("http server listening", "addr", s.srv.Addr)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("http server failed", "addr", s.srv.Addr, "error", err)
		}
	}()

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	s.log.Info("http server shutdown started", "addr", s.srv.Addr)

	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}

	s.log.Info("http server shutdown completed", "addr", s.srv.Addr)
	return nil
}
