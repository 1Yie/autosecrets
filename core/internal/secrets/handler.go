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
	cursor, limit, page, err := middleware.PageParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的分页游标")
		return
	}
	rows, next, total, err := h.store.ListApplicationsPage(r.Context(), cursor, limit, page)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"items": rows, "next_cursor": next, "total": total})
}

func (h *Handler) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 64 个字符）")
		return
	}
	id := uuid.NewString()
	if err := h.store.CreateApplication(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		middleware.WriteError(w, http.StatusConflict, "duplicate", "应用名称已存在")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "application.create", Resource: id,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func writeDeleteErr(w http.ResponseWriter, err error, notFound, conflict string) {
	if errors.Is(err, database.ErrNotFound) {
		middleware.WriteError(w, http.StatusNotFound, "not_found", notFound)
		return
	}
	if errors.Is(err, database.ErrConflict) {
		middleware.WriteError(w, http.StatusConflict, "conflict", conflict)
		return
	}
	middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
}

func (h *Handler) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")
	if err := h.store.DeleteApplication(r.Context(), appID); err != nil {
		writeDeleteErr(w, err, "应用不存在", "应用仍有关联的分配，无法删除")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "application.delete", Resource: appID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	writeNoContent(w)
}

func (h *Handler) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")
	app, err := h.store.GetApplication(r.Context(), appID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用不存在")
		return
	}
	envs, err := h.store.ListEnvironments(r.Context(), appID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"id": app.ID, "name": app.Name,
		"environments": envs,
	})
}

func (h *Handler) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 64 个字符）")
		return
	}
	appID := r.PathValue("appID")
	if _, err := h.store.GetApplication(r.Context(), appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用不存在")
		return
	}
	id := uuid.NewString()
	name := strings.TrimSpace(body.Name)
	if err := h.store.CreateEnvironment(r.Context(), id, appID, name); err != nil {
		middleware.WriteError(w, http.StatusConflict, "duplicate", "该应用中已存在同名环境")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "environment.create", Resource: id,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{
		"id": id, "name": name,
	})
}

func (h *Handler) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	if err := h.store.DeleteEnvironment(r.Context(), envID, appID); err != nil {
		writeDeleteErr(w, err, "应用或环境不存在", "环境仍有关联的分配，无法删除")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "environment.delete", Resource: envID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	writeNoContent(w)
}

