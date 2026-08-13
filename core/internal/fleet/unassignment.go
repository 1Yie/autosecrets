package fleet

import (
	"encoding/json"
	"net/http"

	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
)

// handleUnassign starts persistent two-phase Unassignment: Desired State
// delivery stops immediately and per-node cleanup tasks are created.
func (h *Handler) handleUnassign(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
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
	if _, err := h.store.AssignmentByID(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	tasks, err := h.store.Unassign(r.Context(), assignmentID)
	if err != nil {
		middleware.WriteError(w, http.StatusConflict, "conflict", "assignment is not active")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "assignment.unassign", Resource: assignmentID,
		Result: "reason=" + reason.Category, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusAccepted, map[string]any{
		"id": assignmentID, "status": "removing", "tasks": tasks,
	})
}

func (h *Handler) handleUnassignmentState(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
	assignment, err := h.store.AssignmentByID(r.Context(), assignmentID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	tasks, err := h.store.ListUnassignmentTasks(r.Context(), assignmentID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"assignment": assignment, "tasks": tasks,
	})
}

// handleAbandonCleanup is the strongest risk path: it ends control-plane
// waiting but records cleanup_unconfirmed per node, never success.
func (h *Handler) handleAbandonCleanup(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
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
	if _, err := h.store.AssignmentByID(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	if err := h.store.AbandonCleanupConfirmation(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusConflict, "conflict", "no pending cleanup remains")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "assignment.abandon_cleanup", Resource: assignmentID,
		Result: "cleanup_unconfirmed reason=" + reason.Category, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "cleanup_unconfirmed"})
}
