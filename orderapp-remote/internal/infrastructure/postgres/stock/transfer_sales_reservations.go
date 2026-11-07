package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Sales batch allocations include quantities already shipped. Aggregate each
// order once before subtracting its outbound deduction, preserving reservations
// from multiple allocations and customers without double-counting shipments.
func transferSalesReservedTx(ctx context.Context, tx pgx.Tx, schema string, batchID int64) (grams, units, weight int64, err error) {
	has, e := stockSchemaRelationExistsTx(ctx, tx, schema, "order_stock_batch_allocations")
	if e != nil || !has {
		return 0, 0, 0, e
	}
	hasD, e := stockSchemaRelationExistsTx(ctx, tx, schema, "order_stock_deductions")
	if e != nil {
		return 0, 0, 0, e
	}
	deductions := `SELECT 0::bigint order_id,0::numeric g,0::numeric n WHERE false`
	if hasD {
		deductions = fmt.Sprintf(`SELECT d.order_id,sum(d.deducted_g) g,sum(d.deducted_units) n FROM %[1]s.order_stock_deductions d JOIN b ON d.product_id=b.item_id AND d.bom_spec_id=b.bom_spec_id AND (d.batch_id=b.id OR d.batch_code=b.batch_code) GROUP BY d.order_id`, schema)
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`WITH b AS(SELECT * FROM %[1]s.stock_batches WHERE id=$1),a AS(
 SELECT a.order_id,sum(a.allocated_g) g,sum(a.allocated_units) n FROM %[1]s.order_stock_batch_allocations a JOIN b ON a.product_id=b.item_id AND a.bom_spec_id=b.bom_spec_id AND (a.batch_id=b.id OR a.batch_code=b.batch_code) GROUP BY a.order_id),d AS(%[2]s),remaining AS(
 SELECT GREATEST(0,a.g-COALESCE(d.g,0)) g,GREATEST(0,a.n-COALESCE(d.n,0)) n FROM a LEFT JOIN d USING(order_id))
 SELECT COALESCE(sum(g),0)::bigint,COALESCE(sum(n),0)::bigint,COALESCE(sum(GREATEST(g,n*(SELECT spec_g FROM b))),0)::bigint FROM remaining`, schema, deductions), batchID).Scan(&grams, &units, &weight)
	return
}
