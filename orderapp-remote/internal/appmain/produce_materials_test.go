package appmain

import "testing"

func assertMaterialQty(t *testing.T, rows []MaterialNeed, name, unit string, want int64) {
	t.Helper()
	for _, row := range rows {
		if row.Name == name && row.Unit == unit {
			if row.Qty != want {
				t.Fatalf("%s %s qty=%d want=%d", name, unit, row.Qty, want)
			}
			return
		}
	}
	t.Fatalf("material %s %s not found", name, unit)
}

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

func TestCalcProducePlanMaterials_BagMappingBySpec(t *testing.T) {
	rows := []UnprodNeedRow{
		{Product: "咖啡豆A", SpecG: 454, GapG: 908}, // 2 bags
		{Product: "咖啡豆B", SpecG: 250, GapG: 250}, // 1 bag
	}
	p := defaultProducePlanParams()
	p.BagNameBySpecG = map[int64]string{454: "银袋454g"}

	out := calcProducePlanMaterials(rows, p)
	m := map[string]MaterialNeed{}
	for _, v := range out {
		m[v.Name] = v
	}
	if got := m["银袋454g"].Qty; got != 2 {
		t.Fatalf("银袋454g qty=%d want=2", got)
	}
	if got := m["豆袋"].Qty; got != 1 {
		t.Fatalf("豆袋 qty=%d want=1", got)
	}
}

func TestCalcNoBomProducePlanMaterialsSplitsRawBeansByProduct(t *testing.T) {
	p := defaultProducePlanParams()
	p.YieldRate = 0.8

	rows := []UnprodNeedRow{
		{Product: "Uraga乌拉嘎", SpecG: 227, GapG: 227},
		{Product: "小菠萝2.0", SpecG: 227, GapG: 227},
	}

	m := map[string]MaterialNeed{}
	for _, row := range rows {
		for _, v := range calcNoBomProducePlanMaterials(row, p) {
			x := m[v.Name]
			x.Name = v.Name
			x.Unit = v.Unit
			x.Qty += v.Qty
			m[v.Name] = x
		}
	}

	if got := m["Uraga乌拉嘎 生豆"].Qty; got != 284 {
		t.Fatalf("Uraga乌拉嘎 生豆 qty=%d want=284", got)
	}
	if got := m["小菠萝2.0 生豆"].Qty; got != 284 {
		t.Fatalf("小菠萝2.0 生豆 qty=%d want=284", got)
	}
	if _, ok := m["咖啡豆(生豆/原豆)"]; ok {
		t.Fatalf("普通无BOM商品不应汇总到通用生豆行")
	}
	if got := m["豆袋"].Qty; got != 2 {
		t.Fatalf("豆袋 qty=%d want=2", got)
	}
}

func TestCalcProducePlanMaterialsFromFinalInputsUsesRoastInputForBomBeans(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 1, Product: "曲奇拼配", SpecG: 1000, GapG: 1000},
	}
	finalInputs := map[string]int64{producePlanKey(1, 1000): 2000}
	bomMap := map[int64][]bomNeedItem{
		1: {
			{ProductID: 1, MaterialName: "豆子A", MaterialUnit: "g", RatioPct: 0.70},
			{ProductID: 1, MaterialName: "豆子B", MaterialUnit: "g", RatioPct: 0.30},
		},
	}

	got := calcProducePlanMaterialsFromFinalInputs(rows, finalInputs, bomMap, defaultProducePlanParams())

	assertMaterialQty(t, got, "豆子A", "g", 1400)
	assertMaterialQty(t, got, "豆子B", "g", 600)
	assertMaterialQty(t, got, "豆袋", "个", 1)
}
