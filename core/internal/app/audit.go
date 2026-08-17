package app

import (
	"net/http"
	"strconv"
	"time"

	"autosecrets.dev/core/internal/database"
)

// handleListAudit returns cursor-paginated structured Audit Events (ADR-0020).
// Audit is a cross-cutting read model, so it stays in the composition root
// rather than a single domain package.
func (a *App) handleListAudit(w http.ResponseWriter, r *http.Request) {
	filter := database.AuditFilter{
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
	if page := r.URL.Query().Get("page"); page != "" {
		if n, err := strconv.Atoi(page); err == nil {
			filter.Page = n
		}
	}
	events, next, total, err := a.store.ListAuditPage(r.Context(), filter, afterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events, "next_cursor": next, "total": total})
}
