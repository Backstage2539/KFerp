package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

func financeOrderRevenueSQL(alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	return fmt.Sprintf(`CASE
			WHEN COALESCE(%[1]sgrand_total,0) <> 0
			  OR COALESCE(%[1]sdiscount_amount,0) <> 0
			  OR COALESCE(%[1]sshipping_amount,0) <> 0
			THEN COALESCE(%[1]sgrand_total,0)
			ELSE COALESCE(%[1]stotal_amount,0)
		END`, prefix)
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

func (r Repository) MonthlySourceTotals(ctx context.Context, filter appfinance.ReportFilter) (domain.MonthlySourceTotals, []appfinance.Exception, error) {
	out := domain.MonthlySourceTotals{Month: filter.Month}
	if r.pool == nil {
		return out, nil, nil
	}
	start := filter.Month + "-01"
	customerID := filter.CustomerID
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(%s),0)::float8
		FROM %s.orders
		WHERE COALESCE(is_void,false)=false
		  AND order_date >= $1::date
		  AND order_date < ($1::date + INTERVAL '1 month')
		  AND ($2::bigint=0 OR customer_id=$2::bigint)
	`, financeOrderRevenueSQL(""), r.schema), start, customerID).Scan(&out.RevenueTaxInclusive); err != nil {
		return out, nil, err
	}
	var productionCost, mainCostExpense, periodExpense, inputVAT, nonDeductibleVAT domain.Money
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(total_cost),0)::float8
		FROM %s.production_batch_costs
		WHERE created_at >= $1::date
		  AND created_at < ($1::date + INTERVAL '1 month')
		  AND $2::bigint=0
	`, r.schema), start, customerID).Scan(&productionCost)
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE allocation='main_cost'),0)::float8,
			COALESCE(SUM(amount) FILTER (WHERE allocation='period_expense'),0)::float8,
			COALESCE(SUM(input_vat),0)::float8,
			COALESCE(SUM(non_deductible_input_vat),0)::float8
		FROM %s.finance_expenses
		WHERE month=$1
		  AND ($2::bigint=0 OR customer_id=$2::bigint)
	`, r.schema), filter.Month, customerID).Scan(&mainCostExpense, &periodExpense, &inputVAT, &nonDeductibleVAT); err != nil {
		return out, nil, err
	}
	out.MainBusinessCost = productionCost + mainCostExpense
	out.PeriodExpenses = periodExpense
	out.InputVAT = inputVAT
	out.NonDeductibleInputVAT = nonDeductibleVAT
	exceptions := make([]appfinance.Exception, 0)
	var uncategorized int
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.finance_expenses WHERE month=$1 AND category='' AND ($2::bigint=0 OR customer_id=$2::bigint)`, r.schema), filter.Month, customerID).Scan(&uncategorized)
	if uncategorized > 0 {
		exceptions = append(exceptions, appfinance.Exception{Code: "uncategorized_expense", Message: "有未分类费用", Count: uncategorized})
	}
	return out, exceptions, nil
}

