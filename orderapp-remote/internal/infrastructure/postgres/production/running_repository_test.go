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

func TestResolveFinishConsumedInputProratesFrozenAdditiveBomInput(t *testing.T) {
	consumedInputG, partial, err := resolveFinishConsumedInput(
		ProduceRunRow{
			InputG: 1200, BomYieldRate: 1, NeedG: 1000,
			MaterialSnapshot: `[{"material_id":1,"material_name":"曲奇生豆","loss_calculation_mode":"additive"}]`,
		},
		productionapp.FinishCommand{Partial: true},
		500,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !partial || consumedInputG != 600 {
		t.Fatalf("partial=%v consumed input=%d, want additive BOM ratio 500/1000 * 1200 = 600", partial, consumedInputG)
	}
}
