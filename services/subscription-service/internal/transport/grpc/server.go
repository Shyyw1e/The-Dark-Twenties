package grpc

import (
	"context"
	"errors"
	"strings"

	subscriptionv1 "github.com/Shyyw1e/The-Dark-Twenties/proto/subscription/v1"
	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/subscription-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer

	subscriptions *usecase.Service
}

func NewServer(subscriptions *usecase.Service) *Server {
	return &Server{subscriptions: subscriptions}
}

func (s *Server) StartTrial(ctx context.Context, req *subscriptionv1.StartTrialRequest) (*subscriptionv1.Subscription, error) {
	if s == nil || s.subscriptions == nil {
		return nil, status.Error(codes.Internal, "subscription usecase is nil")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	subscription, err := s.subscriptions.StartTrial(ctx, usecase.StartTrialInput{
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}

	return toProtoSubscription(subscription), nil
}

func (s *Server) ActivateOrRenew(ctx context.Context, req *subscriptionv1.ActivateOrRenewRequest) (*subscriptionv1.Subscription, error) {
	if s == nil || s.subscriptions == nil {
		return nil, status.Error(codes.Internal, "subscription usecase is nil")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if strings.TrimSpace(req.GetPlanCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "plan_code is required")
	}

	subscription, err := s.subscriptions.ActivateOrRenew(ctx, usecase.ActivateOrRenewInput{
		UserID:    req.GetUserId(),
		PlanCode:  req.GetPlanCode(),
		AutoRenew: req.GetAutoRenew(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}

	return toProtoSubscription(subscription), nil
}

func (s *Server) GetActive(ctx context.Context, req *subscriptionv1.GetActiveRequest) (*subscriptionv1.Subscription, error) {
	if s == nil || s.subscriptions == nil {
		return nil, status.Error(codes.Internal, "subscription usecase is nil")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	subscription, err := s.subscriptions.GetActive(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}

	return toProtoSubscription(subscription), nil
}

func toProtoSubscription(subscription *domain.Subscription) *subscriptionv1.Subscription {
	if subscription == nil {
		return nil
	}

	response := &subscriptionv1.Subscription{
		Id:        subscription.ID,
		UserId:    subscription.UserID,
		PlanId:    subscription.PlanID,
		Status:    string(subscription.Status),
		StartsAt:  timestamppb.New(subscription.StartsAt),
		ExpiresAt: timestamppb.New(subscription.ExpiresAt),
		AutoRenew: subscription.AutoRenew,
		Trial:     subscription.Trial,
		CreatedAt: timestamppb.New(subscription.CreatedAt),
		UpdatedAt: timestamppb.New(subscription.UpdatedAt),
	}
	if subscription.CancelledAt != nil {
		response.CancelledAt = timestamppb.New(*subscription.CancelledAt)
	}

	return response
}

func toProtoPlan(plan *domain.Plan) *subscriptionv1.Plan {
	if plan == nil {
		return nil
	}

	response := &subscriptionv1.Plan{
		Id:           plan.ID,
		Code:         plan.Code,
		Name:         plan.Name,
		Description:  plan.Description,
		PriceAmount:  plan.PriceAmount,
		Currency:     plan.Currency,
		DurationDays: int32(plan.DurationDays),
		DeviceLimit:  int32(plan.DeviceLimit),
		IsTrial:      plan.IsTrial,
		IsActive:     plan.IsActive,
		CreatedAt:    timestamppb.New(plan.CreatedAt),
		UpdatedAt:    timestamppb.New(plan.UpdatedAt),
	}
	if plan.TrafficLimitBytes != nil {
		response.TrafficLimitBytes = plan.TrafficLimitBytes
	}

	return response
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, domain.ErrPlanNotFound), errors.Is(err, domain.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrTrialAlreadyUsed):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
