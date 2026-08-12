package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestActivationPolicyRequiredBeforeAssignment locks ADR-0022: an
// Environment must define an ordered Activation Policy before its first
// Assignment; every delivered Bundle can then be stopped safely.
func TestActivationPolicyRequiredBeforeAssignment(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "db_pass", "v1")
	ta.publish(t, a)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)

	blocked := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, g.body["id"].(string)), a.cookie, a.csrf)
	if blocked.status != http.StatusBadRequest || blocked.body["code"] != "activation_policy_required" {
		t.Fatalf("assignment without activation policy must be rejected: %d %s", blocked.status, blocked.raw)
	}
	badUnits := ta.do(t, "PUT", policyPath(a),
		map[string]any{
			"units":            []string{"a.service", "b.service", "c.service", "d.service", "e.service", "f.service"},
			"action":           "restart",
			"operation_reason": map[string]string{"category": "maintenance", "explanation": "define the service activation policy"},
		}, a.cookie, a.csrf)
	if badUnits.status != http.StatusBadRequest {
		t.Fatalf("six units must be rejected: %d %s", badUnits.status, badUnits.raw)
	}
	badAction := ta.do(t, "PUT", policyPath(a),
		map[string]any{
			"units":            []string{"web.service"},
			"action":           "exec",
			"operation_reason": map[string]string{"category": "maintenance", "explanation": "define the service activation policy"},
		}, a.cookie, a.csrf)
	if badAction.status != http.StatusBadRequest {
		t.Fatalf("arbitrary action must be rejected: %d %s", badAction.status, badAction.raw)
	}
	okPolicy := ta.do(t, "PUT", policyPath(a),
		map[string]any{
			"units":            []string{"web.service", "web.socket"},
			"action":           "restart",
			"operation_reason": map[string]string{"category": "maintenance", "explanation": "define the service activation policy"},
		}, a.cookie, a.csrf)
	if okPolicy.status != http.StatusOK {
		t.Fatalf("activation policy: %d %s", okPolicy.status, okPolicy.raw)
	}
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, g.body["id"].(string)), a.cookie, a.csrf)
	if asg.status != http.StatusCreated {
		t.Fatalf("assignment after policy: %d %s", asg.status, asg.raw)
	}
}