func (r Repository) CreateExpense(ctx context.Context, cmd appfinance.CreateExpenseCommand) (appfinance.Expense, error) {
	var row appfinance.Expense
	if err := r.validateExpenseDimensionRefs(ctx, cmd); err != nil {
		return row, err
	}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		WITH inserted AS (
			INSERT INTO %s.finance_expenses(
				expense_date,month,category,amount,allocation,employee_id,
				order_id,customer_id,product_id,batch_no,dimension_note,
				input_vat,non_deductible_input_vat,payment,note,created_by
			)
			VALUES($1::date,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,0,0,$12,$13,$14)
			RETURNING id,expense_date,month,category,amount,allocation,employee_id,
			          order_id,customer_id,product_id,batch_no,dimension_note,
			          payment,note,created_by,created_at
		)
		SELECT i.id,to_char(i.expense_date,'YYYY-MM-DD'),i.month,i.category,i.amount::float8,i.allocation,
		       COALESCE(i.employee_id,0),COALESCE(e.name,''),
		       i.order_id,i.customer_id,i.product_id,i.batch_no,i.dimension_note,
		       i.payment,i.note,i.created_by,to_char(i.created_at,'YYYY-MM-DD HH24:MI')
		FROM inserted i
		LEFT JOIN %s.company_employees e ON e.id=i.employee_id
	`, r.schema, r.schema), cmd.Date, cmd.Month, cmd.Category, cmd.Amount, cmd.Allocation, nullableEmployeeID(cmd.EmployeeID),
		cmd.OrderID, cmd.CustomerID, cmd.ProductID, cmd.BatchNo, cmd.DimensionNote, cmd.Payment, cmd.Note, cmd.Actor).
		Scan(&row.ID, &row.Date, &row.Month, &row.Category, &row.Amount, &row.Allocation, &row.EmployeeID, &row.EmployeeName,
			&row.OrderID, &row.CustomerID, &row.ProductID, &row.BatchNo, &row.DimensionNote, &row.Payment, &row.Note, &row.Actor, &row.CreatedAt)
	return row, err
}

func (r Repository) validateExpenseDimensionRefs(ctx context.Context, cmd appfinance.CreateExpenseCommand) error {
	if err := r.ensureExpenseDimensionRefExists(ctx, "orders", cmd.OrderID, "order"); err != nil {
		return err
	}
	if err := r.ensureExpenseDimensionRefExists(ctx, "customers", cmd.CustomerID, "customer"); err != nil {
		return err
	}
	if err := r.ensureExpenseDimensionRefExists(ctx, "products", cmd.ProductID, "product"); err != nil {
		return err
	}
	if err := r.ensureExpenseOrderCustomerMatch(ctx, cmd.OrderID, cmd.CustomerID); err != nil {
		return err
	}
	if err := r.ensureExpenseOrderProductMatch(ctx, cmd.OrderID, cmd.ProductID); err != nil {
		return err
	}
	return nil
}

func (r Repository) ensureExpenseOrderCustomerMatch(ctx context.Context, orderID, customerID int64) error {
	if orderID <= 0 || customerID <= 0 {
		return nil
	}
	var orderCustomerID int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.orders WHERE id=$1`, r.schema), orderID).Scan(&orderCustomerID); err != nil {
		return err
	}
	if orderCustomerID != customerID {
		return fmt.Errorf("finance dimension customer does not match order")
	}
	return nil
}

func (r Repository) ensureExpenseOrderProductMatch(ctx context.Context, orderID, productID int64) error {
	if orderID <= 0 || productID <= 0 {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_items WHERE order_id=$1 AND product_id=$2)`, r.schema), orderID, productID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("finance dimension product does not match order")
	}
	return nil
}

func (r Repository) ensureExpenseDimensionRefExists(ctx context.Context, table string, id int64, label string) error {
	if id <= 0 {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.%s WHERE id=$1)`, r.schema, table), id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("finance dimension %s not found", label)
	}
	return nil
}

