package staticnodes

import (
	"context"
	"testing"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
)

func TestProviderSelectsSortedLimitedNodes(t *testing.T) {
	provider, err := NewProvider([]domain.ProxyNode{
		testNode("b", 20, 100),
		testNode("a", 10, 50),
		testNode("c", 10, 100),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	nodes, err := provider.SelectNodes(context.Background(), ports.NodeSelectionRequest{MaxNodes: 2})
	if err != nil {
		t.Fatalf("SelectNodes returned error: %v", err)
	}

	if len(nodes) != 2 || nodes[0].ID != "c" || nodes[1].ID != "a" {
		t.Fatalf("nodes = %+v", nodes)
	}
}

func TestNewProviderFromJSONUsesFallback(t *testing.T) {
	provider, err := NewProviderFromJSON("", []domain.ProxyNode{testNode("a", 10, 100)})
	if err != nil {
		t.Fatalf("NewProviderFromJSON returned error: %v", err)
	}
	nodes, err := provider.SelectNodes(context.Background(), ports.NodeSelectionRequest{})
	if err != nil {
		t.Fatalf("SelectNodes returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "a" {
		t.Fatalf("nodes = %+v", nodes)
	}
}

func testNode(id string, priority int, weight int) domain.ProxyNode {
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
		Priority:   priority,
		Weight:     weight,
	}
}
