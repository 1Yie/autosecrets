// Package fleet owns the Managed Node domain: Node Groups, Assignments,
// enrollment, convergence, and the Agent-facing delivery surface (ADR-0025).
package fleet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
	"github.com/google/uuid"
)

const (
	tokenTTL = 10 * time.Minute
	certTTL  = 30 * 24 * time.Hour
)

// Config carries the deployment settings the fleet domain needs.
type Config struct {
	PublicAgentURL  string
	ArtifactDir     string
	InstallCurlOpts string
	OfflineAfter    time.Duration
}

type Handler struct {
	store     *database.Store
	mk        *crypto.MasterKey
	ca        *crypto.CA
	signer    *crypto.Signer
	now       func() time.Time
	cfg       Config
	agentBase string
}

func NewHandler(st *database.Store, mk *crypto.MasterKey, ca *crypto.CA, signer *crypto.Signer,
	now func() time.Time, cfg Config, agentBase string) *Handler {
	return &Handler{store: st, mk: mk, ca: ca, signer: signer, now: now, cfg: cfg, agentBase: agentBase}
}

func (h *Handler) handleListNodeGroups(w http.ResponseWriter, r *http.Request) {
	cursor, limit, page, err := middleware.PageParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的分页游标")
		return
	}
	groups, next, total, err := h.store.ListNodeGroupsPage(r.Context(), cursor, limit, page)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"items": groups, "next_cursor": next, "total": total})
}

func (h *Handler) handleCreateNodeGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 64 个字符）")
		return
	}
	id := uuid.NewString()
	if err := h.store.CreateNodeGroup(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		if errors.Is(err, database.ErrDuplicate) {
			middleware.WriteError(w, http.StatusConflict, "duplicate", "node group name already exists")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "nodegroup.create", Resource: id,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (h *Handler) handleDeleteNodeGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("groupID")
	if err := h.store.DeleteNodeGroup(r.Context(), groupID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		}
		if errors.Is(err, database.ErrConflict) {
			middleware.WriteError(w, http.StatusConflict, "conflict", "node group still has an active assignment")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "nodegroup.delete", Resource: groupID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NodeID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "node_id is required")
		return
	}
	if err := h.store.AddGroupMember(r.Context(), r.PathValue("groupID"), body.NodeID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "node group or node not found")
			return
		}
		if errors.Is(err, database.ErrConflict) {
			middleware.WriteError(w, http.StatusConflict, "conflict",
				"membership would give a managed node multiple sources for the same application and environment")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "member added"})
}

func (h *Handler) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RemoveGroupMember(r.Context(), r.PathValue("groupID"), r.PathValue("nodeID")); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

func (h *Handler) handleListAssignments(w http.ResponseWriter, r *http.Request) {
	cursor, limit, page, err := middleware.PageParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的分页游标")
		return
	}
	assignments, next, total, err := h.store.ListAssignmentsPage(r.Context(), cursor, limit, page)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"items": assignments, "next_cursor": next, "total": total})
}

func (h *Handler) handleCreateAssignment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GroupID         string                           `json:"group_id"`
		ApplicationID   string                           `json:"application_id"`
		EnvironmentID   string                           `json:"environment_id"`
		OperationReason *middleware.OperationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.GroupID == "" || body.ApplicationID == "" || body.EnvironmentID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "group_id, application_id, and environment_id are required")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	asg, err := h.store.CreateAssignment(r.Context(), uuid.NewString(), body.GroupID, body.ApplicationID, body.EnvironmentID)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrNotFound):
			middleware.WriteError(w, http.StatusNotFound, "not_found", "node group or environment not found")
		case errors.Is(err, database.ErrBadPayload):
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "environment has no published revision to assign")
		case errors.Is(err, database.ErrConflict):
			middleware.WriteError(w, http.StatusConflict, "conflict",
				"assignment would give a managed node multiple sources for the same application and environment")
		case errors.Is(err, database.ErrDuplicate):
			middleware.WriteError(w, http.StatusConflict, "duplicate", "assignment already exists for this node group and bundle")
		default:
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		}
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "assignment.create", Resource: asg.ID,
		Result:        fmt.Sprintf("bundle=%s/%s group=%s reason=%s", asg.ApplicationID, asg.EnvironmentID, asg.GroupName, reason.Category),
		CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, asg)
}

