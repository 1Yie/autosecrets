package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
)

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
		"code": "wrong", "organization_name": "Acme Platform",
		"username": "admin", "password": "correct-horse-battery-42",
	}, "", "")
	if bad.status != http.StatusForbidden {
		t.Fatalf("wrong code accepted: %d", bad.status)
	}
	ok := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Acme Platform",
		"username": "admin", "password": "correct-horse-battery-42",
	}, "", "")
	if ok.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", ok.status, ok.raw)
	}
	if ok.body["status"] != "pending_mfa" || ok.body["enrollment_token"] == "" || ok.body["totp_uri"] == "" {
		t.Fatalf("bootstrap must start pending MFA enrollment: %s", ok.raw)
	}
	pendingLogin := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42",
		"totp_code": "123456",
	}, "", "")
	if pendingLogin.status != http.StatusUnauthorized {
		t.Fatalf("pending member logged in before MFA enrollment: %d %s", pendingLogin.status, pendingLogin.raw)
	}
	again := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Other",
		"username": "other", "password": "correct-horse-battery-42",
	}, "", "")
	if again.status != http.StatusConflict {
		t.Fatalf("second bootstrap must conflict: %d", again.status)
	}
	code2, _ := ta.app.EmitBootstrapCode(context.Background())
	if code2 != "" {
		t.Fatal("bootstrap code emitted after admin exists")
	}
}

func TestBootstrapMFAActivationAndLogin(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return fixedNow } })
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	started := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Acme Platform", "username": "admin",
		"password": "correct-horse-battery-42",
	}, "", "")
	if started.status != http.StatusCreated {
		t.Fatalf("start bootstrap enrollment: %d %s", started.status, started.raw)
	}
	totpURI, _ := started.body["totp_uri"].(string)
	secret := totpSecretFromURI(t, totpURI)
	totpCode, err := crypto.TOTPCode(secret, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	verified := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/verify", map[string]string{
		"enrollment_token": started.body["enrollment_token"].(string), "totp_code": totpCode,
	}, "", "")
	if verified.status != http.StatusOK {
		t.Fatalf("verify MFA enrollment: %d %s", verified.status, verified.raw)
	}
	codes, ok := verified.body["recovery_codes"].([]any)
	if !ok || len(codes) != recoveryCodeCount {
		t.Fatalf("one-time recovery codes: %s", verified.raw)
	}
	confirmed := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/confirm", map[string]string{
		"confirmation_token": verified.body["confirmation_token"].(string),
	}, "", "")
	if confirmed.status != http.StatusOK || confirmed.body["status"] != "active" {
		t.Fatalf("confirm MFA enrollment: %d %s", confirmed.status, confirmed.raw)
	}
	login := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42", "totp_code": totpCode,
	}, "", "")
	if login.status != http.StatusOK || login.body["role"] != "administrator" {
		t.Fatalf("MFA login: %d %s", login.status, login.raw)
	}
	cookie := sessionCookieFrom(t, login)
	me := ta.do(t, "GET", "/api/v1/me", nil, cookie, "")
	if me.status != http.StatusOK || me.body["step_up"] != true {
		t.Fatalf("authenticated me: %d %s", me.status, me.raw)
	}
	organization, ok := me.body["organization"].(map[string]any)
	if !ok || organization["display_name"] != "Acme Platform" {
		t.Fatalf("organization identity: %s", me.raw)
	}
	member, ok := me.body["member"].(map[string]any)
	if !ok || member["role"] != "administrator" || me.body["session_expires_at"] == "" || me.body["idle_expires_at"] == "" {
		t.Fatalf("member session state: %s", me.raw)
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
