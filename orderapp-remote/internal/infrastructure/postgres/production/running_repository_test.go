package production

import (
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestResolveFinishConsumedInputUsesEditedInputForFullCompletion(t *testing.T) {
	consumedInputG, partial, err := resolveFinishConsumedInput(
		ProduceRunRow{InputG: 600, BomYieldRate: 0.82, NeedG: 454},
		productionapp.FinishCommand{ConsumedInputG: 700},
		464,
	)
	if err != nil {
		t.Fatal(err)
	}
	if partial {
		t.Fatalf("partial = true, want false for full completion")
	}
	if consumedInputG != 700 {
		t.Fatalf("consumed input = %d, want edited 700", consumedInputG)
	}
}

func TestResolveFinishConsumedInputKeepsPartialGuardrails(t *testing.T) {
	consumedInputG, partial, err := resolveFinishConsumedInput(
		ProduceRunRow{InputG: 600, BomYieldRate: 0.82, NeedG: 1000},
		productionapp.FinishCommand{Partial: true},
		464,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !partial {
		t.Fatalf("partial = false, want true when output is below remaining need")
	}
	if consumedInputG != 566 {
		t.Fatalf("consumed input = %d, want ceil(464 / 0.82) = 566", consumedInputG)
	}
}
