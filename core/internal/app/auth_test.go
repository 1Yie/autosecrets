package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
)

// recoveryCodeCount mirrors the identity domain's TOTP recovery-code count.
const recoveryCodeCount = 10

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
	if ok.body["status"] != "active" || ok.body["csrf_token"] == "" {
		t.Fatalf("bootstrap must activate the Administrator and issue a Session: %s", ok.raw)
	}
	bootstrapCookie := sessionCookieFrom(t, ok)
	meAfterBootstrap := ta.do(t, "GET", "/api/v1/me", nil, bootstrapCookie, "")
	if meAfterBootstrap.status != http.StatusOK || meAfterBootstrap.body["member"] == nil {
		t.Fatalf("bootstrap Session is not authenticated: %d %s", meAfterBootstrap.status, meAfterBootstrap.raw)
	}
	passwordLogin := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42",
	}, "", "")
	if passwordLogin.status != http.StatusOK {
		t.Fatalf("password-only login must be the default: %d %s", passwordLogin.status, passwordLogin.raw)
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

func TestAdministratorCanEnableTOTPAndCompleteTwoStageLogin(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return fixedNow } })
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bootstrapped := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Acme Platform", "username": "admin",
		"password": "correct-horse-battery-42",
	}, "", "")
	if bootstrapped.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", bootstrapped.status, bootstrapped.raw)
	}
	cookie := sessionCookieFrom(t, bootstrapped)
	csrf := bootstrapped.body["csrf_token"].(string)
	started := ta.do(t, "POST", "/api/v1/auth/totp/enrollment", map[string]string{
		"password": "correct-horse-battery-42",
	}, cookie, csrf)
	if started.status != http.StatusCreated {
		t.Fatalf("start TOTP enrollment: %d %s", started.status, started.raw)
	}
	totpURI, _ := started.body["totp_uri"].(string)
	secret := totpSecretFromURI(t, totpURI)
	totpCode, err := crypto.TOTPCode(secret, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	verified := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/verify", map[string]string{
		"enrollment_token": started.body["enrollment_token"].(string), "totp_code": totpCode,
	}, cookie, csrf)
	if verified.status != http.StatusOK {
		t.Fatalf("verify MFA enrollment: %d %s", verified.status, verified.raw)
	}
	codes, ok := verified.body["recovery_codes"].([]any)
	if !ok || len(codes) != recoveryCodeCount {
		t.Fatalf("one-time recovery codes: %s", verified.raw)
	}
	confirmed := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/confirm", map[string]string{
		"confirmation_token": verified.body["confirmation_token"].(string),
	}, cookie, csrf)
	if confirmed.status != http.StatusOK || confirmed.body["status"] != "active" {
		t.Fatalf("confirm MFA enrollment: %d %s", confirmed.status, confirmed.raw)
	}
	firstStep := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42",
	}, "", "")
	if firstStep.status != http.StatusAccepted || firstStep.body["code"] != "second_factor_required" {
		t.Fatalf("password step must require a second factor: %d %s", firstStep.status, firstStep.raw)
	}
	challengeCookie := cookieValueFrom(t, firstStep, "autosecrets_login_challenge")
	secondStep := ta.doH(t, "POST", "/api/v1/auth/login/second-factor", map[string]string{
		"totp_code": totpCode,
	}, map[string]string{"Cookie": "autosecrets_login_challenge=" + challengeCookie})
	if secondStep.status != http.StatusOK || secondStep.body["role"] != "administrator" {
		t.Fatalf("TOTP login: %d %s", secondStep.status, secondStep.raw)
	}
	loginCookie := sessionCookieFrom(t, secondStep)
	me := ta.do(t, "GET", "/api/v1/me", nil, loginCookie, "")
	if me.status != http.StatusOK {
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
	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, loginCookie, "")
	assertAuditResults(t, audit.raw, map[string][]string{
		"administrator.totp_enabled": {"ok"},
		"auth.second_factor":         {"totp"},
		"auth.login":                 {"local"},
	})
	for _, value := range []string{secret, totpCode} {
		if strings.Contains(string(audit.raw), value) {
			t.Fatalf("TOTP value %q leaked into Audit Events", value)
		}
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

func TestPasswordAndSecondFactorFailuresAreRateLimitedWithoutPermanentLockout(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	cookie, csrf := ta.bootstrap(t)
	secret, _ := ta.enableTOTP(t, cookie, csrf, "correct-horse-42")

	for i := 0; i < 5; i++ {
		failed := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
			"username": "admin", "password": "wrong-password-42",
		}, "", "")
		if failed.status != http.StatusUnauthorized {
			t.Fatalf("password failure %d: %d %s", i+1, failed.status, failed.raw)
		}
	}
	limited := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	if limited.status != http.StatusTooManyRequests || limited.body["code"] != "rate_limited" || limited.header.Get("Retry-After") == "" {
		t.Fatalf("password attempts not limited: %d %s", limited.status, limited.raw)
	}
	events, _, err := ta.store.ListAuditPage(t.Context(), database.AuditFilter{Action: "auth.login", Limit: 100}, 0)
	if err != nil || !hasAuditResult(events, "rate_limited") {
		t.Fatalf("password rate limit Audit Event: events=%v err=%v", events, err)
	}

	current = current.Add(5*time.Minute + time.Second)
	for i := 0; i < 5; i++ {
		started := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
			"username": "admin", "password": "correct-horse-42",
		}, "", "")
		if started.status != http.StatusAccepted {
			t.Fatalf("start factor attempt %d: %d %s", i+1, started.status, started.raw)
		}
		challenge := cookieValueFrom(t, started, "autosecrets_login_challenge")
		failed := ta.doH(t, "POST", "/api/v1/auth/login/second-factor", map[string]string{
			"totp_code": "not-a-code",
		}, map[string]string{"Cookie": "autosecrets_login_challenge=" + challenge})
		if failed.status != http.StatusUnauthorized {
			t.Fatalf("factor failure %d: %d %s", i+1, failed.status, failed.raw)
		}
	}
	started := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	challenge := cookieValueFrom(t, started, "autosecrets_login_challenge")
	limited = ta.doH(t, "POST", "/api/v1/auth/login/second-factor", map[string]string{
		"totp_code": "not-a-code",
	}, map[string]string{"Cookie": "autosecrets_login_challenge=" + challenge})
	if limited.status != http.StatusTooManyRequests || limited.body["code"] != "rate_limited" {
		t.Fatalf("factor attempts not limited: %d %s", limited.status, limited.raw)
	}
	events, _, err = ta.store.ListAuditPage(t.Context(), database.AuditFilter{Action: "auth.second_factor", Limit: 100}, 0)
	if err != nil || !hasAuditResult(events, "rate_limited") {
		t.Fatalf("factor rate limit Audit Event: events=%v err=%v", events, err)
	}

	current = current.Add(5*time.Minute + time.Second)
	started = ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-42",
	}, "", "")
	challenge = cookieValueFrom(t, started, "autosecrets_login_challenge")
	totp, err := crypto.TOTPCode(secret, current)
	if err != nil {
		t.Fatal(err)
	}
	recovered := ta.doH(t, "POST", "/api/v1/auth/login/second-factor", map[string]string{
		"totp_code": totp,
	}, map[string]string{"Cookie": "autosecrets_login_challenge=" + challenge})
	if recovered.status != http.StatusOK {
		t.Fatalf("factor limit did not expire: %d %s", recovered.status, recovered.raw)
	}
}

