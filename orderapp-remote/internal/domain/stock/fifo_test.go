package stock

import "testing"

func TestAllocateFIFOConsumesOldestPositiveBatches(t *testing.T) {
	batches := []BatchAvailability{
		{BatchID: 10, BatchCode: "MB-OLD", AvailableG: 800},
		{BatchID: 11, BatchCode: "MB-NEW", AvailableG: 700},
	}

	alloc, err := AllocateFIFO(batches, 1000)
	if err != nil {
		t.Fatalf("AllocateFIFO: %v", err)
	}

	if len(alloc) != 2 {
		t.Fatalf("len(alloc)=%d, want 2", len(alloc))
	}
	if alloc[0].BatchID != 10 || alloc[0].QtyG != 800 {
		t.Fatalf("first allocation = %+v, want batch 10 / 800g", alloc[0])
	}
	if alloc[1].BatchID != 11 || alloc[1].QtyG != 200 {
		t.Fatalf("second allocation = %+v, want batch 11 / 200g", alloc[1])
	}
}

func TestAllocateFIFOFailsWhenStockIsInsufficient(t *testing.T) {
	_, err := AllocateFIFO([]BatchAvailability{{BatchID: 1, BatchCode: "MB-1", AvailableG: 300}}, 450)
	if err == nil {
		t.Fatal("expected insufficient stock error")
	}
}

func TestMovementKindValidatesStockDocumentSources(t *testing.T) {
	for _, kind := range []MovementKind{MovementMaterialReceipt, MovementMaterialIssue, MovementFinishedReceipt, MovementMaterialTransfer, MovementAdjustment} {
		if !kind.Valid() {
			t.Fatalf("expected %q to be valid", kind)
		}
	}
	if MovementKind("random").Valid() {
		t.Fatal("unexpected valid movement kind")
	}
}
