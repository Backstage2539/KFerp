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
		{table: "req_product", code: "PR-ERP-001", title: "参考 ERPNext 建立库存批次、原料入库、BOM版本、工单工序和成本闭环", status: "review", assignee: "VA", evidence: "docs/superpowers/plans/2026-04-27-erpnext-p0-p3.md"},
		{table: "req_dev", code: "DEV-ERP-001", title: "新增 stock DDD 模块，提供库存流水、库存批次、原料批次查询 API 和 Vue 页面", status: "done", assignee: "Codex", evidence: "internal/application/stock; internal/interfaces/http/stock; StockLedgerView/StockBatchesView/MaterialBatchesView"},
		{table: "req_dev", code: "DEV-ERP-002", title: "新增原料入库单，提交后创建 material_batches、stock_batches 和 stock_ledger_entries", status: "done", assignee: "Codex", evidence: "postgres/stock.ReceiveMaterial; MaterialReceiptsView.vue"},
		{table: "req_dev", code: "DEV-ERP-003", title: "生产完工按 material_batches FIFO 扣料，并在消耗日志和库存流水记录原料批次号", status: "done", assignee: "Codex", evidence: "materialBatchAllocationsTx; material_consumption_logs.material_batch_code"},
		{table: "req_dev", code: "DEV-ERP-004", title: "新增库存调整单，提交后更新库存并写 stock_adjustments、stock_batches 和 ledger", status: "done", assignee: "Codex", evidence: "stock.CreateAdjustment; StockAdjustmentsView.vue"},
		{table: "req_dev", code: "DEV-ERP-005", title: "BOM 支持版本保存和启用，启用版本会覆盖当前商品 BOM", status: "done", assignee: "Codex", evidence: "bom_versions/bom_version_items; /api/bom/versions"},
		{table: "req_dev", code: "DEV-ERP-006", title: "生产开始生成 Work Order 和 Job Card，生产完成关闭工单并记录批次成本", status: "done", assignee: "Codex", evidence: "work_orders/job_cards/production_batch_costs; WorkOrdersView/JobCardsView/ProductionCostsView"},
		{table: "req_unit", code: "UT-ERP-001", title: "库存 FIFO 领域规则、stock application 校验、生产工单状态和成本规则单测", status: "done", assignee: "Codex", evidence: "go test ./internal/domain/stock ./internal/application/stock ./internal/domain/production"},
		{table: "req_api", code: "API-ERP-001", title: "库存 API、BOM版本 API、生产工单/工序/成本 API handler 级验证", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/stock ./internal/interfaces/http/bom ./internal/interfaces/http/production"},
		{table: "req_review", code: "REV-ERP-001", prCode: "PR-ERP-001", title: "验收：P0-P3 ERPNext 主干页面和 API 可用，生产链路生成工单、批次、流水和成本", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
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
		{table: "req_product", code: "PR-075", title: "成本参数设置、物料批次号/生豆信息卡、商用批发四档梯度和零售豆单", status: "review", assignee: "VA", evidence: "codex/costing-settings-material-meta"},
		{table: "req_dev", code: "DEV-075-01", title: "成本参数设置 API/UI 可维护 Excel 迁移参数", status: "done", assignee: "Codex", evidence: "GET/POST /api/costing/settings"},
		{table: "req_dev", code: "DEV-075-02", title: "物料档案增加批次号、风味、产地、处理站、品种、处理法、等级、海拔和豆单信息", status: "done", assignee: "Codex", evidence: "materials schema + repository"},
		{table: "req_dev", code: "DEV-075-03", title: "成本试算和豆单预览展示商用 2-13磅、14-23磅、24-47磅、大于47磅四档与零售价", status: "done", assignee: "Codex", evidence: "domain costing + CostingView"},
		{table: "req_unit", code: "UT-075-01", title: "商用批发梯度、批次号默认今天、需求种子覆盖", status: "done", assignee: "Codex", evidence: "go test costing/materials/support focused tests"},
		{table: "req_api", code: "API-075-01", title: "成本设置接口支持读取和更新参数，价格试算返回四档梯度", status: "done", assignee: "Codex", evidence: "TestCostingSettingsAPI + route test"},
		{table: "req_review", code: "REV-075-01", prCode: "PR-075", title: "验收：商用四档批发价格、零售豆单、物料批次和风味/产地信息在 ERP 可见可用", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-076", title: "物料档案通用字段保持精简，咖啡豆物料特征迁移到 material_bean_profiles 子表", status: "review", assignee: "VA", evidence: "codex/material-bean-profile-table"},
		{table: "req_dev", code: "DEV-076-01", title: "新增咖啡豆物料子表 material_bean_profiles，迁移并停止使用材料主表中的生豆特征列", status: "done", assignee: "Codex", evidence: "materials schema + repository"},
		{table: "req_dev", code: "DEV-076-02", title: "物料 API 使用 bean_profile 子对象，只在 kind=bean 时读写生豆特征", status: "done", assignee: "Codex", evidence: "materials application DTO + Vue material view"},
		{table: "req_dev", code: "DEV-076-03", title: "成本核算豆单字段改为从 material_bean_profiles 读取，进货价仍来自 materials.purchase_price", status: "done", assignee: "Codex", evidence: "costing postgres repository"},
		{table: "req_unit", code: "UT-076-01", title: "材料主表 DDL 不含生豆特征列，子表存在，bean_profile 仅对生豆保留", status: "done", assignee: "Codex", evidence: "schema/material repository focused tests"},
		{table: "req_api", code: "API-076-01", title: "物料接口返回 bean_profile 子对象，成本豆单接口继续返回风味/产地信息", status: "done", assignee: "Codex", evidence: "postdeploy smoke"},
		{table: "req_review", code: "REV-076-01", prCode: "PR-076", title: "验收：包材物料不显示生豆字段，生豆物料可维护咖啡豆信息并进入豆单", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-077", title: "物料列表咖啡豆信息改为弹框设置，并从 Excel 生豆信息卡导入现有资料", status: "review", assignee: "VA", evidence: "codex/material-bean-profile-modal-import"},
		{table: "req_dev", code: "DEV-077-01", title: "物料列表只显示咖啡豆信息摘要和设置按钮，详细字段在咖啡豆信息弹框维护", status: "done", assignee: "Codex", evidence: "MaterialsView.vue profile-modal"},
		{table: "req_dev", code: "DEV-077-02", title: "从 Excel 生豆信息卡抽取资料并导入 material_bean_profiles 子表", status: "done", assignee: "Codex", evidence: "Excel 生豆信息导入 SQL 与 postdeploy API smoke"},
		{table: "req_unit", code: "UT-077-01", title: "MaterialsView 源码守卫覆盖咖啡豆信息弹框，不允许回到表格内联编辑", status: "done", assignee: "Codex", evidence: "TestMaterialsViewUsesModalForBeanProfile"},
		{table: "req_api", code: "API-077-01", title: "导入后物料 API 返回 bean_profile，成本豆单继续读取风味/产地", status: "done", assignee: "Codex", evidence: "postdeploy materials + costing smoke"},
		{table: "req_review", code: "REV-077-01", prCode: "PR-077", title: "验收：物料列表清爽，点击设置可维护生豆信息，Excel 生豆信息已导入", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-078", title: "成本参数按分类展示并带小字说明，成本核算页可右侧快速设置", status: "review", assignee: "VA", evidence: "codex/erpnext-p0-p3-20260427"},
		{table: "req_dev", code: "DEV-078-01", title: "成本参数设置页按基础换算、生产包装、商用熟豆、零售熟豆、挂耳分类展示并显示说明", status: "done", assignee: "Codex", evidence: "CostingSettingsPanel.vue + costing-settings.js"},
		{table: "req_dev", code: "DEV-078-02", title: "成本核算页提供右侧快速参数抽屉，保存参数后刷新当前试算", status: "done", assignee: "Codex", evidence: "CostingView.vue drawer + CostingSettingsPanel"},
		{table: "req_unit", code: "UT-078-01", title: "成本参数分类、说明、排序和未知参数兜底覆盖", status: "done", assignee: "Codex", evidence: "node --test src/lib/costing-settings.test.js"},
		{table: "req_api", code: "API-078-01", title: "成本设置仍复用 GET /api/costing/settings 与 POST /api/costing/settings/:key", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/costing -run TestCostingSettingsAPI -count=1"},
		{table: "req_review", code: "REV-078-01", prCode: "PR-078", title: "验收：成本参数有分类和说明，成本核算页可不跳转快速修改参数", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-079", title: "物料档案/库存改成物料主从详情页，旧物料只能废弃并复制新物料维护基础字段", status: "review", assignee: "VA", evidence: "codex/material-master-detail-profiles"},
		{table: "req_dev", code: "DEV-079-01", title: "左侧主列表只显示物料类别、物料名称、批次号，右侧详情显示完整信息", status: "done", assignee: "Codex", evidence: "MaterialsView.vue materials-layout"},
		{table: "req_dev", code: "DEV-079-02", title: "基础字段不可修改，支持废弃物料和复制保存为新物料", status: "done", assignee: "Codex", evidence: "materials Create/Update/Deprecate API"},
		{table: "req_dev", code: "DEV-079-03", title: "物料属性按类型区分：生豆使用咖啡豆属性，包材使用包材属性", status: "done", assignee: "Codex", evidence: "material_bean_profiles + material_pack_profiles"},
		{table: "req_unit", code: "UT-079-01", title: "覆盖物料主从详情页源码守卫、不可变字段校验和包材属性归属", status: "done", assignee: "Codex", evidence: "materials source/schema/repository tests"},
		{table: "req_api", code: "API-079-01", title: "覆盖复制新物料、废弃旧物料、包材属性返回和废弃物料默认隐藏", status: "done", assignee: "Codex", evidence: "TestMaterialsAPICreateCopyDeprecateAndPackProfile"},
		{table: "req_review", code: "REV-079-01", prCode: "PR-079", title: "验收：物料列表清爽，详情页可查看/复制/废弃，包材属性与生豆属性分开", status: "todo", assignee: "VA", evidence: "待 Van 功能分支服务器验收"},
		{table: "req_product", code: "PR-080", title: "物料详情库存不支持直接修改，只能通过入库、出库或库存补录调整且补录必须说明情况", status: "review", assignee: "VA", evidence: "codex/material-master-detail-profiles"},
		{table: "req_dev", code: "DEV-080-01", title: "物料详情库存字段改为只读，新增库存补录弹框并要求填写补录说明", status: "done", assignee: "Codex", evidence: "MaterialsView.vue 库存补录"},
		{table: "req_dev", code: "DEV-080-02", title: "普通物料更新拒绝修改 onhand 库存，库存补录复用 stock adjustment 流水", status: "done", assignee: "Codex", evidence: "assertImmutableMaterialFields stock adjustment"},
		{table: "req_unit", code: "UT-080-01", title: "覆盖物料页只读库存源码守卫、库存字段不可变校验和需求种子", status: "done", assignee: "Codex", evidence: "TestMaterialsViewDisallowsInlineStockAndUsesBackfill; TestAssertImmutableMaterialFieldsRejectsInlineStockChange; TestMaterialStockBackfillRequirementSeeds"},
		{table: "req_api", code: "API-080-01", title: "覆盖 POST /api/materials/:id 拒绝库存直接变更，库存补录走 POST /api/stock/adjustments 并保留 reason", status: "done", assignee: "Codex", evidence: "TestMaterialsAPIUpdateRejectsInlineStockChange + stock adjustment smoke"},
		{table: "req_review", code: "REV-080-01", prCode: "PR-080", title: "验收：物料库存不可直接改，点击库存补录填写说明后生成库存调整流水", status: "todo", assignee: "VA", evidence: "待 Van 功能分支服务器验收"},
		{table: "req_product", code: "PR-081", title: "修正 Excel 商用熟豆三套梯度：曲奇 kg 三档、454g 四档、227g 两档，并对齐 Nenka 2包-13包价格", status: "review", assignee: "VA", evidence: "codex/costing-tier-schemes"},
		{table: "req_dev", code: "DEV-081-01", title: "成本引擎支持 kg 三档、454g 四档、227g 两档，并返回 spec_g、min_qty、price_per_unit 元数据", status: "done", assignee: "Codex", evidence: "CommercialWholesaleTier scheme/spec/price_per_unit"},
		{table: "req_dev", code: "DEV-081-02", title: "按 Excel 产品名应用 Nenka 高利润、曲奇 kg 档和 227g 两档 profile，发布价格写入对应规格档", status: "done", assignee: "Codex", evidence: "ApplyExcelCommercialPricingProfile + PublishRun product_price_tiers"},
		{table: "req_unit", code: "UT-081-01", title: "覆盖 Nenka 127.431 金标准、24-49kg/50-99kg/100-199kg、2包-7包/8包+ 三套梯度", status: "done", assignee: "Codex", evidence: "go test ./internal/domain/costing -run TestNenkaExcelCommercialProfileMatchesWorkbook|TestCommercialWholesaleTiersSupportExcelSchemes"},
		{table: "req_api", code: "API-081-01", title: "POST /api/costing/calculate 返回梯度规格和单位价格，成本页动态展示不同梯度", status: "done", assignee: "Codex", evidence: "TestCostingCalculateAPIReturnsExcelTierSchemeMetadata"},
		{table: "req_review", code: "REV-081-01", prCode: "PR-081", title: "验收：Nenka 2包-13包为 127 左右，曲奇显示 kg 三档，227g 产品显示两档", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-082", title: "WIP共享池支持多天多商品共用领料，领料不消耗，生产完工从 WIP 扣料", status: "review", assignee: "VA", evidence: "codex/wip-shared-pool-20260427"},
		{table: "req_dev", code: "DEV-082-01", title: "新增仓库预设、material_batch_locations、material_transfers 和转仓流水", status: "done", assignee: "Codex", evidence: "stock schema + TransferMaterial"},
		{table: "req_dev", code: "DEV-082-02", title: "原料入库初始化原料仓批次位置，领料/退料只移动批次位置不减少物料总库存", status: "done", assignee: "Codex", evidence: "ReceiveMaterial + material_batch_locations"},
		{table: "req_dev", code: "DEV-082-03", title: "生产完工按 WIP 批次 FIFO 扣料，原料仓未领用库存不能直接被生产消耗", status: "done", assignee: "Codex", evidence: "materialBatchAllocationsTx warehouse=wip"},
		{table: "req_unit", code: "UT-082-01", title: "覆盖 WIP 转仓校验、批次位置移动、生产只能扣 WIP 和 Vue 入口源码守卫", status: "done", assignee: "Codex", evidence: "stock/production focused tests"},
		{table: "req_api", code: "API-082-01", title: "覆盖 GET /api/stock/warehouses、GET /api/stock/material-batch-locations、POST /api/stock/material-transfers", status: "done", assignee: "Codex", evidence: "TestStockAPIRoutes"},
		{table: "req_review", code: "REV-082-01", prCode: "PR-082", title: "验收：60kg 生豆可先领到 WIP，3天内被多个商品分批消耗，剩余料可留存或退回", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
		{table: "req_product", code: "PR-083", title: "对齐 Excel 熟豆豆单-3.0 和 零售豆单-3.0 的每个产品价格，豆单价格四舍五入且不显示小数", status: "review", assignee: "VA", evidence: "codex/costing-excel-price-audit"},
		{table: "req_dev", code: "DEV-083-01", title: "成本核算不再把已发布 default_price 当生豆成本，产品成本必须来自 BOM/物料进货价", status: "done", assignee: "Codex", evidence: "LoadProductInputs no default_price fallback"},
		{table: "req_dev", code: "DEV-083-02", title: "成本引擎按 Excel 豆单行返回整数商用价、零售价和动态零售规格 227g/250g 或 100g/200g", status: "done", assignee: "Codex", evidence: "RetailBeanTiers + rounded price outputs"},
		{table: "req_unit", code: "UT-083-01", title: "覆盖熟豆豆单-3.0 商用价格和零售豆单-3.0 零售价格的逐产品四舍五入金标准", status: "done", assignee: "Codex", evidence: "TestExcelBeanListCommercialPricesMatchRoundedWorkbook; TestExcelRetailBeanListPricesMatchRoundedWorkbook"},
		{table: "req_api", code: "API-083-01", title: "POST /api/costing/calculate 返回四舍五入后的商用/零售豆单价格和 retail_bean_tiers", status: "done", assignee: "Codex", evidence: "TestCostingCalculateAPIRoundsExcelBeanListPrices"},
		{table: "req_review", code: "REV-083-01", prCode: "PR-083", title: "验收：线上熟豆商用豆单和零售豆单逐产品价格与 Excel 整数价格一致", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-084", title: "生产工单带出烘焙建议并支持打印，旧物料总库存可生成期初批次用于 WIP 领料", status: "review", assignee: "VA", evidence: "codex/work-order-print-wip-fix-20260427"},
		{table: "req_dev", code: "DEV-084-01", title: "工单 API 返回烘焙度、建议投料、设备锅次、预计产出、订单和 BOM 原料摘要", status: "done", assignee: "Codex", evidence: "ListWorkOrders roast advice fields"},
		{table: "req_dev", code: "DEV-084-02", title: "生产工单 Vue 页面展示烘焙建议并提供单张工单打印版式", status: "done", assignee: "Codex", evidence: "WorkOrdersView.vue print sheet"},
		{table: "req_dev", code: "DEV-084-03", title: "stock schema 对有 onhand_g 但没有 material_batches 的旧物料补建 LEGACY-MAT 期初批次和原料仓库位", status: "done", assignee: "Codex", evidence: "ensureMaterialBatchTables legacy backfill"},
		{table: "req_unit", code: "UT-084-01", title: "覆盖旧物料库存补建批次后可领用到 WIP", status: "done", assignee: "Codex", evidence: "TestEnsureSchemaBackfillsLegacyMaterialOnhandIntoRawBatchLocation"},
		{table: "req_api", code: "API-084-01", title: "覆盖 GET /api/produce/work-orders 返回烘焙建议字段", status: "done", assignee: "Codex", evidence: "TestWorkOrderAPIIncludesRoastAdvice"},
		{table: "req_review", code: "REV-084-01", prCode: "PR-084", title: "验收：WO-0000000020 可领用孟连水洗5T批次到 WIP，工单可查看烘焙建议并打印", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-085", title: "商用批发豆单和零售豆单按 Excel 分类并编号，补充建议出品类型、风味和说明", status: "review", assignee: "VA", evidence: "codex/bean-list-excel-metadata"},
		{table: "req_dev", code: "DEV-085-01", title: "成本豆单 API 返回 commercial_bean_list / retail_bean_list 元数据：分类、编号、展示名、建议出品类型、风味和说明", status: "done", assignee: "Codex", evidence: "BeanListDisplay + Excel metadata mapping"},
		{table: "req_dev", code: "DEV-085-02", title: "成本核算页商用批发豆单和零售豆单按 Excel 分类分组、编号排序，并展示建议出品类型和说明", status: "done", assignee: "Codex", evidence: "CostingView commercialGroups/retailGroups"},
		{table: "req_unit", code: "UT-085-01", title: "覆盖熟豆豆单-3.0 和零售豆单-3.0 的分类编号、建议出品类型、风味和说明金标准", status: "done", assignee: "Codex", evidence: "TestExcelBeanListDisplayMetadataMatchesWorkbook"},
		{table: "req_api", code: "API-085-01", title: "POST /api/costing/calculate 返回 Excel 豆单展示元数据，Vue 源码守卫覆盖分类分组展示", status: "done", assignee: "Codex", evidence: "TestCostingCalculateAPIReturnsExcelBeanListDisplayMetadata; TestCostingViewGroupsBeanListsByExcelCategoryAndShowsMetadata"},
		{table: "req_review", code: "REV-085-01", prCode: "PR-085", title: "验收：商用批发豆单和零售豆单分类、编号、建议出品类型、风味与 Excel 一致", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-086", title: "成本核算页支持划出弹窗生成PDF格式豆单，商用单和零售单从 V3.0.5 开始并适配手机查看", status: "review", assignee: "VA", evidence: "codex/bean-list-pdf-export"},
		{table: "req_dev", code: "DEV-086-01", title: "生成豆单 PDF 抽屉支持商用/零售切换、版本号、背景颜色、字体颜色和背景上传设置", status: "done", assignee: "Codex", evidence: "CostingView PDF drawer + bean-list-pdf helper"},
		{table: "req_dev", code: "DEV-086-02", title: "PDF 打印版按 Excel 风格显示分类、编号、建议出品类型、风味、说明和价格，并使用手机宽度版式", status: "done", assignee: "Codex", evidence: "bean-list-pdf-page @media print max-width 430px"},
		{table: "req_unit", code: "UT-086-01", title: "覆盖默认版本 V3.0.5、商用/零售分组和 PDF 主题参数清洗", status: "done", assignee: "Codex", evidence: "node --test src/lib/bean-list-pdf.test.js"},
		{table: "req_api", code: "API-086-01", title: "覆盖成本页 PDF 生成入口、背景上传、打印样式和移动端 PDF 源码守卫", status: "done", assignee: "Codex", evidence: "TestCostingViewHasBeanListPDFDrawerAndMobilePrintStyles"},
		{table: "req_review", code: "REV-086-01", prCode: "PR-086", title: "验收：商用豆单和零售豆单可分别设置样式并保存为手机友好的 PDF", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-087", title: "库存入口按仓库视角整合：仓库库存统一查询，库存作业集中处理入库、WIP领退和盘点调整", status: "review", assignee: "VA", evidence: "codex/warehouse-inventory-menu-20260427"},
		{table: "req_dev", code: "DEV-087-01", title: "新增仓库库存 read model/API/Vue 页面，按仓库、物品、批次展示原料、包材、WIP 和成品余额", status: "done", assignee: "Codex", evidence: "GET /api/stock/warehouse-inventory; WarehouseInventoryView.vue"},
		{table: "req_dev", code: "DEV-087-02", title: "左树菜单改为可折叠大菜单，库存管理只保留仓库库存、库存作业、物料档案三个主入口", status: "done", assignee: "Codex", evidence: "menu-ia.js; App.vue"},
		{table: "req_dev", code: "DEV-087-03", title: "旧成品库存改造：成品库存从手工覆盖改成成品入库、盘点、转仓单据驱动，并支持多成品仓", status: "done", assignee: "Codex", evidence: "finished_inventory warehouse PK; finished_product_transfers; StockOperations 成品转仓"},
		{table: "req_dev", code: "DEV-087-04", title: "旧生产追溯改造：工单冻结 BOM/原料快照，贯通原料入库、转仓、生产消耗和成品批次链路", status: "done", assignee: "Codex", evidence: "produce_running_items/work_orders material_snapshot; /api/stock/trace"},
		{table: "req_unit", code: "UT-087-01", title: "覆盖库存菜单 IA：重叠旧页面不进入主菜单，当前大菜单可持久化展开", status: "done", assignee: "Codex", evidence: "node --test src/lib/menu-ia.test.js"},
		{table: "req_unit", code: "UT-087-02", title: "覆盖成品多仓转仓校验、工单物料快照源码守卫和仓库库存追溯入口", status: "done", assignee: "Codex", evidence: "TestTransferFinishedProductNormalizesWarehouseAndDefaultsOperator; TestProductionSourceStoresAndUsesMaterialSnapshot; TestVueStockWorkspaceIncludesFinishedTransferAndTraceLookup"},
		{table: "req_api", code: "API-087-01", title: "覆盖 GET /api/stock/warehouse-inventory 按仓库返回物品、批次、数量和成本字段", status: "done", assignee: "Codex", evidence: "TestStockAPIRoutes warehouse inventory"},
		{table: "req_api", code: "API-087-02", title: "覆盖 POST /api/stock/finished-transfers、POST /api/stock/adjustments 成品仓库字段和 GET /api/stock/trace", status: "done", assignee: "Codex", evidence: "TestStockAPIRoutes finished transfer + trace; TestStockAdjustmentsAPIRecordsFinishedWarehouse"},
		{table: "req_review", code: "REV-087-01", prCode: "PR-087", title: "验收：左树菜单可折叠，库存主入口不再重复；仓库库存可查分仓/批次，库存作业可处理常用单据", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
		{table: "req_review", code: "REV-087-02", prCode: "PR-087", title: "验收：成品可按仓盘点和转仓，生产工单冻结物料快照，仓库库存内可按 FP 批次追溯原料批次", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
		{table: "req_product", code: "PR-088", title: "修正 PDF 豆单避免只生成曲奇，并在生成前预览完整报价、风味、特点、出品建议", status: "review", assignee: "VA", evidence: "codex/bean-list-pdf-preview-fix"},
		{table: "req_dev", code: "DEV-088-01", title: "生成豆单 PDF 抽屉的预览区域渲染完整产品卡片，而不是只显示分类数量", status: "done", assignee: "Codex", evidence: "CostingView PDF preview product cards"},
		{table: "req_dev", code: "DEV-088-02", title: "PDF 打印样式允许分类跨页流动，单个产品卡尽量不跨页，避免只生成曲奇", status: "done", assignee: "Codex", evidence: "CostingView PDF print CSS"},
		{table: "req_unit", code: "UT-088-01", title: "源码守卫覆盖 PDF 预览必须包含报价、风味、特点、出品建议和产品价格卡片", status: "done", assignee: "Codex", evidence: "TestCostingViewPDFPreviewShowsFullBeanCardsBeforePrinting"},
		{table: "req_api", code: "API-088-01", title: "接口级源码守卫覆盖 PDF 分组分页不能整组禁止换页", status: "done", assignee: "Codex", evidence: "TestCostingViewPDFPrintDoesNotKeepWholeGroupsOnOnePage"},
		{table: "req_review", code: "REV-088-01", prCode: "PR-088", title: "验收：PDF 生成前能预览完整商用/零售豆单，生成 PDF 不再只包含曲奇", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-089", title: "成本核算改名为产品设置，合并旧商品档案并支持商品一、二级分类拖拽编号", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-089-01", title: "新增产品设置 API：返回分类树和商品列表，并支持创建分类、拖动二级分类、拖入某个分类", status: "done", assignee: "Codex", evidence: "GET /api/product-settings; category/product assignment APIs"},
		{table: "req_dev", code: "DEV-089-02", title: "产品设置页面合并旧商品档案和旧成本核算，支持一级分类、二级分类和商品编号从 1 开始排序", status: "done", assignee: "Codex", evidence: "ProductSettingsView.vue"},
		{table: "req_dev", code: "DEV-089-03", title: "旧商品档案删除阶梯价编辑，基础商品信息与成本核算发布价格继续联动", status: "done", assignee: "Codex", evidence: "ProductsView.vue; UpdateProductBasics"},
		{table: "req_unit", code: "UT-089-01", title: "源码守卫覆盖产品设置入口、拖拽分类、旧商品档案阶梯价编辑删除和需求种子", status: "done", assignee: "Codex", evidence: "TestProductSettingsRequirementSeeds; TestProductSettingsVueWiringAndLegacyTierEditorRemoval; menu-ia.test.js"},
		{table: "req_api", code: "API-089-01", title: "覆盖 /api/product-settings 分类树、创建分类、移动二级分类、商品归类和旧页面重定向", status: "done", assignee: "Codex", evidence: "TestProductSettingsAPISupportsCategoryTreeAndDragAssignments; TestLegacyProductAndCostingRoutesRedirectToProductSettings"},
		{table: "req_review", code: "REV-089-01", prCode: "PR-089", title: "验收：产品设置替代成本核算/商品档案主入口，分类与商品拖拽编号可保存，旧商品档案不再编辑阶梯价", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-090", title: "优化产品设置：二级分类拖动时显示插入横线，商品基础信息直接设置产品出品率并支持面板收起", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-090-01", title: "修正二级分类同级排序保存逻辑，并在拖动时用横线显示插入位置", status: "done", assignee: "Codex", evidence: "placeCategoryPosition; ProductSettingsView category-drop-line"},
		{table: "req_dev", code: "DEV-090-02", title: "商品基础信息删除默认价、零售价和操作列，列表内直接编辑烘焙度与产品出品率", status: "done", assignee: "Codex", evidence: "ProductSettingsView yield editor; PUT /api/products yield_rate"},
		{table: "req_dev", code: "DEV-090-03", title: "商品分类和商品基础信息支持收起/展开", status: "done", assignee: "Codex", evidence: "ProductSettingsView categoryCollapsed/productsCollapsed"},
		{table: "req_unit", code: "UT-090-01", title: "覆盖产品设置拖拽横线、出品率列表编辑、移除价格列和面板收起源码守卫", status: "done", assignee: "Codex", evidence: "TestProductSettingsCategoryDragYieldAndCollapseRefinements"},
		{table: "req_api", code: "API-090-01", title: "覆盖产品设置 API 返回 yield_rate，PUT /api/products 可保存 yield_rate", status: "done", assignee: "Codex", evidence: "TestProductSettingsAPIUpdatesProductYieldRate"},
		{table: "req_review", code: "REV-090-01", prCode: "PR-090", title: "验收：二级分类拖拽排序可保存且有横线位置提示，商品基础列表可直接设置产品出品率并可收起", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-091", title: "生产流程用户手册展示在前端页面，操作人员可在系统内查看图表化流程", status: "review", assignee: "VA", evidence: "codex/production-manual-page-20260427"},
		{table: "req_dev", code: "DEV-091-01", title: "新增 Vue 生产流程手册页面并挂入生产管理菜单，覆盖原料入库、WIP、工单、完工入库和追溯", status: "done", assignee: "Codex", evidence: "ProductionManualView.vue; menu-ia.js"},
		{table: "req_unit", code: "UT-091-01", title: "覆盖生产手册菜单入口和 Vue 页面源码守卫", status: "done", assignee: "Codex", evidence: "menu-ia.test.js; TestProductionManualVueWiring"},
		{table: "req_api", code: "API-091-01", title: "接口级源码守卫覆盖生产手册 Vue 注册、菜单入口和需求种子", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support -run TestProductionManual"},
		{table: "req_review", code: "REV-091-01", prCode: "PR-091", title: "验收：生产管理菜单中可打开生产手册，并能查看流程图、仓库说明、操作步骤、常见问题和检查清单", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
	} {
		if err := seedReqRow(ctx, pool, schema, row); err != nil {
			return err
		}
	}
	return nil
}
