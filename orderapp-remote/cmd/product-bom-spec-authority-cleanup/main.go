package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	postgresmigration "orderapp/internal/infrastructure/postgres/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

type commandResult struct {
	BackupPath    string                               `json:"backup_path,omitempty"`
	BackupSHA256  string                               `json:"backup_sha256,omitempty"`
	CleanupReport postgresmigration.PR622CleanupReport `json:"cleanup_report"`
}

// product-bom-spec-authority-cleanup is deliberately not registered as an
// HTTP endpoint. Apply always takes and verifies a full database backup before
// the repository opens its single serializable cleanup transaction.
func main() {
	mode := flag.String("mode", "preview", "preview, apply, or verify")
	actor := flag.String("actor", "operator-pr622", "operation-log actor")
	confirm := flag.String("confirm", "", "manifest id printed by preview")
	backupDir := flag.String("backup-dir", strings.TrimSpace(os.Getenv("PR622_BACKUP_DIR")), "directory for the full PostgreSQL backup taken before apply")
	flag.Parse()

	normalized := postgresmigration.PR622CleanupMode(strings.ToLower(strings.TrimSpace(*mode)))
	if normalized != postgresmigration.PR622CleanupPreview && normalized != postgresmigration.PR622CleanupApply && normalized != postgresmigration.PR622CleanupVerify {
		fatal(fmt.Errorf("--mode must be preview, apply, or verify"))
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fatal(fmt.Errorf("DATABASE_URL is required"))
	}
	schema := strings.TrimSpace(os.Getenv("DB_SCHEMA"))
	if schema == "" {
		schema = "p2rms15pepb5ciz"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	repo := postgresmigration.NewPR622CleanupRepository(pool, schema)
	result := commandResult{}
	switch normalized {
	case postgresmigration.PR622CleanupPreview:
		result.CleanupReport, err = repo.Preview(ctx)
	case postgresmigration.PR622CleanupVerify:
		result.CleanupReport, err = repo.Verify(ctx)
	case postgresmigration.PR622CleanupApply:
		if strings.TrimSpace(*confirm) == "" {
			fatal(fmt.Errorf("--confirm MANIFEST_ID is required for apply"))
		}
		result.BackupPath, result.BackupSHA256, err = createFullBackup(ctx, databaseURL, *backupDir)
		if err == nil {
			result.CleanupReport, err = repo.Apply(ctx, *confirm, *actor)
		}
	}
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
}

func createFullBackup(ctx context.Context, databaseURL, backupDir string) (string, string, error) {
	if strings.TrimSpace(backupDir) == "" {
		backupDir = filepath.Join("var", "backups", "pr622")
	}
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return "", "", fmt.Errorf("create PR-622 backup directory: %w", err)
	}
	path := filepath.Join(backupDir, "pr622-before-apply-"+time.Now().UTC().Format("20060102T150405Z")+".dump")
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--file", path, databaseURL)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("create PR-622 PostgreSQL backup: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("open PR-622 backup: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return "", "", fmt.Errorf("PR-622 backup is empty or unreadable")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", fmt.Errorf("checksum PR-622 backup: %w", err)
	}
	return path, hex.EncodeToString(hash.Sum(nil)), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
