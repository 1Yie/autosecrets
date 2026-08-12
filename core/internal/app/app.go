// Package app wires Core's store and key material into the two HTTP surfaces:
// the session-authenticated management API and the mTLS-authenticated Agent
// API. All product behavior is tested through these HTTP seams.
package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/server"
	"autosecrets.dev/core/internal/store"
)

const (
	sessionCookie = "autosecrets_session"
	csrfHeader    = "X-CSRF-Token"
	defaultTTL    = 30 * 24 * time.Hour
	bootstrapTTL  = 1 * time.Hour
	tokenTTL      = 10 * time.Minute
	certTTL       = 30 * 24 * time.Hour
)

type Options struct {
	Version        string
	PublicAgentURL string
	ArtifactDir    string
	TrustedProxy   []*net.IPNet
	CertHeader     string
	// OfflineAfter is the heartbeat threshold after which Core projects a
	// Managed Node as offline (ADR-0015). Defaults to 75 seconds.
	OfflineAfter time.Duration
	// InstallCurlOpts is prepended to the generated Install Command as an
	// environment assignment (e.g. "AUTOSECRETS_CURL_OPTS='-k'"). LAN/dev
	// deployments use it to skip TLS verification of self-signed dev
	// certificates; production leaves it empty.
	InstallCurlOpts string
	Logger          *log.Logger
	Now             func() time.Time
}

// App is the Core service: store, key material, and both HTTP surfaces.
type App struct {
	store          *store.Store
	mk             *crypto.MasterKey
	ca             *crypto.CA
	signer         *crypto.Signer
	cfg            Options
	managementBase string
	agentBase      string
	logger         *log.Logger
	now            func() time.Time
}

func New(st *store.Store, mk *crypto.MasterKey, ca *crypto.CA, signer *crypto.Signer,
	managementBase, agentBase string, opts Options) *App {
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.OfflineAfter <= 0 {
		opts.OfflineAfter = 75 * time.Second
	}
	return &App{
		store: st, mk: mk, ca: ca, signer: signer, cfg: opts,
		managementBase: managementBase, agentBase: agentBase,
		logger: opts.Logger, now: opts.Now,
	}
}

// EmitBootstrapCode generates and logs a one-time Bootstrap Code when no
// Administrator exists yet, returning it for the operator (and tests). Only
// the hash is stored.
func (a *App) EmitBootstrapCode(ctx context.Context) (string, error) {
	count, err := a.store.AdminCount(ctx)
	if err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}
	code, err := crypto.NewSecret(96)
	if err != nil {
		return "", err
	}
	if err := a.store.SaveBootstrapCode(ctx, crypto.HashToken(code), a.now().Add(bootstrapTTL)); err != nil {
		return "", err
	}
	a.logger.Printf("AUTOSECRETS BOOTSTRAP CODE: %s (valid for %s; paste it into the panel)", code, bootstrapTTL)
	return code, nil
}

