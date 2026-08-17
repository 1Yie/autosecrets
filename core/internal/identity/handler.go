package identity

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
)

// Handler is the HTTP surface of the identity domain: it decodes and
// validates requests, delegates to the Service, and maps domain/store errors
// onto the unified error envelope. It owns no business rules.
type Handler struct {
	svc              *Service
	base             string
	oidc             *OIDCClient
	oidcUnavailable  string
	oauth            *OAuthClient
	oauthUnavailable string
	loginFailures    authFailureLimiter
	factorFailures   authFailureLimiter
	publicURL        string
	trustedProxies   []*net.IPNet
}

const (
	loginChallengeCookie = "autosecrets_login_challenge"
	oidcStateCookie      = "autosecrets_oidc_state"
	oauthStateCookie     = "autosecrets_oauth_state"
)

func NewHandler(svc *Service, publicURL string, trustedProxies []*net.IPNet) *Handler {
	return &Handler{svc: svc, publicURL: publicURL, trustedProxies: trustedProxies}
}

func (h *Handler) ConfigureOIDC(client *OIDCClient, unavailable string) {
	h.oidc = client
	h.oidcUnavailable = unavailable
}

func (h *Handler) ConfigureOAuth(client *OAuthClient, unavailable string) {
	h.oauth = client
	h.oauthUnavailable = unavailable
}

// Register mounts the identity routes: the public bootstrap/login/MFA flow
// plus the session-authenticated logout, renewal, and password change.
func (h *Handler) Register(mux *http.ServeMux, base string, requireSession func(http.Handler) http.Handler) {
	h.base = base
	mux.HandleFunc("POST "+base+"/bootstrap", h.Bootstrap)
	mux.Handle("POST "+base+"/auth/totp/enrollment", requireSession(http.HandlerFunc(h.StartTOTPEnrollment)))
	mux.Handle("POST "+base+"/auth/mfa-enrollment/verify", requireSession(http.HandlerFunc(h.VerifyMFAEnrollment)))
	mux.Handle("POST "+base+"/auth/mfa-enrollment/confirm", requireSession(http.HandlerFunc(h.ConfirmMFAEnrollment)))
	mux.HandleFunc("POST "+base+"/auth/login", h.Login)
	mux.HandleFunc("POST "+base+"/auth/login/second-factor", h.LoginSecondFactor)
	mux.HandleFunc("GET "+base+"/auth/oidc/status", h.OIDCStatus)
	mux.HandleFunc("GET "+base+"/auth/oidc/login", h.StartOIDCLogin)
	mux.HandleFunc("GET "+base+"/auth/oidc/callback", h.OIDCCallback)
	mux.HandleFunc("GET "+base+"/auth/oauth/status", h.OAuthStatus)
	mux.HandleFunc("GET "+base+"/auth/oauth/login", h.StartOAuthLogin)
	mux.HandleFunc("GET "+base+"/auth/oauth/callback", h.OAuthCallback)
	mux.Handle("GET "+base+"/auth/security", requireSession(http.HandlerFunc(h.SecurityStatus)))
	mux.Handle("POST "+base+"/auth/oidc/binding", requireSession(http.HandlerFunc(h.StartOIDCBinding)))
	mux.Handle("DELETE "+base+"/auth/oidc/binding", requireSession(http.HandlerFunc(h.UnbindOIDC)))
	mux.Handle("POST "+base+"/auth/oauth/binding", requireSession(http.HandlerFunc(h.StartOAuthBinding)))
	mux.Handle("DELETE "+base+"/auth/oauth/binding", requireSession(http.HandlerFunc(h.UnbindOAuth)))
	mux.Handle("DELETE "+base+"/auth/totp", requireSession(http.HandlerFunc(h.DisableTOTP)))
	mux.Handle("POST "+base+"/auth/logout", requireSession(http.HandlerFunc(h.Logout)))
	mux.Handle("POST "+base+"/auth/renew", requireSession(http.HandlerFunc(h.SessionRenewal)))
	mux.Handle("POST "+base+"/auth/step-up", requireSession(http.HandlerFunc(h.StepUp)))
	mux.Handle("POST "+base+"/auth/username", requireSession(http.HandlerFunc(h.UsernameChange)))
	mux.Handle("POST "+base+"/auth/password", requireSession(http.HandlerFunc(h.PasswordChange)))
	mux.HandleFunc("GET "+base+"/me", h.Me)
}

