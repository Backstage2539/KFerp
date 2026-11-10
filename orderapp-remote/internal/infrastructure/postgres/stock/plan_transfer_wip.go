package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
)

type planWIPSource struct {
	ID                                      int64
	Kind                                    string
	ItemID, SpecID, VariantID, SpecG, Owner int64
	RequiredG, RequiredUnits                int64
	Mode                                    string
	Manual                                  []productionapp.ProductionPlanSourceAllocation
}
type planWIPQty struct{ g, n int64 }

// Finished stock options expose loose grams separately from packaged units;
// production requirements use total grams as well as unit count.
func planWIPOptionAvailable(kind string, specG, g, n int64) (planWIPQty, error) {
	if g < 0 || n < 0 || specG < 0 {
		return planWIPQty{}, fmt.Errorf("WIP库存数量无效，请先核对")
	}
	if kind == "finished_product" && specG > 0 {
		const maxInt64 = int64(^uint64(0) >> 1)
		if n > (maxInt64-g)/specG {
			return planWIPQty{}, fmt.Errorf("WIP库存数量过大，请先核对")
		}
		g += n * specG
	}
	return planWIPQty{g, n}, nil
}

// Both availability and consumption budgets must preserve frozen variant identity.
func planWIPAvailableCacheKey(s planWIPSource, warehouse string) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%s:%d", s.Kind, s.ItemID, s.SpecID, s.VariantID, s.SpecG, warehouse, s.Owner)
}

func validatePlanWIPNoMixedVariants(sources []planWIPSource) error {
	frozen := map[string]bool{}
	identity := func(s planWIPSource) string { return fmt.Sprintf("%d:%d:%d", s.ItemID, s.SpecID, s.SpecG) }
	for _, s := range sources {
		if (s.Kind == "product" || s.Kind == "finished_product") && s.VariantID > 0 {
			frozen[identity(s)] = true
		}
	}
	for _, s := range sources {
		if (s.Kind == "product" || s.Kind == "finished_product") && s.VariantID == 0 && (s.RequiredG > 0 || s.RequiredUnits > 0) && frozen[identity(s)] {
			return fmt.Errorf("历史物料未冻结版本，与同计划冻结版本混用，请先核对生产配置")
		}
	}
	return nil
}

