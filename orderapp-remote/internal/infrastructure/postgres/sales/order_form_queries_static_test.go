package sales

import (
	"encoding/json"
	salesapp "orderapp/internal/application/sales"
	"os"
	"strings"
	"testing"
)

func TestOrderFormProductQueryKeepsRoastLevelAndProductKindScanShape(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "SELECT p.id, p.name, COALESCE(p.roast_level,'')") {
		t.Fatalf("order form product query must select roast_level as the third product column")
	}
	if strings.Contains(text, "SELECT id, name, COALESCE(NULLIF(product_kind,''),'roasted')") {
		t.Fatalf("order form product query must not scan product_kind into ProductOption.RoastLevel")
	}
}

func TestOrderFormProductsUsePublishedPriceSnapshotsOnly(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"p.default_price",
		"COALESCE(p.retail_price_227g, p.default_price, 0)",
		"FROM %[1]s.product_price_tiers",
		"direct_tiers",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("order form products must not expose legacy product price fallback %q", forbidden)
		}
	}
	for _, want := range []string{
		"bean_list_publications",
		"source_price_record_id",
		"inventory_conversion_json",
		"published_price_snapshot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form products must expose published price snapshot metadata; missing %q", want)
		}
	}
}

func TestOrderCommercialProductKindIncludesDerivedDripSKUs(t *testing.T) {
	if !orderCommercialProductKind("drip_bag") {
		t.Fatal("derived drip SKUs must load commercial publication tiers")
	}
	if orderCommercialProductKind("green_bean") {
		t.Fatal("green beans must keep the green publication path")
	}
}

func TestCommercialOrderTierOptionKeepsDerivedDripSalesUnit(t *testing.T) {
	tier := orderCommercialPublicationTier{
		Label: "100袋+", SpecG: 10, MinQty: 100, FinalUnitPrice: 3.08,
		DisplayUnit: "袋（10g）", PriceUnit: "袋（10g）", SalesUnit: "bag", UnitBagCount: 1, InventoryUnit: "袋",
		InventoryConversionJSON: json.RawMessage(`{"袋（10g）":{"袋":1}}`),
	}
	got := commercialOrderTierOption(99, "KMM-V1", 0, tier, "drip_bag")
	if got.UnitPrice != 3.08 || got.DisplayUnit != "袋（10g）" || got.ProductKind != "drip_bag" || got.SalesUnit != "bag" || got.UnitBagCount != 1 {
		t.Fatalf("derived drip order tier = %+v", got)
	}
}

