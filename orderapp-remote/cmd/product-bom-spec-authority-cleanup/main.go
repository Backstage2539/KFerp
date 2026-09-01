package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	postgresmigration "orderapp/internal/infrastructure/postgres/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

type commandResult struct {
	BackupPath    string                               `json:"backup_path,omitempty"`
	BackupSHA256  string                               `json:"backup_sha256,omitempty"`
	BackupSize    int64                                `json:"backup_size,omitempty"`
	CleanupReport postgresmigration.PR622CleanupReport `json:"cleanup_report"`
}

// product-bom-spec-authority-cleanup is deliberately not registered as an
// HTTP endpoint. Apply always takes and verifies a full database backup before
// the repository opens its single serializable cleanup transaction.
func main() {
	mode := flag.String("mode", "preview", "preview, apply, or verify")
	actor := flag.String("actor", "operator-pr622", "operation-log actor")
	confirm := flag.String("confirm", "", "manifest id printed by preview")
	backupPath := flag.String("backup-path", "", "host path of the verified full PostgreSQL backup")
	backupSHA256 := flag.String("backup-sha256", "", "SHA-256 checksum of the verified full PostgreSQL backup")
	backupSize := flag.Int64("backup-size", 0, "byte size of the verified full PostgreSQL backup")
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
		result.BackupPath = strings.TrimSpace(*backupPath)
		result.BackupSHA256 = strings.ToLower(strings.TrimSpace(*backupSHA256))
		result.BackupSize = *backupSize
		if result.BackupPath == "" || *backupSize <= 0 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(result.BackupSHA256) {
			fatal(fmt.Errorf("apply requires --backup-path, --backup-size > 0, and a valid --backup-sha256 from a host-retained full backup"))
		}
		result.CleanupReport, err = repo.Apply(ctx, *confirm, *actor, postgresmigration.PR622BackupEvidence{
			Path: result.BackupPath, SHA256: result.BackupSHA256, Size: result.BackupSize,
		})
	}
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
