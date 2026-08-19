package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestMasterKeySealOpenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mk, err := LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	value := []byte("s3cret-value-with-binary\x00\x01\x02")
	wrapped, nonces, ct, err := mk.Seal(value)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mk.Open(wrapped, nonces, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, value) {
		t.Fatalf("round trip mismatch: %q != %q", got, value)
	}
	// Tampered ciphertext must fail.
	ct[0] ^= 0xff
	if _, err := mk.Open(wrapped, nonces, ct); err == nil {
		t.Fatal("tampered ciphertext decrypted successfully")
	}
}

func TestMasterKeyPersists(t *testing.T) {
	dir := t.TempDir()
	mk1, err := LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, nonces, ct, err := mk1.Seal([]byte("persisted"))
	if err != nil {
		t.Fatal(err)
	}
	mk2, err := LoadOrCreateMasterKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mk2.Open(wrapped, nonces, ct)
	if err != nil || string(got) != "persisted" {
		t.Fatalf("reloaded key cannot open: %q %v", got, err)
	}
}

func TestPasswordHashVerify(t *testing.T) {
	phc, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := VerifyPassword("correct horse", phc); !ok {
		t.Fatal("correct password rejected")
	}
	if ok, _ := VerifyPassword("wrong horse", phc); ok {
		t.Fatal("wrong password accepted")
	}
	if _, err := VerifyPassword("x", "not-a-phc"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}

func TestHashTokenStable(t *testing.T) {
	if HashToken("abc") != HashToken("abc") {
		t.Fatal("hash not stable")
	}
	if HashToken("abc") == HashToken("abd") {
		t.Fatal("collision")
	}
	if len(HashToken("x")) != 64 {
		t.Fatal("expected sha256 hex")
	}
}

func TestCAIssuesAgentCert(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Build a CSR with an ephemeral key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	if err != nil {
		t.Fatal(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	certPEM, serial, expiresAt, err := ca.IssueAgentCert("node-1", csrPEM, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if serial == "" || expiresAt.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("bad cert metadata: serial=%q expires=%v", serial, expiresAt)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("no cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := cert.CheckSignatureFrom(ca.cert); err != nil {
		t.Fatalf("cert not signed by CA: %v", err)
	}
	if cert.Subject.CommonName != "node:node-1" {
		t.Fatalf("unexpected CN %q", cert.Subject.CommonName)
	}
	parsed, err := ca.ParseAgentCert(certPEM, time.Now())
	if err != nil || parsed.SerialNumber.Text(16) != serial {
		t.Fatalf("ParseAgentCert: %v serial=%s", err, serial)
	}
}

func TestSignerSignVerify(t *testing.T) {
	dir := t.TempDir()
	s1, err := LoadOrCreateSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("envelope payload")
	sig := s1.Sign(msg)
	if !ed25519.Verify(s1.PublicKey(), msg, sig) {
		t.Fatal("signature does not verify")
	}
	pemBytes, err := s1.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("no public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	recovered, ok := pub.(ed25519.PublicKey)
	if !ok || !ed25519.Verify(recovered, msg, sig) {
		t.Fatal("public key PEM does not verify signature")
	}
}
