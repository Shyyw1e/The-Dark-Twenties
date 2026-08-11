package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/profile/xray"
)

type Service struct {
	repository            ports.Repository
	subscriptionChecker   ports.SubscriptionChecker
	nodeProvider          ports.NodeProvider
	profileRenderer       ports.ProfileRenderer
	maxProfileNodes       int
	now                   func() time.Time
	hashSubscriptionToken func(token string) string
}

type ServiceOption func(*Service)

type RefreshSubscriptionInput struct {
	Token     string
	UserAgent string
	SourceIP  string
}

type RefreshSubscriptionOutput struct {
	Content     string
	ContentType string
	Format      string
	ClientType  string
}

type ProvisionSubscriptionInput struct {
	UserID         string
	SubscriptionID string
	ExpiresAt      time.Time
	ClientType     string
	Format         string
	PublicBaseURL  string
}

type ProvisionSubscriptionOutput struct {
	SubscriptionURL string
	TokenID         string
	ProfileVersion  int
	ClientType      string
	Format          string
	ExpiresAt       time.Time
}

func NewService(repository ports.Repository, subscriptionChecker ports.SubscriptionChecker, opts ...ServiceOption) *Service {
	service := &Service{
		repository:            repository,
		subscriptionChecker:   subscriptionChecker,
		profileRenderer:       xray.NewRenderer(),
		maxProfileNodes:       4,
		now:                   func() time.Time { return time.Now().UTC() },
		hashSubscriptionToken: HashSubscriptionToken,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithNodeProvider(nodeProvider ports.NodeProvider) ServiceOption {
	return func(s *Service) {
		s.nodeProvider = nodeProvider
	}
}

func WithProfileRenderer(profileRenderer ports.ProfileRenderer) ServiceOption {
	return func(s *Service) {
		s.profileRenderer = profileRenderer
	}
}

func WithMaxProfileNodes(maxProfileNodes int) ServiceOption {
	return func(s *Service) {
		if maxProfileNodes > 0 {
			s.maxProfileNodes = maxProfileNodes
		}
	}
}

func (s *Service) ProvisionSubscription(ctx context.Context, input ProvisionSubscriptionInput) (*ProvisionSubscriptionOutput, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	subscriptionID := strings.TrimSpace(input.SubscriptionID)
	if subscriptionID == "" {
		return nil, errors.New("subscription_id is required")
	}
	expiresAt := input.ExpiresAt.UTC()
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) {
		return nil, errors.New("expires_at must be in the future")
	}

	clientType := normalizeDefault(input.ClientType, "happ")
	format := normalizeDefault(input.Format, xray.Format)
	publicBaseURL := strings.TrimRight(strings.TrimSpace(input.PublicBaseURL), "/")
	if publicBaseURL == "" {
		return nil, errors.New("public_base_url is required")
	}

	token, err := s.repository.FindActiveTokenByUserID(ctx, userID, clientType, format, now)
	if err != nil && !errors.Is(err, domain.ErrSubscriptionTokenNotFound) {
		return nil, err
	}
	createdToken := false
	if token == nil || errors.Is(err, domain.ErrSubscriptionTokenNotFound) {
		token = newSubscriptionToken(userID, subscriptionID, clientType, format, expiresAt, now)
		if err := s.repository.CreateSubscriptionToken(ctx, token); err != nil {
			return nil, err
		}
		createdToken = true
	}

	profile, err := s.repository.FindLatestProfileByTokenID(ctx, token.ID)
	if err != nil && !errors.Is(err, domain.ErrConfigProfileNotFound) {
		return nil, err
	}
	if profile == nil || errors.Is(err, domain.ErrConfigProfileNotFound) || createdToken {
		profile, err = s.newConfigProfile(ctx, userID, token.ID, clientType, format, expiresAt, now)
		if err != nil {
			return nil, err
		}
		if err := s.repository.CreateConfigProfile(ctx, profile); err != nil {
			return nil, err
		}
	}

	return &ProvisionSubscriptionOutput{
		SubscriptionURL: publicBaseURL + "/sub/" + token.ID,
		TokenID:         token.ID,
		ProfileVersion:  profile.ProfileVersion,
		ClientType:      token.ClientType,
		Format:          token.Format,
		ExpiresAt:       expiresAt,
	}, nil
}

func (s *Service) RefreshSubscription(ctx context.Context, input RefreshSubscriptionInput) (*RefreshSubscriptionOutput, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rawToken := strings.TrimSpace(input.Token)
	if rawToken == "" {
		return nil, errors.New("subscription token is required")
	}

	now := s.now()
	token, err := s.repository.FindTokenByHash(ctx, s.hashSubscriptionToken(rawToken))
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	if !token.IsActiveAt(now) {
		if token.Status == domain.TokenStatusActive {
			return nil, domain.ErrSubscriptionTokenExpired
		}
		return nil, domain.ErrSubscriptionTokenInactive
	}

	if s.subscriptionChecker != nil {
		if err := s.subscriptionChecker.HasActiveSubscription(ctx, token.UserID, now); err != nil {
			return nil, err
		}
	}

	profile, err := s.repository.FindLatestProfileByTokenID(ctx, token.ID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrConfigProfileNotFound
	}

	userAgent := strings.TrimSpace(input.UserAgent)
	if err := s.repository.MarkTokenUsed(ctx, token.ID, now, userAgent); err != nil {
		return nil, err
	}
	if err := s.repository.CreateRefreshEvent(ctx, &domain.RefreshEvent{
		ID:                     uuid.NewString(),
		SubscriptionTokenID:    token.ID,
		UserID:                 token.UserID,
		DeviceID:               token.DeviceID,
		ClientType:             token.ClientType,
		UserAgent:              userAgent,
		SourceIPHash:           hashOptional(input.SourceIP),
		ReturnedProfileVersion: profile.ProfileVersion,
		ReturnedServerCount:    profile.ServerCount,
		CreatedAt:              now,
	}); err != nil {
		return nil, err
	}

	return &RefreshSubscriptionOutput{
		Content:     profile.Content,
		ContentType: contentTypeForFormat(profile.Format),
		Format:      profile.Format,
		ClientType:  profile.ClientType,
	}, nil
}

func (s *Service) validate() error {
	if s == nil || s.repository == nil {
		return errors.New("config repository is nil")
	}
	if s.now == nil {
		return errors.New("time source is nil")
	}
	if s.hashSubscriptionToken == nil {
		return errors.New("subscription token hasher is nil")
	}
	if s.profileRenderer == nil {
		return errors.New("profile renderer is nil")
	}
	return nil
}

func HashSubscriptionToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func newSubscriptionToken(userID string, subscriptionID string, clientType string, format string, expiresAt time.Time, now time.Time) *domain.SubscriptionToken {
	tokenID := uuid.NewString()
	return &domain.SubscriptionToken{
		ID:             tokenID,
		UserID:         userID,
		DeviceID:       uuid.NewString(),
		SubscriptionID: subscriptionID,
		TokenHash:      HashSubscriptionToken(tokenID),
		Status:         domain.TokenStatusActive,
		ClientType:     clientType,
		Format:         format,
		ExpiresAt:      &expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s *Service) newConfigProfile(ctx context.Context, userID string, tokenID string, clientType string, format string, expiresAt time.Time, now time.Time) (*domain.ConfigProfile, error) {
	nodes, err := s.selectNodes(ctx, userID, clientType, format)
	if err != nil {
		return nil, err
	}

	rendered, err := s.profileRenderer.RenderProfile(ctx, ports.ProfileRenderRequest{
		UserID:     userID,
		ClientType: clientType,
		Format:     format,
		Nodes:      nodes,
	})
	if err != nil {
		return nil, err
	}

	return &domain.ConfigProfile{
		ID:                  uuid.NewString(),
		SubscriptionTokenID: tokenID,
		ProfileVersion:      1,
		ClientType:          clientType,
		Format:              rendered.Format,
		ServerCount:         rendered.ServerCount,
		Content:             rendered.Content,
		ContentHash:         HashContent(rendered.Content),
		NodeRefs:            rendered.NodeRefs,
		Metadata:            rendered.Metadata,
		GeneratedAt:         now,
		ExpiresAt:           &expiresAt,
	}, nil
}

func (s *Service) selectNodes(ctx context.Context, userID string, clientType string, format string) ([]domain.ProxyNode, error) {
	if s.nodeProvider == nil {
		return nil, errors.New("node provider is nil")
	}

	nodes, err := s.nodeProvider.SelectNodes(ctx, ports.NodeSelectionRequest{
		UserID:     userID,
		ClientType: clientType,
		Format:     format,
		MaxNodes:   s.maxProfileNodes,
	})
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("no proxy nodes available")
	}
	for _, node := range nodes {
		if err := node.Validate(); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func hashOptional(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return HashSubscriptionToken(value)
}

func normalizeDefault(value string, def string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	return value
}

func contentTypeForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "sing-box", xray.Format:
		return "application/json; charset=utf-8"
	case "uri-list", "":
		return "text/plain; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
