package app

import (
	"net/http"
	"testing"
	"time"
)

// TestOverviewProjection locks the read-only Overview contract: one
// timestamped projection with asset counts, risk-sorted attention items
// derived from current state, recent Publish activity, and recent Audit
// Events. Attention items cannot be dismissed because they are not stored.
func TestOverviewProjection(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	a := ta.authoringSetup(t)

	overview := ta.do(t, "GET", "/api/v1/overview", nil, a.cookie, "")
	if overview.status != http.StatusOK {
		t.Fatalf("overview: %d %s", overview.status, overview.raw)
	}
	body := overview.body
	if body["generated_at"] == "" {
		t.Fatalf("overview must carry generated_at: %s", overview.raw)
	}
	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("overview counts missing: %s", overview.raw)
	}
	if counts["applications"] != float64(1) || counts["environments"] != float64(1) {
		t.Fatalf("overview counts: %s", overview.raw)
	}

	// A registered node that never came online.
	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	ta.enrollNode(t, tokenFromCommand(command), "node-1")

	overview = ta.do(t, "GET", "/api/v1/overview", nil, a.cookie, "")
	attention := overview.body["attention"].([]any)
	kinds := map[string]bool{}
	for _, raw := range attention {
		kinds[raw.(map[string]any)["kind"].(string)] = true
	}
	if !kinds["unassigned_node"] {
		t.Fatalf("registered unassigned node must be an attention item: %s", overview.raw)
	}
}
