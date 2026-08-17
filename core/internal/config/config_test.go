package config

import (
	"net"
	"reflect"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	for _, key := range []string{
		"CORE_LISTEN_ADDR", "CORE_MANAGEMENT_BASE", "CORE_AGENT_BASE",
		"CORE_TRUSTED_PROXY_CIDRS", "CORE_PROXY_CERT_HEADER", "CORE_CORS_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.ManagementBase != "/api/v1" || cfg.AgentBase != "/agent/v1" {
		t.Fatalf("bases = %q %q", cfg.ManagementBase, cfg.AgentBase)
	}
	if len(cfg.TrustedProxyCIDRs) != 0 {
		t.Fatalf("expected no trusted proxies by default, got %v", cfg.TrustedProxyCIDRs)
	}
	if cfg.ProxyCertHeader != "X-Autosecrets-Client-Cert" {
		t.Fatalf("ProxyCertHeader = %q", cfg.ProxyCertHeader)
	}
	if len(cfg.CORSOrigins) != 0 {
		t.Fatalf("expected no CORS origins by default, got %v", cfg.CORSOrigins)
	}
}

func TestFromEnvParsesCIDRs(t *testing.T) {
	t.Setenv("CORE_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	t.Setenv("CORE_LISTEN_ADDR", "127.0.0.1:9090")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 {
		t.Fatalf("got %d CIDRs", len(cfg.TrustedProxyCIDRs))
	}
	if !cfg.TrustedProxyCIDRs[0].Contains(net.ParseIP("10.1.2.3")) {
		t.Fatal("10.1.2.3 should be inside 10.0.0.0/8")
	}
	if cfg.TrustedProxyCIDRs[0].Contains(net.ParseIP("172.16.0.1")) {
		t.Fatal("172.16.0.1 should be outside 10.0.0.0/8")
	}
}

func TestFromEnvRejectsInvalidCIDR(t *testing.T) {
	t.Setenv("CORE_TRUSTED_PROXY_CIDRS", "not-a-cidr")
	if _, err := FromEnv(); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
}

func TestParseCIDRsRejectsBareIPs(t *testing.T) {
	if _, err := ParseCIDRs([]string{"10.0.0.1"}); err == nil {
		t.Fatal("bare IP accepted; CIDR required so scope is explicit")
	}
}

func TestParseCIDRsEmpty(t *testing.T) {
	got, err := ParseCIDRs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []*net.IPNet{}) {
		t.Fatalf("got %v", got)
	}
}

func TestFromEnvParsesCORSOrigins(t *testing.T) {
	t.Setenv("CORE_CORS_ORIGINS",
		"http://154.222.24.116:18081, https://as.ichiyo.in")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://154.222.24.116:18081", "https://as.ichiyo.in"}
	if !reflect.DeepEqual(cfg.CORSOrigins, want) {
		t.Fatalf("CORSOrigins = %v, want %v", cfg.CORSOrigins, want)
	}
}

func TestFromEnvRejectsInvalidCORSOrigin(t *testing.T) {
	t.Setenv("CORE_CORS_ORIGINS", "http://example.com/path")
	if _, err := FromEnv(); err == nil {
		t.Fatal("origin with path accepted")
	}
}

func TestParseOriginsRejectsMissingScheme(t *testing.T) {
	if _, err := ParseOrigins([]string{"example.com"}); err == nil {
		t.Fatal("origin without scheme accepted")
	}
}

