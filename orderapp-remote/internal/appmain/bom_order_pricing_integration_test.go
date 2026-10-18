package appmain

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	bomapp "orderapp/internal/application/bom"
	costingapp "orderapp/internal/application/costing"
	salesapp "orderapp/internal/application/sales"
	bompg "orderapp/internal/infrastructure/postgres/bom"
	costingpg "orderapp/internal/infrastructure/postgres/costing"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"strings"
	"testing"
)

func TestPriceTableOnlyOrderScopePostgresHTTP(t *testing.T) {
	f := newSampleFixture(t)
	f.e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Admin") == "yes" {
				c.Set("employee_id", f.admin.EmployeeID)
				c.Set("operator_employee", "BOM测试员")
			}
			return next(c)
		}
	})
	service := salesapp.NewService(salespg.NewRepository(f.pool, f.schema))
	saleshttp.RegisterRoutes(f.e, saleshttp.Dependencies{Sales: service, Authz: wechatTestAuthz{}})
	pid := f.productIDs[0]
	f.exec("INSERT INTO %s.product_unit_definitions(code,name,unit_type,active) VALUES('袋','袋','package',true),('个','个','package',true) ON CONFLICT(code) DO UPDATE SET active=true,deleted_at=NULL")
	f.exec("UPDATE %s.products SET unit_rule_override_json='{\"inventory_unit\":\"袋\"}' WHERE id=$1", pid)
	mat := f.id("INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('BOM-PRICE-MAT','熟豆','bean','kg','kg',80) RETURNING id")
	bag := f.id("INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('BOM-PRICE-BAG','小袋','pack','个','个',1) RETURNING id")
	bigBag := f.id("INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('BOM-PRICE-BIG','大袋','pack','个','个',3) RETURNING id")
	bomsvc := bomapp.NewService(bompg.NewRepository(f.pool, f.schema))
	variants := []bomapp.ProductionBomDraftVariant{}
	for i, qty := range []float64{.1, .5, .25} {
		packaging := bag
		if i == 1 {
			packaging = bigBag
		}
		variants = append(variants, bomapp.ProductionBomDraftVariant{SpecKey: fmt.Sprintf("spec%d", i), Name: []string{"100g", "500g", "250g"}[i], InventoryUnit: "袋", IsDefault: i == 1, SortOrder: i + 1, Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: mat, ConsumeUnit: "kg", QtyPerUnit: qty}, {ComponentType: "material", MaterialID: packaging, ConsumeUnit: "个", QtyPerUnit: 1}}})
	}
	bom, err := bomsvc.CreateProductionBom(f.ctx, bomapp.CreateProductionBomCommand{Name: "报价测试BOM", OutputType: "product", OutputProductID: pid, OutputQty: 1, Actor: "test", Variants: variants})
	if err != nil {
		t.Fatal(err)
	}
	if err = bomsvc.PublishProductionBomVersion(f.ctx, bomapp.PublishProductionBomVersionCommand{VersionID: bom.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err = bomsvc.BindProductProductionBom(f.ctx, bomapp.BindProductProductionBomCommand{ProductID: pid, BomID: bom.ID, BomVersionID: bom.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	spec := func(name string) (int64, int64) {
		var a, b int64
		err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT bom_spec_id,id FROM %s.production_bom_version_variants WHERE version_id=$1 AND spec_name_snapshot=$2", f.schema), bom.LatestVersionID, name).Scan(&a, &b)
		if err != nil {
			t.Fatal(err)
		}
		return a, b
	}
	source, sourceV := spec("100g")
	target, targetV := spec("500g")
	_, _ = spec("250g")
	customer := f.id("INSERT INTO %s.customers(name,customer_type) VALUES('BOM报价客户','wholesale') RETURNING id")
	sourceID := f.id("INSERT INTO %s.sources(name) VALUES('BOM来源') RETURNING id")
	orderTypeID := f.id("INSERT INTO %s.order_types(name) VALUES('BOM批发') RETURNING id")
	f.exec("UPDATE %s.customers SET default_source_id=$1,default_order_type_id=$2,responsible_employee_id=$3 WHERE id=$4", sourceID, orderTypeID, f.sales.EmployeeID, customer)
	ruleID := f.id("INSERT INTO %s.product_pricing_rules(name,code,cost_source_mode,margin_rate,tax_rate,rounding_mode,calculation_json,formula_version,active) VALUES('档位一','BOM-PRICE','bom_current_cost',.2,0,'cent','{\"tax_mode\":\"none\"}','v1',true) RETURNING id")
	rule := costingapp.ProductPricingRule{ID: ruleID, Name: "档位一", Active: true, MarginRate: .2, RoundingMode: "cent", FormulaVersion: "v1", CalculationJSON: map[string]any{"tax_mode": "none"}}
	secondRuleID := f.id("INSERT INTO %s.product_pricing_rules(name,code,cost_source_mode,margin_rate,tax_rate,rounding_mode,calculation_json,formula_version,active) VALUES('档位二','BOM-PRICE-2','bom_current_cost',.1,0,'cent','{\"tax_mode\":\"none\"}','v1',true) RETURNING id")
	rows := []map[string]any{}
	for i, rate := range []float64{.2, .1} {
		frozen := rule
		frozen.MarginRate = rate
		if i == 1 {
			frozen.ID = secondRuleID
		}
		frozen.Name = fmt.Sprintf("档位%d", i+1)
		rows = append(rows, map[string]any{"product_id": pid, "parent_product_id": pid, "sku_id": source, "bom_spec_id": source, "bom_variant_id": sourceV, "spec_name": "100g", "price_tier": fmt.Sprint(i), "effective_sales_spec": map[string]any{"bom_spec_id": source, "bom_variant_id": sourceV, "spec_name": "100g", "inventory_unit": "袋"}, "quantity_basis": "sales_spec_count", "min_qty": i * 48, "final_unit_price": 10.8 - float64(i)*.9, "price_unit": "袋", "inventory_unit": "袋", "tier_label": frozen.Name, "pricing_mode": "tier_template", "pricing_rule_id": frozen.ID, "tier_pricing_rule_id": frozen.ID, "pricing_rule_snapshot": frozen})
	}
	rows[0]["max_qty"] = 47
	// Server publishing overwrites forged client rule snapshots and retains sale prices.
	publishRows := []any{}
	for _, row := range rows {
		copy := map[string]any{}
		for k, v := range row {
			copy[k] = v
		}
		copy["pricing_rule_snapshot"] = map[string]any{"id": ruleID, "margin_rate": 99}
		publishRows = append(publishRows, copy)
	}
	published, err := costingpg.NewRepository(f.pool, f.schema).PublishBeanList(f.ctx, costingapp.PublishBeanListCommand{OwnerType: "official", ListType: "commercial", Version: "V-BOM-694", Actor: "test", Config: map[string]any{}, Content: map[string]any{"price_rows": publishRows}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(published.Content)
	if strings.Contains(string(encoded), `"margin_rate":99`) {
		t.Fatalf("client template snapshot trusted: %s", encoded)
	}
	f.exec("UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=$1", published.ID)
	content, _ := json.Marshal(map[string]any{"price_rows": rows})
	f.exec("UPDATE %s.bean_list_publications SET content_json=$1,config_json='{\"publication_batch\":{\"release_id\":\"bom-test\",\"table_key\":\"source\",\"table_name\":\"100g表\",\"is_default_table\":true}}' WHERE id=$2", content, f.publication)
	key := orderbeans.PublishedPriceRowKey(f.publication, "commercial", pid, source, sourceV, "flat", 0)

	for _, route := range []string{"/api/order/spec-conversions", "/api/mini/employee/spec-conversions"} {
		token := "basic-admin"
		if strings.Contains(route, "/mini/") {
			token = f.salesToken
		}
		f.call("GET", fmt.Sprintf("%s?customer_id=%d&product_id=%d&publication_id=%d", route, customer, pid, f.publication), nil, token, 410)
		f.call("POST", route+"/quote", map[string]any{}, token, 410)
	}
	if _, err := service.OrderForm(f.ctx, 0); err != nil {
		t.Fatal("order form", err)
	}
	// Both ordinary APIs must reject a real BOM spec absent from the selected table.
	payload := map[string]any{"pay_status_id": f.id("SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1"), "ship_status_id": f.id("SELECT id FROM %s.ship_statuses WHERE name='未发货' LIMIT 1"), "order_date": "2026-10-18", "customer_id": customer, "product_id": []string{fmt.Sprint(pid)}, "bom_spec_id": []string{fmt.Sprint(target)}, "bom_variant_id": []string{fmt.Sprint(targetV)}, "qty": []string{"2"}, "item_name": []string{"测试商品"}, "unit_price": []string{"0.01"}, "price_selection_mode": []string{"auto"}, "item_bean_list_publication_id": []string{fmt.Sprint(f.publication)}, "selected_price_table_ids": []int64{f.publication}}
	miniItem := map[string]any{"product_id": pid, "bom_spec_id": target, "bom_variant_id": targetV, "qty": 2, "unit": "袋", "name": "测试商品", "unit_price": 0.01, "bean_list_publication_id": f.publication, "price_selection_mode": "auto"}
	miniPayload := map[string]any{"order_date": "2026-10-18", "customer_id": customer, "pay_status_id": payload["pay_status_id"], "ship_status_id": payload["ship_status_id"], "selected_price_table_ids": []int64{f.publication}, "items": []any{miniItem}}
	f.call("POST", "/api/order", payload, "basic-admin", 400)
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 400)
	payload["price_selection_mode"] = []string{"manual"}
	payload["manual_price"] = []string{"1"}
	payload["unit_price"] = []string{"99"}
	miniItem["price_selection_mode"] = "manual"
	miniItem["price_override"] = true
	miniItem["unit_price"] = 99
	f.call("POST", "/api/order", payload, "basic-admin", 400)
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 400)
	delete(payload, "manual_price")
	payload["unit_price"] = []string{"0.01"}
	miniItem["price_override"] = false
	miniItem["unit_price"] = 0.01
	// Internal legacy calculator is used solely to reconstruct a pre-rollback fixture.
	req := salesapp.SpecConversionQuoteRequest{SpecConversionSelection: salesapp.SpecConversionSelection{PricingMethod: "bom_rule", SourceSpecID: source, SourceVariantID: sourceV}, CustomerID: customer, ProductID: pid, PublicationID: f.publication, PriceRowKey: key, TargetSpecID: target, TargetVariantID: targetV, Quantity: 2}
	quote, err := salespg.NewRepository(f.pool, f.schema).QuoteSpecConversion(f.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	selection, _ := json.Marshal(quote.SpecConversionSelection)
	payload["price_selection_mode"] = []string{"bom_tier"}
	payload["selected_price_row_key"] = []string{key}
	payload["spec_conversion"] = []string{string(selection)}
	miniItem["price_selection_mode"] = "bom_tier"
	miniItem["selected_price_row_key"] = key
	miniItem["spec_conversion"] = quote.SpecConversionSelection
	f.call("POST", "/api/order", payload, "basic-admin", 400)
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 400)
	// A priced spec still creates an ordinary order, with server-side price verification.
	payload["bom_spec_id"] = []string{fmt.Sprint(source)}
	payload["bom_variant_id"] = []string{fmt.Sprint(sourceV)}
	payload["price_selection_mode"] = []string{"auto"}
	delete(payload, "selected_price_row_key")
	delete(payload, "spec_conversion")
	miniItem["bom_spec_id"] = source
	miniItem["bom_variant_id"] = sourceV
	miniItem["price_selection_mode"] = "auto"
	delete(miniItem, "selected_price_row_key")
	delete(miniItem, "spec_conversion")
	saved := f.call("POST", "/api/order", payload, "basic-admin", 200)
	var orderID int64
	_ = json.Unmarshal(saved["order_id"], &orderID)
	miniSaved := f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 201)
	var miniID int64
	_ = json.Unmarshal(miniSaved["order_id"], &miniID)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id IN ($1,$2) AND bom_spec_id=$3 AND unit_price=10.80", orderID, miniID, source); n != 2 {
		t.Fatal("ordinary price changed")
	}
	for _, id := range []int64{orderID, miniID} {
		f.exec("UPDATE %s.order_items SET bom_spec_id=$1,bom_variant_id=$2,unit_price=51.60,line_total=103.20,price_source_json=$3 WHERE order_id=$4", target, targetV, quote.PriceSourceJSON, id)
	}
	// Editing a genuine saved conversion freezes its price. A copied/forged line cannot do so.
	itemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1", orderID)
	payload["edit_id"] = orderID
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["bom_spec_id"] = []string{fmt.Sprint(target)}
	payload["bom_variant_id"] = []string{fmt.Sprint(targetV)}
	payload["price_selection_mode"] = []string{"bom_tier"}
	payload["selected_price_row_key"] = []string{key}
	payload["spec_conversion"] = []string{string(selection)}
	payload["qty"] = []string{"3"}
	payload["price_source_json"] = []string{`{"spec_conversion":{"unit_price":"0.01"}}`}
	f.call("POST", "/api/order", payload, "basic-admin", 200)
	miniDetail := f.call("GET", fmt.Sprintf("/api/mini/employee/orders/%d", miniID), nil, f.salesToken, 200)
	var detail map[string]any
	_ = json.Unmarshal(miniDetail["order"], &detail)
	miniPayload["edit_revision"] = detail["edit_revision"]
	miniItem["item_id"] = f.id("SELECT id FROM %s.order_items WHERE order_id=$1", miniID)
	miniItem["bom_spec_id"] = target
	miniItem["bom_variant_id"] = targetV
	miniItem["price_selection_mode"] = "bom_tier"
	miniItem["selected_price_row_key"] = key
	miniItem["spec_conversion"] = quote.SpecConversionSelection
	miniItem["qty"] = 3
	f.call("PUT", fmt.Sprintf("/api/mini/employee/orders/%d", miniID), miniPayload, f.salesToken, 200)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id IN ($1,$2) AND bom_spec_id=$3 AND unit_price=51.60 AND line_total=154.80", orderID, miniID, target); n != 2 {
		t.Fatal("historical target or price changed")
	}
	delete(payload, "edit_id")
	f.call("POST", "/api/order", payload, "basic-admin", 400)
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 400)
	var after string
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT content_json::text FROM %s.bean_list_publications WHERE id=$1", f.schema), f.publication).Scan(&after); err != nil {
		t.Fatal(err)
	}
	var beforeJSON, afterJSON any
	_ = json.Unmarshal(content, &beforeJSON)
	_ = json.Unmarshal([]byte(after), &afterJSON)
	a, _ := json.Marshal(beforeJSON)
	b, _ := json.Marshal(afterJSON)
	if string(a) != string(b) {
		t.Fatal("price table mutated")
	}
	if n := f.id("SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1", orderID); n == 0 {
		t.Fatal("missing order audit")
	}
}
