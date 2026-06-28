package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
)

const defaultShutdownTimeout = 10 * time.Second

type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Starter struct {
	services        []Service
	log             logger.Logger
	shutdownTimeout time.Duration
}

func NewStarter(log logger.Logger, shutdownTimeout time.Duration) *Starter {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	return &Starter{
		log:             log,
		shutdownTimeout: shutdownTimeout,
	}
}

func (s *Starter) Add(service Service) {
	if service == nil {
		s.log.Warn("nil service ignored")
		return
	}

	for _, existing := range s.services {
		if existing == service {
			s.log.Warn("service already added", "service", service.Name())
			return
		}
	}

	s.services = append(s.services, service)
}

func (s *Starter) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	if err := s.Start(runCtx); err != nil {
		return err
	}

	<-runCtx.Done()

	if err := ctx.Err(); err != nil {
		s.log.Info("shutdown requested by context", "error", err)
	} else {
		s.log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := s.Stop(shutdownCtx); err != nil {
		return err
	}

	s.log.Info("all services have been stopped")
	return nil
}

func (s *Starter) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	started := make([]Service, 0, len(s.services))

	for _, service := range s.services {
		s.log.Info("starting service", "service", service.Name())

		if err := service.Start(ctx); err != nil {
			s.log.Error("failed to start service", "service", service.Name(), "error", err)

			stopCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
			stopErr := s.stopServices(stopCtx, started)
			cancel()

			return errors.Join(err, stopErr)
		}

		started = append(started, service)
		s.log.Info("service started", "service", service.Name())
	}

	return nil
}

func (s *Starter) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return s.stopServices(ctx, s.services)
}

func (s *Starter) stopServices(ctx context.Context, services []Service) error {
	var joinedErr error

	for i := len(services) - 1; i >= 0; i-- {
		service := services[i]

		s.log.Info("stopping service", "service", service.Name())

		if err := service.Stop(ctx); err != nil {
			s.log.Error("failed to stop service", "service", service.Name(), "error", err)
			joinedErr = errors.Join(joinedErr, err)
			continue
		}

		s.log.Info("service stopped", "service", service.Name())
	}

	return joinedErr
}
