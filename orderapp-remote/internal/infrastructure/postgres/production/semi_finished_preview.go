package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
)

// previewSemiFinishedCapacity computes task changes from the entire graph,
// including upstream tasks introduced or eliminated by a different choice.
func previewSemiFinishedCapacity(ctx context.Context, tx pgx.Tx, schema string, plan app.ProductionPlanDetail, expansion manufacturingExpansion, oldUpstream map[string]app.ProductionPlanItem) ([]app.SemiFinishedStockChange, error) {
	var oldGraph domain.ManufacturingPlan
	if len(plan.SupplyPlan) > 0 {
		if err := json.Unmarshal(plan.SupplyPlan, &oldGraph); err != nil {
			return nil, err
		}
	}
	oldNodes, newNodes := map[string]domain.ManufacturingPlanNode{}, map[string]domain.ManufacturingPlanNode{}
	for _, n := range oldGraph.Nodes {
		if _, root := expansion.Roots[n.Item.Key()]; !root {
			oldNodes[n.Item.Key()] = n
		}
	}
	for _, n := range expansion.Plan.Nodes {
		if _, root := expansion.Roots[n.Item.Key()]; !root {
			newNodes[n.Item.Key()] = n
		}
	}
	all := map[string]bool{}
	for k := range oldNodes {
		all[k] = true
	}
	for k := range newNodes {
		all[k] = true
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := []app.SemiFinishedStockChange{}
	for _, key := range keys {
		before, after := oldNodes[key], newNodes[key]
		basis, manufacturable := expansion.Bases[key]
		oldItem, existed := oldUpstream[key]
		activeBefore := existed
		archivedSplits := []app.ProductionPlanOperationSplit{}
		if !existed && after.ShortageQty > 0 {
			var err error
			oldItem, archivedSplits, err = loadArchivedSemiFinishedCapacityTx(ctx, tx, schema, plan.ID, key)
			if err != nil {
				return nil, err
			}
			existed = oldItem.OutputQty > 0 && len(archivedSplits) > 0
		}
		if !manufacturable && !existed {
			continue
		}
		if math.Abs(before.ShortageQty-after.ShortageQty) < 0.000001 {
			continue
		}
		name, unit := basis.OutputName, basis.InventoryUnit
		if existed {
			name, unit = oldItem.OutputName, oldItem.OutputUnit
		}
		if name == "" {
			name = after.Item.Name
		}
		if unit == "" {
			unit = after.Item.Unit
		}
		change := app.SemiFinishedStockChange{Name: name, Unit: unit, BeforeQty: before.ShortageQty, AfterQty: after.ShortageQty, Operations: []app.SemiFinishedCapacityChange{}}
		var next app.ProductionPlanItem
		if after.ShortageQty > 0 {
			var err error
			next, err = insertManufacturingOutputProductionPlanItemTx(ctx, tx, schema, 0, basis, after.ShortageQty, manufacturingUpstreamDemandRoots(expansion, key))
			if err != nil {
				return nil, err
			}
			if existed {
				next.ID = oldItem.ID
			}
		}
		if (existed && oldItem.PlannedG > 0) || next.PlannedG > 0 {
			oldInput, newInput := float64(0), float64(next.PlannedG)/1000
			if activeBefore {
				oldInput = float64(oldItem.PlannedG) / 1000
			}
			change.BeforeInputQty, change.AfterInputQty = &oldInput, &newInput
			change.InputUnit = "kg"
		}
		previousSplits := []app.ProductionPlanOperationSplit{}
		for _, split := range plan.OperationSplits {
			if existed && split.ProductionPlanItemID == oldItem.ID {
				previousSplits = append(previousSplits, split)
			}
		}
		if !activeBefore {
			previousSplits = archivedSplits
		}
		var newSplits []app.ProductionPlanOperationSplit
		if existed {
			var err error
			newSplits, err = replanSemiFinishedCapacity(oldItem, next, previousSplits)
			if err != nil {
				return nil, err
			}
		}
		newByID := map[int64]app.ProductionPlanOperationSplit{}
		for _, split := range newSplits {
			newByID[split.ID] = split
		}
		for _, old := range previousSplits {
			row := app.SemiFinishedCapacityChange{Name: old.Operation, Workstation: old.Workstation, Batches: fmt.Sprintf("%d → 0", old.PlannedBatchCount)}
			previousBatches := old.PlannedBatchCount
			if !activeBefore {
				previousBatches = 0
			}
			bm, bc := old.PlannedMinutes, old.PlannedOperationCost
			if !activeBefore {
				bm, bc = 0, 0
			}
			row.BeforeMinutes, row.BeforeCost = &bm, &bc
			if n, ok := newByID[old.ID]; ok {
				planItemID := next.ID
				if !activeBefore {
					planItemID = 0
				}
				prepared, prepareErr := prepareOperationSplitForSaveTx(ctx, tx, schema, n, plan.ID, planItemID, next.SpecG, next.SalesSpecCount, productionPlanItemOutputTargetG(next))
				if prepareErr != nil {
					if !unavailableSemiFinishedCapacity(prepareErr) {
						return nil, prepareErr
					}
					row.Workstation = "原工位不可用，待安排"
					row.Batches = "待安排"
					change.Operations = append(change.Operations, row)
					continue
				}
				n = prepared
				am, ac := n.PlannedMinutes, n.PlannedOperationCost
				row.AfterMinutes, row.AfterCost = &am, &ac
				row.Batches = fmt.Sprintf("%d → %d", previousBatches, n.PlannedBatchCount)
			} else {
				zero := 0
				zc := 0.0
				row.AfterMinutes, row.AfterCost = &zero, &zc
			}
			change.Operations = append(change.Operations, row)
		}
		if len(previousSplits) == 0 {
			target := next
			if after.ShortageQty <= 0 {
				target = oldItem
			}
			for _, op := range productionPlanPreviewOperations(target, nil) {
				change.Operations = append(change.Operations, app.SemiFinishedCapacityChange{Name: op.Operation, Workstation: "待安排", Batches: "待安排"})
			}
		}
		result = append(result, change)
	}
	return result, nil
}