func TestStepUpFollowsTOTPPolicyAndGrantRevocation(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	cookie, csrf := ta.bootstrap(t)
	sessionHash := crypto.HashToken(cookie)

	passwordOnly := ta.do(t, "POST", "/api/v1/auth/step-up", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if passwordOnly.status != http.StatusOK || passwordOnly.body["expires_at"] == "" {
		t.Fatalf("password-only step-up: %d %s", passwordOnly.status, passwordOnly.raw)
	}
	if valid, err := ta.store.HasValidStepUpGrant(t.Context(), sessionHash, current); err != nil || !valid {
		t.Fatalf("password-only grant not persisted: valid=%v err=%v", valid, err)
	}

	secret, recoveryCodes := ta.enableTOTP(t, cookie, csrf, "correct-horse-42")
	if valid, err := ta.store.HasValidStepUpGrant(t.Context(), sessionHash, current); err != nil || valid {
		t.Fatalf("TOTP enablement did not revoke grant: valid=%v err=%v", valid, err)
	}
	missingFactor := ta.do(t, "POST", "/api/v1/auth/step-up", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if missingFactor.status != http.StatusUnauthorized {
		t.Fatalf("step-up without required factor: %d %s", missingFactor.status, missingFactor.raw)
	}
	recovery := recoveryCodes[0].(string)
	withRecovery := ta.do(t, "POST", "/api/v1/auth/step-up", map[string]string{
		"password": "correct-horse-42", "recovery_code": recovery,
	}, cookie, csrf)
	if withRecovery.status != http.StatusOK {
		t.Fatalf("step-up with Recovery Code: %d %s", withRecovery.status, withRecovery.raw)
	}
	replayed := ta.do(t, "POST", "/api/v1/auth/step-up", map[string]string{
		"password": "correct-horse-42", "recovery_code": recovery,
	}, cookie, csrf)
	if replayed.status != http.StatusUnauthorized {
		t.Fatalf("Recovery Code replay accepted for step-up: %d %s", replayed.status, replayed.raw)
	}
	totp, err := crypto.TOTPCode(secret, current)
	if err != nil {
		t.Fatal(err)
	}
	disabled := ta.doH(t, "DELETE", "/api/v1/auth/totp", map[string]string{
		"password": "correct-horse-42", "totp_code": totp,
	}, map[string]string{"Cookie": sessionCookie + "=" + cookie, csrfHeader: csrf})
	if disabled.status != http.StatusOK {
		t.Fatalf("disable TOTP: %d %s", disabled.status, disabled.raw)
	}
	if valid, err := ta.store.HasValidStepUpGrant(t.Context(), sessionHash, current); err != nil || valid {
		t.Fatalf("TOTP disablement did not revoke grant: valid=%v err=%v", valid, err)
	}

	passwordOnly = ta.do(t, "POST", "/api/v1/auth/step-up", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if passwordOnly.status != http.StatusOK {
		t.Fatalf("password-only step-up after disable: %d %s", passwordOnly.status, passwordOnly.raw)
	}
	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, cookie, "")
	assertAuditResults(t, audit.raw, map[string][]string{
		"administrator.totp_enabled":  {"ok"},
		"administrator.totp_disabled": {"ok"},
		"auth.step_up":                {"ok", "denied"},
	})
	for _, value := range []string{secret, recovery, totp} {
		if strings.Contains(string(audit.raw), value) {
			t.Fatalf("step-up value %q leaked into Audit Events", value)
		}
	}
	logout := ta.do(t, "POST", "/api/v1/auth/logout", nil, cookie, csrf)
	if logout.status != http.StatusOK {
		t.Fatalf("logout: %d %s", logout.status, logout.raw)
	}
	if valid, err := ta.store.HasValidStepUpGrant(t.Context(), sessionHash, current); err != nil || valid {
		t.Fatalf("logout did not revoke grant: valid=%v err=%v", valid, err)
	}
}

// TestPasswordChangeRevokesOtherSessions locks US-53: a password change
// revokes every Session and reissues only the current browser's Session,
// with the change audited atomically.
func TestPasswordChangeRevokesOtherSessions(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	cookie, csrf := ta.bootstrap(t)
	secret, _ := ta.enableTOTP(t, cookie, csrf, "correct-horse-42")
	current = current.Add(31 * time.Second)
	totp2, err := crypto.TOTPCode(secret, current)
	if err != nil {
		t.Fatal(err)
	}
	changed := ta.do(t, "POST", "/api/v1/auth/password", map[string]string{
		"current_password": "correct-horse-42",
		"new_password":     "correct-horse-battery-42",
		"totp_code":        totp2,
	}, cookie, csrf)
	if changed.status != http.StatusOK {
		t.Fatalf("password change: %d %s", changed.status, changed.raw)
	}
	newCookie := sessionCookieFrom(t, changed)
	oldMe := ta.do(t, "GET", "/api/v1/me", nil, cookie, "")
	if oldMe.status != http.StatusUnauthorized {
		t.Fatalf("old session must be revoked: %d %s", oldMe.status, oldMe.raw)
	}
	newMe := ta.do(t, "GET", "/api/v1/me", nil, newCookie, "")
	if newMe.status != http.StatusOK {
		t.Fatalf("current browser session must be reissued: %d %s", newMe.status, newMe.raw)
	}
}

func TestPasswordChangeAndRenewalFollowTOTPPolicy(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	cookie, csrf := ta.bootstrap(t)

	renewed := ta.do(t, "POST", "/api/v1/auth/renew", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if renewed.status != http.StatusOK {
		t.Fatalf("password-only renewal: %d %s", renewed.status, renewed.raw)
	}
	cookie, csrf = sessionCookieFrom(t, renewed), renewed.body["csrf_token"].(string)

	secret, _ := ta.enableTOTP(t, cookie, csrf, "correct-horse-42")
	missing := ta.do(t, "POST", "/api/v1/auth/renew", map[string]string{
		"password": "correct-horse-42",
	}, cookie, csrf)
	if missing.status != http.StatusUnauthorized {
		t.Fatalf("renewal without TOTP accepted: %d %s", missing.status, missing.raw)
	}

	current = current.Add(31 * time.Second)
	totp, err := crypto.TOTPCode(secret, current)
	if err != nil {
		t.Fatal(err)
	}
	disabled := ta.doH(t, "DELETE", "/api/v1/auth/totp", map[string]string{
		"password": "correct-horse-42", "totp_code": totp,
	}, map[string]string{"Cookie": sessionCookie + "=" + cookie, csrfHeader: csrf})
	if disabled.status != http.StatusOK {
		t.Fatalf("disable TOTP: %d %s", disabled.status, disabled.raw)
	}

	changed := ta.do(t, "POST", "/api/v1/auth/password", map[string]string{
		"current_password": "correct-horse-42", "new_password": "correct-horse-battery-42",
	}, cookie, csrf)
	if changed.status != http.StatusOK {
		t.Fatalf("password-only change after disable: %d %s", changed.status, changed.raw)
	}
}

func TestAuthenticationAuditMatrixIsValueFree(t *testing.T) {
	ta := newTestApp(t)
	cookie, csrf := ta.bootstrap(t)
	password := "correct-horse-42"
	newPassword := "correct-horse-battery-42"

	deniedLogin := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "wrong-password-42",
	}, "", "")
	if deniedLogin.status != http.StatusUnauthorized {
		t.Fatalf("denied login: %d %s", deniedLogin.status, deniedLogin.raw)
	}
	loggedIn := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": password,
	}, "", "")
	if loggedIn.status != http.StatusOK {
		t.Fatalf("local login: %d %s", loggedIn.status, loggedIn.raw)
	}

	deniedRenewal := ta.do(t, "POST", "/api/v1/auth/renew", map[string]string{
		"password": "wrong-password-42",
	}, cookie, csrf)
	if deniedRenewal.status != http.StatusUnauthorized {
		t.Fatalf("denied renewal: %d %s", deniedRenewal.status, deniedRenewal.raw)
	}
	renewed := ta.do(t, "POST", "/api/v1/auth/renew", map[string]string{
		"password": password,
	}, cookie, csrf)
	if renewed.status != http.StatusOK {
		t.Fatalf("renewal: %d %s", renewed.status, renewed.raw)
	}
	cookie, csrf = sessionCookieFrom(t, renewed), renewed.body["csrf_token"].(string)

	deniedPassword := ta.do(t, "POST", "/api/v1/auth/password", map[string]string{
		"current_password": "wrong-password-42", "new_password": newPassword,
	}, cookie, csrf)
	if deniedPassword.status != http.StatusUnauthorized {
		t.Fatalf("denied password change: %d %s", deniedPassword.status, deniedPassword.raw)
	}
	changed := ta.do(t, "POST", "/api/v1/auth/password", map[string]string{
		"current_password": password, "new_password": newPassword,
	}, cookie, csrf)
	if changed.status != http.StatusOK {
		t.Fatalf("password change: %d %s", changed.status, changed.raw)
	}
	cookie, csrf = sessionCookieFrom(t, changed), changed.body["csrf_token"].(string)

	audit := ta.do(t, "GET", "/api/v1/audit-events?limit=100", nil, cookie, "")
	if audit.status != http.StatusOK {
		t.Fatalf("list audit events: %d %s", audit.status, audit.raw)
	}
	assertAuditResults(t, audit.raw, map[string][]string{
		"administrator.bootstrapped": {"active"},
		"auth.login":                 {"local", "denied"},
		"auth.renew":                 {"local", "denied"},
		"member.password_changed":    {"ok", "denied"},
	})
	for _, secret := range []string{password, newPassword, "wrong-password-42"} {
		if strings.Contains(string(audit.raw), secret) {
			t.Fatalf("authentication value %q leaked into Audit Events", secret)
		}
	}

	logout := ta.do(t, "POST", "/api/v1/auth/logout", nil, cookie, csrf)
	if logout.status != http.StatusOK {
		t.Fatalf("logout: %d %s", logout.status, logout.raw)
	}
	events, _, err := ta.store.ListAuditPage(t.Context(), database.AuditFilter{Action: "auth.logout", Limit: 10}, 0)
	if err != nil || len(events) != 1 || events[0].Result != "ok" {
		t.Fatalf("logout audit: events=%v err=%v", events, err)
	}
}

