package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"autosecrets.dev/core/internal/store"
	"github.com/google/uuid"
)

func (a *App) handleListNodeGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := a.store.ListNodeGroups(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (a *App) handleCreateNodeGroup(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validName(body.Name, 64) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required (max 64 chars)"})
		return
	}
	id := uuid.NewString()
	if err := a.store.CreateNodeGroup(r.Context(), id, strings.TrimSpace(body.Name)); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "node group name already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "nodegroup.create", Resource: id,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (a *App) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	var body struct{ NodeID string `json:"node_id"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
		return
	}
	if err := a.store.AddGroupMember(r.Context(), r.PathValue("groupID"), body.NodeID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node group or node not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member added"})
}

func (a *App) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := a.store.RemoveGroupMember(r.Context(), r.PathValue("groupID"), r.PathValue("nodeID")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "member removed"})
}

func (a *App) handleListAssignments(w http.ResponseWriter, r *http.Request) {
	assignments, err := a.store.ListAssignments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

func (a *App) handleCreateAssignment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GroupID    string `json:"group_id"`
		RevisionID string `json:"revision_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.GroupID == "" || body.RevisionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "group_id and revision_id are required"})
		return
	}
	id := uuid.NewString()
	if err := a.store.CreateAssignment(r.Context(), id, body.GroupID, body.RevisionID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node group or revision not found"})
		case errors.Is(err, store.ErrDuplicate):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "assignment already exists for this group and revision"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "assignment.create", Resource: id,
		Result: "ok", CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "group_id": body.GroupID, "revision_id": body.RevisionID})
}

func (a *App) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.store.ListNode(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (a *App) handleListAudit(w http.ResponseWriter, r *http.Request) {
	filter := store.AuditFilter{
		Actor:  r.URL.Query().Get("actor"),
		Action: r.URL.Query().Get("action"),
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			filter.Limit = n
		}
	}
	events, err := a.store.ListAudit(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, events)
}
