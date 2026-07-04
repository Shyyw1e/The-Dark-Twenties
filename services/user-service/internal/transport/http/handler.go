package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
)

type Handler struct {
	users *usecase.Service
}

type getOrCreateTelegramUserRequest struct {
	TelegramID   int64  `json:"telegram_id"`
	Username     string `json:"username"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	LanguageCode string `json:"language_code"`
}

type userResponse struct {
	ID           string  `json:"id"`
	TelegramID   int64   `json:"telegram_id"`
	Username     string  `json:"username"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	LanguageCode string  `json:"language_code"`
	Status       string  `json:"status"`
	IsAdmin      bool    `json:"is_admin"`
	BlockedAt    *string `json:"blocked_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(users *usecase.Service) *Handler {
	return &Handler{users: users}
}

func RegisterRoutes(mux *http.ServeMux, handler *Handler) {
	mux.HandleFunc("/internal/users/telegram/get-or-create", handler.GetOrCreateTelegramUser)
}

func (h *Handler) GetOrCreateTelegramUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h == nil || h.users == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "user usecase is nil"})
		return
	}

	var req getOrCreateTelegramUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	if req.TelegramID <= 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "telegram_id must be positive"})
		return
	}

	user, err := h.users.GetOrCreateTelegramUser(r.Context(), usecase.GetOrCreateTelegramUserInput{
		TelegramID:   req.TelegramID,
		Username:     req.Username,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		LanguageCode: req.LanguageCode,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, domain.ErrUserNotFound) {
			status = http.StatusNotFound
		}

		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func toUserResponse(user *domain.User) userResponse {
	response := userResponse{
		ID:           user.ID,
		TelegramID:   user.TelegramID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		Status:       string(user.Status),
		IsAdmin:      user.IsAdmin,
		CreatedAt:    formatTime(user.CreatedAt),
		UpdatedAt:    formatTime(user.UpdatedAt),
	}
	if user.BlockedAt != nil {
		blockedAt := formatTime(*user.BlockedAt)
		response.BlockedAt = &blockedAt
	}
	return response
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