func assertAuditResults(t *testing.T, raw []byte, expected map[string][]string) {
	t.Helper()
	var page struct {
		Items []database.AuditEvent `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]map[string]bool)
	for _, event := range page.Items {
		if seen[event.Action] == nil {
			seen[event.Action] = make(map[string]bool)
		}
		seen[event.Action][event.Result] = true
	}
	for action, results := range expected {
		for _, result := range results {
			if !seen[action][result] {
				t.Fatalf("missing Audit Event action=%q result=%q; seen=%v", action, result, seen)
			}
		}
	}
}

func hasAuditResult(events []database.AuditEvent, result string) bool {
	for _, event := range events {
		if event.Result == result {
			return true
		}
	}
	return false
}

// TestIdleExpirySlidesOnMutation locks US-44-46: only deliberate
// interactions extend the idle window; polling never does.
func TestIdleExpirySlidesOnMutation(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	cookie, csrf := ta.bootstrap(t)

	// 20 minutes in, a deliberate mutation slides the idle window.
	current = current.Add(20 * time.Minute)
	create := ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "web"}, cookie, csrf)
	if create.status != http.StatusCreated {
		t.Fatalf("mutation: %d %s", create.status, create.raw)
	}
	// Another 20 minutes later the Session is still valid (idle slid).
	current = current.Add(20 * time.Minute)
	me := ta.do(t, "GET", "/api/v1/me", nil, cookie, "")
	if me.status != http.StatusOK {
		t.Fatalf("idle window must slide on mutation: %d %s", me.status, me.raw)
	}
	// Without any further interaction the idle window expires.
	current = current.Add(31 * time.Minute)
	expired := ta.do(t, "GET", "/api/v1/me", nil, cookie, "")
	if expired.status != http.StatusUnauthorized {
		t.Fatalf("idle session must expire: %d %s", expired.status, expired.raw)
	}
}
