package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"autosecrets.dev/core/internal/store"
	"github.com/google/uuid"
)

var safeModes = map[int64]bool{0o400: true, 0o440: true, 0o444: true, 0o600: true, 0o640: true, 0o644: true}

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

var reasonCategories = map[string]bool{
	"maintenance": true, "incident_response": true, "access_change": true,
	"configuration_correction": true, "other": true,
}

type operationReasonInput struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
	ExternalRef string `json:"external_ref"`
}

// operationReason validates and normalizes an Operation Reason. The
// explanation is 10-500 characters; the external reference is optional and
// capped so Audit Events stay bounded.
func operationReason(body *operationReasonInput) (store.OperationReason, bool) {
	if body == nil {
		return store.OperationReason{}, false
	}
	category := strings.ToLower(strings.TrimSpace(body.Category))
	explanation := strings.TrimSpace(body.Explanation)
	if !reasonCategories[category] {
		return store.OperationReason{}, false
	}
	runes := []rune(explanation)
	if len(runes) < 10 || len(runes) > 500 {
		return store.OperationReason{}, false
	}
	externalRef := strings.TrimSpace(body.ExternalRef)
	if len(externalRef) > 128 {
		return store.OperationReason{}, false
	}
	return store.OperationReason{Category: category, Explanation: explanation, ExternalRef: externalRef}, true
}

func modeString(m int64) string { return fmt.Sprintf("%04o", m) }