func (h *Handler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.ListNode(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	convergence, err := h.store.AllNodeConvergence(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	counts, err := h.store.NodeAssignmentCounts(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	now := h.now()
	out := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		rows := convergence[node.ID]
		hasFailed := false
		allConverged := len(rows) > 0
		for _, row := range rows {
			if row.Result == "failed" {
				hasFailed = true
			}
			if row.DesiredRevision != row.ObservedRevision || row.ObservedRevision == "" {
				allConverged = false
			}
		}
		hasAssignment := counts[node.ID] > 0
		state, unassigned := deriveNodeState(node.LastSeenAt, now, h.cfg.OfflineAfter,
			hasFailed, hasAssignment, allConverged)
		out = append(out, map[string]any{
			"id": node.ID, "name": node.Name, "serial": node.Serial,
			"created_at": node.CreatedAt, "last_seen_at": node.LastSeenAt,
			"desired_etag": node.DesiredETag, "observed_revision": node.ObservedRevision,
			"last_result": node.LastResult, "state": state, "unassigned": unassigned,
			"poll_interval_seconds": node.PollIntervalSeconds,
			"bundle_dir":            node.BundleDir, "enrolled": node.Enrolled,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i]["created_at"].(time.Time)
		right := out[j]["created_at"].(time.Time)
		if !left.Equal(right) {
			return left.After(right)
		}
		return out[i]["id"].(string) > out[j]["id"].(string)
	})
	cursor, limit, pageNum, err := middleware.PageParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的分页游标")
		return
	}
	var start int
	if pageNum > 1 && cursor.ID == "" {
		start = (pageNum - 1) * limit
	} else if cursor.ID != "" {
		for i, item := range out {
			at := item["created_at"].(time.Time)
			if item["id"].(string) == cursor.ID && (cursor.At.IsZero() || at.Equal(cursor.At)) {
				start = i + 1
				break
			}
		}
	}
	page := out
	next := ""
	if start < len(out) {
		end := start + limit
		if end > len(out) {
			end = len(out)
		}
		page = out[start:end]
		if end < len(out) {
			last := out[end-1]
			next = database.EncodeCursor(last["created_at"].(time.Time), last["id"].(string))
		}
	} else {
		page = []map[string]any{}
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"items": page, "next_cursor": next, "total": len(out)})
}

func (h *Handler) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		BundleDir string `json:"bundle_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !middleware.ValidName(body.Name, 64) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 64 个字符）")
		return
	}
	if err := validateBundleDir(body.BundleDir); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id := uuid.NewString()
	name := strings.TrimSpace(body.Name)
	if err := h.store.CreatePendingNode(r.Context(), id, name, strings.TrimSpace(body.BundleDir)); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	node, err := h.store.GetNode(r.Context(), id)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "node.create", Resource: id,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusCreated, nodeSettingsJSON(node))
}

func (h *Handler) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("nodeID")
	if err := h.store.DeleteNode(r.Context(), nodeID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "node.delete", Resource: nodeID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateNode adjusts node-level settings such as the name, bundle
// directory, and Agent polling interval. Returns 404 for unknown nodes.
func (h *Handler) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("nodeID")
	var body struct {
		Name                *string `json:"name"`
		BundleDir           *string `json:"bundle_dir"`
		PollIntervalSeconds *int    `json:"poll_interval_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		(body.Name == nil && body.BundleDir == nil && body.PollIntervalSeconds == nil) {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "至少填写一项")
		return
	}
	if body.PollIntervalSeconds != nil {
		if *body.PollIntervalSeconds < 5 || *body.PollIntervalSeconds > 86400 {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request",
				"poll_interval_seconds must be between 5 and 86400")
			return
		}
		if err := h.store.SetNodePollInterval(r.Context(), nodeID, *body.PollIntervalSeconds); err != nil {
			writeNodeStoreError(w, err)
			return
		}
	}
	if body.Name != nil {
		if !middleware.ValidName(*body.Name, 64) {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", "名称不能为空（最多 64 个字符）")
			return
		}
		if err := h.store.RenameNode(r.Context(), nodeID, strings.TrimSpace(*body.Name)); err != nil {
			writeNodeStoreError(w, err)
			return
		}
	}
	if body.BundleDir != nil {
		if err := validateBundleDir(*body.BundleDir); err != nil {
			middleware.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := h.store.SetNodeBundleDir(r.Context(), nodeID, strings.TrimSpace(*body.BundleDir)); err != nil {
			writeNodeStoreError(w, err)
			return
		}
	}
	node, err := h.store.GetNode(r.Context(), nodeID)
	if err != nil {
		writeNodeStoreError(w, err)
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "node.update", Resource: nodeID,
		Result: "ok", CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, nodeSettingsJSON(node))
}

