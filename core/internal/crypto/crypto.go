// Package crypto owns Core's key material and cryptographic primitives: the
// data master key with wrapped per-version data keys, password hashing, token
// hashing, the internal Agent CA, and the Core signing key. Keys live on the
// filesystem outside PostgreSQL (ADR-0003) with strict permissions.
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	masterKeyFile  = "master.key"
	caKeyFile      = "agent-ca.key"
	caCertFile     = "agent-ca.crt"
	signingKeyFile = "core-signing.key"
)

// --- master key and AEAD --------------------------------------------------

// MasterKey is the AES-256 data master key kept outside PostgreSQL.
type MasterKey struct {
	key []byte
}

// LoadOrCreateMasterKey loads the master key file or creates it with 0600
// permissions when absent.
func LoadOrCreateMasterKey(dir string) (*MasterKey, error) {
	path := filepath.Join(dir, masterKeyFile)
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := writeKeyFile(path, key); err != nil {
			return nil, err
		}
		return &MasterKey{key: key}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: master key file %s must be 32 bytes", path)
	}
	return &MasterKey{key: key}, nil
}

// Seal encrypts value with a fresh random data key, wraps the data key with
// the master key, and returns (wrappedKey, wrapNonce, ciphertext).
func (m *MasterKey) Seal(value []byte) (wrappedKey, wrapNonce, ciphertext []byte, err error) {
	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, nil, nil, err
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return nil, nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	wrapNonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(wrapNonce); err != nil {
		return nil, nil, nil, err
	}
	wrappedKey = aead.Seal(nil, wrapNonce, dataKey, nil)

	dataBlock, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, nil, nil, err
	}
	dataAEAD, err := cipher.NewGCM(dataBlock)
	if err != nil {
		return nil, nil, nil, err
	}
	dataNonce := make([]byte, dataAEAD.NonceSize())
	if _, err := rand.Read(dataNonce); err != nil {
		return nil, nil, nil, err
	}
	ciphertext = dataAEAD.Seal(nil, dataNonce, value, nil)
	return wrappedKey, append(wrapNonce, dataNonce...), ciphertext, nil
}

// Open reverses Seal.
func (m *MasterKey) Open(wrappedKey, nonces, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonces) < aead.NonceSize() {
		return nil, errors.New("crypto: truncated nonce")
	}
	wrapNonce, dataNonce := nonces[:aead.NonceSize()], nonces[aead.NonceSize():]
	dataKey, err := aead.Open(nil, wrapNonce, wrappedKey, nil)
	if err != nil {
		return nil, err
	}
	dataBlock, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	dataAEAD, err := cipher.NewGCM(dataBlock)
	if err != nil {
		return nil, err
	}
	if len(dataNonce) != dataAEAD.NonceSize() {
		return nil, errors.New("crypto: truncated data nonce")
	}
	return dataAEAD.Open(nil, dataNonce, ciphertext, nil)
}

// --- password and token hashing -------------------------------------------

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword returns an argon2id PHC string.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a PHC string produced by HashPassword. Constant-time
// comparison of the derived key prevents timing side channels.
func VerifyPassword(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("crypto: malformed password hash")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", new(int)); err != nil {
		return false, errors.New("crypto: malformed password hash")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, errors.New("crypto: malformed password hash")
	}
	saltB64, keyB64 := parts[4], parts[5]
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(keyB64)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtleConstantTimeEqual(got, want), nil
}

func subtleConstantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// HashToken returns the lowercase hex SHA-256 of a secret token; only hashes
// are ever persisted.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewSecret returns a URL-safe random token suitable for session IDs,
// bootstrap codes, and enrollment tokens.
func NewSecret(bits int) (string, error) {
	n := bits / 8
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// PasswordValid applies the member password policy without logging or
// retaining the supplied password. The local denylist catches the most
// common leaked values while keeping the check independent of a third party.
func PasswordValid(password string) bool {
	if utf8.RuneCountInString(password) < 12 || utf8.RuneCountInString(password) > 128 {
		return false
	}
	_, common := commonPasswords[strings.ToLower(password)]
	return !common
}

var commonPasswords = map[string]struct{}{
	"password":            {},
	"password123":         {},
	"123456789012":        {},
	"qwertyuiop":          {},
	"letmeinletmein":      {},
	"autosecrets":         {},
	"changemechangeme":    {},
	"administrator":       {},
	"correcthorsebattery": {},
}

const (
	totpPeriod = 30 * time.Second
	totpDigits = 6
)

// NewTOTPSecret returns an RFC 4648 base32 secret suitable for a standard
// RFC 6238 authenticator application.
func NewTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// TOTPURI gives authenticator applications a portable enrollment URI. The
// caller owns the returned URI and must never persist it in browser storage.
func TOTPURI(issuer, username, secret string) string {
	label := url.PathEscape(issuer + ":" + username)
	values := url.Values{
		"algorithm": {"SHA1"},
		"digits":    {strconv.Itoa(totpDigits)},
		"issuer":    {issuer},
		"period":    {strconv.Itoa(int(totpPeriod / time.Second))},
		"secret":    {secret},
	}
	return "otpauth://totp/" + label + "?" + values.Encode()
}

// TOTPCode returns the six digit RFC 6238 code for a testable timestamp.
func TOTPCode(secret string, at time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", err
	}
	if len(key) == 0 {
		return "", errors.New("crypto: empty TOTP secret")
	}
	counter := uint64(at.UTC().Unix() / int64(totpPeriod/time.Second))
	var message [8]byte
	for i := 7; i >= 0; i-- {
		message[i] = byte(counter)
		counter >>= 8
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := int(digest[len(digest)-1] & 0x0f)
	value := (int(digest[offset])&0x7f)<<24 |
		int(digest[offset+1])<<16 |
		int(digest[offset+2])<<8 |
		int(digest[offset+3])
	return fmt.Sprintf("%0*d", totpDigits, value%1_000_000), nil
}

// TOTPMatchingCounter returns the matching RFC 6238 counter for the current
// code plus one adjacent period. Callers store the counter to reject replay.
func TOTPMatchingCounter(secret, code string, at time.Time) (int64, bool) {
	if len(code) != totpDigits {
		return 0, false
	}
	for _, offset := range []time.Duration{-totpPeriod, 0, totpPeriod} {
		candidate := at.Add(offset)
		want, err := TOTPCode(secret, candidate)
		if err == nil && subtleConstantTimeEqual([]byte(want), []byte(code)) {
			return candidate.UTC().Unix() / int64(totpPeriod/time.Second), true
		}
	}
	return 0, false
}

// VerifyTOTP accepts the current code plus one adjacent period to tolerate a
// small, expected clock skew. Authentication handlers should prefer
// TOTPMatchingCounter so they can prevent code replay.
func VerifyTOTP(secret, code string, at time.Time) bool {
	_, ok := TOTPMatchingCounter(secret, code, at)
	return ok
}

// NewRecoveryCodes returns human-transcribable one-time recovery codes. Only
// HashToken(code) may be persisted by callers.
func NewRecoveryCodes(count int) ([]string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	if count <= 0 {
		return nil, nil
	}
	codes := make([]string, count)
	for i := range codes {
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		var b strings.Builder
		b.Grow(14)
		for j, value := range bytes {
			if j == 4 || j == 8 {
				b.WriteByte('-')
			}
			b.WriteByte(alphabet[int(value)%len(alphabet)])
		}
		codes[i] = b.String()
	}
	return codes, nil
}

// NormalizeRecoveryCode lets a member enter a displayed code with or without
// separators while retaining case-insensitive one-time semantics.
func NormalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

// --- internal Agent CA (ADR-0006) -----------------------------------------

type CA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// LoadOrCreateCA loads the internal CA or creates a fresh one with a 10-year
// validity. The CA certificate is exported for Caddy; the key stays in Core.
func LoadOrCreateCA(dir string) (*CA, error) {
	keyPath := filepath.Join(dir, caKeyFile)
	certPath := filepath.Join(dir, caCertFile)
	keyBytes, err := os.ReadFile(keyPath)
	certBytes, errCert := os.ReadFile(certPath)
	if errors.Is(err, os.ErrNotExist) || errors.Is(errCert, os.ErrNotExist) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
		if err := writeKeyFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})); err != nil {
			return nil, err
		}
		now := time.Now()
		template := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "AutoSecrets Agent CA"},
			NotBefore:             now.Add(-time.Hour),
			NotAfter:              now.AddDate(10, 0, 0),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			SubjectKeyId:          subjectKeyID(&key.PublicKey),
		}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			return nil, err
		}
		if err := writeKeyFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})); err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, err
		}
		return &CA{cert: cert, key: key}, nil
	}
	if err != nil || errCert != nil {
		return nil, err
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, errors.New("crypto: malformed CA key")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}
	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil {
		return nil, errors.New("crypto: malformed CA certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return &CA{cert: cert, key: key}, nil
}

func (c *CA) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw})
}

