package app

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/store"
	"github.com/google/uuid"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,64}$`)

const (
	mfaEnrollmentTTL   = 30 * time.Minute
	sessionAbsoluteTTL = 12 * time.Hour
	sessionIdleTTL     = 30 * time.Minute
	stepUpTTL          = 5 * time.Minute
	recoveryCodeCount  = 10
)

type bootstrapRequest struct {
	Code             string `json:"code"`
	OrganizationName string `json:"organization_name"`
	Username         string `json:"username"`
	Password         string `json:"password"`
}

func (a *App) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var body bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON")
		return
	}
	if !validName(body.OrganizationName, 128) || !usernameRe.MatchString(body.Username) || !crypto.PasswordValid(body.Password) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid bootstrap enrollment")
		return
	}
	passwordHash, err := crypto.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	totpSecret, err := crypto.NewTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	wrappedKey, nonces, ciphertext, err := a.mk.Seal([]byte(totpSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	enrollmentToken, err := crypto.NewSecret(192)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	memberID := uuid.NewString()
	now := a.now()
	err = a.store.StartBootstrapEnrollment(r.Context(), crypto.HashToken(body.Code), strings.TrimSpace(body.OrganizationName),
		memberID, body.Username, passwordHash, crypto.HashToken(enrollmentToken), wrappedKey, nonces, ciphertext,
		now.Add(mfaEnrollmentTTL), store.AuditEvent{
			Actor: "bootstrap", Action: "member.bootstrap_started", Resource: memberID,
			Result: "pending_mfa", CorrelationID: a.correlationID(r),
		})
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusForbidden, codeForbidden, "invalid or expired bootstrap code")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, codeConflict, "already bootstrapped")
		case store.ErrDuplicate:
			writeError(w, http.StatusConflict, codeDuplicate, "username already taken")
		default:
			writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id": memberID, "username": body.Username, "status": "pending_mfa",
		"enrollment_token": enrollmentToken,
		"totp_uri":         crypto.TOTPURI("AutoSecrets", body.Username, totpSecret),
	})
}

func (a *App) handleVerifyMFAEnrollment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnrollmentToken string `json:"enrollment_token"`
		TOTPCode        string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EnrollmentToken == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid MFA enrollment")
		return
	}
	now := a.now()
	enrollment, err := a.store.MFAEnrollmentByToken(r.Context(), crypto.HashToken(body.EnrollmentToken), now)
	if err != nil {
		writeError(w, http.StatusForbidden, codeForbidden, "invalid or expired MFA enrollment")
		return
	}
	secret, err := a.mk.Open(enrollment.WrappedKey, enrollment.Nonces, enrollment.Ciphertext)
	if err != nil || !crypto.VerifyTOTP(string(secret), body.TOTPCode, now) {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid authentication code")
		return
	}
	recoveryCodes, err := crypto.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	hashes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hashes[i] = crypto.HashToken(crypto.NormalizeRecoveryCode(code))
	}
	confirmationToken, err := crypto.NewSecret(192)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if err := a.store.CompleteMFAEnrollment(r.Context(), crypto.HashToken(body.EnrollmentToken), crypto.HashToken(confirmationToken), hashes, now, store.AuditEvent{
		Actor: "member:" + enrollment.MemberID, Action: "member.mfa_verified", Resource: enrollment.MemberID,
		Result: "pending_recovery_confirmation", CorrelationID: a.correlationID(r),
	}); err != nil {
		writeError(w, http.StatusConflict, codeConflict, "MFA enrollment is no longer pending")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"confirmation_token": confirmationToken,
		"recovery_codes":     recoveryCodes,
	})
}

func (a *App) handleConfirmMFAEnrollment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConfirmationToken string `json:"confirmation_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ConfirmationToken == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid recovery confirmation")
		return
	}
	member, err := a.store.ConfirmMFAEnrollment(r.Context(), crypto.HashToken(body.ConfirmationToken), a.now(), store.AuditEvent{
		Action: "member.activated", Resource: "", Result: "ok", CorrelationID: a.correlationID(r),
	})
	if err != nil {
		writeError(w, http.StatusConflict, codeConflict, "MFA enrollment is no longer pending")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": member.ID, "username": member.Username, "status": member.Status})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		TOTPCode     string `json:"totp_code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON")
		return
	}
	member, err := a.store.MemberByUsername(r.Context(), body.Username)
	if err != nil || member.Status != store.MemberActive {
		a.auditLoginDenied(r, body.Username)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	passwordOK, err := crypto.VerifyPassword(body.Password, member.PasswordHash)
	if err != nil || !passwordOK {
		a.auditLoginDenied(r, body.Username)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	enrolled, err := a.store.HasConfirmedMFA(r.Context(), member.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if !enrolled {
		// Legacy account that predates mandatory MFA: the password is valid,
		// but the member must complete TOTP enrollment before a Session.
		writeError(w, http.StatusForbidden, codeMFAEnrollmentRequired,
			"MFA enrollment is required before login")
		return
	}
	if !a.verifySecondFactor(r, member, body.TOTPCode, body.RecoveryCode) {
		a.auditLoginDenied(r, body.Username)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	a.issueSession(w, r, member)
}

// handleResumeMFAEnrollment lets an existing active member without a
// confirmed TOTP enrollment (e.g. an account created before mandatory MFA)
// start a fresh enrollment with the Bootstrap-equivalent verify and
// Recovery Code confirmation flow.
func (a *App) handleResumeMFAEnrollment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid JSON")
		return
	}
	member, err := a.store.MemberByUsername(r.Context(), body.Username)
	if err != nil || member.Status != store.MemberActive {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	passwordOK, err := crypto.VerifyPassword(body.Password, member.PasswordHash)
	if err != nil || !passwordOK {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	enrolled, err := a.store.HasConfirmedMFA(r.Context(), member.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if enrolled {
		writeError(w, http.StatusConflict, codeConflict, "MFA is already enrolled")
		return
	}
	totpSecret, err := crypto.NewTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	wrappedKey, nonces, ciphertext, err := a.mk.Seal([]byte(totpSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	enrollmentToken, err := crypto.NewSecret(192)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if err := a.store.CreateEnrollmentForMember(r.Context(), member.ID,
		crypto.HashToken(enrollmentToken), wrappedKey, nonces, ciphertext,
		a.now().Add(mfaEnrollmentTTL)); err != nil {
		writeError(w, http.StatusConflict, codeConflict, "MFA is already enrolled")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: "member:" + member.Username, Action: "member.mfa_resumed", Resource: member.ID,
		Result: "pending_mfa", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"username": member.Username, "enrollment_token": enrollmentToken,
		"totp_uri": crypto.TOTPURI("AutoSecrets", member.Username, totpSecret),
	})
}

func (a *App) handleSessionRenewal(w http.ResponseWriter, r *http.Request) {
	member := sessionFrom(r)
	var body struct {
		Password     string `json:"password"`
		TOTPCode     string `json:"totp_code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || member == nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid session renewal")
		return
	}
	identity, err := a.store.MemberByID(r.Context(), member.AdminID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "not authenticated")
		return
	}
	passwordOK, err := crypto.VerifyPassword(body.Password, identity.PasswordHash)
	if err != nil || !passwordOK || !a.verifySecondFactor(r, identity, body.TOTPCode, body.RecoveryCode) {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	_ = a.store.DeleteSession(r.Context(), member.SessionIDHash)
	a.issueSession(w, r, identity)
}

func (a *App) handleStepUp(w http.ResponseWriter, r *http.Request) {
	session := sessionFrom(r)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || session == nil || body.Password == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid step-up request")
		return
	}
	member, err := a.store.MemberByID(r.Context(), session.AdminID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "not authenticated")
		return
	}
	ok, err := crypto.VerifyPassword(body.Password, member.PasswordHash)
	if err != nil || !ok {
		_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{Actor: actorFrom(r), Action: "auth.step_up", Resource: member.ID, Result: "denied", CorrelationID: a.correlationID(r)})
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	if err := a.store.GrantStepUp(r.Context(), session.SessionIDHash, a.now().Add(stepUpTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{Actor: actorFrom(r), Action: "auth.step_up", Resource: member.ID, Result: "ok", CorrelationID: a.correlationID(r)})
	writeJSON(w, http.StatusOK, map[string]string{"expires_at": timeString(a.now().Add(stepUpTTL))})
}

func (a *App) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	session := sessionFrom(r)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		TOTPCode        string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || session == nil || !crypto.PasswordValid(body.NewPassword) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid password change")
		return
	}
	member, err := a.store.MemberByID(r.Context(), session.AdminID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "not authenticated")
		return
	}
	ok, err := crypto.VerifyPassword(body.CurrentPassword, member.PasswordHash)
	if err != nil || !ok || !a.verifySecondFactor(r, member, body.TOTPCode, "") {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid credentials")
		return
	}
	newHash, err := crypto.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if err := a.store.ChangePassword(r.Context(), member.ID, newHash, store.AuditEvent{
		Actor: "member:" + member.Username, Action: "member.password_changed", Resource: member.ID,
		Result: "ok", CorrelationID: a.correlationID(r),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	a.issueSession(w, r, member)
}

func (a *App) issueSession(w http.ResponseWriter, r *http.Request, member *store.Member) {
	sessionID, err := crypto.NewSecret(256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	csrf, err := crypto.NewSecret(128)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	now := a.now()
	absoluteExpires := now.Add(sessionAbsoluteTTL)
	idleExpires := now.Add(sessionIdleTTL)
	if err := a.store.CreateBoundedSession(r.Context(), crypto.HashToken(sessionID), member.ID, csrf, absoluteExpires, idleExpires); err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if err := a.store.GrantStepUp(r.Context(), crypto.HashToken(sessionID), now.Add(stepUpTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: sessionID, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, Expires: absoluteExpires})
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{Actor: "member:" + member.Username, Action: "auth.login", Resource: member.ID, Result: "ok", CorrelationID: a.correlationID(r)})
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": csrf, "username": member.Username, "id": member.ID, "role": member.Role, "expires_at": timeString(absoluteExpires)})
}

// requireStepUp enforces the server-side risk policy: the current Session
// must hold a Step-up Grant issued by recent password confirmation.
func (a *App) requireStepUp(w http.ResponseWriter, r *http.Request) bool {
	session := sessionFrom(r)
	if session == nil {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "not authenticated")
		return false
	}
	has, err := a.store.HasStepUp(r.Context(), session.SessionIDHash, a.now())
	if err != nil || !has {
		writeError(w, http.StatusForbidden, codeStepUp, "current password confirmation is required for this action")
		return false
	}
	return true
}

func (a *App) verifySecondFactor(r *http.Request, member *store.Member, totpCode, recoveryCode string) bool {
	if recoveryCode != "" {
		used, err := a.store.ConsumeRecoveryCode(r.Context(), member.ID, crypto.HashToken(crypto.NormalizeRecoveryCode(recoveryCode)), a.now())
		return err == nil && used
	}
	enrollment, err := a.store.TOTPEnrollmentForMember(r.Context(), member.ID)
	if err != nil || enrollment.VerifiedAt == nil || enrollment.ConfirmedAt == nil {
		return false
	}
	secret, err := a.mk.Open(enrollment.WrappedKey, enrollment.Nonces, enrollment.Ciphertext)
	if err != nil {
		return false
	}
	counter, valid := crypto.TOTPMatchingCounter(string(secret), totpCode, a.now())
	if !valid {
		return false
	}
	used, err := a.store.UseTOTP(r.Context(), member.ID, counter)
	return err == nil && used
}

func (a *App) auditLoginDenied(r *http.Request, username string) {
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{Actor: "member:" + username, Action: "auth.login", Result: "denied", CorrelationID: a.correlationID(r)})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = a.store.DeleteSession(r.Context(), crypto.HashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	count, err := a.store.AdminCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	if count == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"bootstrap_required": true})
		return
	}
	if pending, err := a.store.PendingMFAEnrollment(r.Context()); err == nil && pending {
		writeJSON(w, http.StatusOK, map[string]any{"bootstrap_required": false, "mfa_enrollment_required": true})
		return
	}
	session, ok := a.sessionFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "not authenticated")
		return
	}
	organization, err := a.store.Organization(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
		return
	}
	stepUp, _ := a.store.HasStepUp(r.Context(), session.SessionIDHash, a.now())
	writeJSON(w, http.StatusOK, map[string]any{
		"bootstrap_required": false,
		"organization":       map[string]string{"display_name": organization.DisplayName},
		"member":             map[string]string{"id": session.AdminID, "username": session.Username, "role": session.Role},
		"csrf_token":         session.CSRFToken,
		"session_expires_at": timeString(session.ExpiresAt),
		"idle_expires_at":    timeString(session.IdleExpiresAt),
		"step_up":            stepUp,
	})
}

func validName(value string, max int) bool {
	value = strings.TrimSpace(value)
	return len(value) > 0 && len(value) <= max
}

func timeString(t time.Time) string { return t.UTC().Format(time.RFC3339) }
