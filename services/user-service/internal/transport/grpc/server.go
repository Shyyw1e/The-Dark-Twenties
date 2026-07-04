package grpc

import (
	"context"

	userv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/user/v1"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/user-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	userv1.UnimplementedUserServiceServer

	users *usecase.Service
}

func NewServer(users *usecase.Service) *Server {
	return &Server{users: users}
}

func (s *Server) GetOrCreateTelegramUser(ctx context.Context, req *userv1.GetOrCreateTelegramUserRequest) (*userv1.User, error) {
	if s == nil || s.users == nil {
		return nil, status.Error(codes.Internal, "user usecase is nil")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.GetTelegramId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_id must be positive")
	}

	user, err := s.users.GetOrCreateTelegramUser(ctx, usecase.GetOrCreateTelegramUserInput{
		TelegramID:   req.GetTelegramId(),
		Username:     req.GetUsername(),
		FirstName:    req.GetFirstName(),
		LastName:     req.GetLastName(),
		LanguageCode: req.GetLanguageCode(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return toProtoUser(user), nil
}

func toProtoUser(user *domain.User) *userv1.User {
	if user == nil {
		return nil
	}

	response := &userv1.User{
		Id:           user.ID,
		TelegramId:   user.TelegramID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		Status:       string(user.Status),
		IsAdmin:      user.IsAdmin,
		CreatedAt:    timestamppb.New(user.CreatedAt),
		UpdatedAt:    timestamppb.New(user.UpdatedAt),
	}
	if user.BlockedAt != nil {
		response.BlockedAt = timestamppb.New(*user.BlockedAt)
	}

	return response
}
