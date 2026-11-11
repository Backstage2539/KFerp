package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

type directTransferKey struct {
	source    int64
	warehouse string
	owner     int64
	batch     string
}
type directPendingLine struct {
	document     int64
	key          directTransferKey
	grams, units int64
}

// Existing linked drafts may be posted only when every line is part of the
// currently selected semi-finished transfer and does not exceed that selection.
func reconcileDirectTransferDrafts(desired []app.PlanMaterialTransferItem, selected map[int64]bool, lines []directPendingLine) ([]app.PlanMaterialTransferItem, []int64, error) {
	amounts := map[directTransferKey][2]int64{}
	for _, item := range desired {
		for _, b := range item.Batches {
			k := directTransferKey{item.SourceID, item.Warehouse, item.OwnerCustomerID, b.BatchCode}
			v := amounts[k]
			v[0] += b.QtyG
			v[1] += b.QtyUnits
			amounts[k] = v
		}
	}
	docs := map[int64][]directPendingLine{}
	order := []int64{}
	for _, line := range lines {
		if _, ok := docs[line.document]; !ok {
			order = append(order, line.document)
		}
		docs[line.document] = append(docs[line.document], line)
	}
	matched := []int64{}
	for _, id := range order {
		relevant := false
		for _, row := range docs[id] {
			if selected[row.key.source] {
				relevant = true
			}
		}
		if !relevant {
			continue
		}
		for _, row := range docs[id] {
			v, ok := amounts[row.key]
			if !ok || !selected[row.key.source] || row.grams < 0 || row.units < 0 || row.grams > v[0] || row.units > v[1] {
				return nil, nil, fmt.Errorf("待确认调拨单与已选半成品不一致，请先处理单据")
			}
			v[0] -= row.grams
			v[1] -= row.units
			amounts[row.key] = v
		}
		matched = append(matched, id)
	}
	remaining := []app.PlanMaterialTransferItem{}
	for _, item := range desired {
		x := item
		x.Batches = nil
		for _, b := range item.Batches {
			k := directTransferKey{item.SourceID, item.Warehouse, item.OwnerCustomerID, b.BatchCode}
			v := amounts[k]
			if v[0] > 0 || v[1] > 0 {
				b.QtyG = v[0]
				b.QtyUnits = v[1]
				x.Batches = append(x.Batches, b)
				amounts[k] = [2]int64{}
			}
		}
		if len(x.Batches) > 0 {
			remaining = append(remaining, x)
		}
	}
	return remaining, matched, nil
}

func loadDirectTransferDraftLines(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]directPendingLine, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id, COALESCE(l.component_source_id,0), i.from_warehouse, i.owner_customer_id, i.batch_code, i.qty_g, i.qty_units
 FROM %[1]s.stock_entries e JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=e.id
 LEFT JOIN %[1]s.production_plan_material_transfer_links l ON l.stock_entry_id=e.id AND l.stock_entry_item_id=i.id AND l.production_plan_id=$1
 WHERE e.status='draft' AND EXISTS (SELECT 1 FROM %[1]s.production_plan_material_transfer_links x WHERE x.production_plan_id=$1 AND x.stock_entry_id=e.id)
 ORDER BY e.id,i.id`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := []directPendingLine{}
	for rows.Next() {
		var x directPendingLine
		if err = rows.Scan(&x.document, &x.key.source, &x.key.warehouse, &x.key.owner, &x.key.batch, &x.grams, &x.units); err != nil {
			return nil, err
		}
		lines = append(lines, x)
	}
	return lines, rows.Err()
}

func (r Repository) TransferSemiFinishedToWIP(ctx context.Context, c app.SemiFinishedTransferCommand) (app.PlanMaterialTransferResult, error) {
	return r.executePlanMaterialTransfers(ctx, app.PlanMaterialTransferCommand{ProductionPlanID: c.PlanID, DraftToken: c.DraftToken, IdempotencyKey: "wip:" + c.IdempotencyKey, Operator: c.Operator}, false, true)
}
