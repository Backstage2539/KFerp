package orderbeans

import (
	"os"
	"strings"
	"testing"
)

func TestPublishedUnitPriceFromContentMatchesGreenBeanTiers(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":88,
				"name":"巴拿马生豆",
				"green_bean_sale_tiers":[
					{"label":"1-23kg","spec_g":1000,"min_qty":1,"max_qty":23,"price_per_unit":128,"price_per_lb":58.112,"display_unit":"kg"},
					{"label":"24kg+","spec_g":1000,"min_qty":24,"price_per_unit":118,"price_per_lb":53.572,"display_unit":"kg"}
				]
			}]
		}]
	}`)

	got, ok := publishedUnitPriceFromContent(content, 88, 1000, 2)
	if !ok || got != 128 {
		t.Fatalf("published price for 2kg = %.2f/%v, want 128/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContent(content, 88, 1000, 30)
	if !ok || got != 118 {
		t.Fatalf("published price for 30kg = %.2f/%v, want 118/true", got, ok)
	}
}

func TestInspectPublishedProductSpecRequiresConcreteSKUAndFreezesSalesSpec(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
			 "effective_sales_spec":{"sku_id":551,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}},
			{"product_id":550,"sku_id":552,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":118,
			 "effective_sales_spec":{"sku_id":552,"spec_key":"bag-454g","spec_name":"454g袋装","spec_label":"454g","sales_unit":"袋","net_content_qty":454,"net_content_unit":"g"}}
		]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect concrete publication: %v", err)
	}
	if !got.ConcretePublication || !got.ProductFound || got.SKUID != 551 || got.ParentProductID != 550 || got.SpecName != "227g袋装" || got.SpecLabel != "227g" || got.SalesUnit != "袋" || got.NetContentQty != 227 || got.NetContentUnit != "g" {
		t.Fatalf("concrete product spec = %+v", got)
	}
	missing, err := inspectPublishedProductSpecContent(content, 553)
	if err != nil {
		t.Fatalf("inspect missing SKU: %v", err)
	}
	if !missing.ConcretePublication || missing.ProductFound {
		t.Fatalf("missing concrete SKU = %+v, want concrete publication with product_found=false", missing)
	}
}

func TestInspectPublishedProductSpecRejectsFrozenSKUIdentityMismatch(t *testing.T) {
	content := []byte(`{
		"price_rows":[{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
		 "effective_sales_spec":{"sku_id":552,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]
	}`)

	if _, err := inspectPublishedProductSpecContent(content, 551); err == nil || !strings.Contains(err.Error(), "SKU") {
		t.Fatalf("frozen SKU mismatch err = %v, want SKU identity error", err)
	}
}

func TestInspectPublishedProductSpecKeepsLegacyPublicationCompatible(t *testing.T) {
	content := []byte(`{"groups":[{"items":[{"productId":7,"commercial_wholesale_tiers":[{"spec_g":454,"min_qty":2,"price_per_unit":68}]}]}]}`)

	got, err := inspectPublishedProductSpecContent(content, 7)
	if err != nil {
		t.Fatalf("inspect legacy publication: %v", err)
	}
	if got.ConcretePublication || !got.ProductFound {
		t.Fatalf("legacy publication identity = %+v", got)
	}
}

func TestInspectPublishedProductSpecMixedRowsRequireTargetConcreteSnapshot(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":550,"sku_id":551,"quantity_basis":"sales_spec_count","final_unit_price":68},
			{"product_id":550,"sku_id":552,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":118,
			 "effective_sales_spec":{"sku_id":552,"spec_name":"454g袋装","spec_label":"454g","sales_unit":"袋","net_content_qty":454,"net_content_unit":"g"}}
		]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect mixed publication: %v", err)
	}
	if !got.ConcretePublication || got.ProductFound {
		t.Fatalf("mixed publication target = %+v, want concrete publication with incomplete target rejected", got)
	}
}

func TestInspectPublishedProductSpecMixedPublicationKeepsLegacyProductCompatible(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{"productId":700,"name":"历史豆"}]}],
		"price_rows":[
			{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
			 "effective_sales_spec":{"sku_id":551,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}},
			{"product_id":700,"spec_g":454,"min_qty":1,"final_unit_price":66}
		]
	}`)

	legacy, err := inspectPublishedProductSpecContent(content, 700)
	if err != nil {
		t.Fatalf("inspect mixed publication legacy product: %v", err)
	}
	if legacy.ConcretePublication || !legacy.ProductFound {
		t.Fatalf("mixed publication legacy product = %+v, want legacy-compatible product", legacy)
	}

	concrete, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect mixed publication concrete product: %v", err)
	}
	if !concrete.ConcretePublication || !concrete.ProductFound || concrete.SKUID != 551 {
		t.Fatalf("mixed publication concrete product = %+v", concrete)
	}

	missing, err := inspectPublishedProductSpecContent(content, 999)
	if err != nil {
		t.Fatalf("inspect mixed publication missing product: %v", err)
	}
	if !missing.ConcretePublication || missing.ProductFound {
		t.Fatalf("mixed publication missing product = %+v, want strict not-found", missing)
	}
}