func TestCommercialOrderTierMapKeepsConcreteSKUCountSemantics(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":550,
			"sku_id":551,
			"parent_product_id":550,
			"quantity_basis":"sales_spec_count",
			"effective_sales_spec":{"sku_id":551,"spec_name":"磅","sales_unit":"磅","net_content_qty":1,"net_content_unit":"lb"},
			"tier_quantity_unit":"磅",
			"tier_label":"2-4磅",
			"min_qty":2,
			"max_qty":4,
			"min_weight_g":1000,
			"final_unit_price":68,
			"price_unit":"磅"
		}]
	}`)

	tiers := commercialOrderTierMapFromPublicationContent(99, "V5.0.0", content)[551]
	if len(tiers) != 1 || tiers[0].MinQty != 2 || tiers[0].MaxQty == nil || *tiers[0].MaxQty != 4 {
		t.Fatalf("concrete SKU tiers = %+v, want unconverted 2-4 sales-spec counts", tiers)
	}
	if tiers[0].SpecG != 454 || tiers[0].DisplayUnit != "磅" {
		t.Fatalf("concrete SKU tier must derive 454g/磅 from frozen effective_sales_spec, got %+v", tiers[0])
	}
	if !strings.Contains(tiers[0].PriceSourceJSON, `"quantity_basis":"sales_spec_count"`) ||
		!strings.Contains(tiers[0].PriceSourceJSON, `"effective_sales_spec"`) {
		t.Fatalf("price source must freeze quantity/spec semantics: %s", tiers[0].PriceSourceJSON)
	}
	if len(commercialOrderTierMapFromPublicationContent(99, "V5.0.0", content)[550]) != 0 {
		t.Fatal("parent product must not receive concrete child SKU tiers")
	}
}

func TestCommercialOrderTierOptionDoesNotInvent454gForCountSnapshot(t *testing.T) {
	tier := orderCommercialPublicationTier{
		QuantityBasis: "sales_spec_count", FinalUnitPrice: 31,
		EffectiveSalesSpec: json.RawMessage(`{"sku_id":711,"spec_name":"盒","sales_unit":"盒"}`),
	}
	got := commercialOrderTierOption(99, "V5.0.1", 0, tier, "instant_coffee")
	if got.SpecG != 0 {
		t.Fatalf("count snapshot without authoritative net content must not default spec_g to 454, got %+v", got)
	}
	if got.DisplayUnit != "盒" || got.UnitPrice != 31 {
		t.Fatalf("count snapshot must use frozen effective sales unit and price, got %+v", got)
	}
}

func TestDripOrderBeanListCandidatesPreferCommercialAndKeepLegacyFallback(t *testing.T) {
	got := dripOrderBeanListCandidates(salesapp.SaveOrderCommand{
		CommercialBeanListPublicationID: 101,
		DripBeanListPublicationID:       202,
	}, 0, "")
	if len(got) != 2 || got[0].ListType != "commercial" || got[0].RequestedPublicationID != 101 || got[1].ListType != "drip" || got[1].RequestedPublicationID != 202 {
		t.Fatalf("drip publication candidates = %+v", got)
	}
}

func TestDripOrderBeanListCandidatesKeepExactItemSnapshotOnEdit(t *testing.T) {
	got := dripOrderBeanListCandidates(salesapp.SaveOrderCommand{
		CommercialBeanListPublicationID: 101,
		DripBeanListPublicationID:       202,
	}, 303, "drip")
	if len(got) != 1 || got[0].ListType != "drip" || got[0].RequestedPublicationID != 303 {
		t.Fatalf("exact legacy item publication candidate = %+v", got)
	}
	unknown := dripOrderBeanListCandidates(salesapp.SaveOrderCommand{}, 404, "")
	if len(unknown) != 2 || unknown[0].ListType != "commercial" || unknown[0].RequestedPublicationID != 404 || unknown[1].ListType != "drip" || unknown[1].RequestedPublicationID != 404 {
		t.Fatalf("unknown item publication candidates = %+v", unknown)
	}
}

func TestOrderBeanListTypeFromPriceSourceKeepsLegacyDripOnEdit(t *testing.T) {
	if got := orderBeanListTypeFromPriceSource(`{"list_type":"drip"}`); got != "drip" {
		t.Fatalf("legacy drip source type = %q", got)
	}
}

func TestOrderFormProductsHideTemplateRemovedDerivedSKUs(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"COALESCE(p.auto_derived_sku,false)",
		"derived_spec_status",
		"template_removed",
		"COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form product candidates must hide template-removed derived SKUs; missing %q", want)
		}
	}
}

func TestOrderFormDerivedSKUsUseNetContentUnitConversion(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"derived_sku_unit_factor",
		"p.net_content_qty",
		"p.net_content_unit",
		"/ 1000.0",
	} {
		if count := strings.Count(text, want); count < 2 {
			t.Fatalf("product and customer-alias order form queries must convert derived SKU net content; %q count = %d, want at least 2", want, count)
		}
	}
	if strings.Contains(text, "jsonb_build_object(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'), 1)") {
		t.Fatalf("order form derived SKU conversion must not hard-code one parent inventory unit per sales unit")
	}
}

func TestOrderSaveRequiresPublishedPriceSnapshotInsteadOfLegacyTierFallback(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"loadOrderUnitPriceTiersTx(ctx, tx, r.schema",
		"FROM %s.product_price_tiers",
		"resolveAutoWeightTierPriceTx(",
		"COALESCE(NULLIF(retail_price_227g,0), default_price, 0)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("order save must not use legacy product price fallback %q", forbidden)
		}
	}
	for _, want := range []string{
		"缺少商品价格表价格",
		"缺少挂耳价格表价格",
		"beanListPriceSourceJSONWithPricing",
		"source_price_record_id",
		"inventory_conversion_json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order save must require published price snapshot metadata; missing %q", want)
		}
	}
}

func TestOrderFormProductsExposeProductTypeAndUnitRuleFields(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"LEFT JOIN %[1]s.product_categories subtype_cat",
		"product_type_category_id",
		"product_subtype_category_id",
		"NULLIF(p.unit_rule_override_json->>'inventory_unit','')",
		"product_direct_unit_template",
		"NULLIF(product_direct_unit_template.inventory_unit,'')",
		"NULLIF(product_direct_unit_template.quote_unit,'')",
		"NULLIF(product_direct_unit_template.unit_conversion_json::text,'{}')",
		"NULLIF(p.unit_rule_override_json->>'default_sales_unit','')",
		"NULLIF(p.unit_rule_override_json->>'unit_conversion_json','')",
		"unit_conversion_json",
		"&p.ProductTypeCategoryID",
		"&p.ProductSubtypeCategoryID",
		"&p.UnitConversionJSON",
		"&p.IntegerUnit",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form products must expose product type/subtype and unit rule fields; missing %q", want)
		}
	}
	if strings.Contains(text, "NULLIF(p.unit_rule_override_json, '{}'::jsonb)") {
		t.Fatalf("order form products must expose unit_conversion_json, not the whole unit_rule_override_json object")
	}
}

func TestOrderFormProductQueryDoesNotExposeBoundRoastedTiersForGreenBeanProducts(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"green_bound_tiers",
		"green_bean_bound_roasted_tier",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("order form product tiers must not expose bound roasted tiers for green bean products; found %q", forbidden)
		}
	}
}

func TestOrderSaveRejectsMissingGreenBeanListPriceWithoutBoundRoastedFallback(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"greenBeanOrderPriceProductIDTx",
		"greenBeanBoundRoastedTierPriceSourceJSON",
		"green_bean_bound_roasted_tier",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("green bean order save must not fall back to bound roasted tiers; found %q", forbidden)
		}
	}
	if !strings.Contains(text, "missing green bean list price") && !strings.Contains(text, "缺少生豆豆单价格") {
		t.Fatalf("green bean order save must return an explicit missing green bean list price error")
	}
}

func TestOrderFormBeanListVersionOptionsArePartitionedByListType(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"b.list_type",
		"PARTITION BY c.id, b.list_type",
		"&row.ListType",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form bean-list versions must be grouped by customer and list type; missing %q", want)
		}
	}
}

func TestOrderFormBeanListVersionOptionsExposeProductTypeFields(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"COALESCE(b.product_type_category_id,0)",
		"COALESCE(b.product_type_name,'')",
		"&row.ProductTypeCategoryID",
		"&row.ProductTypeName",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form product price list versions must expose product type fields; missing %q", want)
		}
	}
}

func TestOrderFormBeanListVersionOptionsUseOnlyPublishedSnapshots(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "b.status IN ('published','withdrawn')") {
		t.Fatalf("order form bean-list version options must not include withdrawn snapshots")
	}
	if !strings.Contains(text, "b.status='published'") {
		t.Fatalf("order form bean-list version options must select only published snapshots")
	}
}

func TestOrderFormBeanListVersionOptionsIncludeGlobalPublicFallback(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"global_public_versions AS",
		"0::bigint AS customer_id",
		"FROM global_public_versions",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form bean-list version options must include global public fallback rows for no-customer entry; missing %q", want)
		}
	}
}

func TestOrderSaveExplicitBeanListPublicationRequiresPublishedSnapshot(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	if strings.Contains(string(source), "status IN ('published','withdrawn')") {
		t.Fatalf("explicit order bean-list publication selection must reject withdrawn snapshots")
	}
	if !strings.Contains(string(source), "status='published'") {
		t.Fatalf("explicit order bean-list publication selection must require a published snapshot")
	}
}

func TestGreenBeanOrderPublicationTiersParseTemplatePrices(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":416,
				"name":"曲奇拼配2.0",
				"green_bean_sale_tiers":[
					{"label":"1KG","spec_g":1000,"min_qty":1,"max_qty":59,"price_per_unit":63.9,"price_per_lb":28.99,"template_id":5,"template_tier_id":50,"display_unit":"kg"},
					{"label":"60kG","spec_g":1000,"min_qty":60,"price_per_unit":63.9,"price_per_lb":28.99,"template_id":5,"template_tier_id":51,"display_unit":"kg"}
				]
			}]
		}]
	}`)

	tiersByProduct := greenBeanOrderTierMapFromPublicationContent(12, "V3.0.5", content)
	tiers := tiersByProduct[416]
	if len(tiers) != 2 {
		t.Fatalf("green bean tiers = %+v, want 2 tiers", tiers)
	}
	if tiers[0].SpecG != 1000 || tiers[0].MinQty != 1 || tiers[0].MaxQty == nil || *tiers[0].MaxQty != 59 || tiers[0].UnitPrice != 63.9 {
		t.Fatalf("first green bean tier = %+v", tiers[0])
	}
	if tiers[0].ProductKind != "green_bean" {
		t.Fatalf("product_kind = %q, want green_bean", tiers[0].ProductKind)
	}
	if tiers[0].DisplayUnit != "kg" || tiers[1].DisplayUnit != "kg" {
		t.Fatalf("display_unit = %q/%q, want kg/kg", tiers[0].DisplayUnit, tiers[1].DisplayUnit)
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(tiers[0].PriceSourceJSON), &source); err != nil {
		t.Fatalf("price source json invalid: %v", err)
	}
	if source["list_type"] != "green" || int64(source["publication_id"].(float64)) != 12 || int64(source["template_id"].(float64)) != 5 {
		t.Fatalf("price source = %+v", source)
	}
}

