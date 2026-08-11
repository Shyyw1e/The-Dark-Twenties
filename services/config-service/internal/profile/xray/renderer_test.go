package xray

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
)

func TestRenderProfileMatchesReferenceShape(t *testing.T) {
	rendered, err := NewRenderer().RenderProfile(context.Background(), ports.ProfileRenderRequest{
		ClientType: "happ",
		Format:     Format,
		Nodes: []domain.ProxyNode{
			testNode("node-1"),
			testNode("node-2"),
		},
	})
	if err != nil {
		t.Fatalf("RenderProfile returned error: %v", err)
	}

	if rendered.Format != Format || rendered.ServerCount != 2 || rendered.NodeRefs == "" || rendered.Metadata == "" {
		t.Fatalf("rendered = %+v", rendered)
	}

	var profile map[string]any
	if err := json.Unmarshal([]byte(rendered.Content), &profile); err != nil {
		t.Fatalf("profile is not json: %v\n%s", err, rendered.Content)
	}
	if profile["burstObservatory"] == nil || profile["routing"] == nil {
		t.Fatalf("profile missing reference sections: %s", rendered.Content)
	}

	outbounds := profile["outbounds"].([]any)
	if len(outbounds) != 4 {
		t.Fatalf("outbounds = %d, want 4", len(outbounds))
	}
	if outbounds[0].(map[string]any)["tag"] != "proxy" || outbounds[1].(map[string]any)["tag"] != "proxy-2" {
		t.Fatalf("outbound tags = %+v", outbounds)
	}

	routing := profile["routing"].(map[string]any)
	balancers := routing["balancers"].([]any)
	strategy := balancers[0].(map[string]any)["strategy"].(map[string]any)
	if strategy["type"] != "leastLoad" {
		t.Fatalf("strategy = %+v", strategy)
	}
}

func TestRenderProfileRequiresNodes(t *testing.T) {
	_, err := NewRenderer().RenderProfile(context.Background(), ports.ProfileRenderRequest{Format: Format})
	if err == nil {
		t.Fatal("expected error")
	}
}

func testNode(id string) domain.ProxyNode {
	return domain.ProxyNode{
		ID:         id,
		Address:    id + ".example.invalid",
		Port:       443,
		Protocol:   "vless",
		UserID:     "00000000-0000-4000-8000-000000000001",
		Network:    "xhttp",
		Security:   "reality",
		PublicKey:  "public-key",
		ServerName: "example.com",
		ShortID:    "short-id",
	}
}
