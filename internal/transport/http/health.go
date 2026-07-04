package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

type healthResponse struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks,omitempty"`
}

type checkResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func RegisterHealthHandlers(mux *http.ServeMux, checks ...HealthCheck) {
	mux.HandleFunc("/health/live", liveHandler)
	mux.HandleFunc("/health/ready", readyHandler(checks))
}

func liveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
	})
}

func readyHandler(checks []HealthCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := healthResponse{
			Status: "ok",
			Checks: make(map[string]checkResult, len(checks)),
		}

		statusCode := http.StatusOK

		for _, check := range checks {
			if check == nil {
				continue
			}

			name := check.Name()
			if name == "" {
				name = "unknown"
			}

			if err := check.Check(ctx); err != nil {
				statusCode = http.StatusServiceUnavailable
				response.Status = "degraded"
				response.Checks[name] = checkResult{
					Status: "failed",
					Error:  err.Error(),
				}
				continue
			}

			response.Checks[name] = checkResult{
				Status: "ok",
			}
		}

		if len(response.Checks) == 0 {
			response.Checks = nil
		}

		writeJSON(w, statusCode, response)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
