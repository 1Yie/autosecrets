package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/identity"
)

func TestOAuthBindingLoginAndUnbindJourney(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	provider := newDeterministicOAuthProvider(t)
	client, err := identity.NewOAuthClient(identity.OAuthConfig{
		PublicURL: "https://core.example", AuthorizationURL: provider.server.URL + "/authorize",
		TokenURL: provider.server.URL + "/token", UserinfoURL: provider.server.URL + "/userinfo",
		ClientID: "autosecrets", ClientSecret: "secret", Scopes: []string{"profile"},
		HTTPClient: provider.server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ta := newTestApp(t, func(options *Options) {
		options.Now = func() time.Time { return now }
		options.OAuthClient = client
	})
	ta.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cookie, csrf := ta.bootstrap(t)

	publicBefore := ta.do(t, "GET", "/api/v1/auth/oauth/status", nil, "", "")
	if publicBefore.status != http.StatusOK || publicBefore.body["available"] != true || publicBefore.body["login_available"] != false {
		t.Fatalf("unbound public OAuth status: %d %s", publicBefore.status, publicBefore.raw)
	}

	deniedBinding := ta.do(t, "POST", "/api/v1/auth/oauth/binding", map[string]string{
		"password": "wrong-password-42", "return_to": "/dashboard/login-and-security",
	}, cookie, csrf)
	if deniedBinding.status != http.StatusUnauthorized {
		t.Fatalf("binding accepted invalid credentials: %d %s", deniedBinding.status, deniedBinding.raw)
	}

	started := ta.do(t, "POST", "/api/v1/auth/oauth/binding", map[string]string{
		"password": "correct-horse-42", "return_to": "/dashboard/login-and-security",
	}, cookie, csrf)
	if started.status != http.StatusOK {
		t.Fatalf("start binding: %d %s", started.status, started.raw)
	}
	stateCookie := cookieValueFrom(t, started, "autosecrets_oauth_state")
	authorizationURL, err := url.Parse(started.body["authorization_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if authorizationURL.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("binding authorization missing PKCE: %s", authorizationURL)
	}
	state := authorizationURL.Query().Get("state")

	bound := ta.doH(t, "GET", "/api/v1/auth/oauth/callback?code=bind-code&state="+url.QueryEscape(state), nil,
		map[string]string{"Cookie": "autosecrets_oauth_state=" + stateCookie + "; " + sessionCookie + "=" + cookie})
	if bound.status != http.StatusFound || bound.header.Get("Location") != "/dashboard/login-and-security" {
		t.Fatalf("binding callback: %d %s location=%s", bound.status, bound.raw, bound.header.Get("Location"))
	}

	security := ta.do(t, "GET", "/api/v1/auth/security", nil, cookie, "")
	oauthState, _ := security.body["oauth"].(map[string]any)
	if security.status != http.StatusOK || oauthState["bound"] != true {
		t.Fatalf("security state after binding: %d %s", security.status, security.raw)
	}
	combined := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	oauthPublic, _ := combined.body["oauth"].(map[string]any)
	if oauthPublic["login_available"] != true {
		t.Fatalf("bound OAuth not advertised: %s", combined.raw)
	}

	loginStart := ta.do(t, "GET", "/api/v1/auth/oauth/login?return_to=/dashboard/overview", nil, "", "")
	if loginStart.status != http.StatusFound {
		t.Fatalf("start OAuth login: %d %s", loginStart.status, loginStart.raw)
	}
	loginAuthorization, _ := url.Parse(loginStart.header.Get("Location"))
	loginStateCookie := cookieValueFrom(t, loginStart, "autosecrets_oauth_state")
	loginCallback := ta.doH(t, "GET", "/api/v1/auth/oauth/callback?code=login-code&state="+url.QueryEscape(loginAuthorization.Query().Get("state")), nil,
		map[string]string{"Cookie": "autosecrets_oauth_state=" + loginStateCookie})
	if loginCallback.status != http.StatusFound {
		t.Fatalf("OAuth login callback: %d %s", loginCallback.status, loginCallback.raw)
	}
	oauthSession := sessionCookieFrom(t, loginCallback)
	me := ta.do(t, "GET", "/api/v1/me", nil, oauthSession, "")
	if me.status != http.StatusOK || me.body["auth_method"] != "oauth" {
		t.Fatalf("OAuth Session: %d %s", me.status, me.raw)
	}

	provider.subject = "another-provider-user"
	rejectedStart := ta.do(t, "GET", "/api/v1/auth/oauth/login", nil, "", "")
	rejectedAuthorization, _ := url.Parse(rejectedStart.header.Get("Location"))
	rejectedCookie := cookieValueFrom(t, rejectedStart, "autosecrets_oauth_state")
	rejected := ta.doH(t, "GET", "/api/v1/auth/oauth/callback?code=wrong-user&state="+url.QueryEscape(rejectedAuthorization.Query().Get("state")), nil,
		map[string]string{"Cookie": "autosecrets_oauth_state=" + rejectedCookie})
	if rejected.status != http.StatusUnauthorized || rejected.body["code"] != "unauthorized" {
		t.Fatalf("unbound provider subject accepted: %d %s", rejected.status, rejected.raw)
	}

	unbound := ta.doH(t, "DELETE", "/api/v1/auth/oauth/binding", map[string]string{
		"password": "correct-horse-42",
	}, map[string]string{"Cookie": sessionCookie + "=" + cookie, csrfHeader: csrf})
	if unbound.status != http.StatusOK {
		t.Fatalf("unbind: %d %s", unbound.status, unbound.raw)
	}
	status := ta.do(t, "GET", "/api/v1/auth/oauth/status", nil, "", "")
	if status.body["bound"] != false || status.body["login_available"] != false {
		t.Fatalf("OAuth advertised after unbind: %s", status.raw)
	}
	stale := ta.do(t, "GET", "/api/v1/me", nil, oauthSession, "")
	if stale.status != http.StatusUnauthorized {
		t.Fatalf("OAuth Session survived unbind: %d %s", stale.status, stale.raw)
	}
	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, cookie, "")
	assertAuditResults(t, audit.raw, map[string][]string{
		"administrator.oauth_bound":   {"ok", "denied"},
		"administrator.oauth_unbound": {"ok"},
		"auth.login":                  {"oauth", "denied"},
	})
	for _, value := range []string{"bind-code", "login-code", "wrong-user", provider.subject} {
		if strings.Contains(string(audit.raw), value) {
			t.Fatalf("OAuth protocol value %q leaked into Audit Events", value)
		}
	}
}

type deterministicOAuthProvider struct {
	server  *httptest.Server
	subject string
}

func newDeterministicOAuthProvider(t *testing.T) *deterministicOAuthProvider {
	t.Helper()
	p := &deterministicOAuthProvider{subject: "provider-user-1"}
	p.server = httptest.NewServer(http.HandlerFunc(p.serveHTTP))
	t.Cleanup(p.server.Close)
	return p
}

func (p *deterministicOAuthProvider) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/token":
		if user, password, ok := r.BasicAuth(); !ok || user != "autosecrets" || password != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("code") == "" || values.Get("code_verifier") == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token"})
	case "/userinfo":
		if r.Header.Get("Authorization") != "Bearer access-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub": p.subject, "name": "E2E Administrator",
		})
	default:
		http.NotFound(w, r)
	}
}