func (h *Handler) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	secrets, err := h.store.ListSecrets(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
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

func (h *Handler) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	secretID := r.PathValue("secretID")
	if err := h.store.DeleteSecret(r.Context(), secretID); err != nil {
		writeDeleteErr(w, err, "密钥不存在", "secret still belongs to an active assignment")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "secret.delete", Resource: secretID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	writeNoContent(w)
}

func (h *Handler) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 128) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 128 个字符）")
		return
	}
	if strings.ContainsAny(body.Name, "/\x00") {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "密钥名称不能包含 '/' 或 NUL 字符")
		return
	}
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		return
	}
	secretID := uuid.NewString()
	versionID := uuid.NewString()
	if err := h.createSecretWithValue(r.Context(), secretID, versionID, appID, envID, strings.TrimSpace(body.Name), []byte(body.Value)); err != nil {
		if errors.Is(err, database.ErrDuplicate) {
			middleware.WriteError(w, http.StatusConflict, "duplicate", "该环境中已存在同名密钥")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	secretID := r.PathValue("secretID")
	wrapped, nonces, ct, err := h.mk.Seal([]byte(body.Value))
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	seq, draftVersion, err := h.store.AddSecretVersion(r.Context(), uuid.NewString(), secretID, wrapped, nonces, ct)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "密钥不存在")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	path, err := validateBindingPath(body.Path)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	mode, err := strconv.ParseInt(body.Mode, 8, 64)
	if err != nil || !safeModes[mode] {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "mode 必须是 0400、0440、0444、0600、0640、0644 之一")
		return
	}
	uid, gid := int64(0), int64(0)
	if body.UID != nil {
		if *body.UID < 0 {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "uid 不能小于 0")
			return
		}
		uid = *body.UID
	}
	if body.GID != nil {
		if *body.GID < 0 {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "gid 不能小于 0")
			return
		}
		gid = *body.GID
	}
	draftVersion, err := h.store.UpdateBinding(r.Context(), r.PathValue("secretID"), path, uid, gid, mode)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "密钥不存在")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		return "", errors.New("路径不能为空")
	}
	if strings.HasPrefix(p, "/") {
		return "", errors.New("路径必须是相对路径")
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("路径不能包含空、'.' 或 '..' 组件")
		}
		if len(part) > 255 {
			return "", errors.New("路径组件超过 255 字节")
		}
		for _, c := range part {
			if c < 0x20 || c == 0x7f {
				return "", errors.New("路径不能包含控制字符")
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
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	if len(body.Selections) == 0 {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "请先选择要发布的密钥")
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
			middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		case errors.Is(err, database.ErrConflict):
			middleware.WriteError(w, http.StatusConflict, "conflict", "草稿已变更，请刷新后重试")
		case errors.Is(err, database.ErrBadPayload):
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "选择项引用了不存在的密钥或版本")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		return
	}
	draft, err := h.store.GetDraft(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		return
	}
	revisions, err := h.store.ListRevisions(r.Context(), appID, envID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	if len(revisions) > 0 && revisions[0].DraftVersion == draft.Version {
		middleware.WriteError(w, http.StatusConflict, "conflict", "草稿没有需要发布的变更")
		return
	}
	revision, err := h.store.Publish(r.Context(), uuid.NewString(), appID, envID, middleware.ActorFrom(r), reason)
	if err != nil {
		if errors.Is(err, database.ErrNoSecrets) {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "草稿中没有可发布的密钥")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	if err := h.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "缺少 source_revision_id 或操作原因")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		return
	}
	revision, err := h.store.Rollback(r.Context(), uuid.NewString(), appID, envID, body.SourceRevisionID, middleware.ActorFrom(r), reason)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusNotFound, "not_found", "该环境中找不到源版本")
		case errors.Is(err, database.ErrNoSecrets):
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "源版本没有可回滚的文件")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		}
		return
	}
	if err := h.store.AdvanceDesiredRevision(r.Context(), appID, envID, revision.ID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, revisions)
}

func (h *Handler) handleGetActivationPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.store.GetActivationPolicy(r.Context(), r.PathValue("envID"))
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "未配置激活策略")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	action := strings.ToLower(strings.TrimSpace(body.Action))
	if !policyActions[action] {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "action 必须是 none、reload、restart 之一")
		return
	}
	if len(body.Units) < 1 || len(body.Units) > 5 {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "units 需要包含 1-5 个 systemd 单元名")
		return
	}
	units := make([]string, 0, len(body.Units))
	for _, unit := range body.Units {
		unit = strings.TrimSpace(unit)
		if unit == "" || strings.ContainsAny(unit, " \t\n;|&$`\"'") || strings.Contains(unit, "..") {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 systemd 单元名")
			return
		}
		units = append(units, unit)
	}
	if _, err := h.store.GetEnvironment(r.Context(), envID, appID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "应用或环境不存在")
		return
	}
	if err := h.store.SaveActivationPolicy(r.Context(), database.ActivationPolicy{
		EnvironmentID: envID, Action: action, Units: units,
	}); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
	mux.Handle("DELETE "+base+"/applications/{appID}", requireSession(http.HandlerFunc(h.handleDeleteApplication)))
	mux.Handle("POST "+base+"/applications/{appID}/environments", requireSession(http.HandlerFunc(h.handleCreateEnvironment)))
	mux.Handle("DELETE "+base+"/applications/{appID}/environments/{envID}", requireSession(http.HandlerFunc(h.handleDeleteEnvironment)))
	mux.Handle("GET "+base+"/applications/{appID}/environments/{envID}/secrets", requireSession(http.HandlerFunc(h.handleListSecrets)))
	mux.Handle("POST "+base+"/applications/{appID}/environments/{envID}/secrets", requireSession(http.HandlerFunc(h.handleCreateSecret)))
	mux.Handle("DELETE "+base+"/secrets/{secretID}", requireSession(http.HandlerFunc(h.handleDeleteSecret)))
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
