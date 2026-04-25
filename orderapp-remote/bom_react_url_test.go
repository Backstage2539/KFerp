package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentBomReactRevUsesDistArtifacts(t *testing.T) {
	tmp := t.TempDir()
	dist := filepath.Join(tmp, "dist")
	assets := filepath.Join(dist, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dist, "index.html"),
		filepath.Join(assets, "index-abc123.js"),
		filepath.Join(assets, "index-def456.css"),
	} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	prev := bomReactDistDir
	bomReactDistDir = dist
	t.Cleanup(func() { bomReactDistDir = prev })

	rev := currentBomReactRev()
	if rev == "" || rev == "dev" {
		t.Fatalf("currentBomReactRev() = %q, want hash-like revision", rev)
	}
	if len(rev) != 8 {
		t.Fatalf("currentBomReactRev() = %q, want 8 hex chars", rev)
	}
	if got := bomReactURL(); !strings.HasPrefix(got, "/bom-react?rev=") {
		t.Fatalf("bomReactURL() = %q", got)
	}
}

func TestCurrentBomReactRevFallsBackWhenDistMissing(t *testing.T) {
	prev := bomReactDistDir
	bomReactDistDir = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { bomReactDistDir = prev })

	if got := currentBomReactRev(); got != "dev" {
		t.Fatalf("currentBomReactRev() = %q, want dev", got)
	}
}
