package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/envelope"
	"filippo.io/age"
)

// desiredResponse mirrors the fleet desired-state response shape the test
// unmarshals; the concrete type lives in the fleet package.
type desiredResponse struct {
	ETag      string               `json:"etag"`
	Envelopes []*envelope.Envelope `json:"envelopes"`
}

// --- Desired State delivery -----------------------------------------------

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

func TestDesiredStateDeliveryAndRedaction(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	secretValue := "super-secret-db-password"
	ta.createSecret(t, a, "db_pass", secretValue)
	pub := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "deliver the initial secret"), a.cookie, a.csrf)
	revisionID := pub.body["id"].(string)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	ta.do(t, "POST", "/api/v1/assignments", assignBody(a, g.body["id"].(string)), a.cookie, a.csrf)

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
	nodeList := parsePage(t, nodes.raw)
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
