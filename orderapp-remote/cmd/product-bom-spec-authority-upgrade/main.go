package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	postgresmigration "orderapp/internal/infrastructure/postgres/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

// product-bom-spec-authority-upgrade is intentionally a command, not an HTTP
// endpoint: the manifest, confirmation code and serializable transaction must
// be reviewed by an operator before apply.
func main() {
	mode := flag.String("mode", "preview", "preview, prepare, apply, or rollback")
	actor := flag.String("actor", "", "operation-log actor")
	confirm := flag.String("confirm", "", "manifest id printed by preview/prepare")
	backupDir := flag.String("backup-dir", strings.TrimSpace(os.Getenv("PR608_BACKUP_DIR")), "directory for the full PostgreSQL backup taken before apply")
	flag.Parse()
	normalized := productspecmigrationapp.AuthorityUpgradeMode(strings.ToLower(strings.TrimSpace(*mode)))
	if normalized != productspecmigrationapp.AuthorityUpgradePreview && normalized != productspecmigrationapp.AuthorityUpgradePrepare && normalized != productspecmigrationapp.AuthorityUpgradeApply && normalized != productspecmigrationapp.AuthorityUpgradeRollback {
		fatal(fmt.Errorf("--mode must be preview, prepare, apply, or rollback"))
	}
	if strings.TrimSpace(*actor) == "" {
		*actor = "operator-pr608"
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
	repo := postgresmigration.NewAuthorityUpgradeRepository(pool, schema)
	svc := productspecmigrationapp.NewAuthorityUpgradeService(repo)
	var report productspecmigrationapp.AuthorityUpgradeReport
	switch normalized {
	case productspecmigrationapp.AuthorityUpgradePreview:
		report, err = svc.Preview(ctx)
	case productspecmigrationapp.AuthorityUpgradePrepare:
		report, err = svc.Prepare(ctx, productspecmigrationapp.AuthorityUpgradeCommand{Actor: *actor, ManifestID: *confirm})
	case productspecmigrationapp.AuthorityUpgradeApply:
		if strings.TrimSpace(*confirm) == "" {
			fatal(fmt.Errorf("--confirm MANIFEST_ID is required for apply"))
		}
		backupPath, backupErr := createFullBackup(ctx, databaseURL, *backupDir)
		if backupErr != nil {
			fatal(backupErr)
		}
		fmt.Fprintf(os.Stderr, "PR-608 database backup: %s\n", backupPath)
		report, err = svc.Apply(ctx, productspecmigrationapp.AuthorityUpgradeCommand{Actor: *actor, ManifestID: *confirm})
	case productspecmigrationapp.AuthorityUpgradeRollback:
		if strings.TrimSpace(*confirm) == "" {
			fatal(fmt.Errorf("--confirm MANIFEST_ID is required for rollback"))
		}
		report, err = svc.Rollback(ctx, productspecmigrationapp.AuthorityUpgradeCommand{Actor: *actor, ManifestID: *confirm})
	}
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
}

func createFullBackup(ctx context.Context, databaseURL, backupDir string) (string, error) {
	if strings.TrimSpace(backupDir) == "" {
		backupDir = filepath.Join("var", "backups", "pr608")
	}
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return "", fmt.Errorf("create PR-608 backup directory: %w", err)
	}
	path := filepath.Join(backupDir, "pr608-before-apply-"+time.Now().UTC().Format("20060102T150405Z")+".dump")
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--file", path, databaseURL)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create PR-608 PostgreSQL backup: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return path, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