func TestApplyGreenBeanOrderPublicationTiersReplacesDirectProductTiers(t *testing.T) {
	products := []salesapp.ProductOption{
		{
			ID:          416,
			Name:        "曲奇拼配2.0",
			ProductKind: "green_bean",
			Tiers: []salesapp.ProductTierOption{{
				ID:          460,
				SpecG:       454,
				MinQty:      2,
				UnitPrice:   63,
				ProductKind: "roasted_bean",
			}},
		},
		{
			ID:          417,
			Name:        "曲奇拼配2.0",
			ProductKind: "roasted",
			Tiers: []salesapp.ProductTierOption{{
				ID:          461,
				SpecG:       454,
				MinQty:      14,
				UnitPrice:   56,
				ProductKind: "roasted_bean",
			}},
		},
	}
	publicationTiers := map[orderPublicationProductKey][]salesapp.ProductTierOption{
		{ProductID: 416}: {{
			ID:          50,
			SpecG:       1000,
			MinQty:      1,
			UnitPrice:   63.9,
			ProductKind: "green_bean",
		}},
	}

	applyGreenBeanOrderPublicationTiers(products, publicationTiers)

	if len(products[0].Tiers) != 1 || products[0].Tiers[0].SpecG != 1000 || products[0].Tiers[0].UnitPrice != 63.9 || products[0].Tiers[0].ProductKind != "green_bean" {
		t.Fatalf("green bean product tiers = %+v", products[0].Tiers)
	}
	if len(products[1].Tiers) != 1 || products[1].Tiers[0].ID != 461 || products[1].Tiers[0].UnitPrice != 56 {
		t.Fatalf("roasted product tiers changed = %+v", products[1].Tiers)
	}
}

