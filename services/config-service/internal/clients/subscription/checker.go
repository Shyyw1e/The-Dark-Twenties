package subscription

import (
	"context"
	"errors"
	"strings"
	"time"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Checker struct {
	conn *grpc.ClientConn
	api  subscriptionv1.SubscriptionServiceClient
}

func Dial(ctx context.Context, addr string) (*Checker, error) {
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

	return &Checker{
		conn: conn,
		api:  subscriptionv1.NewSubscriptionServiceClient(conn),
	}, nil
}

func (c *Checker) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Checker) HasActiveSubscription(ctx context.Context, userID string, at time.Time) error {
	if c == nil || c.api == nil {
		return errors.New("subscription-service grpc client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := c.api.GetActive(ctx, &subscriptionv1.GetActiveRequest{UserId: userID})
	if err == nil {
		return nil
	}

	if code := status.Code(err); code == codes.NotFound || code == codes.FailedPrecondition {
		return domain.ErrActiveSubscriptionNotFound
	}

	return err
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}
