package production

import "testing"

func TestPlanTransferRequestValidation(t *testing.T) {
	good := PlanMaterialTransferCommand{ProductionPlanID: 1, DraftToken: "token", IdempotencyKey: "key", Items: []PlanMaterialTransferItem{{SourceID: 2, Warehouse: "raw_materials", Batches: []PlanMaterialTransferBatch{{BatchCode: "B1", QtyG: 10}}}}}
	if err := validatePlanMaterialTransferCommand(good); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	bad := []PlanMaterialTransferCommand{good, good, good, good}
	bad[0].DraftToken = ""
	bad[1].IdempotencyKey = ""
	bad[2].Items = []PlanMaterialTransferItem{{SourceID: 2, Warehouse: "wip", Batches: []PlanMaterialTransferBatch{{BatchCode: "B1", QtyG: 1}}}}
	bad[3].Items = []PlanMaterialTransferItem{{SourceID: 2, Warehouse: "raw_materials", Batches: []PlanMaterialTransferBatch{{BatchCode: "B1", QtyG: -1}}}}
	for i, v := range bad {
		if err := validatePlanMaterialTransferCommand(v); err == nil {
			t.Fatalf("case %d accepted invalid request", i)
		}
	}
}

func TestPlanTransferRequestAllowsEquivalentWeightAndCountForValidationByFrozenSource(t *testing.T) {
	c := PlanMaterialTransferCommand{ProductionPlanID: 1, DraftToken: "t", IdempotencyKey: "k", Items: []PlanMaterialTransferItem{{SourceID: 2, Warehouse: "raw_materials", Batches: []PlanMaterialTransferBatch{{BatchCode: "B", QtyG: 908, QtyUnits: 2}}}}}
	if err := validatePlanMaterialTransferCommand(c); err != nil {
		t.Fatalf("domain must defer mixed quantity to frozen-source validation: %v", err)
	}
}
