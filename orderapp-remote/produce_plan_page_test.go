package main

import "testing"

func TestInstantMaterialsOnly_FilterInstantRows(t *testing.T) {
	rows := []MaterialNeed{
		{Name: "速溶-盒子", Qty: 5, Unit: "个"},
		{Name: "挂耳-过滤袋", Qty: 10, Unit: "个"},
	}
	out := instantMaterialsOnly(rows)
	if len(out) != 1 || out[0].Name != "速溶-盒子" {
		t.Fatalf("unexpected out=%+v", out)
	}
}
