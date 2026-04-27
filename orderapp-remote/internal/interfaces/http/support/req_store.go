package support

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReqRow struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	PRCode    string    `json:"pr_code"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Assignee  string    `json:"assignee"`
	Evidence  string    `json:"evidence"`
	CreatedAt time.Time `json:"created_at"`
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
		{table: "req_product", code: "PR-DDD-001", title: "将 appmain 临时大包拆成 DDD 业务模块，降低后续复杂功能耦合", status: "review", assignee: "VA", evidence: "docs/superpowers/plans/2026-04-26-ddd-module-split.md"},
		{table: "req_dev", code: "DEV-DDD-001", title: "appmain 仅保留启动、HTTP server 装配、路由组合和 schema 组合", status: "done", assignee: "Codex", evidence: "TestAppmainIsCompositionRootOnly"},
		{table: "req_dev", code: "DEV-DDD-002", title: "按 support/company/customer/production/sales 拆出 HTTP 业务模块", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/* modules"},
		{table: "req_dev", code: "DEV-DDD-003", title: "将 schema 初始化入口改为模块级 EnsureSchema 组合", status: "done", assignee: "Codex", evidence: "internal/appmain/schema_setup.go"},
		{table: "req_dev", code: "DEV-DDD-004", title: "增加架构守卫，禁止 appmain 回流业务代码并限制 domain/application 依赖方向", status: "done", assignee: "Codex", evidence: "internal/architecture/ddd_module_test.go"},
		{table: "req_unit", code: "UT-DDD-001", title: "架构守卫覆盖 appmain 组合根和 DDD 模块目录", status: "done", assignee: "Codex", evidence: "go test ./internal/architecture -count=1"},
		{table: "req_api", code: "API-DDD-001", title: "拆包后核心 Vue/API 路由保持兼容", status: "done", assignee: "Codex", evidence: "部署后 smoke: Vue shell/order/production/BOM APIs"},
		{table: "req_review", code: "REV-DDD-001", prCode: "PR-DDD-001", title: "架构验收：appmain 不再承载业务代码，业务按模块维护", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
		{table: "req_product", code: "PR-DDD-002", title: "继续收敛纯 DDD 终态：拆分生产大模块并清理剩余模板债", status: "review", assignee: "VA", evidence: "codex/pure-ddd-p2-20260426"},
		{table: "req_dev", code: "DEV-DDD-005", title: "将 production HTTP 聚合拆成 catalog/materials/bom/inventory/production 子模块", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/{catalog,materials,bom,inventory,production}"},
		{table: "req_dev", code: "DEV-DDD-006", title: "分配日志迁移为 Vue/Vite view + JSON API，并删除 allocation_logs.html", status: "done", assignee: "Codex", evidence: "AllocationLogsView.vue; /api/produce/allocations"},
		{table: "req_dev", code: "DEV-DDD-007", title: "外包模板设置迁移为 Vue/Vite view + sales application service，并删除 outsource_settings.html", status: "done", assignee: "Codex", evidence: "OutsourceSettingsView.vue; Service.SaveOutsourceTemplate"},
		{table: "req_dev", code: "DEV-DDD-008", title: "禁止 HTTP 业务模块互相导入，跨模块共享逻辑下沉到 domain/infrastructure", status: "done", assignee: "Codex", evidence: "TestHTTPModulesDoNotImportSiblingHTTPModules"},
		{table: "req_unit", code: "UT-DDD-002", title: "架构守卫覆盖生产模块拆分、模板债清理和 HTTP sibling import 禁止规则", status: "done", assignee: "Codex", evidence: "go test ./internal/architecture -count=1"},
		{table: "req_api", code: "API-DDD-002", title: "分配日志与外包模板设置通过 Vue shell 入口和 JSON API 验证", status: "done", assignee: "Codex", evidence: "TestCatalogAndSettingsRoutesRedirectToVueShell; deployment smoke"},
		{table: "req_review", code: "REV-DDD-002", prCode: "PR-DDD-002", title: "架构验收：生产/库存/BOM/物料边界清晰且剩余模板债已删除", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
		{table: "req_product", code: "PR-DDD-003", title: "纯 DDD 收口：BOM 持久化下沉、生产运行编排进 application，并删除订单旧模板", status: "review", assignee: "VA", evidence: "codex/pure-ddd-final-20260426"},
		{table: "req_dev", code: "DEV-DDD-009", title: "BOM Postgres adapter 从 HTTP interface 迁到 infrastructure/postgres/bom", status: "done", assignee: "Codex", evidence: "internal/infrastructure/postgres/bom/repository.go"},
		{table: "req_dev", code: "DEV-DDD-010", title: "生产开始流程由 application service 负责筛选、校验和用例编排", status: "done", assignee: "Codex", evidence: "internal/application/production/running_service.go"},
		{table: "req_dev", code: "DEV-DDD-011", title: "删除 order_edit/order_detail 旧模板并增加架构守卫防回流", status: "done", assignee: "Codex", evidence: "TestLegacyOrderTemplatesAreRemoved"},
		{table: "req_unit", code: "UT-DDD-003", title: "架构守卫覆盖 BOM adapter 位置、生产运行用例边界和订单旧模板删除", status: "done", assignee: "Codex", evidence: "go test ./internal/architecture -count=1"},
		{table: "req_api", code: "API-DDD-003", title: "BOM 与生产运行接口模块在 DDD 收口后保持兼容", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/bom ./internal/interfaces/http/production ./internal/interfaces/http/support -count=1"},
		{table: "req_review", code: "REV-DDD-003", prCode: "PR-DDD-003", title: "架构验收：剩余 P2/P3 收口后不再存在这轮发现的 DDD 违例", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
		{table: "req_product", code: "PR-074", title: "将 Excel 成本核算与豆单生成逻辑迁移到 ERP：成本试算、价格发布、豆单预览", status: "review", assignee: "VA", evidence: "codex/excel-costing-erp-ddd"},
		{table: "req_dev", code: "DEV-074-01", title: "实现 DDD 成本核算公式引擎：物料/BOM/生产参数 -> 成本/供应价/零售价/挂耳价", status: "done", assignee: "Codex", evidence: "internal/domain/costing; TestEngineMatchesExcelCachedGoldens"},
		{table: "req_dev", code: "DEV-074-02", title: "实现成本核算 application、Postgres adapter、API、试算批次保存和发布价格", status: "done", assignee: "Codex", evidence: "internal/application/costing; internal/infrastructure/postgres/costing; internal/interfaces/http/costing"},
		{table: "req_dev", code: "DEV-074-03", title: "实现 Vue 成本核算入口和豆单预览页面", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/views/CostingView.vue"},
		{table: "req_unit", code: "UT-074-01", title: "成本核算公式与 Excel 缓存金标准对齐，误差 <= 0.0001", status: "done", assignee: "Codex", evidence: "go test ./internal/domain/costing -count=1"},
		{table: "req_unit", code: "UT-074-02", title: "成本 application 校验、试算保存编排和发布 ID 校验通过", status: "done", assignee: "Codex", evidence: "go test ./internal/application/costing -count=1"},
		{table: "req_api", code: "API-074-01", title: "GET /api/costing/parameters 返回生产参数", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/costing route test"},
		{table: "req_api", code: "API-074-02", title: "POST /api/costing/calculate 返回成本/供应价/零售价/挂耳价", status: "done", assignee: "Codex", evidence: "TestCostingCalculateAPI"},
		{table: "req_api", code: "API-074-03", title: "GET /api/costing/bean-list 返回豆单预览数据", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/costing route test"},
		{table: "req_api", code: "API-074-04", title: "POST /api/costing/runs/:id/publish 发布价格并写审计日志", status: "done", assignee: "Codex", evidence: "Postgres adapter PublishRun + audit"},
		{table: "req_review", code: "REV-074-01", prCode: "PR-074", title: "ERP 成本核算结果可对齐 Excel 样本，豆单预览可打开，发布后录单使用新价格", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
	} {
		if err := seedReqRow(ctx, pool, schema, row); err != nil {
			return err
		}
	}
	return nil
}
