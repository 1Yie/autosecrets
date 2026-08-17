package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	h := CORS([]string{"http://154.222.24.116:18081"})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://154.222.24.116:18081")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got !=
		"http://154.222.24.116:18081" {
		t.Fatalf("allow origin: %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentials not allowed")
	}
}

func TestCORSRejectsUnlistedOrigin(t *testing.T) {
	h := CORS([]string{"http://154.222.24.116:18081"})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://evil.example:18081")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unlisted origin must not be reflected")
	}
}

func TestCORSEmptyListIsNoop(t *testing.T) {
	h := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://154.222.24.116:18081")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("empty CORS list must not reflect origins")
	}
}