func (a *App) handleListApplications(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.ListApplications(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validName(body.Name, 64) {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required (max 64 chars)")
		return
	}
	id := uuid.NewString()
	if err := a.store.CreateApplication(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		writeError(w, http.StatusConflict, "duplicate", "application name already exists")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "application.create", Resource: id,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (a *App) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")
	app, err := a.store.GetApplication(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application not found")
		return
	}
	envs, err := a.store.ListEnvironments(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": app.ID, "name": app.Name,
		"environments": envs,
	})
}

func (a *App) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		Protection string `json:"protection"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validName(body.Name, 64) {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required (max 64 chars)")
		return
	}
	protection := strings.ToLower(strings.TrimSpace(body.Protection))
	if protection != "standard" && protection != "protected" {
		writeError(w, http.StatusBadRequest, "bad_request", "protection must be 'standard' or 'protected'")
		return
	}
	appID := r.PathValue("appID")
	if _, err := a.store.GetApplication(r.Context(), appID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application not found")
		return
	}
	id := uuid.NewString()
	if err := a.store.CreateEnvironment(r.Context(), id, appID, strings.TrimSpace(body.Name), protection); err != nil {
		writeError(w, http.StatusConflict, "duplicate", "environment name already exists in this application")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "environment.create", Resource: id,
		Result: "protection=" + protection, CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{
		"id": id, "name": strings.TrimSpace(body.Name), "protection": protection,
	})
}

func (a *App) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	secrets, err := a.store.ListSecrets(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	out := make([]secretDTO, 0, len(secrets))
	for _, s := range secrets {
		item := secretDTO{ID: s.ID, Name: s.Name, LatestVersion: s.LatestVersion, SelectedVersion: s.SelectedVersion}
		if s.Path != "" {
			item.Binding = &bindingDTO{Path: s.Path, UID: s.UID, GID: s.GID, Mode: modeString(s.Mode)}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validName(body.Name, 128) {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required (max 128 chars)")
		return
	}
	if strings.ContainsAny(body.Name, "/\x00") {
		writeError(w, http.StatusBadRequest, "bad_request", "secret name must not contain '/' or NUL")
		return
	}
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	if _, err := a.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	secretID := uuid.NewString()
	versionID := uuid.NewString()
	if err := a.createSecretWithValue(r.Context(), secretID, versionID, appID, envID, strings.TrimSpace(body.Name), []byte(body.Value)); err != nil {
		if errors.Is(err, errDuplicate) {
			writeError(w, http.StatusConflict, "duplicate", "secret name already exists in this environment")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "secret.create", Resource: secretID,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": secretID, "name": strings.TrimSpace(body.Name)})
}

// createSecretWithValue creates a Secret, its first encrypted Secret Version,
// the default File Binding, and the Draft selection in one transaction.
func (a *App) createSecretWithValue(ctx context.Context, secretID, versionID, appID, envID, name string, value []byte) error {
	wrapped, nonces, ct, err := a.mk.Seal(value)
	if err != nil {
		return err
	}
	return a.store.CreateSecretWithValue(ctx, secretID, versionID, appID, envID, name, wrapped, nonces, ct)
}

func (a *App) handleCreateSecretVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	secretID := r.PathValue("secretID")
	wrapped, nonces, ct, err := a.mk.Seal([]byte(body.Value))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	seq, draftVersion, err := a.store.AddSecretVersion(r.Context(), uuid.NewString(), secretID, wrapped, nonces, ct)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "secret not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "secret.version", Resource: secretID,
		Result: fmt.Sprintf("seq=%d draft=%d", seq, draftVersion), CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]any{"seq": seq, "draft_version": draftVersion})
}

// handleRotateSecret moves the Draft selection of a Secret to the next
// candidate version (cyclically) and bumps the Draft. The next Publish then
// delivers the new value to nodes; until then nodes keep their current
// value (see buildEnvelope keep-old-value).
func (a *App) handleRotateSecret(w http.ResponseWriter, r *http.Request) {
	secretID := r.PathValue("secretID")
	appID, envID, err := a.store.SecretAppEnv(r.Context(), secretID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "secret not found")
		return
	}
	draft, err := a.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	curSeq := int64(-1)
	for _, sel := range draft.Selections {
		if sel.SecretID == secretID {
			curSeq = sel.VersionSeq
			break
		}
	}
	if curSeq < 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "secret is not selected in the draft")
		return
	}
	seqs, err := a.store.SecretVersionSeqs(r.Context(), secretID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if len(seqs) < 2 {
		writeError(w, http.StatusBadRequest, "bad_request", "secret has no candidate versions to rotate to")
		return
	}
	next := nextVersion(seqs, curSeq)
	if next == curSeq {
		writeError(w, http.StatusConflict, "conflict", "no next candidate version")
		return
	}
	draftVersion, err := a.store.UpdateDraftSelections(r.Context(), appID, envID,
		draft.Version, map[string]int64{secretID: next})
	if err != nil {
		switch {
		case errors.Is(err, errConflict):
			writeError(w, http.StatusConflict, "conflict", "draft changed; reload and retry")
		default:
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	if err := a.store.MarkRotation(r.Context(), secretID, next); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "secret.rotate", Resource: secretID,
		Result:        fmt.Sprintf("seq=%d draft=%d", next, draftVersion),
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"version_seq": next, "draft_version": draftVersion})
}

// nextVersion returns the candidate version after cur in seqs, wrapping
// around. Unknown current versions restart from the first candidate.
func nextVersion(seqs []int64, cur int64) int64 {
	for i, s := range seqs {
		if s == cur {
			return seqs[(i+1)%len(seqs)]
		}
	}
	return seqs[0]
}

func (a *App) handleUpdateBinding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		UID  *int64 `json:"uid"`
		GID  *int64 `json:"gid"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	path, err := validateBindingPath(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	mode, err := strconv.ParseInt(body.Mode, 8, 64)
	if err != nil || !safeModes[mode] {
		writeError(w, http.StatusBadRequest, "bad_request", "mode must be one of 0400, 0440, 0444, 0600, 0640, 0644")
		return
	}
	uid, gid := int64(0), int64(0)
	if body.UID != nil {
		if *body.UID < 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "uid must be >= 0")
			return
		}
		uid = *body.UID
	}
	if body.GID != nil {
		if *body.GID < 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "gid must be >= 0")
			return
		}
		gid = *body.GID
	}
	draftVersion, err := a.store.UpdateBinding(r.Context(), r.PathValue("secretID"), path, uid, gid, mode)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "secret not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "binding.update", Resource: r.PathValue("secretID"),
		Result: fmt.Sprintf("path=%s draft=%d", path, draftVersion), CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"draft_version": draftVersion, "path": path})
}

