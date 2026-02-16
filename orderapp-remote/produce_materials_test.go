package main

import "testing"

func TestCalcProducePlanMaterials(t *testing.T) {
	rows := []UnprodNeedRow{
		{Product: "咖啡豆A", SpecG: 250, GapG: 500},  // 2 bags
		{Product: "挂耳B", SpecG: 100, GapG: 1000},  // 10 units
		{Product: "速溶C", SpecG: 1, GapG: 3},       // 3 units
		{Product: "挂耳B", SpecG: 100, GapG: 1},     // 1 unit (ceil)
		{Product: "咖啡豆A", SpecG: 250, GapG: 1},    // 1 unit (ceil)
		{Product: "无缺口", SpecG: 250, GapG: 0},     // ignored
		{Product: "坏规格", SpecG: 0, GapG: 100},     // ignored
		{Product: "空", SpecG: 100, GapG: -100},    // ignored
		{Product: "挂耳-特调", SpecG: 100, GapG: 100}, // 1 unit
	}

	p := defaultProducePlanParams()
	p.YieldRate = 0.8
	p.DripExtraG = 100
	p.DripBoxSpec = 10

	out := calcProducePlanMaterials(rows, p)
	m := map[string]MaterialNeed{}
	for _, v := range out {
		m[v.Name] = v
	}

	// Coffee beans: finished gap 500g + 1g => 501 => raw ceil(501/0.8)=627g
	// Drip roast beans: finished gap 1000 + 1 + 100 => 1101 => raw ceil(1101/0.8)=1377g, plus extra 100g added later -> Actually function adds raw for drip gaps plus extra 100g.
	if got := m["豆袋"].Qty; got != 3 { // 500g=2 units + 1g=1 unit
		t.Fatalf("豆袋 qty=%d want=3", got)
	}
	if got := m["速溶-盒子"].Qty; got != 3 {
		t.Fatalf("速溶-盒子 qty=%d want=3", got)
	}
	if got := m["挂耳-过滤袋"].Qty; got != 12 { // 10 + 1 + 1
		t.Fatalf("挂耳-过滤袋 qty=%d want=12", got)
	}
	if got := m["挂耳-盒彩"].Qty; got != 2 { // ceil(12/10)=2
		t.Fatalf("挂耳-盒彩 qty=%d want=2", got)
	}
	if got := m["咖啡豆(生豆/原豆)"].Qty; got != 627 {
		t.Fatalf("咖啡豆(生豆/原豆) qty=%d want=627", got)
	}
	// Drip roast beans: finished gap = 1000 + 1 + 100 = 1101 => raw ceil(1101/0.8)=1377, plus extra 100g => 1477
	if got := m["咖啡豆(烘焙)"].Qty; got != 1477 {
		t.Fatalf("咖啡豆(烘焙) qty=%d want=1477", got)
	}
}