func TestOIDCConfigurationIsOptionalAndValidatedSeparately(t *testing.T) {
	for _, key := range []string{"CORE_PUBLIC_URL", "CORE_OIDC_ISSUER_URL", "CORE_OIDC_CLIENT_ID", "CORE_OIDC_CLIENT_SECRET", "CORE_OIDC_SCOPES"} {
		t.Setenv(key, "")
	}
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCConfigurationError() == nil {
		t.Fatal("missing OIDC configuration reported as available")
	}

	t.Setenv("CORE_PUBLIC_URL", "http://localhost:8080")
	t.Setenv("CORE_OIDC_ISSUER_URL", "http://127.0.0.1:5556")
	t.Setenv("CORE_OIDC_CLIENT_ID", "autosecrets")
	t.Setenv("CORE_OIDC_SCOPES", "profile,email,offline_access")
	cfg, err = FromEnv()
	if err != nil || cfg.OIDCConfigurationError() != nil {
		t.Fatalf("valid development OIDC config rejected: %v / %v", err, cfg.OIDCConfigurationError())
	}
	want := []string{"openid", "profile", "email"}
	if !reflect.DeepEqual(cfg.OIDCScopes, want) {
		t.Fatalf("scopes = %v, want %v", cfg.OIDCScopes, want)
	}
}

func TestOAuthConfigurationIsOptionalAndValidatedSeparately(t *testing.T) {
	for _, key := range []string{
		"CORE_PUBLIC_URL", "CORE_OAUTH_AUTHORIZATION_URL", "CORE_OAUTH_TOKEN_URL",
		"CORE_OAUTH_USERINFO_URL", "CORE_OAUTH_CLIENT_ID", "CORE_OAUTH_CLIENT_SECRET", "CORE_OAUTH_SCOPES",
	} {
		t.Setenv(key, "")
	}
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuthConfigurationError() == nil {
		t.Fatal("missing OAuth configuration reported as available")
	}

	t.Setenv("CORE_PUBLIC_URL", "http://localhost:8080")
	t.Setenv("CORE_OAUTH_AUTHORIZATION_URL", "http://127.0.0.1:5556/authorize")
	t.Setenv("CORE_OAUTH_TOKEN_URL", "http://127.0.0.1:5556/token")
	t.Setenv("CORE_OAUTH_USERINFO_URL", "http://127.0.0.1:5556/userinfo")
	t.Setenv("CORE_OAUTH_CLIENT_ID", "autosecrets")
	t.Setenv("CORE_OAUTH_SCOPES", "profile,email,offline_access")
	cfg, err = FromEnv()
	if err != nil || cfg.OAuthConfigurationError() != nil {
		t.Fatalf("valid development OAuth config rejected: %v / %v", err, cfg.OAuthConfigurationError())
	}
	want := []string{"profile", "email"}
	if !reflect.DeepEqual(cfg.OAuthScopes, want) {
		t.Fatalf("scopes = %v, want %v", cfg.OAuthScopes, want)
	}
}

func TestBuildVersionPrefersCORE_VERSION(t *testing.T) {
	t.Setenv("CORE_VERSION", "884f0021718f592f0025d4afcf65c87aa2b1137a")
	if got := BuildVersion("dev"); got != "884f0021718f592f0025d4afcf65c87aa2b1137a" {
		t.Fatalf("BuildVersion() = %q", got)
	}
}

func TestBuildVersionFallsBackWhenUnset(t *testing.T) {
	t.Setenv("CORE_VERSION", "")
	got := BuildVersion("dev")
	if got == "" {
		t.Fatal("BuildVersion() returned empty")
	}
	// Tests run as `go test`, so VCS info is usually present. Accept either
	// the embedded revision or the explicit fallback.
	if got != "dev" && len(got) < 7 {
		t.Fatalf("BuildVersion() = %q, want fallback or a revision", got)
	}
}

func TestOIDCConfigurationRejectsInsecurePublicURLWithoutBreakingFromEnv(t *testing.T) {
	t.Setenv("CORE_PUBLIC_URL", "http://example.com")
	t.Setenv("CORE_OIDC_ISSUER_URL", "https://id.example.com")
	t.Setenv("CORE_OIDC_CLIENT_ID", "autosecrets")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCConfigurationError() == nil {
		t.Fatal("insecure production public URL accepted")
	}
}