func (h *Handler) setOIDCState(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookie, Value: value, Path: h.base + "/auth/oidc/callback",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secureCookies(r), MaxAge: maxAge,
	})
}

func (h *Handler) setOAuthState(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: value, Path: h.base + "/auth/oauth/callback",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secureCookies(r), MaxAge: maxAge,
	})
}

func (h *Handler) oidcAvailable() bool { return h.oidc != nil }

func (h *Handler) oauthAvailable() bool { return h.oauth != nil }

func (h *Handler) binding(ctx context.Context) (*database.ExternalIdentityBinding, bool) {
	binding, err := h.svc.repo.ExternalIdentityBinding(ctx)
	return binding, err == nil
}

func (h *Handler) oauthBinding(ctx context.Context) (*database.ExternalIdentityBinding, bool) {
	binding, err := h.svc.repo.OAuthIdentityBinding(ctx)
	return binding, err == nil
}

func publicProviderStatus(available, bound bool) map[string]bool {
	return map[string]bool{
		"available": available, "bound": bound, "login_available": available && bound,
	}
}

func securityProviderStatus(available, bound bool, unavailable string, binding *database.ExternalIdentityBinding) map[string]any {
	status := map[string]any{"available": available, "bound": bound}
	if unavailable != "" {
		status["configuration_error"] = unavailable
	}
	if bound && binding != nil {
		status["issuer"] = binding.Issuer
		status["display_name"] = binding.DisplayName
	}
	return status
}

func (h *Handler) OIDCStatus(w http.ResponseWriter, r *http.Request) {
	_, oidcBound := h.binding(r.Context())
	_, oauthBound := h.oauthBinding(r.Context())
	oidc := publicProviderStatus(h.oidcAvailable(), oidcBound)
	oauth := publicProviderStatus(h.oauthAvailable(), oauthBound)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"available": oidc["available"], "bound": oidc["bound"],
		"login_available": oidc["login_available"],
		"oidc":            oidc,
		"oauth":           oauth,
	})
}

func (h *Handler) OAuthStatus(w http.ResponseWriter, r *http.Request) {
	_, bound := h.oauthBinding(r.Context())
	middleware.WriteJSON(w, http.StatusOK, publicProviderStatus(h.oauthAvailable(), bound))
}

func (h *Handler) SecurityStatus(w http.ResponseWriter, r *http.Request) {
	organization, err := h.svc.repo.Organization(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		return
	}
	binding, bound := h.binding(r.Context())
	oauthBinding, oauthBound := h.oauthBinding(r.Context())
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"totp_login_required": organization.TOTPLoginRequired,
		"oidc":                securityProviderStatus(h.oidcAvailable(), bound, h.oidcUnavailable, binding),
		"oauth":               securityProviderStatus(h.oauthAvailable(), oauthBound, h.oauthUnavailable, oauthBinding),
	})
}

func (h *Handler) StartOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !h.oidcAvailable() {
		middleware.WriteError(w, http.StatusServiceUnavailable, middleware.CodeUnavailable, "OIDC login unavailable")
		return
	}
	start, err := h.svc.StartOIDCLogin(h.withContext(r), validReturnTo(r.URL.Query().Get("return_to")))
	if err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login unavailable")
		return
	}
	h.setOIDCState(w, r, start.State, int(oidcTransactionTTL/time.Second))
	http.Redirect(w, r, h.oidc.AuthorizationURL(h.base, start.State, start.Nonce, start.Verifier), http.StatusFound)
}

