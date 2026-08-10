package http

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/usecase"
)

type Handler struct {
	configs *usecase.Service
}

type errorResponse struct {
	Error string `json:"error"`
}

type provisionSubscriptionRequest struct {
	UserID         string    `json:"user_id"`
	SubscriptionID string    `json:"subscription_id"`
	ExpiresAt      time.Time `json:"expires_at"`
	ClientType     string    `json:"client_type"`
	Format         string    `json:"format"`
	PublicBaseURL  string    `json:"public_base_url"`
}

type provisionSubscriptionResponse struct {
	SubscriptionURL string `json:"subscription_url"`
	TokenID         string `json:"token_id"`
	ProfileVersion  int    `json:"profile_version"`
	ClientType      string `json:"client_type"`
	Format          string `json:"format"`
	ExpiresAt       string `json:"expires_at"`
}

func NewHandler(configs *usecase.Service) *Handler {
	return &Handler{configs: configs}
}

func RegisterRoutes(mux *http.ServeMux, handler *Handler) {
	mux.HandleFunc("/sub/", handler.RefreshSubscription)
	mux.HandleFunc("/internal/configs/subscription/provision", handler.ProvisionSubscription)
}

func (h *Handler) ProvisionSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h == nil || h.configs == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "config usecase is nil"})
		return
	}

	var req provisionSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}

	output, err := h.configs.ProvisionSubscription(r.Context(), usecase.ProvisionSubscriptionInput{
		UserID:         req.UserID,
		SubscriptionID: req.SubscriptionID,
		ExpiresAt:      req.ExpiresAt,
		ClientType:     req.ClientType,
		Format:         req.Format,
		PublicBaseURL:  req.PublicBaseURL,
	})
	if err != nil {
		writeJSON(w, statusCodeForError(err), errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, provisionSubscriptionResponse{
		SubscriptionURL: output.SubscriptionURL,
		TokenID:         output.TokenID,
		ProfileVersion:  output.ProfileVersion,
		ClientType:      output.ClientType,
		Format:          output.Format,
		ExpiresAt:       output.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func (h *Handler) RefreshSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h == nil || h.configs == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "config usecase is nil"})
		return
	}

	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, "/") {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "subscription token is required"})
		return
	}

	output, err := h.configs.RefreshSubscription(r.Context(), usecase.RefreshSubscriptionInput{
		Token:     token,
		UserAgent: r.UserAgent(),
		SourceIP:  clientIP(r),
	})
	if err != nil {
		writeJSON(w, statusCodeForError(err), errorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", output.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(output.Content))
}

func statusCodeForError(err error) int {
	switch {
	case errors.Is(err, domain.ErrSubscriptionTokenNotFound), errors.Is(err, domain.ErrConfigProfileNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrSubscriptionTokenExpired):
		return http.StatusGone
	case errors.Is(err, domain.ErrSubscriptionTokenInactive), errors.Is(err, domain.ErrActiveSubscriptionNotFound):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
