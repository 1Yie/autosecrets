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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	if _, err := h.store.AssignmentByID(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "分配不存在")
		return
	}
	tasks, err := h.store.Unassign(r.Context(), assignmentID)
	if err != nil {
		middleware.WriteError(w, http.StatusConflict, "conflict", "该分配已在解除中")
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
		middleware.WriteError(w, http.StatusNotFound, "not_found", "分配不存在")
		return
	}
	tasks, err := h.store.ListUnassignmentTasks(r.Context(), assignmentID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "内部错误")
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
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "无效的 JSON")
		return
	}
	reason, ok := middleware.OperationReasonOr(body.OperationReason)
	if !ok {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "需要提供有效的操作原因分类和 10-500 字符的说明")
		return
	}
	if _, err := h.store.AssignmentByID(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "分配不存在")
		return
	}
	if err := h.store.AbandonCleanupConfirmation(r.Context(), assignmentID); err != nil {
		middleware.WriteError(w, http.StatusConflict, "conflict", "没有待确认的清理任务")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: middleware.ActorFrom(r), Action: "assignment.abandon_cleanup", Resource: assignmentID,
		Result: "cleanup_unconfirmed reason=" + reason.Category, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "cleanup_unconfirmed"})
}