func (h *Handler) StartOIDCBinding(w http.ResponseWriter, r *http.Request) {
	if !h.oidcAvailable() {
		middleware.WriteError(w, http.StatusServiceUnavailable, middleware.CodeUnavailable, "OIDC unavailable")
		return
	}
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
		ReturnTo string `json:"return_to"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid binding request")
		return
	}
	start, err := h.svc.StartOIDCBinding(h.withContext(r), session.AdminID, body.Password, body.TOTPCode, validReturnTo(body.ReturnTo))
	if err != nil {
		if errors.Is(err, database.ErrConflict) {
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "external identity is already bound")
		} else {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		}
		return
	}
	h.setOIDCState(w, r, start.State, int(oidcTransactionTTL/time.Second))
	middleware.WriteJSON(w, http.StatusOK, map[string]string{
		"authorization_url": h.oidc.AuthorizationURL(h.base, start.State, start.Nonce, start.Verifier),
	})
}

func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	h.setOIDCState(w, r, "", -1)
	if !h.oidcAvailable() || r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		h.svc.AuditOIDCCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		h.svc.AuditOIDCCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	transaction, err := h.svc.ConsumeOIDCTransaction(ctx, state)
	if err != nil {
		h.svc.AuditOIDCCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	identity, err := h.oidc.ExchangeAndValidate(r.Context(), h.base, r.URL.Query().Get("code"), transaction.PKCEVerifier, transaction.Nonce, h.svc.now())
	if err != nil {
		h.svc.AuditOIDCCallbackDenied(ctx, transaction.Purpose, transaction.MemberID)
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	if transaction.Purpose == "bind" {
		session, ok := h.sessionFromRequest(r)
		if !ok || session.AdminID != transaction.MemberID {
			h.svc.AuditOIDCCallbackDenied(ctx, transaction.Purpose, transaction.MemberID)
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
		if h.svc.CompleteOIDCBinding(ctx, transaction, identity, session.SessionIDHash) != nil {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
	} else {
		session, err := h.svc.CompleteOIDCLogin(ctx, transaction, identity)
		if err != nil {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
		h.setSessionCookie(w, r, session)
	}
	http.Redirect(w, r, transaction.ReturnTo, http.StatusFound)
}

func (h *Handler) UnbindOIDC(w http.ResponseWriter, r *http.Request) {
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid unbind request")
		return
	}
	err := h.svc.UnbindOIDC(h.withContext(r), session.AdminID, session.SessionIDHash, body.Password, body.TOTPCode)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, middleware.CodeNotFound, "external identity binding not found")
		} else {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]bool{"bound": false})
}

func (h *Handler) StartOAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !h.oauthAvailable() {
		middleware.WriteError(w, http.StatusServiceUnavailable, middleware.CodeUnavailable, "OAuth login unavailable")
		return
	}
	start, err := h.svc.StartOAuthLogin(h.withContext(r), validReturnTo(r.URL.Query().Get("return_to")))
	if err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login unavailable")
		return
	}
	h.setOAuthState(w, r, start.State, int(oidcTransactionTTL/time.Second))
	http.Redirect(w, r, h.oauth.AuthorizationURL(h.base, start.State, start.Verifier), http.StatusFound)
}

func (h *Handler) StartOAuthBinding(w http.ResponseWriter, r *http.Request) {
	if !h.oauthAvailable() {
		middleware.WriteError(w, http.StatusServiceUnavailable, middleware.CodeUnavailable, "OAuth unavailable")
		return
	}
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
		ReturnTo string `json:"return_to"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid binding request")
		return
	}
	start, err := h.svc.StartOAuthBinding(h.withContext(r), session.AdminID, body.Password, body.TOTPCode, validReturnTo(body.ReturnTo))
	if err != nil {
		if errors.Is(err, database.ErrConflict) {
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "external identity is already bound")
		} else {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		}
		return
	}
	h.setOAuthState(w, r, start.State, int(oidcTransactionTTL/time.Second))
	middleware.WriteJSON(w, http.StatusOK, map[string]string{
		"authorization_url": h.oauth.AuthorizationURL(h.base, start.State, start.Verifier),
	})
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	h.setOAuthState(w, r, "", -1)
	if !h.oauthAvailable() || r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		h.svc.AuditOAuthCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		h.svc.AuditOAuthCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	transaction, err := h.svc.ConsumeOIDCTransaction(ctx, state)
	if err != nil {
		h.svc.AuditOAuthCallbackDenied(ctx, "", "")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	identity, err := h.oauth.ExchangeAndIdentify(r.Context(), h.base, r.URL.Query().Get("code"), transaction.PKCEVerifier)
	if err != nil {
		h.svc.AuditOAuthCallbackDenied(ctx, transaction.Purpose, transaction.MemberID)
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
		return
	}
	if transaction.Purpose == "bind" {
		session, ok := h.sessionFromRequest(r)
		if !ok || session.AdminID != transaction.MemberID {
			h.svc.AuditOAuthCallbackDenied(ctx, transaction.Purpose, transaction.MemberID)
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
		if h.svc.CompleteOAuthBinding(ctx, transaction, identity, session.SessionIDHash) != nil {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
	} else {
		session, err := h.svc.CompleteOAuthLogin(ctx, transaction, identity)
		if err != nil {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "external login failed")
			return
		}
		h.setSessionCookie(w, r, session)
	}
	http.Redirect(w, r, transaction.ReturnTo, http.StatusFound)
}

func (h *Handler) UnbindOAuth(w http.ResponseWriter, r *http.Request) {
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid unbind request")
		return
	}
	err := h.svc.UnbindOAuth(h.withContext(r), session.AdminID, session.SessionIDHash, body.Password, body.TOTPCode)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, middleware.CodeNotFound, "external identity binding not found")
		} else {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]bool{"bound": false})
}

