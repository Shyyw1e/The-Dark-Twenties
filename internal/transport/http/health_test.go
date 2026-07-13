package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeHealthCheck struct {
	name string
	err  error
}

func (c fakeHealthCheck) Name() string {
	return c.name
}

func (c fakeHealthCheck) Check(ctx context.Context) error {
	return c.err
}

func TestHealthHandlers(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthHandlers(mux, fakeHealthCheck{name: "postgres"})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("live status/body = %d/%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"postgres":{"status":"ok"}`) {
		t.Fatalf("ready status/body = %d/%s", rec.Code, rec.Body.String())
	}
}

func TestReadyHandlerReportsDegraded(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthHandlers(mux, fakeHealthCheck{name: "postgres", err: errors.New("down")})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), `"status":"degraded"`) || !strings.Contains(rec.Body.String(), `"error":"down"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
