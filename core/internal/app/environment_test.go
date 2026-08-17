package app

import (
	"net/http"
	"testing"
)

// TestCreateEnvironmentByName locks the product decision that Environment
// protection classification is gone: create takes only a name.
func TestCreateEnvironmentByName(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)

	missing := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{}, a.cookie, a.csrf)
	if missing.status != http.StatusBadRequest {
		t.Fatalf("environment without name must be rejected: %d %s", missing.status, missing.raw)
	}
	staging := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "staging"}, a.cookie, a.csrf)
	if staging.status != http.StatusCreated || staging.body["name"] != "staging" {
		t.Fatalf("create staging: %d %s", staging.status, staging.raw)
	}
	if _, ok := staging.body["protection"]; ok {
		t.Fatalf("create response must not expose protection: %s", staging.raw)
	}
	detail := ta.do(t, "GET", "/api/v1/applications/"+a.appID, nil, a.cookie, "")
	envs, ok := detail.body["environments"].([]any)
	if !ok || len(envs) != 2 {
		t.Fatalf("environment list: %s", detail.raw)
	}
	names := map[string]bool{}
	for _, raw := range envs {
		env := raw.(map[string]any)
		names[env["name"].(string)] = true
		if _, ok := env["protection"]; ok {
			t.Fatalf("listed environment must not expose protection: %s", detail.raw)
		}
	}
	if !names["production"] || !names["staging"] {
		t.Fatalf("expected production and staging: %s", detail.raw)
	}
}
