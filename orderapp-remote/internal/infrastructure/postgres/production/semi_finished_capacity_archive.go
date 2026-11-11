package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	"strings"
)

func archiveSemiFinishedCapacityTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, item app.ProductionPlanItem) error {
	var policy string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT semi_finished_stock_policy FROM %s.production_plans WHERE id=$1`, schema), planID).Scan(&policy); err != nil {
		return err
	}
	if policy != "manual" {
		return nil
	}
	splits, err := loadProductionPlanOperationSplitsTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	selected := []app.ProductionPlanOperationSplit{}
	for _, split := range splits {
		if split.ProductionPlanItemID == item.ID {
			selected = append(selected, split)
		}
	}
	itemJSON, err := json.Marshal(item)
	if err != nil {
		return err
	}
	splitsJSON, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	typ, id := item.OutputType, item.OutputProductID
	if typ == "material" {
		id = item.OutputMaterialID
	}
	key := manufacturingItemIdentityKey(typ, id, item.BomSpecID)
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_plan_frozen_manufacturing_outputs(production_plan_id,item_key,item_snapshot,operation_splits_snapshot) VALUES($1,$2,$3::jsonb,$4::jsonb) ON CONFLICT(production_plan_id,item_key) DO UPDATE SET item_snapshot=EXCLUDED.item_snapshot,operation_splits_snapshot=EXCLUDED.operation_splits_snapshot`, schema), planID, key, itemJSON, splitsJSON)
	return err
}
func loadArchivedSemiFinishedCapacityTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, key string) (app.ProductionPlanItem, []app.ProductionPlanOperationSplit, error) {
	var item app.ProductionPlanItem
	var splits []app.ProductionPlanOperationSplit
	var raw, splitRaw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT item_snapshot,operation_splits_snapshot FROM %s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=$1 AND item_key=$2`, schema), planID, key).Scan(&raw, &splitRaw)
	if err == pgx.ErrNoRows {
		return item, nil, nil
	}
	if err != nil {
		return item, nil, err
	}
	if err = json.Unmarshal(raw, &item); err != nil {
		return item, nil, err
	}
	if err = json.Unmarshal(splitRaw, &splits); err != nil {
		return item, nil, err
	}
	return item, splits, nil
}

// applySemiFinishedCapacityTx validates current station availability; unavailable
// stations are omitted and the task remains visibly unassigned for review.
func applySemiFinishedCapacityTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, before, after app.ProductionPlanItem, splits []app.ProductionPlanOperationSplit) error {
	replanned, err := replanSemiFinishedCapacity(before, after, splits)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_operation_splits WHERE production_plan_id=$1 AND production_plan_item_id=$2`, schema), planID, after.ID); err != nil {
		return err
	}
	for _, split := range replanned {
		prepared, err := prepareOperationSplitForSaveTx(ctx, tx, schema, split, planID, after.ID, after.SpecG, after.SalesSpecCount, productionPlanItemOutputTargetG(after))
		if err != nil {
			if unavailableSemiFinishedCapacity(err) {
				continue
			}
			return err
		}
		if err = insertProductionPlanOperationSplitTx(ctx, tx, schema, prepared); err != nil {
			return err
		}
	}
	return nil
}

func unavailableSemiFinishedCapacity(err error) bool {
	return err == pgx.ErrNoRows || strings.Contains(err.Error(), "工位已不适用") || strings.Contains(err.Error(), "产能已停用") || strings.Contains(err.Error(), "workstation capacity not found or inactive")
}
