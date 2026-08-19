// Package app is the Core composition root: it wires the store and key
// material into the identity, secrets, and fleet domain handlers and the two
// HTTP surfaces (ADR-0001, ADR-0025). All product behavior is tested through
// these HTTP seams.
package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/fleet"
	"autosecrets.dev/core/internal/identity"
	"autosecrets.dev/core/internal/middleware"
	"autosecrets.dev/core/internal/secrets"
)

const (
	sessionCookie = middleware.SessionCookie
	csrfHeader    = middleware.CSRFHeader
	bootstrapTTL  = 1 * time.Hour
)

type Options struct {
	Version        string
	PublicURL      string
	PublicAgentURL string
	ArtifactDir    string
	TrustedProxy   []*net.IPNet
	CertHeader     string
	CORSOrigins    []string
	// OfflineAfter is the heartbeat threshold after which Core projects a
	// Managed Node as offline (ADR-0015). Defaults to 75 seconds.
	OfflineAfter time.Duration
	// InstallCurlOpts is prepended to the generated Install Command as an
	// environment assignment (e.g. "AUTOSECRETS_CURL_OPTS='-k'"). LAN/dev
	// deployments use it to skip TLS verification of self-signed dev
	// certificates; production leaves it empty.
	InstallCurlOpts  string
	Logger           *log.Logger
	Now              func() time.Time
	OIDCClient       *identity.OIDCClient
	OIDCUnavailable  string
	OAuthClient      *identity.OAuthClient
	OAuthUnavailable string
}

// App is the Core service: store, key material, and both HTTP surfaces.
type App struct {
	store           *database.Store
	mk              *crypto.MasterKey
	ca              *crypto.CA
	signer          *crypto.Signer
	cfg             Options
	managementBase  string
	agentBase       string
	logger          *log.Logger
	now             func() time.Time
	identityHandler *identity.Handler
	secretsHandler  *secrets.Handler
	fleetHandler    *fleet.Handler
}

func New(st *database.Store, mk *crypto.MasterKey, ca *crypto.CA, signer *crypto.Signer,
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
	a := &App{
		store: st, mk: mk, ca: ca, signer: signer, cfg: opts,
		managementBase: managementBase, agentBase: agentBase,
		logger: opts.Logger, now: opts.Now,
	}
	identitySvc := identity.NewService(st, identityAudit{st: st}, mk, opts.Now)
	a.identityHandler = identity.NewHandler(identitySvc, opts.PublicURL, opts.TrustedProxy)
	a.identityHandler.ConfigureOIDC(opts.OIDCClient, opts.OIDCUnavailable)
	a.identityHandler.ConfigureOAuth(opts.OAuthClient, opts.OAuthUnavailable)
	a.secretsHandler = secrets.NewHandler(st, mk, opts.Now)
	a.fleetHandler = fleet.NewHandler(st, mk, ca, signer, opts.Now, fleet.Config{
		PublicAgentURL:  opts.PublicAgentURL,
		ArtifactDir:     opts.ArtifactDir,
		InstallCurlOpts: opts.InstallCurlOpts,
		OfflineAfter:    opts.OfflineAfter,
	}, agentBase)
	return a
}

// identityAudit adapts *database.Store to the identity domain's narrow
// AuditRecorder seam (ADR-0025): domains collaborate through narrow
// interfaces, not by importing another domain's concrete package.
type identityAudit struct{ st *database.Store }

func (a identityAudit) AppendAudit(ctx context.Context, event database.AuditEvent) error {
	return a.st.AppendAudit(ctx, nil, event)
}

// EmitBootstrapCode generates and logs a one-time Bootstrap Code when no
// Administrator exists yet, returning it for the operator (and tests). Only
// the hash is stored.
func (a *App) EmitBootstrapCode(ctx context.Context) (string, error) {
	count, err := a.store.HumanIdentityCount(ctx)
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
	a.identityHandler.Register(mux, a.managementBase, a.requireSession)
	a.secretsHandler.Register(mux, a.managementBase, a.requireSession)
	a.fleetHandler.Register(mux, a.managementBase, a.agentBase, a.requireSession, a.agentAuth)
	mux.Handle("GET "+a.managementBase+"/overview", a.requireSession(http.HandlerFunc(a.handleOverview)))
	mux.Handle("GET "+a.managementBase+"/search", a.requireSession(http.HandlerFunc(a.handleSearch)))
	mux.Handle("GET "+a.managementBase+"/audit-events", a.requireSession(http.HandlerFunc(a.handleListAudit)))
	mux.HandleFunc("GET "+a.agentBase+"/health", a.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
	})
	return a.withRequestLog(middleware.CORS(a.cfg.CORSOrigins)(mux))
}

func (a *App) agentAuth(next http.Handler) http.Handler {
	return middleware.AgentIdentityMiddleware(a.cfg.TrustedProxy, a.cfg.CertHeader, a.ca, a.now)(next)
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

// requireSession guards the session-authenticated management surface.
func (a *App) requireSession(next http.Handler) http.Handler {
	return middleware.RequireSession(a.store, a.now)(next)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	middleware.WriteJSON(w, code, body)
}

// writeError emits the unified error envelope. The machine-readable code is
// part of the API contract (api/openapi.yaml): clients must match on code,
// never on the human-readable message.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	middleware.WriteError(w, status, code, msg)
}
