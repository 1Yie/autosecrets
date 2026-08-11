package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/envelope"
	"autosecrets.dev/core/internal/store"
	"filippo.io/age"
)

const testDSN = "postgres://autosecrets:test@localhost:55433/autosecrets"

type testApp struct {
	app    *App
	server *httptest.Server
	store  *store.Store
	client *http.Client
	signer *crypto.Signer
	ca     *crypto.CA
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = testDSN
	}
	st, err := store.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v); set TEST_DATABASE_URL to run integration tests", err)
	}
	t.Cleanup(st.Close)
	truncate(t, st)

	dir := t.TempDir()
	mk, err := crypto.LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := crypto.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.LoadOrCreateSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, trusted, _ := net.ParseCIDR("127.0.0.0/8")
	a := New(st, mk, ca, signer, "/api/v1", "/agent/v1", Options{
		Version:        "test",
		PublicAgentURL: "https://agent.test",
		ArtifactDir:    dir,
		TrustedProxy:   []*net.IPNet{trusted},
		CertHeader:     "X-Autosecrets-Client-Cert",
		Now:            func() time.Time { return time.Now() },
	})
	server := httptest.NewServer(a.Handler())
	t.Cleanup(server.Close)
	return &testApp{
		app: a, server: server, store: st,
		client: server.Client(), signer: signer, ca: ca,
	}
}

