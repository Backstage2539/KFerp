package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/stock"
)

// This capability is installed only by the production completion transaction.
// It is deliberately not a JSON flag: ordinary stock APIs cannot bypass the
// frozen plan's checks by supplying a source_type or a user-controlled field.
type operationConsumptionContextKey struct{}

func isOperationConsumption(ctx context.Context, workOrderID int64) bool {
	id, _ := ctx.Value(operationConsumptionContextKey{}).(int64)
	return id > 0 && id == workOrderID
}

func (r Repository) ConsumeOperationMaterialsTx(ctx context.Context, tx pgx.Tx, cmd app.StockDocumentCommand) (app.StockDocumentDetail, error) {
	if cmd.WorkOrderID <= 0 || cmd.JobCardID <= 0 || cmd.SourceID <= 0 || cmd.SourceType != "operation_completion" || cmd.Purpose != app.PurposeMaterialConsumption || cmd.IsReturn {
		return app.StockDocumentDetail{}, fmt.Errorf("invalid operation consumption")
	}
	var authorized bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_operation_events WHERE id=$1 AND work_order_id=$2 AND job_card_id=$3 AND status='posting')`, r.schema), cmd.SourceID, cmd.WorkOrderID, cmd.JobCardID).Scan(&authorized); err != nil {
		return app.StockDocumentDetail{}, err
	}
	if !authorized {
		return app.StockDocumentDetail{}, fmt.Errorf("operation completion event is not active")
	}
	ctx = context.WithValue(ctx, operationConsumptionContextKey{}, cmd.WorkOrderID)
	return r.CreateAndSubmitStockDocumentTx(ctx, tx, cmd)
}

func recordOperationFreeConsumptionTx(ctx context.Context, tx pgx.Tx, schema string, detail app.StockDocumentDetail, item app.StockDocumentItemRow, batchID, g, n int64) error {
	var reservationID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=$2 AND source_owner_customer_id=$3 AND status IN('reserved','consumed') ORDER BY id LIMIT 1 FOR UPDATE`, schema), detail.WorkOrderID, item.MaterialID, item.OwnerCustomerID).Scan(&reservationID); err != nil {
		return fmt.Errorf("物料缺少冻结来源身份：%w", err)
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,reserved_units,consumed_g,consumed_units,status)
 SELECT $1,$2,$3,'material',$3,id,batch_code,$5,$6,$7,$8,$7,$8,'consumed' FROM %[1]s.material_batches WHERE id=$4
 ON CONFLICT(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse) DO UPDATE SET
 reserved_g=work_order_material_reservation_batches.reserved_g+EXCLUDED.reserved_g,reserved_units=work_order_material_reservation_batches.reserved_units+EXCLUDED.reserved_units,
 consumed_g=work_order_material_reservation_batches.consumed_g+EXCLUDED.consumed_g,consumed_units=work_order_material_reservation_batches.consumed_units+EXCLUDED.consumed_units,updated_at=now()`, schema), reservationID, detail.WorkOrderID, item.MaterialID, batchID, item.FromWarehouse, item.OwnerCustomerID, g, n)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET reserved_g=reserved_g+$2,reserved_units=reserved_units+$3,updated_at=now() WHERE id=$1`, schema), reservationID, g, n)
	return err
}
