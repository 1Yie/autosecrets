package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"autosecrets.dev/core/internal/store"
)

var policyActions = map[string]bool{"none": true, "reload": true, "restart": true}

func (a *App) handleGetActivationPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := a.store.GetActivationPolicy(r.Context(), r.PathValue("envID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "activation policy not configured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": policy.Action, "units": policy.Units})
}

// handlePutActivationPolicy stores the ordered 1-5 unit list and the bounded
// none/reload/restart action. Protected Environments additionally need a
// current Step-up Grant; every policy change needs an Operation Reason.
func (a *App) handlePutActivationPolicy(w http.ResponseWriter, r *http.Request) {
	appID, envID := r.PathValue("appID"), r.PathValue("envID")
	var body struct {
		Action          string                `json:"action"`
		Units           []string              `json:"units"`
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
	action := strings.ToLower(strings.TrimSpace(body.Action))
	if !policyActions[action] {
		writeError(w, http.StatusBadRequest, "bad_request", "action must be one of none, reload, restart")
		return
	}
	if len(body.Units) < 1 || len(body.Units) > 5 {
		writeError(w, http.StatusBadRequest, "bad_request", "units must contain 1-5 systemd unit names")
		return
	}
	units := make([]string, 0, len(body.Units))
	for _, unit := range body.Units {
		unit = strings.TrimSpace(unit)
		if unit == "" || strings.ContainsAny(unit, " \t\n;|&$`\"'") || strings.Contains(unit, "..") {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid systemd unit name")
			return
		}
		units = append(units, unit)
	}
	env, err := a.store.GetEnvironment(r.Context(), envID, appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "application or environment not found")
		return
	}
	if env.Protection != "standard" && !a.requireStepUp(w, r) {
		return
	}
	if err := a.store.SaveActivationPolicy(r.Context(), store.ActivationPolicy{
		EnvironmentID: envID, Action: action, Units: units,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "activation_policy.update", Resource: envID,
		Result: action + " reason=" + reason.Category, CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "units": units})
}

// handleUnassign starts persistent two-phase Unassignment: Desired State
// delivery stops immediately and per-node cleanup tasks are created.
func (a *App) handleUnassign(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
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
	assignment, err := a.store.AssignmentByID(r.Context(), assignmentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	env, err := a.store.GetEnvironment(r.Context(), assignment.EnvironmentID, assignment.ApplicationID)
	if err == nil && env.Protection != "standard" && !a.requireStepUp(w, r) {
		return
	}
	tasks, err := a.store.Unassign(r.Context(), assignmentID)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "assignment is not active")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "assignment.unassign", Resource: assignmentID,
		Result: "reason=" + reason.Category, CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id": assignmentID, "status": "removing", "tasks": tasks,
	})
}

func (a *App) handleUnassignmentState(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
	assignment, err := a.store.AssignmentByID(r.Context(), assignmentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	tasks, err := a.store.ListUnassignmentTasks(r.Context(), assignmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"assignment": assignment, "tasks": tasks,
	})
}

// handleAbandonCleanup is the strongest risk path: it ends control-plane
// waiting but records cleanup_unconfirmed per node, never success.
func (a *App) handleAbandonCleanup(w http.ResponseWriter, r *http.Request) {
	assignmentID := r.PathValue("assignmentID")
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
	if !a.requireStepUp(w, r) {
		return
	}
	if _, err := a.store.AssignmentByID(r.Context(), assignmentID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "assignment not found")
		return
	}
	if err := a.store.AbandonCleanupConfirmation(r.Context(), assignmentID); err != nil {
		writeError(w, http.StatusConflict, "conflict", "no pending cleanup remains")
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: actorFrom(r), Action: "assignment.abandon_cleanup", Resource: assignmentID,
		Result: "cleanup_unconfirmed reason=" + reason.Category, CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleanup_unconfirmed"})
}
