package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR607AppliedCloneSurvivesLegacyBindingRepair(t *testing.T) {
	if os.Getenv("KF_RUN_PR607_CLONE") != "1" {
		t.Skip("set KF_RUN_PR607_CLONE=1 for an applied disposable production clone")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := strings.TrimSpace(os.Getenv("DB_SCHEMA"))
	if schema == "" {
		schema = "p2rms15pepb5ciz"
	}
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		t.Fatal(err)
	}
	publishedIDs, draftIDs := []int64{}, []int64{}
	for _, entry := range manifest.Entries {
		if entry.Publish {
			publishedIDs = append(publishedIDs, entry.TargetProductID)
		} else {
			draftIDs = append(draftIDs, entry.TargetProductID)
		}
	}
	counts := func() (int, int, int) {
		t.Helper()
		var publishedBindings, draftBindings, repairBOMs int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=ANY($1)`, schema), publishedIDs).Scan(&publishedBindings); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=ANY($1)`, schema), draftIDs).Scan(&draftBindings); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE output_product_id=ANY($1) AND created_by='system-pr403-legacy-binding-repair'`, schema), pr607ProductIDs(manifest)).Scan(&repairBOMs); err != nil {
			t.Fatal(err)
		}
		return publishedBindings, draftBindings, repairBOMs
	}
	beforePublished, beforeDraft, beforeRepair := counts()
	if beforePublished != 26 || beforeDraft != 0 {
		t.Fatalf("pre-repair bindings=%d/%d want 26/0", beforePublished, beforeDraft)
	}
	if err := backfillProductionBomLibrary(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := repairLegacyProductionBomBindings(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	afterPublished, afterDraft, afterRepair := counts()
	if afterPublished != 26 || afterDraft != 0 || afterRepair != beforeRepair {
		t.Fatalf("post-repair bindings/repair=%d/%d/%d want 26/0/%d", afterPublished, afterDraft, afterRepair, beforeRepair)
	}
}
