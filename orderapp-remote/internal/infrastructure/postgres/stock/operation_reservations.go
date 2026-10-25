package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Customer processing has its own reservation projection. Count only the part
// not already protected by the same work order's generic batch reservation.
func operationOtherProcessingReservedTx(ctx context.Context, tx pgx.Tx, schema string, workID, batchID, itemID, owner int64, warehouse string, product bool) (int64, int64, error) {
	has, err := stockSchemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil || !has {
		return 0, 0, err
	}
	batchColumn, itemColumn, genericBatch := "material_batch_id", "material_id", "material_batch_id"
	if product {
		batchColumn, itemColumn, genericBatch = "finished_stock_batch_id", "component_product_id", "stock_batch_id"
	}
	var g, n int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`WITH processing AS(SELECT work_order_id,SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.customer_processing_material_reservations WHERE (%[2]s=$1 OR %[2]s=0) AND %[3]s=$2 AND work_order_id<>$3 AND source_warehouse_code=$4 AND CASE WHEN source_owner_type='customer' THEN source_customer_id ELSE 0 END=$5 AND status='reserved' GROUP BY work_order_id)
 SELECT COALESCE(SUM(GREATEST(0,p.g-COALESCE(b.g,0))),0)::bigint,COALESCE(SUM(GREATEST(0,p.n-COALESCE(b.n,0))),0)::bigint FROM processing p LEFT JOIN LATERAL(SELECT SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)) g,SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)) n FROM %[1]s.work_order_material_reservation_batches rb JOIN %[1]s.work_order_material_reservations r ON r.id=rb.reservation_id WHERE rb.work_order_id=p.work_order_id AND p.work_order_id>0 AND rb.%[4]s=$1 AND rb.warehouse=$4 AND rb.owner_customer_id=$5 AND rb.status='reserved' AND r.status='reserved') b ON true`, schema, batchColumn, itemColumn, genericBatch), batchID, itemID, workID, warehouse, owner).Scan(&g, &n)
	return g, n, err
}

// OperationOtherProcessingReservedTx exposes the reservation-aware availability
// calculation to production previews so they use the same customer-processing
// protection as the stock posting path.
func OperationOtherProcessingReservedTx(ctx context.Context, tx pgx.Tx, schema string, workID, batchID, itemID, owner int64, warehouse string, product bool) (int64, int64, error) {
	return operationOtherProcessingReservedTx(ctx, tx, schema, workID, batchID, itemID, owner, warehouse, product)
}

func consumeOperationProcessingReservationTx(ctx context.Context, tx pgx.Tx, schema string, workID, batchID, itemID, owner int64, warehouse string, g, n int64, product bool) error {
	has, err := stockSchemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil || !has {
		return err
	}
	batchColumn, itemColumn := "material_batch_id", "material_id"
	if product {
		batchColumn, itemColumn = "finished_stock_batch_id", "component_product_id"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,GREATEST(0,reserved_g-consumed_g-returned_g),GREATEST(0,reserved_units-consumed_units-returned_units) FROM %s.customer_processing_material_reservations WHERE work_order_id=$1 AND %s=$2 AND %s=$3 AND source_warehouse_code=$4 AND CASE WHEN source_owner_type='customer' THEN source_customer_id ELSE 0 END=$5 AND status='reserved' ORDER BY id FOR UPDATE`, schema, batchColumn, itemColumn), workID, batchID, itemID, warehouse, owner)
	if err != nil {
		return err
	}
	type balance struct{ id, g, n int64 }
	balances := []balance{}
	for rows.Next() {
		var b balance
		if err = rows.Scan(&b.id, &b.g, &b.n); err != nil {
			rows.Close()
			return err
		}
		balances = append(balances, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range balances {
		tg, tn := minInt64(g, b.g), minInt64(n, b.n)
		g -= tg
		n -= tn
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_processing_material_reservations SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,status=CASE WHEN consumed_g+$2+returned_g>=reserved_g AND consumed_units+$3+returned_units>=reserved_units THEN 'consumed' ELSE status END,updated_at=now() WHERE id=$1`, schema), b.id, tg, tn); err != nil {
			return err
		}
	}
	return nil
}
