package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"autosecrets.dev/core/internal/config"
)

func testConfig(cidrs ...string) config.Config {
	parsed, _ := config.ParseCIDRs(cidrs)
	return config.Config{
		ManagementBase:    "/api/v1",
		AgentBase:         "/agent/v1",
		TrustedProxyCIDRs: parsed,
		ProxyCertHeader:   "X-Autosecrets-Client-Cert",
	}
}

func TestAgentMiddlewareRejectsMissingIdentity(t *testing.T) {
	handler := AgentIdentityMiddleware(testConfig("10.0.0.0/8").TrustedProxyCIDRs, "X-Autosecrets-Client-Cert")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/anything", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAgentMiddlewareRejectsUntrustedProxy(t *testing.T) {
	handler := AgentIdentityMiddleware(testConfig("10.0.0.0/8").TrustedProxyCIDRs, "X-Autosecrets-Client-Cert")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/anything", nil)
	request.RemoteAddr = "192.168.1.9:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "deadbeef")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAgentMiddlewareFailsClosedWithoutTrustedProxies(t *testing.T) {
	handler := AgentIdentityMiddleware(testConfig().TrustedProxyCIDRs, "X-Autosecrets-Client-Cert")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/anything", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "deadbeef")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAgentMiddlewareAcceptsTrustedProxyIdentity(t *testing.T) {
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := AgentSerialFromContext(r.Context()); !ok {
			t.Fatal("serial missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := AgentIdentityMiddleware(testConfig("10.0.0.0/8").TrustedProxyCIDRs, "X-Autosecrets-Client-Cert")(probe)
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/anything", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "c0ffee")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestAgentSerialAvailableInContext(t *testing.T) {
	ctx := context.Background()
	if _, ok := AgentSerialFromContext(ctx); ok {
		t.Fatal("empty context must not carry a serial")
	}
}

func TestTrustedProxyMatchingUsesCIDR(t *testing.T) {
	cfg := testConfig("10.0.0.0/8")
	if len(cfg.TrustedProxyCIDRs) != 1 {
		t.Fatalf("want 1 CIDR, got %d", len(cfg.TrustedProxyCIDRs))
	}
}
