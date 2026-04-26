package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderappRootContainsOnlyEntrypoint(t *testing.T) {
	root := moduleRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if name != "main.go" {
			t.Fatalf("root Go file %s should move under internal/appmain", name)
		}
	}
}

func TestOrderappRootContainsNoTests(t *testing.T) {
	root := moduleRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) > 0 {
		t.Fatalf("root test files should move under internal/appmain: %v", matches)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(body), "module orderapp") {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("module root not found")
		}
		dir = next
	}
}