func (r Repository) ListExpenses(ctx context.Context, filter appfinance.ExpenseFilter) ([]appfinance.Expense, error) {
	where := "WHERE fe.month=$1"
	args := []any{filter.Month}
	argn := 2
	if filter.EmployeeID > 0 {
		where += fmt.Sprintf(" AND fe.employee_id=$%d", argn)
		args = append(args, filter.EmployeeID)
		argn++
	}
	if filter.CustomerID > 0 {
		where += fmt.Sprintf(" AND fe.customer_id=$%d", argn)
		args = append(args, filter.CustomerID)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT fe.id,to_char(fe.expense_date,'YYYY-MM-DD'),fe.month,fe.category,fe.amount::float8,fe.allocation,
		       COALESCE(fe.employee_id,0),COALESCE(e.name,''),
		       fe.order_id,fe.customer_id,fe.product_id,fe.batch_no,fe.dimension_note,
		       fe.payment,fe.note,fe.created_by,to_char(fe.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finance_expenses fe
		LEFT JOIN %s.company_employees e ON e.id=fe.employee_id
		%s
		ORDER BY fe.expense_date DESC,fe.id DESC
	`, r.schema, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appfinance.Expense{}
	for rows.Next() {
		var row appfinance.Expense
		if err := rows.Scan(&row.ID, &row.Date, &row.Month, &row.Category, &row.Amount, &row.Allocation, &row.EmployeeID, &row.EmployeeName,
			&row.OrderID, &row.CustomerID, &row.ProductID, &row.BatchNo, &row.DimensionNote, &row.Payment, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) FinanceSourceDetails(ctx context.Context, filter appfinance.ReportFilter) ([]appfinance.SourceDetail, error) {
	if r.pool == nil {
		return []appfinance.SourceDetail{}, nil
	}
	start := filter.Month + "-01"
	customerID := filter.CustomerID
	out := []appfinance.SourceDetail{}
	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql: fmt.Sprintf(`
				SELECT 'revenue' AS section,'order_revenue' AS source_type,o.id AS source_id,
				       COALESCE(to_char(o.order_date,'YYYY-MM-DD'),'') AS source_date,
				       COALESCE(NULLIF(o.order_no,''), '订单#' || o.id::text) AS name,
				       '' AS category,COALESCE(NULLIF(c.name,''),NULLIF(c.company_name,''),'') AS counterparty,
				       COALESCE(o.payment_method,'') AS payment_method,
				       COALESCE(a.filename,'') AS payment_voucher_filename,
				       CASE WHEN COALESCE(a.object_key,'') <> '' THEN '/assets/' || a.object_key ELSE '' END AS payment_voucher_url,
				       %s::float8 AS amount,
				       '/app/vue-shell?view=orders' AS link
				FROM %s.orders o
				LEFT JOIN %s.customers c ON c.id=o.customer_id
				LEFT JOIN %s.sales_order_assets a ON a.id=o.payment_voucher_asset_id AND a.kind='payment_voucher'
				WHERE COALESCE(o.is_void,false)=false
				  AND o.order_date >= $1::date
				  AND o.order_date < ($1::date + INTERVAL '1 month')
				  AND ($2::bigint=0 OR o.customer_id=$2::bigint)
				ORDER BY o.order_date,o.id
			`, financeOrderRevenueSQL("o"), r.schema, r.schema, r.schema),
			args: []any{start, customerID},
		},
		{
			sql: fmt.Sprintf(`
				SELECT 'main_cost' AS section,'production_cost' AS source_type,id AS source_id,
				       to_char(created_at,'YYYY-MM-DD') AS source_date,
				       COALESCE(NULLIF(product_name,''),'生产批次成本') AS name,
				       '生产批次成本' AS category,'' AS counterparty,'' AS payment_method,
				       '' AS payment_voucher_filename,'' AS payment_voucher_url,total_cost::float8 AS amount,
				       '/app/vue-shell?view=productionCosts' AS link
				FROM %s.production_batch_costs
				WHERE created_at >= $1::date
				  AND created_at < ($1::date + INTERVAL '1 month')
				  AND $2::bigint=0
				ORDER BY created_at,id
			`, r.schema),
			args: []any{start, customerID},
		},
		{
			sql: fmt.Sprintf(`
				SELECT fe.allocation AS section,'expense' AS source_type,fe.id AS source_id,
				       to_char(fe.expense_date,'YYYY-MM-DD') AS source_date,
				       fe.category AS name,fe.category AS category,COALESCE(e.name,'') AS counterparty,
				       '' AS payment_method,
				       '' AS payment_voucher_filename,'' AS payment_voucher_url,
				       fe.amount::float8 AS amount,'/app/vue-shell?view=financeExpenses' AS link
				FROM %s.finance_expenses fe
				LEFT JOIN %s.company_employees e ON e.id=fe.employee_id
				WHERE fe.month=$1
				  AND ($2::bigint=0 OR fe.customer_id=$2::bigint)
				ORDER BY fe.expense_date,fe.id
			`, r.schema, r.schema),
			args: []any{filter.Month, customerID},
		},
		{
			sql: fmt.Sprintf(`
				SELECT 'tax' AS section,'tax_ledger' AS source_type,id AS source_id,
				       to_char(created_at,'YYYY-MM-DD') AS source_date,
				       COALESCE(NULLIF(invoice_no,''),kind) AS name,kind AS category,counterparty,
				       '' AS payment_method,
				       '' AS payment_voucher_filename,'' AS payment_voucher_url,
				       CASE WHEN tax_amount > 0 THEN tax_amount ELSE total_amount END::float8 AS amount,
				       '/app/vue-shell?view=financeTaxLedger' AS link
				FROM %s.finance_tax_ledger
				WHERE month=$1
				  AND $2::bigint=0
				ORDER BY id
			`, r.schema),
			args: []any{filter.Month, customerID},
		},
	}
	for _, query := range queries {
		rows, err := r.pool.Query(ctx, query.sql, query.args...)
		if err != nil {
			return nil, err
		}
		if err := scanSourceDetails(rows, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func nullableEmployeeID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func (r Repository) ListExpenseEmployees(ctx context.Context) ([]appfinance.ExpenseEmployee, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,name,active
		FROM %s.company_employees
		ORDER BY active DESC,name,id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appfinance.ExpenseEmployee{}
	for rows.Next() {
		var row appfinance.ExpenseEmployee
		if err := rows.Scan(&row.ID, &row.Name, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListTaxLedger(ctx context.Context, month string) ([]appfinance.TaxLedgerEntry, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,month,kind,invoice_no,counterparty,total_amount::float8,tax_amount::float8,status,note,created_by,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finance_tax_ledger
		WHERE month=$1
		ORDER BY id DESC
	`, r.schema), month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []appfinance.TaxLedgerEntry{}
	for rows.Next() {
		var row appfinance.TaxLedgerEntry
		if err := rows.Scan(&row.ID, &row.Month, &row.Kind, &row.InvoiceNo, &row.Counterparty, &row.TotalAmount, &row.TaxAmount, &row.Status, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) CreateTaxLedgerEntry(ctx context.Context, cmd appfinance.CreateTaxLedgerCommand) (appfinance.TaxLedgerEntry, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.ensureTaxLedgerInvoiceUniqueTx(ctx, tx, cmd); err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}

	var row appfinance.TaxLedgerEntry
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.finance_tax_ledger(month,kind,invoice_no,counterparty,total_amount,tax_amount,status,note,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id,month,kind,invoice_no,counterparty,total_amount::float8,tax_amount::float8,status,note,created_by,to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.Month, cmd.Kind, cmd.InvoiceNo, cmd.Counterparty, cmd.TotalAmount, cmd.TaxAmount, cmd.Status, cmd.Note, cmd.Actor).
		Scan(&row.ID, &row.Month, &row.Kind, &row.InvoiceNo, &row.Counterparty, &row.TotalAmount, &row.TaxAmount, &row.Status, &row.Note, &row.Actor, &row.CreatedAt); err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "finance_tax_ledger", &row.ID, "create", postgresinfra.StrPtr(row.Kind), nil, postgresinfra.StrPtr(row.InvoiceNo), postgresinfra.AuditMeta{
		"month":         row.Month,
		"counterparty":  row.Counterparty,
		"total_amount":  row.TotalAmount,
		"tax_amount":    row.TaxAmount,
		"ledger_status": row.Status,
	}); err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appfinance.TaxLedgerEntry{}, err
	}
	return row, nil
}

