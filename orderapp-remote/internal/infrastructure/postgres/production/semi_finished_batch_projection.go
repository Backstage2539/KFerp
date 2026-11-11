package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

// Pools are ordered by selection and with WIP first. Each selected batch has a
// shared plan-wide budget even when it exists in more than one location.
type semiFinishedBatchPool struct {
	materialID, batchID, owner                           int64
	batchCode, warehouse, warehouseName, ownerName       string
	selectedG, selectedUnits, availableG, availableUnits int64
}

func projectSemiFinishedBatchSources(sources []app.ProductionPlanComponentSource, pools []semiFinishedBatchPool) {
	selected := map[int64]bool{}
	for _, p := range pools {
		selected[p.materialID] = true
	}
	usedG, usedUnits := map[int64]int64{}, map[int64]int64{}
	for i := range sources {
		s := &sources[i]
		if s.ComponentType != "material" || !selected[s.ComponentID] {
			continue
		}
		s.Allocations = []app.ProductionPlanSourceAllocation{}
		s.ShortageG, s.ShortageUnits = s.RequiredG, s.RequiredUnits
		s.WIPCoveredG, s.WIPCoveredUnits, s.TransferG, s.TransferUnits = 0, 0, 0, 0
		s.AllocationMode = "semi_finished_selected"
		for j := range pools {
			p := &pools[j]
			if p.materialID != s.ComponentID || p.owner != s.SourceOwnerCustomerID {
				continue
			}
			g := minInt64(s.ShortageG, minInt64(p.availableG, nonnegativeQuantity(p.selectedG-usedG[p.batchID])))
			n := minInt64(s.ShortageUnits, minInt64(p.availableUnits, nonnegativeQuantity(p.selectedUnits-usedUnits[p.batchID])))
			if g <= 0 && n <= 0 {
				continue
			}
			a := app.ProductionPlanSourceAllocation{Warehouse: p.warehouse, WarehouseName: p.warehouseName, OwnerCustomerID: p.owner, OwnerName: p.ownerName, QtyG: g, QtyUnits: n, Batches: []app.ProductionPlanSourceBatch{{BatchID: p.batchID, BatchCode: p.batchCode, QtyG: g, QtyUnits: n}}}
			s.Allocations = append(s.Allocations, a)
			s.ShortageG -= g
			s.ShortageUnits -= n
			p.availableG -= g
			p.availableUnits -= n
			usedG[p.batchID] += g
			usedUnits[p.batchID] += n
			if p.warehouse == "wip" {
				s.WIPCoveredG += g
				s.WIPCoveredUnits += n
			} else {
				s.TransferG += g
				s.TransferUnits += n
			}
		}
		s.Selected = s.ShortageG == 0 && s.ShortageUnits == 0
		s.DemandG, s.DemandUnits = s.RequiredG+s.UpstreamG, s.RequiredUnits+s.UpstreamUnits
		s.PreparationStatus = "ready"
		if s.UpstreamG > 0 || s.UpstreamUnits > 0 {
			s.PreparationStatus = "awaiting_production"
		}
		if s.TransferG > 0 || s.TransferUnits > 0 {
			s.PreparationStatus = "awaiting_transfer"
		}
		if !s.Selected {
			s.PreparationStatus = "shortage"
			s.AdjustmentMessage = "所选半成品批次可用量已变化，请重新选择"
		}
	}
}

func applySelectedSemiFinishedBatchSourcesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, sources []app.ProductionPlanComponentSource) error {
	if planID <= 0 {
		// Preview has no persisted plan or selected batch intent.
		return nil
	}
	exists, err := schemaColumnExistsTx(ctx, tx, schema, "production_plans", "semi_finished_stock_policy")
	if err != nil || !exists {
		return err
	}
	var policy string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(semi_finished_stock_policy,'') FROM %s.production_plans WHERE id=$1`, schema), planID).Scan(&policy); err != nil {
		return err
	}
	if policy != "manual" {
		return nil
	}
	normalized, err := normalizedSelectedSemiFinishedTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	totalsG, totalsN := map[int64]int64{}, map[int64]int64{}
	for _, item := range normalized {
		totalsG[item.batchID] += item.qtyG
		totalsN[item.batchID] += item.qtyUnits
	}
	pools := []semiFinishedBatchPool{}
	for _, item := range normalized {
		wh := item.warehouse
		p := semiFinishedBatchPool{materialID: item.materialID, batchID: item.batchID, batchCode: item.batchCode, owner: item.owner, warehouse: wh, warehouseName: wh, selectedG: totalsG[item.batchID], selectedUnits: totalsN[item.batchID]}
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(w.name,$3), COALESCE(c.name,'工厂'),
    CASE WHEN b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject') AND b.material_id=$2 THEN GREATEST(0,COALESCE(l.qty_g,0)-COALESCE(r.g,0)) ELSE 0 END,
    CASE WHEN b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject') AND b.material_id=$2 THEN GREATEST(0,COALESCE(l.qty_units,0)-COALESCE(r.n,0)) ELSE 0 END
   FROM %[1]s.material_batches b LEFT JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id AND l.warehouse=$3
   LEFT JOIN %[1]s.warehouses w ON w.code=$3 LEFT JOIN %[1]s.customers c ON c.id=b.owner_customer_id
   LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse=$3 AND status='reserved') r ON true WHERE b.id=$1`, schema), item.batchID, item.materialID, wh).Scan(&p.warehouseName, &p.ownerName, &p.availableG, &p.availableUnits)
		if err == pgx.ErrNoRows {
			err = nil
		}
		if err != nil {
			return err
		}
		p.availableG = minInt64(p.availableG, item.qtyG)
		p.availableUnits = minInt64(p.availableUnits, item.qtyUnits)
		pools = append(pools, p)
	}
	projectSemiFinishedBatchSources(sources, pools)
	return nil
}

type semiFinishedNormalizedSelection struct {
	materialID, batchID, owner, qtyG, qtyUnits int64
	batchCode, warehouse                       string
}

// Normalize only transfers made for this plan after each selection's baseline.
// Historical warehouse moves are never attributed to a later selection.
func normalizedSelectedSemiFinishedTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]semiFinishedNormalizedSelection, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT s.material_id,s.batch_id,COALESCE(b.batch_code,''),COALESCE(b.owner_customer_id,0),s.warehouse,s.qty_g,s.qty_units,s.transfer_baseline_g,s.transfer_baseline_units
 FROM %[1]s.production_plan_semi_finished_selections s LEFT JOIN %[1]s.material_batches b ON b.id=s.batch_id WHERE s.production_plan_id=$1 ORDER BY s.material_id,s.batch_id,s.warehouse`, schema), planID)
	if err != nil {
		return nil, err
	}
	type selection struct {
		semiFinishedNormalizedSelection
		baselineG, baselineN int64
	}
	selections := []selection{}
	for rows.Next() {
		var x selection
		if err = rows.Scan(&x.materialID, &x.batchID, &x.batchCode, &x.owner, &x.warehouse, &x.qtyG, &x.qtyUnits, &x.baselineG, &x.baselineN); err != nil {
			rows.Close()
			return nil, err
		}
		selections = append(selections, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []semiFinishedNormalizedSelection{}
	wip := map[int64]semiFinishedNormalizedSelection{}
	for _, x := range selections {
		base := semiFinishedNormalizedSelection{materialID: x.materialID, batchID: x.batchID, batchCode: x.batchCode, owner: x.owner, warehouse: "wip"}
		w, ok := wip[x.batchID]
		if !ok {
			w = base
		}
		if x.warehouse == "wip" {
			w.qtyG += x.qtyG
			w.qtyUnits += x.qtyUnits
			wip[x.batchID] = w
			continue
		}
		var movedG, movedN int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0),COALESCE(SUM(i.qty_units),0) FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id AND e.status='submitted' JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id AND i.stock_entry_id=e.id WHERE l.production_plan_id=$1 AND i.item_type='material' AND i.material_id=$2 AND i.batch_code=$3 AND i.from_warehouse=$4 AND i.owner_customer_id=$5`, schema), planID, x.materialID, x.batchCode, x.warehouse, x.owner).Scan(&movedG, &movedN)
		if err != nil {
			return nil, err
		}
		movedG = minInt64(x.qtyG, nonnegativeQuantity(movedG-x.baselineG))
		movedN = minInt64(x.qtyUnits, nonnegativeQuantity(movedN-x.baselineN))
		w.qtyG += movedG
		w.qtyUnits += movedN
		wip[x.batchID] = w
		x.qtyG -= movedG
		x.qtyUnits -= movedN
		if x.qtyG > 0 || x.qtyUnits > 0 {
			out = append(out, x.semiFinishedNormalizedSelection)
		}
	}
	result := make([]semiFinishedNormalizedSelection, 0, len(wip)+len(out))
	// Follow deterministic batch order without iterating the map.
	seen := map[int64]bool{}
	for _, x := range selections {
		if !seen[x.batchID] {
			seen[x.batchID] = true
			w := wip[x.batchID]
			if w.qtyG > 0 || w.qtyUnits > 0 {
				result = append(result, w)
			}
		}
	}
	result = append(result, out...)
	return result, nil
}

