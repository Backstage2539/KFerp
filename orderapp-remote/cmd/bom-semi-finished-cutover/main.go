package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	postgresbom "orderapp/internal/infrastructure/postgres/bom"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	mode := flag.String("mode", "preview", "preview, apply, or rollback")
	actor := flag.String("actor", "", "operation log actor")
	confirm := flag.String("confirm", "", "required manifest id for apply or rollback")
	flag.Parse()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	schema := strings.TrimSpace(os.Getenv("DB_SCHEMA"))
	if schema == "" {
		schema = "p2rms15pepb5ciz"
	}
	if databaseURL == "" {
		fatal(fmt.Errorf("DATABASE_URL is required"))
	}
	manifest, err := postgresbom.LoadPR606ProductionManifest()
	if err != nil {
		fatal(err)
	}
	normalizedMode := strings.ToLower(strings.TrimSpace(*mode))
	if normalizedMode != "preview" && *confirm != manifest.ManifestID {
		fatal(fmt.Errorf("--confirm %s is required for %s", manifest.ManifestID, normalizedMode))
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	repo := postgresbom.NewRepository(pool, schema)
	var result postgresbom.PR606CutoverSummary
	switch normalizedMode {
	case "preview":
		result, err = repo.PreviewPR606Cutover(ctx)
	case "apply":
		result, err = repo.ApplyPR606Cutover(ctx, *actor)
	case "rollback":
		result, err = repo.RollbackPR606Cutover(ctx, *actor)
	default:
		err = fmt.Errorf("--mode must be preview, apply, or rollback")
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
