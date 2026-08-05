package costing

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	domain "orderapp/internal/domain/costing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPricingRuleTrialProductionOptionsPostgresEmptyPublishedWithDraft(t *testing.T) {
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set KF_RUN_POSTGRES_INTEGRATION=1 to run against a disposable PostgreSQL schema")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	schema := fmt.Sprintf("codex_pr577_cost_options_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, dropErr := pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); dropErr != nil {
			t.Errorf("drop test schema: %v", dropErr)
		}
	}()

	ddl := []string{
		`CREATE TABLE ` + schema + `.products (id BIGINT PRIMARY KEY, active BOOLEAN NOT NULL DEFAULT true, parent_product_id BIGINT NOT NULL DEFAULT 0)`,
		`CREATE TABLE ` + schema + `.product_production_configs (product_id BIGINT PRIMARY KEY, production_bom_version_id BIGINT NOT NULL DEFAULT 0)`,
		`CREATE TABLE ` + schema + `.product_production_bom_bindings (product_id BIGINT PRIMARY KEY, bom_version_id BIGINT NOT NULL DEFAULT 0)`,
		`CREATE TABLE ` + schema + `.production_boms (id BIGSERIAL PRIMARY KEY, code TEXT, name TEXT, output_product_id BIGINT NOT NULL, status TEXT NOT NULL DEFAULT 'active')`,
		`CREATE TABLE ` + schema + `.production_bom_versions (
			id BIGSERIAL PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			version_no TEXT,
			status TEXT NOT NULL,
			process_route_id BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE ` + schema + `.production_bom_version_items (id BIGSERIAL PRIMARY KEY, version_id BIGINT NOT NULL)`,
		`CREATE TABLE ` + schema + `.process_routes (id BIGINT PRIMARY KEY, name TEXT, status TEXT NOT NULL DEFAULT 'active')`,
		`CREATE TABLE ` + schema + `.operation_templates (id BIGINT PRIMARY KEY, name TEXT, active BOOLEAN NOT NULL DEFAULT true)`,
	}
	for _, statement := range ddl {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO `+schema+`.products(id) VALUES (911);
		INSERT INTO `+schema+`.production_boms(id,code,name,output_product_id) VALUES (144,'BOM-000911','萨其姆-生豆',911);
		INSERT INTO `+schema+`.production_bom_versions(id,bom_id,version_no,status,created_at) VALUES
		  (228,144,'V001','published',now()-interval '1 hour'),
		  (231,144,'V002','draft',now());
		INSERT INTO `+schema+`.production_bom_version_items(version_id) VALUES (231);
		INSERT INTO `+schema+`.product_production_bom_bindings(product_id,bom_version_id) VALUES (911,228);
	`); err != nil {
		t.Fatal(err)
	}

	options, err := NewRepository(pool, schema).LoadPricingRuleTrialProductionOptions(ctx, domain.ProductInput{ProductID: 911})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.BomVersions) != 1 {
		t.Fatalf("BOM versions = %+v, want only the published version", options.BomVersions)
	}
	got := options.BomVersions[0]
	if got.VersionID != 228 || got.VersionNo != "V001" || got.ComponentCount != 0 || !got.IsDefault {
		t.Fatalf("published option = %+v", got)
	}
	if got.LatestNonEmptyDraftVersionID != 231 || got.LatestNonEmptyDraftVersionNo != "V002" {
		t.Fatalf("draft diagnostic metadata = %+v", got)
	}
}
