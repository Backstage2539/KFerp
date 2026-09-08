package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	mode := flag.String("mode", "preview", "preview, apply, or rollback")
	actor := flag.String("actor", "", "operation log actor")
	confirm := flag.String("confirm", "", "manifest_id returned by preview; required for apply")
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
	repo := postgresmaterials.NewRepository(pool, schema)
	var result postgresmaterials.MaterialOwnershipCutoverReport
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "preview":
		result, err = repo.PreviewMaterialOwnershipCutover(ctx)
	case "apply":
		result, err = repo.ApplyMaterialOwnershipCutover(ctx, *actor, *confirm)
	case "rollback":
		result, err = repo.RollbackMaterialOwnershipCutover(ctx, *actor)
	default:
		err = fmt.Errorf("--mode must be preview, apply, or rollback")
	}
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
