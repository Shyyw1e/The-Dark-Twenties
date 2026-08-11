package domain

import (
	"errors"
	"strings"
)

type NodeRole string

const (
	NodeRoleForeignExit NodeRole = "foreign_exit"
	NodeRoleRURelay     NodeRole = "ru_relay"
	NodeRoleBridge      NodeRole = "bridge"
)

type ProxyNode struct {
	ID          string
	Name        string
	Region      string
	Country     string
	Role        NodeRole
	Address     string
	Port        int
	Protocol    string
	UserID      string
	Flow        string
	Network     string
	Security    string
	Fingerprint string
	PublicKey   string
	ServerName  string
	ShortID     string
	SpiderX     string
	XHTTPMode   string
	XHTTPPath   string
	Priority    int
	Weight      int
}

func (n ProxyNode) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return errors.New("node id is required")
	}
	if strings.TrimSpace(n.Address) == "" {
		return errors.New("node address is required")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return errors.New("node port is invalid")
	}
	if strings.TrimSpace(n.Protocol) == "" {
		return errors.New("node protocol is required")
	}
	if strings.TrimSpace(n.UserID) == "" {
		return errors.New("node user id is required")
	}
	if strings.TrimSpace(n.Network) == "" {
		return errors.New("node network is required")
	}
	if strings.TrimSpace(n.Security) == "" {
		return errors.New("node security is required")
	}
	return nil
}
