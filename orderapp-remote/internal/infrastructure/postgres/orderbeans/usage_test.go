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

func TestPublishedPricingKeepsKgDisplayUnitForSmallCommercialPack(t *testing.T) {
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

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 80, 1, "", 0)
	if !ok {
		t.Fatalf("published pricing missing")
	}
	if got.UnitPrice != 82 || got.PriceUnit != "kg" || got.UnitG != 1000 {
		t.Fatalf("published pricing = %+v, want 82 kg/1000g", got)
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
