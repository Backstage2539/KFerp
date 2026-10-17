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

func TestBOMOrderPricingPostgresHTTP(t *testing.T) {
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
	small, smallV := spec("250g")
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
		response := f.call("GET", fmt.Sprintf("%s?customer_id=%d&product_id=%d&publication_id=%d", route, customer, pid, f.publication), nil, token, 200)
		encodedCatalog, _ := json.Marshal(response)
		var catalog salesapp.SpecConversionCatalog
		if err := json.Unmarshal(encodedCatalog, &catalog); err != nil {
			t.Fatal(err)
		}
		if len(catalog.SourceTiers) != 2 {
			t.Fatalf("source tiers: %s", response)
		}
		for i, tier := range catalog.SourceTiers {
			if tier.MinQty == nil || *tier.MinQty != float64(i*48) || (i == 0 && (tier.MaxQty == nil || *tier.MaxQty != 47)) || (i == 1 && tier.MaxQty != nil) || tier.QuantityBasis != "sales_spec_count" {
				t.Fatalf("missing published range: %+v", tier)
			}
		}
	}

	request := salesapp.SpecConversionQuoteRequest{SpecConversionSelection: salesapp.SpecConversionSelection{PricingMethod: "bom_rule", SourceSpecID: source, SourceVariantID: sourceV}, CustomerID: customer, ProductID: pid, PublicationID: f.publication, PriceRowKey: key, TargetSpecID: target, TargetVariantID: targetV, Quantity: 2}
	q := func(req salesapp.SpecConversionQuoteRequest, mini bool, want int) salesapp.SpecConversionQuote {
		path, token := "/api/order/spec-conversions/quote", "basic-admin"
		if mini {
			path, token = "/api/mini/employee/spec-conversions/quote", f.salesToken
		}
		out := f.call("POST", path, req, token, want)
		b, _ := json.Marshal(out)
		var quote salesapp.SpecConversionQuote
		_ = json.Unmarshal(b, &quote)
		return quote
	}
	catalog := f.call("GET", fmt.Sprintf("/api/order/spec-conversions?customer_id=%d&product_id=%d&publication_id=%d", customer, pid, f.publication), nil, "basic-admin", 200)
	var choices []salesapp.SpecConversionSpec
	_ = json.Unmarshal(catalog["specs"], &choices)
	if len(choices) != 3 || choices[0].SpecID != target || !choices[0].IsDefault {
		t.Fatalf("default/all specs: %+v", choices)
	}
	first := q(request, false, 200)
	if first.UnitPrice != "51.60" || first.LineTotal != "103.20" {
		t.Fatalf("actual target BOM: %+v", first)
	}
	if second := q(request, true, 200); second.UnitPrice != first.UnitPrice || second.QuoteRevision != first.QuoteRevision {
		t.Fatal("ERP/mini differ")
	}
	next := request
	next.PriceRowKey = orderbeans.PublishedPriceRowKey(f.publication, "commercial", pid, source, sourceV, "flat", 1)
	if got := q(next, false, 200); got.UnitPrice != "47.30" {
		t.Fatalf("selected template params: %+v", got)
	}
	next.TargetSpecID = small
	next.TargetVariantID = smallV
	if got := q(next, true, 200); got.UnitPrice != "23.10" {
		t.Fatalf("small BOM: %+v", got)
	}
	next = request
	next.TargetSpecID = source
	next.TargetVariantID = sourceV
	if got := q(next, false, 200); got.UnitPrice != "10.80" {
		t.Fatal("published price changed")
	}
	f.call("POST", "/api/mini/employee/spec-conversions/quote", request, f.visitorToken, 403)
	for _, change := range []func(*salesapp.SpecConversionQuoteRequest){func(q *salesapp.SpecConversionQuoteRequest) { q.PriceRowKey = "forged" }, func(q *salesapp.SpecConversionQuoteRequest) { q.ProductID = f.productIDs[1] }, func(q *salesapp.SpecConversionQuoteRequest) { q.TargetVariantID = sourceV }, func(q *salesapp.SpecConversionQuoteRequest) { q.Quantity = 0 }} {
		bad := request
		change(&bad)
		q(bad, false, 400)
	}
	// Three order lines independently keep ordinary, first-tier and second-tier identities.
	ordinary := map[string]any{"product_id": pid, "bom_spec_id": source, "bom_variant_id": sourceV, "qty": 1, "unit": "袋", "name": "测试商品", "bean_list_publication_id": f.publication, "price_selection_mode": "tier", "selected_price_row_key": key}
	converted := func(req salesapp.SpecConversionQuoteRequest) map[string]any {
		quote := q(req, true, 200)
		return map[string]any{"product_id": pid, "bom_spec_id": req.TargetSpecID, "bom_variant_id": req.TargetVariantID, "qty": 2, "unit": "袋", "name": "测试商品", "unit_price": 0.01, "bean_list_publication_id": f.publication, "price_selection_mode": "bom_tier", "selected_price_row_key": req.PriceRowKey, "spec_conversion": quote.SpecConversionSelection}
	}
	mixedSmall := request
	mixedSmall.TargetSpecID, mixedSmall.TargetVariantID = small, smallV
	mixedSmall.PriceRowKey = orderbeans.PublishedPriceRowKey(f.publication, "commercial", pid, source, sourceV, "flat", 1)
	mixed := f.call("POST", "/api/mini/employee/orders", map[string]any{"order_date": "2026-10-17", "customer_id": customer, "pay_status_id": f.id("SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1"), "ship_status_id": f.id("SELECT id FROM %s.ship_statuses WHERE name='未发货' LIMIT 1"), "selected_price_table_ids": []int64{f.publication}, "items": []any{ordinary, converted(request), converted(mixedSmall)}}, f.salesToken, 201)
	var mixedID int64
	_ = json.Unmarshal(mixed["order_id"], &mixedID)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND ((bom_spec_id=$2 AND unit_price=10.8) OR (bom_spec_id=$3 AND unit_price=51.6) OR (bom_spec_id=$4 AND unit_price=23.1))", mixedID, source, target, small); n != 3 {
		t.Fatal("mixed line price/spec identity changed")
	}
	f.exec("UPDATE %s.bean_list_publications SET owner_type='customer',owner_key='999999' WHERE id=$1", f.publication)
	q(request, false, 400)
	q(request, true, 400)
	f.exec("UPDATE %s.bean_list_publications SET owner_type='official',owner_key='' WHERE id=$1", f.publication)
	// Changing a global template does not alter the published rule snapshot.
	f.exec("UPDATE %s.product_pricing_rules SET margin_rate=9 WHERE id=$1", ruleID)
	if got := q(request, false, 200); got.UnitPrice != "51.60" {
		t.Fatal("mutable template replaced published rule")
	}
	selected, _ := json.Marshal(first.SpecConversionSelection)
	payload := map[string]any{"pay_status_id": f.id("SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1"), "ship_status_id": f.id("SELECT id FROM %s.ship_statuses WHERE name='未发货' LIMIT 1"), "order_date": "2026-10-17", "customer_id": customer, "product_id": []string{fmt.Sprint(pid)}, "bom_spec_id": []string{fmt.Sprint(target)}, "bom_variant_id": []string{fmt.Sprint(targetV)}, "qty": []string{"2"}, "item_name": []string{"测试商品"}, "unit_price": []string{"0.01"}, "price_selection_mode": []string{"bom_tier"}, "selected_price_row_key": []string{key}, "spec_conversion": []string{string(selected)}, "item_bean_list_publication_id": []string{fmt.Sprint(f.publication)}, "selected_price_table_ids": []int64{f.publication}}
	saved := f.call("POST", "/api/order", payload, "basic-admin", 200)
	var orderID int64
	_ = json.Unmarshal(saved["order_id"], &orderID)
	itemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1", orderID)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND bom_spec_id=$2 AND unit_price=51.60 AND line_total=103.20", orderID, target); n != 1 {
		t.Fatal("save trusted client price or wrong target")
	}
	miniItem := map[string]any{"product_id": pid, "bom_spec_id": target, "bom_variant_id": targetV, "qty": 2, "unit": "袋", "name": "测试商品", "unit_price": 0.01, "bean_list_publication_id": f.publication, "price_selection_mode": "bom_tier", "selected_price_row_key": key, "spec_conversion": first.SpecConversionSelection}
	miniPayload := map[string]any{"order_date": "2026-10-17", "customer_id": customer, "pay_status_id": payload["pay_status_id"], "ship_status_id": payload["ship_status_id"], "selected_price_table_ids": []int64{f.publication}, "items": []any{miniItem}}
	miniSaved := f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 201)
	var miniID int64
	_ = json.Unmarshal(miniSaved["order_id"], &miniID)
	miniItemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1", miniID)
	f.exec("UPDATE %s.materials SET purchase_price=100 WHERE id=$1", mat)
	stale := request
	stale.QuoteRevision = first.QuoteRevision
	q(stale, false, 409)
	detail := f.call("GET", fmt.Sprintf("/api/mini/employee/orders/%d", miniID), nil, f.salesToken, 200)
	var miniDetail map[string]any
	_ = json.Unmarshal(detail["order"], &miniDetail)
	miniPayload["edit_revision"] = miniDetail["edit_revision"]
	miniItem["item_id"] = miniItemID
	miniItem["qty"] = 3
	f.call("PUT", fmt.Sprintf("/api/mini/employee/orders/%d", miniID), miniPayload, f.salesToken, 200)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND unit_price=51.60 AND qty=3", miniID); n != 1 {
		t.Fatal("mini historical price changed")
	}
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 409)
	payload["edit_id"] = orderID
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["qty"] = []string{"3"}
	payload["price_source_json"] = []string{`{"spec_conversion":{"unit_price":"0.01"}}`}
	f.call("POST", "/api/order", payload, "basic-admin", 200)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND unit_price=51.60 AND line_total=154.80", orderID); n != 1 {
		t.Fatal("quantity did not retain trusted historical price")
	}
	delete(payload, "edit_id")
	delete(payload, "item_id")
	f.call("POST", "/api/order", payload, "basic-admin", 409)
	refreshed := q(request, true, 200)
	if refreshed.UnitPrice != "63.60" {
		t.Fatalf("current cost: %+v", refreshed)
	}
	refreshed.Reprice = true
	selected, _ = json.Marshal(refreshed.SpecConversionSelection)
	payload["spec_conversion"] = []string{string(selected)}
	payload["edit_id"], payload["item_id"] = orderID, []string{fmt.Sprint(itemID)}
	f.call("POST", "/api/order", payload, "basic-admin", 200)
	if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND unit_price=63.60", orderID); n != 1 {
		t.Fatal("explicit reprice did not use current cost")
	}
	delete(payload, "edit_id")
	delete(payload, "item_id")
	miniItem["spec_conversion"] = refreshed.SpecConversionSelection
	delete(miniItem, "item_id")
	f.call("POST", "/api/mini/employee/orders", miniPayload, f.salesToken, 201)
	preview := f.call("POST", "/api/order/stock-batch-preview", payload, "basic-admin", 200)
	previewJSON, _ := json.Marshal(preview)
	if !strings.Contains(string(previewJSON), fmt.Sprintf(`"bom_spec_id":%d`, target)) {
		t.Fatal("stock preview lost target spec", string(previewJSON))
	}
	f.call("POST", "/api/order", payload, "basic-admin", 200)
	var after string
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT content_json::text FROM %s.bean_list_publications WHERE id=$1", f.schema), f.publication).Scan(&after); err != nil {
		t.Fatal(err)
	}
	var normalized any
	_ = json.Unmarshal(content, &normalized)
	want, _ := json.Marshal(normalized)
	_ = json.Unmarshal([]byte(after), &normalized)
	got, _ := json.Marshal(normalized)
	if string(want) != string(got) {
		t.Fatal("quotes mutated source price table")
	}
	if n := f.id("SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1", orderID); n == 0 {
		t.Fatal("missing order audit")
	}
	// Missing / manually adjusted source templates never silently fall back.
	rows[0]["manual_adjusted"] = true
	raw, _ := json.Marshal(map[string]any{"price_rows": rows})
	f.exec("UPDATE %s.bean_list_publications SET content_json=$1 WHERE id=$2", raw, f.publication)
	if msg := f.call("POST", "/api/order/spec-conversions/quote", request, "basic-admin", 400); !strings.Contains(string(msg["error"]), "手工") {
		t.Fatal(msg)
	}
	rows[0]["manual_adjusted"] = false
	delete(rows[0], "pricing_rule_snapshot")
	raw, _ = json.Marshal(map[string]any{"price_rows": rows})
	f.exec("UPDATE %s.bean_list_publications SET content_json=$1 WHERE id=$2", raw, f.publication)
	q(request, false, 400)
	f.exec("UPDATE %s.production_boms SET status='inactive' WHERE id=$1", bom.ID)
	inactive := f.call("GET", fmt.Sprintf("/api/order/spec-conversions?customer_id=%d&product_id=%d&publication_id=%d", customer, pid, f.publication), nil, "basic-admin", 200)
	_ = json.Unmarshal(inactive["specs"], &choices)
	if len(choices) != 0 {
		t.Fatal("inactive BOM exposed as orderable")
	}

}
