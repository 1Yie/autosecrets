package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

// --- Enrollment and artifacts ---------------------------------------------

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

// TestInstallCommandWithCurlOpts proves the LAN/dev mode: the generated
// command carries the AUTOSECRETS_CURL_OPTS assignment so it runs verbatim
// against a self-signed dev endpoint.
func TestInstallCommandWithCurlOpts(t *testing.T) {
	ta := newTestApp(t, func(o *Options) { o.InstallCurlOpts = "AUTOSECRETS_CURL_OPTS='-k'" })
	cookie, csrf := ta.bootstrap(t)
	command := ta.installCommand(t, cookie, csrf, "web-1")
	if !strings.HasPrefix(command, "curl -k -fsSL") {
		t.Fatalf("command must skip TLS verification: %s", command)
	}
	if !strings.HasSuffix(command, "--insecure") {
		t.Fatalf("command must pass --insecure to install.sh: %s", command)
	}
	if !strings.Contains(command, "--server https://agent.test") {
		t.Fatalf("command shape: %s", command)
	}
}

func TestInstallCommandWithBundleDir(t *testing.T) {
	ta := newTestApp(t)
	cookie, csrf := ta.bootstrap(t)
	res := ta.do(t, "POST", "/api/v1/nodes/install-command", map[string]any{
		"name": "web-1", "bundle_dir": "~/.autosecrets",
	}, cookie, csrf)
	if res.status != 201 {
		t.Fatalf("install command: %d %s", res.status, res.raw)
	}
	command := res.body["command"].(string)
	if !strings.Contains(command, `--bundle-dir "~/.autosecrets"`) {
		t.Fatalf("command must carry bundle dir: %s", command)
	}
	bad := ta.do(t, "POST", "/api/v1/nodes/install-command", map[string]any{
		"name": "web-1", "bundle_dir": "relative/path",
	}, cookie, csrf)
	if bad.status != 400 || bad.body["code"] != "bad_request" {
		t.Fatalf("relative bundle_dir must be rejected: %d %s", bad.status, bad.raw)
	}
}

func TestInstallScriptAndArtifacts(t *testing.T) {
	ta := newTestApp(t)
	script := ta.do(t, "GET", "/agent/v1/install.sh", nil, "", "")
	if script.status != 200 || !strings.Contains(string(script.raw), "pkeyutl -verify") {
		t.Fatalf("install script: %d", script.status)
	}
	// Default Materialized Bundle location is ~/.autosecrets (legacy layout).
	if !strings.Contains(string(script.raw), "BUNDLE_DIR=\"~/.autosecrets\"") {
		t.Fatal("install script must default bundle dir to ~/.autosecrets")
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
