package config

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProvisionSubscription(t *testing.T) {
	expiresAt := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
		if request.ClientType != "happ" || request.Format != "xray-json" {
			t.Fatalf("request defaults = %+v", request)
		}
		if !request.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("expires_at = %v", request.ExpiresAt)
		}

		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(ProvisionSubscriptionOutput{
			SubscriptionURL: "https://vpn.example.com/sub/token-1",
			TokenID:         "token-1",
			ProfileVersion:  1,
			ClientType:      "happ",
			Format:          "xray-json",
			ExpiresAt:       expiresAt.Format(time.RFC3339Nano),
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(&body),
		}, nil
	})

	client := NewClient("http://config-service.local", "https://vpn.example.com/", &http.Client{Transport: transport})
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

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
