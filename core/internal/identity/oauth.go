package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const oauthUserAgent = "autosecrets-core"

// OAuthConfig is the deployment-provided Authorization Code + PKCE client.
// Endpoints are explicit: there is no OpenID discovery and no ID Token.
type OAuthConfig struct {
	PublicURL        string
	AuthorizationURL string
	TokenURL         string
	UserinfoURL      string
	ClientID         string
	ClientSecret     string
	Scopes           []string
	HTTPClient       *http.Client
}

// OAuthClient exchanges an authorization code for a userinfo subject.
// It does not persist provider tokens.
type OAuthClient struct {
	cfg  OAuthConfig
	http *http.Client
}

func NewOAuthClient(cfg OAuthConfig) (*OAuthClient, error) {
	for _, raw := range []string{cfg.AuthorizationURL, cfg.TokenURL, cfg.UserinfoURL} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || !oauthEndpointAllowed(parsed) {
			return nil, errors.New("OAuth endpoints must be HTTPS URLs except on localhost")
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &OAuthClient{cfg: cfg, http: client}, nil
}

func (c *OAuthClient) Issuer() string {
	return strings.TrimRight(c.cfg.AuthorizationURL, "/")
}

func (c *OAuthClient) RedirectURI(base string) string {
	return strings.TrimRight(c.cfg.PublicURL, "/") + base + "/auth/oauth/callback"
}

func (c *OAuthClient) AuthorizationURL(base, state, verifier string) string {
	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.RedirectURI(base)},
		"state":                 {state},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	if len(c.cfg.Scopes) > 0 {
		values.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	return c.cfg.AuthorizationURL + "?" + values.Encode()
}

func (c *OAuthClient) ExchangeAndIdentify(ctx context.Context, base, code, verifier string) (*OIDCIdentity, error) {
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURI(base)},
		"code_verifier": {verifier},
	}
	if c.cfg.ClientSecret == "" {
		values.Set("client_id", c.cfg.ClientID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", oauthUserAgent)
	if c.cfg.ClientSecret != "" {
		req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth token exchange unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("OAuth token exchange rejected")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, errors.New("OAuth token response did not contain an access token")
	}
	accessToken, err := decodeOAuthAccessToken(body)
	if err != nil {
		return nil, err
	}
	return c.userinfoIdentity(ctx, accessToken)
}

func (c *OAuthClient) userinfoIdentity(ctx context.Context, accessToken string) (*OIDCIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.UserinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", oauthUserAgent)
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth userinfo unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("OAuth userinfo rejected")
	}
	claims := map[string]any{}
	if err := decodeBoundedJSON(response.Body, &claims); err != nil || len(claims) == 0 {
		return nil, errors.New("OAuth userinfo was not valid JSON")
	}
	subject := oauthStringClaim(claims, "sub")
	if subject == "" {
		subject = oauthStringClaim(claims, "id")
	}
	if subject == "" {
		return nil, errors.New("OAuth userinfo did not contain a subject")
	}
	display := oauthStringClaim(claims, "name")
	if display == "" {
		display = oauthStringClaim(claims, "login")
	}
	if display == "" {
		display = oauthStringClaim(claims, "preferred_username")
	}
	return &OIDCIdentity{Issuer: c.Issuer(), Subject: subject, DisplayName: display}, nil
}

func decodeOAuthAccessToken(body []byte) (string, error) {
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &tokens) == nil && tokens.AccessToken != "" {
		return tokens.AccessToken, nil
	}
	values, err := url.ParseQuery(string(body))
	if err != nil || values.Get("access_token") == "" {
		return "", errors.New("OAuth token response did not contain an access token")
	}
	return values.Get("access_token"), nil
}

func oauthEndpointAllowed(endpoint *url.URL) bool {
	if endpoint == nil || endpoint.Host == "" {
		return false
	}
	if endpoint.Scheme == "https" {
		return true
	}
	return endpoint.Scheme == "http" && isLoopbackOIDCHost(endpoint.Hostname())
}

func oauthStringClaim(claims map[string]any, key string) string {
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
	}
	return ""
}
