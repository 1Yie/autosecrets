package app

// Smoke coverage for handlers that only the full flows reach: health,
// application detail, node heartbeats, the CA endpoint, and the remaining
// fleet list endpoints. Each test asserts the contract-validated status code
// and a minimal body invariant; deep behavior lives in the flow tests.

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHealthAndMeFlows(t *testing.T) {
	ta := newTestApp(t)

	health := ta.do(t, "GET", "/api/v1/health", nil, "", "")
	if health.status != 200 || health.body["service"] != "core" {
		t.Fatalf("health: %d %s", health.status, health.raw)
	}
	meAnon := ta.do(t, "GET", "/api/v1/me", nil, "", "")
	if meAnon.status != 200 || meAnon.body["bootstrap_required"] != true {
		t.Fatalf("anonymous me: %d %s", meAnon.status, meAnon.raw)
	}

	cookie, _ := ta.bootstrap(t)
	meAuth := ta.do(t, "GET", "/api/v1/me", nil, cookie, "")
	if meAuth.status != 200 || meAuth.body["bootstrap_required"] != false {
		t.Fatalf("authenticated me: %d %s", meAuth.status, meAuth.raw)
	}
	member, ok := meAuth.body["member"].(map[string]any)
	if !ok || member["username"] != "admin" || member["role"] != "administrator" || meAuth.body["csrf_token"] == "" {
		t.Fatalf("me member payload: %s", meAuth.raw)
	}

	// Logout requires a session: anonymous logout is 401.
	anonLogout := ta.do(t, "POST", "/api/v1/auth/logout", nil, "", "")
	if anonLogout.status != http.StatusUnauthorized {
		t.Fatalf("anonymous logout must be 401: %d", anonLogout.status)
	}
}

func TestApplicationListAndDetail(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)

	list := ta.do(t, "GET", "/api/v1/applications", nil, a.cookie, "")
	if list.status != 200 {
		t.Fatalf("list applications: %d %s", list.status, list.raw)
	}
	var rows []map[string]any
	if err := json.Unmarshal(list.raw, &rows); err != nil || len(rows) != 1 {
		t.Fatalf("applications payload: %s", list.raw)
	}
	detail := ta.do(t, "GET", "/api/v1/applications/"+a.appID, nil, a.cookie, "")
	if detail.status != 200 {
		t.Fatalf("application detail: %d %s", detail.status, detail.raw)
	}
	envs := detail.body["environments"].([]any)
	if len(envs) != 1 || envs[0].(map[string]any)["name"] != "production" {
		t.Fatalf("application environments: %s", detail.raw)
	}
	missing := ta.do(t, "GET", "/api/v1/applications/00000000-0000-0000-0000-000000000000",
		nil, a.cookie, "")
	if missing.status != http.StatusNotFound || missing.body["code"] != "not_found" {
		t.Fatalf("missing application: %d %s", missing.status, missing.raw)
	}
}

func TestFleetListAndMembershipFlows(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "tok", "v")
	ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "smoke publish for fleet flow"), a.cookie, a.csrf)

	// Empty lists first (covers the 0-coverage list handlers).
	emptyGroups := ta.do(t, "GET", "/api/v1/node-groups", nil, a.cookie, "")
	if emptyGroups.status != 200 {
		t.Fatalf("empty node groups: %d", emptyGroups.status)
	}
	emptyAssigns := ta.do(t, "GET", "/api/v1/assignments", nil, a.cookie, "")
	if emptyAssigns.status != 200 {
		t.Fatalf("empty assignments: %d", emptyAssigns.status)
	}

	group := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	groupID := group.body["id"].(string)
	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")

	add := ta.do(t, "POST", "/api/v1/node-groups/"+groupID+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)
	if add.status != 200 {
		t.Fatalf("add member: %d %s", add.status, add.raw)
	}
	assign := ta.do(t, "POST", "/api/v1/assignments",
		assignBody(a, groupID), a.cookie, a.csrf)
	if assign.status != 201 {
		t.Fatalf("assignment: %d %s", assign.status, assign.raw)
	}

	groups := ta.do(t, "GET", "/api/v1/node-groups", nil, a.cookie, "")
	var groupRows []map[string]any
	if err := json.Unmarshal(groups.raw, &groupRows); err != nil || len(groupRows) != 1 ||
		len(groupRows[0]["member_ids"].([]any)) != 1 {
		t.Fatalf("node groups payload: %s", groups.raw)
	}
	assigns := ta.do(t, "GET", "/api/v1/assignments", nil, a.cookie, "")
	var assignRows []map[string]any
	if err := json.Unmarshal(assigns.raw, &assignRows); err != nil || len(assignRows) != 1 {
		t.Fatalf("assignments payload: %s", assigns.raw)
	}

	remove := ta.do(t, "DELETE", "/api/v1/node-groups/"+groupID+"/nodes/"+node.nodeID,
		nil, a.cookie, a.csrf)
	if remove.status != 200 {
		t.Fatalf("remove member: %d %s", remove.status, remove.raw)
	}
	groupsAfter := ta.do(t, "GET", "/api/v1/node-groups", nil, a.cookie, "")
	var after []map[string]any
	_ = json.Unmarshal(groupsAfter.raw, &after)
	if len(after[0]["member_ids"].([]any)) != 0 {
		t.Fatalf("member not removed: %s", groupsAfter.raw)
	}
}

func TestAgentHeartbeatAndCA(t *testing.T) {
	ta := newTestApp(t)
	cookie, csrf := ta.bootstrap(t)
	command := ta.installCommand(t, cookie, csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")

	ca := ta.do(t, "GET", "/agent/v1/ca.pem", nil, "", "")
	if ca.status != 200 || len(ca.raw) == 0 {
		t.Fatalf("ca.pem: %d", ca.status)
	}
	beat := ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/heartbeat", node.serial, nil)
	if beat.status != 200 {
		t.Fatalf("heartbeat: %d %s", beat.status, beat.raw)
	}
	mismatch := ta.doAgent(t, "POST", "/agent/v1/nodes/other-node/heartbeat", node.serial, nil)
	if mismatch.status != http.StatusForbidden {
		t.Fatalf("heartbeat with mismatched identity must be 403: %d", mismatch.status)
	}
}
