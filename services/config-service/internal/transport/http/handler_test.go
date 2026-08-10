package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/usecase"
)

type fakeRepository struct {
	token          *domain.SubscriptionToken
	profile        *domain.ConfigProfile
	event          *domain.RefreshEvent
	createdToken   *domain.SubscriptionToken
	createdProfile *domain.ConfigProfile
	err            error
}

func (r *fakeRepository) FindTokenByHash(ctx context.Context, tokenHash string) (*domain.SubscriptionToken, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.token == nil {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	copy := *r.token
	return &copy, nil
}

func (r *fakeRepository) FindActiveTokenByUserID(ctx context.Context, userID string, clientType string, format string, now time.Time) (*domain.SubscriptionToken, error) {
	if r.token == nil {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	copy := *r.token
	return &copy, nil
}

func (r *fakeRepository) FindLatestProfileByTokenID(ctx context.Context, tokenID string) (*domain.ConfigProfile, error) {
	if r.profile == nil {
		return nil, domain.ErrConfigProfileNotFound
	}
	copy := *r.profile
	return &copy, nil
}

func (r *fakeRepository) CreateSubscriptionToken(ctx context.Context, token *domain.SubscriptionToken) error {
	copy := *token
	r.createdToken = &copy
	r.token = &copy
	return nil
}

func (r *fakeRepository) CreateConfigProfile(ctx context.Context, profile *domain.ConfigProfile) error {
	copy := *profile
	r.createdProfile = &copy
	r.profile = &copy
	return nil
}

func (r *fakeRepository) MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time, userAgent string) error {
	return nil
}

func (r *fakeRepository) CreateRefreshEvent(ctx context.Context, event *domain.RefreshEvent) error {
	copy := *event
	r.event = &copy
	return nil
}

func TestProvisionSubscription(t *testing.T) {
	handler := NewHandler(usecase.NewService(&fakeRepository{}, nil))
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)
	body := bytes.NewBufferString(`{"user_id":"user-1","subscription_id":"subscription-1","expires_at":"` + expiresAt + `","public_base_url":"https://vpn.example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/configs/subscription/provision", body)
	rec := httptest.NewRecorder()

	handler.ProvisionSubscription(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response provisionSubscriptionResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SubscriptionURL == "" || response.TokenID == "" || response.ProfileVersion != 1 {
		t.Fatalf("response = %+v", response)
	}
}

func TestRefreshSubscriptionWritesProfileContent(t *testing.T) {
	repo := &fakeRepository{
		token: &domain.SubscriptionToken{
			ID:     "token-1",
			UserID: "user-1",
			Status: domain.TokenStatusActive,
		},
		profile: &domain.ConfigProfile{
			ID:             "profile-1",
			ProfileVersion: 2,
			Format:         "sing-box",
			Content:        `{"outbounds":[]}`,
		},
	}
	service := usecase.NewService(repo, nil)
	handler := NewHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/sub/plain-token", nil)
	req.Header.Set("User-Agent", "Happ/1.0")
	req.Header.Set("X-Forwarded-For", "192.0.2.1, 198.51.100.1")
	rec := httptest.NewRecorder()

	handler.RefreshSubscription(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q", got)
	}
	if got := rec.Body.String(); got != `{"outbounds":[]}` {
		t.Fatalf("body = %q", got)
	}
	if repo.event == nil || repo.event.SourceIPHash == "" {
		t.Fatalf("refresh event = %+v", repo.event)
	}
}

func TestRefreshSubscriptionRejectsInvalidRequests(t *testing.T) {
	handler := NewHandler(usecase.NewService(&fakeRepository{}, nil))

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "method", method: http.MethodPost, path: "/sub/token", status: http.StatusMethodNotAllowed},
		{name: "missing token", method: http.MethodGet, path: "/sub/", status: http.StatusBadRequest},
		{name: "nested path", method: http.MethodGet, path: "/sub/token/extra", status: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.RefreshSubscription(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}

func TestStatusCodeForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "token not found", err: domain.ErrSubscriptionTokenNotFound, want: http.StatusNotFound},
		{name: "profile not found", err: domain.ErrConfigProfileNotFound, want: http.StatusNotFound},
		{name: "expired token", err: domain.ErrSubscriptionTokenExpired, want: http.StatusGone},
		{name: "inactive token", err: domain.ErrSubscriptionTokenInactive, want: http.StatusForbidden},
		{name: "inactive subscription", err: domain.ErrActiveSubscriptionNotFound, want: http.StatusForbidden},
		{name: "unknown", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusCodeForError(tt.err); got != tt.want {
				t.Fatalf("statusCodeForError() = %d, want %d", got, tt.want)
			}
		})
	}
}
