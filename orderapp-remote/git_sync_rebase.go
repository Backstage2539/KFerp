package main

import (
	"context"
	"os"
	"path/filepath"
)

func cleanupRebaseDirs(repoDir string) error {
	gitDir := filepath.Join(repoDir, ".git")
	_ = os.RemoveAll(filepath.Join(gitDir, "rebase-merge"))
	_ = os.RemoveAll(filepath.Join(gitDir, "rebase-apply"))
	return nil
}

func cleanupPossibleRebase(ctx context.Context, repoDir, keyPath string) error {
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		_ = runGit(ctx, repoDir, keyPath, "rebase", "--abort")
		_ = cleanupRebaseDirs(repoDir)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		_ = runGit(ctx, repoDir, keyPath, "rebase", "--abort")
		_ = cleanupRebaseDirs(repoDir)
	}
	return nil
}