func validReturnTo(value string) string {
	const fallback = "/dashboard/overview"
	if value == "" || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n") {
		return fallback
	}
	destination, err := url.ParseRequestURI(value)
	if err != nil || destination.IsAbs() || destination.Host != "" {
		return fallback
	}
	if path.Clean(destination.Path) != destination.Path {
		return fallback
	}
	if destination.Path != "/dashboard" && !strings.HasPrefix(destination.Path, "/dashboard/") {
		return "/dashboard/overview"
	}
	return value
}

func (h *Handler) withContext(r *http.Request) context.Context {
	return WithCorrelationID(r.Context(), middleware.CorrelationID(h.svc.now, r))
}

func (h *Handler) writeSession(w http.ResponseWriter, r *http.Request, s *Session) {
	h.writeSessionStatus(w, r, http.StatusOK, s)
}

func (h *Handler) writeSessionStatus(w http.ResponseWriter, r *http.Request, status int, s *Session) {
	h.setSessionCookie(w, r, s)
	middleware.WriteJSON(w, status, map[string]string{
		"csrf_token": s.CSRFToken, "username": s.Username, "id": s.MemberID,
		"role": s.Role, "expires_at": timeString(s.ExpiresAt),
	})
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, r *http.Request, s *Session) {
	http.SetCookie(w, &http.Cookie{
		Name: middleware.SessionCookie, Value: s.ID, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.secureCookies(r), Expires: s.ExpiresAt,
	})
}

func (h *Handler) sourceHash(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return crypto.HashToken(host + "\n" + r.UserAgent())
}

func (h *Handler) setLoginChallenge(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: loginChallengeCookie, Value: value, Path: h.base + "/auth/login",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secureCookies(r),
		MaxAge: maxAge,
	})
}

func (h *Handler) secureCookies(r *http.Request) bool {
	if r.TLS != nil || strings.HasPrefix(h.publicURL, "https://") {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(host)
	if remote == nil || !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		return false
	}
	for _, network := range h.trustedProxies {
		if network.Contains(remote) {
			return true
		}
	}
	return false
}

