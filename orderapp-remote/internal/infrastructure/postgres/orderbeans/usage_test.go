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

func TestListTypeForProductKindUsesGreenBeanList(t *testing.T) {
	if got := ListTypeForProductKind("green_bean", false); got != ListTypeGreen {
		t.Fatalf("green bean list type = %q, want %q", got, ListTypeGreen)
	}
	if got := ListTypeForProductKind("roasted", true); got != ListTypeRetail {
		t.Fatalf("retail roasted list type = %q, want %q", got, ListTypeRetail)
	}
}
