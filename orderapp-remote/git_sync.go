package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GitAcceptanceEntry struct {
	TimeUnix   int64
	ReviewCode string
	PRCode     string
	Title      string
	Actor      string
}

func gitSyncAcceptance(ctx context.Context, e GitAcceptanceEntry) error {
	if strings.TrimSpace(os.Getenv("GIT_SYNC_ENABLED")) != "1" {
		return nil
	}
	// Write same acceptance log into both repos (code + notes)
	codeRepo := strings.TrimSpace(os.Getenv("GIT_SYNC_CODE_REPO"))
	codeKey := strings.TrimSpace(os.Getenv("GIT_SYNC_CODE_KEY"))
	notesRepo := strings.TrimSpace(os.Getenv("GIT_SYNC_NOTES_REPO"))
	notesKey := strings.TrimSpace(os.Getenv("GIT_SYNC_NOTES_KEY"))
	baseDir := strings.TrimSpace(os.Getenv("GIT_SYNC_DIR"))
	if baseDir == "" {
		baseDir = "/app/data/git-sync"
	}
	if codeRepo == "" || notesRepo == "" || codeKey == "" || notesKey == "" {
		return fmt.Errorf("git sync not configured")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	entryText := renderAcceptanceEntry(e)

	if err := ensureRepoAndAppend(ctx, filepath.Join(baseDir, "code"), codeRepo, codeKey, entryText, e); err != nil {
		return fmt.Errorf("code repo sync: %w", err)
	}
	if err := ensureRepoAndAppend(ctx, filepath.Join(baseDir, "notes"), notesRepo, notesKey, entryText, e); err != nil {
		return fmt.Errorf("notes repo sync: %w", err)
	}
	return nil
}

func renderAcceptanceEntry(e GitAcceptanceEntry) string {
	ts := time.Unix(e.TimeUnix, 0).In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	pr := strings.TrimSpace(e.PRCode)
	if pr == "" {
		pr = "(none)"
	}
	actor := strings.TrimSpace(e.Actor)
	if actor == "" {
		actor = "(unknown)"
	}
	title := strings.TrimSpace(e.Title)
	return fmt.Sprintf("- %s | %s | PR=%s | %s | by %s\n", ts, e.ReviewCode, pr, title, actor)
}

func ensureRepoAndAppend(ctx context.Context, dir, repoURL, keyPath, entryText string, e GitAcceptanceEntry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// ensure known_hosts BEFORE any git ssh
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	kh := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(kh); err != nil {
		_ = os.WriteFile(kh, []byte(""), 0o600)
	}
	_ = runCmd(ctx, dir, "sh", "-lc", "ssh-keyscan -t rsa,ed25519 github.com 2>/dev/null >> .ssh/known_hosts || true")

	// init repo if needed (dir may be non-empty because we create .ssh)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := runGit(ctx, dir, keyPath, "init"); err != nil {
			return err
		}
		// reset origin
		_ = runGit(ctx, dir, keyPath, "remote", "remove", "origin")
		if err := runGit(ctx, dir, keyPath, "remote", "add", "origin", repoURL); err != nil {
			return err
		}
		// try to fetch main if exists
		_ = runGit(ctx, dir, keyPath, "fetch", "origin", "main")
		// create or reset local main
		_ = runGit(ctx, dir, keyPath, "checkout", "-B", "main")
		_ = runGit(ctx, dir, keyPath, "reset", "--hard", "origin/main")
	}

	// pull latest (must succeed)
	if err := runGit(ctx, dir, keyPath, "fetch", "origin", "main"); err != nil {
		return err
	}
	if err := runGit(ctx, dir, keyPath, "rebase", "origin/main"); err != nil {
		// if rebase fails, abort to leave repo clean
		_ = runGit(ctx, dir, keyPath, "rebase", "--abort")
		return err
	}

	// append log file
	logPath := filepath.Join(dir, "acceptance", "acceptance_log.md")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(logPath); err != nil {
		// init file
		header := "# Acceptance Log\n\n"
		if err := os.WriteFile(logPath, []byte(header), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(entryText); err != nil {
		return err
	}

	// commit & push
	if err := runGit(ctx, dir, keyPath, "add", "acceptance/acceptance_log.md"); err != nil {
		return err
	}
	msg := fmt.Sprintf("chore: acceptance %s done", e.ReviewCode)
	// commit may fail if no changes; treat as ok
	_ = runGit(ctx, dir, keyPath, "-c", "user.name=KFerp-bot", "-c", "user.email=kferp-bot@local", "commit", "-m", msg)
	if err := runGit(ctx, dir, keyPath, "push", "origin", "main"); err != nil {
		// Handle non-fast-forward due to concurrent updates: rebase and retry once.
		es := err.Error()
		if strings.Contains(es, "non-fast-forward") || strings.Contains(es, "fetch first") || strings.Contains(es, "rejected") {
			if err2 := runGit(ctx, dir, keyPath, "fetch", "origin", "main"); err2 != nil {
				return err
			}
			if err2 := runGit(ctx, dir, keyPath, "rebase", "origin/main"); err2 != nil {
				_ = runGit(ctx, dir, keyPath, "rebase", "--abort")
				return err
			}
			if err2 := runGit(ctx, dir, keyPath, "push", "origin", "main"); err2 == nil {
				return nil
			}
		}
		return err
	}
	return nil
}

func runGit(ctx context.Context, dir, keyPath string, args ...string) error {
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s", keyPath, filepath.Join(dir, ".ssh", "known_hosts"))
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, buf.String())
	}
	return nil
}

func runCmd(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, buf.String())
	}
	return nil
}