// sessionFromRequest resolves the session cookie without requiring it: /me
// needs the bootstrap state anonymously while still recognizing a session.
func (h *Handler) sessionFromRequest(r *http.Request) (*database.SessionRow, bool) {
	cookie, err := r.Cookie(middleware.SessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	session, err := h.svc.repo.SessionByID(r.Context(), crypto.HashToken(cookie.Value), h.svc.now())
	if err != nil {
		return nil, false
	}
	return session, true
}

type bootstrapRequest struct {
	Code             string `json:"code"`
	OrganizationName string `json:"organization_name"`
	Username         string `json:"username"`
	Password         string `json:"password"`
}

func (h *Handler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	var body bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.OrganizationName) == "" {
		body.OrganizationName = "AutoSecrets"
	}
	out, err := h.svc.Bootstrap(ctx, body.Code, body.OrganizationName, body.Username, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid bootstrap enrollment")
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusForbidden, middleware.CodeForbidden, "invalid or expired bootstrap code")
		case errors.Is(err, database.ErrConflict):
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "already bootstrapped")
		case errors.Is(err, database.ErrDuplicate):
			middleware.WriteError(w, http.StatusConflict, middleware.CodeDuplicate, "username already taken")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	h.setSessionCookie(w, r, out.Session)
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"id": out.ID, "username": out.Username, "status": out.Status,
		"csrf_token": out.Session.CSRFToken, "role": out.Session.Role,
		"expires_at": timeString(out.Session.ExpiresAt),
	})
}

func (h *Handler) StartTOTPEnrollment(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid TOTP enrollment")
		return
	}
	out, err := h.svc.StartTOTPEnrollment(ctx, session.AdminID, body.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		} else if errors.Is(err, ErrAlreadyEnrolled) || errors.Is(err, database.ErrConflict) {
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "TOTP is already enabled")
		} else {
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"username": out.Username, "enrollment_token": out.EnrollmentToken, "totp_uri": out.TOTPURI,
	})
}

func (h *Handler) VerifyMFAEnrollment(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	var body struct {
		EnrollmentToken string `json:"enrollment_token"`
		TOTPCode        string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EnrollmentToken == "" {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid MFA enrollment")
		return
	}
	out, err := h.svc.VerifyMFAEnrollment(ctx, body.EnrollmentToken, body.TOTPCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid authentication code")
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusForbidden, middleware.CodeForbidden, "invalid or expired MFA enrollment")
		case errors.Is(err, ErrEnrollmentNotPending):
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "MFA enrollment is no longer pending")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"confirmation_token": out.ConfirmationToken,
		"recovery_codes":     out.RecoveryCodes,
	})
}

func (h *Handler) ConfirmMFAEnrollment(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		ConfirmationToken string `json:"confirmation_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ConfirmationToken == "" || session == nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid recovery confirmation")
		return
	}
	member, err := h.svc.ConfirmMFAEnrollment(ctx, session.AdminID, session.SessionIDHash, body.ConfirmationToken)
	if err != nil {
		if errors.Is(err, ErrEnrollmentNotPending) {
			middleware.WriteError(w, http.StatusConflict, middleware.CodeConflict, "MFA enrollment is no longer pending")
		} else {
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"id": member.ID, "username": member.Username, "status": member.Status})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	limitKey := h.clientIP(r)
	if limited, retryAfter := h.loginFailures.limited(limitKey, h.svc.now()); limited {
		h.svc.auditEvent(ctx, "authentication", "auth.login", "", "rate_limited")
		h.writeRateLimited(w, retryAfter)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.svc.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid JSON")
		return
	}
	out, err := h.svc.Login(ctx, body.Username, body.Password, h.sourceHash(r))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			h.loginFailures.failed(limitKey, h.svc.now())
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	h.loginFailures.succeeded(limitKey)
	if out.Session != nil {
		h.writeSession(w, r, out.Session)
		return
	}
	h.setLoginChallenge(w, r, out.ChallengeToken, int(loginChallengeTTL.Seconds()))
	middleware.WriteJSON(w, http.StatusAccepted, map[string]string{
		"status": "second_factor_required", "code": middleware.CodeSecondFactorRequired,
	})
}

func (h *Handler) LoginSecondFactor(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	limitKey := h.clientIP(r)
	if limited, retryAfter := h.factorFailures.limited(limitKey, h.svc.now()); limited {
		h.svc.auditEvent(ctx, "authentication", "auth.second_factor", "", "rate_limited")
		h.setLoginChallenge(w, r, "", -1)
		h.writeRateLimited(w, retryAfter)
		return
	}
	challenge, err := r.Cookie(loginChallengeCookie)
	if err != nil || challenge.Value == "" {
		h.svc.auditEvent(ctx, "authentication", "auth.second_factor", "", "denied")
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeChallengeExpired, "login challenge expired")
		return
	}
	h.setLoginChallenge(w, r, "", -1)
	var body struct {
		TOTPCode     string `json:"totp_code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		h.svc.auditEvent(ctx, "authentication", "auth.second_factor", "", "denied")
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid second factor")
		return
	}
	session, err := h.svc.CompleteLogin(ctx, challenge.Value, h.sourceHash(r), body.TOTPCode, body.RecoveryCode)
	if err != nil {
		h.factorFailures.failed(limitKey, h.svc.now())
		if errors.Is(err, ErrChallengeExpired) {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeChallengeExpired, "login challenge expired")
		} else {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		}
		return
	}
	h.factorFailures.succeeded(limitKey)
	h.writeSession(w, r, session)
}

