// Package secrets owns the Secret Bundle domain: Applications, Environments,
// Secrets, File Bindings, Drafts, Publish, Rollback, Revisions, and
// Activation Policy (ADR-0025). It depends on the shared database and
// middleware packages, never on the app composition root.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
	"github.com/google/uuid"
)

type Handler struct {
	store *database.Store
	mk    *crypto.MasterKey
	now   func() time.Time
}

func NewHandler(st *database.Store, mk *crypto.MasterKey, now func() time.Time) *Handler {
	return &Handler{store: st, mk: mk, now: now}
}

var safeModes = map[int64]bool{0o400: true, 0o440: true, 0o444: true, 0o600: true, 0o640: true, 0o644: true}

var policyActions = map[string]bool{"none": true, "reload": true, "restart": true}

type secretDTO struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Binding         *bindingDTO `json:"binding"`
	LatestVersion   int64       `json:"latest_version"`
	SelectedVersion int64       `json:"selected_version"`
}

type bindingDTO struct {
	Path string `json:"path"`
	UID  int64  `json:"uid"`
	GID  int64  `json:"gid"`
	Mode string `json:"mode"`
}

func (h *Handler) handleListApplications(w http.ResponseWriter, r *http.Request) {
	cursor, limit, err := middleware.PageParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	}
	rows, next, err := h.store.ListApplicationsPage(r.Context(), cursor, limit)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"items": rows, "next_cursor": next})
}

func (h *Handler) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "name is required (max 64 chars)")
		return
	}
	id := uuid.NewString()
	if err := h.store.CreateApplication(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		middleware.WriteError(w, http.StatusConflict, "duplicate", "application name already exists")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "application.create", Resource: id,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (h *Handler) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")
	app, err := h.store.GetApplication(r.Context(), appID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application not found")
		return
	}
	envs, err := h.store.ListEnvironments(r.Context(), appID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"id": app.ID, "name": app.Name,
		"environments": envs,
	})
}

func (h *Handler) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		Protection string `json:"protection"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "name is required (max 64 chars)")
		return
	}
	protection := strings.ToLower(strings.TrimSpace(body.Protection))
	if protection != "standard" && protection != "protected" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "protection must be 'standard' or 'protected'")
		return
	}
	appID := r.PathValue("appID")
	if _, err := h.store.GetApplication(r.Context(), appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application not found")
		return
	}
	id := uuid.NewString()
	if err := h.store.CreateEnvironment(r.Context(), id, appID, strings.TrimSpace(body.Name), protection); err != nil {
		middleware.WriteError(w, http.StatusConflict, "duplicate", "environment name already exists in this application")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "environment.create", Resource: id,
		Result: "protection=" + protection, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"id": id, "name": strings.TrimSpace(body.Name), "protection": protection,
	})
}

func (h *Handler) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	secrets, err := h.store.ListSecrets(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	out := make([]secretDTO, 0, len(secrets))
	for _, s := range secrets {
		item := secretDTO{ID: s.ID, Name: s.Name, LatestVersion: s.LatestVersion, SelectedVersion: s.SelectedVersion}
		if s.Path != "" {
			item.Binding = &bindingDTO{Path: s.Path, UID: s.UID, GID: s.GID, Mode: middleware.ModeString(s.Mode)}
		}
		out = append(out, item)
	}
	middleware.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 128) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "name is required (max 128 chars)")
		return
	}
	if strings.ContainsAny(body.Name, "/\x00") {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "secret name must not contain '/' or NUL")
		return
	}
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	secretID := uuid.NewString()
	versionID := uuid.NewString()
	if err := h.createSecretWithValue(r.Context(), secretID, versionID, appID, envID, strings.TrimSpace(body.Name), []byte(body.Value)); err != nil {
		if errors.Is(err, database.ErrDuplicate) {
			middleware.WriteError(w, http.StatusConflict, "duplicate", "secret name already exists in this environment")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "secret.create", Resource: secretID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{"id": secretID, "name": strings.TrimSpace(body.Name)})
}

func (h *Handler) createSecretWithValue(ctx context.Context, secretID, versionID, appID, envID, name string, value []byte) error {
	wrapped, nonces, ct, err := h.mk.Seal(value)
	if err != nil {
		return err
	}
	return h.store.CreateSecretWithValue(ctx, secretID, versionID, appID, envID, name, wrapped, nonces, ct)
}

func (h *Handler) handleCreateSecretVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	secretID := r.PathValue("secretID")
	wrapped, nonces, ct, err := h.mk.Seal([]byte(body.Value))
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	seq, draftVersion, err := h.store.AddSecretVersion(r.Context(), uuid.NewString(), secretID, wrapped, nonces, ct)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "secret not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "secret.version", Resource: secretID,
		Result: fmt.Sprintf("seq=%d draft=%d", seq, draftVersion), CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]any{"seq": seq, "draft_version": draftVersion})
}

func (h *Handler) handleUpdateBinding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		UID  *int64 `json:"uid"`
		GID  *int64 `json:"gid"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	path, err := validateBindingPath(body.Path)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	mode, err := strconv.ParseInt(body.Mode, 8, 64)
	if err != nil || !safeModes[mode] {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "mode must be one of 0400, 0440, 0444, 0600, 0640, 0644")
		return
	}
	uid, gid := int64(0), int64(0)
	if body.UID != nil {
		if *body.UID < 0 {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "uid must be >= 0")
			return
		}
		uid = *body.UID
	}
	if body.GID != nil {
		if *body.GID < 0 {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "gid must be >= 0")
			return
		}
		gid = *body.GID
	}
	draftVersion, err := h.store.UpdateBinding(r.Context(), r.PathValue("secretID"), path, uid, gid, mode)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "secret not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "binding.update", Resource: r.PathValue("secretID"),
		Result: fmt.Sprintf("path=%s draft=%d", path, draftVersion), CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"draft_version": draftVersion, "path": path})
}

func validateBindingPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path is required")
	}
	if strings.HasPrefix(p, "/") {
		return "", errors.New("path must be relative")
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("path must not contain empty, '.', or '..' components")
		}
		if len(part) > 255 {
			return "", errors.New("path component exceeds 255 bytes")
		}
		for _, c := range part {
			if c < 0x20 || c == 0x7f {
				return "", errors.New("path must not contain control characters")
			}
		}
	}
	return p, nil
}

func (h *Handler) handleGetDraft(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	draft, err := h.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteJSON(w, http.StatusOK, map[string]any{"version": 0, "selections": []any{}})
			return
		}
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	type selectionDTO struct {
		SecretID   string     `json:"secret_id"`
		Name       string     `json:"name"`
		VersionSeq int64      `json:"version_seq"`
		Binding    bindingDTO `json:"binding"`
	}
	out := struct {
		Version    int64          `json:"version"`
		Selections []selectionDTO `json:"selections"`
	}{Version: draft.Version, Selections: []selectionDTO{}}
	for _, sel := range draft.Selections {
		out.Selections = append(out.Selections, selectionDTO{
			SecretID: sel.SecretID, Name: sel.Name, VersionSeq: sel.VersionSeq,
			Binding: bindingDTO{Path: sel.Path, UID: sel.UID, GID: sel.GID, Mode: middleware.ModeString(sel.Mode)},
		})
	}
	middleware.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) handleUpdateDraft(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Selections map[string]int64 `json:"selections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if len(body.Selections) == 0 {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "selections must not be empty")
		return
	}
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	expected := int64(-1)
	if raw := r.Header.Get("If-Match"); raw != "" {
		expected, _ = strconv.ParseInt(raw, 10, 64)
	}
	draftVersion, err := h.store.UpdateDraftSelections(r.Context(), appID, envID, expected, body.Selections)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		case errors.Is(err, database.ErrConflict):
			middleware.WriteError(w, http.StatusConflict, "conflict", "draft changed; reload and retry")
		case errors.Is(err, database.ErrBadPayload):
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "selection references an unknown secret or version")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"draft_version": draftVersion})
}

func (h *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		OperationReason *middleware.OperationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	draft, err := h.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	revisions, err := h.store.ListRevisions(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if len(revisions) > 0 && revisions[0].DraftVersion == draft.Version {
		middleware.WriteError(w, http.StatusConflict, "conflict", "draft has no changes to publish")
		return
	}
	revision, err := h.store.Publish(r.Context(), uuid.NewString(), appID, envID, middleware.ActorFrom(r), reason)
	if err != nil {
		if errors.Is(err, database.ErrNoSecrets) {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "draft has no secrets to publish")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if err := h.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "revision.publish", Resource: revision.ID,
		Result:        fmt.Sprintf("draft=%d files=%d reason=%s", revision.DraftVersion, revision.FileCount, reason.Category),
		CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, revision)
}

func (h *Handler) handleRollback(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		SourceRevisionID string                           `json:"source_revision_id"`
		OperationReason  *middleware.OperationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SourceRevisionID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "source_revision_id and operation_reason are required")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	revision, err := h.store.Rollback(r.Context(), uuid.NewString(), appID, envID, body.SourceRevisionID, middleware.ActorFrom(r), reason)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusNotFound, "not_found", "source revision not found in this environment")
		case errors.Is(err, database.ErrNoSecrets):
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "source revision has no files to roll back to")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	if err := h.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "revision.rollback", Resource: revision.ID,
		Result:        fmt.Sprintf("source=%s files=%d reason=%s", body.SourceRevisionID, revision.FileCount, reason.Category),
		CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, revision)
}

func (h *Handler) handleListRevisions(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	revisions, err := h.store.ListRevisions(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, revisions)
}

func (h *Handler) handleGetActivationPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.store.GetActivationPolicy(r.Context(), r.PathValue("envID"))
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "activation policy not configured")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"action": policy.Action, "units": policy.Units})
}

func (h *Handler) handlePutActivationPolicy(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		Action          string                           `json:"action"`
		Units           []string                         `json:"units"`
		OperationReason *middleware.OperationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	action := strings.ToLower(strings.TrimSpace(body.Action))
	if !policyActions[action] {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "action must be one of none, reload, restart")
		return
	}
	if len(body.Units) < 1 || len(body.Units) > 5 {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "units must contain 1-5 systemd unit names")
		return
	}
	units := make([]string, 0, len(body.Units))
	for _, unit := range body.Units {
		unit = strings.TrimSpace(unit)
		if unit == "" || strings.ContainsAny(unit, " \t\n;|&$`\"'") || strings.Contains(unit, "..") {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "invalid systemd unit name")
			return
		}
		units = append(units, unit)
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	if err := h.store.SaveActivationPolicy(r.Context(), database.ActivationPolicy{
		EnvironmentID: envID, Action: action, Units: units,
	}); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "activation_policy.update", Resource: envID,
		Result: action + " reason=" + reason.Category, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"action": action, "units": units})
}

