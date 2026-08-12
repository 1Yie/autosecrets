package app

import (
	"net/http"
	"testing"
)

// TestEnvironmentProtectionClassification locks the ADR-0016 policy: a new
// Environment must be explicitly classified standard or protected; legacy
// rows migrate to unclassified and are not creatable through the API.
func TestEnvironmentProtectionClassification(t *testing.T) {
	ta := newTestApp(t)
	a := ta.authoringSetup(t)

	missing := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "staging"}, a.cookie, a.csrf)
	if missing.status != http.StatusBadRequest {
		t.Fatalf("environment without protection must be rejected: %d %s", missing.status, missing.raw)
	}
	invalid := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "staging", "protection": "prod"}, a.cookie, a.csrf)
	if invalid.status != http.StatusBadRequest {
		t.Fatalf("unknown protection value must be rejected: %d %s", invalid.status, invalid.raw)
	}
	standard := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "staging", "protection": "standard"}, a.cookie, a.csrf)
	if standard.status != http.StatusCreated || standard.body["protection"] != "standard" {
		t.Fatalf("standard environment: %d %s", standard.status, standard.raw)
	}
	protected := ta.do(t, "POST", "/api/v1/applications/"+a.appID+"/environments",
		map[string]string{"name": "prod", "protection": "protected"}, a.cookie, a.csrf)
	if protected.status != http.StatusCreated || protected.body["protection"] != "protected" {
		t.Fatalf("protected environment: %d %s", protected.status, protected.raw)
	}
	detail := ta.do(t, "GET", "/api/v1/applications/"+a.appID, nil, a.cookie, "")
	envs, ok := detail.body["environments"].([]any)
	if !ok || len(envs) != 3 {
		t.Fatalf("environment list: %s", detail.raw)
	}
	byName := map[string]string{}
	for _, raw := range envs {
		env := raw.(map[string]any)
		byName[env["name"].(string)] = env["protection"].(string)
	}
	if byName["production"] != "standard" || byName["staging"] != "standard" || byName["prod"] != "protected" {
		t.Fatalf("protection not listed per environment: %s", detail.raw)
	}
}
