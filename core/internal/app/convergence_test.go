package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
)

// TestPerAssignmentConvergenceAndNodeState locks ADR-0015: activation
// reports land per Managed Node + Assignment, and the node list projects a
// ranked primary state with an explicit offline threshold.
func TestPerAssignmentConvergenceAndNodeState(t *testing.T) {
	current := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	ta := newTestApp(t, func(options *Options) { options.Now = func() time.Time { return current } })
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	ta.createSecret(t, a, "db_pass", "v1")
	rev1 := ta.publish(t, a)

	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	groupID := g.body["id"].(string)
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	assignmentID := asg.body["id"].(string)
	command := ta.installCommand(t, a.cookie, a.csrf, "node-1")
	node := ta.enrollNode(t, tokenFromCommand(command), "node-1")
	add := ta.do(t, "POST", "/api/v1/node-groups/"+groupID+"/nodes",
		map[string]string{"node_id": node.nodeID}, a.cookie, a.csrf)
	if add.status != 200 {
		t.Fatalf("add member: %d %s", add.status, add.raw)
	}
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/heartbeat", node.serial, nil)

	// Assigned but not yet converged: converging.
	nodes := ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	if state := nodeState(t, nodes); state != "converging" {
		t.Fatalf("assigned node without reports must be converging, got %s", state)
	}

	// Successful activation: healthy, converged.
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/reports", node.serial, map[string]string{
		"revision_id": rev1, "stage": "activate", "result": "ok",
	})
	nodes = ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	if state := nodeState(t, nodes); state != "healthy" {
		t.Fatalf("converged node must be healthy, got %s", state)
	}

	// Failed activation: failed state outranks connection freshness.
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/reports", node.serial, map[string]string{
		"revision_id": rev1, "stage": "activate", "result": "failed", "error": "systemd unit crashed",
	})
	nodes = ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	if state := nodeState(t, nodes); state != "failed" {
		t.Fatalf("failed activation must project failed, got %s", state)
	}

	// The failed report is retained on the Assignment's convergence record.
	rows, err := ta.store.NodeConvergence(context.Background(), node.nodeID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("node convergence rows: %v (%d)", err, len(rows))
	}
	if rows[0].AssignmentID != assignmentID || rows[0].Result != "failed" || rows[0].Error != "systemd unit crashed" {
		t.Fatalf("convergence record: %+v", rows[0])
	}

	// Offline after the 75s threshold without a fresh heartbeat.
	current = current.Add(90 * time.Second)
	nodes = ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	if state := nodeState(t, nodes); state != "failed" {
		t.Fatalf("failed must outrank offline, got %s", state)
	}
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+node.nodeID+"/reports", node.serial, map[string]string{
		"revision_id": rev1, "stage": "activate", "result": "ok",
	})
	current = current.Add(90 * time.Second)
	nodes = ta.do(t, "GET", "/api/v1/nodes", nil, a.cookie, "")
	if state := nodeState(t, nodes); state != "offline" {
		t.Fatalf("healthy node past the heartbeat threshold must be offline, got %s", state)
	}
}

func nodeState(t *testing.T, res result) string {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(res.raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one node, got %d", len(rows))
	}
	return rows[0]["state"].(string)
}

// TestRecoveryCodeSingleUse locks the one-time property of Recovery Codes.
func TestRecoveryCodeSingleUse(t *testing.T) {
	ta := newTestApp(t)
	code, err := ta.app.EmitBootstrapCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	started := ta.do(t, "POST", "/api/v1/bootstrap", map[string]string{
		"code": code, "organization_name": "Acme", "username": "admin",
		"password": "correct-horse-battery-42",
	}, "", "")
	if started.status != http.StatusCreated {
		t.Fatalf("bootstrap: %d %s", started.status, started.raw)
	}
	secret := totpSecretFromURI(t, started.body["totp_uri"].(string))
	totp, err := crypto.TOTPCode(secret, ta.app.now())
	if err != nil {
		t.Fatal(err)
	}
	verified := ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/verify", map[string]string{
		"enrollment_token": started.body["enrollment_token"].(string), "totp_code": totp,
	}, "", "")
	recovery := verified.body["recovery_codes"].([]any)[0].(string)
	ta.do(t, "POST", "/api/v1/auth/mfa-enrollment/confirm", map[string]string{
		"confirmation_token": verified.body["confirmation_token"].(string),
	}, "", "")

	login := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42", "recovery_code": recovery,
	}, "", "")
	if login.status != http.StatusOK {
		t.Fatalf("recovery code login: %d %s", login.status, login.raw)
	}
	replay := ta.do(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "correct-horse-battery-42", "recovery_code": recovery,
	}, "", "")
	if replay.status != http.StatusUnauthorized {
		t.Fatalf("recovery code replay must be rejected: %d %s", replay.status, replay.raw)
	}
}
