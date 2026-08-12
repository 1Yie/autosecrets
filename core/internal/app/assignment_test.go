package app

import (
	"context"
	"net/http"
	"testing"

	"autosecrets.dev/core/internal/crypto"
)

func assignBody(a authoring, groupID string) map[string]any {
	return map[string]any{
		"group_id": groupID, "application_id": a.appID, "environment_id": a.envID,
		"operation_reason": map[string]string{
			"category": "maintenance", "explanation": "test bundle assignment",
		},
	}
}

// TestAssignmentBindsBundleAndFollowsDesiredRevision locks ADR-0018: an
// Assignment names a Secret Bundle, not a Revision; Publish and Rollback
// advance the Bundle's Desired Revision for every active Assignment.
func TestAssignmentBindsBundleAndFollowsDesiredRevision(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "db_pass", "v1")
	rev1 := ta.publish(t, a)

	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	groupID := g.body["id"].(string)

	legacyBody := ta.do(t, "POST", "/api/v1/assignments",
		map[string]string{"group_id": groupID, "revision_id": rev1}, a.cookie, a.csrf)
	if legacyBody.status != http.StatusBadRequest {
		t.Fatalf("assignment with revision_id must be rejected: %d %s", legacyBody.status, legacyBody.raw)
	}
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	if asg.status != http.StatusCreated {
		t.Fatalf("bundle assignment: %d %s", asg.status, asg.raw)
	}
	if asg.body["revision_id"] != rev1 || asg.body["status"] != "active" {
		t.Fatalf("assignment must follow the current desired revision: %s", asg.raw)
	}
	dup := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	if dup.status != http.StatusConflict {
		t.Fatalf("duplicate bundle assignment must conflict: %d %s", dup.status, dup.raw)
	}

	// Publish v2: every active Assignment follows the new Desired Revision.
	ta.addVersion(t, a, ta.secretID(t, a), "v2")
	rev2 := ta.publish(t, a)
	list := ta.do(t, "GET", "/api/v1/assignments", nil, a.cookie, "")
	rows := parsePage(t, list.raw)
	if len(rows) != 1 || rows[0]["revision_id"] != rev2 {
		t.Fatalf("assignment must follow the new desired revision: %s", list.raw)
	}
}

// TestAssignmentNodeAmbiguity locks the invariant that one Managed Node can
// never receive the same Secret Bundle from two Assignment sources.
func TestAssignmentNodeAmbiguity(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "db_pass", "v1")
	ta.publish(t, a)

	g1 := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	g2 := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g2"}, a.cookie, a.csrf)
	group1, group2 := g1.body["id"].(string), g2.body["id"].(string)
	first := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, group1), a.cookie, a.csrf)
	if first.status != http.StatusCreated {
		t.Fatalf("first assignment: %d %s", first.status, first.raw)
	}
	second := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, group2), a.cookie, a.csrf)
	if second.status != http.StatusCreated {
		t.Fatalf("second group assignment without shared nodes: %d %s", second.status, second.raw)
	}

	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")
	add1 := ta.do(t, "POST", "/api/v1/node-groups/"+group1+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)
	if add1.status != 200 {
		t.Fatalf("add member g1: %d %s", add1.status, add1.raw)
	}
	add2 := ta.do(t, "POST", "/api/v1/node-groups/"+group2+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)
	if add2.status != http.StatusConflict {
		t.Fatalf("ambiguous membership must conflict: %d %s", add2.status, add2.raw)
	}
}

// TestAssignmentRequiresReasonAndProtectedStepUp locks US-232/US-148: every
// Assignment carries an Operation Reason, and Protected Environments need a
// current Step-up Grant.
func TestAssignmentRequiresReasonAndProtectedStepUp(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "db_pass", "v1")
	ta.publish(t, a)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)

	noReason := ta.do(t, "POST", "/api/v1/assignments", map[string]any{
		"group_id": g.body["id"].(string), "application_id": a.appID, "environment_id": a.envID,
	}, a.cookie, a.csrf)
	if noReason.status != http.StatusBadRequest {
		t.Fatalf("assignment without operation reason: %d %s", noReason.status, noReason.raw)
	}
	ok := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, g.body["id"].(string)), a.cookie, a.csrf)
	if ok.status != http.StatusCreated {
		t.Fatalf("assignment with reason: %d %s", ok.status, ok.raw)
	}

	// Protected Environment: assignment requires Step-up.
	prot := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "prod", "protection": "protected"}, a.cookie, a.csrf)
	prod := authoring{appID: a.appID, envID: prot.body["id"].(string), cookie: a.cookie, csrf: a.csrf}
	ta.putPolicy(t, prod)
	ta.createSecret(t, prod, "prod_token", "p1")
	pub := ta.do(t, "POST", publishPath(prod),
		reason("maintenance", "publish the production token"), a.cookie, a.csrf)
	if pub.status != http.StatusCreated {
		t.Fatalf("protected publish: %d %s", pub.status, pub.raw)
	}
	if err := ta.store.RevokeStepUp(context.Background(), crypto.HashToken(a.cookie)); err != nil {
		t.Fatal(err)
	}
	g2 := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g2"}, a.cookie, a.csrf)
	denied := ta.do(t, "POST", "/api/v1/assignments", map[string]any{
		"group_id": g2.body["id"].(string), "application_id": a.appID,
		"environment_id": prod.envID,
		"operation_reason": map[string]string{
			"category": "maintenance", "explanation": "assign the production bundle",
		},
	}, a.cookie, a.csrf)
	if denied.status != http.StatusForbidden || denied.body["code"] != "step_up_required" {
		t.Fatalf("protected assignment without step-up: %d %s", denied.status, denied.raw)
	}
	stepUp := ta.do(t, "POST", "/api/v1/auth/step-up",
		map[string]string{"password": "correct-horse-42"}, a.cookie, a.csrf)
	if stepUp.status != http.StatusOK {
		t.Fatalf("step-up: %d %s", stepUp.status, stepUp.raw)
	}
	allowed := ta.do(t, "POST", "/api/v1/assignments", map[string]any{
		"group_id": g2.body["id"].(string), "application_id": a.appID,
		"environment_id": prod.envID,
		"operation_reason": map[string]string{
			"category": "maintenance", "explanation": "assign the production bundle",
		},
	}, a.cookie, a.csrf)
	if allowed.status != http.StatusCreated {
		t.Fatalf("protected assignment with step-up: %d %s", allowed.status, allowed.raw)
	}
}
