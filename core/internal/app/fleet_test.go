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
