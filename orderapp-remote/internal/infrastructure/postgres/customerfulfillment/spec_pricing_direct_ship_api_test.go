package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	catalogapp "orderapp/internal/application/catalog"
	costapp "orderapp/internal/application/costing"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	pgcatalog "orderapp/internal/infrastructure/postgres/catalog"
	pgcosting "orderapp/internal/infrastructure/postgres/costing"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
)

func TestSpecPricingGroupPublishBindAndDirectShipHTTP(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	seedPR706FrozenWeightMaterials(t, pool, schema)
	configurePR703StockFees(t, pool, schema, 7031)
	repo := NewRepository(pool, schema)
	settings, err := repo.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	for i := range settings.Modes {
		settings.Modes[i].ShippingPolicy = nil
	}
	mode := settings.Modes[0]
	mode.Mode = "on_demand"
	mode.WarehouseCode = ""
	settings.Modes = []app.FulfillmentModeSettings{settings.Modes[0], mode}
	if _, err = repo.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Settings: settings, ExpectedRevision: settings.Revision, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_spec_templates(id,code,name) VALUES(9001,'E2E-SPEC','规格模板');
 INSERT INTO %[1]s.production_bom_spec_template_versions(id,template_id,version_no,status) VALUES(9002,9001,'V1','published');
 INSERT INTO %[1]s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit) VALUES(9002,'s1','227g','袋'),(9002,'s2','100g','袋'),(9002,'s3','50g','袋');
 UPDATE %[1]s.production_bom_specs SET source_spec_template_id=9001,source_spec_template_key='s1' WHERE id=7031;
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit,source_spec_template_id,source_spec_template_key) VALUES(7035,7030,'S100','g100','100g','袋',9001,'s2'),(7036,7030,'S50','g50','50g','袋',9001,'s3');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,logistics_weight_g) VALUES(7037,7030,7035,'100g','袋',120),(7038,7030,7036,'50g','袋',65);
 UPDATE %[1]s.materials SET purchase_price=CASE id WHEN 70601 THEN 100 ELSE 1 END WHERE id IN(70601,70602);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate) VALUES(7030,7037,70601,'material','fixed_qty',.100,0,0),(7030,7037,70602,'material','fixed_qty',1,0,0),(7030,7038,70601,'material','fixed_qty',.050,0,0),(7030,7038,70602,'material','fixed_qty',1,0,0);
 INSERT INTO %[1]s.product_pricing_rules(id,name,cost_source_mode,margin_rate,formula_version,active)VALUES(9030,'规则227','bom_current_cost',.2,'v1',true),(9031,'规则100','bom_current_cost',.3,'v1',true),(9032,'规则50','bom_current_cost',.4,'v1',true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalogapp.NewService(pgcatalog.NewRepository(pool, schema)).CopyCustomerCatalog(ctx, catalogapp.CopyCustomerCatalogCommand{CustomerID: 501, Mode: "selected", ProductIDs: []int64{910}, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	group, err := catalogapp.NewService(pgcatalog.NewRepository(pool, schema)).SaveSpecPricingGroup(ctx, catalogapp.SpecPricingGroup{Name: "测试计价组", SpecTemplateID: 9001, SpecTemplateVersionID: 9002, Active: true, Actor: "test", Items: []catalogapp.SpecPricingGroupItem{{SpecKey: "s1", InventoryUnit: "袋", PricingRuleID: 9030}, {SpecKey: "s2", InventoryUnit: "袋", PricingRuleID: 9031}, {SpecKey: "s3", InventoryUnit: "袋", PricingRuleID: 9032}}})
	if err != nil {
		t.Fatal(err)
	}
	cr := pgcosting.NewRepository(pool, schema)
	cs := costapp.NewService(cr)
	specs := []int64{7031, 7035, 7036}
	variants := []int64{7032, 7037, 7038}
	prices := []float64{30, 15, 8}
	selections := []any{}
	priceRows := []any{}
	for i := range specs {
		rule, err := cr.LoadProductPricingRule(ctx, int64(9030+i))
		if err != nil {
			t.Fatal(err)
		}
		selections = append(selections, map[string]any{"parent_product_id": 910, "bom_id": 7030, "bom_version_id": 7030, "bom_spec_id": specs[i], "bom_variant_id": variants[i], "selection_source": "explicit"})
		priceRows = append(priceRows, map[string]any{"product_id": 910, "parent_product_id": 910, "bom_id": 7030, "bom_version_id": 7030, "bom_spec_id": specs[i], "bom_variant_id": variants[i], "spec_pricing_group_id": group.ID, "spec_pricing_group_revision": group.Revision, "pricing_rule_id": rule.ID, "pricing_rule_fingerprint": rule.Fingerprint, "pricing_mode": "pricing_rule", "pricing_mode_source": "category", "pricing_rule_source": "group", "pricing_rule_version": "v1", "min_qty": 1, "final_unit_price": prices[i], "price_unit": "袋", "inventory_unit": "袋", "inventory_conversion_json": map[string]any{"袋": map[string]any{"袋": 1}}, "group_snapshot": map[string]any{"group_item_id": 101, "group_item_name": "咖啡"}, "group_source": "product_catalog", "cost_source_snapshot": map[string]any{"source": "test"}, "customer_reference_snapshot": map[string]any{}, "manual_adjusted": false})
	}
	cfg := map[string]any{"product_spec_selections": selections, "price_list_template_selection": map[string]any{"product_pricing_scope": "spec_pricing_group_aware", "group_selections": []any{map[string]any{"group_item_id": 101, "pricing_mode": "spec_pricing_group", "spec_pricing_group_id": group.ID}}}}
	pub, err := cs.PublishBeanList(ctx, costapp.PublishBeanListCommand{ListType: "commercial", Version: "V3.0.99", OwnerType: "customer", OwnerKey: "501", PublicationPurpose: "factory_supply", Config: cfg, Content: map[string]any{"price_rows": priceRows}, Actor: "test"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pub.Content["groups"] == nil {
		t.Fatal("new publication groups is null")
	}
	// An older publication can still contain JSON null: preserve the snapshot
	// and prove ordering reads it safely without a data repair.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.bean_list_publications SET content_json=jsonb_set(content_json,'{groups}','null'::jsonb) WHERE id=$1`, schema), pub.ID); err != nil {
		t.Fatal(err)
	}
	_, err = cs.SaveCustomerOrderPriceTableBinding(ctx, costapp.SaveCustomerOrderPriceTableBindingCommand{CustomerID: 501, UsageCode: "direct_ship", PublicationID: pub.ID, Actor: "test"})
	if err != nil {
		t.Fatalf("bind direct ship price table: %v", err)
	}

	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("operator_employee", "销售员")
			return next(c)
		}
	})
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: svc})
	cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", FulfillmentMode: "on_demand", IdempotencyKey: "SPEC-GROUP-E2E", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}, {ProductID: 910, BomSpecID: 7035, BomVariantID: 7037, Qty: 3}, {ProductID: 910, BomSpecID: 7036, BomVariantID: 7038, Qty: 4}}}
	call := func(path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status %d: %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var preview app.MiniDirectShipPreview
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/preview", cmd, 200), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.CanSubmit || len(preview.Items) != 3 || preview.TotalAmount != 137 {
		t.Fatalf("preview %+v", preview)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	var result app.MiniDirectShipRequest
	if err = json.Unmarshal(call("/api/customer-fulfillment/501/direct-ship/requests", cmd, 201), &result); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT bom_spec_id,qty,unit_price,line_total,price_source_json::text FROM %s.order_items WHERE order_id=$1 ORDER BY line_no`, schema), result.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i := range specs {
		if !rows.Next() {
			t.Fatal("missing order line")
		}
		var spec, qty int64
		var price, total float64
		var source string
		if err = rows.Scan(&spec, &qty, &price, &total, &source); err != nil {
			t.Fatal(err)
		}
		wantQty := int64(i + 2)
		if spec != specs[i] || qty != wantQty || price != prices[i] || total != float64(wantQty)*prices[i] {
			t.Fatalf("line %d: %d %d %v %v", i, spec, qty, price, total)
		}
		if !strings.Contains(source, fmt.Sprintf(`"publication_id": %d`, pub.ID)) {
			t.Fatalf("price snapshot missing publication: %s", source)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatalf("unexpected lines: %v", rows.Err())
	}
}