func TestCommercialOrderPublicationTiersParseTemplatePrices(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":425,
				"name":"芬纳定制-红酒日晒-中深烘",
				"commercial_wholesale_tiers":[
					{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":65,"price_per_lb":65,"template_id":6,"template_tier_id":56,"display_unit":"lb"},
					{"label":"14-23磅","spec_g":454,"min_qty":14,"max_qty":23,"price_per_unit":59,"price_per_lb":59,"template_id":6,"template_tier_id":57,"display_unit":"lb"}
				]
			}]
		}]
	}`)

	tiersByProduct := commercialOrderTierMapFromPublicationContent(31, "V3.0.6", content)
	tiers := tiersByProduct[425]
	if len(tiers) != 2 {
		t.Fatalf("commercial tiers = %+v, want 2 tiers", tiers)
	}
	if tiers[0].ID != 56 || tiers[0].SpecG != 454 || tiers[0].MinQty != 2 || tiers[0].MaxQty == nil || *tiers[0].MaxQty != 13 || tiers[0].UnitPrice != 65 {
		t.Fatalf("first commercial tier = %+v", tiers[0])
	}
	if tiers[0].ProductKind != "roasted_bean" || tiers[0].DisplayUnit != "lb" {
		t.Fatalf("first commercial tier kind/unit = %q/%q", tiers[0].ProductKind, tiers[0].DisplayUnit)
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(tiers[0].PriceSourceJSON), &source); err != nil {
		t.Fatalf("price source json invalid: %v", err)
	}
	if source["list_type"] != "commercial" || int64(source["publication_id"].(float64)) != 31 || int64(source["template_id"].(float64)) != 6 {
		t.Fatalf("price source = %+v", source)
	}
}

func TestCommercialOrderPublicationTiersPreserveProductKindAndCustomUnit(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"product_id":433,
				"name":"Codex测试速溶盒装 10条/盒",
				"product_kind":"instant_coffee",
				"commercial_wholesale_tiers":[
					{"label":"10盒起","spec_g":100,"min_qty":10,"max_qty":99,"price_per_unit":15,"price_per_kg":150,"template_id":11,"template_tier_id":77,"display_unit":"盒","price_unit":"盒"}
				]
			}]
		}]
	}`)

	tiersByProduct := commercialOrderTierMapFromPublicationContent(47, "CODX-速溶盒装-20260525", content)
	tiers := tiersByProduct[433]
	if len(tiers) != 1 {
		t.Fatalf("commercial custom tiers = %+v, want 1 tier", tiers)
	}
	if tiers[0].ProductKind != "instant_coffee" || tiers[0].DisplayUnit != "盒" || tiers[0].UnitPrice != 15 {
		t.Fatalf("commercial custom tier kind/unit/price = %+v", tiers[0])
	}
}