func (r Repository) ensureTaxLedgerInvoiceUniqueTx(ctx context.Context, tx pgx.Tx, cmd appfinance.CreateTaxLedgerCommand) error {
	invoiceNo := strings.TrimSpace(cmd.InvoiceNo)
	if invoiceNo == "" {
		return nil
	}
	kind := strings.TrimSpace(cmd.Kind)
	lockKey := r.schema + ":finance_tax_ledger:" + strings.ToLower(kind) + ":" + strings.ToLower(invoiceNo)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, lockKey); err != nil {
		return err
	}

	var existingID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.finance_tax_ledger
		WHERE kind=$1 AND lower(invoice_no)=lower($2)
		ORDER BY id
		LIMIT 1
	`, r.schema), kind, invoiceNo).Scan(&existingID)
	if err == nil {
		return fmt.Errorf("tax ledger invoice already exists")
	}
	if err == pgx.ErrNoRows {
		return nil
	}
	return err
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

func scanSourceDetails(rows pgx.Rows, out *[]appfinance.SourceDetail) error {
	defer rows.Close()
	for rows.Next() {
		var row appfinance.SourceDetail
		if err := rows.Scan(&row.Section, &row.SourceType, &row.SourceID, &row.Date, &row.Name, &row.Category, &row.Counterparty, &row.PaymentMethod, &row.PaymentVoucherFilename, &row.PaymentVoucherURL, &row.Amount, &row.Link); err != nil {
			return err
		}
		*out = append(*out, row)
	}
	return rows.Err()
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
