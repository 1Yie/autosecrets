package identity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type OIDCConfig struct {
	PublicURL    string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string
	HTTPClient   *http.Client
}

type OIDCIdentity struct {
	Issuer      string
	Subject     string
	DisplayName string
}

type oidcMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	ResponseTypes         []string `json:"response_types_supported"`
	SigningAlgorithms     []string `json:"id_token_signing_alg_values_supported"`
}

type OIDCClient struct {
	cfg      OIDCConfig
	metadata oidcMetadata
	http     *http.Client
}

const maxIDTokenAge = time.Hour

func DiscoverOIDC(ctx context.Context, cfg OIDCConfig) (*OIDCClient, error) {
	issuerURL, err := url.Parse(cfg.IssuerURL)
	if err != nil || issuerURL.RawQuery != "" || issuerURL.Fragment != "" || !oidcEndpointAllowed(cfg.IssuerURL, issuerURL) {
		return nil, errors.New("OIDC issuer must use HTTPS except on localhost")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	client = oidcHTTPClient(client, cfg.IssuerURL)
	discoveryURL := strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	var metadata oidcMetadata
	if err := getJSON(ctx, client, discoveryURL, &metadata); err != nil {
		return nil, fmt.Errorf("OIDC discovery unavailable: %w", err)
	}
	if metadata.Issuer != cfg.IssuerURL || metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" || metadata.JWKSURI == "" {
		return nil, errors.New("OIDC discovery metadata does not match the configured issuer")
	}
	if !slices.Contains(metadata.ResponseTypes, "code") || !slices.Contains(metadata.SigningAlgorithms, "RS256") {
		return nil, errors.New("OIDC provider must support authorization code and RS256 ID tokens")
	}
	for _, endpoint := range []string{metadata.AuthorizationEndpoint, metadata.TokenEndpoint, metadata.JWKSURI} {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || !oidcEndpointAllowed(cfg.IssuerURL, parsed) {
			return nil, errors.New("OIDC discovery returned an invalid endpoint")
		}
	}
	return &OIDCClient{cfg: cfg, metadata: metadata, http: client}, nil
}

func (c *OIDCClient) RedirectURI(base string) string {
	return strings.TrimRight(c.cfg.PublicURL, "/") + base + "/auth/oidc/callback"
}

func (c *OIDCClient) AuthorizationURL(base, state, nonce, verifier string) string {
	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.RedirectURI(base)},
		"scope":                 {strings.Join(c.cfg.Scopes, " ")},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	return c.metadata.AuthorizationEndpoint + "?" + values.Encode()
}

func (c *OIDCClient) ExchangeAndValidate(ctx context.Context, base, code, verifier, nonce string, now time.Time) (*OIDCIdentity, error) {
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURI(base)},
		"code_verifier": {verifier},
	}
	if c.cfg.ClientSecret == "" {
		values.Set("client_id", c.cfg.ClientID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.metadata.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.cfg.ClientSecret != "" {
		req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC token exchange unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("OIDC token exchange rejected")
	}
	var tokens struct {
		IDToken string `json:"id_token"`
	}
	if err := decodeBoundedJSON(response.Body, &tokens); err != nil || tokens.IDToken == "" {
		return nil, errors.New("OIDC token response did not contain an ID token")
	}
	return c.validateIDToken(ctx, tokens.IDToken, nonce, now)
}

func (c *OIDCClient) validateIDToken(ctx context.Context, raw, nonce string, now time.Time) (*OIDCIdentity, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid ID token")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil || header.Algorithm != "RS256" || header.KeyID == "" {
		return nil, errors.New("unsupported ID token signature")
	}
	key, err := c.signingKey(ctx, header.KeyID)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return nil, errors.New("invalid ID token signature")
	}
	var claims struct {
		Issuer   string          `json:"iss"`
		Subject  string          `json:"sub"`
		Audience json.RawMessage `json:"aud"`
		AZP      string          `json:"azp"`
		Expires  int64           `json:"exp"`
		IssuedAt int64           `json:"iat"`
		Nonce    string          `json:"nonce"`
		Name     string          `json:"name"`
	}
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return nil, errors.New("invalid ID token claims")
	}
	audiences, err := parseAudience(claims.Audience)
	if err != nil || !slices.Contains(audiences, c.cfg.ClientID) || len(audiences) > 1 && claims.AZP != c.cfg.ClientID || claims.AZP != "" && claims.AZP != c.cfg.ClientID {
		return nil, errors.New("invalid ID token audience")
	}
	clockSkew := 2 * time.Minute
	issuedAt := time.Unix(claims.IssuedAt, 0)
	expiresAt := time.Unix(claims.Expires, 0)
	if claims.Issuer != c.cfg.IssuerURL || claims.Subject == "" || claims.Nonce != nonce ||
		claims.IssuedAt <= 0 || claims.Expires <= 0 || expiresAt.Before(issuedAt) ||
		now.After(expiresAt.Add(clockSkew)) || issuedAt.After(now.Add(clockSkew)) ||
		now.Sub(issuedAt) > maxIDTokenAge+clockSkew {
		return nil, errors.New("invalid ID token claims")
	}
	return &OIDCIdentity{Issuer: claims.Issuer, Subject: claims.Subject, DisplayName: claims.Name}, nil
}

func oidcHTTPClient(client *http.Client, issuer string) *http.Client {
	clone := *client
	originalCheckRedirect := client.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !oidcEndpointAllowed(issuer, req.URL) {
			return errors.New("OIDC redirect attempted an insecure endpoint")
		}
		if originalCheckRedirect != nil {
			return originalCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &clone
}

func oidcEndpointAllowed(issuer string, endpoint *url.URL) bool {
	issuerURL, err := url.Parse(issuer)
	if err != nil || endpoint == nil || endpoint.Host == "" {
		return false
	}
	if issuerURL.Scheme == "https" {
		return endpoint.Scheme == "https"
	}
	return issuerURL.Scheme == "http" && isLoopbackOIDCHost(issuerURL.Hostname()) &&
		endpoint.Scheme == "http" && isLoopbackOIDCHost(endpoint.Hostname())
}

func isLoopbackOIDCHost(host string) bool {
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func (c *OIDCClient) signingKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	var set struct {
		Keys []struct {
			KeyID     string `json:"kid"`
			KeyType   string `json:"kty"`
			Algorithm string `json:"alg"`
			Use       string `json:"use"`
			Modulus   string `json:"n"`
			Exponent  string `json:"e"`
		} `json:"keys"`
	}
	if err := getJSON(ctx, c.http, c.metadata.JWKSURI, &set); err != nil {
		return nil, fmt.Errorf("OIDC signing keys unavailable: %w", err)
	}
	for _, candidate := range set.Keys {
		if candidate.KeyID != keyID || candidate.KeyType != "RSA" || candidate.Algorithm != "RS256" || candidate.Use != "sig" {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(candidate.Modulus)
		if err != nil {
			break
		}
		exponentBytes, err := base64.RawURLEncoding.DecodeString(candidate.Exponent)
		if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
			break
		}
		exponent, err := strconv.ParseUint(fmt.Sprintf("%x", exponentBytes), 16, 32)
		if err != nil || exponent < 3 {
			break
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(exponent)}, nil
	}
	return nil, errors.New("ID token signing key not found")
}

func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	return decodeBoundedJSON(response.Body, target)
}

func decodeBoundedJSON(reader io.Reader, target any) error {
	return json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(target)
}

func decodeJWTPart(raw string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, target)
}

func parseAudience(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil || len(multiple) == 0 {
		return nil, errors.New("invalid audience")
	}
	return multiple, nil
}
