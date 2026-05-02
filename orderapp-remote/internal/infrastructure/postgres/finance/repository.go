package finance

import (
	"context"
	"encoding/json"
	"fmt"

	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) LoadSettings(ctx context.Context) (appfinance.SettingsSnapshot, error) {
	if r.pool == nil {
		return appfinance.SettingsSnapshot{Settings: domain.DefaultSettings()}, nil
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT company_type,taxpayer_type,declaration_period,closing_mode,
		       small_scale_vat_rate::float8,small_scale_vat_threshold::float8,
		       general_output_vat_rate::float8,default_input_vat_rate::float8,
		       surtax_rate::float8,cit_standard_rate::float8,
		       small_low_profit_enabled,small_low_profit_effective_rate::float8,
		       small_low_profit_annual_profit_limit::float8,close_mode_admin_users
		FROM %s.finance_settings
		WHERE id=1
	`, r.schema))
	return scanSettings(row)
}

func (r Repository) SaveSettings(ctx context.Context, snapshot appfinance.SettingsSnapshot, actor string) (appfinance.SettingsSnapshot, error) {
	if r.pool == nil {
		return snapshot, nil
	}
	users, err := json.Marshal(snapshot.CloseModeAdminUsers)
	if err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldMode string
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT closing_mode FROM %s.finance_settings WHERE id=1 FOR UPDATE`, r.schema)).Scan(&oldMode)
	row := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.finance_settings(
			id,company_type,taxpayer_type,declaration_period,closing_mode,
			small_scale_vat_rate,small_scale_vat_threshold,general_output_vat_rate,
			default_input_vat_rate,surtax_rate,cit_standard_rate,small_low_profit_enabled,
			small_low_profit_effective_rate,small_low_profit_annual_profit_limit,close_mode_admin_users,
			updated_at,updated_by
		)
		VALUES(1,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,now(),$15)
		ON CONFLICT(id) DO UPDATE SET
			company_type=EXCLUDED.company_type,
			taxpayer_type=EXCLUDED.taxpayer_type,
			declaration_period=EXCLUDED.declaration_period,
			closing_mode=EXCLUDED.closing_mode,
			small_scale_vat_rate=EXCLUDED.small_scale_vat_rate,
			small_scale_vat_threshold=EXCLUDED.small_scale_vat_threshold,
			general_output_vat_rate=EXCLUDED.general_output_vat_rate,
			default_input_vat_rate=EXCLUDED.default_input_vat_rate,
			surtax_rate=EXCLUDED.surtax_rate,
			cit_standard_rate=EXCLUDED.cit_standard_rate,
			small_low_profit_enabled=EXCLUDED.small_low_profit_enabled,
			small_low_profit_effective_rate=EXCLUDED.small_low_profit_effective_rate,
			small_low_profit_annual_profit_limit=EXCLUDED.small_low_profit_annual_profit_limit,
			close_mode_admin_users=EXCLUDED.close_mode_admin_users,
			updated_at=now(),
			updated_by=EXCLUDED.updated_by
		RETURNING company_type,taxpayer_type,declaration_period,closing_mode,
		       small_scale_vat_rate::float8,small_scale_vat_threshold::float8,
		       general_output_vat_rate::float8,default_input_vat_rate::float8,
		       surtax_rate::float8,cit_standard_rate::float8,
		       small_low_profit_enabled,small_low_profit_effective_rate::float8,
		       small_low_profit_annual_profit_limit::float8,close_mode_admin_users
	`, r.schema),
		snapshot.CompanyType, snapshot.TaxpayerType, snapshot.DeclarationPeriod, snapshot.ClosingMode,
		snapshot.SmallScaleVATRate, snapshot.SmallScaleVATThreshold, snapshot.GeneralOutputVATRate,
		snapshot.DefaultInputVATRate, snapshot.SurtaxRate, snapshot.CITStandardRate, snapshot.SmallLowProfitEnabled,
		snapshot.SmallLowProfitEffectiveRate, snapshot.SmallLowProfitAnnualProfitLimit, string(users), actor)
	saved, err := scanSettings(row)
	if err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "finance_settings", nil, "update", postgresinfra.StrPtr("closing_mode"), postgresinfra.StrPtr(oldMode), postgresinfra.StrPtr(saved.ClosingMode), postgresinfra.AuditMeta{
		"company_type":  saved.CompanyType,
		"taxpayer_type": saved.TaxpayerType,
	}); err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	return saved, nil
}

func (r Repository) MonthlySourceTotals(ctx context.Context, month string) (domain.MonthlySourceTotals, []appfinance.Exception, error) {
	out := domain.MonthlySourceTotals{Month: month}
	if r.pool == nil {
		return out, nil, nil
	}
	start := month + "-01"
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(COALESCE(grand_total,total_amount,0)),0)::float8
		FROM %s.orders
		WHERE COALESCE(is_void,false)=false
		  AND order_date >= $1::date
		  AND order_date < ($1::date + INTERVAL '1 month')
	`, r.schema), start).Scan(&out.RevenueTaxInclusive); err != nil {
		return out, nil, err
	}
	var productionCost, mainCostExpense, periodExpense, inputVAT, nonDeductibleVAT domain.Money
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(total_cost),0)::float8
		FROM %s.production_batch_costs
		WHERE created_at >= $1::date
		  AND created_at < ($1::date + INTERVAL '1 month')
	`, r.schema), start).Scan(&productionCost)
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE allocation='main_cost'),0)::float8,
			COALESCE(SUM(amount) FILTER (WHERE allocation='period_expense'),0)::float8,
			COALESCE(SUM(input_vat),0)::float8,
			COALESCE(SUM(non_deductible_input_vat),0)::float8
		FROM %s.finance_expenses
		WHERE month=$1
	`, r.schema), month).Scan(&mainCostExpense, &periodExpense, &inputVAT, &nonDeductibleVAT); err != nil {
		return out, nil, err
	}
	out.MainBusinessCost = productionCost + mainCostExpense
	out.PeriodExpenses = periodExpense
	out.InputVAT = inputVAT
	out.NonDeductibleInputVAT = nonDeductibleVAT
	exceptions := make([]appfinance.Exception, 0)
	var uncategorized int
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.finance_expenses WHERE month=$1 AND category=''`, r.schema), month).Scan(&uncategorized)
	if uncategorized > 0 {
		exceptions = append(exceptions, appfinance.Exception{Code: "uncategorized_expense", Message: "有未分类费用", Count: uncategorized})
	}
	return out, exceptions, nil
}

func (r Repository) CreateExpense(ctx context.Context, cmd appfinance.CreateExpenseCommand) (appfinance.Expense, error) {
	var row appfinance.Expense
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.finance_expenses(expense_date,month,category,amount,allocation,input_vat,non_deductible_input_vat,payment,note,created_by)
		VALUES($1::date,$2,$3,$4,$5,0,0,$6,$7,$8)
		RETURNING id,to_char(expense_date,'YYYY-MM-DD'),month,category,amount::float8,allocation,payment,note,created_by,to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.Date, cmd.Month, cmd.Category, cmd.Amount, cmd.Allocation, cmd.Payment, cmd.Note, cmd.Actor).
		Scan(&row.ID, &row.Date, &row.Month, &row.Category, &row.Amount, &row.Allocation, &row.Payment, &row.Note, &row.Actor, &row.CreatedAt)
	return row, err
}

func (r Repository) ListExpenses(ctx context.Context, month string) ([]appfinance.Expense, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,to_char(expense_date,'YYYY-MM-DD'),month,category,amount::float8,allocation,payment,note,created_by,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finance_expenses
		WHERE month=$1
		ORDER BY expense_date DESC,id DESC
	`, r.schema), month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appfinance.Expense{}
	for rows.Next() {
		var row appfinance.Expense
		if err := rows.Scan(&row.ID, &row.Date, &row.Month, &row.Category, &row.Amount, &row.Allocation, &row.Payment, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveMonthlyReport(ctx context.Context, report domain.MonthlyReport, actor string) (domain.MonthlyReport, error) {
	payload, err := json.Marshal(report)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	_, err = r.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finance_monthly_reports(month,status,snapshot_json,generated_at,closed_at,closed_by,updated_at)
		VALUES($1,$2,$3::jsonb,now(),CASE WHEN $2 IN ('closed','adjusted') THEN now() ELSE NULL END,$4,now())
		ON CONFLICT(month) DO UPDATE SET
			status=EXCLUDED.status,
			snapshot_json=EXCLUDED.snapshot_json,
			closed_at=CASE WHEN EXCLUDED.status IN ('closed','adjusted') THEN COALESCE(%s.finance_monthly_reports.closed_at, now()) ELSE NULL END,
			closed_by=EXCLUDED.closed_by,
			updated_at=now()
	`, r.schema, r.schema), report.Month, report.Status, string(payload), actor)
	return report, err
}

func (r Repository) MonthlyReportStatus(ctx context.Context, month string) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.finance_monthly_reports WHERE month=$1`, r.schema), month).Scan(&status)
	if err == pgx.ErrNoRows {
		return domain.MonthStatusDraft, nil
	}
	return status, err
}

