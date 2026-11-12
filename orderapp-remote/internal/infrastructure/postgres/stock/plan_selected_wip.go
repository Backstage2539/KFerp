package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type selectedWIPBatch struct {
	materialID, batchID, owner                   int64
	selectedG, selectedN, availableG, availableN int64
}

// selectedPlanWIPCoverageTx mirrors the plan's frozen batch choices. It must
// never count other batches of the same material as WIP for a transfer.
func (r Repository) selectedPlanWIPCoverageTx(ctx context.Context, tx pgx.Tx, planID int64, sources []planWIPSource) (map[int64]planWIPQty, map[int64]bool, error) {
	coverage := map[int64]planWIPQty{}
	selectedSources := map[int64]bool{}
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='production_plans' AND column_name='semi_finished_stock_policy')`, r.schema).Scan(&exists)
	if err != nil || !exists {
		return coverage, selectedSources, err
	}
	var policy string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(semi_finished_stock_policy,'') FROM %s.production_plans WHERE id=$1`, r.schema), planID).Scan(&policy); err != nil {
		return nil, nil, err
	}
	if policy != "manual" {
		return coverage, selectedSources, nil
	}
	// Only inspect selected batches for material identities participating in
	// this calculation. Other plan items cannot cover these sources and an
	// unrelated invalid selection must not block the current transfer.
	materialIDs := make([]int64, 0, len(sources))
	seenMaterials := map[int64]bool{}
	for _, source := range sources {
		if source.Kind == "material" && !seenMaterials[source.ItemID] {
			seenMaterials[source.ItemID] = true
			materialIDs = append(materialIDs, source.ItemID)
		}
	}
	if len(materialIDs) == 0 {
		return coverage, selectedSources, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT s.material_id,s.batch_id,COALESCE(b.owner_customer_id,0),s.qty_g,s.qty_units,
 GREATEST(0,COALESCE(loc.qty_g,0)-COALESCE(res.g,0)),GREATEST(0,COALESCE(loc.qty_units,0)-COALESCE(res.n,0)),(b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject') AND b.material_id=s.material_id)
 FROM %[1]s.production_plan_semi_finished_selections s JOIN %[1]s.material_batches b ON b.id=s.batch_id
 LEFT JOIN %[1]s.material_batch_locations loc ON loc.material_batch_id=b.id AND loc.warehouse='wip'
 LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse='wip' AND status='reserved') res ON true
 WHERE s.production_plan_id=$1 AND s.material_id=ANY($2::bigint[]) ORDER BY s.material_id,s.batch_id,s.warehouse`, r.schema), planID, materialIDs)
	if err != nil {
		return nil, nil, err
	}
	batches := []selectedWIPBatch{}
	index := map[int64]int{}
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
	for rows.Next() {
		var material, batch, owner, g, n, availG, availN int64
		var active bool
		if err = rows.Scan(&material, &batch, &owner, &g, &n, &availG, &availN, &active); err != nil {
			rows.Close()
			return nil, nil, err
		}
		idx, ok := index[batch]
		if !ok {
			idx = len(batches)
			index[batch] = idx
			batches = append(batches, selectedWIPBatch{materialID: material, batchID: batch, owner: owner, availableG: availG, availableN: availN})
		}
		if !active {
			batches[idx].availableG = 0
			batches[idx].availableN = 0
		}
		batches[idx].selectedG += g
		batches[idx].selectedN += n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	materials := map[int64]bool{}
	for _, b := range batches {
		materials[b.materialID] = true
	}
	for _, s := range sources {
		if s.Kind != "material" || !materials[s.ItemID] {
			continue
		}
		selectedSources[s.ID] = true
		remG, remN := s.RequiredG, s.RequiredUnits
		for i := range batches {
			b := &batches[i]
			if b.materialID != s.ItemID || b.owner != s.Owner {
				continue
			}
			g := min(remG, min(b.selectedG, b.availableG))
			n := min(remN, min(b.selectedN, b.availableN))
			b.selectedG -= g
			b.selectedN -= n
			b.availableG -= g
			b.availableN -= n
			remG -= g
			remN -= n
			c := coverage[s.ID]
			coverage[s.ID] = planWIPQty{c.g + g, c.n + n}
		}
	}
	return coverage, selectedSources, nil
}
