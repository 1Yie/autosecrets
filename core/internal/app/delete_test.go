package app

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAuthoringDeletes removes unpublished authoring objects and refuses to
// delete a Secret Bundle that still has an Assignment.
func TestAuthoringDeletes(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	secretID := ta.createSecret(t, a, "db_token", "s3cret-value-1")

	deleted := ta.do(t, "DELETE", "/api/v1/secrets/"+secretID, nil, a.cookie, a.csrf)
	if deleted.status != http.StatusNoContent {
		t.Fatalf("delete unpublished secret: %d %s", deleted.status, deleted.raw)
	}
	list := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/secrets",
		nil, a.cookie, "")
	var remaining []any
	if err := json.Unmarshal(list.raw, &remaining); err != nil {
		t.Fatalf("decode secret list: %v %s", err, list.raw)
	}
	if list.status != http.StatusOK || len(remaining) != 0 {
		t.Fatalf("secret list after delete: %d %s", list.status, list.raw)
	}

	secretID = ta.createSecret(t, a, "db_token", "s3cret-value-1")
	ta.publish(t, a)
	g := ta.do(t, "POST", "/api/v1/node-groups", map[string]string{"name": "g1"}, a.cookie, a.csrf)
	if g.status != http.StatusCreated {
		t.Fatalf("create group: %d %s", g.status, g.raw)
	}
	asg := ta.do(t, "POST", "/api/v1/assignments", assignBody(a, g.body["id"].(string)), a.cookie, a.csrf)
	if asg.status != http.StatusCreated {
		t.Fatalf("assignment: %d %s", asg.status, asg.raw)
	}

	blockedSecret := ta.do(t, "DELETE", "/api/v1/secrets/"+secretID, nil, a.cookie, a.csrf)
	if blockedSecret.status != http.StatusConflict {
		t.Fatalf("delete assigned secret must conflict: %d %s", blockedSecret.status, blockedSecret.raw)
	}
	blockedEnv := ta.do(t, "DELETE", "/api/v1/applications/"+a.appID+"/environments/"+a.envID, nil, a.cookie, a.csrf)
	if blockedEnv.status != http.StatusConflict {
		t.Fatalf("delete assigned environment must conflict: %d %s", blockedEnv.status, blockedEnv.raw)
	}
	blockedApp := ta.do(t, "DELETE", "/api/v1/applications/"+a.appID, nil, a.cookie, a.csrf)
	if blockedApp.status != http.StatusConflict {
		t.Fatalf("delete assigned application must conflict: %d %s", blockedApp.status, blockedApp.raw)
	}

	unassign := ta.do(t, "POST", "/api/v1/assignments/"+asg.body["id"].(string)+"/unassign",
		reason("maintenance", "remove the test assignment before delete"), a.cookie, a.csrf)
	if unassign.status != http.StatusAccepted {
		t.Fatalf("unassign: %d %s", unassign.status, unassign.raw)
	}

	deletedEnv := ta.do(t, "DELETE", "/api/v1/applications/"+a.appID+"/environments/"+a.envID, nil, a.cookie, a.csrf)
	if deletedEnv.status != http.StatusNoContent {
		t.Fatalf("delete unassigned environment: %d %s", deletedEnv.status, deletedEnv.raw)
	}
	detail := ta.do(t, "GET", "/api/v1/applications/"+a.appID, nil, a.cookie, "")
	envs, _ := detail.body["environments"].([]any)
	if detail.status != http.StatusOK || len(envs) != 0 {
		t.Fatalf("environments after delete: %d %s", detail.status, detail.raw)
	}

	deletedApp := ta.do(t, "DELETE", "/api/v1/applications/"+a.appID, nil, a.cookie, a.csrf)
	if deletedApp.status != http.StatusNoContent {
		t.Fatalf("delete unassigned application: %d %s", deletedApp.status, deletedApp.raw)
	}
	missing := ta.do(t, "GET", "/api/v1/applications/"+a.appID, nil, a.cookie, "")
	if missing.status != http.StatusNotFound {
		t.Fatalf("deleted application must 404: %d %s", missing.status, missing.raw)
	}
}
