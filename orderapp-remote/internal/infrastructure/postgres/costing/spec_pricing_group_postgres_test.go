package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/costing"
	"os"
	"testing"
	"time"
)

func TestResolveSpecPricingGroupForProductPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	s := fmt.Sprintf("test_specprice_%d", time.Now().UnixNano())
	if _, err = p.Exec(ctx, "CREATE SCHEMA "+s); err != nil {
		t.Fatal(err)
	}
	defer p.Exec(ctx, "DROP SCHEMA "+s+" CASCADE")
	_, err = p.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.spec_pricing_groups(id bigint,spec_template_id bigint,spec_template_version_id bigint,revision bigint,active bool);
 CREATE TABLE %[1]s.spec_pricing_group_items(id bigint,group_id bigint,spec_key text,inventory_unit text,pricing_rule_id bigint,sort_order int);
 CREATE TABLE %[1]s.production_bom_spec_templates(id bigint,active bool);
 CREATE TABLE %[1]s.production_bom_spec_template_versions(id bigint,template_id bigint,status text);
 CREATE TABLE %[1]s.product_pricing_rules(id bigint,active bool,cost_source_mode text DEFAULT 'bom',margin_rate numeric DEFAULT 0.5,tax_rate numeric DEFAULT 0,rounding_mode text DEFAULT 'none',calculation_json jsonb DEFAULT '{}',formula_version text DEFAULT 'v1');
 CREATE TABLE %[1]s.production_bom_output_bindings(output_type text,output_id bigint,is_default bool,bom_id bigint,bom_version_id bigint);
 CREATE TABLE %[1]s.production_bom_versions(id bigint,bom_id bigint,status text);
 CREATE TABLE %[1]s.production_bom_specs(id bigint,bom_id bigint,source_spec_template_id bigint,source_spec_template_key text,inventory_unit text);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint,version_id bigint,bom_spec_id bigint);
 INSERT INTO %[1]s.spec_pricing_groups VALUES(1,2,3,4,true);
 INSERT INTO %[1]s.spec_pricing_group_items VALUES(1,1,'spec-1','袋',5,1),(2,1,'spec-2','袋',6,2);
 INSERT INTO %[1]s.production_bom_spec_templates VALUES(2,true);
 INSERT INTO %[1]s.production_bom_spec_template_versions VALUES(3,2,'published');
 INSERT INTO %[1]s.product_pricing_rules(id,active) VALUES(5,true),(6,true);
 INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',7,true,8,9);
 INSERT INTO %[1]s.production_bom_versions VALUES(9,8,'published');
 INSERT INTO %[1]s.production_bom_specs VALUES(10,8,2,'spec-1','袋'),(11,8,2,'spec-2','袋');
 INSERT INTO %[1]s.production_bom_version_variants VALUES(12,9,10),(13,9,11);`, s))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(p, s)
	g, err := repo.ResolveSpecPricingGroupForProduct(ctx, 1, 7)
	if err != nil || g.Revision != 4 || !g.Active || len(g.Items) != 2 || g.Items[0].BomVariantID != 12 || g.Items[1].PricingRuleID != 6 {
		t.Fatalf("group=%+v err=%v", g, err)
	}

	fingerprints := map[int64]string{}
	for _, item := range g.Items {
		fingerprints[item.PricingRuleID] = item.PricingRuleFingerprint
	}
	row := func(spec, variant, rule float64) map[string]any {
		return map[string]any{"parent_product_id": float64(7), "bom_spec_id": spec, "bom_variant_id": variant, "pricing_rule_id": rule, "pricing_rule_fingerprint": fingerprints[int64(rule)], "pricing_mode": "pricing_rule", "spec_pricing_group_id": float64(1), "spec_pricing_group_revision": float64(4)}
	}
	cmd := app.PublishBeanListCommand{Config: map[string]any{"price_list_template_selection": map[string]any{"product_pricing_scope": "spec_pricing_group_aware", "defaults": map[string]any{"pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(1)}}}, Content: map[string]any{"price_rows": []any{row(10, 12, 5), row(11, 13, 6)}}}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = repo.lockAndValidateSpecPricingGroups(ctx, tx, cmd); err != nil {
		t.Fatalf("transactional validation: %v", err)
	}
	concurrent, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = concurrent.Exec(ctx, "SET LOCAL lock_timeout='50ms'"); err != nil {
		t.Fatal(err)
	}
	if _, err = concurrent.Exec(ctx, "UPDATE "+s+".spec_pricing_groups SET revision=revision+1 WHERE id=1"); err == nil {
		t.Fatal("concurrent edit was not blocked")
	}
	_ = concurrent.Rollback(ctx)
	_ = tx.Rollback(ctx)
	_, err = p.Exec(ctx, "UPDATE "+s+".production_bom_specs SET source_spec_template_id=99 WHERE id=11")
	if err != nil {
		t.Fatal(err)
	}
	g, err = repo.ResolveSpecPricingGroupForProduct(ctx, 1, 7)
	if err != nil || g.Items[1].BomVariantID != 0 {
		t.Fatalf("foreign spec matched: %+v %v", g, err)
	}
}
