package user

import (
	"context"
	"errors"
	"strings"

	userv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	api  userv1.UserServiceClient
}

type TelegramUserInput struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
}

func Dial(ctx context.Context, addr string) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	addr = normalizeAddr(addr)
	if addr == "" {
		return nil, errors.New("user-service grpc addr is empty")
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
		api:  userv1.NewUserServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) GetOrCreateTelegramUser(ctx context.Context, input TelegramUserInput) (*userv1.User, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("user-service grpc client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return c.api.GetOrCreateTelegramUser(ctx, &userv1.GetOrCreateTelegramUserRequest{
		TelegramId:   input.TelegramID,
		Username:     input.Username,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		LanguageCode: input.LanguageCode,
	})
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}