func TestCommercialOrderPublicationTiersParseFlatRowsByDerivedSKUID(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":500,
			"sku_id":711,
			"product_kind":"drip_bag",
			"commercial_wholesale_tiers":[{"spec_g":100,"min_qty":10,"max_qty":99,"price_per_unit":31,"sales_unit":"box","unit_bag_count":10}]
		}]}],
		"price_rows":[{
			"product_id":500,
			"sku_id":711,
			"parent_product_id":500,
			"product_kind":"drip_bag",
			"tier_label":"10盒+",
			"spec_g":100,
			"min_qty":10,
			"max_qty":99,
			"final_unit_price":32.8,
			"price_unit":"box",
			"sales_unit":"box",
			"unit_bag_count":10
		}]
	}`)

	tiersByProduct := commercialOrderTierMapFromPublicationContent(48, "KMM-FLAT-V1", content)
	if _, covered := tiersByProduct[500]; covered {
		t.Fatalf("parent product_id 500 must not own derived SKU price rows: %+v", tiersByProduct[500])
	}
	tiers := tiersByProduct[711]
	if len(tiers) != 1 || tiers[0].UnitPrice != 32.8 || tiers[0].SalesUnit != "box" || tiers[0].UnitBagCount != 10 {
		t.Fatalf("derived SKU flat tiers = %+v, want sku_id 711 box price", tiers)
	}
}

func TestRetailOrderPublicationTiersReuseConcreteSKUCountSnapshots(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":550,
			"sku_id":551,
			"parent_product_id":550,
			"quantity_basis":"sales_spec_count",
			"effective_sales_spec":{"sku_id":551,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"},
			"tier_quantity_unit":"227g袋装",
			"tier_label":"2袋+",
			"min_qty":2,
			"final_unit_price":68,
			"price_unit":"袋"
		}]
	}`)

	tiers := commercialOrderTierMapFromPublicationContent(88, "RETAIL-V1", content, "retail")[551]
	if len(tiers) != 1 {
		t.Fatalf("retail concrete SKU tiers = %+v, want one tier", tiers)
	}
	got := tiers[0]
	if got.SpecG != 227 || got.UnitPrice != 68 || got.QuantityBasis != "sales_spec_count" || got.DisplayUnit != "袋" {
		t.Fatalf("retail concrete SKU count tier = %+v", got)
	}
	if got.EffectiveSalesSpec["spec_key"] != "bag-227g" || got.EffectiveSalesSpec["sales_unit"] != "袋" {
		t.Fatalf("retail effective sales spec = %#v", got.EffectiveSalesSpec)
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(got.PriceSourceJSON), &source); err != nil {
		t.Fatalf("decode retail price source: %v", err)
	}
	if source["list_type"] != "retail" || int64(source["publication_id"].(float64)) != 88 {
		t.Fatalf("retail price source = %#v", source)
	}
}

