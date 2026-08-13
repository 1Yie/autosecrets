package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOIDCDiscoveryAuthorizationAndIDTokenValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	var issuer string
	var tokenNonce = "nonce-1"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": issuer, "authorization_endpoint": issuer + "/authorize",
				"token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks",
				"response_types_supported":              []string{"code"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
				"kid": "test-key", "kty": "RSA", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}}})
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("code") != "valid-code" || r.Form.Get("code_verifier") != "verifier" ||
				r.Form.Get("redirect_uri") != "https://core.example/api/v1/auth/oidc/callback" ||
				r.Form.Get("client_id") != "autosecrets" {
				t.Errorf("token request = %v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signTestIDToken(t, key, map[string]any{
				"iss": issuer, "sub": "provider-user-1", "aud": "autosecrets",
				"exp": now.Add(time.Hour).Unix(), "iat": now.Add(-time.Minute).Unix(),
				"nonce": tokenNonce, "name": "Administrator",
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	issuer = provider.URL

	client, err := DiscoverOIDC(context.Background(), OIDCConfig{
		PublicURL: "https://core.example", IssuerURL: issuer,
		ClientID: "autosecrets", Scopes: []string{"openid", "profile"}, HTTPClient: provider.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := url.Parse(client.AuthorizationURL("/api/v1", "state-1", "nonce-1", "verifier"))
	if err != nil {
		t.Fatal(err)
	}
	query := authorization.Query()
	if authorization.Path != "/authorize" || query.Get("state") != "state-1" || query.Get("nonce") != "nonce-1" ||
		query.Get("response_type") != "code" || query.Get("code_challenge_method") != "S256" ||
		query.Get("code_challenge") != pkceChallenge("verifier") {
		t.Fatalf("authorization URL = %s", authorization)
	}
	identity, err := client.ExchangeAndValidate(context.Background(), "/api/v1", "valid-code", "verifier", "nonce-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Issuer != issuer || identity.Subject != "provider-user-1" || identity.DisplayName != "Administrator" {
		t.Fatalf("identity = %#v", identity)
	}

	tokenNonce = "another-transaction"
	if _, err := client.ExchangeAndValidate(context.Background(), "/api/v1", "valid-code", "verifier", "nonce-1", now); err == nil {
		t.Fatal("ID token with the wrong nonce accepted")
	}
}

func TestOIDCDiscoveryRejectsIssuerMismatch(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": "https://other.example", "authorization_endpoint": "https://other.example/authorize",
			"token_endpoint": "https://other.example/token", "jwks_uri": "https://other.example/jwks",
			"response_types_supported":              []string{"code"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	}))
	defer provider.Close()
	_, err := DiscoverOIDC(context.Background(), OIDCConfig{
		PublicURL: "https://core.example", IssuerURL: provider.URL,
		ClientID: "autosecrets", Scopes: []string{"openid"}, HTTPClient: provider.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("issuer mismatch accepted: %v", err)
	}
}

func TestOIDCDiscoveryRejectsInsecureProviderEndpoints(t *testing.T) {
	var issuer string
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": issuer, "authorization_endpoint": "http://provider.example/authorize",
			"token_endpoint": "http://provider.example/token", "jwks_uri": "http://provider.example/jwks",
			"response_types_supported":              []string{"code"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	}))
	defer provider.Close()
	issuer = provider.URL
	_, err := DiscoverOIDC(context.Background(), OIDCConfig{
		PublicURL: "https://core.example", IssuerURL: issuer,
		ClientID: "autosecrets", Scopes: []string{"openid"}, HTTPClient: provider.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid endpoint") {
		t.Fatalf("insecure discovery endpoints accepted: %v", err)
	}
}

func TestOIDCDiscoveryRejectsInsecureIssuerBeforeNetworkIO(t *testing.T) {
	requested := false
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		requested = true
		return nil, errors.New("must not be called")
	})
	_, err := DiscoverOIDC(context.Background(), OIDCConfig{
		PublicURL: "https://core.example", IssuerURL: "http://provider.example",
		ClientID: "autosecrets", Scopes: []string{"openid"}, HTTPClient: &http.Client{Transport: transport},
	})
	if err == nil || requested {
		t.Fatalf("insecure issuer was contacted: err=%v requested=%v", err, requested)
	}
}

func TestOIDCRejectsInvalidTokenClaimsAndSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	var issuer string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "test-key", "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer provider.Close()
	issuer = provider.URL
	client := &OIDCClient{
		cfg:      OIDCConfig{IssuerURL: issuer, ClientID: "autosecrets"},
		metadata: oidcMetadata{JWKSURI: issuer + "/jwks"},
		http:     provider.Client(),
	}
	validClaims := func() map[string]any {
		return map[string]any{
			"iss": issuer, "sub": "provider-user-1", "aud": "autosecrets",
			"exp": now.Add(time.Hour).Unix(), "iat": now.Add(-time.Minute).Unix(), "nonce": "nonce-1",
		}
	}
	for _, test := range []struct {
		name   string
		claims func(map[string]any)
		key    *rsa.PrivateKey
	}{
		{name: "wrong audience", claims: func(c map[string]any) { c["aud"] = "other-client" }, key: key},
		{name: "expired", claims: func(c map[string]any) { c["exp"] = now.Add(-3 * time.Minute).Unix() }, key: key},
		{name: "missing issued at", claims: func(c map[string]any) { delete(c, "iat") }, key: key},
		{name: "stale issued at", claims: func(c map[string]any) { c["iat"] = now.Add(-2 * time.Hour).Unix() }, key: key},
		{name: "wrong signature", claims: func(map[string]any) {}, key: wrongKey},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := validClaims()
			test.claims(claims)
			raw := signTestIDToken(t, test.key, claims)
			if _, err := client.validateIDToken(context.Background(), raw, "nonce-1", now); err == nil {
				t.Fatal("invalid ID token accepted")
			}
		})
	}
}

func signTestIDToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
