package main

import "testing"

func TestPositiveGapRows_KeepOnlyPositiveGapAndSpecRows(t *testing.T) {
	rows := []UnprodNeedRow{
		{Product: "咖啡豆A", SpecG: 250, GapG: 500},
		{Product: "咖啡豆A", SpecG: 454, GapG: 0},
		{Product: "咖啡豆B", SpecG: 250, GapG: 1},
		{Product: "挂耳X", SpecG: 10, GapG: -1},
	}

	out := positiveGapRows(rows)
	if len(out) != 2 {
		t.Fatalf("rows=%d want=2", len(out))
	}
	if out[0].Product != "咖啡豆A" || out[0].SpecG != 250 {
		t.Fatalf("row0 mismatch: %+v", out[0])
	}
	if out[1].Product != "咖啡豆B" || out[1].SpecG != 250 {
		t.Fatalf("row1 mismatch: %+v", out[1])
	}
}
