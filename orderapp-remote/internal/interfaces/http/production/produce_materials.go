package production

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
	Name string `json:"name"`
	Qty  int64  `json:"qty"`
	Unit string `json:"unit"`
}

type ProducePlanBomItem struct {
	MaterialName string
	RatioPct     float64
	Unit         string
}

type ProducePlanParams struct {
	YieldRate      float64 // compatibility only; current plans use BOM material loss
	DripExtraG     int64   // e.g. 100 (extra roast grams per plan if any drip exists)
	DripBoxSpec    int64   // e.g. 5 or 10
	EnableDripBox  bool
	BagNameBySpecG map[int64]string // DEV-043: spec_g -> 袋子物料名
	BomByProductID map[int64][]ProducePlanBomItem
}

func defaultProducePlanParams() ProducePlanParams {
	return ProducePlanParams{
		YieldRate:     1,
		DripExtraG:    100,
		DripBoxSpec:   10,
		EnableDripBox: true,
	}
}

func instantMaterialsOnly(rows []MaterialNeed) []MaterialNeed {
	out := make([]MaterialNeed, 0, len(rows))
	for _, r := range rows {
		if strings.Contains(r.Name, "速溶") {
			out = append(out, r)
		}
	}
	return out
}

func calcProducePlanMaterials(rows []UnprodNeedRow, p ProducePlanParams) []MaterialNeed {
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
		return finishedG
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
			add("速溶-盒子", unitsMissing, "个")
			continue
		}

		// Default: use BOM (if configured) to split into concrete materials.
		rawG := yieldRawG(r.GapG)
		usedBom := false
		if p.BomByProductID != nil {
			if bomItems := p.BomByProductID[r.ProductID]; len(bomItems) > 0 {
				for _, bi := range bomItems {
					qty := int64(math.Ceil(float64(rawG) * bi.RatioPct / 100.0))
					unit := strings.TrimSpace(bi.Unit)
					if unit == "" {
						unit = "g"
					}
					add(strings.TrimSpace(bi.MaterialName), qty, unit)
				}
				usedBom = true
			}
		}
		if !usedBom {
			add("咖啡豆(生豆/原豆)", rawG, "g")
		}
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