func nodeSettingsJSON(node *database.Node) map[string]any {
	state := "never_online"
	if node.Enrolled && node.LastSeenAt != nil {
		state = "offline"
	}
	return map[string]any{
		"id": node.ID, "name": node.Name, "serial": node.Serial,
		"created_at": node.CreatedAt, "last_seen_at": node.LastSeenAt,
		"desired_etag": node.DesiredETag, "observed_revision": node.ObservedRevision,
		"last_result": node.LastResult, "state": state, "unassigned": true,
		"poll_interval_seconds": node.PollIntervalSeconds,
		"bundle_dir":            node.BundleDir, "enrolled": node.Enrolled,
	}
}

func writeNodeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, database.ErrNotFound) {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
}

// Register mounts the management and Agent routes owned by the fleet domain.
func (h *Handler) Register(mux *http.ServeMux, mgmtBase, agentBase string,
	requireSession, agentAuth func(http.Handler) http.Handler) {
	mux.Handle("GET "+mgmtBase+"/node-groups", requireSession(http.HandlerFunc(h.handleListNodeGroups)))
	mux.Handle("POST "+mgmtBase+"/node-groups", requireSession(http.HandlerFunc(h.handleCreateNodeGroup)))
	mux.Handle("DELETE "+mgmtBase+"/node-groups/{groupID}", requireSession(http.HandlerFunc(h.handleDeleteNodeGroup)))
	mux.Handle("POST "+mgmtBase+"/node-groups/{groupID}/nodes", requireSession(http.HandlerFunc(h.handleAddGroupMember)))
	mux.Handle("DELETE "+mgmtBase+"/node-groups/{groupID}/nodes/{nodeID}", requireSession(http.HandlerFunc(h.handleRemoveGroupMember)))
	mux.Handle("GET "+mgmtBase+"/assignments", requireSession(http.HandlerFunc(h.handleListAssignments)))
	mux.Handle("POST "+mgmtBase+"/assignments", requireSession(http.HandlerFunc(h.handleCreateAssignment)))
	mux.Handle("GET "+mgmtBase+"/assignments/{assignmentID}", requireSession(http.HandlerFunc(h.handleUnassignmentState)))
	mux.Handle("POST "+mgmtBase+"/assignments/{assignmentID}/unassign", requireSession(http.HandlerFunc(h.handleUnassign)))
	mux.Handle("POST "+mgmtBase+"/assignments/{assignmentID}/abandon-cleanup", requireSession(http.HandlerFunc(h.handleAbandonCleanup)))
	mux.Handle("GET "+mgmtBase+"/nodes", requireSession(http.HandlerFunc(h.handleListNodes)))
	mux.Handle("POST "+mgmtBase+"/nodes", requireSession(http.HandlerFunc(h.handleCreateNode)))
	mux.Handle("PATCH "+mgmtBase+"/nodes/{nodeID}", requireSession(http.HandlerFunc(h.handleUpdateNode)))
	mux.Handle("DELETE "+mgmtBase+"/nodes/{nodeID}", requireSession(http.HandlerFunc(h.handleDeleteNode)))
	mux.Handle("POST "+mgmtBase+"/nodes/install-command", requireSession(http.HandlerFunc(h.handleInstallCommand)))
	mux.Handle("POST "+mgmtBase+"/nodes/{nodeID}/install-command", requireSession(http.HandlerFunc(h.handleNodeInstallCommand)))

	mux.HandleFunc("GET "+agentBase+"/install.sh", h.handleInstallScript)
	mux.HandleFunc("GET "+agentBase+"/ca.pem", h.handleCAPEM)
	mux.HandleFunc("GET "+agentBase+"/artifacts/{name}", h.handleArtifact)
	mux.HandleFunc("POST "+agentBase+"/enroll", h.handleEnroll)
	mux.Handle("GET "+agentBase+"/desired", agentAuth(http.HandlerFunc(h.handleDesired)))
	mux.Handle("POST "+agentBase+"/nodes/{nodeID}/reports", agentAuth(http.HandlerFunc(h.handleReport)))
	mux.Handle("POST "+agentBase+"/nodes/{nodeID}/cleanup", agentAuth(http.HandlerFunc(h.handleCleanupReport)))
	mux.Handle("POST "+agentBase+"/nodes/{nodeID}/heartbeat", agentAuth(http.HandlerFunc(h.handleHeartbeat)))
}
