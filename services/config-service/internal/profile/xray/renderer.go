package xray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/domain"
	"github.com/Shyyw1e/The-Dark-Twenties/services/config-service/internal/ports"
)

const Format = "xray-json"

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderProfile(ctx context.Context, request ports.ProfileRenderRequest) (*ports.RenderedProfile, error) {
	format := strings.TrimSpace(request.Format)
	if format == "" {
		format = Format
	}
	if format != Format && format != "json" {
		return nil, fmt.Errorf("unsupported xray profile format %q", request.Format)
	}
	if len(request.Nodes) == 0 {
		return nil, errors.New("profile requires at least one proxy node")
	}

	profile := newProfile(request.Nodes)
	content, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray profile: %w", err)
	}

	nodeRefs, err := marshalNodeRefs(request.Nodes)
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]any{
		"source":       "config-service",
		"profile_kind": "xray-reference",
		"renderer":     Format,
		"node_count":   len(request.Nodes),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal profile metadata: %w", err)
	}

	return &ports.RenderedProfile{
		Content:     string(content),
		Format:      Format,
		ServerCount: len(request.Nodes),
		NodeRefs:    string(nodeRefs),
		Metadata:    string(metadata),
	}, nil
}

type profile struct {
	BurstObservatory burstObservatory `json:"burstObservatory"`
	DNS              dns              `json:"dns"`
	Inbounds         []inbound        `json:"inbounds"`
	Meta             meta             `json:"meta"`
	Outbounds        []outbound       `json:"outbounds"`
	Remark           string           `json:"remark"`
	Remarks          string           `json:"remarks"`
	Routing          routing          `json:"routing"`
}

type burstObservatory struct {
	PingConfig      pingConfig `json:"pingConfig"`
	SubjectSelector []string   `json:"subjectSelector"`
}

type pingConfig struct {
	Connectivity string `json:"connectivity"`
	Destination  string `json:"destination"`
	Interval     string `json:"interval"`
	Sampling     int    `json:"sampling"`
	Timeout      string `json:"timeout"`
}

type dns struct {
	QueryStrategy string   `json:"queryStrategy"`
	Servers       []string `json:"servers"`
}

type inbound struct {
	Listen   string          `json:"listen"`
	Port     int             `json:"port"`
	Protocol string          `json:"protocol"`
	Settings inboundSettings `json:"settings"`
	Sniffing inboundSniffing `json:"sniffing"`
	Tag      string          `json:"tag"`
}

type inboundSettings struct {
	Auth             string `json:"auth,omitempty"`
	UDP              bool   `json:"udp,omitempty"`
	AllowTransparent bool   `json:"allowTransparent,omitempty"`
}

type inboundSniffing struct {
	DestOverride []string `json:"destOverride"`
	Enabled      bool     `json:"enabled"`
	RouteOnly    bool     `json:"routeOnly"`
}

type meta struct {
	ServerDescription string `json:"serverDescription"`
}

type outbound struct {
	Protocol       string          `json:"protocol"`
	Settings       outboundSetting `json:"settings"`
	StreamSettings *streamSettings `json:"streamSettings,omitempty"`
	Tag            string          `json:"tag"`
}

type outboundSetting struct {
	VNext []vnext `json:"vnext,omitempty"`
}

type vnext struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []vlessUser `json:"users"`
}

type vlessUser struct {
	Encryption string `json:"encryption"`
	Flow       string `json:"flow,omitempty"`
	ID         string `json:"id"`
}

type streamSettings struct {
	Network         string          `json:"network"`
	RealitySettings realitySettings `json:"realitySettings"`
	Security        string          `json:"security"`
	XHTTPSettings   *xhttpSettings  `json:"xhttpSettings,omitempty"`
}

type realitySettings struct {
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"publicKey"`
	ServerName  string `json:"serverName"`
	ShortID     string `json:"shortId"`
	SpiderX     string `json:"spiderX"`
}

type xhttpSettings struct {
	Mode string `json:"mode"`
	Path string `json:"path"`
}

type routing struct {
	Balancers      []balancer `json:"balancers"`
	DomainMatcher  string     `json:"domainMatcher"`
	DomainStrategy string     `json:"domainStrategy"`
	Rules          []rule     `json:"rules"`
}

type balancer struct {
	FallbackTag string           `json:"fallbackTag"`
	Selector    []string         `json:"selector"`
	Strategy    balancerStrategy `json:"strategy"`
	Tag         string           `json:"tag"`
}

type balancerStrategy struct {
	Settings balancerSettings `json:"settings"`
	Type     string           `json:"type"`
}

type balancerSettings struct {
	Baselines []string `json:"baselines"`
	Expected  int      `json:"expected"`
	MaxRTT    string   `json:"maxRTT"`
	Tolerance float64  `json:"tolerance"`
}

type rule struct {
	BalancerTag string   `json:"balancerTag,omitempty"`
	Network     string   `json:"network,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	Protocol    []string `json:"protocol,omitempty"`
	Type        string   `json:"type"`
}

