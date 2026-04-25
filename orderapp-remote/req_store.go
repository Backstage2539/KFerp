package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReqRow struct {
	ID        int64
	Code      string
	PRCode    string
	Title     string
	Status    string
	Assignee  string
	Evidence  string
	CreatedAt time.Time
}

func ensureReqTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	ddl := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_product (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_dev (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_unit (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_api (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_review (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			pr_code TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
	}
	for _, q := range ddl {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	// Schema migration (safe, idempotent)
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.req_review ADD COLUMN IF NOT EXISTS pr_code TEXT NOT NULL DEFAULT ''`, schema))
	return nil
}

func listReqRows(ctx context.Context, pool *pgxpool.Pool, schema, table string, limit, offset int) (rowsOut []ReqRow, hasNext bool, err error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	cols := "id, code, title, status, assignee, evidence, created_at"
	if table == "req_review" {
		cols = "id, code, pr_code, title, status, assignee, evidence, created_at"
	}

	orderBy := "id DESC"
	// Archive ordering: put imported/archived items last.
	if table == "req_product" {
		orderBy = "(code ILIKE 'old-%') ASC, id DESC"
	}
	if table == "req_unit" {
		orderBy = "(code ILIKE 'old-%' OR code ILIKE 'OLD-%') ASC, id DESC"
	}

	q := fmt.Sprintf(`SELECT %s
		FROM %s.%s
		ORDER BY %s
		LIMIT %d OFFSET %d`, cols, schema, table, orderBy, limit+1, offset)
	r, e := pool.Query(ctx, q)
	if e != nil {
		return nil, false, e
	}
	defer r.Close()

	out := make([]ReqRow, 0)
	for r.Next() {
		var row ReqRow
		if table == "req_review" {
			if e := r.Scan(&row.ID, &row.Code, &row.PRCode, &row.Title, &row.Status, &row.Assignee, &row.Evidence, &row.CreatedAt); e != nil {
				return nil, false, e
			}
		} else {
			if e := r.Scan(&row.ID, &row.Code, &row.Title, &row.Status, &row.Assignee, &row.Evidence, &row.CreatedAt); e != nil {
				return nil, false, e
			}
		}
		out = append(out, row)
	}
	if e := r.Err(); e != nil {
		return nil, false, e
	}
	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func createReqRow(ctx context.Context, pool *pgxpool.Pool, schema, table, code, title, status, assignee string) error {
	code = strings.TrimSpace(code)
	title = strings.TrimSpace(title)
	status = strings.TrimSpace(status)
	assignee = strings.TrimSpace(assignee)
	if title == "" {
		return fmt.Errorf("title required")
	}
	if status == "" {
		status = "todo"
	}
	if code == "" {
		n, err := nextReqCodeForTable(ctx, pool, schema, table)
		if err != nil {
			return err
		}
		code = n
	}
	q := fmt.Sprintf(`INSERT INTO %s.%s (code, title, status, assignee)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) DO NOTHING`, schema, table)
	ct, err := pool.Exec(ctx, q, code, title, status, assignee)
	if err != nil {
		return err
	}
	// if conflict (no insert), surface as error to the UI
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("code already exists: %s", code)
	}
	return nil
}

type reqSeedRow struct {
	table    string
	code     string
	prCode   string
	title    string
	status   string
	assignee string
	evidence string
}

func seedReqRow(ctx context.Context, pool *pgxpool.Pool, schema string, row reqSeedRow) error {
	row.code = strings.TrimSpace(row.code)
	row.prCode = strings.TrimSpace(row.prCode)
	row.title = strings.TrimSpace(row.title)
	row.status = strings.TrimSpace(row.status)
	row.assignee = strings.TrimSpace(row.assignee)
	row.evidence = strings.TrimSpace(row.evidence)
	if row.code == "" || row.title == "" {
		return fmt.Errorf("seed req row code/title required")
	}
	if row.status == "" {
		row.status = "todo"
	}
	if row.table == "req_review" {
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.req_review(code, pr_code, title, status, assignee, evidence)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT (code) DO UPDATE SET
				pr_code=excluded.pr_code,
				title=excluded.title,
				status=excluded.status,
				assignee=excluded.assignee,
				evidence=excluded.evidence
		`, schema), row.code, row.prCode, row.title, row.status, row.assignee, row.evidence)
		return err
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.%s(code, title, status, assignee, evidence)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (code) DO UPDATE SET
			title=excluded.title,
			status=excluded.status,
			assignee=excluded.assignee,
			evidence=excluded.evidence
	`, schema, row.table), row.code, row.title, row.status, row.assignee, row.evidence)
	return err
}

func seedReqWorkflowA(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	// One-time seed for the "A" small requirement: add top shortcuts on req pages.
	// Safe: uses ON CONFLICT DO NOTHING.
	if err := createReqRow(ctx, pool, schema, "req_product", "PR-001", "需求管理5页面增加统一顶部快捷入口/互相跳转", "todo", "VA"); err != nil {
		// ignore if already exists
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	for _, it := range []struct {
		table, code, title, status, assignee string
	}{
		{"req_dev", "DEV-001", "req_*.html 顶部增加按钮组：产品/开发/单测/API/审核互相跳转", "todo", "JJ"},
		{"req_dev", "DEV-002", "统一样式（复用现有 btn/pill 样式，移动端可用）", "todo", "JJ"},
		{"req_unit", "UT-001", "打开5个页面返回200（无500），模板渲染不报错", "todo", "JJ"},
		{"req_api", "API-001", "GET /app/req/product 返回200", "todo", "JJ"},
		{"req_api", "API-002", "GET /app/req/dev 返回200", "todo", "JJ"},
		{"req_api", "API-003", "GET /app/req/unit 返回200", "todo", "JJ"},
		{"req_api", "API-004", "GET /app/req/api 返回200", "todo", "JJ"},
		{"req_api", "API-005", "GET /app/req/review 返回200", "todo", "JJ"},
		{"req_review", "REV-001", "需求管理页面顶部入口可点击跳转；无404/500", "todo", "VA"},
		{"req_product", "PR-PRICE-003", "零售录单支持自定义克数；无对应规格时按227g零售价折算并向上取整", "done", "VA"},
		{"req_dev", "DEV-ORDER-001", "录单页迁移到 Vue/Vite，新增订单表单 JSON API 并支持自定义克数", "done", "Codex"},
		{"req_dev", "DEV-PRICE-005", "零售规格选择保留已有有价规格，并提供自定义克数输入", "done", "Codex"},
		{"req_unit", "UT-PRICE-003", "覆盖零售自定义克数价格折算和订单 payload 规格保存", "done", "Codex"},
		{"req_api", "API-ORDER-001", "覆盖 GET /api/order/form 和 POST /api/order 自定义规格保存", "done", "Codex"},
		{"req_review", "REV-PRICE-003", "验收：零售录单可输入自定义克数，保存后规格和金额正确", "done", "VA"},
	} {
		if err := createReqRow(ctx, pool, schema, it.table, it.code, it.title, it.status, it.assignee); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	for _, row := range []reqSeedRow{
		{table: "req_product", code: "PR-STOCK-001", title: "生产完成时生成库存流水与成品批次，支持库存追溯", status: "review", assignee: "VA", evidence: "功能分支 codex/erpnext-stock-ledger-batches；go test ./..."},
		{table: "req_dev", code: "DEV-STOCK-001", title: "新增 stock_ledger_entries / stock_batches schema", status: "done", assignee: "JJ", evidence: "ensureStockLedgerTables + TestProduceFinishHandlerWritesStockLedgerAndFinishedBatch"},
		{table: "req_dev", code: "DEV-STOCK-002", title: "完成生产时写入成品入库流水、物料出库流水和成品批次", status: "done", assignee: "JJ", evidence: "finishRunningItem -> recordFinishedProductStockMovementTx / deductMaterialsForRunningItemTx"},
		{table: "req_unit", code: "UT-STOCK-001", title: "成品库存流水数量换算与成品批次号规则", status: "done", assignee: "JJ", evidence: "TestFinishedInventoryLedgerQtyConvertsUnitsAndLooseToGrams; TestFinishedProductionBatchCodeUsesRunningItemID"},
		{table: "req_api", code: "API-STOCK-001", title: "POST /produce/running/finish 写入库存流水和成品批次", status: "done", assignee: "JJ", evidence: "TestProduceFinishHandlerWritesStockLedgerAndFinishedBatch"},
		{table: "req_review", code: "REV-STOCK-001", prCode: "PR-STOCK-001", title: "库存流水与成品批次可按 production_run 追溯", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
	} {
		if err := seedReqRow(ctx, pool, schema, row); err != nil {
			return err
		}
	}
	return nil
}