func TestPublishedUnitPriceFromContentMatchesCommercialAndDripTiers(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"1-9kg","spec_g":1000,"min_qty":1,"max_qty":9,"price_per_unit":80},
					{"label":"10kg+","spec_g":1000,"min_qty":10,"price_per_unit":75}
				]
			},{
				"productId":12,
				"drip_wholesale_tiers":[
					{"label":"100袋+","sales_unit":"bag","min_qty":100,"price_per_unit":3},
					{"label":"20盒+","sales_unit":"box","unit_bag_count":10,"min_qty":20,"price_per_unit":28}
				]
			}]
		}]
	}`)

	got, ok := publishedUnitPriceFromContentForListType(content, 11, ListTypeCommercial, 1000, 12, "", 0)
	if !ok || got != 75 {
		t.Fatalf("commercial published price = %.2f/%v, want 75/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 10, 120, "bag", 1)
	if !ok || got != 3 {
		t.Fatalf("drip bag published price = %.2f/%v, want 3/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 100, 25, "box", 10)
	if !ok || got != 28 {
		t.Fatalf("drip box published price = %.2f/%v, want 28/true", got, ok)
	}
}

func TestCommercialFlatRowsPriceDerivedDripSKUsAndKeepSalesUnits(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[
			{"productId":700,"name":"金色山脉 挂耳 袋（10g）"},
			{"productId":701,"name":"金色山脉 挂耳 盒（10袋）"}
		]}],
		"price_rows":[
			{"product_id":700,"tier_label":"100袋+","spec_g":10,"min_qty":100,"max_qty":999,"final_unit_price":3.08,"price_unit":"袋（10g）","inventory_unit":"袋","inventory_conversion_json":{"袋（10g）":{"袋":1}}},
			{"product_id":701,"tier_label":"10盒+","spec_g":100,"min_qty":10,"max_qty":99,"final_unit_price":32.8,"price_unit":"盒（10袋）","inventory_unit":"袋","inventory_conversion_json":{"盒（10袋）":{"袋":10}}}
		]
	}`)

	bag, ok := publishedPricingFromContentForListType(content, 700, ListTypeCommercial, 10, 120, "bag", 1)
	if !ok || bag.UnitPrice != 3.08 || bag.PriceUnit != "袋（10g）" || bag.InventoryUnit != "袋" {
		t.Fatalf("commercial derived drip bag pricing = %+v/%v", bag, ok)
	}
	box, ok := publishedPricingFromContentForListType(content, 701, ListTypeCommercial, 100, 20, "box", 10)
	if !ok || box.UnitPrice != 32.8 || box.PriceUnit != "盒（10袋）" || box.InventoryUnit != "袋" {
		t.Fatalf("commercial derived drip box pricing = %+v/%v", box, ok)
	}
}

func TestPublishedPricingKeepsKgDisplayUnitForSmallCommercialPackInsideTier(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25-49kg","spec_g":1000,"min_qty":25,"max_qty":49,"price_per_unit":82,"price_per_lb":37.23,"display_unit":"kg","price_unit":"kg"}
				]
			}]
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 80, 313, "", 0)
	if !ok {
		t.Fatalf("published pricing missing")
	}
	if got.UnitPrice != 82 || got.PriceUnit != "kg" || got.UnitG != 1000 {
		t.Fatalf("published pricing = %+v, want 82 kg/1000g", got)
	}
}

