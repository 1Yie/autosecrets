package app

// Shared integration-test harness for the Core HTTP surfaces. Every domain
// test file in this package goes through newTestApp so the PostgreSQL, key
// material, and HTTP seams are set up exactly once.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/testutil"
)

func sessionCookieFrom(t *testing.T, response result) string {
	return cookieValueFrom(t, response, sessionCookie)
}

func cookieValueFrom(t *testing.T, response result, name string) string {
	t.Helper()
	for _, value := range response.header.Values("Set-Cookie") {
		if strings.HasPrefix(value, name+"=") {
			return strings.SplitN(strings.TrimPrefix(value, name+"="), ";", 2)[0]
		}
	}
	t.Fatalf("no %s cookie in response headers", name)
	return ""
}

func totpSecretFromURI(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse TOTP URI: %v", err)
	}
	secret := u.Query().Get("secret")
	if secret == "" {
		t.Fatal("TOTP URI has no secret")
	}
	return secret
}

type testApp struct {
	app      *App
	server   *httptest.Server
	store    *database.Store
	client   *http.Client
	signer   *crypto.Signer
	ca       *crypto.CA
	mk       *crypto.MasterKey
	contract *contractValidator
}

// newTestApp boots the full Core handler against a real PostgreSQL database
// (see testutil.Connect) with fresh key material in a temp dir.
func newTestApp(t *testing.T, adjust ...func(*Options)) *testApp {
	t.Helper()
	st := testutil.Connect(t)
	testutil.Truncate(t, st)

	mk, ca, signer := testutil.NewKeyMaterial(t)
	_, trusted, _ := net.ParseCIDR("127.0.0.0/8")
	opts := Options{
		Version:        "test",
		PublicAgentURL: "https://agent.test",
		ArtifactDir:    t.TempDir(),
		TrustedProxy:   []*net.IPNet{trusted},
		CertHeader:     "X-Autosecrets-Client-Cert",
		Now:            func() time.Time { return time.Now() },
	}
	for _, fn := range adjust {
		fn(&opts)
	}
	a := New(st, mk, ca, signer, "/api/v1", "/agent/v1", opts)
	server := httptest.NewServer(a.Handler())
	t.Cleanup(server.Close)
	return &testApp{
		app: a, server: server, store: st, contract: newContractValidator(t),
		client: server.Client(), signer: signer, ca: ca, mk: mk,
	}
}

type result struct {
	status int
	body   map[string]any
	raw    []byte
	header http.Header
}

func (ta *testApp) do(t *testing.T, method, path string, body any, cookie, csrf string) result {
	t.Helper()
	headers := map[string]string{}
	if cookie != "" {
		headers["Cookie"] = sessionCookie + "=" + cookie
	}
	if csrf != "" {
		headers[csrfHeader] = csrf
	}
	return ta.doH(t, method, path, body, headers)
}

func (ta *testApp) doH(t *testing.T, method, path string, body any, headers map[string]string) result {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, ta.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ta.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if ta.contract != nil && strings.HasPrefix(path, "/api/v1") {
		ta.contract.validate(t, method, path, resp.StatusCode, resp.Header, raw)
	}
	out := result{status: resp.StatusCode, raw: raw, header: resp.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// bootstrap creates the single Administrator and returns the Session issued
// by Bootstrap. New Organizations use password-only local login by default.
func (ta *testApp) bootstrap(t *testing.T) (cookie, csrf string) {
	t.Helper()
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil || code == "" {
		t.Fatalf("emit bootstrap code: %v", err)
	}
	res := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Test Organization",
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if res.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", res.status, res.raw)
	}
	cookie = sessionCookieFrom(t, res)
	csrf, _ = res.body["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("no csrf token")
	}
	return cookie, csrf
}

func (ta *testApp) enableTOTP(t *testing.T, cookie, csrf, password string) (secret string, recoveryCodes []any) {
	t.Helper()
	started := ta.do(t, "POST", "/api/v1/auth/totp/enrollment", map[string]string{
		"password": password,
	}, cookie, csrf)
	if started.status != http.StatusCreated {
		t.Fatalf("start TOTP enrollment: %d %s", started.status, started.raw)
	}
	secret = totpSecretFromURI(t, started.body["totp_uri"].(string))
	code, err := crypto.TOTPCode(secret, ta.app.now())
	if err != nil {
		t.Fatal(err)
	}
	verified := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/verify", map[string]string{
		"enrollment_token": started.body["enrollment_token"].(string), "totp_code": code,
	}, cookie, csrf)
	if verified.status != http.StatusOK {
		t.Fatalf("verify TOTP enrollment: %d %s", verified.status, verified.raw)
	}
	recoveryCodes, _ = verified.body["recovery_codes"].([]any)
	confirmed := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/confirm", map[string]string{
		"confirmation_token": verified.body["confirmation_token"].(string),
	}, cookie, csrf)
	if confirmed.status != http.StatusOK {
		t.Fatalf("confirm TOTP enrollment: %d %s", confirmed.status, confirmed.raw)
	}
	return secret, recoveryCodes
}

