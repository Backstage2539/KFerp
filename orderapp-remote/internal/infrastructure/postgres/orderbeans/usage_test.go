package orderbeans

import "testing"

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

func TestListTypeForProductKindUsesGreenBeanList(t *testing.T) {
	if got := ListTypeForProductKind("green_bean", false); got != ListTypeGreen {
		t.Fatalf("green bean list type = %q, want %q", got, ListTypeGreen)
	}
	if got := ListTypeForProductKind("drip_bag", false); got != ListTypeDrip {
		t.Fatalf("drip list type = %q, want %q", got, ListTypeDrip)
	}
	if got := ListTypeForProductKind("roasted", true); got != ListTypeRetail {
		t.Fatalf("retail roasted list type = %q, want %q", got, ListTypeRetail)
	}
}
