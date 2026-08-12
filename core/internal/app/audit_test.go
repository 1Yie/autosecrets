package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestAuditCursorPaginationAndFilters locks ADR-0017/0020: Audit Events are
// cursor-paginated with structured actor/action/resource/outcome filters and
// never returned as unbounded arrays.
func TestAuditCursorPaginationAndFilters(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	// authoringSetup already produced several audit events (bootstrap,
	// mfa, login, application, environment); add a couple more for volume.
	ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "storefront"}, a.cookie, a.csrf)
	ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "inventory"}, a.cookie, a.csrf)
	ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "catalog"}, a.cookie, a.csrf)

	page := ta.do(t, "GET", "/api/v1/audit-events?limit=2", nil, a.cookie, "")
	if page.status != http.StatusOK {
		t.Fatalf("audit page: %d %s", page.status, page.raw)
	}
	if page.body["next_cursor"] == "" {
		t.Fatalf("audit page must carry a next_cursor: %s", page.raw)
	}
	items := page.body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("audit page size: %s", page.raw)
	}
	item := items[0].(map[string]any)
	if item["actor_type"] == "" || item["outcome"] == "" || item["actor_display"] == "" {
		t.Fatalf("audit item must be structured: %s", page.raw)
	}

	next := ta.do(t, "GET", "/api/v1/audit-events?limit=2&cursor="+page.body["next_cursor"].(string), nil, a.cookie, "")
	if next.status != http.StatusOK {
		t.Fatalf("audit next page: %d %s", next.status, next.raw)
	}
	nextItems := next.body["items"].([]any)
	if len(nextItems) == 0 {
		t.Fatalf("next page must not repeat the first page: %s", next.raw)
	}
	firstIDs := map[any]bool{}
	for _, raw := range items {
		firstIDs[raw.(map[string]any)["id"]] = true
	}
	for _, raw := range nextItems {
		if firstIDs[raw.(map[string]any)["id"]] {
			t.Fatalf("cursor pagination repeated an item: %s", next.raw)
		}
	}

	filtered := ta.do(t, "GET", "/api/v1/audit-events?action=application.create", nil, a.cookie, "")
	if filtered.status != http.StatusOK {
		t.Fatalf("filtered audit: %d %s", filtered.status, filtered.raw)
	}
	var filteredItems []map[string]any
	if err := json.Unmarshal(filtered.raw, &struct {
		Items *[]map[string]any `json:"items"`
	}{Items: &filteredItems}); err != nil {
		t.Fatal(err)
	}
	if len(filteredItems) != 4 {
		t.Fatalf("expected 4 application.create events, got %d", len(filteredItems))
	}
	for _, item := range filteredItems {
		if item["action"] != "application.create" {
			t.Fatalf("filter leaked a different action: %v", item)
		}
	}
}

// TestGlobalSearch locks the scoped search surface: Applications,
// Environments, Managed Nodes, and Node Groups only.
func TestGlobalSearch(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.do(t, "POST", "/api/v1/applications", map[string]string{"name": "payments"}, a.cookie, a.csrf)
	ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "web-tier"}, a.cookie, a.csrf)
	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	ta.enrollNode(t, tokenFromCommand(command), "node-1")

	byApp := ta.do(t, "GET", "/api/v1/search?q=pay", nil, a.cookie, "")
	if byApp.status != http.StatusOK {
		t.Fatalf("search: %d %s", byApp.status, byApp.raw)
	}
	if !containsSearchResult(t, byApp.raw, "application", "payments") {
		t.Fatalf("search must find the application: %s", byApp.raw)
	}
	byGroup := ta.do(t, "GET", "/api/v1/search?q=web-tier", nil, a.cookie, "")
	if !containsSearchResult(t, byGroup.raw, "node_group", "web-tier") {
		t.Fatalf("search must find the node group: %s", byGroup.raw)
	}
	byNode := ta.do(t, "GET", "/api/v1/search?q=node-1", nil, a.cookie, "")
	if !containsSearchResult(t, byNode.raw, "node", "node-1") {
		t.Fatalf("search must find the node: %s", byNode.raw)
	}
	byEnv := ta.do(t, "GET", "/api/v1/search?q=production", nil, a.cookie, "")
	if !containsSearchResultPrefix(t, byEnv.raw, "environment", "production") {
		t.Fatalf("search must find the environment: %s", byEnv.raw)
	}
}

func containsSearchResultPrefix(t *testing.T, raw []byte, resultType, namePart string) bool {
	t.Helper()
	var body struct {
		Results []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, result := range body.Results {
		if result.Type == resultType && strings.Contains(result.Name, namePart) {
			return true
		}
	}
	return false
}

func containsSearchResult(t *testing.T, raw []byte, resultType, name string) bool {
	t.Helper()
	var body struct {
		Results []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, result := range body.Results {
		if result.Type == resultType && result.Name == name {
			return true
		}
	}
	return false
}
