package production

import (
	"context"
	"errors"
	"fmt"
	"sort"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type customerProcessingOutputReservationRow struct {
	ID                      int64
	CustomerID              int64
	RequestID               int64
	RequestItemID           int64
	OrderID                 int64
	OrderItemID             int64
	ProcessingRequestItemID int64
	ProductID               int64
	BomSpecID               int64
	BomVariantID            int64
	SpecG                   int64
	ReservedQty             int64
	ConvertedQty            int64
	ReleasedQty             int64
}

func allocateCustomerProcessingOutputReservationsTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	runningItemID, productID, bomSpecID, bomVariantID, specG, producedQty int64,
	batchCode, warehouse, operator string,
) error {
	if producedQty <= 0 || runningItemID <= 0 {
		return nil
	}
	hasReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_output_reservations", "id")
	if err != nil || !hasReservations {
		return err
	}
	var processingRequestItemID, customerID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT d.request_item_id,d.customer_id
		FROM %s.customer_processing_production_demands d
		WHERE d.linked_running_item_id=$1 AND d.product_id=$2
		  AND COALESCE(d.bom_spec_id,0)=$3 AND d.spec_g=$4
		  AND d.request_item_id>0
		ORDER BY d.id LIMIT 1
	`, schema), runningItemID, productID, bomSpecID, specG).Scan(&processingRequestItemID, &customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var batchID, batchOwnerCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,COALESCE(owner_customer_id,0)
		FROM %s.stock_batches
		WHERE batch_code=$1 AND item_type='finished_product' AND item_id=$2
		  AND COALESCE(bom_spec_id,0)=$3 AND spec_g=$4
		ORDER BY id DESC LIMIT 1
	`, schema), batchCode, productID, bomSpecID, specG).Scan(&batchID, &batchOwnerCustomerID); err != nil {
		return err
	}
	if customerID <= 0 || batchOwnerCustomerID != customerID {
		return fmt.Errorf("customer processing output owner mismatch")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,customer_id,request_id,request_item_id,order_id,order_item_id,
		       processing_request_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,
		       reserved_qty,converted_qty,released_qty
		FROM %s.customer_processing_output_reservations
		WHERE processing_request_item_id=$1 AND customer_id=$2
		  AND product_id=$3 AND bom_spec_id=$4 AND spec_g=$5
		  AND status IN ('reserved','partially_converted')
		  AND reserved_qty>converted_qty+released_qty
		ORDER BY created_at,id
		FOR UPDATE
	`, schema), processingRequestItemID, customerID, productID, bomSpecID, specG)
	if err != nil {
		return err
	}
	reservations := make([]customerProcessingOutputReservationRow, 0)
	for rows.Next() {
		var row customerProcessingOutputReservationRow
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.RequestID, &row.RequestItemID, &row.OrderID, &row.OrderItemID,
			&row.ProcessingRequestItemID, &row.ProductID, &row.BomSpecID, &row.BomVariantID, &row.SpecG,
			&row.ReservedQty, &row.ConvertedQty, &row.ReleasedQty); err != nil {
			rows.Close()
			return err
		}
		reservations = append(reservations, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	remaining := producedQty
	orderNos := map[string]bool{}
	for _, reservation := range reservations {
		if remaining <= 0 {
			break
		}
		outstanding := reservation.ReservedQty - reservation.ConvertedQty - reservation.ReleasedQty
		take := minInt64(outstanding, remaining)
		if take <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_output_conversions(
				output_reservation_id,processing_request_item_id,order_id,order_item_id,
				stock_batch_id,batch_code,warehouse,converted_qty
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT(output_reservation_id,stock_batch_id,batch_code)
			DO UPDATE SET converted_qty=%s.customer_processing_output_conversions.converted_qty+EXCLUDED.converted_qty
		`, schema, schema), reservation.ID, processingRequestItemID, reservation.OrderID, reservation.OrderItemID,
			batchID, batchCode, warehouse, take); err != nil {
			return err
		}
		newConverted := reservation.ConvertedQty + take
		status := "partially_converted"
		if newConverted+reservation.ReleasedQty >= reservation.ReservedQty {
			status = "converted"
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_processing_output_reservations
			SET converted_qty=$2,converted_stock_batch_id=$3,converted_batch_code=$4,status=$5,updated_at=now()
			WHERE id=$1
		`, schema), reservation.ID, newConverted, batchID, batchCode, status); err != nil {
			return err
		}
		allocatedG, allocatedUnits := take*specG, int64(0)
		if bomSpecID > 0 {
			allocatedG, allocatedUnits = 0, take
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_stock_batch_allocations(
				order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,
				batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$7,$11,$12,$13,$14)
		`, schema), reservation.OrderID, reservation.OrderItemID, productID, bomSpecID, bomVariantID,
			specG, allocatedG, allocatedUnits, batchID, batchCode, allocatedUnits, warehouse, reservation.RequestID, operator); err != nil {
			return err
		}
		var requestOrderID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_direct_ship_request_orders WHERE request_id=$1 AND order_id=$2`, schema), reservation.RequestID, reservation.OrderID).Scan(&requestOrderID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_direct_ship_request_allocations(
				request_id,request_item_id,request_order_id,order_id,order_item_id,
				product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,
				allocated_qty,allocated_units,allocated_g,status
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,'reserved')
		`, schema), reservation.RequestID, reservation.RequestItemID, requestOrderID, reservation.OrderID, reservation.OrderItemID,
			productID, bomSpecID, bomVariantID, specG, warehouse, batchID, batchCode, take, allocatedUnits, allocatedG); err != nil {
			return err
		}
		var orderNo string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT order_no FROM %s.orders WHERE id=$1`, schema), reservation.OrderID).Scan(&orderNo); err != nil {
			return err
		}
		orderNos[orderNo] = true
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "customer_processing_output_reservation", &reservation.ID, "processing_output_convert", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr(status), postgresinfra.AuditMeta{
			"customer_id": customerID, "processing_request_item_id": processingRequestItemID,
			"order_id": reservation.OrderID, "order_item_id": reservation.OrderItemID,
			"stock_batch_id": batchID, "batch_code": batchCode, "converted_qty": take,
		}); err != nil {
			return err
		}
		remaining -= take
	}
	keys := make([]string, 0, len(orderNos))
	for orderNo := range orderNos {
		keys = append(keys, orderNo)
	}
	sort.Strings(keys)
	for _, orderNo := range keys {
		if _, _, err := completeOrderIfAllRunningDone(ctx, tx, schema, orderNo); err != nil {
			return err
		}
	}
	return nil
}

func markCustomerProcessingOutputShortfallsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, operator string) error {
	hasReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_output_reservations", "id")
	if err != nil || !hasReservations {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_output_reservations r
		SET shortfall_qty=GREATEST(0,r.reserved_qty-r.converted_qty-r.released_qty),
		    shortfall_reason='完工数量不足承诺',status='shortfall',updated_at=now()
		FROM %s.customer_processing_production_demands d
		WHERE d.linked_running_item_id=$1 AND d.request_item_id=r.processing_request_item_id
		  AND r.status IN ('reserved','partially_converted')
		  AND r.reserved_qty>r.converted_qty+r.released_qty
		RETURNING r.id,r.customer_id,r.order_id,r.shortfall_qty
	`, schema, schema), runningItemID)
	if err != nil {
		return err
	}
	type shortfallRow struct {
		reservationID int64
		customerID    int64
		orderID       int64
		qty           int64
	}
	shortfalls := make([]shortfallRow, 0)
	for rows.Next() {
		var row shortfallRow
		if err := rows.Scan(&row.reservationID, &row.customerID, &row.orderID, &row.qty); err != nil {
			rows.Close()
			return err
		}
		shortfalls = append(shortfalls, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, row := range shortfalls {
		reservationID := row.reservationID
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "customer_processing_output_reservation", &reservationID, "processing_output_shortfall", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("shortfall"), postgresinfra.AuditMeta{
			"customer_id": row.customerID, "order_id": row.orderID, "shortfall_qty": row.qty,
		}); err != nil {
			return err
		}
	}
	return nil
}

func guardCustomerProcessingPromisedOutputTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) error {
	if workOrderID <= 0 {
		return nil
	}
	hasReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_output_reservations", "id")
	if err != nil || !hasReservations {
		return err
	}
	var promised bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.work_orders wo
			JOIN %s.customer_processing_output_reservations r ON r.processing_request_item_id=wo.processing_request_item_id
			WHERE wo.id=$1 AND r.status IN ('reserved','partially_converted')
			  AND r.reserved_qty>r.converted_qty+r.released_qty
		)
	`, schema, schema), workOrderID).Scan(&promised)
	if err != nil {
		return err
	}
	if promised {
		return fmt.Errorf("该代加工工单已有订单预订产出，不能取消或缩减")
	}
	return nil
}
