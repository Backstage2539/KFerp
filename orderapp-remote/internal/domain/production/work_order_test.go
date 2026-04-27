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
