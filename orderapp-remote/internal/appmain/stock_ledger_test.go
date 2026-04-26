package appmain

import "testing"

func TestFinishedInventoryLedgerQtyConvertsUnitsAndLooseToGrams(t *testing.T) {
	got, err := finishedInventoryLedgerQty(
		227,
		InvQty{Units: 1, LooseG: 20},
		InvQty{Units: 2, LooseG: 10},
		InvQty{Units: 3, LooseG: 30},
	)
	if err != nil {
		t.Fatalf("finishedInventoryLedgerQty: %v", err)
	}

	if got.BeforeG != 247 {
		t.Fatalf("BeforeG = %d, want 247", got.BeforeG)
	}
	if got.ChangeG != 464 {
		t.Fatalf("ChangeG = %d, want 464", got.ChangeG)
	}
	if got.AfterG != 711 {
		t.Fatalf("AfterG = %d, want 711", got.AfterG)
	}
	if got.BeforeUnits != 1 || got.ChangeUnits != 2 || got.AfterUnits != 3 {
		t.Fatalf("unit movement = before %d change %d after %d, want 1/2/3", got.BeforeUnits, got.ChangeUnits, got.AfterUnits)
	}
}

func TestFinishedProductionBatchCodeUsesRunningItemID(t *testing.T) {
	got := finishedProductionBatchCode(42)
	if got != "FP-0000000042" {
		t.Fatalf("finishedProductionBatchCode(42) = %q, want FP-0000000042", got)
	}
}
