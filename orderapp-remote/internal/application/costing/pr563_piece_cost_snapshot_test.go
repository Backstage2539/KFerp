package costing

import (
	"math"
	"testing"

	domain "orderapp/internal/domain/costing"
)

func TestPR563PricingTrialPieceCostUsesConcreteSKUInventoryConversion(t *testing.T) {
	input := domain.ProductInput{
		ProductID:          644,
		SKUID:              1664,
		ParentProductID:    644,
		Name:               "初晓",
		InventoryUnit:      "kg",
		QuoteUnit:          "227g",
		OrderUnit:          "227g",
		UnitConversionJSON: `{"227g":{"kg":0.227}}`,
	}
	row := PricingRuleTrialBaseCostDetail{
		Type:       "operation",
		Name:       "包装",
		CostMethod: "piece",
		PieceRate:  0.5,
		RateUnit:   "sales_spec_count",
	}

	details, _, operationTotal := pricingRuleTrialNormalizeBaseCostDetails(input, "227g", "standard", 0, false, []PricingRuleTrialBaseCostDetail{row})
	if len(details) != 1 || math.Abs(operationTotal-0.5) > 1e-9 || details[0].Unit != "227g" {
		t.Fatalf("227g piece detail = %+v total %.6f, want 0.5/227g", details, operationTotal)
	}

	details, _, operationTotal = pricingRuleTrialNormalizeBaseCostDetails(input, "kg", "standard", 0, false, []PricingRuleTrialBaseCostDetail{row})
	wantPerKg := 2.2 // standard pricing details round 0.5 / 0.227 to cents
	if len(details) != 1 || math.Abs(operationTotal-wantPerKg) > 1e-6 || details[0].Unit != "kg" {
		t.Fatalf("kg piece detail = %+v total %.6f, want %.6f/kg", details, operationTotal, wantPerKg)
	}
}
