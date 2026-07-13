package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
)

type fakeUserRepository struct {
	user *domain.User
}

func (r *fakeUserRepository) FindByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	if r.user == nil {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeUserRepository) Create(ctx context.Context, user *domain.User) error {
	copy := *user
	r.user = &copy
	return nil
}

func (r *fakeUserRepository) UpdateTelegramProfile(ctx context.Context, user *domain.User) error {
	copy := *user
	r.user = &copy
	return nil
}

func TestGetOrCreateTelegramUser(t *testing.T) {
	handler := NewHandler(usecase.NewService(&fakeUserRepository{}))
	body := bytes.NewBufferString(`{"telegram_id":1001,"username":"shyywie","first_name":"Shyy","last_name":"Wie","language_code":"ru"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/users/telegram/get-or-create", body)
	rec := httptest.NewRecorder()

	handler.GetOrCreateTelegramUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response userResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID == "" || response.TelegramID != 1001 || response.Username != "shyywie" || response.Status != string(domain.StatusActive) {
		t.Fatalf("response = %+v", response)
	}
}

func TestGetOrCreateTelegramUserRejectsInvalidRequests(t *testing.T) {
	handler := NewHandler(usecase.NewService(&fakeUserRepository{}))

	tests := []struct {
		name   string
		method string
		body   string
		status int
	}{
		{name: "method", method: http.MethodGet, body: `{}`, status: http.StatusMethodNotAllowed},
		{name: "json", method: http.MethodPost, body: `{`, status: http.StatusBadRequest},
		{name: "telegram id", method: http.MethodPost, body: `{"telegram_id":0}`, status: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/internal/users/telegram/get-or-create", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			handler.GetOrCreateTelegramUser(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}
