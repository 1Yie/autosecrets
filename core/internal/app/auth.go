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

func (a *App) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if !usernameRe.MatchString(body.Username) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 2-64 chars of [a-zA-Z0-9._-]"})
		return
	}
	if len(body.Password) < 10 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 10 characters"})
		return
	}
	count, err := a.store.AdminCount(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if count > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already bootstrapped"})
		return
	}
	ok, err := a.store.ConsumeBootstrapCode(r.Context(), crypto.HashToken(body.Code), a.now())
	if err != nil || !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid or expired bootstrap code"})
		return
	}
	hash, err := crypto.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	adminID := uuid.NewString()
	if err := a.store.CreateAdmin(r.Context(), adminID, body.Username, hash); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already taken"})
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: "bootstrap", Action: "admin.create", Resource: adminID,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": adminID, "username": body.Username})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	admin, err := a.store.AdminByUsername(r.Context(), body.Username)
	if err != nil {
		_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
			Actor: body.Username, Action: "auth.login", Result: "denied",
			CorrelationID: a.correlationID(r),
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	ok, err := crypto.VerifyPassword(body.Password, admin.PasswordHash)
	if err != nil || !ok {
		_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
			Actor: body.Username, Action: "auth.login", Result: "denied",
			CorrelationID: a.correlationID(r),
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	sessionID, err := crypto.NewSecret(256)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	csrf, err := crypto.NewSecret(128)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	expires := a.now().Add(defaultTTL)
	if err := a.store.CreateSession(r.Context(), crypto.HashToken(sessionID), admin.ID, csrf, expires); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sessionID, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil, Expires: expires,
	})
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: admin.Username, Action: "auth.login", Result: "ok",
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"csrf_token": csrf, "username": admin.Username, "id": admin.ID,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = a.store.DeleteSession(r.Context(), crypto.HashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	// Bootstrap state is exposed here so the panel can choose its first screen.
	count, err := a.store.AdminCount(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if count == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"bootstrap_required": true})
		return
	}
	s, ok := a.sessionFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bootstrap_required": false,
		"admin":              map[string]string{"id": s.AdminID, "username": s.Username},
		"csrf_token":         s.CSRFToken,
	})
}

func validName(s string, max int) bool {
	s = strings.TrimSpace(s)
	return len(s) > 0 && len(s) <= max
}

func timeString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