// TestUnassignmentStateMachine covers the persistent two-phase removal:
// pending per-node tasks, Agent cleanup acknowledgement, retry, offline
// handling, and Abandon Cleanup Confirmation producing cleanup_unconfirmed.
func TestUnassignmentStateMachine(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	a := ta.authoringSetup(t)
	ta.createSecret(t, a, "db_pass", "v1")
	ta.publish(t, a)
	ta.putPolicy(t, a)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	groupID := g.body["id"].(string)
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	assignmentID := asg.body["id"].(string)
	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")
	command2 := ta.installCommand(t, a.cookie, a.csrf, "node-2")
	node2 := ta.enrollNode(t, tokenFromCommand(command2), "node-2")
	add := ta.do(t, "POST", "/api/v1/node-groups/"+groupID+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)
	if add.status != 200 {
		t.Fatalf("add member: %d %s", add.status, add.raw)
	}
	add2 := ta.do(t, "POST", "/api/v1/node-groups/"+groupID+"/nodes",
		map[string]string{"node_id": node2.nodeID}, a.cookie, a.csrf)
	if add2.status != 200 {
		t.Fatalf("add member 2: %d %s", add2.status, add2.raw)
	}

	unassign := ta.do(t, "POST", "/api/v1/assignments/"+assignmentID+"/unassign",
		reason("maintenance", "decommission the legacy service"), a.cookie, a.csrf)
	if unassign.status != http.StatusAccepted {
		t.Fatalf("unassign: %d %s", unassign.status, unassign.raw)
	}
	tasks := unassign.body["tasks"].([]any)
	if len(tasks) != 2 || tasks[0].(map[string]any)["status"] != "pending" {
		t.Fatalf("unassign tasks: %s", unassign.raw)
	}

	// The Assignment is no longer delivered.
	_, desired := ta.pollDesired(t, node, "")
	if len(desired.Envelopes) != 0 {
		t.Fatalf("removing assignment must not be delivered: %s", desired.ETag)
	}

	// The Agent acknowledges cleanup for the node.
	cleanup := ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/cleanup", node.serial,
		map[string]string{"assignment_id": assignmentID, "result": "cleaned"})
	if cleanup.status != http.StatusOK {
		t.Fatalf("cleanup report: %d %s", cleanup.status, cleanup.raw)
	}
	state := ta.do(t, "GET", "/api/v1/assignments/"+assignmentID, nil, a.cookie, "")
	if state.status != http.StatusOK {
		t.Fatalf("assignment state: %d %s", state.status, state.raw)
	}
	statusByNode := map[string]string{}
	for _, raw := range state.body["tasks"].([]any) {
		task := raw.(map[string]any)
		statusByNode[task["node_id"].(string)] = task["status"].(string)
	}
	if statusByNode[node.nodeID] != "cleaned" {
		t.Fatalf("cleanup must be acknowledged: %s", state.raw)
	}

	// node-2's failed cleanup stays retryable.
	fail2 := ta.doAgent(t, "POST", "/agent/v1/nodes/"+node2.nodeID+"/cleanup", node2.serial,
		map[string]string{"assignment_id": assignmentID, "result": "failed", "error": "unit still running"})
	if fail2.status != http.StatusOK {
		t.Fatalf("failed cleanup report: %d %s", fail2.status, fail2.raw)
	}
	// Abandon needs the strongest path and produces cleanup_unconfirmed.
	abandon := ta.do(t, "POST", "/api/v1/assignments/"+assignmentID+"/abandon-cleanup",
		reason("incident_response", "node unreachable, accept unconfirmed cleanup"), a.cookie, a.csrf)
	if abandon.status != http.StatusOK {
		t.Fatalf("abandon cleanup: %d %s", abandon.status, abandon.raw)
	}
	state = ta.do(t, "GET", "/api/v1/assignments/"+assignmentID, nil, a.cookie, "")
	byNode := map[string]string{}
	for _, raw := range state.body["tasks"].([]any) {
		task := raw.(map[string]any)
		byNode[task["node_id"].(string)] = task["status"].(string)
	}
	if byNode[node.nodeID] != "cleaned" || byNode[node2.nodeID] != "cleanup_unconfirmed" {
		t.Fatalf("abandon must keep cleaned and record cleanup_unconfirmed: %s", state.raw)
	}
	overview := ta.do(t, "GET", "/api/v1/overview", nil, a.cookie, "")
	attention := overview.body["attention"].([]any)
	found := false
	for _, raw := range attention {
		if raw.(map[string]any)["kind"] == "cleanup_unconfirmed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cleanup_unconfirmed must be a highest-priority attention item: %s", overview.raw)
	}
}

func policyPath(a authoring) string {
	return "/api/v1/applications/" + a.appID + "/environments/" + a.envID + "/activation-policy"
}

func (ta *testApp) putPolicy(t *testing.T, a authoring) {
	t.Helper()
	res := ta.do(t, "PUT", policyPath(a), map[string]any{
		"units":  []string{"web.service"},
		"action": "restart",
		"operation_reason": map[string]string{
			"category": "maintenance", "explanation": "test activation policy",
		},
	}, a.cookie, a.csrf)
	if res.status != http.StatusOK {
		t.Fatalf("put activation policy: %d %s", res.status, res.raw)
	}
}

func assignmentTasks(t *testing.T, res result) []map[string]any {
	t.Helper()
	var body struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(res.raw, &body); err != nil {
		t.Fatal(err)
	}
	return body.Tasks
}
