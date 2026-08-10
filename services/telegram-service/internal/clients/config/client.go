package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
)

type Client struct {
	baseURL       string
	publicBaseURL string
	httpClient    *http.Client
}

type ProvisionSubscriptionInput struct {
	UserID       string
	Subscription *subscriptionv1.Subscription
	ClientType   string
	Format       string
}

type ProvisionSubscriptionOutput struct {
	SubscriptionURL string `json:"subscription_url"`
	TokenID         string `json:"token_id"`
	ProfileVersion  int    `json:"profile_version"`
	ClientType      string `json:"client_type"`
	Format          string `json:"format"`
	ExpiresAt       string `json:"expires_at"`
}

type provisionSubscriptionRequest struct {
	UserID         string    `json:"user_id"`
	SubscriptionID string    `json:"subscription_id"`
	ExpiresAt      time.Time `json:"expires_at"`
	ClientType     string    `json:"client_type"`
	Format         string    `json:"format"`
	PublicBaseURL  string    `json:"public_base_url"`
}

func NewClient(baseURL string, publicBaseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		publicBaseURL: strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
		httpClient:    httpClient,
	}
}

func (c *Client) ProvisionSubscription(ctx context.Context, input ProvisionSubscriptionInput) (*ProvisionSubscriptionOutput, error) {
	if c == nil || c.httpClient == nil {
		return nil, errors.New("config-service http client is nil")
	}
	if c.baseURL == "" {
		return nil, errors.New("config-service internal base url is empty")
	}
	if c.publicBaseURL == "" {
		return nil, errors.New("config-service public base url is empty")
	}
	if input.Subscription == nil {
		return nil, errors.New("subscription is nil")
	}
	if input.Subscription.GetExpiresAt() == nil {
		return nil, errors.New("subscription expires_at is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	payload := provisionSubscriptionRequest{
		UserID:         strings.TrimSpace(input.UserID),
		SubscriptionID: strings.TrimSpace(input.Subscription.GetId()),
		ExpiresAt:      input.Subscription.GetExpiresAt().AsTime(),
		ClientType:     normalizeDefault(input.ClientType, "happ"),
		Format:         normalizeDefault(input.Format, "sing-box"),
		PublicBaseURL:  c.publicBaseURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal provision subscription request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/configs/subscription/provision", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create provision subscription request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provision subscription config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error == "" {
			apiErr.Error = resp.Status
		}
		return nil, fmt.Errorf("provision subscription config: %s", apiErr.Error)
	}

	var output ProvisionSubscriptionOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("decode provision subscription response: %w", err)
	}
	if strings.TrimSpace(output.SubscriptionURL) == "" {
		return nil, errors.New("config-service returned empty subscription url")
	}

	return &output, nil
}

func normalizeDefault(value string, def string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	return value
}