func validateSelectedSemiFinishedTransfer(source app.ProductionPlanComponentSource, warehouse string, owner int64, code string, qtyG, qtyUnits, alreadyG, alreadyUnits int64) error {
	if source.AllocationMode != "semi_finished_selected" {
		return nil
	}
	var allowedG, allowedUnits int64
	for _, a := range source.Allocations {
		if a.Warehouse != warehouse || a.OwnerCustomerID != owner {
			continue
		}
		for _, b := range a.Batches {
			if b.BatchCode == code {
				allowedG += b.QtyG
				allowedUnits += b.QtyUnits
			}
		}
	}
	if qtyG+alreadyG > allowedG || qtyUnits+alreadyUnits > allowedUnits || (allowedG == 0 && allowedUnits == 0) {
		return fmt.Errorf("调拨批次或数量与计划选择的半成品不一致，请重新核对")
	}
	return nil
}

func validateSelectedReservationAllocation(a app.ProductionPlanSourceAllocation) error {
	var g, n int64
	for _, b := range a.Batches {
		if b.BatchID <= 0 || b.QtyG < 0 || b.QtyUnits < 0 {
			return fmt.Errorf("半成品所选批次无效，请重新核对")
		}
		g += b.QtyG
		n += b.QtyUnits
	}
	if g != a.QtyG || n != a.QtyUnits {
		return fmt.Errorf("半成品所选批次数量不一致，请重新核对")
	}
	return nil
}

func hasSelectedSemiFinishedBatchesTx(ctx context.Context, tx pgx.Tx, schema string, planID, materialID int64) (bool, error) {
	exists, err := schemaColumnExistsTx(ctx, tx, schema, "production_plans", "semi_finished_stock_policy")
	if err != nil || !exists {
		return false, err
	}
	var selected bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_plans p JOIN %[1]s.production_plan_semi_finished_selections s ON s.production_plan_id=p.id WHERE p.id=$1 AND p.semi_finished_stock_policy='manual' AND s.material_id=$2)`, schema), planID, materialID).Scan(&selected)
	return selected, err
}

func bindSelectedSemiFinishedReservationTx(ctx context.Context, tx pgx.Tx, schema string, reservationID, workOrderID, materialID int64, a app.ProductionPlanSourceAllocation) error {
	if err := validateSelectedReservationAllocation(a); err != nil {
		return err
	}
	for _, batch := range a.Batches {
		var code string
		var g, n int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.batch_code,GREATEST(0,l.qty_g-COALESCE(r.g,0)),GREATEST(0,l.qty_units-COALESCE(r.n,0))
   FROM %[1]s.material_batches b JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id AND l.warehouse=$3
   LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse=$3 AND status='reserved' AND reservation_id<>$5) r ON true
   WHERE b.id=$1 AND b.material_id=$2 AND COALESCE(b.owner_customer_id,0)=$4 AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject') FOR UPDATE OF b,l`, schema), batch.BatchID, materialID, a.Warehouse, a.OwnerCustomerID, reservationID).Scan(&code, &g, &n)
		if err == pgx.ErrNoRows {
			return fmt.Errorf("所选半成品批次不可用，请重新选择")
		}
		if err != nil {
			return err
		}
		if code != batch.BatchCode || g < batch.QtyG || n < batch.QtyUnits {
			return fmt.Errorf("所选半成品批次库存不足，请重新选择")
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,stock_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,reserved_units,status,created_at,updated_at) VALUES($1,$2,$3,'material',$3,$4,0,$5,$6,$7,$8,$9,'reserved',now(),now()) ON CONFLICT(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse) DO UPDATE SET reserved_g=excluded.reserved_g,reserved_units=excluded.reserved_units,owner_customer_id=excluded.owner_customer_id,status='reserved',updated_at=now()`, schema), reservationID, workOrderID, materialID, batch.BatchID, code, a.Warehouse, a.OwnerCustomerID, batch.QtyG, batch.QtyUnits)
		if err != nil {
			return err
		}
	}
	return nil
}
