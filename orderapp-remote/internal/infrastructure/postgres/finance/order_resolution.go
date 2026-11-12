package finance

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	app "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
	infra "orderapp/internal/infrastructure/postgres"
)

type financeQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (r Repository) query() financeQuerier {
	if r.tx != nil {
		return r.tx
	}
	return r.pool
}
func (r Repository) hasResolutionEntries(ctx context.Context) (bool, error) {
	var yes bool
	err := r.query().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".order_resolution_financial_entries").Scan(&yes)
	return yes, err
}
func (r Repository) resolutionRevenue(ctx context.Context, filter app.ReportFilter) (domain.Money, error) {
	exists, err := r.hasResolutionEntries(ctx)
	if err != nil || !exists {
		return 0, err
	}
	var amount float64
	err = r.query().QueryRow(ctx, fmt.Sprintf(`SELECT -COALESCE(sum(amount_cents),0)::float8/100 FROM %s.order_resolution_financial_entries WHERE kind='receivable_credit' AND posting_date >= $1::date AND posting_date < $1::date+interval '1 month' AND ($2::bigint=0 OR customer_id=$2)`, r.schema), filter.Month+"-01", filter.CustomerID).Scan(&amount)
	return domain.Money(amount), err
}
func (r Repository) resolutionDetails(ctx context.Context, filter app.ReportFilter) ([]app.SourceDetail, error) {
	out := []app.SourceDetail{}
	exists, err := r.hasResolutionEntries(ctx)
	if err != nil || !exists {
		return out, err
	}
	rows, err := r.query().Query(ctx, fmt.Sprintf(`SELECT 'revenue','order_resolution',f.request_id,to_char(f.posting_date,'YYYY-MM-DD'),'订单处理 · '||o.order_no,'订单冲减',COALESCE(c.name,''),'','','',-f.amount_cents::float8/100,'/app/vue-shell?view=orders&order_id='||o.id::text FROM %[1]s.order_resolution_financial_entries f JOIN %[1]s.orders o ON o.id=f.order_id LEFT JOIN %[1]s.customers c ON c.id=f.customer_id WHERE f.kind='receivable_credit' AND f.posting_date >= $1::date AND f.posting_date < $1::date+interval '1 month' AND ($2::bigint=0 OR f.customer_id=$2) ORDER BY f.id`, r.schema), filter.Month+"-01", filter.CustomerID)
	if err != nil {
		return out, err
	}
	err = scanSourceDetails(rows, &out)
	return out, err
}

// CloseMonthAtomic builds the report after acquiring the same period lock as
// order resolutions, and saves the snapshot and audit in that transaction.
func (r Repository) CloseMonthAtomic(ctx context.Context, cmd app.CloseMonthCommand) (domain.MonthlyReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":finance-period"); err != nil {
		return domain.MonthlyReport{}, err
	}
	bound := r
	bound.tx = tx
	report, err := app.NewService(bound).DraftReport(ctx, app.ReportFilter{Month: cmd.Month})
	if err != nil {
		return report, err
	}
	if report.CostUnavailable {
		return domain.MonthlyReport{}, fmt.Errorf("生产成本待核算，暂不能结账")
	}
	if report.Status == domain.MonthStatusClosed || report.Status == domain.MonthStatusAdjusted {
		_, found, err := bound.LoadMonthlyReportSnapshot(ctx, cmd.Month)
		if err != nil {
			return domain.MonthlyReport{}, err
		}
		if found {
			return report, nil
		}
	}
	if report.Status != domain.MonthStatusAdjusted {
		report.Status = domain.MonthStatusClosed
	}
	daily, err := app.NewService(bound).DailySnapshotDraft(ctx, app.ReportFilter{Month: cmd.Month})
	if err != nil {
		return report, err
	}
	if err = app.ValidateDailySnapshot(report, daily); err != nil {
		return report, err
	}
	settings, err := bound.LoadSettings(ctx)
	if err != nil {
		return report, err
	}
	customers, err := bound.buildCustomerSnapshots(ctx, cmd.Month, report.Status, settings.Settings)
	if err != nil {
		return report, err
	}
	report, err = bound.SaveMonthlyReport(ctx, report, cmd.Actor)
	if err != nil {
		return report, err
	}
	if err = bound.saveDailySnapshot(ctx, cmd.Month, daily); err != nil {
		return report, err
	}
	if err = bound.saveCustomerSnapshots(ctx, cmd.Month, customers); err != nil {
		return report, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "finance_monthly_report", nil, "close", nil, nil, infra.StrPtr(cmd.Month), infra.AuditMeta{"month": cmd.Month}); err != nil {
		return report, err
	}
	return report, tx.Commit(ctx)
}

// RebuildClosedMonthTx is only used after the caller has verified a finance
// approval and retained the prior snapshot under the shared period lock.
func (r Repository) RebuildClosedMonthTx(ctx context.Context, tx pgx.Tx, month, actor string) error {
	bound := r
	bound.tx = tx
	totals, _, err := bound.MonthlySourceTotals(ctx, app.ReportFilter{Month: month})
	if err != nil {
		return err
	}
	if totals.CostUnavailable {
		return fmt.Errorf("生产成本待核算，暂不能重算已结月份")
	}
	settings, err := bound.LoadSettings(ctx)
	if err != nil {
		return err
	}
	report := domain.BuildMonthlyReport(settings.Settings, totals)
	rows, err := bound.ListAdjustments(ctx, month)
	if err != nil {
		return err
	}
	adjustments := make([]domain.Adjustment, 0, len(rows))
	for _, row := range rows {
		adjustments = append(adjustments, domain.Adjustment{Type: row.Type, Amount: row.Amount})
	}
	report = domain.ApplyAdjustments(report, adjustments)
	report.Status = domain.MonthStatusAdjusted
	daily, err := app.NewService(bound).DailySnapshotDraft(ctx, app.ReportFilter{Month: month})
	if err != nil {
		return err
	}
	if err = app.ValidateDailySnapshot(report, daily); err != nil {
		return err
	}
	customers, err := bound.buildCustomerSnapshots(ctx, month, report.Status, settings.Settings)
	if err != nil {
		return err
	}
	_, err = bound.SaveMonthlyReport(ctx, report, actor)
	if err != nil {
		return err
	}
	if err = bound.saveDailySnapshot(ctx, month, daily); err != nil {
		return err
	}
	return bound.saveCustomerSnapshots(ctx, month, customers)
}
