package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"autosecrets.dev/core/internal/config"
)

func testConfig(trusted ...string) config.Config {
	cidrs, err := config.ParseCIDRs(trusted)
	if err != nil {
		panic(err)
	}
	return config.Config{
		ManagementBase:    "/api/v1",
		AgentBase:         "/agent/v1",
		TrustedProxyCIDRs: cidrs,
		ProxyCertHeader:   "X-Autosecrets-Client-Cert",
	}
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestManagementHealth(t *testing.T) {
	handler := New(testConfig(), "test-version")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["status"] != "ok" || body["service"] != "core" || body["version"] != "test-version" {
		t.Fatalf("unexpected body: %v", body)
	}
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
	}
}

func TestAgentHealthRejectsMissingIdentity(t *testing.T) {
	handler := New(testConfig("10.0.0.0/8"), "v1")
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/health", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAgentHealthRejectsUntrustedProxy(t *testing.T) {
	handler := New(testConfig("10.0.0.0/8"), "v1")
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/health", nil)
	request.RemoteAddr = "203.0.113.9:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "serial-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAgentHealthFailsClosedWithoutTrustedProxies(t *testing.T) {
	handler := New(testConfig(), "v1")
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/health", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "serial-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (fail closed with no configured proxy)", recorder.Code)
	}
}

func TestAgentHealthAcceptsTrustedProxyIdentity(t *testing.T) {
	handler := New(testConfig("10.0.0.0/8"), "test-version")
	request := httptest.NewRequest(http.MethodGet, "/agent/v1/health", nil)
	request.RemoteAddr = "10.0.0.5:12345"
	request.Header.Set("X-Autosecrets-Client-Cert", "serial-77")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["service"] != "core-agent" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestAgentSerialAvailableInContext(t *testing.T) {
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serial, ok := AgentSerialFromContext(r.Context())
		if !ok || serial != "serial-42" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	wrapped := AgentIdentityMiddleware(testConfig("10.0.0.0/8").TrustedProxyCIDRs, "X-Autosecrets-Client-Cert")(probe)
	probeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	probeRequest.RemoteAddr = "10.0.0.5:12345"
	probeRequest.Header.Set("X-Autosecrets-Client-Cert", "serial-42")
	probeRecorder := httptest.NewRecorder()
	wrapped.ServeHTTP(probeRecorder, probeRequest)
	if probeRecorder.Code != http.StatusOK {
		t.Fatalf("serial not propagated to context: status %d", probeRecorder.Code)
	}
}

func TestUnknownRouteIsJSON404(t *testing.T) {
	handler := New(testConfig(), "v1")
	request := httptest.NewRequest(http.MethodGet, "/nope", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &map[string]any{}); err != nil {
		t.Fatalf("404 body is not JSON: %q", recorder.Body.String())
	}
}

func TestTrustedProxyMatchingUsesCIDR(t *testing.T) {
	cidrs, err := config.ParseCIDRs([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP("10.255.255.255")
	for _, cidr := range cidrs {
		if !cidr.Contains(ip) {
			t.Fatalf("%v should be inside 10.0.0.0/8", ip)
		}
	}
}
