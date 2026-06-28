package runtimeobs

import (
	"context"
	"runtime"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/internal/config"
	"github.com/Shyyw1e/The-Dark-Twenties/internal/logger"
)

const serviceName = "runtime-monitor"

type Monitor struct {
	enabled         bool
	interval        time.Duration
	warnThreshold   int
	growthThreshold int
	log             logger.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

func NewMonitor(cfg config.RuntimeConfig, log logger.Logger) *Monitor {
	if log == nil {
		log = logger.FromContext(context.Background())
	}
	if cfg.MonitorInterval <= 0 {
		cfg.MonitorInterval = 30 * time.Second
	}

	return &Monitor{
		enabled:         cfg.MonitorEnabled,
		interval:        cfg.MonitorInterval,
		warnThreshold:   cfg.GoroutineWarnThreshold,
		growthThreshold: cfg.GoroutineGrowthThreshold,
		log:             log,
		done:            make(chan struct{}),
	}
}

func (m *Monitor) Name() string {
	return serviceName
}

func (m *Monitor) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !m.enabled {
		m.log.Info("runtime monitor disabled")
		close(m.done)
		return nil
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})

	go m.run(monitorCtx)

	m.log.Info(
		"runtime monitor started",
		"interval", m.interval.String(),
		"goroutine_warn_threshold", m.warnThreshold,
		"goroutine_growth_threshold", m.growthThreshold,
	)

	return nil
}

func (m *Monitor) Stop(ctx context.Context) error {
	if m.cancel != nil {
		m.cancel()
	}

	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Monitor) run(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	lastGoroutines := runtime.NumGoroutine()
	m.observe(lastGoroutines, 0)

	for {
		select {
		case <-ctx.Done():
			m.log.Info("runtime monitor stopped")
			return
		case <-ticker.C:
			current := runtime.NumGoroutine()
			delta := current - lastGoroutines
			m.observe(current, delta)
			lastGoroutines = current
		}
	}
}

func (m *Monitor) observe(goroutines, delta int) {
	fields := []any{
		"goroutines", goroutines,
		"delta", delta,
	}

	if m.warnThreshold > 0 && goroutines >= m.warnThreshold {
		m.log.Warn("goroutine count exceeded threshold", fields...)
		return
	}
	if m.growthThreshold > 0 && delta >= m.growthThreshold {
		m.log.Warn("goroutine count grew quickly", fields...)
		return
	}

	m.log.Debug("runtime snapshot", fields...)
}
