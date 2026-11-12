package costing

import "testing"

func TestSpecTemplateMetadataSurvivesAvailableAndUnavailableCosts(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		in := ProductInput{ProductID: 42, Name: "商品", CostUnavailable: unavailable, SourceSpecTemplateID: 7, SourceSpecTemplateKey: "spec-2", BomSpecID: 15, InventoryUnit: "袋"}
		out := CalculateProduct(DefaultParameters(), in)
		if out.SourceSpecTemplateID != 7 || out.SourceSpecTemplateKey != "spec-2" || out.BomSpecID != 15 {
			t.Fatalf("unavailable=%v lost specification template metadata: %+v", unavailable, out)
		}
	}
}
