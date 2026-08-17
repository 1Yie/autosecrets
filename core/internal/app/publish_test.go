package app

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func publishPath(a authoring) string {
	return "/api/v1/applications/" + a.appID + "/environments/" + a.envID + "/publish"
}

func reason(category, explanation string) map[string]any {
	return map[string]any{
		"operation_reason": map[string]string{
			"category": category, "explanation": explanation,
		},
	}
}

// TestPublishOptionalReason locks the risk policy after
// the 2026-08 product decision: Operation Reason is optional (omitted ->
// 'other', malformed -> rejected).
func TestPublishOptionalReason(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "db_pass", "v1")

	badCategory := ta.do(t, "POST", publishPath(a),
		reason("emergency", "rotate the database password"), a.cookie, a.csrf)
	if badCategory.status != http.StatusBadRequest {
		t.Fatalf("unknown reason category: %d %s", badCategory.status, badCategory.raw)
	}
	shortExplanation := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "short"), a.cookie, a.csrf)
	if shortExplanation.status != http.StatusBadRequest {
		t.Fatalf("short reason explanation: %d %s", shortExplanation.status, shortExplanation.raw)
	}
	// Omitted reasons are fine: the publish succeeds with an 'other' default.
	noReason := ta.do(t, "POST", publishPath(a), map[string]any{}, a.cookie, a.csrf)
	if noReason.status != http.StatusCreated {
		t.Fatalf("publish without operation reason must succeed: %d %s", noReason.status, noReason.raw)
	}
	recorded := noReason.body["operation_reason"].(map[string]any)
	if recorded["category"] != "other" {
		t.Fatalf("omitted reason must record the other default: %s", noReason.raw)
	}
	unchanged := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "rotate the database password again"), a.cookie, a.csrf)
	if unchanged.status != http.StatusConflict {
		t.Fatalf("unchanged draft publish must conflict: %d %s", unchanged.status, unchanged.raw)
	}
}

// TestRollbackCreatesNewRevisionFromSnapshot locks ADR-0019: Rollback never
// mutates history; it freezes an earlier snapshot into a new Revision with
// the same Operation Reason policy as Publish.
func TestRollbackCreatesNewRevisionFromSnapshot(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "db_pass", "v1")
	first := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "initial database password"), a.cookie, a.csrf)
	if first.status != http.StatusCreated {
		t.Fatalf("first publish: %d %s", first.status, first.raw)
	}
	firstID := first.body["id"].(string)

	version := ta.do(t, "POST", "/api/v1/secrets/"+ta.secretID(t, a)+"/versions",
		map[string]string{"value": "v2"}, a.cookie, a.csrf)
	if version.status != http.StatusCreated {
		t.Fatalf("create version: %d %s", version.status, version.raw)
	}
	second := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "rotate the database password"), a.cookie, a.csrf)
	if second.status != http.StatusCreated {
		t.Fatalf("second publish: %d %s", second.status, second.raw)
	}

	noReason := ta.do(t, "POST", rollbackPath(a),
		map[string]any{"source_revision_id": firstID}, a.cookie, a.csrf)
	if noReason.status != http.StatusCreated {
		t.Fatalf("rollback without operation reason must succeed: %d %s", noReason.status, noReason.raw)
	}
	missing := ta.do(t, "POST", rollbackPath(a),
		map[string]any{
			"source_revision_id": uuid.NewString(),
			"operation_reason": map[string]string{
				"category": "incident_response", "explanation": "restore the previous working state",
			},
		}, a.cookie, a.csrf)
	if missing.status != http.StatusNotFound {
		t.Fatalf("rollback to unknown revision: %d %s", missing.status, missing.raw)
	}
	rolled := ta.do(t, "POST", rollbackPath(a),
		map[string]any{
			"source_revision_id": firstID,
			"operation_reason": map[string]string{
				"category": "incident_response", "explanation": "restore the previous working state",
			},
		}, a.cookie, a.csrf)
	if rolled.status != http.StatusCreated {
		t.Fatalf("rollback: %d %s", rolled.status, rolled.raw)
	}
	if rolled.body["id"] == firstID || rolled.body["file_count"] != float64(1) {
		t.Fatalf("rollback must freeze a new revision from the snapshot: %s", rolled.raw)
	}
	if rolled.body["operation_reason"] == nil {
		t.Fatalf("rollback revision must carry its operation reason: %s", rolled.raw)
	}
}

func rollbackPath(a authoring) string {
	return "/api/v1/applications/" + a.appID + "/environments/" + a.envID + "/rollback"
}
