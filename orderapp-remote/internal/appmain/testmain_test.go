package appmain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	if root, err := findModuleRootForTests(); err == nil {
		_ = os.Chdir(root)
	}
	os.Exit(m.Run())
}

func findModuleRootForTests() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", os.ErrNotExist
		}
		dir = next
	}
}
