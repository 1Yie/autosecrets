package server

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type agentSerialKey struct{}

// AgentSerialFromContext returns the certificate serial forwarded by the
// trusted reverse proxy, if the request passed the middleware.
func AgentSerialFromContext(ctx context.Context) (string, bool) {
	serial, ok := ctx.Value(agentSerialKey{}).(string)
	return serial, ok && serial != ""
}

// AgentIdentityMiddleware enforces the Agent trust boundary. A request is only
// admitted when it arrives from a configured trusted proxy CIDR AND carries the
// forwarded certificate header. Every other request gets 403: an Agent API
// route must never be reachable without a proven node identity.
func AgentIdentityMiddleware(trusted []*net.IPNet, certHeader string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remoteIP, err := remoteIP(r.RemoteAddr)
			if err != nil || !isTrusted(remoteIP, trusted) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "untrusted proxy"})
				return
			}
			serial := strings.TrimSpace(r.Header.Get(certHeader))
			if serial == "" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing client certificate identity"})
				return
			}
			ctx := context.WithValue(r.Context(), agentSerialKey{}, serial)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func remoteIP(remoteAddr string) (net.IP, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Requests without a port (rare in production) are still checkable.
		if ip := net.ParseIP(remoteAddr); ip != nil {
			return ip, nil
		}
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, &net.AddrError{Err: "invalid IP", Addr: host}
	}
	return ip, nil
}

func isTrusted(ip net.IP, trusted []*net.IPNet) bool {
	if len(trusted) == 0 {
		return false
	}
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
