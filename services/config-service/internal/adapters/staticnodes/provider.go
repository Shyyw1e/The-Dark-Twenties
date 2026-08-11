package staticnodes

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
)

type Provider struct {
	nodes []domain.ProxyNode
}

func NewProvider(nodes []domain.ProxyNode) (*Provider, error) {
	if len(nodes) == 0 {
		return nil, errors.New("static node catalog is empty")
	}

	catalog := make([]domain.ProxyNode, 0, len(nodes))
	for _, node := range nodes {
		if err := node.Validate(); err != nil {
			return nil, err
		}
		catalog = append(catalog, node)
	}

	sortNodes(catalog)
	return &Provider{nodes: catalog}, nil
}

func NewProviderFromJSON(raw string, fallback []domain.ProxyNode) (*Provider, error) {
	var nodes []domain.ProxyNode
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
			return nil, err
		}
	}
	if len(nodes) == 0 {
		nodes = fallback
	}
	return NewProvider(nodes)
}

func DefaultNodes() []domain.ProxyNode {
	return []domain.ProxyNode{
		{
			ID:          "dev-node-nl-1",
			Name:        "Netherlands 1",
			Region:      "eu-west",
			Country:     "NL",
			Role:        domain.NodeRoleForeignExit,
			Address:     "node-nl-1.example.invalid",
			Port:        443,
			Protocol:    "vless",
			UserID:      "00000000-0000-4000-8000-000000000001",
			Network:     "xhttp",
			Security:    "reality",
			Fingerprint: "chrome",
			PublicKey:   "REPLACE_WITH_REALITY_PUBLIC_KEY_1",
			ServerName:  "example.com",
			ShortID:     "0000000000000001",
			SpiderX:     "/",
			XHTTPMode:   "stream-one",
			XHTTPPath:   "/api/v1/update",
			Priority:    10,
			Weight:      100,
		},
		{
			ID:          "dev-node-nl-2",
			Name:        "Netherlands 2",
			Region:      "eu-west",
			Country:     "NL",
			Role:        domain.NodeRoleForeignExit,
			Address:     "node-nl-2.example.invalid",
			Port:        443,
			Protocol:    "vless",
			UserID:      "00000000-0000-4000-8000-000000000002",
			Network:     "xhttp",
			Security:    "reality",
			Fingerprint: "chrome",
			PublicKey:   "REPLACE_WITH_REALITY_PUBLIC_KEY_2",
			ServerName:  "example.com",
			ShortID:     "0000000000000002",
			SpiderX:     "/",
			XHTTPMode:   "stream-one",
			XHTTPPath:   "/api/v1/update",
			Priority:    20,
			Weight:      100,
		},
	}
}

func (p *Provider) SelectNodes(ctx context.Context, request ports.NodeSelectionRequest) ([]domain.ProxyNode, error) {
	if p == nil || len(p.nodes) == 0 {
		return nil, errors.New("node provider catalog is empty")
	}

	limit := request.MaxNodes
	if limit <= 0 || limit > len(p.nodes) {
		limit = len(p.nodes)
	}

	selected := make([]domain.ProxyNode, 0, limit)
	for _, node := range p.nodes {
		selected = append(selected, node)
		if len(selected) == limit {
			break
		}
	}

	return selected, nil
}

func sortNodes(nodes []domain.ProxyNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		if nodes[i].Weight != nodes[j].Weight {
			return nodes[i].Weight > nodes[j].Weight
		}
		return nodes[i].ID < nodes[j].ID
	})
}
