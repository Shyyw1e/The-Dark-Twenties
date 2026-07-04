package pprof

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
)

const serviceName = "pprof-server"

type Service struct {
	enabled bool
	addr    string
	srv     *http.Server
	log     logger.Logger
}

func NewService(cfg config.RuntimeConfig, log logger.Logger) *Service {
	if log == nil {
		log = logger.FromContext(context.Background())
	}

	mux := http.NewServeMux()
	registerHandlers(mux)

	return &Service{
		enabled: cfg.PprofEnabled,
		addr:    cfg.PprofAddr,
		log:     log,
		srv: &http.Server{
			Addr:    cfg.PprofAddr,
			Handler: mux,
		},
	}
}

func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Start(ctx context.Context) error {
	if !s.enabled {
		s.log.Info("pprof server disabled")
		return nil
	}
	if s.srv == nil {
		return errors.New("pprof http server is nil")
	}
	if s.addr == "" {
		return errors.New("pprof addr is empty")
	}

	s.log.Info("pprof server listening", "addr", s.addr)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("pprof server failed", "addr", s.addr, "error", err)
		}
	}()

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if !s.enabled || s.srv == nil {
		return nil
	}

	s.log.Info("pprof server shutdown started", "addr", s.addr)

	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}

	s.log.Info("pprof server shutdown completed", "addr", s.addr)
	return nil
}

func registerHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
}
