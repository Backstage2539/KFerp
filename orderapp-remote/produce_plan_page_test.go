package main

import "testing"

func TestInstantMaterialsOnly_FilterInstantRows(t *testing.T) {
	rows := []MaterialNeed{
		{Name: "速溶-盒子", Qty: 5, Unit: "个"},
		{Name: "挂耳-过滤袋", Qty: 10, Unit: "个"},
		{Name: "咖啡豆(烘焙)", Qty: 500, Unit: "g"},
	}
	out := instantMaterialsOnly(rows)
	if len(out) != 1 {
		t.Fatalf("len=%d want=1", len(out))
	}
	if out[0].Name != "速溶-盒子" {
		t.Fatalf("name=%s want=速溶-盒子", out[0].Name)
	}
}
