package app

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/identity"
)

func TestOIDCBindingLoginAndUnbindJourney(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	provider := newDeterministicOIDCProvider(t, now)
	client, err := identity.DiscoverOIDC(t.Context(), identity.OIDCConfig{
		PublicURL: "https://core.example", IssuerURL: provider.server.URL,
		ClientID: "autosecrets", Scopes: []string{"openid", "profile"},
		HTTPClient: provider.server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ta := newTestApp(t, func(options *Options) {
		options.Now = func() time.Time { return now }
		options.OIDCClient = client
	})
	ta.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cookie, csrf := ta.bootstrap(t)

	publicBefore := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	if publicBefore.status != http.StatusOK || publicBefore.body["available"] != true || publicBefore.body["login_available"] != false {
		t.Fatalf("unbound public OIDC status: %d %s", publicBefore.status, publicBefore.raw)
	}
	deniedBinding := ta.do(t, "POST", "/api/v1/auth/oidc/binding", map[string]string{
		"password": "wrong-password-42", "return_to": "/dashboard/settings",
	}, cookie, csrf)
	if deniedBinding.status != http.StatusUnauthorized {
		t.Fatalf("binding accepted invalid credentials: %d %s", deniedBinding.status, deniedBinding.raw)
	}

	started := ta.do(t, "POST", "/api/v1/auth/oidc/binding", map[string]string{
		"password": "correct-horse-42", "return_to": "/dashboard/settings",
	}, cookie, csrf)
	if started.status != http.StatusOK {
		t.Fatalf("start binding: %d %s", started.status, started.raw)
	}
	stateCookie := cookieValueFrom(t, started, "autosecrets_oidc_state")
	authorizationURL, err := url.Parse(started.body["authorization_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	provider.expectAuthorization(t, authorizationURL)
	state := authorizationURL.Query().Get("state")
	provider.nonce = authorizationURL.Query().Get("nonce")

	bound := ta.doH(t, "GET", "/api/v1/auth/oidc/callback?code=bind-code&state="+url.QueryEscape(state), nil,
		map[string]string{"Cookie": "autosecrets_oidc_state=" + stateCookie + "; " + sessionCookie + "=" + cookie})
	if bound.status != http.StatusFound || bound.header.Get("Location") != "/dashboard/settings" {
		t.Fatalf("binding callback: %d %s location=%s", bound.status, bound.raw, bound.header.Get("Location"))
	}

	security := ta.do(t, "GET", "/api/v1/auth/security", nil, cookie, "")
	oidcState, _ := security.body["oidc"].(map[string]any)
	if security.status != http.StatusOK || oidcState["bound"] != true || oidcState["issuer"] != provider.server.URL {
		t.Fatalf("security state after binding: %d %s", security.status, security.raw)
	}
	publicAfter := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	if publicAfter.body["login_available"] != true {
		t.Fatalf("bound OIDC not advertised: %s", publicAfter.raw)
	}

	loginStart := ta.do(t, "GET", "/api/v1/auth/oidc/login?return_to=/dashboard/overview", nil, "", "")
	if loginStart.status != http.StatusFound {
		t.Fatalf("start OIDC login: %d %s", loginStart.status, loginStart.raw)
	}
	loginAuthorization, _ := url.Parse(loginStart.header.Get("Location"))
	provider.expectAuthorization(t, loginAuthorization)
	provider.nonce = loginAuthorization.Query().Get("nonce")
	loginStateCookie := cookieValueFrom(t, loginStart, "autosecrets_oidc_state")
	loginCallback := ta.doH(t, "GET", "/api/v1/auth/oidc/callback?code=login-code&state="+url.QueryEscape(loginAuthorization.Query().Get("state")), nil,
		map[string]string{"Cookie": "autosecrets_oidc_state=" + loginStateCookie})
	if loginCallback.status != http.StatusFound {
		t.Fatalf("OIDC login callback: %d %s", loginCallback.status, loginCallback.raw)
	}
	oidcSession := sessionCookieFrom(t, loginCallback)
	me := ta.do(t, "GET", "/api/v1/me", nil, oidcSession, "")
	if me.status != http.StatusOK || me.body["auth_method"] != "oidc" {
		t.Fatalf("OIDC Session: %d %s", me.status, me.raw)
	}

	provider.subject = "another-provider-user"
	rejectedStart := ta.do(t, "GET", "/api/v1/auth/oidc/login", nil, "", "")
	rejectedAuthorization, _ := url.Parse(rejectedStart.header.Get("Location"))
	provider.nonce = rejectedAuthorization.Query().Get("nonce")
	rejectedCookie := cookieValueFrom(t, rejectedStart, "autosecrets_oidc_state")
	rejected := ta.doH(t, "GET", "/api/v1/auth/oidc/callback?code=wrong-user&state="+url.QueryEscape(rejectedAuthorization.Query().Get("state")), nil,
		map[string]string{"Cookie": "autosecrets_oidc_state=" + rejectedCookie})
	if rejected.status != http.StatusUnauthorized || rejected.body["code"] != "unauthorized" {
		t.Fatalf("unbound provider subject accepted: %d %s", rejected.status, rejected.raw)
	}

	deniedUnbind := ta.doH(t, "DELETE", "/api/v1/auth/oidc/binding", map[string]string{
		"password": "wrong-password-42",
	}, map[string]string{"Cookie": sessionCookie + "=" + cookie, csrfHeader: csrf})
	if deniedUnbind.status != http.StatusUnauthorized {
		t.Fatalf("unbind accepted invalid credentials: %d %s", deniedUnbind.status, deniedUnbind.raw)
	}
	unbound := ta.doH(t, "DELETE", "/api/v1/auth/oidc/binding", map[string]string{
		"password": "correct-horse-42",
	}, map[string]string{"Cookie": sessionCookie + "=" + cookie, csrfHeader: csrf})
	if unbound.status != http.StatusOK {
		t.Fatalf("unbind: %d %s", unbound.status, unbound.raw)
	}
	status := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	if status.body["bound"] != false || status.body["login_available"] != false {
		t.Fatalf("OIDC advertised after unbind: %s", status.raw)
	}
	staleOIDCSession := ta.do(t, "GET", "/api/v1/me", nil, oidcSession, "")
	if staleOIDCSession.status != http.StatusUnauthorized {
		t.Fatalf("OIDC Session survived unbind: %d %s", staleOIDCSession.status, staleOIDCSession.raw)
	}
	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, cookie, "")
	assertAuditResults(t, audit.raw, map[string][]string{
		"administrator.oidc_bound":   {"ok", "denied"},
		"administrator.oidc_unbound": {"ok", "denied"},
		"auth.login":                 {"oidc", "denied"},
	})
	for _, value := range []string{"bind-code", "login-code", "wrong-user", provider.subject} {
		if strings.Contains(string(audit.raw), value) {
			t.Fatalf("OIDC protocol value %q leaked into Audit Events", value)
		}
	}
}

func TestOIDCUnavailablePreservesLocalLoginAndHidesDiagnosticAnonymously(t *testing.T) {
	ta := newTestApp(t, func(options *Options) { options.OIDCUnavailable = "provider discovery timed out" })
	cookie, _ := ta.bootstrap(t)
	public := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	if public.status != http.StatusOK || public.body["available"] != false || strings.Contains(string(public.raw), "timed out") {
		t.Fatalf("anonymous OIDC status leaked detail: %d %s", public.status, public.raw)
	}
	login := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if login.status != http.StatusOK {
		t.Fatalf("local login unavailable with OIDC failure: %d %s", login.status, login.raw)
	}
	security := ta.do(t, "GET", "/api/v1/auth/security", nil, cookie, "")
	oidcState, _ := security.body["oidc"].(map[string]any)
	if oidcState["configuration_error"] != "provider discovery timed out" {
		t.Fatalf("authenticated diagnostic missing: %s", security.raw)
	}
}

type deterministicOIDCProvider struct {
	server  *httptest.Server
	key     *rsa.PrivateKey
	now     time.Time
	nonce   string
	subject string
}

func newDeterministicOIDCProvider(t *testing.T, now time.Time) *deterministicOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &deterministicOIDCProvider{key: key, now: now, subject: "provider-user-1"}
	p.server = httptest.NewServer(http.HandlerFunc(p.serveHTTP))
	t.Cleanup(p.server.Close)
	return p
}

func (p *deterministicOIDCProvider) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": p.server.URL, "authorization_endpoint": p.server.URL + "/authorize",
			"token_endpoint": p.server.URL + "/token", "jwks_uri": p.server.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case "/jwks":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "test-key", "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(p.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes()),
		}}})
	case "/token":
		_ = r.ParseForm()
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": p.sign(map[string]any{
			"iss": p.server.URL, "sub": p.subject, "aud": "autosecrets",
			"exp": p.now.Add(time.Hour).Unix(), "iat": p.now.Add(-time.Minute).Unix(),
			"nonce": p.nonce, "name": "Platform Administrator",
		})})
	default:
		http.NotFound(w, r)
	}
}

func (p *deterministicOIDCProvider) expectAuthorization(t *testing.T, authorization *url.URL) {
	t.Helper()
	query := authorization.Query()
	if authorization.Host != strings.TrimPrefix(p.server.URL, "http://") || authorization.Path != "/authorize" ||
		query.Get("response_type") != "code" || query.Get("state") == "" || query.Get("nonce") == "" ||
		query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" ||
		query.Get("redirect_uri") != "https://core.example/api/v1/auth/oidc/callback" {
		t.Fatalf("authorization request: %s", authorization)
	}
}

func (p *deterministicOIDCProvider) sign(claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
