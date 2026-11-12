package costing

import (
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/costing"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"testing"
)

func TestSpecPricingGroupEndToEndPublicationPostgres(t *testing.T) {
	r, ctx := namedBatchPostgres(t)
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.products(id bigint PRIMARY KEY,name text,active bool,parent_product_id bigint DEFAULT 0,customer_id bigint DEFAULT 0);
 CREATE TABLE %[1]s.product_bom_spec_authorities(product_id bigint,state text,spec_identity_mode text,configured bool);
 CREATE TABLE %[1]s.production_boms(id bigint PRIMARY KEY,output_product_id bigint,output_type text DEFAULT 'product',output_material_id bigint,status text DEFAULT 'active');
 CREATE TABLE %[1]s.production_bom_output_bindings(output_type text,output_id bigint,is_default bool,bom_id bigint,bom_version_id bigint);
 CREATE TABLE %[1]s.production_bom_versions(id bigint,bom_id bigint,version_no text,status text,yield_rate numeric DEFAULT 1,output_qty numeric DEFAULT 1,output_unit text DEFAULT '袋');
 CREATE TABLE %[1]s.production_bom_specs(id bigint,bom_id bigint,code text,barcode text,spec_key text,name text,inventory_unit text,source_spec_template_id bigint,source_spec_template_key text);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint,version_id bigint,bom_spec_id bigint,spec_name_snapshot text,inventory_unit text,is_default bool,sort_order int);
 CREATE TABLE %[1]s.production_bom_version_operation_costs(version_id bigint,operation_unit_cost numeric,cost_method text,operation_cost_unit text);
 CREATE TABLE %[1]s.production_bom_version_items(id bigint,version_id bigint,component_type text,material_id bigint,component_product_id bigint,component_spec_g numeric,consume_unit text,qty_per_unit numeric,ratio_pct numeric,material_loss_rate numeric,unit_cost_snapshot numeric);
 CREATE TABLE %[1]s.materials(id bigint,name text,unit text,is_semi_finished bool,purchase_price numeric,estimated_unit_price numeric);
 CREATE TABLE %[1]s.material_batches(id bigint,unit_cost numeric,status text,quality_status text);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,material_id bigint,qty_g numeric,qty_units numeric);
 CREATE TABLE %[1]s.production_bom_spec_templates(id bigint,active bool);
 CREATE TABLE %[1]s.production_bom_spec_template_versions(id bigint,template_id bigint,status text);
 CREATE TABLE %[1]s.product_pricing_rules(id bigint,active bool,name text DEFAULT 'rule',code text DEFAULT '',remark text DEFAULT '',cost_source_mode text DEFAULT 'bom',margin_rate numeric DEFAULT 0.5,tax_rate numeric DEFAULT 0,rounding_mode text DEFAULT 'none',calculation_json jsonb DEFAULT '{}',formula_version text DEFAULT 'v1');
 CREATE TABLE %[1]s.spec_pricing_groups(id bigint,spec_template_id bigint,spec_template_version_id bigint,revision bigint,active bool);
 CREATE TABLE %[1]s.spec_pricing_group_items(id bigint,group_id bigint,spec_key text,inventory_unit text,pricing_rule_id bigint,sort_order int);
 INSERT INTO %[1]s.products(id,name,active) VALUES(7,'商品',true);
 INSERT INTO %[1]s.product_bom_spec_authorities VALUES(7,'cutover','bom_spec',true);
 INSERT INTO %[1]s.production_boms(id,output_product_id) VALUES(8,7);
 INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',7,true,8,9);
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status) VALUES(9,8,'V1','published');
 INSERT INTO %[1]s.production_bom_specs VALUES(10,8,'S10','','g227','227g','袋',2,'spec-1'),(11,8,'S11','','g100','100g','袋',2,'spec-2'),(12,8,'S12','','g50','50g','袋',2,'spec-3');
 INSERT INTO %[1]s.production_bom_version_variants VALUES(20,9,10,'227g','袋',true,1),(21,9,11,'100g','袋',false,2),(22,9,12,'50g','袋',false,3);
 INSERT INTO %[1]s.production_bom_spec_templates VALUES(2,true);
 INSERT INTO %[1]s.production_bom_spec_template_versions VALUES(3,2,'published');
 INSERT INTO %[1]s.product_pricing_rules(id,active) VALUES(30,true),(31,true),(32,true);
 INSERT INTO %[1]s.spec_pricing_groups VALUES(1,2,3,4,true);
 INSERT INTO %[1]s.spec_pricing_group_items VALUES(1,1,'spec-1','袋',30,1),(2,1,'spec-2','袋',31,2),(3,1,'spec-3','袋',32,3);`, r.schema))
	if err != nil {
		t.Fatal(err)
	}
	resolvedGroup, err := r.ResolveSpecPricingGroupForProduct(ctx, 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	loadedRule, err := r.LoadProductPricingRule(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolvedGroup.Items) == 0 || loadedRule.Fingerprint != resolvedGroup.Items[0].PricingRuleFingerprint {
		t.Fatalf("trial and publication fingerprint differ: %q %+v", loadedRule.Fingerprint, resolvedGroup.Items)
	}
	fingerprints := map[int64]string{}
	for _, item := range resolvedGroup.Items {
		fingerprints[item.PricingRuleID] = item.PricingRuleFingerprint
	}
	selections := []any{}
	rows := []any{}
	for i := 0; i < 3; i++ {
		spec, variant, rule := float64(10+i), float64(20+i), float64(30+i)
		selections = append(selections, map[string]any{"parent_product_id": float64(7), "bom_id": float64(8), "bom_version_id": float64(9), "bom_spec_id": spec, "bom_variant_id": variant, "selection_source": "explicit"})
		rows = append(rows, map[string]any{"product_id": float64(7), "parent_product_id": float64(7), "bom_id": float64(8), "bom_version_id": float64(9), "bom_spec_id": spec, "bom_variant_id": variant, "spec_pricing_group_id": float64(1), "spec_pricing_group_revision": float64(4), "pricing_mode": "pricing_rule", "pricing_mode_source": "category", "pricing_rule_id": rule, "pricing_rule_fingerprint": fingerprints[int64(rule)], "pricing_rule_source": "group", "pricing_rule_version": "v1", "final_unit_price": float64(30 + i), "price_unit": "袋", "inventory_unit": "袋", "inventory_conversion_json": map[string]any{"袋": map[string]any{"袋": float64(1)}}, "group_snapshot": map[string]any{"group_item_id": float64(102), "group_item_name": "子分类"}, "group_source": "product_catalog", "cost_source_snapshot": map[string]any{"source": "test"}, "customer_reference_snapshot": map[string]any{}, "manual_adjusted": false})
	}
	config := map[string]any{"product_spec_selections": selections, "price_list_template_selection": map[string]any{"product_pricing_scope": "spec_pricing_group_aware", "group_selections": []any{map[string]any{"group_item_id": float64(101), "pricing_mode": "spec_pricing_group", "spec_pricing_group_id": float64(1)}, map[string]any{"group_item_id": float64(102), "parent_group_item_id": float64(101)}}}}
	content := map[string]any{"price_rows": rows}
	cmd := app.PublishBeanListCommand{ListType: "commercial", Version: "V3.0.6", OwnerType: "official", PublicationPurpose: "factory_supply", Config: config, Content: content, Actor: "test"}
	svc := app.NewService(r)
	published, err := svc.PublishBeanList(ctx, cmd)
	if err != nil {
		t.Fatalf("single publication: %v", err)
	}
	if len(published.Content["price_rows"].([]any)) != 3 {
		t.Fatalf("rows=%+v", published.Content)
	}
	storedContent, err := json.Marshal(published.Content)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		price, ok := orderbeans.PublishedFixedBOMSpecPricing(storedContent, 7, int64(10+i), "commercial")
		if !ok || price.UnitPrice != float64(30+i) {
			t.Fatalf("spec %d fixed quote=%+v ok=%v", 10+i, price, ok)
		}
	}
	batch := app.BeanListBatchCommand{PublishBeanListCommand: cmd, DefaultTableKey: "one", Tables: []app.BeanListBatchTable{{Key: "one", Name: "三规格", Config: config, Content: content}}}
	out, err := svc.PublishBeanListBatch(ctx, batch)
	if err != nil {
		t.Fatalf("batch publication: %v", err)
	}
	if len(out.Tables) != 1 {
		t.Fatal(out)
	}
	var n int
	if err = r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+".audit_logs WHERE action='publish'").Scan(&n); err != nil || n < 2 {
		t.Fatalf("publish audits=%d %v", n, err)
	}
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".product_pricing_rules SET margin_rate=0.75 WHERE id=30")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.PublishBeanList(ctx, cmd); err == nil {
		t.Fatal("publication accepted changed pricing rule after trial")
	}
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".product_pricing_rules SET margin_rate=0.5 WHERE id=30")
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".spec_pricing_groups SET revision=5 WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.PublishBeanList(ctx, cmd); err == nil {
		t.Fatal("stale group publication accepted")
	}
}
