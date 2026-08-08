package production

import (
	"fmt"
	"math"
	bomdomain "orderapp/internal/domain/bom"
	"sort"
	"strings"
)

type bomNeedItem struct {
	ProductID    int64
	RoastLevel   string
	YieldRate    float64
	MaterialName string
	MaterialUnit string
	RatioPct     float64
}

func producePlanKey(productID, specG int64) string {
	return fmt.Sprintf("%d-%d", productID, specG)
}

func calcProducePlanMaterialsFromFinalInputs(rows []UnprodNeedRow, finalInputByKey map[string]int64, bomMap map[int64][]bomNeedItem, p ProducePlanParams) []MaterialNeed {
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
	ceilDiv := func(a, b int64) int64 {
		if b <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}
	for _, r := range rows {
		if r.GapG <= 0 || r.SpecG <= 0 {
			continue
		}
		finalInputG := finalInputByKey[producePlanKey(r.ProductID, r.SpecG)]
		items := bomMap[r.ProductID]
		if finalInputG <= 0 {
			finalInputG = r.GapG
		}
		if len(items) == 0 {
			noBom := r
			noBom.GapG = finalInputG
			for _, item := range calcNoBomProducePlanMaterials(noBom, p) {
				if item.Unit == "个" && strings.Contains(item.Name, "豆袋") {
					item.Qty = ceilDiv(r.GapG, r.SpecG)
				}
				add(item.Name, item.Qty, item.Unit)
			}
			continue
		}

		unitsMissing := ceilDiv(r.GapG, r.SpecG)
		for _, bi := range items {
			u := strings.TrimSpace(bi.MaterialUnit)
			if u == "" {
				u = "g"
			}
			ratioPct := bomdomain.NormalizeRatioPct(bi.RatioPct)
			switch {
			case strings.EqualFold(u, "g"):
				add(bi.MaterialName, int64(math.Ceil(float64(finalInputG)*ratioPct/100.0)), "g")
			case strings.EqualFold(u, "kg"):
				add(bi.MaterialName, int64(math.Ceil((float64(finalInputG)*ratioPct/100.0)/1000.0)), "kg")
			default:
				add(bi.MaterialName, int64(math.Ceil(float64(unitsMissing)*ratioPct/100.0)), u)
			}
		}

		bagName := "豆袋"
		if p.BagNameBySpecG != nil {
			if v := strings.TrimSpace(p.BagNameBySpecG[r.SpecG]); v != "" {
				bagName = v
			}
		}
		add(bagName, unitsMissing, "个")
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

func calcNoBomProducePlanMaterials(r UnprodNeedRow, p ProducePlanParams) []MaterialNeed {
	if r.GapG <= 0 || r.SpecG <= 0 {
		return nil
	}
	if p.DripBoxSpec <= 0 {
		p.DripBoxSpec = 10
	}
	ceilDiv := func(a, b int64) int64 {
		if b <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}
	unitsMissing := ceilDiv(r.GapG, r.SpecG)
	name := strings.TrimSpace(r.Product)
	if strings.Contains(name, "挂耳") || strings.Contains(name, "速溶") {
		return calcProducePlanMaterials([]UnprodNeedRow{r}, p)
	}

	rawName := name + " 生豆"
	if strings.TrimSpace(rawName) == "生豆" {
		rawName = "咖啡豆(生豆/原豆)"
	}
	out := []MaterialNeed{{
		Name: rawName,
		Qty:  r.GapG,
		Unit: "g",
	}}
	bagName := "豆袋"
	if p.BagNameBySpecG != nil {
		if v := strings.TrimSpace(p.BagNameBySpecG[r.SpecG]); v != "" {
			bagName = v
		}
	}
	out = append(out, MaterialNeed{Name: bagName, Qty: unitsMissing, Unit: "个"})
	return out
}
