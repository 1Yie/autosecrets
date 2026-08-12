package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// --- Authoring ------------------------------------------------------------

func TestAuthoringFlow(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)

	secretID := ta.createSecret(t, a, "db_token", "s3cret-value-1")
	if secretID == "" {
		t.Fatal("no secret id")
	}

	// Default binding: filename equals the Secret name, 0400.
	list := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/secrets",
		nil, a.cookie, "")
	var rows []map[string]any
	if err := json.Unmarshal(list.raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 secret, got %d", len(rows))
	}
	row := rows[0]
	binding := row["binding"].(map[string]any)
	if binding["path"] != "db_token" || binding["mode"] != "0400" {
		t.Fatalf("default binding wrong: %v", binding)
	}

	// Binding validation: absolute path, .., dot, and unsafe mode rejected.
	for _, tc := range []map[string]any{
		{"path": "/etc/passwd", "mode": "0400"},
		{"path": "../escape", "mode": "0400"},
		{"path": "a/./b", "mode": "0400"},
		{"path": "ok/path", "mode": "0777"},
	} {
		res := ta.do(t, "PUT", "/api/v1/secrets/"+secretID+"/binding", tc, a.cookie, a.csrf)
		if res.status != http.StatusBadRequest {
			t.Fatalf("binding %v accepted: %d %s", tc, res.status, res.raw)
		}
	}
	good := ta.do(t, "PUT", "/api/v1/secrets/"+secretID+"/binding",
		map[string]any{"path": "app/token", "uid": 1000, "gid": 1000, "mode": "0600"},
		a.cookie, a.csrf)
	if good.status != http.StatusOK {
		t.Fatalf("valid binding rejected: %d %s", good.status, good.raw)
	}

	// Rotation: new version bumps the Draft and selects it.
	rot := ta.do(t, "POST", "/api/v1/secrets/"+secretID+"/versions",
		map[string]string{"value": "s3cret-value-2"}, a.cookie, a.csrf)
	if rot.status != 201 || rot.body["seq"].(float64) != 2 {
		t.Fatalf("rotate: %d %s", rot.status, rot.raw)
	}
	draft := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		nil, a.cookie, "")
	if draft.status != 200 {
		t.Fatalf("draft: %d %s", draft.status, draft.raw)
	}
	version := draft.body["version"].(float64)
	selections := draft.body["selections"].([]any)
	if len(selections) != 1 {
		t.Fatalf("draft selections: %d", len(selections))
	}
	sel := selections[0].(map[string]any)
	if sel["version_seq"].(float64) != 2 || sel["name"] != "db_token" {
		t.Fatalf("selection wrong: %v", sel)
	}

	// Optimistic concurrency: a stale If-Match must conflict.
	authH := map[string]string{"If-Match": fmtInt(version), "Cookie": sessionCookie + "=" + a.cookie, csrfHeader: a.csrf}
	current := ta.doH(t, "PUT", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		map[string]any{"selections": map[string]any{secretID: 1}}, authH)
	if current.status != http.StatusOK {
		t.Fatalf("draft update: %d %s", current.status, current.raw)
	}
	stale := ta.doH(t, "PUT", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		map[string]any{"selections": map[string]any{secretID: 1}}, authH)
	if stale.status != http.StatusConflict {
		t.Fatalf("stale draft update must conflict: %d", stale.status)
	}

	pub := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/publish",
		reason("maintenance", "publish the authored secret"), a.cookie, a.csrf)
	if pub.status != 201 {
		t.Fatalf("publish: %d %s", pub.status, pub.raw)
	}
	revs := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/revisions",
		nil, a.cookie, "")
	var revList []any
	if err := json.Unmarshal(revs.raw, &revList); err != nil {
		t.Fatal(err)
	}
	if len(revList) != 1 {
		t.Fatalf("revisions: %s", revs.raw)
	}
}

// TestEmptyEnvironmentDraftIsAnEmptyState locks the empty-environment
// experience: an Environment without Secrets returns an empty Draft (not a
// 404), so the authoring page never shows a misleading error.
func TestEmptyEnvironmentDraftIsAnEmptyState(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)
	draft := ta.do(t, "GET", "/api/v1/applications/"+a.appID+"/environments/"+a.envID+"/draft",
		nil, a.cookie, "")
	if draft.status != http.StatusOK {
		t.Fatalf("empty environment draft must be 200: %d %s", draft.status, draft.raw)
	}
	if draft.body["version"] != float64(0) {
		t.Fatalf("empty draft version: %s", draft.raw)
	}
	if selections, ok := draft.body["selections"].([]any); !ok || len(selections) != 0 {
		t.Fatalf("empty draft selections: %s", draft.raw)
	}
}

func fmtInt(n float64) string { return strconv.FormatInt(int64(n), 10) }
