package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/costing"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCutoverProductSpecPreservesTemplateOrigin(t *testing.T) {
	inputs := []domain.ProductInput{{ProductID: 42, Name: "商品"}}
	specs := []cutoverProductBOMSpec{{ParentProductID: 42, BomID: 3, BomVersionID: 4, BomSpecID: 15, BomVariantID: 16, InventoryUnit: "袋", SourceSpecTemplateID: 7, SourceSpecTemplateKey: "spec-2"}}
	got := applyCutoverProductBOMSpecs(inputs, specs)
	if len(got) != 1 || got[0].SourceSpecTemplateID != 7 || got[0].SourceSpecTemplateKey != "spec-2" {
		t.Fatalf("lost template origin: %+v", got)
	}
}

func TestSpecTemplateOriginPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("test_spec_origin_%d_%d", os.Getpid(), time.Now().UnixNano())
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`
 CREATE TABLE %[1]s.products(id bigint, name text, active boolean);
 CREATE TABLE %[1]s.product_bom_spec_authorities(product_id bigint, state text, spec_identity_mode text);
 CREATE TABLE %[1]s.production_bom_output_bindings(output_type text, output_id bigint, is_default boolean, bom_id bigint, bom_version_id bigint);
 CREATE TABLE %[1]s.production_bom_versions(id bigint, bom_id bigint, version_no text, status text);
 CREATE TABLE %[1]s.production_bom_specs(id bigint, bom_id bigint, code text, barcode text, spec_key text, name text, inventory_unit text, source_spec_template_id bigint, source_spec_template_key text);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint, version_id bigint, bom_spec_id bigint, spec_name_snapshot text, inventory_unit text, process_route_id bigint, is_default boolean, sort_order integer);
 INSERT INTO %[1]s.products VALUES(42,'商品',true);
 INSERT INTO %[1]s.product_bom_spec_authorities VALUES(42,'cutover','bom_spec');
 INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',42,true,3,4);
 INSERT INTO %[1]s.production_bom_versions VALUES(4,3,'V1','published');
 INSERT INTO %[1]s.production_bom_specs VALUES(15,3,'A','','bag','227g','袋',7,'spec-2');
 INSERT INTO %[1]s.production_bom_version_variants VALUES(16,4,15,'227g','袋',0,true,1);
 `, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo := Repository{pool: pool, schema: schema}
	got, err := repo.loadCutoverProductBOMSpecs(ctx, []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SourceSpecTemplateID != 7 || got[0].SourceSpecTemplateKey != "spec-2" {
		t.Fatalf("wrong source: %+v", got)
	}
	if _, err := repo.ResolveProductBOMSpecIdentity(ctx, 42, 15, 16); err != nil {
		t.Fatalf("identity query regression: %v", err)
	}
}
