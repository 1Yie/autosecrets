package app

import (
	"net/http"
	"strings"
)

// handleSearch runs the scoped global search: Applications, Environments,
// Managed Nodes, and Node Groups. Results are grouped by type and filtered
// by role in the caller's UI; the server never searches Secret names or
// Audit Events here.
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}
	if len(query) > 128 {
		query = query[:128]
	}
	results, err := a.store.Search(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