// validateBindingPath normalizes and validates a relative POSIX path for a
// File Binding. Absolute paths, ".", "..", empty components, and control
// characters are rejected; no component may exceed 255 bytes.
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

func (a *App) handleGetDraft(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	draft, err := a.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
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
			Binding: bindingDTO{Path: sel.Path, UID: sel.UID, GID: sel.GID, Mode: modeString(sel.Mode)},
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleUpdateDraft(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Selections map[string]int64 `json:"selections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if len(body.Selections) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "selections must not be empty")
		return
	}
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	expected := int64(-1)
	if raw := r.Header.Get("If-Match"); raw != "" {
		expected, _ = strconv.ParseInt(raw, 10, 64)
	}
	draftVersion, err := a.store.UpdateDraftSelections(r.Context(), appID, envID, expected, body.Selections)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		case errors.Is(err, errConflict):
			writeError(w, http.StatusConflict, "conflict", "draft changed; reload and retry")
		case errors.Is(err, errBadSelection):
			writeError(w, http.StatusBadRequest, "bad_request", "selection references an unknown secret or version")
		default:
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft_version": draftVersion})
}

func (a *App) handlePublish(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		OperationReason *operationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	reason, ok := operationReason(body.OperationReason)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	env, err := a.store.GetEnvironment(r.Context(), envID, appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	if env.Protection != "standard" && !a.requireStepUp(w, r) {
		return
	}
	draft, err := a.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	revisions, err := a.store.ListRevisions(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if len(revisions) > 0 && revisions[0].DraftVersion == draft.Version {
		writeError(w, http.StatusConflict, "conflict", "draft has no changes to publish")
		return
	}
	revision, err := a.store.Publish(r.Context(), uuid.NewString(), appID, envID, actorFrom(r), reason)
	if err != nil {
		if errors.Is(err, errNoSecrets) {
			writeError(w, http.StatusBadRequest, "bad_request", "draft has no secrets to publish")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if err := a.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "revision.publish", Resource: revision.ID,
		Result:        fmt.Sprintf("draft=%d files=%d reason=%s", revision.DraftVersion, revision.FileCount, reason.Category),
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, revision)
}

// handleRollback creates a new immutable Bundle Revision from an earlier
// snapshot with the same Operation Reason and Step-up policy as Publish.
func (a *App) handleRollback(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		SourceRevisionID string                `json:"source_revision_id"`
		OperationReason  *operationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SourceRevisionID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "source_revision_id and operation_reason are required")
		return
	}
	reason, ok := operationReason(body.OperationReason)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	env, err := a.store.GetEnvironment(r.Context(), envID, appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	if env.Protection != "standard" && !a.requireStepUp(w, r) {
		return
	}
	revision, err := a.store.Rollback(r.Context(), uuid.NewString(), appID, envID, body.SourceRevisionID, actorFrom(r), reason)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			writeError(w, http.StatusNotFound, "not_found", "source revision not found in this environment")
		case errors.Is(err, errNoSecrets):
			writeError(w, http.StatusBadRequest, "bad_request", "source revision has no files to roll back to")
		default:
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	if err := a.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "revision.rollback", Resource: revision.ID,
		Result:        fmt.Sprintf("source=%s files=%d reason=%s", body.SourceRevisionID, revision.FileCount, reason.Category),
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, revision)
}

func (a *App) handleListRevisions(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	revisions, err := a.store.ListRevisions(r.Context(), appID, envID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, revisions)
}