func newProfile(nodes []domain.ProxyNode) profile {
	proxyTags := make([]string, 0, len(nodes))
	outbounds := make([]outbound, 0, len(nodes)+2)
	for i, node := range nodes {
		tag := "proxy"
		if i > 0 {
			tag = fmt.Sprintf("proxy-%d", i+1)
		}
		proxyTags = append(proxyTags, tag)
		outbounds = append(outbounds, newVLESSOutbound(tag, node))
	}
	outbounds = append(outbounds,
		outbound{Protocol: "freedom", Settings: outboundSetting{}, Tag: "direct"},
		outbound{Protocol: "blackhole", Settings: outboundSetting{}, Tag: "block"},
	)

	return profile{
		BurstObservatory: burstObservatory{
			PingConfig: pingConfig{
				Connectivity: "",
				Destination:  "http://www.gstatic.com/generate_204",
				Interval:     "5m",
				Sampling:     1,
				Timeout:      "3s",
			},
			SubjectSelector: []string{"proxy"},
		},
		DNS: dns{
			QueryStrategy: "UseIP",
			Servers:       []string{"1.1.1.1", "1.0.0.1"},
		},
		Inbounds: []inbound{
			{
				Listen:   "127.0.0.1",
				Port:     10808,
				Protocol: "socks",
				Settings: inboundSettings{Auth: "noauth", UDP: true},
				Sniffing: defaultSniffing(),
				Tag:      "socks",
			},
			{
				Listen:   "127.0.0.1",
				Port:     10809,
				Protocol: "http",
				Settings: inboundSettings{AllowTransparent: false},
				Sniffing: defaultSniffing(),
				Tag:      "http",
			},
		},
		Meta:      meta{ServerDescription: "Auto bridge selection by client ping"},
		Outbounds: outbounds,
		Remark:    "Auto fastest",
		Remarks:   "Auto fastest",
		Routing: routing{
			Balancers: []balancer{
				{
					FallbackTag: proxyTags[0],
					Selector:    proxyTags,
					Strategy: balancerStrategy{
						Settings: balancerSettings{
							Baselines: []string{"1.5s"},
							Expected:  1,
							MaxRTT:    "3s",
							Tolerance: 0.15,
						},
						Type: "leastLoad",
					},
					Tag: "auto-bypass",
				},
			},
			DomainMatcher:  "hybrid",
			DomainStrategy: "IPIfNonMatch",
			Rules: []rule{
				{
					OutboundTag: "direct",
					Protocol:    []string{"bittorrent"},
					Type:        "field",
				},
				{
					BalancerTag: "auto-bypass",
					Network:     "tcp,udp",
					Type:        "field",
				},
			},
		},
	}
}

func newVLESSOutbound(tag string, node domain.ProxyNode) outbound {
	user := vlessUser{
		Encryption: "none",
		ID:         node.UserID,
	}
	if strings.TrimSpace(node.Flow) != "" {
		user.Flow = node.Flow
	}

	settings := streamSettings{
		Network: normalizeDefault(node.Network, "xhttp"),
		RealitySettings: realitySettings{
			Fingerprint: normalizeDefault(node.Fingerprint, "chrome"),
			PublicKey:   node.PublicKey,
			ServerName:  node.ServerName,
			ShortID:     node.ShortID,
			SpiderX:     normalizeDefault(node.SpiderX, "/"),
		},
		Security: normalizeDefault(node.Security, "reality"),
	}
	if settings.Network == "xhttp" {
		settings.XHTTPSettings = &xhttpSettings{
			Mode: normalizeDefault(node.XHTTPMode, "stream-one"),
			Path: normalizeDefault(node.XHTTPPath, "/api/v1/update"),
		}
	}

	return outbound{
		Protocol: "vless",
		Settings: outboundSetting{
			VNext: []vnext{
				{
					Address: node.Address,
					Port:    node.Port,
					Users:   []vlessUser{user},
				},
			},
		},
		StreamSettings: &settings,
		Tag:            tag,
	}
}

func defaultSniffing() inboundSniffing {
	return inboundSniffing{
		DestOverride: []string{"http", "tls", "quic"},
		Enabled:      true,
		RouteOnly:    false,
	}
}

func marshalNodeRefs(nodes []domain.ProxyNode) ([]byte, error) {
	refs := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		refs = append(refs, map[string]any{
			"id":      node.ID,
			"name":    node.Name,
			"region":  node.Region,
			"country": node.Country,
			"role":    string(node.Role),
		})
	}

	value, err := json.Marshal(refs)
	if err != nil {
		return nil, fmt.Errorf("marshal node refs: %w", err)
	}
	return value, nil
}

func normalizeDefault(value string, def string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	return value
}
