package identity

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewOAuthClientRejectsNonLoopbackHTTP(t *testing.T) {
	_, err := NewOAuthClient(OAuthConfig{
		PublicURL:        "https://core.example",
		AuthorizationURL: "http://idp.example/authorize",
		TokenURL:         "https://idp.example/token",
		UserinfoURL:      "https://idp.example/userinfo",
		ClientID:         "autosecrets",
	})
	if err == nil {
		t.Fatal("insecure OAuth authorization URL accepted")
	}
}

func TestOAuthClientExchangesCodeForUserinfoSubject(t *testing.T) {
	var sawBasicAuth bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			user, password, ok := r.BasicAuth()
			if !ok || user != "autosecrets" || password != "secret" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			sawBasicAuth = true
			body, _ := io.ReadAll(r.Body)
			values, _ := url.ParseQuery(string(body))
			if values.Get("code") != "auth-code" || values.Get("code_verifier") == "" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") == "" {
				http.Error(w, "missing github headers", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token"})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if r.Header.Get("User-Agent") == "" {
				http.Error(w, "missing user-agent", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "login": "alice"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)

	client, err := NewOAuthClient(OAuthConfig{
		PublicURL:        "http://127.0.0.1:8080",
		AuthorizationURL: provider.URL + "/authorize",
		TokenURL:         provider.URL + "/token",
		UserinfoURL:      provider.URL + "/userinfo",
		ClientID:         "autosecrets",
		ClientSecret:     "secret",
		Scopes:           []string{"profile"},
		HTTPClient:       provider.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := url.Parse(client.AuthorizationURL("/api/v1", "state-1", "verifier-1"))
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Query().Get("code_challenge_method") != "S256" || !strings.Contains(authorization.RawQuery, "code_challenge=") {
		t.Fatalf("authorization URL missing PKCE: %s", authorization)
	}

	identity, err := client.ExchangeAndIdentify(t.Context(), "/api/v1", "auth-code", "verifier-1")
	if err != nil {
		t.Fatal(err)
	}
	if !sawBasicAuth {
		t.Fatal("token exchange did not use client secret basic auth")
	}
	if identity.Subject != "42" || identity.DisplayName != "alice" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestOAuthClientAcceptsGitHubFormTokenResponse(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Header.Get("User-Agent") == "" {
				http.Error(w, "missing user-agent", http.StatusForbidden)
				return
			}
			if r.Header.Get("Accept") != "application/json" {
				w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
				_, _ = io.WriteString(w, "access_token=gho-form-token&token_type=bearer&scope=read%3Auser")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "gho-json-token"})
		case "/userinfo":
			if r.Header.Get("User-Agent") == "" {
				http.Error(w, "missing user-agent", http.StatusForbidden)
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token != "gho-json-token" && token != "gho-form-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "login": "octocat"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)

	client, err := NewOAuthClient(OAuthConfig{
		PublicURL:        "https://as.example",
		AuthorizationURL: provider.URL + "/authorize",
		TokenURL:         provider.URL + "/token",
		UserinfoURL:      provider.URL + "/userinfo",
		ClientID:         "autosecrets",
		ClientSecret:     "secret",
		HTTPClient:       provider.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.ExchangeAndIdentify(t.Context(), "/api/v1", "auth-code", "verifier-1")
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "7" || identity.DisplayName != "octocat" {
		t.Fatalf("identity = %+v", identity)
	}
}
