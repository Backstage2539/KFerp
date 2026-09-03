package sales

import (
	"encoding/json"
	"testing"
)

func TestManualBOMSpecPriceSnapshotFreezesSpecificationUnitAndPrice(t *testing.T) {
	identity := orderBOMSpecIdentity{
		ProductID:     52,
		BomSpecID:     222,
		BomVariantID:  272,
		BomSpecKey:    "spec-9",
		BomSpecName:   "2Kg袋装",
		InventoryUnit: "袋",
	}
	raw := withOrderBOMSpecPriceSourceJSON(`{"source":"manual"}`, identity)
	raw = withOrderManualPriceSnapshotJSON(raw, identity.InventoryUnit, 162.3)

	var snapshot map[string]any
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["price_unit"] != "袋" || snapshot["inventory_unit"] != "袋" {
		t.Fatalf("snapshot units = price:%v inventory:%v, want 袋/袋", snapshot["price_unit"], snapshot["inventory_unit"])
	}
	if snapshot["final_unit_price"] != 162.3 || snapshot["manual_adjusted"] != true {
		t.Fatalf("manual price snapshot = price:%v adjusted:%v", snapshot["final_unit_price"], snapshot["manual_adjusted"])
	}
}