func (r Repository) CreateAdjustment(ctx context.Context, cmd appfinance.CreateAdjustmentCommand) (appfinance.AdjustmentRecord, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var row appfinance.AdjustmentRecord
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.finance_adjustments(month,type,amount,reason,note,actor)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id,month,type,amount::float8,reason,note,actor,to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.Month, cmd.Type, cmd.Amount, cmd.Reason, cmd.Note, cmd.Actor).Scan(&row.ID, &row.Month, &row.Type, &row.Amount, &row.Reason, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finance_monthly_reports SET status='adjusted',updated_at=now() WHERE month=$1`, r.schema), cmd.Month); err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "finance_adjustment", &row.ID, "create", postgresinfra.StrPtr(row.Type), nil, postgresinfra.StrPtr(fmt.Sprintf("%.2f", row.Amount)), postgresinfra.AuditMeta{
		"month":  row.Month,
		"reason": row.Reason,
	}); err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appfinance.AdjustmentRecord{}, err
	}
	return row, nil
}

func (r Repository) ListAdjustments(ctx context.Context, month string) ([]appfinance.AdjustmentRecord, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,month,type,amount::float8,reason,note,actor,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finance_adjustments
		WHERE month=$1
		ORDER BY id
	`, r.schema), month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appfinance.AdjustmentRecord{}
	for rows.Next() {
		var row appfinance.AdjustmentRecord
		if err := rows.Scan(&row.ID, &row.Month, &row.Type, &row.Amount, &row.Reason, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanSettings(row pgx.Row) (appfinance.SettingsSnapshot, error) {
	var out appfinance.SettingsSnapshot
	var usersJSON []byte
	if err := row.Scan(
		&out.CompanyType, &out.TaxpayerType, &out.DeclarationPeriod, &out.ClosingMode,
		&out.SmallScaleVATRate, &out.SmallScaleVATThreshold,
		&out.GeneralOutputVATRate, &out.DefaultInputVATRate,
		&out.SurtaxRate, &out.CITStandardRate, &out.SmallLowProfitEnabled,
		&out.SmallLowProfitEffectiveRate, &out.SmallLowProfitAnnualProfitLimit, &usersJSON,
	); err != nil {
		return appfinance.SettingsSnapshot{}, err
	}
	_ = json.Unmarshal(usersJSON, &out.CloseModeAdminUsers)
	out.Settings = domain.NormalizeSettings(out.Settings)
	return out, nil
}
