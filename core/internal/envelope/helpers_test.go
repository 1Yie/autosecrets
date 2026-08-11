package envelope

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRootDir walks up from the package directory to the repository root,
// identified by the root CONTEXT.md.
func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "CONTEXT.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
