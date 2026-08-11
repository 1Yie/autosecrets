package envelope

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age"
)

// interopPayload is exchanged with the Python side across the subprocess
// boundary in both directions.
type interopPayload struct {
	Envelope         Envelope `json:"envelope"`
	RecipientPublic  string   `json:"recipient_public"`
	RecipientPrivate string   `json:"recipient_private"`
	SigningPublic    string   `json:"signing_public"`
	SigningPrivate   string   `json:"signing_private"`
	Manifest         string   `json:"manifest"`
	Plaintext        string   `json:"plaintext"`
}

// TestPythonInteropRoundTrip is the phase-0 risk gate: Go encrypts and signs an
// envelope, the Python Agent implementation verifies and decrypts it, then
// Python creates its own envelope that Go verifies and decrypts.
func TestPythonInteropRoundTrip(t *testing.T) {
	root := repoRootDir(t)
	pythonBin := os.Getenv("PYTHON_BIN")
	if pythonBin == "" {
		pythonBin = filepath.Join(root, "agent", ".venv", "bin", "python")
	}
	if _, err := os.Stat(pythonBin); err != nil {
		t.Skip("python venv not available; set PYTHON_BIN")
	}

	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	manifest := testManifest()
	plaintext := []byte("go->python round trip payload")
	opts := testOptions(t, ident, priv)
	opts.ExpiresAt = time.Now().UTC().Add(time.Hour)
	env, err := New(optsWithPlaintext(opts, plaintext))
	if err != nil {
		t.Fatal(err)
	}

	in := interopPayload{
		Envelope: *env, RecipientPublic: ident.Recipient().String(),
		RecipientPrivate: ident.String(), SigningPublic: base64.StdEncoding.EncodeToString(pub),
		SigningPrivate: base64.StdEncoding.EncodeToString(priv), Manifest: string(manifest),
		Plaintext: base64.StdEncoding.EncodeToString(plaintext),
	}
	input, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	inPath := filepath.Join(t.TempDir(), "input.json")
	outPath := filepath.Join(t.TempDir(), "output.json")
	if err := os.WriteFile(inPath, input, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(pythonBin, "-m", "autosecrets_agent.interop", "roundtrip", inPath, outPath)
	cmd.Dir = filepath.Join(root, "agent")
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "agent", "src"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("python interop failed: %v\nstderr:\n%s", err, stderr.String())
	}

	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var outPayload interopPayload
	if err := json.Unmarshal(out, &outPayload); err != nil {
		t.Fatal(err)
	}
	pyIdentity, err := age.ParseX25519Identity(outPayload.RecipientPrivate)
	if err != nil {
		t.Fatal(err)
	}
	pyPub, err := base64.StdEncoding.DecodeString(outPayload.SigningPublic)
	if err != nil {
		t.Fatal(err)
	}
	got, err := outPayload.Envelope.Open(pyIdentity, pyPub, time.Now().UTC())
	if err != nil {
		t.Fatalf("go failed to open python envelope: %v", err)
	}
	if string(got) != "python->go round trip payload" {
		t.Fatalf("python envelope plaintext mismatch: %q", got)
	}
	if err := outPayload.Envelope.VerifyManifest([]byte(outPayload.Manifest)); err != nil {
		t.Fatalf("python envelope manifest mismatch: %v", err)
	}
}
