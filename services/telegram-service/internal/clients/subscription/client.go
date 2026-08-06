package subscription

import (
	"context"
	"errors"
	"strings"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	api  subscriptionv1.SubscriptionServiceClient
}

type ActivateOrRenewInput struct {
	UserID    string
	PlanCode  string
	AutoRenew bool
}

func Dial(ctx context.Context, addr string) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	addr = normalizeAddr(addr)
	if addr == "" {
		return nil, errors.New("subscription-service grpc addr is empty")
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn: conn,
		api:  subscriptionv1.NewSubscriptionServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) StartTrial(ctx context.Context, userID string) (*subscriptionv1.Subscription, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("subscription-service grpc client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return c.api.StartTrial(ctx, &subscriptionv1.StartTrialRequest{UserId: userID})
}

func (c *Client) GetActive(ctx context.Context, userID string) (*subscriptionv1.Subscription, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("subscription-service grpc client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return c.api.GetActive(ctx, &subscriptionv1.GetActiveRequest{UserId: userID})
}

func (c *Client) ActivateOrRenew(ctx context.Context, input ActivateOrRenewInput) (*subscriptionv1.Subscription, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("subscription-service grpc client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return c.api.ActivateOrRenew(ctx, &subscriptionv1.ActivateOrRenewRequest{
		UserId:    input.UserID,
		PlanCode:  input.PlanCode,
		AutoRenew: input.AutoRenew,
	})
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}
