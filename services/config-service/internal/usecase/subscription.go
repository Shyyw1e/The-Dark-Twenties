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
)

type Service struct {
	repository            ports.Repository
	subscriptionChecker   ports.SubscriptionChecker
	now                   func() time.Time
	hashSubscriptionToken func(token string) string
}

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

func NewService(repository ports.Repository, subscriptionChecker ports.SubscriptionChecker) *Service {
	return &Service{
		repository:            repository,
		subscriptionChecker:   subscriptionChecker,
		now:                   func() time.Time { return time.Now().UTC() },
		hashSubscriptionToken: HashSubscriptionToken,
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
	format := normalizeDefault(input.Format, "sing-box")
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
		profile = newConfigProfile(token.ID, clientType, format, expiresAt, now)
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

func newConfigProfile(tokenID string, clientType string, format string, expiresAt time.Time, now time.Time) *domain.ConfigProfile {
	content := buildProfileContent(clientType, format)
	return &domain.ConfigProfile{
		ID:                  uuid.NewString(),
		SubscriptionTokenID: tokenID,
		ProfileVersion:      1,
		ClientType:          clientType,
		Format:              format,
		ServerCount:         0,
		Content:             content,
		ContentHash:         HashContent(content),
		NodeRefs:            "[]",
		Metadata:            `{"source":"config-service","profile_kind":"base"}`,
		GeneratedAt:         now,
		ExpiresAt:           &expiresAt,
	}
}

func buildProfileContent(clientType string, format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "sing-box":
		return strings.Join([]string{
			`{`,
			`  "log": { "level": "info" },`,
			`  "dns": {`,
			`    "servers": [`,
			`      { "tag": "cloudflare", "address": "1.1.1.1" },`,
			`      { "tag": "google", "address": "8.8.8.8" }`,
			`    ],`,
			`    "strategy": "ipv4_only"`,
			`  },`,
			`  "inbounds": [`,
			`    { "type": "tun", "tag": "tun-in", "interface_name": "tdt0", "inet4_address": "172.19.0.1/30", "auto_route": true, "strict_route": false, "sniff": true }`,
			`  ],`,
			`  "outbounds": [`,
			`    { "type": "selector", "tag": "proxy", "outbounds": ["direct"], "default": "direct" },`,
			`    { "type": "direct", "tag": "direct" },`,
			`    { "type": "block", "tag": "block" }`,
			`  ],`,
			`  "route": {`,
			`    "rules": [`,
			`      { "protocol": "bittorrent", "outbound": "block" },`,
			`      { "domain_suffix": [".ru", ".su", ".рф"], "outbound": "direct" }`,
			`    ],`,
			`    "final": "proxy"`,
			`  }`,
			`}`,
		}, "\n")
	default:
		return "# The Dark Twenties subscription\n"
	}
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
	case "json", "sing-box":
		return "application/json; charset=utf-8"
	case "uri-list", "":
		return "text/plain; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
