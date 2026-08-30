package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestProductionPlanCreatesDirectProductUnitDependencyChain(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products
		SET name='盒装挂耳',product_kind='drip_bag',spec_label='盒',net_content_qty=1,net_content_unit='盒',
		    unit_rule_override_json='{"inventory_unit":"盒","default_sales_unit":"盒","unit_conversion_json":{"盒":{"盒":1}}}'::jsonb
		WHERE id=1;
		INSERT INTO %[1]s.products(
			id,name,default_price,active,product_kind,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (
			2,'袋装挂耳',5,true,'drip_bag','袋',1,'袋',
			'{"inventory_unit":"袋","default_sales_unit":"袋","unit_conversion_json":{"袋":{"袋":1}}}'::jsonb
		);

		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		UPDATE %[1]s.production_bom_versions
		SET output_qty=1,output_unit='盒',process_route_id=31,material_loss_rate=0
		WHERE id=100;
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,component_product_id,component_bom_spec_id,
			component_spec_g,consume_unit,qty_per_unit,ratio_pct
		) VALUES (100,0,'finished_product',2,0,0,'unit',10,0);

		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status)
		VALUES(300,'PBOM-DIRECT-BAG','袋装挂耳 BOM','product',2,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,output_qty,output_unit,process_route_id,published_at
		) VALUES(300,300,'V001','published',1,'袋',31,now());
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct
		) VALUES(300,20,'material','unit',1,0);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES(2,300,300,'test');
		INSERT INTO %[1]s.production_bom_output_bindings(
			output_type,output_id,bom_id,bom_version_id,is_default,updated_by
		) VALUES('product',2,300,300,true,'test');

		UPDATE %[1]s.order_items
		SET product_id=1,bom_spec_id=0,bom_variant_id=0,item_name='盒装挂耳',spec='盒',
		    unit='盒',sales_unit='盒',qty=2,
		    price_source_json='{"production_quantity_snapshot":{"parent_product_id":1,"bom_spec_id":0,"bom_variant_id":0,"spec_label":"盒","sales_unit":"盒","inventory_unit":"盒","inventory_qty_per_sales_unit":1,"conversion_source":"product_unit_rule"}}'::jsonb
		WHERE order_id=1;
	`, schema))

	app := newProductionFlowTestEcho(pool, schema)
	preview := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if preview.Code != http.StatusOK {
		t.Fatalf("GET direct-product demand status=%d body=%s", preview.Code, preview.Body.String())
	}
	var previewPayload struct {
		Rows []struct {
			ProductID       int64   `json:"product_id"`
			SelectionKey    string  `json:"selection_key"`
			SpecG           int64   `json:"spec_g"`
			InventoryUnit   string  `json:"inventory_unit"`
			GapInventoryQty float64 `json:"gap_inventory_qty"`
			Selectable      bool    `json:"demand_selectable"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &previewPayload); err != nil {
		t.Fatalf("decode direct-product preview: %v body=%s", err, preview.Body.String())
	}
	matching := 0
	for _, row := range previewPayload.Rows {
		if row.ProductID == 1 {
			matching++
			if row.SelectionKey != "1-0" || row.SpecG != 0 || row.InventoryUnit != "盒" || row.GapInventoryQty != 2 || !row.Selectable {
				t.Fatalf("direct-product demand row=%+v, want selectable 2盒 with product identity", row)
			}
		}
	}
	if matching != 1 {
		t.Fatalf("direct-product demand rows=%d, want one authoritative row body=%s", matching, preview.Body.String())
	}

	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-0"},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("POST direct-product plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_product_id=1 AND bom_spec_id=0 AND bom_variant_id=0 AND output_qty=2 AND output_unit='盒' AND planned_inventory_qty=2", planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_product_id=2 AND bom_spec_id=0 AND bom_variant_id=0 AND output_qty=20 AND output_unit='袋' AND planned_inventory_qty=20", planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='product' AND component_id=2 AND component_bom_spec_id=0 AND component_bom_variant_id=0 AND required_units=20", planID,
	), 1)
	if strings.Contains(create.Body.String(), "requires a weight inventory unit") {
		t.Fatalf("direct-product unit plan must not use legacy weight-only rejection: %s", create.Body.String())
	}
}
