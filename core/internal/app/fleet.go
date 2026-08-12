package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/store"
	"github.com/google/uuid"
)

func (a *App) handleListNodeGroups(w http.ResponseWriter, r *http.Request) {
	cursor, limit, err := a.pageParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	}
	groups, next, err := a.store.ListNodeGroupsPage(r.Context(), cursor, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": groups, "next_cursor": next})
}

func (a *App) handleCreateNodeGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validName(body.Name, 64) {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required (max 64 chars)")
		return
	}
	id := uuid.NewString()
	if err := a.store.CreateNodeGroup(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeError(w, http.StatusConflict, "duplicate", "node group name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "nodegroup.create", Resource: id,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (a *App) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NodeID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "node_id is required")
		return
	}
	if err := a.store.AddGroupMember(r.Context(), r.PathValue("groupID"), body.NodeID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "node group or node not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict",
				"membership would give a managed node multiple sources for the same application and environment")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member added"})
}

func (a *App) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := a.store.RemoveGroupMember(r.Context(), r.PathValue("groupID"), r.PathValue("nodeID")); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

func (a *App) handleListAssignments(w http.ResponseWriter, r *http.Request) {
	cursor, limit, err := a.pageParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	}
	assignments, next, err := a.store.ListAssignmentsPage(r.Context(), cursor, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": assignments, "next_cursor": next})
}

func (a *App) handleCreateAssignment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GroupID         string                `json:"group_id"`
		ApplicationID   string                `json:"application_id"`
		EnvironmentID   string                `json:"environment_id"`
		OperationReason *operationReasonInput `json:"operation_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.GroupID == "" || body.ApplicationID == "" || body.EnvironmentID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "group_id, application_id, and environment_id are required")
		return
	}
	reason, ok := operationReasonOr(body.OperationReason)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "operation_reason with a valid category and a 10-500 character explanation is required")
		return
	}
	env, err := a.store.GetEnvironment(r.Context(), body.EnvironmentID, body.ApplicationID)
	if err == nil && env.Protection != "standard" && !a.requireStepUp(w, r) {
		return
	}
	asg, err := a.store.CreateAssignment(r.Context(), uuid.NewString(), body.GroupID, body.ApplicationID, body.EnvironmentID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "node group or environment not found")
		case errors.Is(err, store.ErrBadPayload):
			writeError(w, http.StatusBadRequest, "bad_request", "environment has no published revision to assign")
		case errors.Is(err, store.ErrPolicy):
			writeError(w, http.StatusBadRequest, "activation_policy_required",
				"environment must define an activation policy before its first assignment")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "conflict",
				"assignment would give a managed node multiple sources for the same application and environment")
		case errors.Is(err, store.ErrDuplicate):
			writeError(w, http.StatusConflict, "duplicate", "assignment already exists for this node group and bundle")
		default:
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "assignment.create", Resource: asg.ID,
		Result:        fmt.Sprintf("bundle=%s/%s group=%s reason=%s", asg.ApplicationID, asg.EnvironmentID, asg.GroupName, reason.Category),
		CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, asg)
}

func (a *App) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.store.ListNode(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	convergence, err := a.store.AllNodeConvergence(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	counts, err := a.store.NodeAssignmentCounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	now := a.now()
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
		state, unassigned := deriveNodeState(node.LastSeenAt, now, a.cfg.OfflineAfter,
			hasFailed, hasAssignment, allConverged)
		out = append(out, map[string]any{
			"id": node.ID, "name": node.Name, "serial": node.Serial,
			"created_at": node.CreatedAt, "last_seen_at": node.LastSeenAt,
			"desired_etag": node.DesiredETag, "observed_revision": node.ObservedRevision,
			"last_result": node.LastResult, "state": state, "unassigned": unassigned,
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
	cursor, limit, err := a.pageParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	}
	var start int
	if cursor.ID != "" {
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
			next = store.EncodeCursor(last["created_at"].(time.Time), last["id"].(string))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page, "next_cursor": next})
}

func (a *App) handleListAudit(w http.ResponseWriter, r *http.Request) {
	filter := store.AuditFilter{
		Actor:          r.URL.Query().Get("actor"),
		Action:         r.URL.Query().Get("action"),
		Resource:       r.URL.Query().Get("resource"),
		Outcome:        r.URL.Query().Get("outcome"),
		ReasonCategory: r.URL.Query().Get("reason_category"),
	}
	if from := r.URL.Query().Get("from"); from != "" {
		if parsed, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = parsed
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if parsed, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = parsed
		}
	}
	var afterID int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		afterID, _ = strconv.ParseInt(raw, 10, 64)
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			filter.Limit = n
		}
	}
	events, next, err := a.store.ListAuditPage(r.Context(), filter, afterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events, "next_cursor": next})
}

// pageParams parses the shared cursor and limit query parameters.
func (a *App) pageParams(r *http.Request) (store.Cursor, int, error) {
	cursor, err := store.DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		return store.Cursor{}, 0, err
	}
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	return cursor, limit, nil
}