func TestPublishedCommercialPricingRejectsQuantityOutsideEveryTier(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25-49kg","spec_g":1000,"min_qty":25,"max_qty":49,"price_per_unit":82,"price_unit":"kg"},
					{"label":"51-60kg","spec_g":1000,"min_qty":51,"max_qty":60,"price_per_unit":78,"price_unit":"kg"}
				]
			}]
		}]
	}`)

	for _, qty := range []int64{1, 50, 61} {
		if got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, qty, "", 0); ok {
			t.Fatalf("commercial qty %d pricing = %+v/true, want explicit missing price", qty, got)
		}
	}
}

func TestPublishedCommercialFlatRowsRejectDerivedDripQuantityBelowTier(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":700,"tier_label":"100袋+","spec_g":10,"min_qty":100,"max_qty":999,"final_unit_price":3.08,"price_unit":"袋（10g）"},
			{"product_id":701,"tier_label":"10盒+","spec_g":100,"min_qty":10,"max_qty":99,"final_unit_price":32.8,"price_unit":"盒（10袋）"}
		]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 700, ListTypeCommercial, 10, 99, "bag", 1); ok {
		t.Fatalf("commercial drip bag below minimum = %+v/true, want missing price", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 701, ListTypeCommercial, 100, 9, "box", 10); ok {
		t.Fatalf("commercial drip box below minimum = %+v/true, want missing price", got)
	}
}

func TestPublishedCommercialFlatRowsDoNotBypassExplicitWeightBoundsWithUnitQuantity(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":710,"tier_label":"1-6kg","spec_g":227,"min_qty":1,"max_qty":6,"min_weight_g":1000,"max_weight_g":6999.999,"final_unit_price":20,"price_unit":"g227"}
		]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 710, ListTypeCommercial, 227, 1, "", 0); ok {
		t.Fatalf("227g one-pack price = %+v/true, want missing price below explicit 1kg bound", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 710, ListTypeCommercial, 227, 5, "", 0); !ok || got.UnitPrice != 20 {
		t.Fatalf("227g five-pack price = %+v/%v, want legal explicit-weight tier", got, ok)
	}
}

func TestPublishedCommercialPricingDoesNotBypassAnOutOfRangeExactSpecTier(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":711,
			"commercial_wholesale_tiers":[
				{"spec_g":227,"min_qty":1,"max_qty":10,"price_per_unit":20},
				{"spec_g":454,"min_qty":5,"max_qty":10,"price_per_unit":30}
			]
		}]}]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 454, 1, "", 0); ok {
		t.Fatalf("454g one-pack price = %+v/true, want missing price instead of falling through to the 227g tier", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 454, 5, "", 0); !ok || got.UnitPrice != 30 {
		t.Fatalf("454g five-pack price = %+v/%v, want legal exact-spec tier 30", got, ok)
	}
}

func TestPublishedLegacyDripPricingRequiresLegalBagAndBoxTier(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":12,
			"drip_wholesale_tiers":[
				{"label":"1-99袋","sales_unit":"bag","min_qty":1,"max_qty":99,"price_per_unit":4},
				{"label":"100-199袋","sales_unit":"bag","min_qty":100,"max_qty":199,"price_per_unit":3},
				{"label":"300袋+","sales_unit":"bag","min_qty":300,"price_per_unit":2.8},
				{"label":"20-29盒","sales_unit":"box","unit_bag_count":10,"min_qty":20,"max_qty":29,"price_per_unit":28}
			]
		}]}]
	}`)

	if got, ok := publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 10, 150, "bag", 1); !ok || got != 3 {
		t.Fatalf("legacy drip valid bag tier = %.2f/%v, want 3/true", got, ok)
	}
	if got, ok := publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 100, 25, "box", 10); !ok || got != 28 {
		t.Fatalf("legacy drip valid box tier = %.2f/%v, want 28/true", got, ok)
	}
	if got, ok := publishedPricingFromContentForListType(content, 12, ListTypeDrip, 10, 250, "bag", 1); ok {
		t.Fatalf("legacy drip bag gap = %+v/true, want missing price", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 12, ListTypeDrip, 100, 10, "box", 10); ok {
		t.Fatalf("legacy drip box below explicit box tier = %+v/true, want missing price", got)
	}
}

func TestPublishedPricingCarriesFinalPriceSnapshotMetadata(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25kg+","source_price_record_id":701,"spec_g":1000,"min_qty":25,"final_unit_price":82,"price_per_unit":82,"display_unit":"kg","price_unit":"kg","inventory_unit":"kg","inventory_conversion_json":{"kg":{"kg":1}}}
				]
			}]
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, 25, "", 0)
	if !ok {
		t.Fatalf("published pricing missing")
	}
	if got.SourcePriceRecordID != 701 || got.InventoryUnit != "kg" || !strings.Contains(got.InventoryConversionJSON, `"kg"`) {
		t.Fatalf("published pricing snapshot metadata = %+v", got)
	}
}

func TestPublishedPricingMatchesPR440FlatPriceRows(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"name":"PR440 平铺价格商品"
			}]
		}],
		"price_rows":[{
			"product_id":11,
			"tier_label":"1kg+",
			"min_qty":1,
			"final_unit_price":88,
			"price_unit":"kg",
			"inventory_unit":"kg",
			"inventory_conversion_json":{"kg":1},
			"pricing_rule_version":"PR440/v1",
			"customer_reference_snapshot":{"customer_id":3,"customer_display_name":"客户显示名"}
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, 1, "", 0)
	if !ok {
		t.Fatalf("PR-440 flat price row should resolve published pricing")
	}
	if got.UnitPrice != 88 || got.PriceUnit != "kg" || got.UnitG != 1000 || got.InventoryUnit != "kg" || !strings.Contains(got.InventoryConversionJSON, `"kg"`) {
		t.Fatalf("published PR-440 flat pricing = %+v, want 88 kg with inventory snapshot", got)
	}
}

