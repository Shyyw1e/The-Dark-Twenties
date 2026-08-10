package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProvisionSubscription(t *testing.T) {
	expiresAt := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/configs/subscription/provision" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}

		var request provisionSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.UserID != "user-1" || request.SubscriptionID != "subscription-1" || request.PublicBaseURL != "https://vpn.example.com" {
			t.Fatalf("request = %+v", request)
		}
		if request.ClientType != "happ" || request.Format != "sing-box" {
			t.Fatalf("request defaults = %+v", request)
		}
		if !request.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("expires_at = %v", request.ExpiresAt)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ProvisionSubscriptionOutput{
			SubscriptionURL: "https://vpn.example.com/sub/token-1",
			TokenID:         "token-1",
			ProfileVersion:  1,
			ClientType:      "happ",
			Format:          "sing-box",
			ExpiresAt:       expiresAt.Format(time.RFC3339Nano),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "https://vpn.example.com/", server.Client())
	output, err := client.ProvisionSubscription(context.Background(), ProvisionSubscriptionInput{
		UserID: "user-1",
		Subscription: &subscriptionv1.Subscription{
			Id:        "subscription-1",
			ExpiresAt: timestamppb.New(expiresAt),
		},
	})
	if err != nil {
		t.Fatalf("ProvisionSubscription returned error: %v", err)
	}
	if output.SubscriptionURL != "https://vpn.example.com/sub/token-1" {
		t.Fatalf("output = %+v", output)
	}
}

func TestProvisionSubscriptionValidatesDependencies(t *testing.T) {
	_, err := NewClient("", "https://vpn.example.com", nil).ProvisionSubscription(context.Background(), ProvisionSubscriptionInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}
