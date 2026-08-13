// Package httpapi holds the shared HTTP concerns of the Core service: the
// unified JSON/error envelope, session authentication middleware, and the
// Agent trust-boundary middleware. Domain packages (identity, secrets,
// fleet) depend on this package for request/response plumbing; it must not
// import any domain package.
package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
)

// --- JSON envelope --------------------------------------------------------

func WriteJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError emits the unified error envelope. The machine-readable code is
// part of the API contract (api/openapi.yaml): clients must match on code,
// never on the human-readable message.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg, "code": code})
}

// Error codes exposed by the management API. Extend the enum in
// api/openapi.yaml together with this list.
const (
	CodeBadRequest            = "bad_request"
	CodeUnauthorized          = "unauthorized"
	CodeForbidden             = "forbidden"
	CodeNotFound              = "not_found"
	CodeConflict              = "conflict"
	CodeDuplicate             = "duplicate"
	CodeInternal              = "internal"
	CodeUnavailable           = "unavailable"
	CodeMFAEnrollmentRequired = "mfa_enrollment_required"
	CodeSecondFactorRequired  = "second_factor_required"
	CodeChallengeExpired      = "challenge_expired"
	CodeRateLimited           = "rate_limited"
)

// --- Session authentication -----------------------------------------------

const (
	SessionCookie      = "autosecrets_session"
	CSRFHeader         = "X-CSRF-Token"
	SessionAbsoluteTTL = 12 * time.Hour
	SessionIdleTTL     = 30 * time.Minute
)

type sessionKey struct{}

// SessionStore is the subset of the store the session middleware needs.
// *database.Store satisfies it directly.
type SessionStore interface {
	SessionByID(ctx context.Context, idHash string, now time.Time) (*database.SessionRow, error)
	TouchSessionActivity(ctx context.Context, idHash string, now, idleExpiresAt time.Time) error
}

func SessionFrom(r *http.Request) *database.SessionRow {
	s, _ := r.Context().Value(sessionKey{}).(*database.SessionRow)
	return s
}

// SessionFromRequest resolves the session cookie without requiring it: /me
// needs the bootstrap state anonymously while still recognizing a session.
func SessionFromRequest(st SessionStore, now func() time.Time, r *http.Request) (*database.SessionRow, bool) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	session, err := st.SessionByID(r.Context(), crypto.HashToken(cookie.Value), now())
	if err != nil {
		return nil, false
	}
	return session, true
}

// RequireSession guards the session-authenticated management surface:
// session resolution, CSRF on mutations, and idle-window sliding.
func RequireSession(st SessionStore, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromRequest(st, now, r)
			if !ok {
				WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "not authenticated")
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				providedHash := crypto.HashToken(r.Header.Get(CSRFHeader))
				if subtle.ConstantTimeCompare([]byte(providedHash), []byte(crypto.HashToken(session.CSRFToken))) != 1 {
					WriteError(w, http.StatusForbidden, CodeForbidden, "invalid CSRF token")
					return
				}
				// Mutations are deliberate interactions: slide the idle window.
				now := now()
				_ = st.TouchSessionActivity(r.Context(), session.SessionIDHash, now, now.Add(SessionIdleTTL))
			}
			ctx := context.WithValue(r.Context(), sessionKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CorrelationID(now func() time.Time, r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	return fmt.Sprintf("%d", now().UnixNano())
}

// ActorFrom reports the authenticated actor for an Audit Event: a session
// Administrator, an Agent node by certificate serial, or anonymous.
func ActorFrom(r *http.Request) string {
	if s := SessionFrom(r); s != nil {
		return "admin:" + s.Username
	}
	if serial, ok := AgentSerialFromContext(r.Context()); ok {
		return "node:" + serial
	}
	return "anonymous"
}

// --- Agent trust boundary -------------------------------------------------

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
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": "untrusted proxy"})
				return
			}
			serial := strings.TrimSpace(r.Header.Get(certHeader))
			if serial == "" {
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": "missing client certificate identity"})
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
