package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
)

type fakeRepository struct {
	tokenByHash map[string]*domain.SubscriptionToken
	activeToken *domain.SubscriptionToken
	profileByID map[string]*domain.ConfigProfile

	findTokenErr       error
	findActiveTokenErr error
	findProfileErr     error
	createTokenErr     error
	createProfileErr   error
	markUsedErr        error
	createEventErr     error

	foundHash       string
	markedTokenID   string
	markedUsedAt    time.Time
	markedUserAgent string
	createdTokens   []*domain.SubscriptionToken
	createdProfiles []*domain.ConfigProfile
	events          []*domain.RefreshEvent
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		tokenByHash: make(map[string]*domain.SubscriptionToken),
		profileByID: make(map[string]*domain.ConfigProfile),
	}
}

func (r *fakeRepository) FindTokenByHash(ctx context.Context, tokenHash string) (*domain.SubscriptionToken, error) {
	r.foundHash = tokenHash
	if r.findTokenErr != nil {
		return nil, r.findTokenErr
	}
	token, ok := r.tokenByHash[tokenHash]
	if !ok {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	copy := *token
	return &copy, nil
}

func (r *fakeRepository) FindActiveTokenByUserID(ctx context.Context, userID string, clientType string, format string, now time.Time) (*domain.SubscriptionToken, error) {
	if r.findActiveTokenErr != nil {
		return nil, r.findActiveTokenErr
	}
	if r.activeToken == nil {
		return nil, domain.ErrSubscriptionTokenNotFound
	}
	copy := *r.activeToken
	return &copy, nil
}

func (r *fakeRepository) FindLatestProfileByTokenID(ctx context.Context, tokenID string) (*domain.ConfigProfile, error) {
	if r.findProfileErr != nil {
		return nil, r.findProfileErr
	}
	profile, ok := r.profileByID[tokenID]
	if !ok {
		return nil, domain.ErrConfigProfileNotFound
	}
	copy := *profile
	return &copy, nil
}

func (r *fakeRepository) CreateSubscriptionToken(ctx context.Context, token *domain.SubscriptionToken) error {
	if r.createTokenErr != nil {
		return r.createTokenErr
	}
	copy := *token
	r.createdTokens = append(r.createdTokens, &copy)
	r.activeToken = &copy
	return nil
}

func (r *fakeRepository) CreateConfigProfile(ctx context.Context, profile *domain.ConfigProfile) error {
	if r.createProfileErr != nil {
		return r.createProfileErr
	}
	copy := *profile
	r.createdProfiles = append(r.createdProfiles, &copy)
	r.profileByID[profile.SubscriptionTokenID] = &copy
	return nil
}

func (r *fakeRepository) MarkTokenUsed(ctx context.Context, tokenID string, usedAt time.Time, userAgent string) error {
	r.markedTokenID = tokenID
	r.markedUsedAt = usedAt
	r.markedUserAgent = userAgent
	return r.markUsedErr
}

func (r *fakeRepository) CreateRefreshEvent(ctx context.Context, event *domain.RefreshEvent) error {
	if r.createEventErr != nil {
		return r.createEventErr
	}
	copy := *event
	r.events = append(r.events, &copy)
	return nil
}

type fakeSubscriptionChecker struct {
	err    error
	userID string
	at     time.Time
	calls  int
}

func (c *fakeSubscriptionChecker) HasActiveSubscription(ctx context.Context, userID string, at time.Time) error {
	c.calls++
	c.userID = userID
	c.at = at
	return c.err
}

type fakeNodeProvider struct {
	nodes []domain.ProxyNode
	err   error
}

func (p fakeNodeProvider) SelectNodes(ctx context.Context, request ports.NodeSelectionRequest) ([]domain.ProxyNode, error) {
	if p.err != nil {
		return nil, p.err
	}
	limit := request.MaxNodes
	if limit <= 0 || limit > len(p.nodes) {
		limit = len(p.nodes)
	}
	nodes := make([]domain.ProxyNode, 0, limit)
	nodes = append(nodes, p.nodes[:limit]...)
	return nodes, nil
}

func testProxyNode(id string) domain.ProxyNode {
	return domain.ProxyNode{
		ID:          id,
		Name:        id,
		Region:      "eu-west",
		Country:     "NL",
		Role:        domain.NodeRoleForeignExit,
		Address:     id + ".example.invalid",
		Port:        443,
		Protocol:    "vless",
		UserID:      "00000000-0000-4000-8000-000000000001",
		Network:     "xhttp",
		Security:    "reality",
		Fingerprint: "chrome",
		PublicKey:   "public-key",
		ServerName:  "example.com",
		ShortID:     "short-id",
		SpiderX:     "/",
		XHTTPMode:   "stream-one",
		XHTTPPath:   "/api/v1/update",
	}
}

func TestRefreshSubscriptionReturnsLatestProfileAndWritesAudit(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	repo := newFakeRepository()
	tokenHash := HashSubscriptionToken("secret-token")
	repo.tokenByHash[tokenHash] = &domain.SubscriptionToken{
		ID:         "token-1",
		UserID:     "user-1",
		DeviceID:   "device-1",
		TokenHash:  tokenHash,
		Status:     domain.TokenStatusActive,
		ClientType: "happ",
		Format:     "sing-box",
		ExpiresAt:  &expiresAt,
	}
	repo.profileByID["token-1"] = &domain.ConfigProfile{
		ID:                  "profile-1",
		SubscriptionTokenID: "token-1",
		ProfileVersion:      3,
		ClientType:          "happ",
		Format:              "sing-box",
		ServerCount:         2,
		Content:             `{"outbounds":[]}`,
	}
	checker := &fakeSubscriptionChecker{}
	service := NewService(repo, checker)
	service.now = func() time.Time { return now }

	output, err := service.RefreshSubscription(context.Background(), RefreshSubscriptionInput{
		Token:     " secret-token ",
		UserAgent: " Happ/1.0 ",
		SourceIP:  " 127.0.0.1 ",
	})
	if err != nil {
		t.Fatalf("RefreshSubscription returned error: %v", err)
	}

	if output.Content != `{"outbounds":[]}` || output.ContentType != "application/json; charset=utf-8" {
		t.Fatalf("output = %+v", output)
	}
	if repo.foundHash != tokenHash {
		t.Fatalf("found hash = %q, want %q", repo.foundHash, tokenHash)
	}
	if repo.markedTokenID != "token-1" || !repo.markedUsedAt.Equal(now) || repo.markedUserAgent != "Happ/1.0" {
		t.Fatalf("mark used = %q/%v/%q", repo.markedTokenID, repo.markedUsedAt, repo.markedUserAgent)
	}
	if checker.calls != 1 || checker.userID != "user-1" || !checker.at.Equal(now) {
		t.Fatalf("checker = %+v", checker)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events count = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.ID == "" || event.SubscriptionTokenID != "token-1" || event.UserID != "user-1" || event.DeviceID != "device-1" {
		t.Fatalf("event identity = %+v", event)
	}
	if event.ReturnedProfileVersion != 3 || event.ReturnedServerCount != 2 || event.SourceIPHash == "" {
		t.Fatalf("event payload = %+v", event)
	}
}

func TestProvisionSubscriptionCreatesTokenAndProfile(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(7 * 24 * time.Hour)
	repo := newFakeRepository()
	service := NewService(repo, nil, WithNodeProvider(fakeNodeProvider{nodes: []domain.ProxyNode{testProxyNode("node-1"), testProxyNode("node-2")}}))
	service.now = func() time.Time { return now }

	output, err := service.ProvisionSubscription(context.Background(), ProvisionSubscriptionInput{
		UserID:         " user-1 ",
		SubscriptionID: " subscription-1 ",
		ExpiresAt:      expiresAt,
		PublicBaseURL:  " https://vpn.example.com/ ",
	})
	if err != nil {
		t.Fatalf("ProvisionSubscription returned error: %v", err)
	}

	if len(repo.createdTokens) != 1 {
		t.Fatalf("created tokens = %d, want 1", len(repo.createdTokens))
	}
	token := repo.createdTokens[0]
	if token.ID == "" || token.DeviceID == "" || token.UserID != "user-1" || token.SubscriptionID != "subscription-1" {
		t.Fatalf("token = %+v", token)
	}
	if token.TokenHash != HashSubscriptionToken(token.ID) {
		t.Fatalf("token hash = %q, want hash of id", token.TokenHash)
	}
	if len(repo.createdProfiles) != 1 {
		t.Fatalf("created profiles = %d, want 1", len(repo.createdProfiles))
	}
	profile := repo.createdProfiles[0]
	if profile.SubscriptionTokenID != token.ID || profile.ProfileVersion != 1 || profile.Content == "" || profile.ContentHash != HashContent(profile.Content) {
		t.Fatalf("profile = %+v", profile)
	}
	if output.SubscriptionURL != "https://vpn.example.com/sub/"+token.ID {
		t.Fatalf("subscription url = %q", output.SubscriptionURL)
	}
	if output.ClientType != "happ" || output.Format != "xray-json" {
		t.Fatalf("output = %+v", output)
	}
	if profile.ServerCount != 2 || !strings.Contains(profile.Content, `"protocol": "vless"`) || !strings.Contains(profile.Content, `"burstObservatory"`) {
		t.Fatalf("profile content = %s", profile.Content)
	}
}

func TestProvisionSubscriptionReusesActiveToken(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(7 * 24 * time.Hour)
	repo := newFakeRepository()
	repo.activeToken = &domain.SubscriptionToken{
		ID:         "token-1",
		UserID:     "user-1",
		Status:     domain.TokenStatusActive,
		ClientType: "happ",
		Format:     "xray-json",
	}
	repo.profileByID["token-1"] = &domain.ConfigProfile{
		ID:                  "profile-1",
		SubscriptionTokenID: "token-1",
		ProfileVersion:      2,
		ClientType:          "happ",
		Format:              "xray-json",
		Content:             `{"route":{}}`,
	}
	service := NewService(repo, nil)
	service.now = func() time.Time { return now }

	output, err := service.ProvisionSubscription(context.Background(), ProvisionSubscriptionInput{
		UserID:         "user-1",
		SubscriptionID: "subscription-1",
		ExpiresAt:      expiresAt,
		PublicBaseURL:  "https://vpn.example.com",
	})
	if err != nil {
		t.Fatalf("ProvisionSubscription returned error: %v", err)
	}

	if len(repo.createdTokens) != 0 || len(repo.createdProfiles) != 0 {
		t.Fatalf("created tokens/profiles = %d/%d, want 0/0", len(repo.createdTokens), len(repo.createdProfiles))
	}
	if output.SubscriptionURL != "https://vpn.example.com/sub/token-1" || output.ProfileVersion != 2 {
		t.Fatalf("output = %+v", output)
	}
}

func TestRefreshSubscriptionRejectsExpiredActiveToken(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Second)
	repo := newFakeRepository()
	repo.tokenByHash[HashSubscriptionToken("secret-token")] = &domain.SubscriptionToken{
		ID:        "token-1",
		UserID:    "user-1",
		Status:    domain.TokenStatusActive,
		ExpiresAt: &expiresAt,
	}
	service := NewService(repo, nil)
	service.now = func() time.Time { return now }

	_, err := service.RefreshSubscription(context.Background(), RefreshSubscriptionInput{Token: "secret-token"})
	if !errors.Is(err, domain.ErrSubscriptionTokenExpired) {
		t.Fatalf("error = %v, want %v", err, domain.ErrSubscriptionTokenExpired)
	}
}

func TestRefreshSubscriptionRejectsRevokedToken(t *testing.T) {
	repo := newFakeRepository()
	repo.tokenByHash[HashSubscriptionToken("secret-token")] = &domain.SubscriptionToken{
		ID:     "token-1",
		UserID: "user-1",
		Status: domain.TokenStatusRevoked,
	}

	_, err := NewService(repo, nil).RefreshSubscription(context.Background(), RefreshSubscriptionInput{Token: "secret-token"})
	if !errors.Is(err, domain.ErrSubscriptionTokenInactive) {
		t.Fatalf("error = %v, want %v", err, domain.ErrSubscriptionTokenInactive)
	}
}

func TestRefreshSubscriptionPropagatesSubscriptionCheckerError(t *testing.T) {
	expectedErr := errors.New("subscription is not active")
	repo := newFakeRepository()
	repo.tokenByHash[HashSubscriptionToken("secret-token")] = &domain.SubscriptionToken{
		ID:     "token-1",
		UserID: "user-1",
		Status: domain.TokenStatusActive,
	}
	checker := &fakeSubscriptionChecker{err: expectedErr}

	_, err := NewService(repo, checker).RefreshSubscription(context.Background(), RefreshSubscriptionInput{Token: "secret-token"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("error = %v, want %v", err, expectedErr)
	}
	if repo.markedTokenID != "" || len(repo.events) != 0 {
		t.Fatalf("unexpected side effects: marked=%q events=%d", repo.markedTokenID, len(repo.events))
	}
}
