package messagecenter

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type ReceiptDetail struct {
	ReceiptNo       string `json:"receipt_no"`
	ReceiptDate     string `json:"receipt_date"`
	ActualCents     int64  `json:"actual_cents"`
	ReceivableCents int64  `json:"receivable_cents"`
}
type BillDetail struct {
	BillNo      string `json:"bill_no"`
	PeriodFrom  string `json:"period_from"`
	PeriodTo    string `json:"period_to"`
	AmountCents int64  `json:"amount_cents"`
}
type financeQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Settlement lines may contain order goods or a customer fee backed by an
// order or freight record. Unknown sources fail closed until mapped explicitly.
func billSourceSafeSQL(schema, billAlias string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM %[1]s.customer_settlement_lines present WHERE present.settlement_batch_id=%[2]s.id) AND NOT EXISTS (
 SELECT 1 FROM %[1]s.customer_settlement_lines l
 LEFT JOIN %[1]s.orders direct_order ON l.source_type='order' AND direct_order.id=l.source_id
 LEFT JOIN %[1]s.customer_fee_items f ON l.source_type='customer_fee_item' AND f.id=l.source_id
 LEFT JOIN %[1]s.orders fee_order ON f.source_type='order' AND fee_order.id=f.source_id
 LEFT JOIN %[1]s.freight_actual_entries fe ON f.source_type='freight_actual' AND fe.id=f.source_id
 LEFT JOIN %[1]s.freight_actual_shipments fs ON fs.id=fe.shipment_id
 LEFT JOIN %[1]s.orders freight_order ON freight_order.id=fs.order_id
 WHERE l.settlement_batch_id=%[2]s.id AND (
 l.customer_id IS DISTINCT FROM $2
 OR (l.source_type='order' AND (direct_order.id IS NULL OR direct_order.customer_id IS DISTINCT FROM $2))
 OR (l.source_type='customer_fee_item' AND (f.id IS NULL OR f.customer_id IS DISTINCT FROM $2
  OR (f.source_type='order' AND (fee_order.id IS NULL OR fee_order.customer_id IS DISTINCT FROM $2))
  OR (f.source_type='freight_actual' AND (fs.id IS NULL OR fs.customer_id IS DISTINCT FROM $2 OR freight_order.id IS NULL OR freight_order.customer_id IS DISTINCT FROM $2))
  OR COALESCE(f.source_type,'') NOT IN ('order','freight_actual','adjustment')))
 OR COALESCE(l.source_type,'') NOT IN ('order','customer_fee_item')))`, schema, billAlias)
}
func receiptDetailTx(ctx context.Context, q financeQuerier, schema string, customerID, id int64) (ReceiptDetail, error) {
	var d ReceiptDetail
	sql := fmt.Sprintf(`SELECT r.receipt_no,r.receipt_date::text,r.actual_cents,r.receivable_cents FROM %[1]s.cash_receipts r
 WHERE r.id=$1 AND r.customer_id=$2 AND r.reversed_at IS NULL AND r.receipt_date IS NOT NULL AND r.source_type IN ('single_payment','combined_payment','billing_payment') AND jsonb_typeof(r.allocations)='array'
 AND (r.source_type NOT IN ('single_payment','combined_payment') OR (jsonb_array_length(CASE WHEN jsonb_typeof(r.allocations)='array' THEN r.allocations ELSE '[]'::jsonb END)>0 AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.allocations) item WHERE NOT (item ? 'order_id'))))
 AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(r.allocations)='array' THEN r.allocations ELSE '[]'::jsonb END) a LEFT JOIN %[1]s.orders o ON o.id=CASE WHEN a->>'order_id' ~ '^[0-9]{1,18}$' THEN (a->>'order_id')::bigint ELSE NULL END WHERE a ? 'order_id' AND (o.id IS NULL OR o.customer_id IS DISTINCT FROM $2))
 AND (r.source_type<>'billing_payment' OR (jsonb_array_length(CASE WHEN jsonb_typeof(r.allocations)='array' THEN r.allocations ELSE '[]'::jsonb END)>0 AND EXISTS(SELECT 1 FROM %[1]s.customer_settlement_payments p JOIN %[1]s.customer_settlement_batches b ON b.id=p.settlement_batch_id WHERE p.id=r.source_id AND p.customer_id=$2 AND b.customer_id=$2 AND p.reversed_at IS NULL AND b.status IN ('confirmed','paid','settled') AND %[2]s AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.allocations) a LEFT JOIN %[1]s.customer_settlement_lines l ON l.id=CASE WHEN a->>'line_id' ~ '^[0-9]{1,18}$' THEN (a->>'line_id')::bigint ELSE NULL END WHERE l.id IS NULL OR l.customer_id IS DISTINCT FROM $2 OR l.settlement_batch_id IS DISTINCT FROM b.id))))`, schema, billSourceSafeSQL(schema, "b"))
	err := q.QueryRow(ctx, sql, id, customerID).Scan(&d.ReceiptNo, &d.ReceiptDate, &d.ActualCents, &d.ReceivableCents)
	return d, err
}
func billDetailTx(ctx context.Context, q financeQuerier, schema string, customerID, id int64) (BillDetail, error) {
	var d BillDetail
	sql := fmt.Sprintf(`SELECT b.settlement_no,b.period_from::text,b.period_to::text,round(b.total_amount*100)::bigint FROM %[1]s.customer_settlement_batches b WHERE b.id=$1 AND b.customer_id=$2 AND b.status IN ('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0 AND %[2]s`, schema, billSourceSafeSQL(schema, "b"))
	err := q.QueryRow(ctx, sql, id, customerID).Scan(&d.BillNo, &d.PeriodFrom, &d.PeriodTo, &d.AmountCents)
	return d, err
}
func (r Repository) ReceiptDetail(ctx context.Context, customerID, id int64) (ReceiptDetail, error) {
	return receiptDetailTx(ctx, r.pool, r.schema, customerID, id)
}
func (r Repository) BillDetail(ctx context.Context, customerID, id int64) (BillDetail, error) {
	return billDetailTx(ctx, r.pool, r.schema, customerID, id)
}