func TestRetailOrderPublicationTiersKeepLegacyRetailBeanTiers(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":551,
			"product_kind":"roasted_bean",
			"retail_bean_tiers":[{"label":"227g","spec_g":227,"min_qty":1,"price_per_unit":82,"display_unit":"g227"}]
		}]}]
	}`)

	tiers := commercialOrderTierMapFromPublicationContent(89, "RETAIL-LEGACY", content, "retail")[551]
	if len(tiers) != 1 || tiers[0].SpecG != 227 || tiers[0].UnitPrice != 82 {
		t.Fatalf("legacy retail tiers = %+v", tiers)
	}
	if !strings.Contains(tiers[0].PriceSourceJSON, `"list_type":"retail"`) {
		t.Fatalf("legacy retail source = %s", tiers[0].PriceSourceJSON)
	}
}

func TestApplyRetailOrderPublicationTiersAppendsWithoutReplacingCommercial(t *testing.T) {
	products := []salesapp.ProductOption{{
		ID: 551, ProductKind: "roasted_bean",
		Tiers: []salesapp.ProductTierOption{{
			ID: 1, UnitPrice: 75, PriceSourceJSON: `{"list_type":"commercial","publication_id":77}`,
		}},
	}}
	retail := map[orderPublicationProductKey][]salesapp.ProductTierOption{
		{ProductID: 551}: {{
			ID: 2, UnitPrice: 68, PriceSourceJSON: `{"list_type":"retail","publication_id":88}`,
		}},
	}

	applyRetailOrderPublicationTiers(products, retail)

	if len(products[0].Tiers) != 2 || products[0].Tiers[0].UnitPrice != 75 || products[0].Tiers[1].UnitPrice != 68 {
		t.Fatalf("commercial + retail tier merge = %+v", products[0].Tiers)
	}
}

func TestCommercialOrderPublicationFlatRowsConvertWeightBoundsToSKUQuantities(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":712,
			"sku_id":712,
			"spec_g":227,
			"min_qty":1,
			"max_qty":6,
			"min_weight_g":1000,
			"max_weight_g":6999.999,
			"final_unit_price":20,
			"price_unit":"g227"
		}]
	}`)

	tiers := commercialOrderTierMapFromPublicationContent(49, "KMM-WEIGHT-V1", content)[712]
	if len(tiers) != 1 || tiers[0].MinQty <= 4 || tiers[0].MinQty > 5 || tiers[0].MaxQty == nil || *tiers[0].MaxQty < 30 {
		t.Fatalf("flat weight bounds were not converted to 227g SKU quantities: %+v", tiers)
	}
}

func TestApplyCommercialOrderPublicationTiersReplacesCustomerRoastedTiers(t *testing.T) {
	products := []salesapp.ProductOption{
		{
			ID:          425,
			Name:        "芬纳定制-红酒日晒-中深烘",
			ProductKind: "roasted",
			Tiers: []salesapp.ProductTierOption{{
				ID:          990,
				SpecG:       454,
				MinQty:      1,
				UnitPrice:   88,
				ProductKind: "roasted_bean",
			}},
		},
		{
			ID:          426,
			Name:        "芬纳曲奇定制",
			ProductKind: "green_bean",
			Tiers: []salesapp.ProductTierOption{{
				ID:          991,
				SpecG:       1000,
				MinQty:      1,
				UnitPrice:   62,
				ProductKind: "green_bean",
			}},
		},
	}
	publicationTiers := map[orderPublicationProductKey][]salesapp.ProductTierOption{
		{ProductID: 425}: {{
			ID:          56,
			SpecG:       454,
			MinQty:      2,
			UnitPrice:   65,
			ProductKind: "roasted_bean",
		}},
	}

	applyCommercialOrderPublicationTiers(products, publicationTiers)

	if len(products[0].Tiers) != 1 || products[0].Tiers[0].ID != 56 || products[0].Tiers[0].UnitPrice != 65 {
		t.Fatalf("roasted product tiers = %+v", products[0].Tiers)
	}
	if len(products[1].Tiers) != 1 || products[1].Tiers[0].ID != 991 || products[1].Tiers[0].UnitPrice != 62 {
		t.Fatalf("green product tiers changed = %+v", products[1].Tiers)
	}
}