// planCurrentWIPCoverageTx reconstructs the draft allocation order for WIP:
// all manual allocations first, then WIP-first automatic allocations. A shared
// budget prevents two tasks from counting the same stock.
func (r Repository) planCurrentWIPCoverageTx(ctx context.Context, tx pgx.Tx, planID int64) (map[int64]planWIPQty, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT s.id,s.component_type,s.component_id,s.component_bom_spec_id,s.component_bom_variant_id,s.component_spec_g,
  CASE WHEN s.component_type='material' THEN COALESCE(m.owner_customer_id,0) ELSE 0 END,
  s.required_g,s.required_units,s.allocation_mode,s.allocations_json
 FROM %[1]s.production_plan_component_sources s LEFT JOIN %[1]s.materials m ON s.component_type='material' AND m.id=s.component_id
 WHERE s.production_plan_id=$1 ORDER BY s.production_plan_item_id,s.id`, r.schema), planID)
	if err != nil {
		return nil, err
	}
	sources := []planWIPSource{}
	for rows.Next() {
		var s planWIPSource
		var raw []byte
		if err := rows.Scan(&s.ID, &s.Kind, &s.ItemID, &s.SpecID, &s.VariantID, &s.SpecG, &s.Owner, &s.RequiredG, &s.RequiredUnits, &s.Mode, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if s.Mode == "manual" {
			if err = json.Unmarshal(raw, &s.Manual); err != nil {
				rows.Close()
				return nil, err
			}
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err := validatePlanWIPNoMixedVariants(sources); err != nil {
		return nil, err
	}
	available := map[string]planWIPQty{}
	used := map[string]planWIPQty{}
	remain := map[int64]planWIPQty{}
	coverage := map[int64]planWIPQty{}
	keyFor := func(s planWIPSource, warehouse string) string {
		return fmt.Sprintf("%s:%d:%d:%d:%d:%s:%d", s.Kind, s.ItemID, s.SpecID, s.VariantID, s.SpecG, warehouse, s.Owner)
	}
	getAvailable := func(s planWIPSource, warehouse string) (planWIPQty, error) {
		key := planWIPAvailableCacheKey(s, warehouse)
		if q, ok := available[key]; ok {
			return q, nil
		}
		var active bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.warehouses WHERE code=$1`, r.schema), warehouse).Scan(&active); err == pgx.ErrNoRows {
			return planWIPQty{}, nil
		} else if err != nil {
			return planWIPQty{}, err
		}
		if !active {
			return planWIPQty{}, nil
		}
		kind := s.Kind
		if kind == "product" {
			kind = "finished_product"
		}
		result, err := r.ListTransferOptionsTx(ctx, tx, stockapp.TransferOptionsQuery{Warehouse: warehouse, ItemType: kind, ItemID: s.ItemID, BomSpecID: s.SpecID, SpecG: s.SpecG, Limit: 100000})
		if err != nil {
			return planWIPQty{}, err
		}
		total := planWIPQty{}
		for _, o := range result.Rows {
			if o.ItemType != kind || o.ItemID != s.ItemID || o.OwnerCustomerID != s.Owner {
				continue
			}
			if kind == "finished_product" && (o.BomSpecID != s.SpecID || o.BomVariantID != s.VariantID || o.SpecG != s.SpecG) {
				continue
			}
			if o.UnavailableReason != "" {
				if warehouse == "wip" && (o.QtyG > 0 || o.QtyUnits > 0) {
					return planWIPQty{}, fmt.Errorf("WIP库存存在不可用或占用异常，请先核对")
				}
				continue
			}
			converted, err := planWIPOptionAvailable(kind, s.SpecG, o.AvailableG, o.AvailableUnits)
			if err != nil {
				return planWIPQty{}, err
			}
			const maxInt64 = int64(^uint64(0) >> 1)
			if total.g > maxInt64-converted.g || total.n > maxInt64-converted.n {
				return planWIPQty{}, fmt.Errorf("WIP库存数量过大，请先核对")
			}
			total.g += converted.g
			total.n += converted.n
		}
		available[key] = total
		return total, nil
	}
	min := func(a, b int64) int64 {
		if a < 0 {
			return 0
		}
		if b < 0 {
			return 0
		}
		if a < b {
			return a
		}
		return b
	}
	allocate := func(s planWIPSource, warehouse string, g, n int64) error {
		av, e := getAvailable(s, warehouse)
		if e != nil {
			return e
		}
		key := keyFor(s, warehouse)
		u := used[key]
		remaining := remain[s.ID]
		g = min(g, remaining.g)
		n = min(n, remaining.n)
		g = min(g, av.g-u.g)
		n = min(n, av.n-u.n)
		used[key] = planWIPQty{u.g + g, u.n + n}
		remain[s.ID] = planWIPQty{remaining.g - g, remaining.n - n}
		if warehouse == "wip" {
			v := coverage[s.ID]
			coverage[s.ID] = planWIPQty{v.g + g, v.n + n}
		}
		return nil
	}
	for _, s := range sources {
		remain[s.ID] = planWIPQty{s.RequiredG, s.RequiredUnits}
	}
	for _, s := range sources {
		if s.Mode != "manual" {
			continue
		}
		for _, a := range s.Manual {
			if a.OwnerCustomerID != s.Owner {
				continue
			}
			if err := allocate(s, a.Warehouse, a.QtyG, a.QtyUnits); err != nil {
				return nil, err
			}
		}
	}
	for _, s := range sources {
		v := remain[s.ID]
		if err := allocate(s, "wip", v.g, v.n); err != nil {
			return nil, err
		}
	}
	return coverage, nil
}

func (r Repository) validatePlanMaterialTransferWIPTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail) error {
	if detail.SourceType != planMaterialTransferSource {
		return nil
	}
	coverage, err := r.planCurrentWIPCoverageTx(ctx, tx, detail.SourceID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT l.component_source_id,s.required_g,s.required_units,SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),SUM(l.qty_units)
 FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id AND s.production_plan_id=l.production_plan_id
 WHERE l.stock_entry_id=$1 AND l.production_plan_id=$2 GROUP BY l.component_source_id,s.required_g,s.required_units`, r.schema), detail.ID, detail.SourceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, requiredG, requiredN, g, n int64
		if err := rows.Scan(&id, &requiredG, &requiredN, &g, &n); err != nil {
			return err
		}
		wip := coverage[id]
		if g > requiredG-wip.g || n > requiredN-wip.n {
			return fmt.Errorf("WIP库存已变化，调拨数量超过当前缺料，请取消单据并重新核对")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("生产计划物料关联已变化，请重新核对")
	}
	return nil
}
