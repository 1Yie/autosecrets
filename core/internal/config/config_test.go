package config

import (
	"net"
	"reflect"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	for _, key := range []string{"CORE_LISTEN_ADDR", "CORE_MANAGEMENT_BASE", "CORE_AGENT_BASE", "CORE_TRUSTED_PROXY_CIDRS", "CORE_PROXY_CERT_HEADER"} {
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
