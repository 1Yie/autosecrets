package identity

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidReturnToAllowsOnlyAuthenticatedApplicationPaths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  string
	}{
		{"/dashboard/security", "/dashboard/security"},
		{"/dashboard/apps?selected=one", "/dashboard/apps?selected=one"},
		{"", "/dashboard/overview"},
		{"https://attacker.example", "/dashboard/overview"},
		{"//attacker.example", "/dashboard/overview"},
		{"/\\attacker.example", "/dashboard/overview"},
		{"/auth/login", "/dashboard/overview"},
		{"/dashboard/../auth/login", "/dashboard/overview"},
		{"/dashboard/security\r\nLocation: https://attacker.example", "/dashboard/overview"},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := validReturnTo(test.input); got != test.want {
				t.Fatalf("validReturnTo(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestAuthenticationCookiesAreSecureForPublicHTTPSAndTrustedTLSProxy(t *testing.T) {
	_, trusted, _ := net.ParseCIDR("127.0.0.0/8")
	for _, test := range []struct {
		name      string
		handler   *Handler
		forwarded string
		want      bool
	}{
		{name: "public HTTPS origin", handler: &Handler{publicURL: "https://core.example"}, want: true},
		{name: "trusted TLS proxy", handler: &Handler{trustedProxies: []*net.IPNet{trusted}}, forwarded: "https", want: true},
		{name: "untrusted forwarded header", handler: &Handler{}, forwarded: "https", want: false},
		{name: "direct TLS", handler: &Handler{}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://core.test/api/v1/auth/login", nil)
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("X-Forwarded-Proto", test.forwarded)
			if test.name == "direct TLS" {
				req.TLS = &tls.ConnectionState{}
			}
			response := httptest.NewRecorder()
			test.handler.setSessionCookie(response, req, &Session{ID: "session", ExpiresAt: time.Now().Add(time.Hour)})
			got := strings.Contains(response.Header().Get("Set-Cookie"), "; Secure")
			if got != test.want {
				t.Fatalf("Secure=%v, want %v: %s", got, test.want, response.Header().Get("Set-Cookie"))
			}
		})
	}
}
