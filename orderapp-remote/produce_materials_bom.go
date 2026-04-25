package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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
	fallbackYield := p.YieldRate
	if fallbackYield <= 0 || fallbackYield > 1 {
		fallbackYield = 0.8
	}

	for _, r := range rows {
		if r.GapG <= 0 || r.SpecG <= 0 {
			continue
		}
		finalInputG := finalInputByKey[producePlanKey(r.ProductID, r.SpecG)]
		items := bomMap[r.ProductID]
		if finalInputG <= 0 {
			yield := fallbackYield
			if len(items) > 0 && items[0].YieldRate > 0 && items[0].YieldRate <= 1 {
				yield = items[0].YieldRate
			}
			finalInputG = int64(math.Ceil(float64(r.GapG) / yield))
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
			switch {
			case strings.EqualFold(u, "g"):
				add(bi.MaterialName, int64(math.Ceil(float64(finalInputG)*bi.RatioPct/100.0)), "g")
			case strings.EqualFold(u, "kg"):
				add(bi.MaterialName, int64(math.Ceil((float64(finalInputG)*bi.RatioPct/100.0)/1000.0)), "kg")
			default:
				add(bi.MaterialName, int64(math.Ceil(float64(unitsMissing)*bi.RatioPct/100.0)), u)
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

func calcProducePlanMaterialsWithBOM(ctx context.Context, pool *pgxpool.Pool, schema string, rows []UnprodNeedRow, p ProducePlanParams) []MaterialNeed {
	productIDs := make([]int64, 0)
	seen := map[int64]bool{}
	for _, r := range rows {
		if r.ProductID > 0 && !seen[r.ProductID] {
			seen[r.ProductID] = true
			productIDs = append(productIDs, r.ProductID)
		}
	}
	bomMap, _ := loadBomNeedItems(ctx, pool, schema, productIDs)

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
		items := bomMap[r.ProductID]
		if len(items) == 0 {
			for _, x := range calcNoBomProducePlanMaterials(r, p) {
				add(x.Name, x.Qty, x.Unit)
			}
			continue
		}

		yield := resolveYieldRate(items[0].RoastLevel, items[0].YieldRate)
		if yield <= 0 || yield > 1 {
			yield = normalizeYieldRate(p.YieldRate)
		}
		rawG := int64(math.Ceil(float64(r.GapG) / yield))
		unitsMissing := ceilDiv(r.GapG, r.SpecG)

		for _, bi := range items {
			u := strings.TrimSpace(bi.MaterialUnit)
			if u == "" {
				u = "g"
			}
			if strings.EqualFold(u, "g") {
				need := int64(math.Ceil(float64(rawG) * bi.RatioPct / 100.0))
				add(bi.MaterialName, need, "g")
				continue
			}
			if strings.EqualFold(u, "kg") {
				needKg := int64(math.Ceil((float64(rawG) * bi.RatioPct / 100.0) / 1000.0))
				add(bi.MaterialName, needKg, "kg")
				continue
			}
			// 非重量单位按“缺口件数 * 比例”估算，至少按件向上取整
			needUnits := int64(math.Ceil(float64(unitsMissing) * bi.RatioPct / 100.0))
			add(bi.MaterialName, needUnits, u)
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
	if p.YieldRate <= 0 || p.YieldRate > 1 {
		p.YieldRate = 0.8
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
		Qty:  int64(math.Ceil(float64(r.GapG) / p.YieldRate)),
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

func loadBomNeedItems(ctx context.Context, pool *pgxpool.Pool, schema string, productIDs []int64) (map[int64][]bomNeedItem, error) {
	out := map[int64][]bomNeedItem{}
	if len(productIDs) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`
		SELECT bi.product_id,
		       COALESCE(p.roast_level,''),
		       COALESCE(pb.yield_rate,0),
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(m.unit,''),'g'),
		       COALESCE(bi.ratio_pct,0)
		FROM %s.product_bom_items bi
		LEFT JOIN %s.products p ON p.id=bi.product_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=bi.product_id
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		WHERE bi.product_id = ANY($1)
		ORDER BY bi.product_id, bi.id
	`, schema, schema, schema, schema)
	rows, err := pool.Query(ctx, q, productIDs)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x bomNeedItem
		if err := rows.Scan(&x.ProductID, &x.RoastLevel, &x.YieldRate, &x.MaterialName, &x.MaterialUnit, &x.RatioPct); err != nil {
			return out, err
		}
		if strings.TrimSpace(x.MaterialName) == "" || x.RatioPct <= 0 {
			continue
		}
		out[x.ProductID] = append(out[x.ProductID], x)
	}
	return out, rows.Err()
}