// Handler builds the full Core HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// Management surface: session-authenticated.
	mux.HandleFunc("GET "+a.managementBase+"/health", a.handleHealth)
	mux.HandleFunc("POST "+a.managementBase+"/bootstrap", a.handleBootstrap)
	mux.HandleFunc("POST "+a.managementBase+"/auth/mfa-enrollment/verify", a.handleVerifyMFAEnrollment)
	mux.HandleFunc("POST "+a.managementBase+"/auth/mfa-enrollment/confirm", a.handleConfirmMFAEnrollment)
	mux.HandleFunc("POST "+a.managementBase+"/auth/login", a.handleLogin)
	mux.Handle("POST "+a.managementBase+"/auth/logout", a.requireSession(http.HandlerFunc(a.handleLogout)))
	mux.Handle("POST "+a.managementBase+"/auth/renew", a.requireSession(http.HandlerFunc(a.handleSessionRenewal)))
	mux.Handle("POST "+a.managementBase+"/auth/step-up", a.requireSession(http.HandlerFunc(a.handleStepUp)))
	mux.Handle("POST "+a.managementBase+"/auth/password", a.requireSession(http.HandlerFunc(a.handlePasswordChange)))
	mux.HandleFunc("GET "+a.managementBase+"/me", a.handleMe)
	mux.Handle("GET "+a.managementBase+"/applications", a.requireSession(http.HandlerFunc(a.handleListApplications)))
	mux.Handle("POST "+a.managementBase+"/applications", a.requireSession(http.HandlerFunc(a.handleCreateApplication)))
	mux.Handle("GET "+a.managementBase+"/applications/{appID}", a.requireSession(http.HandlerFunc(a.handleGetApplication)))
	mux.Handle("POST "+a.managementBase+"/applications/{appID}/environments", a.requireSession(http.HandlerFunc(a.handleCreateEnvironment)))
	mux.Handle("GET "+a.managementBase+"/applications/{appID}/environments/{envID}/secrets", a.requireSession(http.HandlerFunc(a.handleListSecrets)))
	mux.Handle("POST "+a.managementBase+"/applications/{appID}/environments/{envID}/secrets", a.requireSession(http.HandlerFunc(a.handleCreateSecret)))
	mux.Handle("POST "+a.managementBase+"/secrets/{secretID}/versions", a.requireSession(http.HandlerFunc(a.handleCreateSecretVersion)))
	mux.Handle("POST "+a.managementBase+"/secrets/{secretID}/rotate", a.requireSession(http.HandlerFunc(a.handleRotateSecret)))
	mux.Handle("PUT "+a.managementBase+"/secrets/{secretID}/binding", a.requireSession(http.HandlerFunc(a.handleUpdateBinding)))
	mux.Handle("GET "+a.managementBase+"/applications/{appID}/environments/{envID}/draft", a.requireSession(http.HandlerFunc(a.handleGetDraft)))
	mux.Handle("PUT "+a.managementBase+"/applications/{appID}/environments/{envID}/draft", a.requireSession(http.HandlerFunc(a.handleUpdateDraft)))
	mux.Handle("POST "+a.managementBase+"/applications/{appID}/environments/{envID}/publish", a.requireSession(http.HandlerFunc(a.handlePublish)))
	mux.Handle("POST "+a.managementBase+"/applications/{appID}/environments/{envID}/rollback", a.requireSession(http.HandlerFunc(a.handleRollback)))
	mux.Handle("GET "+a.managementBase+"/applications/{appID}/environments/{envID}/revisions", a.requireSession(http.HandlerFunc(a.handleListRevisions)))
	mux.Handle("GET "+a.managementBase+"/node-groups", a.requireSession(http.HandlerFunc(a.handleListNodeGroups)))
	mux.Handle("POST "+a.managementBase+"/node-groups", a.requireSession(http.HandlerFunc(a.handleCreateNodeGroup)))
	mux.Handle("POST "+a.managementBase+"/node-groups/{groupID}/nodes", a.requireSession(http.HandlerFunc(a.handleAddGroupMember)))
	mux.Handle("DELETE "+a.managementBase+"/node-groups/{groupID}/nodes/{nodeID}", a.requireSession(http.HandlerFunc(a.handleRemoveGroupMember)))
	mux.Handle("GET "+a.managementBase+"/assignments", a.requireSession(http.HandlerFunc(a.handleListAssignments)))
	mux.Handle("POST "+a.managementBase+"/assignments", a.requireSession(http.HandlerFunc(a.handleCreateAssignment)))
	mux.Handle("GET "+a.managementBase+"/assignments/{assignmentID}", a.requireSession(http.HandlerFunc(a.handleUnassignmentState)))
	mux.Handle("POST "+a.managementBase+"/assignments/{assignmentID}/unassign", a.requireSession(http.HandlerFunc(a.handleUnassign)))
	mux.Handle("POST "+a.managementBase+"/assignments/{assignmentID}/abandon-cleanup", a.requireSession(http.HandlerFunc(a.handleAbandonCleanup)))
	mux.Handle("GET "+a.managementBase+"/applications/{appID}/environments/{envID}/activation-policy", a.requireSession(http.HandlerFunc(a.handleGetActivationPolicy)))
	mux.Handle("PUT "+a.managementBase+"/applications/{appID}/environments/{envID}/activation-policy", a.requireSession(http.HandlerFunc(a.handlePutActivationPolicy)))
	mux.Handle("GET "+a.managementBase+"/nodes", a.requireSession(http.HandlerFunc(a.handleListNodes)))
	mux.Handle("GET "+a.managementBase+"/overview", a.requireSession(http.HandlerFunc(a.handleOverview)))
	mux.Handle("GET "+a.managementBase+"/search", a.requireSession(http.HandlerFunc(a.handleSearch)))
	mux.Handle("POST "+a.managementBase+"/nodes/install-command", a.requireSession(http.HandlerFunc(a.handleInstallCommand)))
	mux.Handle("GET "+a.managementBase+"/audit-events", a.requireSession(http.HandlerFunc(a.handleListAudit)))

	// Agent surface: public pre-certificate routes plus mTLS-protected routes.
	mux.HandleFunc("GET "+a.agentBase+"/health", a.handleHealth)
	mux.HandleFunc("GET "+a.agentBase+"/install.sh", a.handleInstallScript)
	mux.HandleFunc("GET "+a.agentBase+"/ca.pem", a.handleCAPEM)
	mux.HandleFunc("GET "+a.agentBase+"/artifacts/{name}", a.handleArtifact)
	mux.HandleFunc("POST "+a.agentBase+"/enroll", a.handleEnroll)
	agentAuth := server.AgentIdentityMiddleware(a.cfg.TrustedProxy, a.cfg.CertHeader)
	mux.Handle("GET "+a.agentBase+"/desired", agentAuth(http.HandlerFunc(a.handleDesired)))
	mux.Handle("POST "+a.agentBase+"/nodes/{nodeID}/reports", agentAuth(http.HandlerFunc(a.handleReport)))
	mux.Handle("POST "+a.agentBase+"/nodes/{nodeID}/cleanup", agentAuth(http.HandlerFunc(a.handleCleanupReport)))
	mux.Handle("POST "+a.agentBase+"/nodes/{nodeID}/heartbeat", agentAuth(http.HandlerFunc(a.handleHeartbeat)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
	})
	return a.withRequestLog(mux)
}

