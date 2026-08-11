package production

import "testing"

func TestIsTwoStageProductionStage(t *testing.T) {
	if !IsTwoStageProductionStage(ProductionStageSemiFinished) {
		t.Fatalf("semi_finished should be two-stage")
	}
	if !IsTwoStageProductionStage(ProductionStagePackaging) {
		t.Fatalf("packaging should be two-stage")
	}
	if IsTwoStageProductionStage("") {
		t.Fatalf("empty stage should not be two-stage")
	}
}

func TestIsPackagingStage(t *testing.T) {
	if !IsPackagingStage(ProductionStagePackaging) {
		t.Fatalf("packaging should be packaging stage")
	}
	if IsPackagingStage(ProductionStageSemiFinished) {
		t.Fatalf("semi_finished should not be packaging stage")
	}
}
