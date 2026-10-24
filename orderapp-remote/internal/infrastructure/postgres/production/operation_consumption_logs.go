package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

func (r Repository) recordOperationConsumptionLogsTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, entryID int64, actor string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.material_consumption_logs(running_item_id,batch_id,product_id,product_name,spec_g,material_id,material_name,unit,deduct_g,deduct_units,before_g,after_g,before_units,after_units,operator,material_batch_id,material_batch_code)
 SELECT $1,$2,$3,$4,$5,l.item_id,l.item_name,CASE WHEN l.qty_change_g<0 THEN 'g' ELSE 'unit' END,-l.qty_change_g,-l.qty_change_units,l.qty_before_g,l.qty_after_g,l.qty_before_units,l.qty_after_units,$6,COALESCE(b.id,0),l.source_batch_code FROM %[1]s.stock_ledger_entries l LEFT JOIN %[1]s.material_batches b ON l.item_type='material' AND b.batch_code=l.source_batch_code AND b.material_id=l.item_id WHERE l.source_doc_type='stock_entry' AND l.source_doc_id=$7 AND (l.qty_change_g<0 OR l.qty_change_units<0)`, r.schema), wo.RunningItemID, wo.BatchID, wo.ProductID, wo.ProductName, wo.SpecG, actor, entryID)
	return err
}