func (a *App) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := a.now()
		next.ServeHTTP(w, r)
		a.logger.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok", "service": "core", "version": a.cfg.Version,
	})
}

// --- middleware -----------------------------------------------------------

type sessionKey struct{}

func sessionFrom(r *http.Request) *store.SessionRow {
	s, _ := r.Context().Value(sessionKey{}).(*store.SessionRow)
	return s
}

func (a *App) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.sessionFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			provided := r.Header.Get(csrfHeader)
			providedHash := crypto.HashToken(provided)
			if subtle.ConstantTimeCompare([]byte(providedHash), []byte(crypto.HashToken(session.CSRFToken))) != 1 {
				writeError(w, http.StatusForbidden, "forbidden", "invalid CSRF token")
				return
			}
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sessionFromRequest resolves the session cookie without requiring it: /me
// needs the bootstrap state anonymously while still recognizing a session.
func (a *App) sessionFromRequest(r *http.Request) (*store.SessionRow, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	session, err := a.store.SessionByID(r.Context(), crypto.HashToken(cookie.Value), a.now())
	if err != nil {
		return nil, false
	}
	return session, true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError emits the unified error envelope. The machine-readable code is
// part of the API contract (api/openapi.yaml): clients must match on code,
// never on the human-readable message.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

// Error codes exposed by the management API. Extend the enum in
// api/openapi.yaml together with this list.
const (
	codeBadRequest   = "bad_request"
	codeUnauthorized = "unauthorized"
	codeForbidden    = "forbidden"
	codeNotFound     = "not_found"
	codeConflict     = "conflict"
	codeDuplicate    = "duplicate"
	codeInternal     = "internal"
	codeUnavailable  = "unavailable"
	codeStepUp       = "step_up_required"
)

func actorFrom(r *http.Request) string {
	if s := sessionFrom(r); s != nil {
		return "admin:" + s.Username
	}
	if serial, ok := server.AgentSerialFromContext(r.Context()); ok {
		return "node:" + serial
	}
	return "anonymous"
}

func (a *App) correlationID(r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	return fmt.Sprintf("%d", a.now().UnixNano())
}

var (
	errNotFound     = store.ErrNotFound
	errDuplicate    = store.ErrDuplicate
	errConflict     = store.ErrConflict
	errBadPayload   = store.ErrBadPayload
	errBadSelection = store.ErrBadPayload
	errNoSecrets    = store.ErrNoSecrets
)
