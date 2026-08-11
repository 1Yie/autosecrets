// Package envelope implements the versioned Agent envelope protocol: a JSON
// object carrying a Secret bundle payload encrypted to one Managed Node's age
// X25519 key and signed by a Core Ed25519 key. Go (Core) and Python (Agent)
// must produce identical canonical signature payloads; see
// api/agent-envelope/envelope-v1.md and the checked-in test vectors.
package envelope

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
)

const (
	ProtocolName   = "autosecrets-envelope"
	Version        = 1
	SuiteAgeX25519 = "age-x25519"
)

var (
	ErrUnsupportedProtocol = errors.New("envelope: unsupported protocol")
	ErrUnsupportedVersion  = errors.New("envelope: unsupported version")
	ErrUnsupportedSuite    = errors.New("envelope: unsupported encryption suite")
	ErrExpired             = errors.New("envelope: expired")
	ErrBadSignature        = errors.New("envelope: bad signature")
	ErrCiphertext          = errors.New("envelope: ciphertext could not be decrypted")
	ErrBadManifest         = errors.New("envelope: manifest hash mismatch")
)

// Envelope is the wire format defined in api/agent-envelope/envelope-v1.md.
type Envelope struct {
	Protocol       string `json:"protocol"`
	Version        int    `json:"version"`
	NodeID         string `json:"node_id"`
	RevisionID     string `json:"revision_id"`
	CreatedAt      string `json:"created_at"`
	ExpiresAt      string `json:"expires_at"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Suite          string `json:"suite"`
	Ciphertext     string `json:"ciphertext"`
	SigningKeyID   string `json:"signing_key_id"`
	Signature      string `json:"signature"`
}

// Options configures New. CreatedAt may be zero to use the current time;
// ExpiresAt zero means the envelope never expires.
type Options struct {
	NodeID       string
	RevisionID   string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Manifest     []byte
	Plaintext    []byte
	Recipient    *age.X25519Recipient
	Signer       ed25519.PrivateKey
	SigningKeyID string
}

// FileSpec describes one file a Bundle Revision materializes. All values are
// strings so Go and Python serialize them identically.
type FileSpec struct {
	Path   string
	Mode   string
	UID    string
	GID    string
	SHA256 string
}

// signaturePayload returns the canonical bytes covered by the Ed25519
// signature: every envelope field except signature, alphabetically sorted,
// no whitespace, all values strings.
func (e *Envelope) signaturePayload() []byte {
	fields := map[string]string{
		"ciphertext":      e.Ciphertext,
		"created_at":      e.CreatedAt,
		"expires_at":      e.ExpiresAt,
		"manifest_sha256": e.ManifestSHA256,
		"node_id":         e.NodeID,
		"protocol":        e.Protocol,
		"revision_id":     e.RevisionID,
		"signing_key_id":  e.SigningKeyID,
		"suite":           e.Suite,
		"version":         strconv.Itoa(e.Version),
	}
	// encoding/json marshals map keys in sorted order, matching Python's
	// json.dumps(sort_keys=True, separators=(",", ":")).
	data, _ := json.Marshal(fields)
	return data
}

// New encrypts Plaintext to Recipient and signs the resulting envelope with
// Signer. It never logs or persists plaintext.
func New(o Options) (*Envelope, error) {
	if len(o.Manifest) == 0 {
		return nil, errors.New("envelope: manifest required")
	}
	if o.Recipient == nil {
		return nil, errors.New("envelope: recipient required")
	}
	if len(o.Signer) != ed25519.PrivateKeySize {
		return nil, errors.New("envelope: signer required")
	}

	var ciphertext bytes.Buffer
	writer, err := age.Encrypt(&ciphertext, o.Recipient)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(o.Plaintext); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	created := o.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	expires := ""
	if !o.ExpiresAt.IsZero() {
		expires = o.ExpiresAt.UTC().Format(time.RFC3339)
	}
	manifestSum := sha256.Sum256(o.Manifest)

	env := &Envelope{
		Protocol:       ProtocolName,
		Version:        Version,
		NodeID:         o.NodeID,
		RevisionID:     o.RevisionID,
		CreatedAt:      created.UTC().Format(time.RFC3339),
		ExpiresAt:      expires,
		ManifestSHA256: hex.EncodeToString(manifestSum[:]),
		Suite:          SuiteAgeX25519,
		Ciphertext:     base64.StdEncoding.EncodeToString(ciphertext.Bytes()),
		SigningKeyID:   o.SigningKeyID,
	}
	signature := ed25519.Sign(o.Signer, env.signaturePayload())
	env.Signature = base64.StdEncoding.EncodeToString(signature)
	return env, nil
}

func (e *Envelope) checkMeta() error {
	if e.Protocol != ProtocolName {
		return ErrUnsupportedProtocol
	}
	if e.Version != Version {
		return ErrUnsupportedVersion
	}
	if e.Suite != SuiteAgeX25519 {
		return ErrUnsupportedSuite
	}
	return nil
}

func (e *Envelope) checkExpiry(now time.Time) error {
	if e.ExpiresAt == "" {
		return nil
	}
	// Both implementations accept exactly "2006-01-02T15:04:05Z" (UTC, Z
	// suffix only) so a Core-produced envelope always parses on the Agent.
	expiry, err := time.Parse("2006-01-02T15:04:05Z", e.ExpiresAt)
	if err != nil {
		return fmt.Errorf("envelope: invalid expires_at: %w", err)
	}
	if now.After(expiry) {
		return ErrExpired
	}
	return nil
}

// Verify checks protocol, version, suite, expiry, and the Ed25519 signature
// without decrypting the payload.
func (e *Envelope) Verify(verifyKey ed25519.PublicKey, now time.Time) error {
	if err := e.checkMeta(); err != nil {
		return err
	}
	if err := e.checkExpiry(now); err != nil {
		return err
	}
	if len(verifyKey) != ed25519.PublicKeySize {
		return ErrBadSignature
	}
	signature, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil {
		return ErrBadSignature
	}
	if !ed25519.Verify(verifyKey, e.signaturePayload(), signature) {
		return ErrBadSignature
	}
	return nil
}

// Open verifies the envelope and decrypts the payload with identity.
func (e *Envelope) Open(identity age.Identity, verifyKey ed25519.PublicKey, now time.Time) ([]byte, error) {
	if err := e.Verify(verifyKey, now); err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(e.Ciphertext)
	if err != nil {
		return nil, ErrCiphertext
	}
	reader, err := age.Decrypt(strings.NewReader(string(ciphertext)), identity)
	if err != nil {
		return nil, ErrCiphertext
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return nil, ErrCiphertext
	}
	return plaintext, nil
}

// VerifyManifest checks that manifest hashes to the envelope's
// manifest_sha256.
func (e *Envelope) VerifyManifest(manifest []byte) error {
	sum := sha256.Sum256(manifest)
	if hex.EncodeToString(sum[:]) != e.ManifestSHA256 {
		return ErrBadManifest
	}
	return nil
}

// CanonicalManifest serializes FileSpecs in the canonical manifest form:
// sorted top-level keys, files sorted by path, every value a string.
func CanonicalManifest(files []FileSpec) ([]byte, error) {
	sorted := append([]FileSpec(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	entries := make([]map[string]string, 0, len(sorted))
	for _, f := range sorted {
		entries = append(entries, map[string]string{
			"gid":    f.GID,
			"mode":   f.Mode,
			"path":   f.Path,
			"sha256": f.SHA256,
			"uid":    f.UID,
		})
	}
	manifest := map[string]any{
		"files":    entries,
		"protocol": "autosecrets-manifest",
		"version":  "1",
	}
	return json.Marshal(manifest)
}
