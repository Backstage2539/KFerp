package production

import (
	"fmt"
	"testing"
)

func TestFrozenOutputCalculationAppliesLossExactlyOnce(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want int64
	}{{"yield_denominator", 1243}, {"additive", 1195}, {"legacy_fraction", 1243}} {
		t.Run(tc.mode, func(t *testing.T) {
			original := fmt.Sprintf(`[{"material_id":7,"material_name":"生豆","unit":"g","source":"bom","consume_unit":"ratio_pct","ratio_pct":100,"material_loss_rate":0.195,"loss_calculation_mode":"%s","input_includes_material_loss":true}]`, tc.mode)
			calculated, err := manufacturingOutputCalculationSnapshot(original)
			if err != nil {
				t.Fatal(err)
			}
			out, ok, err := materialSnapshotNeedsTx(ProduceRunRow{NeedG: 1000, InputG: 1000, MaterialSnapshot: calculated}, InvQty{LooseG: 1000})
			if err != nil || !ok || len(out) != 1 || out[0].DeductG != tc.want {
				t.Fatalf("output basis needs %+v ok=%v err=%v, want %d", out, ok, err, tc.want)
			}
			frozen, err := freezeManufacturingOutputMaterialSnapshot(original)
			if err != nil {
				t.Fatal(err)
			}
			consumed, ok, err := materialSnapshotNeedsTx(ProduceRunRow{NeedG: 1000, InputG: tc.want, MaterialSnapshot: frozen}, InvQty{LooseG: 1000})
			if err != nil || !ok || len(consumed) != 1 || consumed[0].DeductG != tc.want {
				t.Fatalf("frozen gross needs %+v ok=%v err=%v", consumed, ok, err)
			}
		})
	}
}
