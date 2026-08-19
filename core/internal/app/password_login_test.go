package app

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"autosecrets.dev/core/internal/identity"
	"autosecrets.dev/core/internal/middleware"
)

func TestPasswordLoginPolicyRequiresUsableExternalLogin(t *testing.T) {
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

	denied := ta.do(t, "PUT", "/api/v1/auth/password-login", map[string]any{
		"enabled": false, "password": "correct-horse-42",
	}, cookie, csrf)
	if denied.status != http.StatusConflict || denied.body["code"] != middleware.CodeConflict {
		t.Fatalf("disable without binding: %d %s", denied.status, denied.raw)
	}

	started := ta.do(t, "POST", "/api/v1/auth/oauth/binding", map[string]string{
		"password": "correct-horse-42", "return_to": "/dashboard/settings",
	}, cookie, csrf)
	if started.status != http.StatusOK {
		t.Fatalf("start binding: %d %s", started.status, started.raw)
	}
	stateCookie := cookieValueFrom(t, started, "autosecrets_oauth_state")
	authorizationURL, err := url.Parse(started.body["authorization_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	bound := ta.doH(t, "GET", "/api/v1/auth/oauth/callback?code=bind-code&state="+url.QueryEscape(authorizationURL.Query().Get("state")), nil,
		map[string]string{"Cookie": "autosecrets_oauth_state=" + stateCookie + "; " + sessionCookie + "=" + cookie})
	if bound.status != http.StatusFound {
		t.Fatalf("binding callback: %d %s", bound.status, bound.raw)
	}

	disabled := ta.do(t, "PUT", "/api/v1/auth/password-login", map[string]any{
		"enabled": false, "password": "correct-horse-42",
	}, cookie, csrf)
	if disabled.status != http.StatusOK || disabled.body["password_login_enabled"] != false || disabled.body["password_login_available"] != false {
		t.Fatalf("disable password login: %d %s", disabled.status, disabled.raw)
	}

	public := ta.do(t, "GET", "/api/v1/auth/oidc/status", nil, "", "")
	if public.body["password_login_available"] != false {
		t.Fatalf("public status still offers password login: %s", public.raw)
	}
	blocked := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if blocked.status != http.StatusForbidden || blocked.body["code"] != middleware.CodePasswordLoginDisabled {
		t.Fatalf("password login after disable: %d %s", blocked.status, blocked.raw)
	}

	lastUnbind := ta.do(t, "DELETE", "/api/v1/auth/oauth/binding", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if lastUnbind.status != http.StatusConflict || lastUnbind.body["code"] != middleware.CodeConflict {
		t.Fatalf("last unbind while password login disabled: %d %s", lastUnbind.status, lastUnbind.raw)
	}

	enabled := ta.do(t, "PUT", "/api/v1/auth/password-login", map[string]any{
		"enabled": true, "password": "correct-horse-42",
	}, cookie, csrf)
	if enabled.status != http.StatusOK || enabled.body["password_login_enabled"] != true {
		t.Fatalf("re-enable password login: %d %s", enabled.status, enabled.raw)
	}
	restored := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if restored.status != http.StatusOK {
		t.Fatalf("password login after re-enable: %d %s", restored.status, restored.raw)
	}
}
