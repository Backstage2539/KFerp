package catalog

import "testing"

func TestResolveProductRuleConfigMergesByPriority(t *testing.T) {
	got := ResolveProductRuleConfig(ProductRuleResolutionInput{
		SystemFallback: ProductRuleConfig{
			GradientTemplateID:  1,
			OperationTemplateID: 2,
			PriceListRuleJSON:   `{"mode":"fallback"}`,
			UnitRule: ProductUnitRule{
				InventoryUnit:  "kg",
				QuoteUnit:      "kg",
				OrderUnit:      "kg",
				ConversionJSON: `{}`,
			},
		},
		ProductTypeDefault: &ProductRuleConfig{
			UnitRule: ProductUnitRule{
				InventoryUnit:  "kg",
				QuoteUnit:      "盒",
				OrderUnit:      "盒",
				ConversionJSON: `{"盒":{"kg":0.2}}`,
				IntegerUnit:    true,
			},
		},
		ProductSubtypeDefault: &ProductRuleConfig{
			PriceListRuleJSON: `{"mode":"subtype"}`,
		},
		CustomerTemplate: &ProductRuleConfig{
			OperationTemplateID: 22,
		},
		CustomerOverride: &ProductRuleConfig{
			GradientTemplateID: 99,
		},
	})

	if got.GradientTemplateID != 99 {
		t.Fatalf("gradient template = %d, want customer override 99", got.GradientTemplateID)
	}
	if got.OperationTemplateID != 22 {
		t.Fatalf("operation template = %d, want customer template 22", got.OperationTemplateID)
	}
	if got.PriceListRuleJSON != `{"mode":"subtype"}` {
		t.Fatalf("price rule = %q, want subtype rule", got.PriceListRuleJSON)
	}
	if got.UnitRule.QuoteUnit != "盒" || got.UnitRule.OrderUnit != "盒" || !got.UnitRule.IntegerUnit {
		t.Fatalf("unit rule = %+v, want product type unit rule", got.UnitRule)
	}
}
