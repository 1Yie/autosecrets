package app

import (
	"net/http"
	"testing"
)

func assignBody(a authoring, groupID string) map[string]string {
	return map[string]string{
		"group_id": groupID, "application_id": a.appID, "environment_id": a.envID,
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
