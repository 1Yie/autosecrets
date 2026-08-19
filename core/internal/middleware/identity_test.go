package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
)

func issueAgent(t *testing.T) (*crypto.CA, ed25519.PrivateKey, []byte, string) {
	t.Helper()
	ca, err := crypto.LoadOrCreateCA(t.TempDir())
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
	certPEM, serial, _, err := ca.IssueAgentCert("node-1", csrPEM, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return ca, priv, certPEM, serial
}

func TestAgentIdentityProofFromUntrustedPeer(t *testing.T) {
	ca, priv, certPEM, serial := issueAgent(t)
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	msg := []byte(ts + "\nGET\n/agent/v1/desired")
	sig := ed25519.Sign(priv, msg)

	handler := AgentIdentityMiddleware(nil, "X-Autosecrets-Client-Cert", ca, func() time.Time { return now })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := AgentSerialFromContext(r.Context())
			if !ok || got != serial {
				t.Fatalf("serial=%q ok=%v want %q", got, ok, serial)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/agent/v1/desired", nil)
	req.Header.Set(AgentProofCertHeader, base64.StdEncoding.EncodeToString(certPEM))
	req.Header.Set(AgentProofTsHeader, ts)
	req.Header.Set(AgentProofSigHeader, base64.StdEncoding.EncodeToString(sig))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		body, _ := io.ReadAll(rec.Result().Body)
		t.Fatalf("status %d body %s", rec.Code, body)
	}
}

func TestAgentIdentityProxySerialStillWorks(t *testing.T) {
	_, cidr, err := net.ParseCIDR("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	handler := AgentIdentityMiddleware([]*net.IPNet{cidr}, "X-Autosecrets-Client-Cert", nil, time.Now)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := AgentSerialFromContext(r.Context())
			if !ok || got != "abc123" {
				t.Fatalf("serial=%q ok=%v", got, ok)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/agent/v1/desired", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("X-Autosecrets-Client-Cert", "abc123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAgentIdentityRejectsBadProof(t *testing.T) {
	ca, _, certPEM, _ := issueAgent(t)
	handler := AgentIdentityMiddleware(nil, "X-Autosecrets-Client-Cert", ca, time.Now)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("should not reach handler")
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/agent/v1/desired", nil)
	req.Header.Set(AgentProofCertHeader, base64.StdEncoding.EncodeToString(certPEM))
	req.Header.Set(AgentProofTsHeader, strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set(AgentProofSigHeader, base64.StdEncoding.EncodeToString([]byte("nope")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}