func TestApplyCommercialOrderPublicationTiersKeepsCustomerAliasSnapshotsSeparate(t *testing.T) {
	products := []salesapp.ProductOption{
		{
			ID:                     538,
			CustomerID:             19,
			CustomerProductAliasID: 82,
			ProductKind:            "roasted_bean",
		},
		{
			ID:                     538,
			CustomerID:             122,
			CustomerProductAliasID: 83,
			ProductKind:            "roasted_bean",
		},
	}
	publicationTiers := map[orderPublicationProductKey][]salesapp.ProductTierOption{
		{CustomerID: 19, ProductID: 538}: {{
			ID:              5600001,
			UnitPrice:       88.5,
			PriceSourceJSON: `{"publication_id":56,"list_type":"commercial"}`,
		}},
		{CustomerID: 122, ProductID: 538}: {{
			ID:              5500001,
			UnitPrice:       88.5,
			PriceSourceJSON: `{"publication_id":55,"list_type":"commercial"}`,
		}},
	}

	applyCommercialOrderPublicationTiers(products, publicationTiers)

	for idx, wantPublicationID := range []int64{56, 55} {
		if len(products[idx].Tiers) != 1 {
			t.Fatalf("product[%d] tiers = %+v, want one customer-specific tier", idx, products[idx].Tiers)
		}
		var source struct {
			PublicationID int64 `json:"publication_id"`
		}
		if err := json.Unmarshal([]byte(products[idx].Tiers[0].PriceSourceJSON), &source); err != nil {
			t.Fatalf("product[%d] price source json invalid: %v", idx, err)
		}
		if source.PublicationID != wantPublicationID {
			t.Fatalf("product[%d] publication_id=%d, want %d", idx, source.PublicationID, wantPublicationID)
		}
	}
}

func TestMergeOrderPublicationTierMapsKeepsMultiplePublishedVersions(t *testing.T) {
	first := map[int64][]salesapp.ProductTierOption{
		7: {{
			ID:              62,
			UnitPrice:       61,
			PriceSourceJSON: `{"publication_id":9902,"list_type":"commercial"}`,
		}},
	}
	second := map[int64][]salesapp.ProductTierOption{
		7: {{
			ID:              63,
			UnitPrice:       64,
			PriceSourceJSON: `{"publication_id":9903,"list_type":"commercial"}`,
		}},
	}

	merged := mergeOrderPublicationTierMaps(first, second)

	if len(merged[7]) != 2 {
		t.Fatalf("merged tiers = %+v, want both published versions", merged[7])
	}
	if merged[7][0].UnitPrice != 61 || merged[7][1].UnitPrice != 64 {
		t.Fatalf("merged tier order/prices = %+v, want 61 then 64", merged[7])
	}
}

func TestMergeLatestCommercialOrderPublicationTierMapsUsesNewestSnapshotPerDerivedSKUProductID(t *testing.T) {
	newer := commercialOrderTierMapFromPublicationContent(9903, "V2", []byte(`{
		"groups":[{"items":[
			{"productId":7,"product_kind":"drip_bag","commercial_wholesale_tiers":[{"spec_g":100,"min_qty":1,"price_per_unit":64,"sales_unit":"box"}]},
			{"productId":9,"product_kind":"drip_bag","commercial_wholesale_tiers":[]}
		]}]
	}`))
	older := commercialOrderTierMapFromPublicationContent(9902, "V1", []byte(`{
		"groups":[{"items":[
			{"productId":7,"product_kind":"drip_bag","commercial_wholesale_tiers":[{"spec_g":100,"min_qty":1,"price_per_unit":61,"sales_unit":"box"}]},
			{"productId":8,"product_kind":"drip_bag","commercial_wholesale_tiers":[{"spec_g":10,"min_qty":1,"price_per_unit":80,"sales_unit":"bag"}]},
			{"productId":9,"product_kind":"drip_bag","commercial_wholesale_tiers":[{"spec_g":10,"min_qty":1,"price_per_unit":90,"sales_unit":"bag"}]}
		]}]
	}`))

	merged := mergeLatestCommercialOrderPublicationTierMaps(nil, newer)
	merged = mergeLatestCommercialOrderPublicationTierMaps(merged, older)

	if len(merged[7]) != 1 || merged[7][0].UnitPrice != 64 {
		t.Fatalf("same derived SKU product_id tiers = %+v, want newest publication price 64 only", merged[7])
	}
	var source struct {
		PublicationID int64 `json:"publication_id"`
	}
	if err := json.Unmarshal([]byte(merged[7][0].PriceSourceJSON), &source); err != nil {
		t.Fatalf("newest product price source invalid: %v", err)
	}
	if source.PublicationID != 9903 {
		t.Fatalf("same product publication_id=%d, want newest 9903", source.PublicationID)
	}
	if len(merged[8]) != 1 || merged[8][0].UnitPrice != 80 {
		t.Fatalf("historical derived SKU product_id coverage = %+v, want latest publication containing product 8", merged[8])
	}
	if tiers, covered := merged[9]; !covered || len(tiers) != 0 {
		t.Fatalf("newest blank product coverage = %+v/%v, want covered without falling back to old price", tiers, covered)
	}
}

