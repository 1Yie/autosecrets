// Package config loads Core configuration from the environment. Defaults are
// safe for a local Compose deployment; an operator who exposes the Agent API
// must explicitly configure trusted proxies or the Agent routes fail closed.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	ListenAddr        string
	ManagementBase    string
	AgentBase         string
	TrustedProxyCIDRs []*net.IPNet
	ProxyCertHeader   string
	Version           string
}

const (
	defaultListenAddr     = ":8080"
	defaultManagementBase = "/api/v1"
	defaultAgentBase      = "/agent/v1"
	defaultCertHeader     = "X-Autosecrets-Client-Cert"
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// FromEnv builds a Config from CORE_* environment variables.
func FromEnv() (Config, error) {
	cidrs, err := ParseCIDRs(splitCSV(os.Getenv("CORE_TRUSTED_PROXY_CIDRS")))
	if err != nil {
		return Config{}, err
	}
	return Config{
		ListenAddr:        envOr("CORE_LISTEN_ADDR", defaultListenAddr),
		ManagementBase:    envOr("CORE_MANAGEMENT_BASE", defaultManagementBase),
		AgentBase:         envOr("CORE_AGENT_BASE", defaultAgentBase),
		TrustedProxyCIDRs: cidrs,
		ProxyCertHeader:   envOr("CORE_PROXY_CERT_HEADER", defaultCertHeader),
		Version:           os.Getenv("CORE_VERSION"),
	}, nil
}

// ParseCIDRs parses CIDR notation only. Bare IPs are rejected so the trusted
// proxy scope is always explicit.
func ParseCIDRs(values []string) ([]*net.IPNet, error) {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !strings.Contains(value, "/") {
			return nil, fmt.Errorf("trusted proxy %q is not a CIDR (e.g. 10.0.0.0/8)", value)
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", value, err)
		}
		result = append(result, network)
	}
	return result, nil
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
