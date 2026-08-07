package customerportal

import (
	"context"
	"errors"
	"fmt"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListCustomerProcessingBills(ctx context.Context, customerID int64, limit int) ([]customerportalapp.CustomerBillSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT s.id,COALESCE(s.settlement_no,''),COALESCE(s.status,''),
		       to_char(COALESCE(s.total_amount,0),'FM999999990.00'),COALESCE(NULLIF(r.currency,''),'CNY'),
		       COALESCE(to_char(s.confirmed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(s.paid_at,'YYYY-MM-DD HH24:MI'),''),
		       COUNT(DISTINCT bwo.work_order_id)::int,
		       COALESCE(string_agg(DISTINCT NULLIF(bwo.product_name,''),'、'),'')
		FROM %s.customer_settlement_batches s
		JOIN %s.processing_billing_runs r ON r.id=s.processing_billing_run_id AND r.customer_id=s.customer_id
		LEFT JOIN %s.processing_billing_work_orders bwo ON bwo.billing_run_id=r.id
		WHERE s.customer_id=$1
		  AND s.processing_billing_run_id>0
		  AND s.status IN ('confirmed','settled','paid','reversed')
		  AND r.status IN ('confirmed','paid','reversed')
		GROUP BY s.id,s.settlement_no,s.status,s.total_amount,r.currency,s.confirmed_at,s.paid_at
		ORDER BY COALESCE(s.confirmed_at,s.created_at) DESC,s.id DESC
		LIMIT $2
	`, r.schema, r.schema, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CustomerBillSummary, 0)
	for rows.Next() {
		var row customerportalapp.CustomerBillSummary
		if err := rows.Scan(&row.ID, &row.SettlementNo, &row.Status, &row.TotalAmount, &row.Currency, &row.ConfirmedAt, &row.PaidAt, &row.WorkOrderCount, &row.Summary); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetCustomerProcessingBill(ctx context.Context, customerID, billID int64) (customerportalapp.CustomerBillDetail, error) {
	var out customerportalapp.CustomerBillDetail
	var billingRunID int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.id,COALESCE(s.settlement_no,''),COALESCE(s.status,''),
		       to_char(COALESCE(s.total_amount,0),'FM999999990.00'),COALESCE(NULLIF(r.currency,''),'CNY'),
		       COALESCE(to_char(s.confirmed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(s.paid_at,'YYYY-MM-DD HH24:MI'),''),
		       (SELECT COUNT(DISTINCT bwo.work_order_id)::int FROM %s.processing_billing_work_orders bwo WHERE bwo.billing_run_id=r.id),
		       COALESCE((SELECT string_agg(DISTINCT NULLIF(bwo.product_name,''),'、') FROM %s.processing_billing_work_orders bwo WHERE bwo.billing_run_id=r.id),''),
		       r.id
		FROM %s.customer_settlement_batches s
		JOIN %s.processing_billing_runs r ON r.id=s.processing_billing_run_id AND r.customer_id=s.customer_id
		WHERE s.customer_id=$1
		  AND s.id=$2
		  AND s.processing_billing_run_id>0
		  AND s.status IN ('confirmed','settled','paid','reversed')
		  AND r.status IN ('confirmed','paid','reversed')
	`, r.schema, r.schema, r.schema, r.schema), customerID, billID).Scan(
		&out.ID, &out.SettlementNo, &out.Status, &out.TotalAmount, &out.Currency,
		&out.ConfirmedAt, &out.PaidAt, &out.WorkOrderCount, &out.Summary, &billingRunID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CustomerBillDetail{}, customerportalapp.ErrCustomerBillNotFound
	}
	if err != nil {
		return customerportalapp.CustomerBillDetail{}, err
	}

	workOrderRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT bwo.work_order_id,COALESCE(bwo.work_order_no,''),COALESCE(bwo.product_name,''),
		       COALESCE(to_char(bwo.completed_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.processing_billing_work_orders bwo
		JOIN %s.processing_billing_runs r ON r.id=bwo.billing_run_id
		JOIN %s.customer_settlement_batches s ON s.processing_billing_run_id=r.id
		WHERE s.customer_id=$1 AND s.id=$2 AND r.id=$3
		  AND s.processing_billing_run_id>0
		  AND s.status IN ('confirmed','settled','paid','reversed')
		ORDER BY bwo.id
	`, r.schema, r.schema, r.schema), customerID, billID, billingRunID)
	if err != nil {
		return customerportalapp.CustomerBillDetail{}, err
	}
	for workOrderRows.Next() {
		var row customerportalapp.CustomerBillWorkOrder
		if err := workOrderRows.Scan(&row.WorkOrderID, &row.WorkOrderNo, &row.ProductName, &row.CompletedAt); err != nil {
			workOrderRows.Close()
			return customerportalapp.CustomerBillDetail{}, err
		}
		out.WorkOrders = append(out.WorkOrders, row)
	}
	if err := workOrderRows.Err(); err != nil {
		workOrderRows.Close()
		return customerportalapp.CustomerBillDetail{}, err
	}
	workOrderRows.Close()

	lineRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT line.work_order_id,COALESCE(line.fee_type,''),COALESCE(line.fee_name,''),COALESCE(line.basis,''),
		       to_char(COALESCE(line.base_quantity,0),'FM999999990.0000'),
		       to_char(COALESCE(line.unit_price,0),'FM999999990.0000'),
		       to_char(COALESCE(line.amount,0),'FM999999990.00')
		FROM %s.processing_billing_line_snapshots line
		JOIN %s.processing_billing_runs r ON r.id=line.billing_run_id
		JOIN %s.customer_settlement_batches s ON s.processing_billing_run_id=r.id
		WHERE s.customer_id=$1 AND s.id=$2 AND r.id=$3
		  AND s.processing_billing_run_id>0
		  AND s.status IN ('confirmed','settled','paid','reversed')
		ORDER BY line.work_order_id,line.id
	`, r.schema, r.schema, r.schema), customerID, billID, billingRunID)
	if err != nil {
		return customerportalapp.CustomerBillDetail{}, err
	}
	defer lineRows.Close()
	for lineRows.Next() {
		var row customerportalapp.CustomerBillLine
		if err := lineRows.Scan(&row.WorkOrderID, &row.FeeType, &row.FeeName, &row.Basis, &row.BaseQuantity, &row.UnitPrice, &row.Amount); err != nil {
			return customerportalapp.CustomerBillDetail{}, err
		}
		out.Lines = append(out.Lines, row)
	}
	if err := lineRows.Err(); err != nil {
		return customerportalapp.CustomerBillDetail{}, err
	}
	if out.WorkOrders == nil {
		out.WorkOrders = []customerportalapp.CustomerBillWorkOrder{}
	}
	if out.Lines == nil {
		out.Lines = []customerportalapp.CustomerBillLine{}
	}
	return out, nil
}
