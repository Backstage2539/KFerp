package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	postgresproduction "orderapp/internal/infrastructure/postgres/production"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	mode := flag.String("mode", "preview", "preview, apply, or verify")
	actor := flag.String("actor", "operator-pr629", "operation log actor")
	confirm := flag.String("confirm", "", "manifest id printed by preview")
	backupPath := flag.String("backup-path", "", "host path of the verified full PostgreSQL backup")
	backupSHA256 := flag.String("backup-sha256", "", "SHA-256 checksum of the backup")
	backupSize := flag.Int64("backup-size", 0, "backup byte size")
	flag.Parse()
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
	repo := postgresproduction.NewRepository(pool, schema)
	var report postgresproduction.PR629CutoverReport
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "preview":
		report, err = repo.PreviewPR629Cutover(ctx)
	case "verify":
		report, err = repo.VerifyPR629Cutover(ctx)
	case "apply":
		sha := strings.ToLower(strings.TrimSpace(*backupSHA256))
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sha) {
			fatal(fmt.Errorf("valid --backup-sha256 is required"))
		}
		report, err = repo.ApplyPR629Cutover(ctx, strings.TrimSpace(*confirm), strings.TrimSpace(*actor), postgresproduction.PR629BackupEvidence{Path: strings.TrimSpace(*backupPath), SHA256: sha, Size: *backupSize})
	default:
		err = fmt.Errorf("--mode must be preview, apply, or verify")
	}
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
