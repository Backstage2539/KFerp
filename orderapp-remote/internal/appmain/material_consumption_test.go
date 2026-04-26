package appmain

import "testing"

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