func truncate(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()
	_, err := st.Exec(ctx, `TRUNCATE admins, sessions, bootstrap_codes, audit_events,
		applications, environments, secrets, secret_versions, file_bindings,
		drafts, draft_selections, bundle_revisions, revision_files,
		nodes, node_groups, group_members, assignments, enrollment_tokens
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
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
	out := result{status: resp.StatusCode, raw: raw, header: resp.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (ta *testApp) bootstrap(t *testing.T) (cookie, csrf string) {
	t.Helper()
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil || code == "" {
		t.Fatalf("emit bootstrap code: %v", err)
	}
	res := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "username": "admin", "password": "correct-horse-42",
	}, "", "")
	if res.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", res.status, res.raw)
	}
	login := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if login.status != http.StatusOK {
		t.Fatalf("login: %d %s", login.status, login.raw)
	}
	cookie = ""
	for _, c := range login.header.Values("Set-Cookie") {
		if strings.HasPrefix(c, sessionCookie+"=") {
			cookie = strings.SplitN(strings.TrimPrefix(c, sessionCookie+"="), ";", 2)[0]
		}
	}
	if cookie == "" {
		t.Fatal("no session cookie")
	}
	csrf, _ = login.body["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("no csrf token")
	}
	return cookie, csrf
}

// --- Identity -------------------------------------------------------------

func TestBootstrapLifecycle(t *testing.T) {
	ta := newTestApp(t)
	me := ta.do(t, "GET", "/api/v1/me", nil, "", "")
	if me.status != 200 || me.body["bootstrap_required"] != true {
		t.Fatalf("fresh core must require bootstrap: %d %s", me.status, me.raw)
	}
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil || code == "" {
		t.Fatal(err)
	}
	bad := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": "wrong", "username": "admin", "password": "correct-horse-42",
	}, "", "")
	if bad.status != http.StatusForbidden {
		t.Fatalf("wrong code accepted: %d", bad.status)
	}
	ok := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "username": "admin", "password": "correct-horse-42",
	}, "", "")
	if ok.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", ok.status, ok.raw)
	}
	again := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "username": "other", "password": "correct-horse-42",
	}, "", "")
	if again.status != http.StatusConflict {
		t.Fatalf("second bootstrap must conflict: %d", again.status)
	}
	code2, _ := ta.app.EmitBootstrapCode(context.Background())
	if code2 != "" {
		t.Fatal("bootstrap code emitted after admin exists")
	}
}

func TestLoginLogoutAndCSRF(t *testing.T) {
	ta := newTestApp(t)
	cookie, csrf := ta.bootstrap(t)

	noCSRF := ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "x"}, cookie, "")
	if noCSRF.status != http.StatusForbidden {
		t.Fatalf("mutation without CSRF accepted: %d", noCSRF.status)
	}
	badCSRF := ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "x"}, cookie, "deadbeef")
	if badCSRF.status != http.StatusForbidden {
		t.Fatalf("mutation with wrong CSRF accepted: %d", badCSRF.status)
	}
	ok := ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "web"}, cookie, csrf)
	if ok.status != http.StatusCreated {
		t.Fatalf("create with CSRF: %d %s", ok.status, ok.raw)
	}
	denied := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "wrong-password-42",
	}, "", "")
	if denied.status != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", denied.status)
	}
	logout := ta.do(t, "POST", "/api/v1/auth/logout", nil, cookie, csrf)
	if logout.status != http.StatusOK {
		t.Fatalf("logout: %d", logout.status)
	}
	stale := ta.do(t, "GET", "/api/v1/applications", nil, cookie, "")
	if stale.status != http.StatusUnauthorized {
		t.Fatalf("session survived logout: %d", stale.status)
	}
}

// --- Authoring ------------------------------------------------------------

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
		map[string]string{"name": "production"}, cookie, csrf)
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

func TestAuthoringFlow(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)

	secretID := ta.createSecret(t, a, "db_token", "s3cret-value-1")
	if secretID == "" {
		t.Fatal("no secret id")
	}

	// Default binding: filename equals the Secret name, 0400.
	list := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/secrets",
		nil, a.cookie, "")
	var rows []map[string]any
	if err := json.Unmarshal(list.raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 secret, got %d", len(rows))
	}
	row := rows[0]
	binding := row["binding"].(map[string]any)
	if binding["path"] != "db_token" || binding["mode"] != "0400" {
		t.Fatalf("default binding wrong: %v", binding)
	}

	// Binding validation: absolute path, .., dot, and unsafe mode rejected.
	for _, tc := range []map[string]any{
		{"path": "/etc/passwd", "mode": "0400"},
		{"path": "../escape", "mode": "0400"},
		{"path": "a/./b", "mode": "0400"},
		{"path": "ok/path", "mode": "0777"},
	} {
		res := ta.do(t, "PUT", "/api/v1/secrets/"+secretID+"/binding", tc, a.cookie, a.csrf)
		if res.status != http.StatusBadRequest {
			t.Fatalf("binding %v accepted: %d %s", tc, res.status, res.raw)
		}
	}
	good := ta.do(t, "PUT", "/api/v1/secrets/"+secretID+"/binding",
		map[string]any{"path": "app/token", "uid": 1000, "gid": 1000, "mode": "0600"},
		a.cookie, a.csrf)
	if good.status != http.StatusOK {
		t.Fatalf("valid binding rejected: %d %s", good.status, good.raw)
	}

	// Rotation: new version bumps the Draft and selects it.
	rot := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/versions",
		map[string]string{"value": "s3cret-value-2"}, a.cookie, a.csrf)
	if rot.status != 201 || rot.body["seq"].(float64) != 2 {
		t.Fatalf("rotate: %d %s", rot.status, rot.raw)
	}
	draft := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		nil, a.cookie, "")
	if draft.status != 200 {
		t.Fatalf("draft: %d %s", draft.status, draft.raw)
	}
	version := draft.body["version"].(float64)
	selections := draft.body["selections"].([]any)
	if len(selections) != 1 {
		t.Fatalf("draft selections: %d", len(selections))
	}
	sel := selections[0].(map[string]any)
	if sel["version_seq"].(float64) != 2 || sel["name"] != "db_token" {
		t.Fatalf("selection wrong: %v", sel)
	}

	// Optimistic concurrency: a stale If-Match must conflict.
	authH := map[string]string{"If-Match": fmtInt(version), "Cookie": sessionCookie + "=" + a.cookie, csrfHeader: a.csrf}
	current := ta.doH(t, "PUT", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		map[string]any{"selections": map[string]any{secretID: 1}}, authH)
	if current.status != http.StatusOK {
		t.Fatalf("draft update: %d %s", current.status, current.raw)
	}
	stale := ta.doH(t, "PUT", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		map[string]any{"selections": map[string]any{secretID: 1}}, authH)
	if stale.status != http.StatusConflict {
		t.Fatalf("stale draft update must conflict: %d", stale.status)
	}

	pub := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		nil, a.cookie, a.csrf)
	if pub.status != 201 {
		t.Fatalf("publish: %d %s", pub.status, pub.raw)
	}
	revs := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/revisions",
		nil, a.cookie, "")
	var revList []any
	if err := json.Unmarshal(revs.raw, &revList); err != nil {
		t.Fatal(err)
	}
	if len(revList) != 1 {
		t.Fatalf("revisions: %s", revs.raw)
	}
}

func fmtInt(n float64) string { return strconv.FormatInt(int64(n), 10) }

// --- Fleet ----------------------------------------------------------------

func TestNodeGroupAndAssignment(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "token", "v")
	pub := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		nil, a.cookie, a.csrf)
	revisionID := pub.body["id"].(string)

	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "web-tier"}, a.cookie, a.csrf)
	if g.status != 201 {
		t.Fatalf("group: %d %s", g.status, g.raw)
	}
	groupID := g.body["id"].(string)
	as := ta.do(t, "POST", "/api/v1/assignments",
		map[string]string{"group_id": groupID, "revision_id": revisionID}, a.cookie, a.csrf)
	if as.status != 201 {
		t.Fatalf("assignment: %d %s", as.status, as.raw)
	}
	dup := ta.do(t, "POST", "/api/v1/assignments",
		map[string]string{"group_id": groupID, "revision_id": revisionID}, a.cookie, a.csrf)
	if dup.status != http.StatusConflict {
		t.Fatalf("duplicate assignment must 409: %d", dup.status)
	}
}

// --- Enrollment and delivery ----------------------------------------------

type nodeIdentity struct {
	ageID  *age.X25519Identity
	serial string
	nodeID string
}

func (ta *testApp) enrollNode(t *testing.T, token, name string) nodeIdentity {
	t.Helper()
	ageID, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	if err != nil {
		t.Fatal(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	res := ta.do(t, "POST", "/agent/v1/enroll", map[string]string{
		"token": token, "name": name,
		"age_pubkey": ageID.Recipient().String(), "csr": string(csrPEM),
	}, "", "")
	if res.status != 201 {
		t.Fatalf("enroll: %d %s", res.status, res.raw)
	}
	certBlock, _ := pem.Decode([]byte(res.body["cert_pem"].(string)))
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return nodeIdentity{
		ageID: ageID, serial: cert.SerialNumber.Text(16),
		nodeID: res.body["node_id"].(string),
	}
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

func (ta *testApp) pollDesired(t *testing.T, node nodeIdentity, etag string) (int, desiredResponse) {
	t.Helper()
	headers := map[string]string{"X-Autosecrets-Client-Cert": node.serial}
	if etag != "" {
		headers["If-None-Match"] = etag
	}
	res := ta.doH(t, "GET", "/agent/v1/desired", nil, headers)
	var out desiredResponse
	_ = json.Unmarshal(res.raw, &out)
	return res.status, out
}

func TestInstallCommandAndEnrollment(t *testing.T) {
	ta := newTestApp(t)
	cookie, csrf := ta.bootstrap(t)
	command := ta.installCommand(t, cookie, csrf, "web-1")
	if !strings.Contains(command, "install.sh") || !strings.Contains(command, "--server https://agent.test") {
		t.Fatalf("command shape: %s", command)
	}
	token := tokenFromCommand(command)
	if token == "" {
		t.Fatal("no token in command")
	}
	ta.enrollNode(t, token, "web-1")

	// Token single-use: a second enrollment with the same token fails.
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	ageID2, _ := age.GenerateX25519Identity()
	again := ta.do(t, "POST", "/agent/v1/enroll", map[string]string{
		"token": token, "name": "web-2",
		"age_pubkey": ageID2.Recipient().String(), "csr": string(csrPEM),
	}, "", "")
	if again.status != http.StatusForbidden {
		t.Fatalf("token reuse accepted: %d", again.status)
	}

	nodes := ta.do(t, "GET", "/api/v1/nodes", nil, cookie, "")
	var nodeList []map[string]any
	if err := json.Unmarshal(nodes.raw, &nodeList); err != nil {
		t.Fatal(err)
	}
	if len(nodeList) != 1 {
		t.Fatalf("nodes: %s", nodes.raw)
	}
}

func TestDesiredStateDeliveryAndRedaction(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	secretValue := "super-secret-db-password"
	ta.createSecret(t, a, "db_pass", secretValue)
	pub := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		nil, a.cookie, a.csrf)
	revisionID := pub.body["id"].(string)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	ta.do(t, "POST", "/api/v1/assignments",
		map[string]string{"group_id": g.body["id"].(string), "revision_id": revisionID}, a.cookie, a.csrf)

	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")
	ta.do(t, "POST", "/api/v1/node-groups/"+g.body["id"].(string)+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)

	status, desired := ta.pollDesired(t, node, "")
	if status != 200 || len(desired.Envelopes) != 1 {
		t.Fatalf("desired: %d %+v", status, desired)
	}
	env := desired.Envelopes[0]
	if env.NodeID != node.nodeID || env.RevisionID != revisionID {
		t.Fatalf("envelope meta: %+v", env)
	}
	if err := env.Verify(ta.signer.PublicKey(), time.Now()); err != nil {
		t.Fatalf("envelope verify: %v", err)
	}
	if strings.Contains(env.Ciphertext, secretValue) {
		t.Fatal("secret plaintext leaked into envelope ciphertext")
	}
	plaintext := decryptEnvelope(t, env, node.ageID)
	var payload struct {
		AppID string `json:"app_id"`
		EnvID string `json:"env_id"`
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.AppID != a.appID || payload.EnvID != a.envID || len(payload.Files) != 1 {
		t.Fatalf("payload: %+v", payload)
	}
	if payload.Files[0].Path != "db_pass" {
		t.Fatalf("payload path: %s", payload.Files[0].Path)
	}
	content, _ := base64.StdEncoding.DecodeString(payload.Files[0].Content)
	if string(content) != secretValue {
		t.Fatalf("payload content mismatch")
	}
	// Manifest hash must match the envelope.
	sum := sha256.Sum256(mustManifest(t, env, node))
	if hex.EncodeToString(sum[:]) != env.ManifestSHA256 {
		t.Fatal("manifest hash mismatch")
	}

	// ETag: second poll with the same tag is 304.
	status2, _ := ta.pollDesired(t, node, desired.ETag)
	if status2 != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", status2)
	}

	// Activation report + heartbeat update node state.
	report := ta.doH(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/reports", map[string]string{
		"revision_id": revisionID, "stage": "activate", "result": "ok",
	}, map[string]string{"X-Autosecrets-Client-Cert": node.serial})
	if report.status != 200 {
		t.Fatalf("report: %d %s", report.status, report.raw)
	}
	nodes := ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	var nodeList []map[string]any
	if err := json.Unmarshal(nodes.raw, &nodeList); err != nil {
		t.Fatal(err)
	}
	if len(nodeList) != 1 {
		t.Fatalf("nodes: %s", nodes.raw)
	}
	n := nodeList[0]
	if n["observed_revision"] != revisionID || n["last_result"] != "ok" {
		t.Fatalf("node state: %v", n)
	}

	// Redaction: the secret value must not appear in audit events.
	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, a.cookie, "")
	if strings.Contains(string(audit.raw), secretValue) {
		t.Fatal("secret value leaked into audit events")
	}
}

func mustManifest(t *testing.T, env *envelope.Envelope, node nodeIdentity) []byte {
	t.Helper()
	plaintext := decryptEnvelope(t, env, node.ageID)
	var payload struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	_ = json.Unmarshal(plaintext, &payload)
	files := []envelope.FileSpec{}
	for _, f := range payload.Files {
		content, _ := base64.StdEncoding.DecodeString(f.Content)
		sum := sha256.Sum256(content)
		files = append(files, envelope.FileSpec{
			Path: f.Path, Mode: "0400", UID: "0", GID: "0",
			SHA256: hex.EncodeToString(sum[:]),
		})
	}
	out, err := envelope.CanonicalManifest(files)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func decryptEnvelope(t *testing.T, env *envelope.Envelope, id *age.X25519Identity) []byte {
	t.Helper()
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

func (ta *testApp) doAgent(t *testing.T, method, path, serial string, body any) result {
	t.Helper()
	headers := map[string]string{"X-Autosecrets-Client-Cert": serial}
	return ta.doH(t, method, path, body, headers)
}

// --- Artifacts and install script -----------------------------------------

func TestInstallScriptAndArtifacts(t *testing.T) {
	ta := newTestApp(t)
	script := ta.do(t, "GET", "/agent/v1/install.sh", nil, "", "")
	if script.status != 200 || !strings.Contains(string(script.raw), "pkeyutl -verify") {
		t.Fatalf("install script: %d", script.status)
	}
	pubPEM, err := ta.signer.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script.raw), string(pubPEM)) {
		t.Fatal("install script does not embed the signing public key")
	}

	// Signed artifact round trip.
	artifact := []byte("#!/bin/sh\necho fake-agent\n")
	artifactPath := filepath.Join(ta.app.cfg.ArtifactDir, "autosecrets-agent-linux-amd64.tar.gz")
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	sig := ta.signer.Sign(artifact)
	if err := os.WriteFile(artifactPath+".sig", sig, 0o600); err != nil {
		t.Fatal(err)
	}
	got := ta.do(t, "GET", "/agent/v1/artifacts/autosecrets-agent-linux-amd64.tar.gz", nil, "", "")
	if got.status != 200 || !bytes.Equal(got.raw, artifact) {
		t.Fatalf("artifact: %d", got.status)
	}
	gotSig := ta.do(t, "GET", "/agent/v1/artifacts/autosecrets-agent-linux-amd64.tar.gz.sig", nil, "", "")
	if gotSig.status != 200 || !ed25519.Verify(ta.signer.PublicKey(), artifact, gotSig.raw) {
		t.Fatal("artifact signature does not verify")
	}
	traversal := ta.do(t, "GET", "/agent/v1/artifacts/..%2f..%2fetc%2fpasswd", nil, "", "")
	if traversal.status != http.StatusNotFound {
		t.Fatalf("path traversal: %d", traversal.status)
	}
}

func TestAgentIdentityBoundary(t *testing.T) {
	ta := newTestApp(t)
	res := ta.doH(t, "GET", "/agent/v1/desired", nil, nil)
	if res.status != http.StatusForbidden {
		t.Fatalf("unauthenticated agent route: %d", res.status)
	}
	res2 := ta.doAgent(t, "GET", "/agent/v1/desired", "deadbeef", nil)
	if res2.status != http.StatusForbidden {
		t.Fatalf("unknown serial: %d", res2.status)
	}
}