// Register mounts the secrets routes onto the provided mux under the given
// management base and require-session middleware.
func (h *Handler) Register(mux *http.ServeMux, base string, requireSession func(http.Handler) http.Handler) {
	mux.Handle("GET "+base+"/applications", requireSession(http.HandlerFunc(h.handleListApplications)))
	mux.Handle("POST "+base+"/applications", requireSession(http.HandlerFunc(h.handleCreateApplication)))
	mux.Handle("GET "+base+"/applications/{appID}", requireSession(http.HandlerFunc(h.handleGetApplication)))
	mux.Handle("POST "+base+"/applications/{appID}/environments", requireSession(http.HandlerFunc(h.handleCreateEnvironment)))
	mux.Handle("GET "+base+"/applications/{appID}/environments/{envID}/secrets", requireSession(http.HandlerFunc(h.handleListSecrets)))
	mux.Handle("POST "+base+"/applications/{appID}/environments/{envID}/secrets", requireSession(http.HandlerFunc(h.handleCreateSecret)))
	mux.Handle("POST "+base+"/secrets/{secretID}/versions", requireSession(http.HandlerFunc(h.handleCreateSecretVersion)))
	mux.Handle("PUT "+base+"/secrets/{secretID}/binding", requireSession(http.HandlerFunc(h.handleUpdateBinding)))
	mux.Handle("GET "+base+"/applications/{appID}/environments/{envID}/draft", requireSession(http.HandlerFunc(h.handleGetDraft)))
	mux.Handle("PUT "+base+"/applications/{appID}/environments/{envID}/draft", requireSession(http.HandlerFunc(h.handleUpdateDraft)))
	mux.Handle("POST "+base+"/applications/{appID}/environments/{envID}/publish", requireSession(http.HandlerFunc(h.handlePublish)))
	mux.Handle("POST "+base+"/applications/{appID}/environments/{envID}/rollback", requireSession(http.HandlerFunc(h.handleRollback)))
	mux.Handle("GET "+base+"/applications/{appID}/environments/{envID}/revisions", requireSession(http.HandlerFunc(h.handleListRevisions)))
	mux.Handle("GET "+base+"/applications/{appID}/environments/{envID}/activation-policy", requireSession(http.HandlerFunc(h.handleGetActivationPolicy)))
	mux.Handle("PUT "+base+"/applications/{appID}/environments/{envID}/activation-policy", requireSession(http.HandlerFunc(h.handlePutActivationPolicy)))
}