func TestPublishedCommercialFlatPriceRowsPreferDerivedSKUIDentity(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":500,
			"sku_id":711,
			"commercial_wholesale_tiers":[{"spec_g":100,"min_qty":10,"max_qty":99,"price_per_unit":31}]
		}]}],
		"price_rows":[{
			"product_id":500,
			"sku_id":711,
			"parent_product_id":500,
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

	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 100, 20, "box", 10); !ok || got.UnitPrice != 32.8 {
		t.Fatalf("derived SKU flat price = %+v/%v, want sku_id 711 price 32.8", got, ok)
	}
	if got, ok := publishedPricingFromContentForListType(content, 500, ListTypeCommercial, 100, 20, "box", 10); ok {
		t.Fatalf("parent product must not consume derived SKU flat price: %+v", got)
	}
}

func TestPublishedCommercialFlatRowsUseConcreteSalesSpecCountAndKeepLegacyWeightFallback(t *testing.T) {
	newContent := []byte(`{
		"price_rows":[{
			"product_id":550,
			"sku_id":551,
			"parent_product_id":550,
			"quantity_basis":"sales_spec_count",
			"effective_sales_spec":{"sku_id":551,"spec_name":"磅","sales_unit":"磅","net_content_qty":1,"net_content_unit":"lb"},
			"tier_label":"2-4磅",
			"min_qty":2,
			"max_qty":4,
			"final_unit_price":68,
			"price_unit":"kg"
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(newContent, 551, ListTypeCommercial, 454, 2, "磅", 0)
	if !ok || got.UnitPrice != 68 {
		t.Fatalf("sales-spec-count pricing = %+v/%v, want two concrete pound SKUs to match", got, ok)
	}
	if got.QuantityBasis != "sales_spec_count" || got.TierQuantityUnit != "" || !strings.Contains(got.EffectiveSalesSpecJSON, `"net_content_unit":"lb"`) {
		t.Fatalf("sales-spec-count pricing must preserve frozen order semantics: %+v", got)
	}
	if _, ok := publishedPricingFromContentForListType(newContent, 550, ListTypeCommercial, 454, 2, "磅", 0); ok {
		t.Fatal("parent product must not consume its child SKU count tier")
	}

	legacyContent := []byte(`{
		"price_rows":[{
			"product_id":552,
			"tier_label":"1kg+",
			"spec_g":1000,
			"min_qty":1,
			"final_unit_price":82,
			"price_unit":"kg"
		}]
	}`)
	legacy, ok := publishedPricingFromContentForListType(legacyContent, 552, ListTypeCommercial, 454, 3, "磅", 0)
	if !ok || legacy.UnitPrice != 82 {
		t.Fatalf("legacy weight fallback = %+v/%v, want 3lb to keep matching the old 1kg tier", legacy, ok)
	}
}

func TestExplicitPublicationSelectionRequiresPublishedSnapshots(t *testing.T) {
	source, err := os.ReadFile("usage.go")
	if err != nil {
		t.Fatalf("read usage.go: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "blp.status IN ('published','withdrawn')") {
		t.Fatalf("explicit bean-list publication selection must reject withdrawn snapshots")
	}
	if !strings.Contains(text, "WHERE blp.status='published'") {
		t.Fatalf("bean-list publication selection must use published snapshots only")
	}
}

func TestPublishedPricingUsageOnlySelectsFactorySupplyPublications(t *testing.T) {
	source, err := os.ReadFile("usage.go")
	if err != nil {
		t.Fatalf("read usage.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func ResolveUsageForPublication")
	end := strings.Index(text, "type publishedBeanListContent")
	if start < 0 || end <= start {
		t.Fatal("published pricing usage resolver not found")
	}
	if count := strings.Count(text[start:end], "blp.publication_purpose='factory_supply'"); count != 2 {
		t.Fatalf("explicit and automatic order pricing usage must reject customer_resale; factory_supply filter count=%d", count)
	}
	for _, want := range []string{"content_json->'price_rows'", "row_json->>'sku_id'"} {
		if !strings.Contains(text[start:end], want) {
			t.Fatalf("automatic order pricing usage must recognize derived SKU flat rows; missing %q", want)
		}
	}
}

func TestListTypeForProductKindUsesGreenBeanList(t *testing.T) {
	if got := ListTypeForProductKind("green_bean", false); got != ListTypeGreen {
		t.Fatalf("green bean list type = %q, want %q", got, ListTypeGreen)
	}
	if got := ListTypeForProductKind("drip_bag", false); got != ListTypeDrip {
		t.Fatalf("shared portal-compatible drip list type = %q, want %q", got, ListTypeDrip)
	}
	if got := ListTypeForProductKind("roasted", true); got != ListTypeRetail {
		t.Fatalf("retail roasted list type = %q, want %q", got, ListTypeRetail)
	}
}
