package fleet

import "testing"

func TestValidArtifactName(t *testing.T) {
	ok := []string{
		"autosecrets-agent-linux-amd64.tar.gz",
		"autosecrets-agent-linux-amd64.tar.gz.sig",
		"autosecrets-agent-darwin-arm64.tar.gz",
		"autosecrets-agent-windows-amd64.tar.gz.sig",
	}
	for _, name := range ok {
		if !validArtifactName(name) {
			t.Fatalf("expected valid: %s", name)
		}
	}
	bad := []string{
		"autosecrets-agent-linux-amd64.tar.gz.exe",
		"autosecrets-agent-freebsd-amd64.tar.gz",
		"../autosecrets-agent-linux-amd64.tar.gz",
		"autosecrets-agent-linux-amd64.tar.gz/../x",
	}
	for _, name := range bad {
		if validArtifactName(name) {
			t.Fatalf("expected invalid: %s", name)
		}
	}
}
