package production

import "testing"

func TestCanTransitionWorkOrderStatus(t *testing.T) {
	if !CanTransitionWorkOrder(StatusDraft, StatusReleased) {
		t.Fatal("draft should transition to released")
	}
	if !CanTransitionWorkOrder(StatusReleased, StatusRunning) {
		t.Fatal("released should transition to running")
	}
	if !CanTransitionWorkOrder(StatusRunning, StatusCompleted) {
		t.Fatal("running should transition to completed")
	}
	if CanTransitionWorkOrder(StatusCompleted, StatusRunning) {
		t.Fatal("completed should not transition to running")
	}
}

func TestActualBatchCost(t *testing.T) {
	got := BatchCost([]CostComponent{{Amount: 12.5}, {Amount: 7.25}})
	if got != 19.75 {
		t.Fatalf("BatchCost = %.2f, want 19.75", got)
	}
}

func TestWIPReservationRemainingAndAdjustment(t *testing.T) {
	current := WIPReservationQuantity{ReservedG: 60000, ConsumedG: 15000, ReturnedG: 5000}
	if got := current.RemainingG(); got != 40000 {
		t.Fatalf("RemainingG() = %d, want 40000", got)
	}

	target, err := ValidateWIPReservationAdjustment(WIPReservationAdjustment{
		Current:         current,
		TargetReservedG: 50000,
		WIPG:            90000,
		OtherReservedG:  25000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.RemainingG() != 30000 {
		t.Fatalf("adjusted remaining = %d, want 30000", target.RemainingG())
	}

	if _, err := ValidateWIPReservationAdjustment(WIPReservationAdjustment{
		Current:         current,
		TargetReservedG: 10000,
		WIPG:            90000,
		OtherReservedG:  25000,
	}); err == nil {
		t.Fatal("expected adjustment below consumed quantity to fail")
	}

	if _, err := ValidateWIPReservationAdjustment(WIPReservationAdjustment{
		Current:         current,
		TargetReservedG: 90000,
		WIPG:            90000,
		OtherReservedG:  25000,
	}); err == nil {
		t.Fatal("expected adjustment above available WIP to fail")
	}
}
