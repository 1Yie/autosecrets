package app

import (
	"net/http"
	"time"

	"autosecrets.dev/core/internal/database"
)

// handleOverview serves one timestamped, read-only projection: asset and
// health counts, risk-sorted attention items derived from current state,
// recent Publish activity, and recent Audit Events (spec: Overview).
func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	counts, err := a.store.OverviewCounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	attention, err := a.store.OverviewAttention(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	revisions, err := a.store.ListAllRevisions(r.Context(), 5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	audit, err := a.store.ListAudit(r.Context(), database.AuditFilter{Limit: 5})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at":     a.now().UTC().Format(time.RFC3339),
		"counts":           counts,
		"attention":        attention,
		"recent_publishes": revisions,
		"recent_audit":     audit,
	})
}
