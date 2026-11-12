package catalog

import (
	"context"
	"fmt"
	app "orderapp/internal/application/catalog"
	"testing"
)

func TestSpecPricingGroupPostgresTransactions(t *testing.T) {
	r, p, s := customerCatalogFixture(t)
	ctx := context.Background()
	_, err := p.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.production_bom_spec_templates(id bigint PRIMARY KEY,name text,active bool);
 CREATE TABLE %[1]s.production_bom_spec_template_versions(id bigint PRIMARY KEY,template_id bigint,version_no text,status text);
 CREATE TABLE %[1]s.production_bom_spec_template_variants(id bigint PRIMARY KEY,version_id bigint,spec_key text,name text,inventory_unit text,sort_order int);
 CREATE TABLE %[1]s.product_pricing_rules(id bigint PRIMARY KEY,name text,active bool);
 CREATE TABLE %[1]s.spec_pricing_groups(id bigserial PRIMARY KEY,name text NOT NULL,spec_template_id bigint NOT NULL,spec_template_version_id bigint NOT NULL,revision bigint NOT NULL DEFAULT 1,active boolean NOT NULL DEFAULT true,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),created_by text DEFAULT '',updated_by text DEFAULT '');
 CREATE TABLE %[1]s.spec_pricing_group_items(id bigserial PRIMARY KEY,group_id bigint NOT NULL REFERENCES %[1]s.spec_pricing_groups(id),spec_key text,inventory_unit text,pricing_rule_id bigint,sort_order int);
 CREATE UNIQUE INDEX ON %[1]s.spec_pricing_group_items(group_id,lower(spec_key),lower(inventory_unit));
 INSERT INTO %[1]s.production_bom_spec_templates VALUES(1,'模板',true);
 INSERT INTO %[1]s.production_bom_spec_template_versions VALUES(2,1,'V1','published'),(3,1,'V2','draft');
 INSERT INTO %[1]s.production_bom_spec_template_variants VALUES(4,2,'g227','227g','袋',0),(5,2,'g100','100g','袋',1);
 INSERT INTO %[1]s.product_pricing_rules VALUES(6,'规则1',true),(7,'规则2',true),(8,'已停用',false);`, s))
	if err != nil {
		t.Fatal(err)
	}
	g := app.SpecPricingGroup{Name: "组", SpecTemplateID: 1, SpecTemplateVersionID: 2, Active: true, Actor: "test", Items: []app.SpecPricingGroupItem{{SpecKey: "g227", InventoryUnit: "袋", PricingRuleID: 6}, {SpecKey: "g100", InventoryUnit: "袋", PricingRuleID: 7}}}
	saved, err := r.SaveSpecPricingGroup(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 {
		t.Fatal(saved)
	}
	list, err := r.ListSpecPricingGroups(ctx)
	if err != nil || len(list) != 1 || len(list[0].Items) != 2 {
		t.Fatalf("list %+v %v", list, err)
	}
	opts, err := r.SpecPricingGroupOptions(ctx)
	if err != nil || len(opts.Templates) != 1 || len(opts.Templates[0].Versions) != 1 || len(opts.PricingRules) != 2 {
		t.Fatalf("options %+v %v", opts, err)
	}
	invalid := saved
	invalid.Items = []app.SpecPricingGroupItem{{SpecKey: "g227", InventoryUnit: "袋", PricingRuleID: 8}}
	if _, err = r.SaveSpecPricingGroup(ctx, invalid); err == nil {
		t.Fatal("inactive rule accepted")
	}
	invalid = saved
	invalid.Revision = 99
	if _, err = r.SaveSpecPricingGroup(ctx, invalid); err == nil {
		t.Fatal("stale update accepted")
	}
	invalid = saved
	invalid.SpecTemplateVersionID = 3
	if _, err = r.SaveSpecPricingGroup(ctx, invalid); err == nil {
		t.Fatal("draft version accepted")
	}
	saved.Name = "更新"
	updated, err := r.SaveSpecPricingGroup(ctx, saved)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("updated %+v %v", updated, err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".product_pricing_rules SET active=false WHERE id=6"); err != nil {
		t.Fatal(err)
	}
	modified := updated
	modified.Active = false
	modified.Items = append([]app.SpecPricingGroupItem(nil), updated.Items...)
	modified.Items[0].PricingRuleID = 7
	if _, err = r.SaveSpecPricingGroup(ctx, modified); err == nil {
		t.Fatal("deactivation changed mapping")
	}
	updated.Active = false
	if _, err = r.SaveSpecPricingGroup(ctx, updated); err != nil {
		t.Fatalf("deactivate with obsolete rule: %v", err)
	}
	var n int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+s+".audit_logs").Scan(&n); err != nil || n != 3 {
		t.Fatalf("audit count %d %v", n, err)
	}
}
