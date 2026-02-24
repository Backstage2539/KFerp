package main

import (
	"math"
	"sort"
	"strings"
)

// MaterialNeed is a simple aggregated output for the first version of /produce/plan.
// We intentionally keep it presentation-oriented (name/unit/qty) to ship value fast.
// Evidence + further refinement can iterate later.
//
// Qty is an integer to avoid floating display; grams are also stored as integers.
// If you later want kg, do formatting in template.
type MaterialNeed struct {
	Name string
	Qty  int64
	Unit string
}

type ProducePlanParams struct {
	YieldRate      float64 // e.g. 0.8
	DripExtraG     int64   // e.g. 100 (extra roast grams per plan if any drip exists)
	DripBoxSpec    int64   // e.g. 5 or 10
	EnableDripBox  bool
	BagNameBySpecG map[int64]string // DEV-043: spec_g -> 袋子物料名
}

func defaultProducePlanParams() ProducePlanParams {
	return ProducePlanParams{
		YieldRate:     0.8,
		DripExtraG:    100,
		DripBoxSpec:   10,
		EnableDripBox: true,
	}
}

func calcProducePlanMaterials(rows []UnprodNeedRow, p ProducePlanParams) []MaterialNeed {
	if p.YieldRate <= 0 || p.YieldRate > 1.0 {
		p.YieldRate = 0.8
	}
	if p.DripBoxSpec <= 0 {
		p.DripBoxSpec = 10
	}

	m := map[string]MaterialNeed{}
	add := func(name string, qty int64, unit string) {
		if qty <= 0 {
			return
		}
		x := m[name]
		x.Name = name
		x.Unit = unit
		x.Qty += qty
		m[name] = x
	}

	yieldRawG := func(finishedG int64) int64 {
		// ceil(finished/yield)
		r := float64(finishedG) / p.YieldRate
		return int64(math.Ceil(r))
	}
	ceilDiv := func(a, b int64) int64 {
		if b <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}

	hasDrip := false
	dripUnits := int64(0)

	for _, r := range rows {
		if r.GapG <= 0 || r.SpecG <= 0 {
			continue
		}
		unitsMissing := ceilDiv(r.GapG, r.SpecG)
		name := strings.TrimSpace(r.Product)

		// Very first version: infer type by product name keywords.
		// Later: add a product_type field and migrate.
		if strings.Contains(name, "挂耳") {
			hasDrip = true
			add("咖啡豆(烘焙)", yieldRawG(r.GapG), "g")
			add("挂耳-过滤袋", unitsMissing, "个")
			add("挂耳-卷膜", unitsMissing, "个")
			add("挂耳-封口贴", unitsMissing, "张")
			dripUnits += unitsMissing
			continue
		}
		if strings.Contains(name, "速溶") {
			// DEV-037: instant material model
			// per-unit consumption: 1 instant box per finished unit
			add("速溶-盒子", unitsMissing, "个")
			continue
		}

		// Default: treat as coffee beans finished product.
		add("咖啡豆(生豆/原豆)", yieldRawG(r.GapG), "g")
		bagName := "豆袋"
		if p.BagNameBySpecG != nil {
			if v := strings.TrimSpace(p.BagNameBySpecG[r.SpecG]); v != "" {
				bagName = v
			}
		}
		add(bagName, unitsMissing, "个")
	}

	if hasDrip {
		if p.EnableDripBox {
			add("挂耳-盒彩", ceilDiv(dripUnits, p.DripBoxSpec), "个")
		}
		if p.DripExtraG > 0 {
			add("咖啡豆(烘焙)", p.DripExtraG, "g")
		}
	}

	out := make([]MaterialNeed, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Unit != out[j].Unit {
			return out[i].Unit < out[j].Unit
		}
		return out[i].Name < out[j].Name
	})
	return out
}
