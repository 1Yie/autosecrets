package envelope

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"filippo.io/age"
)

func mustIdentity(t *testing.T) *age.X25519Identity {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustSigner(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func testManifest() []byte {
	return []byte(`{"files":[{"gid":"0","mode":"0400","path":"app/token","sha256":"abc123","uid":"1000"}],"protocol":"autosecrets-manifest","version":"1"}`)
}

func testOptions(t *testing.T, ident *age.X25519Identity, priv ed25519.PrivateKey) Options {
	t.Helper()
	return Options{
		NodeID:       "node-1",
		RevisionID:   "rev-42",
		CreatedAt:    time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC),
		ExpiresAt:    time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Manifest:     testManifest(),
		Recipient:    ident.Recipient(),
		Signer:       priv,
		SigningKeyID: "core-signing-1",
	}
}

func optsWithPlaintext(o Options, plaintext []byte) Options {
	o.Plaintext = plaintext
	return o
}

func TestRoundTrip(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	plaintext := []byte("super-secret-token-value")

	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), plaintext))
	if err != nil {
		t.Fatal(err)
	}

	got, err := env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
	if err := env.VerifyManifest(testManifest()); err != nil {
		t.Fatalf("manifest verification failed: %v", err)
	}
}

func TestRoundTripSurvivesRemarshal(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var again Envelope
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	got, err := again.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x" {
		t.Fatalf("got %q", got)
	}
}

func TestExpiredEnvelope(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	opts := testOptions(t, ident, priv)
	opts.ExpiresAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	env, err := New(optsWithPlaintext(opts, []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	if _, err := env.Open(ident, pub, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("Open: got %v, want ErrExpired", err)
	}
	if err := env.Verify(pub, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify: got %v, want ErrExpired", err)
	}
}

func TestNoExpiryAllowsFutureOpen(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	opts := testOptions(t, ident, priv)
	opts.ExpiresAt = time.Time{}
	env, err := New(optsWithPlaintext(opts, []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Open(ident, pub, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("no-expiry envelope should open far in the future: %v", err)
	}
}

func TestBadSignatureRejected(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		t.Fatal(err)
	}
	sig[0] ^= 0xff
	env.Signature = base64.StdEncoding.EncodeToString(sig)
	if _, err := env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestWrongIdentityRejected(t *testing.T) {
	ident := mustIdentity(t)
	other := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Open(other, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("got %v, want ErrCiphertext", err)
	}
}

func TestWrongSignerRejected(t *testing.T) {
	ident := mustIdentity(t)
	pub, _ := mustSigner(t)
	_, otherPriv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, otherPriv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestUnsupportedProtocolVersionSuite(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	env.Protocol = "other"
	if _, err := env.Open(ident, pub, now); !errors.Is(err, ErrUnsupportedProtocol) {
		t.Fatalf("got %v, want ErrUnsupportedProtocol", err)
	}
	env.Protocol = ProtocolName
	env.Version = 2
	if _, err := env.Open(ident, pub, now); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("got %v, want ErrUnsupportedVersion", err)
	}
	env.Version = Version
	env.Suite = "aes-gcm"
	if _, err := env.Open(ident, pub, now); !errors.Is(err, ErrUnsupportedSuite) {
		t.Fatalf("got %v, want ErrUnsupportedSuite", err)
	}
}

func TestVerifyManifestMismatch(t *testing.T) {
	ident := mustIdentity(t)
	_, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.VerifyManifest([]byte(`{"files":[]}`)); err == nil {
		t.Fatal("mismatched manifest accepted")
	}
}

// TestCanonicalSignaturePayload is the cross-language anchor: both Go and Python
// must produce exactly these bytes before signing.
func TestCanonicalSignaturePayload(t *testing.T) {
	env := &Envelope{
		Protocol:       ProtocolName,
		Version:        Version,
		NodeID:         "node-1",
		RevisionID:     "rev-42",
		CreatedAt:      "2026-08-11T00:00:00Z",
		ExpiresAt:      "",
		ManifestSHA256: "deadbeef",
		Suite:          SuiteAgeX25519,
		Ciphertext:     "cipher",
		SigningKeyID:   "core-signing-1",
	}
	want := `{"ciphertext":"cipher","created_at":"2026-08-11T00:00:00Z","expires_at":"","manifest_sha256":"deadbeef","node_id":"node-1","protocol":"autosecrets-envelope","revision_id":"rev-42","signing_key_id":"core-signing-1","suite":"age-x25519","version":"1"}`
	got := string(env.signaturePayload())
	if got != want {
		t.Fatalf("signature payload mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestCanonicalManifestSortedKeys(t *testing.T) {
	got, err := CanonicalManifest([]FileSpec{{Path: "b", Mode: "0400", UID: "1", GID: "1", SHA256: "y"}, {Path: "a", Mode: "0600", UID: "2", GID: "2", SHA256: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"files":[{"gid":"2","mode":"0600","path":"a","sha256":"x","uid":"2"},{"gid":"1","mode":"0400","path":"b","sha256":"y","uid":"1"}],"protocol":"autosecrets-manifest","version":"1"}`
	if string(got) != want {
		t.Fatalf("manifest mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestInvalidCiphertextBase64(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	env.Ciphertext = "!!!not-base64!!!"
	if _, err := env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("invalid base64 ciphertext accepted")
	}
}

func TestVerifyRejectsShortSigningKeyWithoutPanic(t *testing.T) {
	ident := mustIdentity(t)
	_, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Open(ident, []byte("too-short"), time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestExpiryRejectsNonZOffset(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	env.ExpiresAt = "2026-08-12T00:00:00+00:00"
	if _, err := env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("non-Z expiry offset accepted")
	}
}

func TestErrorsNeverContainPlaintext(t *testing.T) {
	ident := mustIdentity(t)
	pub, priv := mustSigner(t)
	plaintext := []byte("TOP-SECRET-7f3a9c")
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), plaintext))
	if err != nil {
		t.Fatal(err)
	}
	sig, _ := base64.StdEncoding.DecodeString(env.Signature)
	sig[0] ^= 0xff
	env.Signature = base64.StdEncoding.EncodeToString(sig)
	_, err = env.Open(ident, pub, time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected failure")
	}
	if bytes.Contains([]byte(err.Error()), plaintext) {
		t.Fatalf("error leaked plaintext: %v", err)
	}
}

func TestEnvelopeJSONShape(t *testing.T) {
	ident := mustIdentity(t)
	_, priv := mustSigner(t)
	env, err := New(optsWithPlaintext(testOptions(t, ident, priv), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"protocol", "version", "node_id", "revision_id", "created_at", "expires_at", "manifest_sha256", "suite", "ciphertext", "signing_key_id", "signature"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("envelope JSON missing field %q: %s", key, data)
		}
	}
	if env.Signature == "" {
		t.Fatal("signature missing")
	}
}
