package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
)

// customerIDsWithFinanceActivity enumerates only customers represented in the
// month, including adjustments posted in a different month from the order.
func (r Repository) customerIDsWithFinanceActivity(ctx context.Context, month string) ([]int64, error) {
	parts := []string{
		fmt.Sprintf(`SELECT customer_id FROM %s.orders WHERE order_date >= $1::date AND order_date < $1::date + interval '1 month'`, r.schema),
		fmt.Sprintf(`SELECT customer_id FROM %s.finance_expenses WHERE month=$2`, r.schema),
	}
	var fee, requests, feeEvents, resolutions, returns bool
	if err := r.query().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL,to_regclass($3) IS NOT NULL,to_regclass($4) IS NOT NULL,to_regclass($5) IS NOT NULL`,
		r.schema+".customer_fee_items", r.schema+".order_resolution_requests", r.schema+".order_resolution_fee_events", r.schema+".order_resolution_financial_entries", r.schema+".order_resolution_stock_returns").Scan(&fee, &requests, &feeEvents, &resolutions, &returns); err != nil {
		return nil, err
	}
	if fee {
		parts = append(parts, fmt.Sprintf(`SELECT f.customer_id FROM %[1]s.customer_fee_items f WHERE %[2]s AND (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date AND (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date < $1::date + interval '1 month'`, r.schema, r.independentFeePredicate()))
	}
	if fee && requests && feeEvents {
		parts = append(parts, fmt.Sprintf(`SELECT f.customer_id FROM %[1]s.order_resolution_fee_events e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id AND r.applied JOIN %[1]s.customer_fee_items f ON f.id=e.source_id WHERE %[2]s AND NULLIF(BTRIM(r.command_json->>'posting_date'),'')::date >= $1::date AND NULLIF(BTRIM(r.command_json->>'posting_date'),'')::date < $1::date + interval '1 month' AND e.credit_cents<>0`, r.schema, r.independentFeePredicate()))
	}
	if resolutions {
		parts = append(parts, fmt.Sprintf(`SELECT customer_id FROM %s.order_resolution_financial_entries WHERE kind='receivable_credit' AND posting_date >= $1::date AND posting_date < $1::date + interval '1 month'`, r.schema))
	}
	if returns {
		parts = append(parts, fmt.Sprintf(`SELECT o.customer_id FROM %[1]s.order_resolution_stock_returns x JOIN %[1]s.orders o ON o.id=x.order_id WHERE x.status='posted' AND x.posting_date >= $1::date AND x.posting_date < $1::date + interval '1 month'`, r.schema))
	}
	rows, err := r.query().Query(ctx, `SELECT DISTINCT customer_id FROM (`+strings.Join(parts, ` UNION ALL `)+`) finance_activity WHERE customer_id>0 ORDER BY customer_id`, month+"-01", month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r Repository) buildCustomerSnapshots(ctx context.Context, month, status string, settings domain.Settings) (app.CustomerClosingSnapshots, error) {
	result := app.CustomerClosingSnapshots{Version: 1, Customers: map[string]app.CustomerClosingSnapshot{}}
	ids, err := r.customerIDsWithFinanceActivity(ctx, month)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		filter := app.ReportFilter{Month: month, CustomerID: id}
		totals, _, err := r.MonthlySourceTotals(ctx, filter)
		if err != nil {
			return result, err
		}
		report := domain.BuildMonthlyReport(settings, totals)
		report.Status = status
		rows, err := r.FinanceSourceDetails(ctx, filter)
		if err != nil {
			return result, err
		}
		daily, err := app.BuildDailySnapshot(month, rows, settings)
		if err != nil {
			return result, err
		}
		if err = app.ValidateDailySnapshot(report, daily); err != nil {
			return result, fmt.Errorf("客户 %d 的财务快照不一致: %w", id, err)
		}
		result.Customers[strconv.FormatInt(id, 10)] = app.CustomerClosingSnapshot{Report: report, Daily: daily}
	}
	return result, nil
}

func (r Repository) saveCustomerSnapshots(ctx context.Context, month string, snapshot app.CustomerClosingSnapshots) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = r.query().Exec(ctx, fmt.Sprintf(`UPDATE %s.finance_monthly_reports SET snapshot_json=jsonb_set(snapshot_json,'{customer_snapshots}',$2::jsonb,true) WHERE month=$1`, r.schema), month, string(raw))
	return err
}

func (r Repository) LoadCustomerSnapshot(ctx context.Context, month string, customerID int64) (app.CustomerClosingSnapshot, bool, error) {
	var raw []byte
	err := r.query().QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json->'customer_snapshots' FROM %s.finance_monthly_reports WHERE month=$1`, r.schema), month).Scan(&raw)
	if err == pgx.ErrNoRows {
		return app.CustomerClosingSnapshot{}, false, nil
	}
	if err != nil {
		return app.CustomerClosingSnapshot{}, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return app.CustomerClosingSnapshot{}, false, nil
	}
	var snapshots app.CustomerClosingSnapshots
	if err := json.Unmarshal(raw, &snapshots); err != nil {
		return app.CustomerClosingSnapshot{}, false, err
	}
	if snapshots.Version != 1 {
		return app.CustomerClosingSnapshot{}, false, nil
	}
	snapshot, found := snapshots.Customers[strconv.FormatInt(customerID, 10)]
	return snapshot, found, nil
}