func TestCommercialOrderPublicationQueryOrdersSnapshotsNewestFirstPerOwner(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (r Repository) fetchCommercialOrderPublicationTiers")
	end := strings.Index(text, "func applyCommercialOrderPublicationTiers")
	if start < 0 || end <= start {
		t.Fatal("commercial order publication query function not found")
	}
	functionSource := text[start:end]
	if !strings.Contains(functionSource, "ORDER BY owner_type, owner_key, published_at DESC, id DESC") {
		t.Fatal("commercial publications must be processed newest-first within each owner")
	}
	if strings.Contains(functionSource, "row_number() OVER") {
		t.Fatal("owner-level row numbers cannot represent latest publication coverage per product/SKU")
	}
	if count := strings.Count(functionSource, "publication_purpose='factory_supply'"); count != 2 {
		t.Fatalf("commercial order publications must only use factory_supply for customer and official owners; filter count=%d", count)
	}
}

func TestOrderFormBeanListVersionsOnlyExposeFactorySupplyPublications(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (r Repository) fetchOrderBeanListVersionOptions")
	end := strings.Index(text, "func (r Repository) fetchOrderCustomerPublicUsages")
	if start < 0 || end <= start {
		t.Fatal("order form bean-list version query function not found")
	}
	if count := strings.Count(text[start:end], "publication_purpose='factory_supply'"); count != 2 {
		t.Fatalf("order form version options must filter customer and official publications to factory_supply; filter count=%d", count)
	}
	if count := strings.Count(text[start:end], "'retail'"); count < 2 {
		t.Fatalf("order form version options must explicitly include retail publications; retail filter count=%d", count)
	}
}

func TestOrderFormProductQueryLoadsCommercialAndRetailPublications(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"fetchCommercialOrderPublicationTiers(ctx, out)",
		"fetchRetailOrderPublicationTiers(ctx, out)",
		"applyCommercialOrderPublicationTiers(out, commercialPublicationTiers)",
		"applyRetailOrderPublicationTiers(out, retailPublicationTiers)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form product query missing %q", want)
		}
	}
}

func TestGreenBeanOrderPublicationTiersOnlyUseFactorySupply(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (r Repository) fetchGreenBeanOrderPublicationTiers")
	end := strings.Index(text, "func mergeOrderPublicationTierMaps")
	if start < 0 || end <= start {
		t.Fatal("green bean order publication query function not found")
	}
	if count := strings.Count(text[start:end], "publication_purpose='factory_supply'"); count != 2 {
		t.Fatalf("green bean order publications must only use factory_supply for customer and official owners; filter count=%d", count)
	}
}

func TestOrderRepositoryRejectsNonPositiveManualPrices(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	for _, want := range []string{"*items[idx].manualPrice <= 0", "手动单价必须大于0"} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("order repository must reject zero and negative manual prices; missing %q", want)
		}
	}
}

func TestOrderBeanListPublicationResolverRejectsCustomerResalePurpose(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (r Repository) resolveOrderBeanListPublicationTx")
	end := strings.Index(text, "type requiredOrderCustomerProfile")
	if start < 0 || end <= start {
		t.Fatal("order bean-list publication resolver not found")
	}
	if count := strings.Count(text[start:end], "publication_purpose='factory_supply'"); count < 4 {
		t.Fatalf("order bean-list resolver must require factory_supply for explicit, fixed, customer and official publication selection; filter count=%d", count)
	}
}
