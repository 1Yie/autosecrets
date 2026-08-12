package app

import (
	"context"
	"net/http"
	"testing"

	"autosecrets.dev/core/internal/crypto"
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

// TestPublishRequiresReasonAndProtectedStepUp locks the Desired State
// risk policy: every Publish needs a valid Operation Reason; Protected and
// Unclassified Environments additionally need a current Step-up Grant.
func TestPublishRequiresReasonAndProtectedStepUp(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "db_pass", "v1")

	noReason := ta.do(t, "POST", publishPath(a), map[string]any{}, a.cookie, a.csrf)
	if noReason.status != http.StatusBadRequest {
		t.Fatalf("publish without operation reason: %d %s", noReason.status, noReason.raw)
	}
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
	first := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "rotate the database password"), a.cookie, a.csrf)
	if first.status != http.StatusCreated {
		t.Fatalf("publish with reason: %d %s", first.status, first.raw)
	}
	if first.body["operation_reason"] == nil {
		t.Fatalf("published revision must echo its operation reason: %s", first.raw)
	}
	unchanged := ta.do(t, "POST", publishPath(a),
		reason("maintenance", "rotate the database password again"), a.cookie, a.csrf)
	if unchanged.status != http.StatusConflict {
		t.Fatalf("unchanged draft publish must conflict: %d %s", unchanged.status, unchanged.raw)
	}

	// Protected Environment: a fresh login grants Step-up, so revoke the
	// grant first to prove the denial, then re-grant via /auth/step-up.
	protected := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "prod", "protection": "protected"}, a.cookie, a.csrf)
	if protected.status != http.StatusCreated {
		t.Fatalf("protected environment: %d %s", protected.status, protected.raw)
	}
	protectedEnv := protected.body["id"].(string)
	prodAuthoring := authoring{appID: a.appID, envID: protectedEnv, cookie: a.cookie, csrf: a.csrf}
	ta.createSecret(t, prodAuthoring, "prod_token", "p1")

	if err := ta.store.RevokeStepUp(context.Background(), crypto.HashToken(a.cookie)); err != nil {
		t.Fatal(err)
	}
	denied := ta.do(t, "POST", publishPath(prodAuthoring),
		reason("maintenance", "rotate the production token"), a.cookie, a.csrf)
	if denied.status != http.StatusForbidden || denied.body["code"] != "step_up_required" {
		t.Fatalf("protected publish without step-up: %d %s", denied.status, denied.raw)
	}
	stepUp := ta.do(t, "POST", "/api/v1/auth/step-up",
		map[string]string{"password": "correct-horse-42"}, a.cookie, a.csrf)
	if stepUp.status != http.StatusOK {
		t.Fatalf("step-up: %d %s", stepUp.status, stepUp.raw)
	}
	allowed := ta.do(t, "POST", publishPath(prodAuthoring),
		reason("maintenance", "rotate the production token"), a.cookie, a.csrf)
	if allowed.status != http.StatusCreated {
		t.Fatalf("protected publish with step-up: %d %s", allowed.status, allowed.raw)
	}

	// Unclassified Environments (legacy migration rows) follow Protected rules.
	legacyEnv := uuid.NewString()
	if err := ta.store.CreateEnvironment(context.Background(), legacyEnv, a.appID, "legacy", "unclassified"); err != nil {
		t.Fatal(err)
	}
	legacyAuthoring := authoring{appID: a.appID, envID: legacyEnv, cookie: a.cookie, csrf: a.csrf}
	ta.createSecret(t, legacyAuthoring, "legacy_token", "l1")
	if err := ta.store.RevokeStepUp(context.Background(), crypto.HashToken(a.cookie)); err != nil {
		t.Fatal(err)
	}
	deniedLegacy := ta.do(t, "POST", publishPath(legacyAuthoring),
		reason("configuration_correction", "classify the legacy environment"), a.cookie, a.csrf)
	if deniedLegacy.status != http.StatusForbidden || deniedLegacy.body["code"] != "step_up_required" {
		t.Fatalf("unclassified publish without step-up: %d %s", deniedLegacy.status, deniedLegacy.raw)
	}
	stepUpAgain := ta.do(t, "POST", "/api/v1/auth/step-up",
		map[string]string{"password": "correct-horse-42"}, a.cookie, a.csrf)
	if stepUpAgain.status != http.StatusOK {
		t.Fatalf("step-up: %d %s", stepUpAgain.status, stepUpAgain.raw)
	}
	allowedLegacy := ta.do(t, "POST", publishPath(legacyAuthoring),
		reason("configuration_correction", "classify the legacy environment"), a.cookie, a.csrf)
	if allowedLegacy.status != http.StatusCreated {
		t.Fatalf("unclassified publish with step-up: %d %s", allowedLegacy.status, allowedLegacy.raw)
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
	if noReason.status != http.StatusBadRequest {
		t.Fatalf("rollback without operation reason: %d %s", noReason.status, noReason.raw)
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
