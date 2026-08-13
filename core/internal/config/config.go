// Package config loads Core configuration from the environment. Defaults are
// safe for a local Compose deployment; an operator who exposes the Agent API
// must explicitly configure trusted proxies or the Agent routes fail closed.
package config

import (
	"fmt"
	"net"
	"net/url"
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
	PublicURL         string
	OIDCIssuerURL     string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCScopes        []string
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

// KeysDir returns the directory holding Core key material (ADR-0003: outside
// PostgreSQL). Defaults to ./keys for local development.
func KeysDir() string {
	return envOr("CORE_KEYS_DIR", "keys")
}

// DatabaseDSN returns the PostgreSQL connection string.
func DatabaseDSN() string {
	return envOr("CORE_DB_DSN", "postgres://autosecrets:autosecrets@localhost:5432/autosecrets")
}

// PublicAgentURL returns the public Agent hostname used in Install Commands
// and the install script. Empty means the install flow is disabled.
func PublicAgentURL() string {
	return os.Getenv("CORE_PUBLIC_AGENT_URL")
}

// ArtifactDir returns the directory holding signed Agent artifacts.
func ArtifactDir() string {
	return envOr("CORE_ARTIFACT_DIR", "artifacts")
}

// InstallCurlOpts returns the environment assignment prepended to generated
// Install Commands (e.g. "AUTOSECRETS_CURL_OPTS='-k'"). Empty in production.
func InstallCurlOpts() string {
	return os.Getenv("CORE_INSTALL_CURL_OPTS")
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
		PublicURL:         strings.TrimRight(os.Getenv("CORE_PUBLIC_URL"), "/"),
		OIDCIssuerURL:     strings.TrimRight(os.Getenv("CORE_OIDC_ISSUER_URL"), "/"),
		OIDCClientID:      os.Getenv("CORE_OIDC_CLIENT_ID"),
		OIDCClientSecret:  os.Getenv("CORE_OIDC_CLIENT_SECRET"),
		OIDCScopes:        oidcScopes(os.Getenv("CORE_OIDC_SCOPES")),
	}, nil
}

// OIDCConfigurationError reports why OIDC is unavailable without making
// local authentication configuration fail.
func (c Config) OIDCConfigurationError() error {
	configured := c.PublicURL != "" || c.OIDCIssuerURL != "" || c.OIDCClientID != "" || c.OIDCClientSecret != ""
	if !configured {
		return fmt.Errorf("OIDC is not configured")
	}
	if c.PublicURL == "" || c.OIDCIssuerURL == "" || c.OIDCClientID == "" {
		return fmt.Errorf("CORE_PUBLIC_URL, CORE_OIDC_ISSUER_URL, and CORE_OIDC_CLIENT_ID are required")
	}
	publicURL, err := url.Parse(c.PublicURL)
	if err != nil || publicURL.Host == "" || publicURL.Path != "" || publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return fmt.Errorf("CORE_PUBLIC_URL must be a canonical origin")
	}
	if publicURL.Scheme != "https" && !(publicURL.Scheme == "http" && isLoopbackHost(publicURL.Hostname())) {
		return fmt.Errorf("CORE_PUBLIC_URL must use HTTPS except on localhost")
	}
	issuer, err := url.Parse(c.OIDCIssuerURL)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || issuer.RawQuery != "" || issuer.Fragment != "" {
		if err == nil && issuer.Scheme == "http" && isLoopbackHost(issuer.Hostname()) && issuer.Host != "" {
			return nil
		}
		return fmt.Errorf("CORE_OIDC_ISSUER_URL must be an HTTPS URL except on localhost")
	}
	return nil
}

func oidcScopes(raw string) []string {
	if raw == "" {
		return []string{"openid", "profile"}
	}
	seen := map[string]bool{"openid": true}
	result := []string{"openid"}
	for _, scope := range strings.Fields(strings.ReplaceAll(raw, ",", " ")) {
		if scope == "offline_access" || seen[scope] {
			continue
		}
		seen[scope] = true
		result = append(result, scope)
	}
	return result
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
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
