package catalog

import "testing"

func TestLegacyKindDefaultTypeNameMapsExistingProductKinds(t *testing.T) {
	cases := map[string]string{
		"roasted":        "熟豆",
		"roasted_bean":   "熟豆",
		"green_bean":     "生豆",
		"drip_bag":       "挂耳",
		"instant_coffee": "速溶咖啡",
		"":               "熟豆",
	}
	for input, want := range cases {
		if got := LegacyKindDefaultTypeName(input); got != want {
			t.Fatalf("LegacyKindDefaultTypeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProductCategoryRoleLabels(t *testing.T) {
	if got := ProductCategoryRoleLabel(1); got != "产品类型" {
		t.Fatalf("ProductCategoryRoleLabel(1) = %q, want 产品类型", got)
	}
	if got := ProductCategoryRoleLabel(2); got != "产品子类型" {
		t.Fatalf("ProductCategoryRoleLabel(2) = %q, want 产品子类型", got)
	}
	if got := ProductCategoryRoleLabel(3); got != "产品分类" {
		t.Fatalf("ProductCategoryRoleLabel(3) = %q, want 产品分类", got)
	}
}

func TestNormalizeProductUnitRuleDefaultsAndIntegerUnits(t *testing.T) {
	got := NormalizeProductUnitRule(ProductUnitRule{})
	if got.InventoryUnit != "kg" || got.QuoteUnit != "kg" || got.OrderUnit != "kg" {
		t.Fatalf("NormalizeProductUnitRule defaults = %+v, want kg units", got)
	}
	if got.ConversionJSON != "{}" {
		t.Fatalf("NormalizeProductUnitRule conversion json = %q, want {}", got.ConversionJSON)
	}
	if got.IntegerUnit {
		t.Fatalf("kg default rule should allow decimal quantities")
	}

	box := NormalizeProductUnitRule(ProductUnitRule{
		InventoryUnit:  "盒",
		QuoteUnit:      "盒",
		OrderUnit:      "盒",
		ConversionJSON: `{"盒":1}`,
		IntegerUnit:    true,
	})
	if box.InventoryUnit != "盒" || box.QuoteUnit != "盒" || box.OrderUnit != "盒" || !box.IntegerUnit {
		t.Fatalf("box rule = %+v, want integer box units", box)
	}
}
