package app

import (
	"net/http"
	"testing"
)

// --- Fleet ----------------------------------------------------------------

func TestNodeGroupAndAssignment(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "token", "v")
	ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "test assignment flow"), a.cookie, a.csrf)

	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "web-tier"}, a.cookie, a.csrf)
	if g.status != 201 {
		t.Fatalf("group: %d %s", g.status, g.raw)
	}
	groupID := g.body["id"].(string)
	as := ta.do(t, "POST", "/api/v1/assignments",
		assignBody(a, groupID), a.cookie, a.csrf)
	if as.status != 201 {
		t.Fatalf("assignment: %d %s", as.status, as.raw)
	}
	dup := ta.do(t, "POST", "/api/v1/assignments",
		assignBody(a, groupID), a.cookie, a.csrf)
	if dup.status != http.StatusConflict {
		t.Fatalf("duplicate assignment must 409: %d", dup.status)
	}
}

func TestNodePollInterval(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	node, nodeID := ta.enrolledNode(t, a, "web-1")

	// Default interval is advertised on list and heartbeat.
	nodes := ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	rows := parsePage(t, nodes.raw)
	if len(rows) != 1 || rows[0]["poll_interval_seconds"] != float64(15) {
		t.Fatalf("default poll interval: %s", nodes.raw)
	}
	beat := ta.doAgent(t, "POST", "/agent/v1/nodes/"+nodeID+"/heartbeat", node.serial, nil)
	if beat.status != http.StatusOK || beat.body["poll_interval_seconds"] != float64(15) {
		t.Fatalf("heartbeat default interval: %d %s", beat.status, beat.raw)
	}

	// Update the interval and observe it everywhere.
	patch := ta.do(t, "PATCH", "/api/v1/nodes/"+nodeID,
		map[string]int{"poll_interval_seconds": 60}, a.cookie, a.csrf)
	if patch.status != http.StatusOK || patch.body["poll_interval_seconds"] != float64(60) {
		t.Fatalf("patch interval: %d %s", patch.status, patch.raw)
	}
	nodes = ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	rows = parsePage(t, nodes.raw)
	if len(rows) != 1 || rows[0]["poll_interval_seconds"] != float64(60) {
		t.Fatalf("updated poll interval in list: %s", nodes.raw)
	}
	beat = ta.doAgent(t, "POST", "/agent/v1/nodes/"+nodeID+"/heartbeat", node.serial, nil)
	if beat.status != http.StatusOK || beat.body["poll_interval_seconds"] != float64(60) {
		t.Fatalf("heartbeat updated interval: %d %s", beat.status, beat.raw)
	}

	// The desired endpoint advertises the interval alongside the envelopes.
	desired := ta.doAgent(t, "GET", "/agent/v1/desired", node.serial, nil)
	if desired.status != http.StatusOK || desired.body["poll_interval_seconds"] != float64(60) {
		t.Fatalf("desired interval: %d %s", desired.status, desired.raw)
	}
}

func TestNodePollIntervalValidation(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	node, nodeID := ta.enrolledNode(t, a, "web-1")

	for _, seconds := range []int{4, 86401, 0, -30} {
		bad := ta.do(t, "PATCH", "/api/v1/nodes/"+nodeID,
			map[string]int{"poll_interval_seconds": seconds}, a.cookie, a.csrf)
		if bad.status != http.StatusBadRequest {
			t.Fatalf("interval %d must 400: %d %s", seconds, bad.status, bad.raw)
		}
	}
	missing := ta.do(t, "PATCH", "/api/v1/nodes/"+nodeID,
		map[string]string{"name": "rename"}, a.cookie, a.csrf)
	if missing.status != http.StatusBadRequest {
		t.Fatalf("missing interval must 400: %d %s", missing.status, missing.raw)
	}
	unknown := ta.do(t, "PATCH", "/api/v1/nodes/00000000-0000-0000-0000-000000000000",
		map[string]int{"poll_interval_seconds": 60}, a.cookie, a.csrf)
	if unknown.status != http.StatusNotFound {
		t.Fatalf("unknown node must 404: %d %s", unknown.status, unknown.raw)
	}

	// Still 15 after the rejected updates.
	beat := ta.doAgent(t, "POST", "/agent/v1/nodes/"+nodeID+"/heartbeat", node.serial, nil)
	if beat.status != http.StatusOK || beat.body["poll_interval_seconds"] != float64(15) {
		t.Fatalf("interval unchanged after rejects: %d %s", beat.status, beat.raw)
	}
}

func TestDeleteNodeGroup(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "token", "v")
	ta.publish(t, a)

	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "web-tier"}, a.cookie, a.csrf)
	if g.status != http.StatusCreated {
		t.Fatalf("group: %d %s", g.status, g.raw)
	}
	groupID := g.body["id"].(string)
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	if asg.status != http.StatusCreated {
		t.Fatalf("assignment: %d %s", asg.status, asg.raw)
	}
	blocked := ta.do(t, "DELETE", "/api/v1/node-groups/"+groupID, nil, a.cookie, a.csrf)
	if blocked.status != http.StatusConflict {
		t.Fatalf("delete assigned node group must conflict: %d %s", blocked.status, blocked.raw)
	}
	unassign := ta.do(t, "POST", "/api/v1/assignments/"+asg.body["id"].(string)+"/unassign",
		reason("maintenance", "remove the test assignment before deleting the node group"), a.cookie, a.csrf)
	if unassign.status != http.StatusAccepted {
		t.Fatalf("unassign: %d %s", unassign.status, unassign.raw)
	}
	deleted := ta.do(t, "DELETE", "/api/v1/node-groups/"+groupID, nil, a.cookie, a.csrf)
	if deleted.status != http.StatusNoContent {
		t.Fatalf("delete unassigned node group: %d %s", deleted.status, deleted.raw)
	}
	list := ta.do(t, "GET", "/api/v1/node-groups", nil, a.cookie, "")
	rows := parsePage(t, list.raw)
	if list.status != http.StatusOK || len(rows) != 0 {
		t.Fatalf("node groups after delete: %d %s", list.status, list.raw)
	}
}
