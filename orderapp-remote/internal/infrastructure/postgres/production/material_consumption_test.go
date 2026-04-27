package production

import (
	"strings"
	"testing"
)

func TestMaterialNeedToDeduct(t *testing.T) {
	cases := []struct {
		unit      string
		qty       int64
		wantG     int64
		wantUnits int64
	}{
		{unit: "g", qty: 250, wantG: 250},
		{unit: "kg", qty: 2, wantG: 2000},
		{unit: "克", qty: 300, wantG: 300},
		{unit: "个", qty: 7, wantUnits: 7},
		{unit: "张", qty: 5, wantUnits: 5},
	}

	for _, tc := range cases {
		gotG, gotUnits := materialNeedToDeduct(tc.unit, tc.qty)
		if gotG != tc.wantG || gotUnits != tc.wantUnits {
			t.Fatalf("materialNeedToDeduct(%q,%d) = %d/%d, want %d/%d", tc.unit, tc.qty, gotG, gotUnits, tc.wantG, tc.wantUnits)
		}
	}
}

func TestIsWeightMaterialUnit(t *testing.T) {
	for _, unit := range []string{"g", "kg", "克", "千克"} {
		if !isWeightMaterialUnit(unit) {
			t.Fatalf("expected %q to be weight unit", unit)
		}
	}
	if isWeightMaterialUnit("个") {
		t.Fatalf("expected 个 not to be weight unit")
	}
}

func TestMarshalMaterialConsumptionSummary(t *testing.T) {
	got, err := marshalMaterialConsumptionSummary([]materialConsumptionSummaryItem{
		{MaterialID: 1, MaterialName: "卡蒂姆水洗", Unit: "g", DeductG: 1200, BatchCode: "MB-0000000001"},
		{MaterialID: 9, MaterialName: "豆袋", Unit: "个", DeductUnits: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, needle := range []string{`"material_id":1`, `"material_name":"卡蒂姆水洗"`, `"deduct_g":1200`, `"batch_code":"MB-0000000001"`, `"material_name":"豆袋"`, `"deduct_units":8`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("material summary json missing %q in %s", needle, text)
		}
	}
}
