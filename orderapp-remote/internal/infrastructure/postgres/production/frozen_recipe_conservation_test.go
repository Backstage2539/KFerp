package production

import (
	"encoding/json"
	"testing"
)

func TestFrozenMarkedRecipePreservesGrossInput(t *testing.T) {
	rows := []materialSnapshotRow{
		{MaterialID: 1, MaterialName: "a", Unit: "kg", Source: "bom", ConsumeUnit: "ratio_pct", RatioPct: 15, MaterialLossRate: .195, InputIncludesMaterialLoss: true},
		{MaterialID: 2, MaterialName: "b", Unit: "g", Source: "bom", ConsumeUnit: "ratio_pct", RatioPct: 15, MaterialLossRate: .195, InputIncludesMaterialLoss: true},
		{MaterialID: 3, MaterialName: "c", Unit: "kg", Source: "bom", ConsumeUnit: "ratio_pct", RatioPct: 20, MaterialLossRate: .195, InputIncludesMaterialLoss: true},
		{MaterialID: 4, MaterialName: "d", Unit: "kg", Source: "bom", ConsumeUnit: "ratio_pct", RatioPct: 50, MaterialLossRate: .195, InputIncludesMaterialLoss: true},
		{MaterialID: 5, MaterialName: "bags", Unit: "袋", Source: "packaging"},
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	if err = json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		entries[i]["ratio_allocation_mode"] = "gross_largest_remainder_v1"
	}
	marked, _ := json.Marshal(entries)
	test := func(name string, snapshot []byte, want []int64) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			needs, ok, err := materialSnapshotNeedsTx(ProduceRunRow{InputG: 74674, NeedG: 60112, MaterialSnapshot: string(snapshot)}, InvQty{Units: 2, LooseG: 60112})
			if err != nil || !ok || len(needs) != 5 {
				t.Fatalf("needs=%+v ok=%v err=%v", needs, ok, err)
			}
			for i, v := range want {
				if needs[i].DeductG != v {
					t.Fatalf("row %d: got %d want %d; all %+v", i, needs[i].DeductG, v, needs)
				}
			}
			if needs[4].DeductUnits != 2 {
				t.Fatalf("packaging changed: %+v", needs[4])
			}
			if needs[0].QtyDecimal != float64(want[0])/1000 || needs[1].QtyDecimal != float64(want[1]) {
				t.Fatalf("unit displays changed: %+v %+v", needs[0], needs[1])
			}
		})
	}
	test("marked conserves", marked, []int64{11201, 11201, 14935, 37337})
	test("unmarked historical remains unchanged", raw, []int64{11202, 11202, 14935, 37337})
}

func TestFrozenMarkedRecipeRemainderAndValidation(t *testing.T) {
	ratios := []float64{15, 15, 20, 50}
	rows := make([]materialSnapshotRow, 0, 4)
	for i, ratio := range ratios {
		rows = append(rows, materialSnapshotRow{MaterialID: int64(i + 1), MaterialName: "bean", Unit: "kg", Source: "bom", ConsumeUnit: "ratio_pct", RatioPct: ratio, MaterialLossRate: .195, InputIncludesMaterialLoss: true, RatioAllocationMode: grossLargestRemainderRatioAllocationMode})
	}
	cases := []struct {
		total int64
		want  []int64
	}{
		{12428, []int64{1864, 1864, 2486, 6214}},
		{62251, []int64{9338, 9338, 12450, 31125}},
		{1, []int64{0, 0, 0, 1}},
	}
	for _, tc := range cases {
		alloc, err := frozenRecipeGrossAllocations(rows, tc.total)
		if err != nil {
			t.Fatal(err)
		}
		sum := int64(0)
		for i, want := range tc.want {
			if alloc[i] != want {
				t.Fatalf("total %d index %d got %d want %d", tc.total, i, alloc[i], want)
			}
			sum += alloc[i]
		}
		if sum != tc.total {
			t.Fatalf("sum %d != %d", sum, tc.total)
		}
	}
	// A partially marked snapshot must fail safely, rather than silently reverting
	// some rows to independently rounded historical behavior.
	broken := append([]materialSnapshotRow(nil), rows...)
	broken[0].RatioAllocationMode = ""
	if _, err := frozenRecipeGrossAllocations(broken, 12428); err == nil {
		t.Fatal("partially marked recipe accepted")
	}
	broken = append([]materialSnapshotRow(nil), rows...)
	broken[0].RatioPct = 14
	if _, err := frozenRecipeGrossAllocations(broken, 12428); err == nil {
		t.Fatal("incomplete ratio accepted")
	}
	broken = append([]materialSnapshotRow(nil), rows...)
	broken[0].MaterialLossRate = .1
	if _, err := frozenRecipeGrossAllocations(broken, 12428); err == nil {
		t.Fatal("inconsistent loss accepted")
	}
}
