package app

// Rotatable Secret semantics (old-project inspired, Core-driven):
//   - a Secret with multiple versions is a candidate list
//   - a Publish that reselects a candidate keeps the value a node already
//     activated (never disturbs a value in use)
//   - an explicit rotate marks the next candidate as the pending target;
//     nodes are forced onto it, then keep-old-value resumes

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

func secretPayloadValue(t *testing.T, plaintext []byte) string {
	t.Helper()
	var payload struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	content, _ := base64.StdEncoding.DecodeString(payload.Files[0].Content)
	return string(content)
}

func (ta *testApp) addVersion(t *testing.T, a authoring, secretID, value string) int64 {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/versions",
		map[string]string{"value": value}, a.cookie, a.csrf)
	if res.status != 201 {
		t.Fatalf("add version: %d %s", res.status, res.raw)
	}
	return int64(res.body["seq"].(float64))
}

func (ta *testApp) publish(t *testing.T, a authoring) string {
	t.Helper()
	res := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "rotate the database password"), a.cookie, a.csrf)
	if res.status != 201 {
		t.Fatalf("publish: %d %s", res.status, res.raw)
	}
	return res.body["id"].(string)
}

// publishAndAssign publishes and creates a Bundle Assignment for the group;
// the Assignment follows the Bundle's Desired Revision automatically.
func (ta *testApp) publishAndAssign(t *testing.T, a authoring, groupID string) string {
	t.Helper()
	revID := ta.publish(t, a)
	as := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	if as.status != 201 {
		t.Fatalf("assign: %d %s", as.status, as.raw)
	}
	return revID
}

func (ta *testApp) enrolledNode(t *testing.T, a authoring, name string) (nodeIdentity, string) {
	t.Helper()
	command := ta.installCommand(t, a.cookie, a.csrf, name)
	node := ta.enrollNode(t, tokenFromCommand(command), name)
	return node, node.nodeID
}

// TestRotateKeepsOldValueUntilForced covers the full lifecycle:
// v1 -> add v2 -> publish (draft selects v2) -> node activates v2
// -> rotate back to v1 -> publish -> node KEEPS v2 (candidate, no force)
// -> rotate to v2 -> publish -> node FORCED to v2 (it already has v2)
// -> rotate to v1 -> publish -> node FORCED to v1 (pending target).
func TestRotateKeepsOldValueUntilForced(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	ta.putPolicy(t, a)
	secretID := ta.createSecret(t, a, "db_pass", "value-v1")
	ta.addVersion(t, a, secretID, "value-v2")

	// Publish R1 with the draft selecting v2.
	rev1 := ta.publish(t, a)
	// Node joins and activates R1 (v2).
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	groupID := g.body["id"].(string)
	ta.do(t, "POST", "/api/v1/assignments", assignBody(a, groupID), a.cookie, a.csrf)
	node, nodeID := ta.enrolledNode(t, a, "node-1")
	ta.do(t, "POST", "/api/v1/node-groups/"+groupID+"/nodes",
		map[string]string{"node_id": nodeID}, a.cookie, a.csrf)
	_, desired := ta.pollDesired(t, node, "")
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+nodeID+"/reports", node.serial, map[string]string{
		"revision_id": rev1, "stage": "activate", "result": "ok",
	})
	if got := secretPayloadValue(t, decryptEnvelope(t, desired.Envelopes[0], node.ageID)); got != "value-v2" {
		t.Fatalf("first activation must be v2, got %q", got)
	}

	// Plain Draft reselect to v1, publish R2: no rotation marked, so the
	// node must KEEP v2 (still a candidate) — the keep-old-value contract.
	draft := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		nil, a.cookie, "")
	draftV := int64(draft.body["version"].(float64))
	update := ta.doH(t, "PUT", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		map[string]any{"selections": map[string]any{secretID: 1}},
		map[string]string{"If-Match": fmtInt(float64(draftV)), "Cookie": sessionCookie + "=" + a.cookie, csrfHeader: a.csrf})
	if update.status != 200 {
		t.Fatalf("draft reselect: %d %s", update.status, update.raw)
	}
	ta.publish(t, a)
	_, desired2 := ta.pollDesired(t, node, "")
	if got := secretPayloadValue(t, decryptEnvelope(t, desired2.Envelopes[0], node.ageID)); got != "value-v2" {
		t.Fatalf("keep-old-value violated: got %q, want value-v2", got)
	}

	// Rotate to v2, publish R3: node already has v2 (observed v2 == target v2).
	rot2 := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/rotate", nil, a.cookie, a.csrf)
	if rot2.body["version_seq"].(float64) != 2 {
		t.Fatalf("rotate 2: %s", rot2.raw)
	}
	rev3 := ta.publish(t, a)
	_, desired3 := ta.pollDesired(t, node, "")
	if got := secretPayloadValue(t, decryptEnvelope(t, desired3.Envelopes[0], node.ageID)); got != "value-v2" {
		t.Fatalf("expected v2 (already converged), got %q", got)
	}
	ta.doAgent(t, "POST", "/agent/v1/nodes/"+nodeID+"/reports", node.serial, map[string]string{
		"revision_id": rev3, "stage": "activate", "result": "ok",
	})

	// Rotate to v1, publish R4: FORCED switch to v1.
	rot3 := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/rotate", nil, a.cookie, a.csrf)
	if rot3.body["version_seq"].(float64) != 1 {
		t.Fatalf("rotate 3: %s", rot3.raw)
	}
	ta.publish(t, a)
	_, desired4 := ta.pollDesired(t, node, "")
	if got := secretPayloadValue(t, decryptEnvelope(t, desired4.Envelopes[0], node.ageID)); got != "value-v1" {
		t.Fatalf("forced rotation must deliver v1, got %q", got)
	}
}

func TestRotateValidation(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	secretID := ta.createSecret(t, a, "single", "only-value")

	// A single-version Secret cannot rotate.
	rot := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/rotate", nil, a.cookie, a.csrf)
	if rot.status != http.StatusBadRequest {
		t.Fatalf("single-version rotate must be 400: %d %s", rot.status, rot.raw)
	}
	missing := ta.do(t, "POST", "/api/v1/secrets/00000000-0000-0000-0000-000000000000/rotate",
		nil, a.cookie, a.csrf)
	if missing.status != http.StatusNotFound {
		t.Fatalf("missing secret rotate must be 404: %d", missing.status)
	}
}