// authoringSetup creates an Application and Environment and returns the ids
// together with a bootstrapped session.
type authoring struct {
	appID  string
	envID  string
	cookie string
	csrf   string
}

func (ta *testApp) authoringSetup(t *testing.T) authoring {
	t.Helper()
	cookie, csrf := ta.bootstrap(t)
	app := ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "payments"}, cookie, csrf)
	if app.status != 201 {
		t.Fatalf("create app: %d %s", app.status, app.raw)
	}
	env := ta.do(t, "POST", "/api/v1/applications/"+app.body["id"].(string)+"/environments",
		map[string]string{"name": "production", "protection": "standard"}, cookie, csrf)
	if env.status != 201 {
		t.Fatalf("create env: %d %s", env.status, env.raw)
	}
	return authoring{
		appID: app.body["id"].(string), envID: env.body["id"].(string),
		cookie: cookie, csrf: csrf,
	}
}

func (ta *testApp) createSecret(t *testing.T, a authoring, name, value string) string {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/secrets",
		map[string]string{"name": name, "value": value}, a.cookie, a.csrf)
	if res.status != 201 {
		t.Fatalf("create secret: %d %s", res.status, res.raw)
	}
	return res.body["id"].(string)
}

func (ta *testApp) secretID(t *testing.T, a authoring) string {
	t.Helper()
	res := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/secrets",
		nil, a.cookie, "")
	if res.status != 200 {
		t.Fatalf("list secrets: %d %s", res.status, res.raw)
	}
	var rows []map[string]any
	_ = json.Unmarshal(res.raw, &rows)
	if len(rows) != 1 {
		t.Fatalf("expected one secret, got %d", len(rows))
	}
	return rows[0]["id"].(string)
}

// parsePage decodes a cursor envelope's items for tests.
func parsePage(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body.Items
}

func (ta *testApp) installCommand(t *testing.T, cookie, csrf, name string) string {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/nodes/install-command", map[string]string{"name": name}, cookie, csrf)
	if res.status != 201 {
		t.Fatalf("install command: %d %s", res.status, res.raw)
	}
	return res.body["command"].(string)
}

func tokenFromCommand(command string) string {
	parts := strings.Fields(command)
	for i, p := range parts {
		if p == "--token" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func secretPayloadValue(t *testing.T, plaintext []byte) string {
	t.Helper()
	var payload struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	content, _ := base64.StdEncoding.DecodeString(payload.Files[0].Content)
	return string(content)
}

func (ta *testApp) addVersion(t *testing.T, a authoring, secretID, value string) int64 {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/versions",
		map[string]string{"value": value}, a.cookie, a.csrf)
	if res.status != 201 {
		t.Fatalf("add version: %d %s", res.status, res.raw)
	}
	return int64(res.body["seq"].(float64))
}

func (ta *testApp) publish(t *testing.T, a authoring) string {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "publish the current draft"), a.cookie, a.csrf)
	if res.status != 201 {
		t.Fatalf("publish: %d %s", res.status, res.raw)
	}
	return res.body["id"].(string)
}

// publishAndAssign publishes and creates a Bundle Assignment for the group;
// the Assignment follows the Bundle's Desired Revision automatically.
func (ta *testApp) publishAndAssign(t *testing.T, a authoring, groupID string) string {
	t.Helper()
	revID := ta.publish(t, a)
	as := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	if as.status != 201 {
		t.Fatalf("assign: %d %s", as.status, as.raw)
	}
	return revID
}

func (ta *testApp) enrolledNode(t *testing.T, a authoring, name string) (nodeIdentity, string) {
	t.Helper()
	command := ta.installCommand(t, a.cookie, a.csrf, name)
	node := ta.enrollNode(t, tokenFromCommand(command), name)
	return node, node.nodeID
}

func (ta *testApp) doAgent(t *testing.T, method, path, serial string, body any) result {
	t.Helper()
	headers := map[string]string{"X-Autosecrets-Client-Cert": serial}
	return ta.doH(t, method, path, body, headers)
}
