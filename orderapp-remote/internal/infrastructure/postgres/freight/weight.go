package freight

import (
	"context"
	"fmt"
	"math"
	domain "orderapp/internal/domain/freight"
	"orderapp/internal/infrastructure/postgres/orderweight"
	"strings"
)

type WeightItem struct{ ProductID, BomSpecID, BomVariantID, Qty int64 }

func relationExists(ctx context.Context, q Queryer, name string) bool {
	var ok bool
	return q.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok) == nil && ok
}

// WeightTotals sums unrounded manual specification weights, then rounds once for
// carrier billing. Free shipping may retain an explicitly unknown cost estimate.
func (r *Repository) WeightTotals(ctx context.Context, q Queryer, customerID int64, items []WeightItem, allowUnknown bool) (int64, []domain.WeightBasis, error) {
	basis := make([]domain.WeightBasis, 0, len(items))
	total := 0.0
	unknown := false
	for _, i := range items {
		if i.ProductID <= 0 || i.Qty <= 0 || i.Qty > 1000000 {
			return 0, nil, fmt.Errorf("商品数量无效")
		}
		var name string
		err := q.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.products p WHERE id=$1 AND COALESCE((to_jsonb(p)->>'active')::boolean,true) AND COALESCE((to_jsonb(p)->>'customer_id')::bigint,0) IN (0,$2)`, r.schema), i.ProductID, customerID).Scan(&name)
		if err != nil {
			return 0, nil, fmt.Errorf("商品不存在或不可用")
		}
		v, err := orderweight.Resolve(ctx, q, r.schema, orderweight.Item{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, Qty: float64(i.Qty)})
		if err != nil {
			return 0, nil, err
		}
		basis = append(basis, domain.WeightBasis{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, BomVersionID: v.BomVersionID, Qty: i.Qty, LogisticsUnitWeightG: v.LogisticsUnitWeightG})
		if v.LogisticsUnitWeightG == nil {
			if !allowUnknown {
				return 0, nil, fmt.Errorf("商品「%s」规格未配置物流重量，请在商品 BOM 规格中手工填写并发布后重新计算运费", name)
			}
			basis[len(basis)-1].MissingReason = fmt.Sprintf("商品「%s」规格未配置物流重量", name)
			unknown = true
			continue
		}
		total += *v.LogisticsUnitWeightG * float64(i.Qty)
		if total > 1000000000 {
			return 0, nil, fmt.Errorf("商品物流重量超出可处理范围")
		}
	}
	if unknown {
		return 0, basis, nil
	}
	// Stored weights have six decimal places; eliminate binary float residue only.
	total = math.Round(total*1e6) / 1e6
	return int64(math.Ceil(total)), basis, nil
}

// ItemNetWeightG is retained for existing integrations; quotes use WeightTotals
// to avoid rounding each unit or line before aggregation.
func (r *Repository) ItemNetWeightG(ctx context.Context, q Queryer, customerID int64, item WeightItem) (int64, error) {
	item.Qty = 1
	g, _, e := r.WeightTotals(ctx, q, customerID, []WeightItem{item}, false)
	return g, e
}

// AttachWeightBasis binds both the manual weight and selected version to the quote.
func AttachWeightBasis(q domain.Snapshot, basis []domain.WeightBasis) domain.Snapshot {
	q.ItemWeights = basis
	var missing []string
	for _, w := range basis {
		if w.MissingReason != "" {
			missing = append(missing, w.MissingReason)
		}
	}
	if !q.EstimateKnown && len(missing) > 0 {
		q.CostPendingReason = strings.Join(missing, "；") + "，预估运费未计算，请补填并发布商品 BOM 规格"
	}
	q.Token = domain.Token(q)
	return q
}