// IssueAgentCert signs a certificate for one Managed Node. The serial is the
// identity Caddy forwards to Core; the CN carries the node ID. Validity is
// short-lived; renewal is a later phase.
func (c *CA) IssueAgentCert(nodeID string, csrPEM []byte, ttl time.Duration) (certPEM []byte, serial string, expiresAt time.Time, err error) {
	csrBlock, _ := pem.Decode(csrPEM)
	if csrBlock == nil {
		return nil, "", time.Time{}, errors.New("crypto: malformed CSR")
	}
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, "", time.Time{}, err
	}
	serialBig, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", time.Time{}, err
	}
	now := time.Now()
	subjectKeyID := subjectKeyID(csr.PublicKey)
	template := &x509.Certificate{
		SerialNumber:   serialBig,
		Subject:        pkix.Name{CommonName: "node:" + nodeID},
		NotBefore:      now.Add(-time.Minute),
		NotAfter:       now.Add(ttl),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		SubjectKeyId:   subjectKeyID[:20],
		AuthorityKeyId: c.cert.SubjectKeyId,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		serialBig.Text(16), now.Add(ttl), nil
}

// ParseAgentCert parses a PEM or DER Agent certificate and checks it was
// issued by this CA and is currently valid.
func (c *CA) ParseAgentCert(raw []byte, now time.Time) (*x509.Certificate, error) {
	der := bytes.TrimSpace(raw)
	if block, _ := pem.Decode(der); block != nil && block.Type == "CERTIFICATE" {
		der = block.Bytes
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(c.cert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return nil, err
	}
	return cert, nil
}

// VerifyAgentProof checks an Ed25519 signature from an Agent certificate.
func VerifyAgentProof(cert *x509.Certificate, message, sig []byte) error {
	pub, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return errors.New("crypto: agent cert is not Ed25519")
	}
	if !ed25519.Verify(pub, message, sig) {
		return errors.New("crypto: invalid agent proof")
	}
	return nil
}

// subjectKeyID returns the RFC 5280 standard Subject Key Identifier: SHA-1 of
// the subjectPublicKey BIT STRING (the raw key bytes, uncompressed points).
func subjectKeyID(pub any) []byte {
	switch p := pub.(type) {
	case *ecdsa.PublicKey:
		sum := sha1.Sum(elliptic.Marshal(p.Curve, p.X, p.Y)) //nolint:gosec // RFC 5280 mandates SHA-1 here
		return sum[:]
	case ed25519.PublicKey:
		sum := sha1.Sum(p) //nolint:gosec
		return sum[:]
	}
	return nil
}

// --- Core signing key -----------------------------------------------------

type Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

func (s *Signer) PublicKey() ed25519.PublicKey   { return s.pub }
func (s *Signer) PrivateKey() ed25519.PrivateKey { return s.priv }
func (s *Signer) KeyID() string                  { return hex.EncodeToString(s.pub[:8]) }
func (s *Signer) Sign(message []byte) []byte     { return ed25519.Sign(s.priv, message) }

// PublicKeyPEM returns the public key as a PKIX PEM for embedding in the
// install script and for agent-side verification.
func (s *Signer) PublicKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(s.pub)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// LoadOrCreateSigner loads the Ed25519 Core signing key or creates it.
func LoadOrCreateSigner(dir string) (*Signer, error) {
	path := filepath.Join(dir, signingKeyFile)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return nil, err
		}
		if err := writeKeyFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err != nil {
			return nil, err
		}
		return &Signer{priv: priv, pub: priv.Public().(ed25519.PublicKey)}, nil
	}
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, errors.New("crypto: malformed signing key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("crypto: signing key is not Ed25519")
	}
	return &Signer{priv: priv, pub: priv.Public().(ed25519.PublicKey)}, nil
}

func writeKeyFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