func (h *Handler) writeRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	middleware.WriteError(w, http.StatusTooManyRequests, middleware.CodeRateLimited, "too many authentication attempts")
}

func (h *Handler) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid TOTP policy change")
		return
	}
	if err := h.svc.DisableTOTP(ctx, session.AdminID, session.SessionIDHash, body.Password, body.TOTPCode); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		} else {
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]bool{"totp_login_required": false})
}

func (h *Handler) SessionRenewal(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	member := middleware.SessionFrom(r)
	var body struct {
		Password     string `json:"password"`
		TOTPCode     string `json:"totp_code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || member == nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid session renewal")
		return
	}
	session, err := h.svc.RenewSession(ctx, member.SessionIDHash, member.AdminID, body.Password, body.TOTPCode, body.RecoveryCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	h.writeSession(w, r, session)
}

func (h *Handler) StepUp(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		Password     string `json:"password"`
		TOTPCode     string `json:"totp_code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if session == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid step-up request")
		return
	}
	expiresAt, err := h.svc.StepUp(ctx, session.SessionIDHash, session.AdminID, body.Password, body.TOTPCode, body.RecoveryCode)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		} else {
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"expires_at": timeString(expiresAt)})
}

func (h *Handler) UsernameChange(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"current_password"`
		TOTPCode        string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || session == nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid username change")
		return
	}
	newSession, err := h.svc.ChangeUsername(ctx, session.AdminID, body.Username, body.CurrentPassword, body.TOTPCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		case errors.Is(err, database.ErrDuplicate):
			middleware.WriteError(w, http.StatusConflict, middleware.CodeDuplicate, "username already taken")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	h.writeSession(w, r, newSession)
}

func (h *Handler) PasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		TOTPCode        string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || session == nil {
		middleware.WriteError(w, http.StatusBadRequest, middleware.CodeBadRequest, "invalid password change")
		return
	}
	newSession, err := h.svc.ChangePassword(ctx, session.AdminID, body.CurrentPassword, body.NewPassword, body.TOTPCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "invalid credentials")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		}
		return
	}
	h.writeSession(w, r, newSession)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	session := middleware.SessionFrom(r)
	if cookie, err := r.Cookie(middleware.SessionCookie); err == nil {
		if session != nil {
			h.svc.Logout(ctx, crypto.HashToken(cookie.Value), session.AdminID, session.Username)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: middleware.SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secureCookies(r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := h.withContext(r)
	count, err := h.svc.repo.AdminCount(ctx)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		return
	}
	if count == 0 {
		middleware.WriteJSON(w, http.StatusOK, map[string]any{"bootstrap_required": true})
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		middleware.WriteError(w, http.StatusUnauthorized, middleware.CodeUnauthorized, "not authenticated")
		return
	}
	organization, err := h.svc.repo.Organization(ctx)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, middleware.CodeInternal, "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"bootstrap_required":  false,
		"organization":        map[string]string{"display_name": organization.DisplayName},
		"member":              map[string]string{"id": session.AdminID, "username": session.Username, "role": session.Role},
		"csrf_token":          session.CSRFToken,
		"session_expires_at":  timeString(session.ExpiresAt),
		"idle_expires_at":     timeString(session.IdleExpiresAt),
		"totp_login_required": organization.TOTPLoginRequired,
		"auth_method":         session.AuthMethod,
	})
}

func timeString(t time.Time) string { return t.UTC().Format(time.RFC3339) }
