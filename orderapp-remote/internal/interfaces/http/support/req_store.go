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
		{table: "req_product", code: "PR-AUTH-001", title: "建立用户角色和页面/API 权限 P0：管理员分配员工角色，菜单和接口按权限拦截", status: "review", assignee: "VA", evidence: "codex/user-permissions-p0-20260427"},
		{table: "req_dev", code: "DEV-AUTH-001", title: "新增 authz application service，统一角色、权限、页面可见性和员工角色分配规则", status: "done", assignee: "Codex", evidence: "internal/application/authz"},
		{table: "req_dev", code: "DEV-AUTH-002", title: "新增 authz Postgres schema/repository，种子管理员、销售、生产、仓库、财务、商品、系统角色", status: "done", assignee: "Codex", evidence: "internal/infrastructure/postgres/authz"},
		{table: "req_dev", code: "DEV-AUTH-003", title: "新增 /api/auth/me、角色列表、员工角色读取和保存 API，并保留 BasicAuth 管理员兜底", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/support/authz_api.go"},
		{table: "req_dev", code: "DEV-AUTH-004", title: "API 权限中间件覆盖订单、客户、生产、库存、物料、BOM、产品、成本、设置、员工、审计和需求模块", status: "done", assignee: "Codex", evidence: "AuthorizationMiddleware"},
		{table: "req_dev", code: "DEV-AUTH-005", title: "Vue/Vite 左侧菜单按 allowed_views 过滤，新增用户权限页面给管理员分配角色", status: "done", assignee: "Codex", evidence: "App.vue; UserPermissionsView.vue"},
		{table: "req_unit", code: "UT-AUTH-001", title: "单测覆盖管理员全权限、角色权限合并去重、员工角色分配规范化和菜单过滤", status: "done", assignee: "Codex", evidence: "authz service tests; menu-permissions.test.js"},
		{table: "req_unit", code: "UT-AUTH-002", title: "单测覆盖默认角色和所有 Vue shell view key 权限种子", status: "done", assignee: "Codex", evidence: "postgres/authz schema tests"},
		{table: "req_unit", code: "UT-AUTH-003", title: "单测覆盖 schema 初始化顺序，company 先于 support/authz 执行", status: "done", assignee: "Codex", evidence: "TestSchemaSetupInitializesCompanyBeforeEmployeeDependentModules"},
		{table: "req_api", code: "API-AUTH-001", title: "API 测试覆盖 GET /api/auth/me 返回当前员工权限和可访问页面", status: "done", assignee: "Codex", evidence: "TestAuthMeReturnsCurrentActorPermissionsAndViews"},
		{table: "req_api", code: "API-AUTH-002", title: "API 测试覆盖 BasicAuth 管理员可作为全权限兜底访问 /api/auth/me", status: "done", assignee: "Codex", evidence: "TestAuthMeTreatsBasicAuthAsAdminFallback"},
		{table: "req_api", code: "API-AUTH-003", title: "API 测试覆盖权限中间件缺权限拒绝、有权限放行", status: "done", assignee: "Codex", evidence: "TestAuthorizationMiddlewareDeniesMissingPermission; TestAuthorizationMiddlewareAllowsMatchingPermission"},
		{table: "req_api", code: "API-AUTH-004", title: "API 测试覆盖员工角色读取和保存必须具备 auth.manage 权限", status: "done", assignee: "Codex", evidence: "TestAssignEmployeeRolesAPIRequiresAuthManage; TestListEmployeeRolesAPIRequiresAuthManage"},
		{table: "req_review", code: "REV-AUTH-001", prCode: "PR-AUTH-001", title: "验收：管理员能打开用户权限页分配角色；普通员工只看到授权菜单，未授权 API 返回 403", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
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
		{table: "req_product", code: "PR-092", title: "修正二级分类拖拽插入线可落位保存，并明确产品设置的 BOM出品率 与 BOM 配方维护共用 product_bom.yield_rate", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-092-01", title: "二级分类 dragend 延后清理拖拽状态，避免插入横线 drop 前状态被清空导致拖不上去", status: "done", assignee: "Codex", evidence: "ProductSettingsView scheduleClearDrag"},
		{table: "req_dev", code: "DEV-092-02", title: "产品设置列表将出品率标注为 BOM出品率，并继续通过 PUT /api/products 写入 product_bom.yield_rate", status: "done", assignee: "Codex", evidence: "ProductSettingsView; UpdateProductBasics; FetchProducts"},
		{table: "req_unit", code: "UT-092-01", title: "源码守卫覆盖拖拽状态延后清理、BOM出品率文案和 product_bom.yield_rate 单一来源", status: "done", assignee: "Codex", evidence: "TestProductSettingsDragEndAndBomYieldAreWiredToSingleSource"},
		{table: "req_api", code: "API-092-01", title: "接口级验证产品设置 API 的 yield_rate 读写同一个 product_bom.yield_rate 字段", status: "done", assignee: "Codex", evidence: "TestProductSettingsAPIUpdatesProductYieldRate; catalog repository source guard"},
		{table: "req_review", code: "REV-092-01", prCode: "PR-092", title: "验收：二级分类拖拽插入横线可保存位置，产品设置 BOM出品率 与 BOM 配方维护出品率一致", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-093", title: "二级分类拖拽改为指针定位，避免鼠标松开时未命中原生 drop target 导致回到原位置", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-093-01", title: "二级分类拖拽改为 pointer 事件按鼠标 Y 坐标持续计算插入线，鼠标松开后按当前插入线保存", status: "done", assignee: "Codex", evidence: "ProductSettingsView pointer category sorting"},
		{table: "req_unit", code: "UT-093-01", title: "源码守卫覆盖二级分类排序不再依赖原生 dragstart/drop，必须使用 pointerdown/move/up 和坐标计算", status: "done", assignee: "Codex", evidence: "TestProductSettingsSecondaryCategoryDragUsesPointerPositionInsteadOfNativeDrop"},
		{table: "req_api", code: "API-093-01", title: "沿用 /api/product-settings/categories/:id/move 保存拖拽后的 parent_id 和 position", status: "done", assignee: "Codex", evidence: "TestProductSettingsAPISupportsCategoryTreeAndDragAssignments"},
		{table: "req_review", code: "REV-093-01", prCode: "PR-093", title: "验收：二级分类拖到目标位置附近松开鼠标，会按横线位置保存，不再大多数时候回到原位置", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-094", title: "支持产品设置分类删除，删除分类后商品回到未分类", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-094-01", title: "新增 DELETE /api/product-settings/categories/:id，软删除一级/二级分类并清空相关商品分类", status: "done", assignee: "Codex", evidence: "DeleteProductCategory"},
		{table: "req_dev", code: "DEV-094-02", title: "产品设置分类树增加删除按钮和确认提示，删除成功后刷新分类树和未分类商品", status: "done", assignee: "Codex", evidence: "ProductSettingsView deleteCategory"},
		{table: "req_unit", code: "UT-094-01", title: "源码守卫覆盖分类删除按钮、确认提示、软删除和商品回未分类逻辑", status: "done", assignee: "Codex", evidence: "TestProductSettingsVueSupportsCategoryDelete; TestProductSettingsRepositorySoftDeletesCategoriesAndUnassignsProducts"},
		{table: "req_api", code: "API-094-01", title: "API 测试覆盖 DELETE 分类接口传递分类 ID 并返回成功", status: "done", assignee: "Codex", evidence: "TestProductSettingsAPISupportsCategoryTreeAndDragAssignments"},
		{table: "req_review", code: "REV-094-01", prCode: "PR-094", title: "验收：一级/二级分类可删除，删除后分类不再显示，商品进入未分类且未丢失", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-095", title: "高级豆单生成：价格试算支持折叠，豆单生成支持选择产品、分级显示、重排编号、样式配置、标签标红、发布撤回、logo 和品牌介绍", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-095-01", title: "价格试算支持折叠，生成豆单按钮移动到商业豆单上方并形成价格试算/商用豆单/零售豆单三段页面", status: "done", assignee: "Codex", evidence: "CostingView pricingCollapsed; bean-list-generate-bar"},
		{table: "req_dev", code: "DEV-095-02", title: "豆单生成支持选择产品、分级显示、选择展示分级、按展示内容重排编号、豆卡样式、表格样式和一行豆卡数量", status: "done", assignee: "Codex", evidence: "buildBeanListPdfGroups selectedProductIDs/showCategoryNumbers/visibleCategoryCodes/layoutStyle/cardsPerRow"},
		{table: "req_dev", code: "DEV-095-03", title: "每个产品支持 NEW/推荐标签、字段/价格关键词标红，豆单支持发布和撤回发布、版本号、历史更新日志、上传logo和品牌介绍", status: "done", assignee: "Codex", evidence: "bean-list publications API; CostingView badge/highlightTerms/logoImage/brandIntro"},
		{table: "req_unit", code: "UT-095-01", title: "单测覆盖豆单产品筛选、分级过滤、编号重排、样式配置、标签和标红拆分", status: "done", assignee: "Codex", evidence: "node --test src/lib/bean-list-pdf.test.js"},
		{table: "req_api", code: "API-095-01", title: "API 测试覆盖 GET/POST /api/costing/bean-list/publications 和 POST /withdraw 发布撤回流程", status: "done", assignee: "Codex", evidence: "TestBeanListPublicationAPI"},
		{table: "req_review", code: "REV-095-01", prCode: "PR-095", title: "验收：可按选择生成商用/零售豆单，编号随筛选重排，PDF 预览含样式、标签、标红、版本日志、logo/品牌介绍，并可发布/撤回", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-096", title: "豆单生成支持设置品牌名字，更新日志放在底部，并修复发布豆单 conn busy", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-096-01", title: "豆单样式设置增加品牌名字，标题和底部品牌使用该名称；更新日志移动到豆单底部", status: "done", assignee: "Codex", evidence: "CostingView brandName; pdf-bottom-changelog"},
		{table: "req_dev", code: "DEV-096-02", title: "修复发布豆单 conn busy：PublishBeanList 的 INSERT RETURNING 改为 QueryRow，避免未关闭 Rows 时继续写审计", status: "done", assignee: "Codex", evidence: "postgres costing repository PublishBeanList QueryRow"},
		{table: "req_unit", code: "UT-096-01", title: "单测/源码守卫覆盖品牌名标题、底部更新日志和 QueryRow 避免 conn busy", status: "done", assignee: "Codex", evidence: "bean-list-pdf.test; TestPublishBeanListUsesQueryRowBeforeAuditToAvoidBusyConnection; TestCostingViewSupportsConfigurableBeanListPublishingWorkflow"},
		{table: "req_api", code: "API-096-01", title: "发布豆单接口线上 smoke 覆盖 POST /api/costing/bean-list/publications 不再返回 conn busy", status: "done", assignee: "Codex", evidence: "postdeploy curl publish + withdraw smoke"},
		{table: "req_review", code: "REV-096-01", prCode: "PR-096", title: "验收：可设置品牌名字；更新日志显示在底部；点击发布豆单成功且不再提示 conn busy", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-097", title: "修正豆单卡片生成：价格文本可通过标红词标红，报价对齐，分类剩余产品动态列数，分类和产品选择联动", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-097-01", title: "标红词作用到价格展示文本，支持如 55/包 这样的价格内容标红", status: "done", assignee: "Codex", evidence: "priceDisplay + priceValueParts"},
		{table: "req_dev", code: "DEV-097-02", title: "豆卡报价改为固定 label/value 网格，价格右对齐且不换行；按分类剩余产品数量动态列数，避免一张/两张卡留下空列", status: "done", assignee: "Codex", evidence: "pdf-price-label/pdf-price-value; cardRows(group)"},
		{table: "req_dev", code: "DEV-097-03", title: "分类和产品选择融合在一个列表中，分类勾选/取消会批量选择或取消该分类产品，产品清空后分类不再生成", status: "done", assignee: "Codex", evidence: "categoryProductGroups; togglePdfCategoryProducts"},
		{table: "req_unit", code: "UT-097-01", title: "单测覆盖空产品/空分类不会生成豆单、价格文本标红；源码守卫覆盖豆卡报价对齐和分类产品联动", status: "done", assignee: "Codex", evidence: "bean-list-pdf.test; TestCostingViewSupportsConfigurableBeanListPublishingWorkflow"},
		{table: "req_api", code: "API-097-01", title: "接口级源码守卫覆盖生成豆单参数不再把空选择解释成全选", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/costing"},
		{table: "req_review", code: "REV-097-01", prCode: "PR-097", title: "验收：价格标红生效；一行 2/3 豆卡报价整齐；单卡/双卡不留空白；取消分类会取消对应产品且豆单不再出现该分类", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-098", title: "左树菜单太小，需要看起来更饱满并更方便点击", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-098-01", title: "Vue shell 左树一级菜单和子菜单增大字号、内边距、点击高度和当前模块选中态", status: "done", assignee: "Codex", evidence: "App.vue sidebar menu density"},
		{table: "req_unit", code: "UT-098-01", title: "源码守卫覆盖左树菜单宽度、字号、点击高度和当前模块高亮", status: "done", assignee: "Codex", evidence: "TestVueShellSidebarMenuHasLargeClickTargets"},
		{table: "req_api", code: "API-098-01", title: "接口级验证需求表包含左树菜单放大优化的 PR/DEV/UT/API/REV 记录", status: "done", assignee: "Codex", evidence: "TestSidebarMenuDensityRequirementSeeds"},
		{table: "req_review", code: "REV-098-01", prCode: "PR-098", title: "验收：左树菜单显示更饱满，一级菜单和子菜单点击区域更大", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-099", title: "优化豆单预览与打印：豆卡同排字段与报价对齐，末行单卡/双卡占满整排，删除标红价格档选项，打印只输出豆单预览内容", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-099-01", title: "豆卡同排字段与报价对齐：卡片等高，风味/特点使用统一行位，报价固定在卡片底部", status: "done", assignee: "Codex", evidence: "CostingView pdf-card-row/pdf-item alignment CSS"},
		{table: "req_dev", code: "DEV-099-02", title: "豆卡按行分组渲染，末行只剩单卡或双卡时按实际数量重算列宽，占满整排不留空白", status: "done", assignee: "Codex", evidence: "cardRows(group); cardRowStyle(row)"},
		{table: "req_dev", code: "DEV-099-03", title: "删除标红价格档输入，仅保留标红词；打印时把豆单节点 Teleport 到 body 并隐藏主应用，只输出豆单预览内容", status: "done", assignee: "Codex", evidence: "CostingView highlightTerms only; bean-list-pdf-printing print CSS"},
		{table: "req_unit", code: "UT-099-01", title: "源码守卫覆盖行分组豆卡、移除 redPriceLabels/标红价格档/可填55文案、打印只显示豆单节点", status: "done", assignee: "Codex", evidence: "TestCostingViewSupportsConfigurableBeanListPublishingWorkflow; bean-list-pdf.test"},
		{table: "req_api", code: "API-099-01", title: "接口级源码守卫和 Vue 构建覆盖豆单预览/打印布局调整不破坏现有生成数据", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/costing; npm run build"},
		{table: "req_review", code: "REV-099-01", prCode: "PR-099", title: "验收：豆卡风味/特点/报价同排对齐；末行单卡/双卡占满整行；无标红价格档输入；打印预览只包含豆单", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-100", title: "录单页改为高级紧凑视图，客户模糊搜索，商品下拉直接搜索，默认已付款未发货，并支持规格梯度价和手动单价", status: "review", assignee: "VA", evidence: "codex/order-entry-polish-20260427"},
		{table: "req_dev", code: "DEV-100-01", title: "录单 API 返回客户默认来源/订单类型和拼音索引，保存新单未传状态时付款状态默认已付款、发货状态默认未发货", status: "done", assignee: "Codex", evidence: "OrderForm customer defaults; SaveOrder default paid/unshipped"},
		{table: "req_dev", code: "DEV-100-02", title: "录单页客户模糊搜索并联动默认来源/订单类型，商品明细改为商品下拉直接搜索，不再保留单独搜索框", status: "done", assignee: "Codex", evidence: "OrderEntryView customer-combobox/product-combobox"},
		{table: "req_dev", code: "DEV-100-03", title: "商品明细支持 36g、80g、100g、227g、454g、500g、1000g、2.5kg，按规格和数量实时匹配梯度价并允许手动单价", status: "done", assignee: "Codex", evidence: "order-entry.js common specs + manual tier payload"},
		{table: "req_unit", code: "UT-100-01", title: "覆盖录单规格列表、商品/客户搜索、默认状态、梯度价和手动单价 payload", status: "done", assignee: "Codex", evidence: "node --test src/lib/order-entry.test.js; TestOrderEntryPolish"},
		{table: "req_api", code: "API-100-01", title: "覆盖 /api/order/form 客户默认字段和 POST /api/order 默认已付款未发货", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/sales -run TestOrderAPI"},
		{table: "req_review", code: "REV-100-01", prCode: "PR-100", title: "验收：录单视图更紧凑，客户和商品可模糊搜索，默认已付款/未发货，选商品后实时价格可改", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-101", title: "录单选中产品后展示所有价格梯度，默认首个梯度规格；快递处理移至订单列表，由生产完成订单生成快递录单 Excel", status: "review", assignee: "VA", evidence: "codex/order-list-shipping-20260427"},
		{table: "req_dev", code: "DEV-101-01", title: "录单商品明细展示所有规格/数量梯度价，选择商品后默认使用第一个价格梯度规格并实时计算小计", status: "done", assignee: "Codex", evidence: "OrderEntryView tier-prices; order-entry.js defaultWholesaleSpec"},
		{table: "req_dev", code: "DEV-101-02", title: "订单列表选择生产完成订单后按服务器快递模板生成快递录单 Excel，并返回下载入口", status: "done", assignee: "Codex", evidence: "OrdersView shipping actions; POST /api/orders/shipping-excel"},
		{table: "req_dev", code: "DEV-101-03", title: "快递 Excel 收件人来自客户，寄件人来自发货人设置，包裹件数 1、托寄物默认茶叶、重量 0.1、备注写订单明细", status: "done", assignee: "Codex", evidence: "buildOrdersShippingWorkbook; LoadOrderShippingExportData"},
		{table: "req_unit", code: "UT-101-01", title: "单测覆盖首个梯度默认规格、全梯度展示数据、录单不含快递处理入口和订单列表快递入口", status: "done", assignee: "Codex", evidence: "node --test src/lib/order-entry.test.js; TestBuildOrdersShippingWorkbookFillsSelectedOrdersOnSeparateRows"},
		{table: "req_api", code: "API-101-01", title: "接口级测试覆盖 POST /api/order 不生成快递 Excel，POST /api/orders/shipping-excel 验证导出 Excel 的收寄件、茶叶、重量和订单明细备注", status: "done", assignee: "Codex", evidence: "TestOrderAPISaveDoesNotGenerateShippingExcel; TestOrdersShippingExcelAPIGeneratesFromSelectedOrders"},
		{table: "req_review", code: "REV-101-01", prCode: "PR-101", title: "验收：录单选商品后所有梯度价清晰可见；订单列表选择生产完成订单后可下载快递录单 Excel", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-102", title: "客户可以通过网址直接访问已发布豆单，支持商用和零售两个公开链接", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-102-01", title: "新增免登录只读公开页 /public/bean-list/commercial 和 /public/bean-list/retail，只读取已发布豆单快照，不暴露编辑接口", status: "done", assignee: "Codex", evidence: "renderPublicBeanListPage; BasicAuth public skip"},
		{table: "req_dev", code: "DEV-102-02", title: "生成豆单抽屉在有已发布版本时展示客户访问链接并支持复制", status: "done", assignee: "Codex", evidence: "CostingView publicBeanListURL/copyPublicBeanListURL"},
		{table: "req_unit", code: "UT-102-01", title: "单测覆盖公开页渲染发布快照、鉴权放行公开路径、后台客户链接入口和需求种子", status: "done", assignee: "Codex", evidence: "TestPublicBeanListPageRendersPublishedSnapshot; TestBasicAuthAllowsPublicBeanListWithoutCredentials; TestBeanListPublicCustomerLinkRequirementSeeds"},
		{table: "req_api", code: "API-102-01", title: "接口级验证 GET /public/bean-list/:list_type 返回已发布豆单 HTML，未登录可访问且不泄露发布/撤回操作", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/costing ./internal/interfaces/http/support"},
		{table: "req_review", code: "REV-102-01", prCode: "PR-102", title: "验收：发布商用或零售豆单后，可把客户链接发给客户直接打开；撤回后客户链接不再展示该版本", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-103", title: "快递录单备注不写单价小计，发货人设置支持寄件人列表和默认寄件人，生成 Excel 可对部分订单单独指定寄件人", status: "review", assignee: "VA", evidence: "codex/per-order-sender-shipping-20260427"},
		{table: "req_dev", code: "DEV-103-01", title: "快递 Excel 备注只写订单号、商品、规格和数量，不写入单价和小计", status: "done", assignee: "Codex", evidence: "orderShippingRemark"},
		{table: "req_dev", code: "DEV-103-02", title: "发货人设置改为寄件人列表，支持新增/编辑/启用和设置默认寄件人，旧单条设置兼容为默认寄件人", status: "done", assignee: "Codex", evidence: "SenderSettingsView; sender_settings schema"},
		{table: "req_dev", code: "DEV-103-03", title: "订单列表生成快递录单时默认使用默认寄件人，并可对本次勾选订单中的某几单单独指定寄件人", status: "done", assignee: "Codex", evidence: "OrdersView selectedSenderID/orderSenderIDs; POST /api/orders/shipping-excel sender_id/order_senders"},
		{table: "req_unit", code: "UT-103-01", title: "单测/源码守卫覆盖备注去除单价小计、订单列表默认/单独寄件人选择和发货人设置列表 UI", status: "done", assignee: "Codex", evidence: "TestBuildOrderShippingWorkbookFillsTemplateDefaultsAndRemark; TestOrdersVueGeneratesShippingExcelForProductionCompletedSelection"},
		{table: "req_api", code: "API-103-01", title: "接口级测试覆盖快递 Excel 使用默认 sender_id、逐订单 order_senders 覆盖和发货人设置 API 返回 profiles/default profile", status: "done", assignee: "Codex", evidence: "TestOrdersShippingExcelAPIUsesPerOrderSenderOverrides; TestSenderSettingsAPIListsProfilesWithDefault"},
		{table: "req_review", code: "REV-103-01", prCode: "PR-103", title: "验收：快递备注无价格；发货人设置有默认寄件人列表；订单列表可为部分订单单独选择寄件人生成 Excel", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-104", title: "修复系统全功能 smoke 发现的 P1/P2/P3 问题，并重新合入 develop 部署验收", status: "review", assignee: "VA", evidence: "codex/fix-smoke-p1-p3-20260427"},
		{table: "req_dev", code: "DEV-104-01", title: "客户空订单聚合、生产开始 WIP 前置校验、物料/库存别名和机台投料量校验提示修复", status: "done", assignee: "Codex", evidence: "customer dashboard COALESCE; ensureWIPStockForRunningItemTx; normalize aliases"},
		{table: "req_dev", code: "DEV-104-02", title: "Vue/Vite 前端统一 API 请求和 history 状态更新，避免 BasicAuth URL 与页面状态不同源异常", status: "done", assignee: "Codex", evidence: "api client usage; url-state.js"},
		{table: "req_dev", code: "DEV-104-03", title: "运行时模板目录可配置，默认使用相对 templates，支持本地 go run 和 Docker 部署", status: "done", assignee: "Codex", evidence: "Runtime.TemplateDir"},
		{table: "req_unit", code: "UT-104-01", title: "单测覆盖客户聚合空值守卫、物料旧枚举兼容、库存 product 别名、机台错误文案、URL 状态和模板目录", status: "done", assignee: "Codex", evidence: "focused Go tests + node --test src/lib/*.test.js"},
		{table: "req_api", code: "API-104-01", title: "API 测试覆盖库存调整 product 别名和部署后 auth/me、需求表、商品、生产/WIP 主流程 smoke", status: "done", assignee: "Codex", evidence: "TestStockAdjustmentsAPIAcceptsProductAlias; postdeploy smoke"},
		{table: "req_review", code: "REV-104-01", prCode: "PR-104", title: "验收：P1/P2/P3 修复后现网页面/API 重新走通，需求表展示本次修复记录", status: "todo", assignee: "VA", evidence: "待部署后验收"},
		{table: "req_product", code: "PR-105", title: "生成豆单支持复制已有豆单配置，快速基于历史版本调整后再生成", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-105-01", title: "生成豆单抽屉增加复制已有豆单配置入口，可选择历史发布记录并复制版本、样式、品牌、分级展示、产品选择、标签和标红词", status: "done", assignee: "Codex", evidence: "CostingView copy-config-box/applyCopiedBeanListPublicationConfig"},
		{table: "req_dev", code: "DEV-105-02", title: "复制配置只带入配置，不复制旧 content 快照；当前最新产品和价格重新生成预览，避免沿用旧价格", status: "done", assignee: "Codex", evidence: "copyBeanListPublicationConfig + buildBeanListPdfGroups current items"},
		{table: "req_unit", code: "UT-105-01", title: "单测覆盖复制发布记录配置时过滤不存在的产品/分类，保留样式、标签、标红词和版本日志", status: "done", assignee: "Codex", evidence: "bean-list-pdf.test copyBeanListPublicationConfig; TestBeanListCopyPublishedConfigRequirementSeeds"},
		{table: "req_api", code: "API-105-01", title: "接口级源码守卫覆盖生成豆单复制配置入口和现有发布记录 API 复用，不新增后端接口", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/costing ./internal/interfaces/http/support"},
		{table: "req_review", code: "REV-105-01", prCode: "PR-105", title: "验收：生成豆单时可选一个历史豆单复制配置，修改后预览按最新产品和价格生成并可继续发布", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-106", title: "订单列表快递处理工具栏中默认寄件人下拉框和操作按钮控件底边对齐", status: "review", assignee: "VA", evidence: "codex/order-shipping-toolbar-align-20260428"},
		{table: "req_dev", code: "DEV-106-01", title: "调整订单列表快递处理工具栏布局，使默认寄件人控件与只看生产完成、勾选本页生产完成、生成 Excel 按底边对齐", status: "done", assignee: "Codex", evidence: "OrdersView shipping-bar/shipping-actions CSS"},
		{table: "req_unit", code: "UT-106-01", title: "源码守卫覆盖快递处理工具栏底部对齐样式，防止回退为居中错位", status: "done", assignee: "Codex", evidence: "TestOrdersVueGeneratesShippingExcelForProductionCompletedSelection; TestOrderShippingToolbarAlignmentRequirementSeeds"},
		{table: "req_api", code: "API-106-01", title: "接口级验证订单列表页面和需求表仍可访问，本次为前端布局调整不新增业务接口", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support; postdeploy smoke /vue-shell?view=orders /api/req/product"},
		{table: "req_review", code: "REV-106-01", prCode: "PR-106", title: "验收：订单列表快递处理区的寄件人下拉框和三个操作按钮视觉上同一底边对齐", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-107", title: "订单发货闭环：快递录单生成发货批次，回填快递单号后订单自动标记已发货", status: "review", assignee: "VA", evidence: "codex/p1-ops-20260428"},
		{table: "req_dev", code: "DEV-107-01", title: "新增订单发货批次和批次订单表，生成快递 Excel 时写入 shipment_id/shipment_no 和订单寄件人", status: "done", assignee: "Codex", evidence: "order_shipments; CreateOrderShipment"},
		{table: "req_dev", code: "DEV-107-02", title: "新增快递单号回填 API，按发货批次订单写入 tracking_no 并把订单发货状态更新为已发货", status: "done", assignee: "Codex", evidence: "POST /api/orders/shipping-tracking"},
		{table: "req_unit", code: "UT-107-01", title: "单测覆盖发货批次命令规范化、重复订单去重和单号回填命令默认操作人", status: "done", assignee: "Codex", evidence: "TestServiceOwnsShipmentClosureUseCases"},
		{table: "req_api", code: "API-107-01", title: "API 测试覆盖快递 Excel 创建发货批次、回填单号后订单已发货", status: "done", assignee: "Codex", evidence: "TestOrdersShippingExcelAPICreatesShipmentRecord; TestOrdersShippingTrackingAPIMarksOrdersShipped"},
		{table: "req_review", code: "REV-107-01", prCode: "PR-107", title: "验收：生产完成订单生成快递 Excel 后出现发货批次号，回填单号后订单状态为已发货", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-108", title: "采购入库 P1：供应商、采购单、采购收货与库存批次/流水打通", status: "review", assignee: "VA", evidence: "codex/p1-ops-20260428"},
		{table: "req_dev", code: "DEV-108-01", title: "新增 purchase application/postgres/http 模块，维护供应商、采购单和采购收货记录", status: "done", assignee: "Codex", evidence: "internal/application/purchase; internal/infrastructure/postgres/purchase; /api/purchase/*"},
		{table: "req_dev", code: "DEV-108-02", title: "采购收货复用库存原料入库能力，生成原料批次、库存批次、原料仓库存和库存流水，并更新物料采购价", status: "done", assignee: "Codex", evidence: "CreatePurchaseReceipt + stock.ReceiveMaterial"},
		{table: "req_dev", code: "DEV-108-03", title: "Vue/Vite 新增采购入库页面和采购权限菜单，支持供应商、采购单和一键收货", status: "done", assignee: "Codex", evidence: "PurchaseView; menu-ia purchase"},
		{table: "req_unit", code: "UT-108-01", title: "单测覆盖采购供应商/采购单命令规范化和采购收货调用库存入库与采购价更新", status: "done", assignee: "Codex", evidence: "TestPurchaseServiceCreatesSupplierAndPurchaseOrder; TestPurchaseReceiptReceivesStockAndUpdatesMaterialPurchasePrice"},
		{table: "req_api", code: "API-108-01", title: "API 测试覆盖 POST /api/purchase/receipts 返回库存批次号并传递操作人", status: "done", assignee: "Codex", evidence: "TestPurchaseReceiptAPIReceivesMaterial"},
		{table: "req_review", code: "REV-108-01", prCode: "PR-108", title: "验收：采购页可建供应商/采购单，收货后原料批次和库存流水增加，物料采购价更新", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-109", title: "账号治理 P1：用户权限页支持员工登录启停和管理员重置密码", status: "review", assignee: "VA", evidence: "codex/p1-ops-20260428"},
		{table: "req_dev", code: "DEV-109-01", title: "登录密码表新增 login_disabled/must_reset_password，登录与 Bearer 会话解析拒绝停用账号", status: "done", assignee: "Codex", evidence: "employee_login_passwords columns; resolveEmployeeBySessionToken"},
		{table: "req_dev", code: "DEV-109-02", title: "新增账号列表、账号启停和密码重置 API，要求 auth.manage 权限并写操作日志", status: "done", assignee: "Codex", evidence: "GET /api/auth/accounts; POST /api/auth/account-state; POST /api/auth/password/reset"},
		{table: "req_dev", code: "DEV-109-03", title: "用户权限 Vue 页面增加账号列，可切换可登录/已停用并重置员工密码", status: "done", assignee: "Codex", evidence: "UserPermissionsView account controls"},
		{table: "req_unit", code: "UT-109-01", title: "单测覆盖 support 鉴权接口和登录账号治理编译路径", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support"},
		{table: "req_api", code: "API-109-01", title: "API 级验证账号治理接口受 auth.manage 保护，停用账号不能继续登录或解析会话", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support; deployment smoke"},
		{table: "req_review", code: "REV-109-01", prCode: "PR-109", title: "验收：管理员可停用/启用员工登录并重置密码；停用员工无法登录和继续访问", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-110", title: "ERPNext制造补齐：物料需求计划、工单WIP软占用/部分完工与生产质检闭环", status: "review", assignee: "VA", evidence: "codex/manufacturing-planning-quality-20260428"},
		{table: "req_dev", code: "DEV-110-01", title: "生产计划新增物料需求计划，按 BOM 汇总需求并对比 WIP、原料仓和已占用库存，输出缺料和采购建议", status: "done", assignee: "Codex", evidence: "GET /api/produce/material-plan; ProducePlanView 物料需求计划"},
		{table: "req_dev", code: "DEV-110-02", title: "生产工单开始时创建 WIP软占用，开始生产校验扣除其他工单占用量，取消生产释放占用", status: "done", assignee: "Codex", evidence: "work_order_material_reservations; ensureWIPStockForNeedsTx"},
		{table: "req_dev", code: "DEV-110-03", title: "生产中支持部分完工，按本次消耗投料扣 WIP、入库成品，保留剩余工单继续生产", status: "done", assignee: "Codex", evidence: "FinishCommand Partial/ConsumedInputG; production_logs completion_no"},
		{table: "req_dev", code: "DEV-110-04", title: "新增生产质检闭环，覆盖原料、生产工单和成品批次，支持通过、待处理、不合格记录", status: "done", assignee: "Codex", evidence: "quality_inspections; /api/produce/quality-inspections; QualityInspectionsView"},
		{table: "req_unit", code: "UT-110-01", title: "单元测试覆盖应用服务归一化、WIP占用源码守卫、Vue 入口和需求种子", status: "done", assignee: "Codex", evidence: "TestServiceOwnsManufacturingGapUseCases; TestManufacturingGapSchemaAndReservationGuards; TestManufacturingPlanningQualityVueWiring"},
		{table: "req_api", code: "API-110-01", title: "API 测试覆盖物料需求计划、部分完工字段和生产质检新增/查询接口", status: "done", assignee: "Codex", evidence: "TestManufacturingGapAPIs"},
		{table: "req_review", code: "REV-110-01", prCode: "PR-110", title: "验收：生产计划能预先看缺料/采购建议，工单能分次完工，WIP占用不互相抢料，生产质检可记录追溯", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
		{table: "req_product", code: "PR-111", title: "P1-A 回填支持上传顺丰寄件列表 Excel，按备注中的订单号回填快递单号", status: "review", assignee: "VA", evidence: "codex/shipping-tracking-excel-20260428"},
		{table: "req_dev", code: "DEV-111-01", title: "新增回传 Excel 解析器，识别寄件列表中的运单号和备注/订单号列，并从备注提取 SO 订单号", status: "done", assignee: "Codex", evidence: "parseShipmentTrackingExcel"},
		{table: "req_dev", code: "DEV-111-02", title: "新增 Excel 上传回填 API，按订单号更新订单快递单号和已发货状态，并同步最近发货批次订单", status: "done", assignee: "Codex", evidence: "POST /api/orders/shipping-tracking-excel"},
		{table: "req_dev", code: "DEV-111-03", title: "订单列表 Vue 快递处理区新增回传 Excel 上传入口，上传后刷新订单列表", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/views/OrdersView.vue"},
		{table: "req_unit", code: "UT-111-01", title: "单测覆盖按订单号回填命令规范化、Excel 表头识别和备注订单号提取", status: "done", assignee: "Codex", evidence: "TestServiceNormalizesShipmentTrackingByOrderNo; TestParseShipmentTrackingExcelUsesWaybillAndRemarkOrderNo"},
		{table: "req_api", code: "API-111-01", title: "API 测试覆盖上传寄件列表 Excel 后订单按备注订单号标记已发货并写入快递单号", status: "done", assignee: "Codex", evidence: "TestOrdersShippingTrackingExcelAPIMarksOrdersByRemarkOrderNo"},
		{table: "req_review", code: "REV-111-01", prCode: "PR-111", title: "验收：上传顺丰寄件列表回传 Excel 后，备注订单号匹配的订单显示快递单号且发货状态为已发货", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-112", title: "客户豆单发布后锁定为自己的快照，可复制官方价格来源和自己的历史样式配置", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-112-01", title: "豆单发布记录增加归属 owner、官方价格来源、样式来源和来源版本，发布/撤回只影响同一归属的当前豆单", status: "done", assignee: "Codex", evidence: "bean_list_publications owner/source columns; PublishBeanList owner scoped withdraw"},
		{table: "req_dev", code: "DEV-112-02", title: "生成豆单抽屉支持官方豆单和我的客户豆单两种归属，客户豆单可复制官方价格来源作为锁定内容快照", status: "done", assignee: "Codex", evidence: "CostingView publicationScope; selectedPriceSourcePublicationID; copyBeanListPublicationContentGroups"},
		{table: "req_dev", code: "DEV-112-03", title: "客户豆单可复制自己的历史样式配置，发布时记录 price_source_publication_id 和 style_source_publication_id", status: "done", assignee: "Codex", evidence: "CostingView copy config + publish payload source ids"},
		{table: "req_unit", code: "UT-112-01", title: "单测覆盖发布记录归属/来源字段、数据库 owner 唯一发布约束和复制已发布内容快照", status: "done", assignee: "Codex", evidence: "TestPublishBeanListKeepsCustomerSnapshotOwnerAndSources; TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots; bean-list-pdf.test"},
		{table: "req_api", code: "API-112-01", title: "API 测试覆盖 scope=mine 发布客户豆单时返回 owner 和官方价格/样式来源 ID，公开页仍只读取官方发布", status: "done", assignee: "Codex", evidence: "TestBeanListPublicationAPI; TestPublicBeanListPageRendersPublishedSnapshot"},
		{table: "req_review", code: "REV-112-01", prCode: "PR-112", title: "验收：客户豆单复制官方价格和自己的样式后发布为独立快照，后续官方豆单更新不会自动改写客户已发布豆单", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-SALES-ORDER-001", title: "订单支持生成正式销售单 PDF，销售单设置可维护公司名称、说明、收款方式、多个收款码和公章", status: "review", assignee: "VA", evidence: "docs/superpowers/specs/2026-04-30-sales-order-pdf-design.md; docs/sales-order-user-manual.md; codex/sales-order-pdf-20260430"},
		{table: "req_dev", code: "DEV-SALES-ORDER-001", title: "新增销售单设置、版本化 PDF 生成、下载 API 和 Vue/Vite 页面入口", status: "done", assignee: "Codex", evidence: "internal/application/sales; internal/infrastructure/postgres/sales; internal/interfaces/http/sales; frontend-vue-shell SalesOrder*"},
		{table: "req_unit", code: "UT-SALES-ORDER-001", title: "覆盖销售单版本号、快照校验、金额格式化、PDF 收款码/公章图片渲染和前端链接辅助函数", status: "done", assignee: "Codex", evidence: "TestRenderSalesOrderPDFEmbedsPaymentCodeAndSealImages; go test ./...; node --test src/lib/*.test.js"},
		{table: "req_api", code: "API-SALES-ORDER-001", title: "覆盖销售单设置保存、付款码/公章上传、生成 V1/V2 和 PDF 下载接口", status: "done", assignee: "Codex", evidence: "TestSalesOrderSettingsAPI; TestSalesOrderDocumentAPI; TestGenerateSalesOrderDocumentCreatesVersions"},
		{table: "req_review", code: "REV-SALES-ORDER-001", prCode: "PR-SALES-ORDER-001", title: "验收：订单可生成并下载销售单 PDF；重新生成保留历史版本；设置快照不被后续修改覆盖", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-113", title: "只有管理员才可以发布豆单；客户登录后只能保存修改和下载豆单", status: "review", assignee: "VA", evidence: "codex/product-settings-categories"},
		{table: "req_dev", code: "DEV-113-01", title: "豆单发布和撤回接口必须校验管理员权限，非管理员即使有成本权限也不能发布或撤回", status: "done", assignee: "Codex", evidence: "requireBeanListPublisher; auth.manage permission split"},
		{table: "req_dev", code: "DEV-113-02", title: "新增客户豆单草稿保存接口，客户保存时强制归属为自己的 actor 快照，不影响官方已发布豆单", status: "done", assignee: "Codex", evidence: "POST /api/costing/bean-list/drafts; SaveBeanListDraft"},
		{table: "req_dev", code: "DEV-113-03", title: "生成豆单抽屉按当前账号角色显示操作：管理员可发布/撤回，客户只能保存修改和生成 PDF 下载", status: "done", assignee: "Codex", evidence: "CostingView fetchCurrentActor/isBeanListAdmin/saveBeanListDraft"},
		{table: "req_unit", code: "UT-113-01", title: "单测覆盖草稿保存服务、数据库草稿插入源码守卫、前端角色入口和需求种子", status: "done", assignee: "Codex", evidence: "TestSaveBeanListDraftValidatesAndKeepsCustomerOwner; TestSaveBeanListDraftInsertsCustomerDraftWithoutPublishing; TestCostingViewSupportsConfigurableBeanListPublishingWorkflow; TestBeanListPublishAdminOnlyRequirementSeeds"},
		{table: "req_api", code: "API-113-01", title: "API 测试覆盖非管理员发布豆单返回 403、客户保存草稿返回自己的 owner，并拆分发布/草稿权限", status: "done", assignee: "Codex", evidence: "TestBeanListPublicationPublishRequiresAdmin; TestBeanListDraftAPISavesCustomerOwnedDraft; TestBeanListPublicationPermissionsSeparatePublishAndDraft"},
		{table: "req_review", code: "REV-113-01", prCode: "PR-113", title: "验收：管理员账号可发布/撤回豆单；客户账号只能保存修改和下载 PDF，无法发布或撤回", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-114", title: "修复手机号密码登录后 Vue 系统仍未认证，并增加退出登录功能", status: "review", assignee: "VA", evidence: "codex/auth-login-logout-20260430"},
		{table: "req_dev", code: "DEV-114-01", title: "统一 Vue/Vite API 客户端从 localStorage.auth_token 自动发送 Bearer token，登录成功后可加载 Vue shell 并进入系统", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/api/client.js; auth_middleware.go; templates/login.html"},
		{table: "req_dev", code: "DEV-114-02", title: "新增退出登录入口，前端清除本地 token 并调用后端注销当前 login_session", status: "done", assignee: "Codex", evidence: "App.vue; POST /api/auth/logout"},
		{table: "req_unit", code: "UT-114-01", title: "单测覆盖 API 客户端自动携带 Bearer token 和 logout header 行为", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/api/client.test.js"},
		{table: "req_api", code: "API-114-01", title: "API 级验证退出登录端点保持鉴权保护，Bearer token 解析符合预期", status: "done", assignee: "Codex", evidence: "TestLogoutEndpointRequiresAuthentication; TestBearerTokenFromHeader"},
		{table: "req_review", code: "REV-114-01", prCode: "PR-114", title: "验收：13800138075 使用密码登录后可进入系统，点击退出后回到登录页且当前 token 不再继续使用", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-115", title: "生产验收与WIP占用可视化：上线后能快速检查生产闭环，并能查看/调整/释放工单WIP占用", status: "review", assignee: "VA", evidence: "codex/production-acceptance-wip-20260430"},
		{table: "req_dev", code: "DEV-115-01", title: "新增生产验收页面和接口，汇总仓库、原料、WIP、工单、日志、质检和成品追溯检查项", status: "done", assignee: "Codex", evidence: "GET /api/produce/acceptance-smoke; ProductionAcceptanceView"},
		{table: "req_dev", code: "DEV-115-02", title: "生产计划物料需求计划新增WIP可用量和建议领到WIP数量，帮助生产前完成领料", status: "done", assignee: "Codex", evidence: "MaterialPlanRow available_g/wip_transfer_suggestion_g; ProducePlanView"},
		{table: "req_dev", code: "DEV-115-03", title: "WIP占用可视化并支持按工单释放、按预约调整占用量，操作写入审计日志", status: "done", assignee: "Codex", evidence: "GET/POST /api/produce/wip-reservations; WorkOrdersView; WarehouseInventoryView"},
		{table: "req_unit", code: "UT-115-01", title: "单元测试覆盖WIP占用领域规则、应用服务归一化、Vue入口和需求种子", status: "done", assignee: "Codex", evidence: "TestWIPReservationRemainingAndAdjustment; TestServiceOwnsWIPReservationAndAcceptanceUseCases; TestProductionAcceptanceWIPVueWiring"},
		{table: "req_api", code: "API-115-01", title: "API测试覆盖生产验收、WIP占用列表、调整、释放和物料计划新增字段", status: "done", assignee: "Codex", evidence: "TestManufacturingGapAPIs"},
		{table: "req_review", code: "REV-115-01", prCode: "PR-115", title: "验收：生产验收页可看核心检查项；生产计划显示建议领到WIP；工单/仓库库存可查看并处理WIP占用", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
		{table: "req_product", code: "PR-116", title: "登录加固：支持用户名密码登录、admin 全权限菜单，并防止退出后 BasicAuth 缓存绕过登录", status: "review", assignee: "VA", evidence: "codex/auth-hardening-20260501"},
		{table: "req_dev", code: "DEV-116-01", title: "密码登录支持用户名或手机号作为登录标识，短信登录继续只允许手机号", status: "done", assignee: "Codex", evidence: "passwordLoginIdentifier; resolveEmployeeByPasswordLogin; login.html"},
		{table: "req_dev", code: "DEV-116-02", title: "Vue 权限判断识别 admin 角色为全权限，allowed_views 为 null 时不再误判为无权限", status: "done", assignee: "Codex", evidence: "actorHasFullViewAccess; App.vue allowedViewKeys"},
		{table: "req_dev", code: "DEV-116-03", title: "Vue 工作台加载前必须存在本机 Bearer token，退出后即使浏览器保留 BasicAuth 缓存也回到登录页", status: "done", assignee: "Codex", evidence: "hasStoredAuthToken; redirectToLogin; clearStoredAuthToken"},
		{table: "req_unit", code: "UT-116-01", title: "单测覆盖用户名登录标识、登录页用户名入口、admin 全权限菜单和退出后 token 预检查", status: "done", assignee: "Codex", evidence: "TestPasswordLoginIdentifierSupportsUsernameOrPhone; auth.test.js; menu-permissions.test.js"},
		{table: "req_api", code: "API-116-01", title: "API/线上 smoke 覆盖 13800138075 密码登录、用户名登录、auth/me 全权限、退出后 token 失效和无 token 工作台回登录页", status: "done", assignee: "Codex", evidence: "postdeploy curl/browser smoke"},
		{table: "req_review", code: "REV-116-01", prCode: "PR-116", title: "验收：13800138075 可用手机号或用户名密码登录并看到全权限菜单，退出后直接打开系统地址不能绕过登录", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-117", title: "客户档案支持客户公司名称、公司地址、联系电话；销售单生成时可维护客户信息并默认客户名为公司名", status: "review", assignee: "VA", evidence: "codex/customer-sales-order-company-20260501"},
		{table: "req_dev", code: "DEV-117-01", title: "客户模型、API、Postgres schema 增加 company_name、company_address、company_phone，并在客户列表/编辑页展示保存", status: "done", assignee: "Codex", evidence: "Customer service/repository/routes; CustomersView"},
		{table: "req_dev", code: "DEV-117-02", title: "销售单快照和 PDF 使用客户公司信息；销售单设置公司名为空时按客户公司名称再按客户名兜底，避免 company_name required", status: "done", assignee: "Codex", evidence: "SalesOrderSnapshot; buildSalesOrderSnapshotTx; sales_order_pdf"},
		{table: "req_dev", code: "DEV-117-03", title: "销售单页面右侧抽屉支持编辑客户信息，生成前可维护客户公司名称、公司地址和联系电话", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/views/SalesOrderView.vue"},
		{table: "req_unit", code: "UT-117-01", title: "单测覆盖客户公司字段 API 映射、销售单客户公司兜底和 Vue 抽屉源码守卫", status: "done", assignee: "Codex", evidence: "TestCustomerAPIStoresCompanyContactFields; TestSalesOrderVueExposesCustomerInfoDrawer"},
		{table: "req_api", code: "API-117-01", title: "API 测试覆盖 POST/PUT 客户公司信息保存，以及未设置销售单公司名时生成销售单使用客户名兜底", status: "done", assignee: "Codex", evidence: "TestCustomerAPIStoresCompanyContactFields; TestSalesOrderDocumentAPIUsesCustomerCompanyFallback"},
		{table: "req_review", code: "REV-117-01", prCode: "PR-117", title: "验收：客户录入可维护公司信息；销售单页面右侧抽屉可编辑客户信息；公司名为空时 PDF 使用客户名", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-118", title: "登录后首屏必须直接显示默认功能菜单，不需要刷新页面", status: "review", assignee: "VA", evidence: "codex/login-first-load-menu-20260501"},
		{table: "req_dev", code: "DEV-118-01", title: "登录成功跳转 Vue 工作台时携带 fresh_login 标记，Vue 首次加载后按默认菜单展开并清除标记", status: "done", assignee: "Codex", evidence: "login.html fresh_login; App.vue defaultExpandedGroups"},
		{table: "req_unit", code: "UT-118-01", title: "单测覆盖登录页 fresh_login 跳转和 Vue 工作台首登默认展开菜单源码守卫", status: "done", assignee: "Codex", evidence: "TestLoginPageSupportsUsernamePasswordAndDoesNotRequirePhoneForPassword; TestVueShellUsesDefaultMenuExpansionAfterFreshLogin"},
		{table: "req_api", code: "API-118-01", title: "线上 smoke 覆盖 13800138075 登录后首屏菜单直接可见，auth/me 仍返回 admin 全权限", status: "done", assignee: "Codex", evidence: "postdeploy browser/API smoke"},
		{table: "req_review", code: "REV-118-01", prCode: "PR-118", title: "验收：13800138075 登录后不刷新也能看到功能菜单，刷新前后菜单权限一致", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-119", title: "销售单生成前必须先显示销售单预览，确认后再生成；修复历史版本下载 invalid document id", status: "review", assignee: "VA", evidence: "codex/sales-order-preview-download-20260501"},
		{table: "req_dev", code: "DEV-119-01", title: "新增销售单预览 API，复用生成快照逻辑返回下一版本号和预览内容，不创建销售单版本", status: "done", assignee: "Codex", evidence: "GET /api/orders/:id/sales-order-preview; PreviewSalesOrderDocument"},
		{table: "req_dev", code: "DEV-119-02", title: "修复历史版本下载路由的 PDF document id 解析，兼容 .pdf 后缀和路由参数为空的回退解析", status: "done", assignee: "Codex", evidence: "parseSalesOrderDocumentID; /orders/:id/sales-orders/:doc_id.pdf"},
		{table: "req_dev", code: "DEV-119-03", title: "销售单 Vue 页面展示销售单预览，确认生成按钮必须在预览加载后才能提交生成", status: "done", assignee: "Codex", evidence: "SalesOrderView preview panel and confirm button"},
		{table: "req_unit", code: "UT-119-01", title: "单测覆盖销售单预览服务、PDF document id 解析和 Vue 预览确认源码守卫", status: "done", assignee: "Codex", evidence: "TestServiceOwnsSalesOrderDocumentUseCases; TestParseSalesOrderDocumentIDAcceptsPDFPathFallback; TestSalesOrderVueRequiresPreviewBeforeGenerate"},
		{table: "req_api", code: "API-119-01", title: "API 测试覆盖预览接口不创建版本、确认生成仍从 V1 开始、历史版本 PDF 下载返回 application/pdf", status: "done", assignee: "Codex", evidence: "TestSalesOrderPreviewAPIDoesNotCreateDocumentVersion; TestSalesOrderDocumentAPI"},
		{table: "req_review", code: "REV-119-01", prCode: "PR-119", title: "验收：SO-20260428-0001 历史销售单可下载；页面先展示预览，点击确认生成后才创建新版本", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-120", title: "生产质检、仓库追溯、生产验收入口和录单新增客户抽屉修正：质检待定/不通过可保存，原料/LEGACY-MAT 批次可查询，验收打开对应仓库，录单可快速新增客户", status: "review", assignee: "VA", evidence: "codex/production-qa-fixes-customer-drawer-20260501"},
		{table: "req_dev", code: "DEV-120-01", title: "生产质检服务接受中文范围和结果并归一化为 work_order/raw_material/finished_batch 与 pass/hold/reject", status: "done", assignee: "Codex", evidence: "normalizeQualityInspectionScope; normalizeQualityInspectionResult"},
		{table: "req_dev", code: "DEV-120-02", title: "仓库库存批次追溯同时支持 FP 成品批次和 MB/LEGACY-MAT 原料批次，LEGACY-MAT 明确展示为系统升级期初批次和当前仓库位置", status: "done", assignee: "Codex", evidence: "GetStockTrace material_batch fallback; WarehouseInventoryView material trace"},
		{table: "req_dev", code: "DEV-120-03", title: "生产验收检查项返回 view_params，点击仓库相关入口时进入对应仓库和库存类型，而不是全部仓库", status: "done", assignee: "Codex", evidence: "AcceptanceSmokeRow.view_params; App.vue currentViewParams; WarehouseInventoryView viewParams"},
		{table: "req_dev", code: "DEV-120-04", title: "录单页支持抽屉式新增客户，粘贴收件信息自动识别姓名/电话/地址，并默认客户来源微信、订单类型批发", status: "done", assignee: "Codex", evidence: "OrderEntryView customerDrawerOpen; customer-recipient.js"},
		{table: "req_unit", code: "UT-120-01", title: "单测覆盖质检中文状态归一化、原料批次追溯结构、Vue 入口参数 wiring、录单客户抽屉和收件信息解析", status: "done", assignee: "Codex", evidence: "TestServiceOwnsManufacturingGapUseCases; TestVueStockWorkspaceIncludesFinishedTransferAndTraceLookup; customer-recipient.test.js; dev_120 source guards"},
		{table: "req_api", code: "API-120-01", title: "API 测试覆盖生产质检创建、生产验收 view_params 返回、库存追溯接口原料批次 fallback 和客户新增接口复用", status: "done", assignee: "Codex", evidence: "TestManufacturingGapAPIs; TestStockAPIRoutes; TestCustomerAPIStoresCompanyContactFields"},
		{table: "req_review", code: "REV-120-01", prCode: "PR-120", title: "验收：WO-0000000020 质检可保存待定/不通过；仓库批次可解释 LEGACY-MAT；生产验收打开对应仓库；录单可抽屉新增客户并自动识别地址", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-121", title: "用户权限页密码操作文案必须区分首次设置和已有密码重置", status: "review", assignee: "VA", evidence: "codex/password-action-label-20260501"},
		{table: "req_dev", code: "DEV-121-01", title: "用户权限页根据 auth_accounts.has_password 显示设置密码或重置密码，并同步输入框占位文案", status: "done", assignee: "Codex", evidence: "UserPermissionsView passwordActionLabel/passwordPlaceholder"},
		{table: "req_unit", code: "UT-121-01", title: "单测覆盖用户权限页必须包含设置密码/重置密码的状态化文案", status: "done", assignee: "Codex", evidence: "TestUserPermissionsDistinguishesSetAndResetPasswordLabels"},
		{table: "req_api", code: "API-121-01", title: "API/线上 smoke 覆盖账号列表 has_password 状态可驱动页面显示设置密码或重置密码", status: "done", assignee: "Codex", evidence: "GET /api/auth/accounts; postdeploy browser smoke"},
		{table: "req_review", code: "REV-121-01", prCode: "PR-121", title: "验收：无密码员工看到设置密码，已有密码员工看到重置密码，管理员不会把首次创建密码理解为重置", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-122", title: "销售单预览与PDF一致：预览展示收款码和公章，说明/收款方式保留换行，公章可拖动盖在公司名称上，并新增全局公司设置", status: "review", assignee: "VA", evidence: "codex/sales-order-layout-settings-20260501"},
		{table: "req_dev", code: "DEV-122-01", title: "新增全局公司设置表、API 和 Vue 设置页，销售单公司名称从全局公司设置读取，不再在销售单设置中维护", status: "done", assignee: "Codex", evidence: "company_profile; GET/POST /api/company/profile; CompanyProfileView"},
		{table: "req_dev", code: "DEV-122-02", title: "销售单快照补充资产 URL 和公章位置，PDF 渲染收款方式/说明时保留换行，公章按坐标覆盖在公司名称上", status: "done", assignee: "Codex", evidence: "SalesOrderAssetRef URL/XMM/YMM/WidthMM; sales_order_pdf"},
		{table: "req_dev", code: "DEV-122-03", title: "销售单预览页按 PDF 内容展示收款码、公章和多行文本；销售单设置页支持拖动公章位置", status: "done", assignee: "Codex", evidence: "SalesOrderView payment-code-preview/seal-stamp-preview; SalesOrderSettingsView seal-position-stage"},
		{table: "req_unit", code: "UT-122-01", title: "单测覆盖公司设置服务、销售单设置公章坐标、PDF 多行文本和公章位置、Vue 源码守卫及菜单权限", status: "done", assignee: "Codex", evidence: "TestServiceValidatesAndNormalizesCompanyProfile; TestSalesOrderPDFMultilineTextAndSealPositionHelpers; TestSalesOrderLayoutCompanySettingsRequirementSeeds"},
		{table: "req_api", code: "API-122-01", title: "API 测试覆盖全局公司设置保存/读取、销售单设置保存多行内容和公章坐标", status: "done", assignee: "Codex", evidence: "TestCompanyProfileAPI; TestSalesOrderSettingsAPI"},
		{table: "req_review", code: "REV-122-01", prCode: "PR-122", title: "验收：销售单预览与下载 PDF 的收款码、公章、换行和公司名称一致；公章位置可拖动调整", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-123", title: "质检拦截生产/库存：待处理或不通过批次进入冻结状态，原料/WIP/成品统一显示 quality_status", status: "review", assignee: "VA", evidence: "codex/quality-gate-p2-20260501"},
		{table: "req_dev", code: "DEV-123-01", title: "质检保存后按 work_order、finished_batch、raw_material 同步更新 stock_batches/material_batches.quality_status", status: "done", assignee: "Codex", evidence: "CreateQualityInspection applyQualityInspectionStatusTx"},
		{table: "req_dev", code: "DEV-123-02", title: "原料批次、WIP 仓库存、库存批次和成品批次追溯统一返回并展示质检状态", status: "done", assignee: "Codex", evidence: "stock read models quality_status; WarehouseInventoryView/MaterialBatchesView/StockBatchesView"},
		{table: "req_dev", code: "DEV-123-03", title: "冻结批次不得被领料到 WIP、生产扣料或成品出库/转仓误用", status: "done", assignee: "Codex", evidence: "TransferMaterial; materialBatchAllocationsTx; TransferFinishedProduct quality block"},
		{table: "req_unit", code: "UT-123-01", title: "单测覆盖质检同步批次状态、冻结原料/WIP/成品拦截、Vue 质检状态显示和需求种子", status: "done", assignee: "Codex", evidence: "quality_gate_test; repository_test; material_consumption_test; stock_vue_source_test; dev_122_quality_gate_test"},
		{table: "req_api", code: "API-123-01", title: "API 测试覆盖仓库库存和批次追溯返回 quality_status，冻结批次不会作为可用库存误用", status: "done", assignee: "Codex", evidence: "TestStockAPIRoutes; go test focused packages"},
		{table: "req_review", code: "REV-123-01", prCode: "PR-123", title: "验收：WO-0000000020 质检不通过后成品批次显示不通过；原料/WIP/成品状态一致；冻结批次不能出库或生产扣料", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-124", title: "销售单预览公章拖动可保存，公章支持去除背景，收款码并排展示", status: "review", assignee: "VA", evidence: "codex/sales-order-preview-seal-tools-20260501"},
		{table: "req_dev", code: "DEV-124-01", title: "销售单预览公章拖动后通过独立接口保存坐标，避免覆盖销售单说明和收款方式", status: "done", assignee: "Codex", evidence: "SalesOrderView previewSealStage; POST /api/settings/sales-order/seal-position"},
		{table: "req_dev", code: "DEV-124-02", title: "销售单设置页新增公章去除背景操作，后端生成透明 PNG 并设为当前公章", status: "done", assignee: "Codex", evidence: "SalesOrderSettingsView removeSealBackground; removeSealImageBackground"},
		{table: "req_dev", code: "DEV-124-03", title: "销售单预览和 PDF 中收款码改为横向并排展示，二维码和说明保持一致", status: "done", assignee: "Codex", evidence: "payment-code-preview-list grid; SalesOrderRenderer.renderPaymentCodes"},
		{table: "req_unit", code: "UT-124-01", title: "单测覆盖需求表种子、预览拖动源码守卫、公章去背景源码守卫和 PDF 并排布局源码守卫", status: "done", assignee: "Codex", evidence: "dev_124_sales_order_preview_seal_tools_test.go"},
		{table: "req_api", code: "API-124-01", title: "API 测试覆盖公章坐标独立保存不覆盖文本，以及公章去背景生成透明 PNG 并更新当前公章", status: "done", assignee: "Codex", evidence: "TestSalesOrderSealPositionAPIOnlyUpdatesCoordinates; TestSalesOrderSealBackgroundRemovalCreatesTransparentPNG"},
		{table: "req_review", code: "REV-124-01", prCode: "PR-124", title: "验收：销售单预览可拖动公章并保存；设置页可去除公章背景；收款码在预览和 PDF 并排展示", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-125", title: "订单列表销售单抽屉内预览销售单，并可打开销售单设置抽屉；公章大小可调整", status: "review", assignee: "VA", evidence: "codex/sales-order-drawer-settings-20260502"},
		{table: "req_dev", code: "DEV-125-01", title: "订单列表点击销售单时在当前页面打开销售单抽屉，不再跳转到独立销售单页面", status: "done", assignee: "Codex", evidence: "OrdersView salesOrderDrawerOpen; SalesOrderView embedded"},
		{table: "req_dev", code: "DEV-125-02", title: "销售单抽屉内提供销售单设置按钮，并在右侧销售单设置抽屉中维护说明、收款码和公章", status: "done", assignee: "Codex", evidence: "SalesOrderView settingsDrawerOpen; SalesOrderSettingsView embedded"},
		{table: "req_dev", code: "DEV-125-03", title: "销售单设置公章区域明确支持公章大小调整，并沿用 seal_width_mm 影响预览和 PDF", status: "done", assignee: "Codex", evidence: "SalesOrderSettingsView seal-size-control"},
		{table: "req_unit", code: "UT-125-01", title: "单测覆盖订单列表销售单抽屉、销售单设置抽屉、公章大小控件和需求表种子", status: "done", assignee: "Codex", evidence: "dev_125_sales_order_drawer_settings_test.go"},
		{table: "req_api", code: "API-125-01", title: "API/构建验证覆盖销售单预览、设置读取和前端 Vue/Vite 打包通过", status: "done", assignee: "Codex", evidence: "go test ./...; npm run build; deployment smoke"},
		{table: "req_review", code: "REV-125-01", prCode: "PR-125", title: "验收：订单列表点击销售单弹出销售单抽屉；抽屉内可打开销售单设置；公章可调位置和大小", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-126", title: "销售单导出 PDF 与预览版式一致，PDF 公章等比渲染，并新增公账收款设置", status: "review", assignee: "VA", evidence: "codex/sales-order-pdf-layout-account-20260502"},
		{table: "req_dev", code: "DEV-126-01", title: "PDF 公章等比渲染，按公章设置区域居中放入，避免透明背景或圆章被横向拉伸", status: "done", assignee: "Codex", evidence: "fitSalesOrderImageInBox; renderSealStamp"},
		{table: "req_dev", code: "DEV-126-02", title: "预览和导出 PDF 版式一致，商品表、合计、收款说明和二维码使用同一信息结构", status: "done", assignee: "Codex", evidence: "renderSalesOrderHeader/items/totals/paymentInfo; SalesOrderView payment-info-grid"},
		{table: "req_dev", code: "DEV-126-03", title: "销售单设置新增公账收款设置，保存户名、开户行和账号并写入销售单快照", status: "done", assignee: "Codex", evidence: "bank_account_name/bank_name/bank_account_no"},
		{table: "req_unit", code: "UT-126-01", title: "单测覆盖需求种子、PDF 公章等比 helper、PDF/预览布局源码守卫和公账字段持久化", status: "done", assignee: "Codex", evidence: "dev_126_sales_order_pdf_layout_account_test.go; sales_order_pdf_test.go; sales_order_repository_test.go"},
		{table: "req_api", code: "API-126-01", title: "API 测试覆盖销售单设置保存和返回公账收款字段，部署后 smoke 验证预览与下载接口", status: "done", assignee: "Codex", evidence: "TestSalesOrderSettingsAPI; postdeploy smoke"},
		{table: "req_review", code: "REV-126-01", prCode: "PR-126", title: "验收：导出 PDF 公章不拉伸；预览与下载 PDF 版式一致；收款说明、二维码、公账信息排版紧凑", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-127", title: "销售单公账收款信息移入公司设置，补充纳税人识别号和公司地址；销售单长地址不截断，收款码自适应填充空白", status: "review", assignee: "VA", evidence: "codex/company-account-sales-order-layout-20260502"},
		{table: "req_dev", code: "DEV-127-01", title: "公司设置维护公账收款信息、纳税人识别号和公司地址，并支持一键复制公账收款信息", status: "done", assignee: "Codex", evidence: "CompanyProfileView copyAccountInfo; /api/company/profile"},
		{table: "req_dev", code: "DEV-127-02", title: "销售单快照从公司设置读取公账户名、开户行、账号、纳税人识别号和公司地址，销售单设置不再维护公账字段", status: "done", assignee: "Codex", evidence: "buildSalesOrderSnapshotTx; SalesOrderSettingsView"},
		{table: "req_dev", code: "DEV-127-03", title: "PDF 和预览修复客户地址不截断，并按收款码数量自适应填充：单码更大，多码竖向排列", status: "done", assignee: "Codex", evidence: "writeSalesOrderMetaRow; salesOrderPaymentCodeMetrics; single-payment-code/payment-code-stack"},
		{table: "req_unit", code: "UT-127-01", title: "单测覆盖公司设置公账字段归一化、PDF 收款码尺寸 helper、Vue/需求种子源码守卫和销售单快照公账来源", status: "done", assignee: "Codex", evidence: "TestServiceValidatesAndNormalizesCompanyProfile; TestSalesOrderPDFPaymentCodeSizingAdaptsToCount; dev_127_company_account_sales_order_layout_test.go"},
		{table: "req_api", code: "API-127-01", title: "API 测试覆盖公司设置保存/读取公账字段，以及销售单预览返回公司公账字段", status: "done", assignee: "Codex", evidence: "TestCompanyProfileAPI; TestSalesOrderPreviewAPIUsesGlobalCompanyProfile"},
		{table: "req_review", code: "REV-127-01", prCode: "PR-127", title: "验收：公司设置可维护并复制公账信息；销售单客户地址完整换行；收款码尺寸和排列更充分利用空白", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-128", title: "原料入库物料模糊搜索；生产中投料和成品数合并编辑，实时显示实际出品率，并明确部分完工说明", status: "review", assignee: "VA", evidence: "codex/production-running-receipts-yield-20260502"},
		{table: "req_dev", code: "DEV-128-01", title: "原料入库物料选择增加名称/编号模糊搜索，且继续排除包装物料", status: "done", assignee: "Codex", evidence: "MaterialReceiptsView; material-receipts.js"},
		{table: "req_dev", code: "DEV-128-02", title: "生产中页面把投料、成品件数/余料和实际出品率合并到同一条生产数据中编辑展示", status: "done", assignee: "Codex", evidence: "ProduceRunningView; produce-running.js"},
		{table: "req_dev", code: "DEV-128-03", title: "完成生产接口在全量完工时也采用页面编辑后的实际投料数，部分完工说明保留剩余继续生产", status: "done", assignee: "Codex", evidence: "resolveFinishConsumedInput; POST /api/produce/running/finish"},
		{table: "req_unit", code: "UT-128-01", title: "单测覆盖原料入库模糊搜索、生产中实时出品率 helper、完成生产实际投料规则和需求种子", status: "done", assignee: "Codex", evidence: "material-receipts.test.js; produce-running.test.js; running_repository_test.go; dev_128_production_running_receipts_test.go"},
		{table: "req_api", code: "API-128-01", title: "API 测试覆盖生产中完成接口提交 consumed_input_g 后写入生产日志并按实际投料扣料", status: "done", assignee: "Codex", evidence: "TestProduceFinishAPIUsesEditedInputForFullCompletion"},
		{table: "req_review", code: "REV-128-01", prCode: "PR-128", title: "验收：原料入库可搜索物料；生产中一行内可改投料和成品数，实际出品率实时变化；部分完工含明确含义", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-129", title: "库存作业下拉框内搜索，统一四个库存作业页面排版，并在生产中WIP不足时打开库存作业抽屉", status: "review", assignee: "VA", evidence: "codex/stock-operation-combobox-layout-20260502"},
		{table: "req_dev", code: "DEV-129-01", title: "原料入库、WIP领退/转仓、成品转仓的物料/成品选择改为下拉框内搜索，不再额外放独立搜索框", status: "done", assignee: "Codex", evidence: "SearchableSelect; MaterialReceiptsView; WipMaterialsView; FinishedTransfersView"},
		{table: "req_dev", code: "DEV-129-02", title: "四个库存作业页面统一为类似仓库库存的面板、网格和表格排版，库存作业输入框高度和列宽对齐", status: "done", assignee: "Codex", evidence: "stock-operation-page; operation-grid"},
		{table: "req_dev", code: "DEV-129-03", title: "生产中完成生产遇到 WIP 库存不足时自动打开右侧库存作业抽屉，默认进入 WIP领退/转仓", status: "done", assignee: "Codex", evidence: "ProduceRunningView stockDrawerOpen; StockOperationsView initialTab"},
		{table: "req_unit", code: "UT-129-01", title: "单测覆盖可搜索下拉过滤、多库存作业页面源码守卫、WIP不足抽屉和需求种子", status: "done", assignee: "Codex", evidence: "searchable-select.test.js; dev_129_stock_operation_combobox_test.go"},
		{table: "req_api", code: "API-129-01", title: "API级回归覆盖库存作业接口和生产完成接口仍可被 Vue/Vite 页面调用，WIP不足错误保持可识别", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/stock ./internal/interfaces/http/production ./internal/interfaces/http/support"},
		{table: "req_review", code: "REV-129-01", prCode: "PR-129", title: "验收：库存作业下拉框可直接输入搜索，三类作业输入对齐；生产中 WIP 不足时右侧抽屉打开库存作业页", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-130", title: "同一商品不同规格的待生产订单合并成一个烘焙工单时，工单必须保留全部订单号并在完工时分别入库各规格", status: "review", assignee: "VA", evidence: "codex/production-merged-order-links-20260502"},
		{table: "req_dev", code: "DEV-130-01", title: "开始生产按 product_id 合并同商品缺口，produce_running_items 保存全部 order_nos，produce_running_outputs 保存各规格需求和计划成品数", status: "done", assignee: "Codex", evidence: "groupStartNeedsForRuns; produce_running_outputs"},
		{table: "req_dev", code: "DEV-130-02", title: "生产中列表和完成生产 API 支持多规格 outputs，完工时按规格分别增加成品库存、批次流水和生产日志", status: "done", assignee: "Codex", evidence: "RunningItem.Outputs; FinishCommand.Outputs; finishRunningOutputs"},
		{table: "req_dev", code: "DEV-130-03", title: "生产中 Vue 页面展示多规格输出行，按规格填写完成件数和散装余量，并按合计成品克重计算实际出品率", status: "done", assignee: "Codex", evidence: "ProduceRunningView multi-output controls; produce-running.js"},
		{table: "req_unit", code: "UT-130-01", title: "单测覆盖同商品多规格分组保留全部订单号、前端多规格完工 payload 和需求种子源码守卫", status: "done", assignee: "Codex", evidence: "running_merge_test.go; produce-running.test.js; dev_130_production_merge_orders_test.go"},
		{table: "req_api", code: "API-130-01", title: "API 测试覆盖开始生产合并 454g/227g 同商品工单，以及多规格完工后两个订单都进入生产完成", status: "done", assignee: "Codex", evidence: "TestProduceStartAPIMergesSameProductSpecsAndKeepsAllOrderNos; TestProduceFinishAPIMultiSpecRunCompletesAllLinkedOrders"},
		{table: "req_review", code: "REV-130-01", prCode: "PR-130", title: "验收：SO-20260427-0002 和 SO-20260501-0001 同属乌拉嘎时只建一个生产工单，但工单显示两个订单号，完工后 454g/227g 分别入库且两个订单均生产完成", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-131", title: "生产质检对象选择抽屉：按当前质检类型选择工单、原料批次或产品批次", status: "review", assignee: "VA", evidence: "codex/production-quality-drawer-20260502; codex/quality-target-specific-drawer-20260502"},
		{table: "req_dev", code: "DEV-131-01", title: "生产质检页改为工作台布局，左侧切换工单质检、原料质检、产品质检，表单保留结果、指标和备注", status: "done", assignee: "Codex", evidence: "QualityInspectionsView workspace/type-panel"},
		{table: "req_dev", code: "DEV-131-02", title: "生产质检页右侧抽屉复用工单、原料批次和库存批次 API；按钮按当前类型显示选择工单/原料批次/产品批次，抽屉内只展示当前类型对象并回填单据号/批次号和名称", status: "done", assignee: "Codex", evidence: "quality-inspections.js; QualityInspectionsView targetActionLabel"},
		{table: "req_unit", code: "UT-131-01", title: "单测覆盖质检对象 helper、动态选择按钮、单类型抽屉源码守卫、需求种子和生产手册更新", status: "done", assignee: "Codex", evidence: "quality-inspections.test.js; dev_131_quality_drawer_test.go"},
		{table: "req_api", code: "API-131-01", title: "API 验证复用 GET /api/produce/work-orders、GET /api/stock/material-batches、GET /api/stock/batches 作为质检对象来源，保存仍走 POST /api/produce/quality-inspections", status: "done", assignee: "Codex", evidence: "TestManufacturingGapAPIs; TestStockAPIRoutes; QualityInspectionsView API wiring"},
		{table: "req_review", code: "REV-131-01", prCode: "PR-131", title: "验收：生产质检页按当前类型显示选择工单/选择原料批次/选择产品批次，右侧抽屉只出现对应候选，选择后可保存质检记录", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-132", title: "销售单支持生成图片，和 PDF 一样按订单快照保留 PNG 图片版本", status: "review", assignee: "VA", evidence: "codex/sales-order-image-export-20260502"},
		{table: "req_dev", code: "DEV-132-01", title: "新增销售单 PNG 图片版本表和应用服务，用同一销售单快照生成图片资产，图片最新版独立于 PDF 最新版", status: "done", assignee: "Codex", evidence: "sales_order_images; GenerateSalesOrderImage; RenderPNG"},
		{table: "req_dev", code: "DEV-132-02", title: "新增销售单图片生成、列表、历史下载和最新版下载 API，下载返回 image/png", status: "done", assignee: "Codex", evidence: "POST/GET /api/orders/:id/sales-order-images; /orders/:id/sales-order-image-latest.png"},
		{table: "req_dev", code: "DEV-132-03", title: "销售单 Vue 抽屉增加确认生成图片、下载最新版图片和图片版本列表，并更新销售单手册", status: "done", assignee: "Codex", evidence: "SalesOrderView imageDocuments; sales-order-user-manual"},
		{table: "req_unit", code: "UT-132-01", title: "单测覆盖销售单图片应用服务、PNG 渲染、仓储版本、前端下载 helper 和需求种子源码守卫", status: "done", assignee: "Codex", evidence: "TestServiceOwnsSalesOrderImageUseCases; TestRenderSalesOrderPNG; TestGenerateSalesOrderImageCreatesIndependentImageVersions; sales-order.test.js; dev_132"},
		{table: "req_api", code: "API-132-01", title: "API 测试覆盖确认生成销售单图片、图片版本列表、历史 PNG 下载和最新版 PNG 下载", status: "done", assignee: "Codex", evidence: "TestSalesOrderDocumentAPI image branch"},
		{table: "req_review", code: "REV-132-01", prCode: "PR-132", title: "验收：销售单抽屉可基于预览生成图片；图片可下载最新版和历史版本；PDF 最新版不受图片生成影响", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-133", title: "订单发货后维护出库单：已发货订单可维护出库信息、预览并下载版本化出库单 PDF", status: "review", assignee: "VA", evidence: "codex/delivery-note-outbound-20260502"},
		{table: "req_dev", code: "DEV-133-01", title: "新增出库单领域快照、出库维护表、版本化出库单文档表和 PDF 渲染", status: "done", assignee: "Codex", evidence: "DeliveryNoteSnapshot; delivery_note_forms/documents; DeliveryNoteRenderer"},
		{table: "req_dev", code: "DEV-133-02", title: "新增出库单 API：维护出库信息、出库单预览、确认生成、历史版本和最新版下载", status: "done", assignee: "Codex", evidence: "GET/POST /api/orders/:id/delivery-note*; /orders/:id/delivery-note-latest.pdf"},
		{table: "req_dev", code: "DEV-133-03", title: "订单列表新增出库单抽屉，Vue 页面维护出库日期、出库仓库、发货方式、快递单号和备注", status: "done", assignee: "Codex", evidence: "OrdersView DeliveryNoteView drawer; DeliveryNoteView.vue"},
		{table: "req_unit", code: "UT-133-01", title: "单测覆盖出库单领域规则、服务用例、Vue 源码守卫、前端 URL helper 和需求种子", status: "done", assignee: "Codex", evidence: "delivery_note_test.go; TestServiceOwnsDeliveryNoteUseCases; dev_133_delivery_note_outbound_test.go; delivery-note.test.js"},
		{table: "req_api", code: "API-133-01", title: "API 测试覆盖已发货订单出库单保存/预览/生成/下载，以及未发货订单禁止生成出库单", status: "done", assignee: "Codex", evidence: "TestDeliveryNoteDocumentAPI; TestDeliveryNoteRequiresShippedOrder"},
		{table: "req_review", code: "REV-133-01", prCode: "PR-133", title: "验收：已发货订单可打开出库单抽屉维护信息，先预览后生成 PDF，最新版和历史版本均可下载", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-134", title: "销售单图片字体和排版必须清晰，不得出现文字叠行、溢出或整页排版混乱", status: "review", assignee: "VA", evidence: "codex/sales-order-image-layout-fix-20260502"},
		{table: "req_dev", code: "DEV-134-01", title: "修正销售单 PNG 渲染字体 DPI 和行高关系，确保 PNG 字体高度不得超过行高，长文本按块正常换行", status: "done", assignee: "Codex", evidence: "salesOrderPNGDPI=72; TestSalesOrderPNGTextMetricsFitConfiguredLineHeights"},
		{table: "req_unit", code: "UT-134-01", title: "单测覆盖销售单 PNG 字体高度与行高匹配，并用源码守卫防止 DPI 回退导致叠行", status: "done", assignee: "Codex", evidence: "TestSalesOrderPNGTextMetricsFitConfiguredLineHeights; dev_134_sales_order_image_layout_test.go"},
		{table: "req_api", code: "API-134-01", title: "API 回归沿用销售单图片生成/下载接口测试，确保修复后图片仍可通过 image/png 接口生成和下载", status: "done", assignee: "Codex", evidence: "TestSalesOrderDocumentAPI image branch; go test ./internal/interfaces/http/sales"},
		{table: "req_review", code: "REV-134-01", prCode: "PR-134", title: "验收：重新生成销售单图片后文字不重叠，订单信息、商品表、收款说明和二维码排版清晰", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-135", title: "订单发票申请：订单列表可申请发票，开票后上传发票文件，支持PDF和图片", status: "review", assignee: "VA", evidence: "codex/order-invoice-request-upload-20260502"},
		{table: "req_dev", code: "DEV-135-01", title: "新增订单发票申请表和应用服务，订单列表读模型返回发票申请状态和发票文件入口", status: "done", assignee: "Codex", evidence: "order_invoices; OrderInvoice; ListOrders invoice fields"},
		{table: "req_dev", code: "DEV-135-02", title: "新增发票申请和发票文件上传 API，上传时只允许真实 PDF和图片 文件并复用销售单资产存储", status: "done", assignee: "Codex", evidence: "GET/POST /api/orders/:id/invoice*; classifyOrderInvoiceFile"},
		{table: "req_dev", code: "DEV-135-03", title: "订单列表新增发票抽屉，支持申请发票、查看状态、上传 PDF 或图片并打开已上传文件", status: "done", assignee: "Codex", evidence: "OrdersView invoice drawer; OrderInvoiceView.vue"},
		{table: "req_unit", code: "UT-135-01", title: "单测覆盖发票应用服务、文件类型 helper、前端发票状态 helper 和需求种子源码守卫", status: "done", assignee: "Codex", evidence: "TestServiceOwnsOrderInvoiceUseCases; TestOrderInvoiceFileContentTypeAllowsPDFAndImagesOnly; order-invoice.test.js; dev_135_order_invoice_test.go"},
		{table: "req_api", code: "API-135-01", title: "API 测试覆盖订单发票申请、PDF/图片 发票上传落库和文本文件拒绝", status: "done", assignee: "Codex", evidence: "TestOrderInvoiceAPIRequestsAndUploadsPDFAndImage; TestOrderInvoiceAPIRejectsTextUpload"},
		{table: "req_review", code: "REV-135-01", prCode: "PR-135", title: "验收：订单可申请发票；开票后可上传 PDF 或图片发票；非 PDF/图片文件被拒绝；订单列表显示发票状态和文件链接", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-136", title: "销售单公章拖动必须稳定，点击公章不跳走；修改公章位置后新生成图片使用最新公章位置", status: "review", assignee: "VA", evidence: "codex/sales-order-seal-drag-image-20260502"},
		{table: "req_dev", code: "DEV-136-01", title: "销售单设置页公章只在按住公章本身时拖动，按拖动位移计算坐标，避免把点击点当成左上角导致跳走", status: "done", assignee: "Codex", evidence: "beginSalesOrderSealDrag; moveSalesOrderSealDrag; SalesOrderSettingsView"},
		{table: "req_dev", code: "DEV-136-02", title: "销售单设置页公章拖动松手自动保存坐标；销售单预览保存后提示需要重新生成图片或 PDF 后下载", status: "done", assignee: "Codex", evidence: "saveSealPosition; POST /api/settings/sales-order/seal-position"},
		{table: "req_unit", code: "UT-136-01", title: "单测覆盖公章拖动保留点击偏移、不跳走，源码守卫覆盖设置页松手自动保存和重新生成提示", status: "done", assignee: "Codex", evidence: "sales-order.test.js; dev_136_sales_order_seal_drag_image_test.go"},
		{table: "req_api", code: "API-136-01", title: "API 测试覆盖保存公章坐标后确认生成销售单图片，新图片快照写入最新 x/y/width", status: "done", assignee: "Codex", evidence: "TestSalesOrderImageGenerationUsesSavedSealPosition"},
		{table: "req_review", code: "REV-136-01", prCode: "PR-136", title: "验收：点击并拖动公章不会跳走；松手后刷新预览坐标不丢；重新生成并下载图片后公章位置跟随最新设置", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-137", title: "订单和出库单支持分享到微信；分享资源到外部必须使用统一代码和统一逻辑", status: "review", assignee: "VA", evidence: "codex/wechat-share-resources-20260502"},
		{table: "req_dev", code: "DEV-137-01", title: "新增外部分享资源统一逻辑、分享 token 表、免登录分享页和文件访问路由，支持销售单 PDF、销售单图片、出库单 PDF", status: "done", assignee: "Codex", evidence: "external_share_resources; /api/share-resources; /share/:token"},
		{table: "req_dev", code: "DEV-137-02", title: "销售单页面新增分享PDF到微信和分享图片到微信；出库单页面新增分享到微信，均调用同一个分享 API", status: "done", assignee: "Codex", evidence: "SalesOrderView; DeliveryNoteView; buildShareResourcePayload"},
		{table: "req_dev", code: "DEV-137-03", title: "分享页免登录但创建分享 token 必须鉴权并具备订单写权限，避免未授权创建外部资源", status: "done", assignee: "Codex", evidence: "isPublicUnauthenticatedPath(/share/); AuthorizationMiddleware /api/share-resources"},
		{table: "req_unit", code: "UT-137-01", title: "单测覆盖共享前端分享 helper、分享公开路径和权限映射、需求种子与源码守卫", status: "done", assignee: "Codex", evidence: "external-share.js tests; auth_middleware_test; dev_137_external_wechat_share_test.go"},
		{table: "req_api", code: "API-137-01", title: "API 测试覆盖同一 /api/share-resources 流程生成销售单 PDF、销售单图片和出库单 PDF 分享资源，公开文件可访问", status: "done", assignee: "Codex", evidence: "TestExternalShareResourceAPIUsesOneFlowForSalesOrderAndDeliveryNote"},
		{table: "req_review", code: "REV-137-01", prCode: "PR-137", title: "验收：销售单和出库单已生成版本后可分享到微信；同一套外部分享逻辑适配不同资源类型", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-138", title: "操作日志可读性修正：对象、菜单、功能和摘要按当前业务流程显示中文", status: "review", assignee: "VA", evidence: "codex/audit-log-readability-20260502"},
		{table: "req_dev", code: "DEV-138-01", title: "扫描审计日志写入对象，补齐订单、生产、库存、成本、销售单、公司和账号实体的菜单/功能映射", status: "done", assignee: "Codex", evidence: "auditMenuFeature entity mapping"},
		{table: "req_dev", code: "DEV-138-02", title: "操作请求路由按当前 Vue 菜单 IA 输出菜单和功能，避免旧菜单、未分类和访问系统页面兜底", status: "done", assignee: "Codex", evidence: "operationMenuFeature route mapping"},
		{table: "req_dev", code: "DEV-138-03", title: "日志对象和摘要优先显示批次号、工单号、订单号、参数名、版本号等业务可读标识，不直接暴露内部对象名", status: "done", assignee: "Codex", evidence: "labelEntityType/labelAction/labelField/auditTargetHint"},
		{table: "req_unit", code: "UT-138-01", title: "单测覆盖扫描到的审计对象、动作、字段、菜单、功能和摘要可读性", status: "done", assignee: "Codex", evidence: "TestDecorateAuditLogRowScannedEntitiesUseReadableLabels; TestDecorateAuditLogRowScannedOperationRoutesUseCurrentMenuIA"},
		{table: "req_api", code: "API-138-01", title: "API 级回归覆盖 /api/audit 输出仍走统一装饰层，返回当前菜单 IA 和中文对象摘要", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support -count=1"},
		{table: "req_review", code: "REV-138-01", prCode: "PR-138", title: "验收：打开操作日志后不再看到大批未分类/英文对象；菜单与系统左侧菜单一致；摘要能直接看懂业务对象", status: "todo", assignee: "VA", evidence: "待 Van 功能分支验收"},
		{table: "req_product", code: "PR-139", title: "订单和出库单分享到微信必须直接分享文件，不发送链接", status: "review", assignee: "VA", evidence: "codex/wechat-share-files-20260503"},
		{table: "req_dev", code: "DEV-139-01", title: "统一前端分享 helper 使用 /api/share-resources 返回的 file_url 下载 Blob，构造 File 后通过 Web Share API files 分享", status: "done", assignee: "Codex", evidence: "shareResourceToWechat"},
		{table: "req_dev", code: "DEV-139-02", title: "销售单和出库单分享按钮只提示直接分享文件；浏览器不支持文件分享时提示下载后手动发送，不再复制分享链接", status: "done", assignee: "Codex", evidence: "SalesOrderView; DeliveryNoteView; sales-order-user-manual; delivery-note-user-manual"},
		{table: "req_unit", code: "UT-139-01", title: "单测覆盖文件分享 payload、canShare 不支持分支、不复制链接和 Vue 文案源码守卫", status: "done", assignee: "Codex", evidence: "sales-order.test.js; dev_139_wechat_file_share_test.go"},
		{table: "req_api", code: "API-139-01", title: "API 回归沿用统一 /api/share-resources 流程，响应必须提供 file_url 并可返回对应 PDF 或 PNG 文件", status: "done", assignee: "Codex", evidence: "TestExternalShareResourceAPIUsesOneFlowForSalesOrderAndDeliveryNote"},
		{table: "req_review", code: "REV-139-01", prCode: "PR-139", title: "验收：点击销售单 PDF、销售单图片或出库单分享到微信后，微信收到的是 PDF/PNG 文件而不是链接", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-140", title: "出库单公章设置和订单抽屉优化：出库单复用销售单公章，订单列表快递信息合并与订单状态合并，单号在订单抽屉回填", status: "review", assignee: "VA", evidence: "codex/delivery-note-seal-order-drawer-20260503"},
		{table: "req_dev", code: "DEV-140-01", title: "出库单预览和 PDF 复用销售单公章资产、坐标和透明背景处理，并提供出库单页公章设置入口", status: "done", assignee: "Codex", evidence: "DeliveryNoteSnapshot.Seal; DeliveryNoteRenderer; DeliveryNoteView; CompanySealSettingsView"},
		{table: "req_dev", code: "DEV-140-02", title: "订单列表收窄为订单、快递信息和订单状态等关键列；点击订单打开抽屉展示寄件人、单号、收款、发货、生产和发票状态，并支持单订单回填单号", status: "done", assignee: "Codex", evidence: "OrdersView orderDetailDrawerOpen/status-stack; POST /api/orders/:id/shipping-tracking"},
		{table: "req_unit", code: "UT-140-01", title: "单测覆盖单订单单号回填服务校验、出库单公章 PDF 渲染、Vue 源码守卫和需求种子", status: "done", assignee: "Codex", evidence: "TestServiceNormalizesSingleOrderTracking; TestRenderDeliveryNotePDFEmbedsConfiguredSeal; dev_140_delivery_note_seal_order_drawer_test.go"},
		{table: "req_api", code: "API-140-01", title: "API 测试覆盖出库单预览返回公章、订单列表返回最近发货寄件人、订单抽屉单号回填后标记已发货", status: "done", assignee: "Codex", evidence: "TestDeliveryNotePreviewIncludesConfiguredSeal; TestOrdersListIncludesLatestShipmentSender; TestOrdersSingleShippingTrackingAPIMarksOrderShipped"},
		{table: "req_review", code: "REV-140-01", prCode: "PR-140", title: "验收：出库单页可设置并拖动公章，新生成出库单 PDF 带公章；订单列表不再堆叠过多列，点击订单抽屉可查看快递和四类状态，并直接回填单号", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-141", title: "销售单二维码和分享图片质量提升：收款码更大、更容易识别，分享 PNG 更高清，整体字体更粗更清晰", status: "review", assignee: "VA", evidence: "codex/sales-order-image-qr-quality-20260503"},
		{table: "req_dev", code: "DEV-141-01", title: "销售单 PNG 渲染改为 2 倍高清画布，所有文字、线条、图片按统一比例绘制，文本加粗绘制", status: "done", assignee: "Codex", evidence: "salesOrderPNGScale=2; salesOrderPNGTextWeightOffsetPixels"},
		{table: "req_dev", code: "DEV-141-02", title: "销售单 PDF 和 PNG 的收款二维码尺寸同步放大；PNG 二维码使用锐利缩放，避免分享图片中二维码识别困难", status: "done", assignee: "Codex", evidence: "salesOrderPaymentCodeMetrics; salesOrderPNGPaymentCodeMetrics; assetImageSharp"},
		{table: "req_unit", code: "UT-141-01", title: "单测覆盖销售单 PNG 高清画布、二维码像素尺寸、PDF 收款码尺寸阈值和需求种子源码守卫", status: "done", assignee: "Codex", evidence: "TestRenderSalesOrderPNGUsesHighResolutionCanvasAndLargePaymentCode; dev_141_sales_order_image_qr_quality_test.go"},
		{table: "req_api", code: "API-141-01", title: "API 测试覆盖生成销售单图片后，最新版 PNG 下载接口返回高清图片", status: "done", assignee: "Codex", evidence: "TestSalesOrderDocumentAPI latest sales order image bounds >= 2480x3508"},
		{table: "req_review", code: "REV-141-01", prCode: "PR-141", title: "验收：生成销售单图片分享到微信后图片清晰，字体更粗，收款二维码足够大且能被微信识别", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-142", title: "订单号编辑抽屉：订单号与编辑入口合并，点击订单号直接在抽屉编辑订单", status: "review", assignee: "VA", evidence: "codex/order-number-edit-drawer-20260503"},
		{table: "req_dev", code: "DEV-142-01", title: "订单列表移除独立编辑跳转，在订单号抽屉内嵌录单编辑表单，保存后刷新当前列表和抽屉数据", status: "done", assignee: "Codex", evidence: "OrdersView OrderEntryView embedded order-edit-panel"},
		{table: "req_unit", code: "UT-142-01", title: "源码守卫覆盖订单号编辑抽屉、移除独立编辑链接、录单页嵌入模式保存不跳转和需求种子", status: "done", assignee: "Codex", evidence: "dev_142_order_number_edit_drawer_test.go"},
		{table: "req_api", code: "API-142-01", title: "API 回归沿用现有 GET /api/order/form 和 POST /api/order 编辑保存接口，列表刷新仍走 GET /api/orders", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/sales -count=1"},
		{table: "req_review", code: "REV-142-01", prCode: "PR-142", title: "验收：订单列表点击订单号打开抽屉即可编辑订单；列表和抽屉中不再出现独立编辑跳转入口；保存后停留在当前列表", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-143", title: "订单抽屉寄件人直接修改：去掉加入本次快递录单选项，寄件人默认跟随全局寄件人并可直接改", status: "review", assignee: "VA", evidence: "codex/order-drawer-direct-sender-20260503"},
		{table: "req_dev", code: "DEV-143-01", title: "订单抽屉快递信息移除加入本次快递录单复选框，寄件人下拉始终可用，默认跟随顶部本次寄件人", status: "done", assignee: "Codex", evidence: "OrdersView globalSenderLabel; orderSenderIDs direct select"},
		{table: "req_unit", code: "UT-143-01", title: "源码守卫覆盖订单抽屉寄件人直接修改、默认跟随全局寄件人、移除加入本次快递录单选项和需求种子", status: "done", assignee: "Codex", evidence: "dev_143_order_drawer_sender_direct_test.go"},
		{table: "req_api", code: "API-143-01", title: "API 回归沿用现有快递录单 Excel 接口，单订单寄件人覆盖仍随 order_senders 提交，不新增后端接口", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/sales -count=1"},
		{table: "req_review", code: "REV-143-01", prCode: "PR-143", title: "验收：打开订单抽屉后没有加入本次快递录单选项；寄件人默认显示本次全局寄件人，并可直接选择其他寄件人", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-144", title: "生产计划删掉物料需求计划；库存充足订单按无需生产处理并可直接发货；生产流程页作为新员工一页式入口", status: "review", assignee: "VA", evidence: "codex/production-direct-ship-flow-20260503"},
		{table: "req_dev", code: "DEV-144-01", title: "生产计划 Vue 页面移除物料需求计划按钮和区块，保留缺口计划、物料需求汇总和烘焙建议", status: "done", assignee: "Codex", evidence: "ProducePlanView direct-ship-tip; no loadMaterialPlan"},
		{table: "req_dev", code: "DEV-144-02", title: "新增无需生产生产状态，并让订单列表、只看可发货和快递录单把无需生产视为可发货", status: "done", assignee: "Codex", evidence: "ensureOrderProcessStatuses; orderShippingReady; ship_ready query"},
		{table: "req_dev", code: "DEV-144-03", title: "生产流程页前置到生产管理菜单，手册说明库存充足直接发货、无需生产状态和发货状态处理", status: "done", assignee: "Codex", evidence: "menu-ia productionManual label; ProductionManualView; production-flow-user-manual"},
		{table: "req_unit", code: "UT-144-01", title: "单测覆盖生产计划移除物料需求计划、无需生产可发货、生产流程菜单入口和需求种子", status: "done", assignee: "Codex", evidence: "dev_144_production_direct_ship_flow_test.go; order_api_test.go"},
		{table: "req_api", code: "API-144-01", title: "API 测试覆盖 GET /api/orders?ship_ready=1 包含无需生产订单，POST /api/orders/shipping-excel 可导出无需生产订单", status: "done", assignee: "Codex", evidence: "TestOrdersShippingExcelAPIAcceptsNoProductionShipReadyOrders"},
		{table: "req_review", code: "REV-144-01", prCode: "PR-144", title: "验收：生产计划看不到物料需求计划；库存充足时按无需生产处理，订单列表可筛出并直接发货；生产流程页成为新员工入口", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-145", title: "手机从 /app 登录时只需一次外层 order 认证和一次系统账号登录，不因根路径跳转反复登录", status: "review", assignee: "VA", evidence: "codex/mobile-login-prefix-20260503"},
		{table: "req_dev", code: "DEV-145-01", title: "登录页短信/密码 API 请求和登录成功回跳必须保留当前 /app 前缀，避免离开手机 BasicAuth 保护空间", status: "done", assignee: "Codex", evidence: "templates/login.html appPath"},
		{table: "req_dev", code: "DEV-145-02", title: "Vue/Vite API 客户端和退出回登录逻辑必须根据当前 URL 自动补 /app 前缀，页面内返回/跳转不离开入口前缀", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/api/client.js appURL/apiURL; App.vue redirectToLogin"},
		{table: "req_dev", code: "DEV-145-03", title: "后端旧入口和 Vue shell 跳转必须使用可解析到当前前缀根的相对 Location，兼容 /app/* 和根路径访问", status: "done", assignee: "Codex", evidence: "support.PrefixRelativeLocation; VueShellRedirectWith; legacy redirects"},
		{table: "req_unit", code: "UT-145-01", title: "单测覆盖登录页前缀保留、Vue API URL 前缀保留、无 token 回登录保留前缀、旧入口相对重定向和需求种子", status: "done", assignee: "Codex", evidence: "auth_middleware_test.go; client.test.js; dev_145_mobile_login_prefix_test.go"},
		{table: "req_api", code: "API-145-01", title: "API/handler 级测试覆盖 legacy routes Location 为相对地址并解析到 Vue shell，订单 API redirect_url 保留当前前缀语义", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support ./internal/interfaces/http/sales ./internal/interfaces/http/production -count=1"},
		{table: "req_review", code: "REV-145-01", prCode: "PR-145", title: "验收：手机打开 https://erp.qacoohee.com/app/ 后输入一次 order 和一次系统账号即可进入；刷新/页面跳转/退出回登录不再要求重复认证", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-146", title: "顺丰发货命名统一；出库日志整合到库存管理并可查看和下载出库单", status: "review", assignee: "VA", evidence: "codex/outbound-log-stock-sf-20260503"},
		{table: "req_dev", code: "DEV-146-01", title: "订单列表顺丰导出入口统一展示为顺丰发货，保留既有导出兼容接口", status: "done", assignee: "Codex", evidence: "OrdersView 顺丰发货; deliveryMethodDisplayName 顺丰发货"},
		{table: "req_dev", code: "DEV-146-02", title: "库存管理新增出库日志页面，按出库单文档列出订单、客户、出库仓、快递、版本和状态，并支持查看抽屉和下载出库单", status: "done", assignee: "Codex", evidence: "GET /api/stock/outbound-logs; StockOutboundLogsView.vue"},
		{table: "req_unit", code: "UT-146-01", title: "源码守卫覆盖出库日志库存菜单、顺丰发货命名、出库单查看下载入口和需求种子", status: "done", assignee: "Codex", evidence: "dev_145_outbound_logs_stock_test.go"},
		{table: "req_api", code: "API-146-01", title: "API 测试覆盖 GET /api/stock/outbound-logs 返回出库单文档、顺丰发货显示名、查看和下载 URL", status: "done", assignee: "Codex", evidence: "TestStockAPIRoutes outbound logs; TestListOutboundLogsReturnsDeliveryNoteDocumentsForInventory"},
		{table: "req_review", code: "REV-146-01", prCode: "PR-146", title: "验收：库存管理可打开出库日志，看到已生成出库单，点击查看出库单打开抽屉，点击下载出库单得到 PDF；订单列表显示顺丰发货", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-147", title: "生产计划库存充足/不足分开展示；录单库存充足时选择成品批次，确认后库存待发货，拒绝后进入生产计划", status: "review", assignee: "VA", evidence: "codex/order-stock-batch-confirm-20260503"},
		{table: "req_dev", code: "DEV-147-01", title: "生产计划 Vue 页面按库存不足和库存充足分区，库存不足区用复选框支持全选和全取消，库存充足只提示", status: "done", assignee: "Codex", evidence: "ProducePlanView stockInsufficientRows stockSufficientRows toggleAllInsufficient"},
		{table: "req_dev", code: "DEV-147-02", title: "录单保存前新增成品批次预检 API 和确认提示；确认使用批次时保存 use_batch 决策和批次分配，订单状态置为库存待发货", status: "done", assignee: "Codex", evidence: "POST /api/order/stock-batch-preview; order_stock_batch_allocations; 库存待发货"},
		{table: "req_dev", code: "DEV-147-03", title: "拒绝使用库存批次时保存 produce 决策，生产计划按强制缺口处理；只看可发货和顺丰发货纳入库存待发货", status: "done", assignee: "Codex", evidence: "order_stock_decisions decision=produce; fetchUnproducedNeeds force_produce_units; ship_ready"},
		{table: "req_unit", code: "UT-147-01", title: "单测/源码守卫覆盖生产计划分区全选、录单批次确认、库存决策和需求种子", status: "done", assignee: "Codex", evidence: "dev_147_order_stock_batch_confirm_test.go; TestServiceValidatesAndDelegatesStockBatchPreview"},
		{table: "req_api", code: "API-147-01", title: "API 测试覆盖批次预检、使用批次保存为库存待发货、拒绝批次进入生产缺口和可发货查询", status: "done", assignee: "Codex", evidence: "TestOrderStockBatchPreviewAPIShowsFIFOBatchChoice; TestOrderAPISaveWithStockBatchDecisionMarksInventoryReadyAndStoresBatchChoice; TestProducePlanTreatsDeclinedStockBatchDecisionAsProductionGap"},
		{table: "req_review", code: "REV-147-01", prCode: "PR-147", title: "验收：生产计划库存不足可全选/全取消，库存充足只提示；录单批次确认使用后库存待发货，取消后进入生产计划", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-148", title: "录单批次预检兼容旧成品库存余额；库存充足但没有 FP 批次时仍可确认库存待发货并发货", status: "review", assignee: "VA", evidence: "codex/order-stock-legacy-batch-hotfix-20260503"},
		{table: "req_dev", code: "DEV-148-01", title: "成品无可用 stock_batches 行时，批次预检按 finished_inventory 生成 LEGACY-FP 库存余额候选；真实成品批次存在时不绕过质检/冻结规则", status: "done", assignee: "Codex", evidence: "loadLegacyFinishedInventoryAvailability; hasAnyFinishedStockBatchRows"},
		{table: "req_unit", code: "UT-148-01", title: "单测覆盖带编码虚拟批次 batch_id=0 可被 FIFO 分配，避免旧库存余额被跳过", status: "done", assignee: "Codex", evidence: "TestAllocateFIFOAllowsNamedVirtualBatchWithoutNumericID"},
		{table: "req_api", code: "API-148-01", title: "API 测试覆盖旧成品库存余额预检、使用库存余额保存为库存待发货，并进入只看可发货列表", status: "done", assignee: "Codex", evidence: "TestOrderStockBatchPreviewAPIUsesLegacyFinishedInventoryWhenNoBatchRows; TestOrderAPISaveWithLegacyFinishedInventoryDecisionMarksReadyAndShipReady"},
		{table: "req_review", code: "REV-148-01", prCode: "PR-148", title: "验收：库存汇总充足但没有 FP 批次的订单保存时出现 LEGACY-FP 库存余额提示；确认使用后可顺丰发货并生成出库单", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-149", title: "生产计划库存不足批量选择只保留标题左侧复选框，去掉顶部全选/全取消按钮，并支持半选状态", status: "review", assignee: "VA", evidence: "codex/produce-plan-insufficient-checkbox-20260503"},
		{table: "req_dev", code: "DEV-149-01", title: "生产计划 Vue 页面移除库存不足全选/全取消、全选库存不足和全取消动作按钮，在库存不足标题左侧放置唯一批量复选框", status: "done", assignee: "Codex", evidence: "ProducePlanView insufficientHeaderCheckbox; toggleAllInsufficient; no obsolete bulk buttons"},
		{table: "req_unit", code: "UT-149-01", title: "单测覆盖库存不足批量选择的未选、全选、半选状态和源码守卫禁止旧按钮回归", status: "done", assignee: "Codex", evidence: "produce-plan.test.js; dev_149_produce_plan_checkbox_test.go"},
		{table: "req_api", code: "API-149-01", title: "API 回归沿用现有生产计划接口，GET /api/produce/unproduced 与 POST /api/produce/start 的 selected key 语义不变", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/production -run 'TestProducePlan|TestProduceStartAPI' -count=1"},
		{table: "req_review", code: "REV-149-01", prCode: "PR-149", title: "验收：库存不足（需生产）标题左侧复选框勾选即全选，取消即全不选；单独勾选部分订单时显示减号；顶部不再有多余批量按钮", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-150", title: "库存待发货订单在回填快递单号时正式扣成品库存，并修复已发货历史订单误入生产计划", status: "review", assignee: "VA", evidence: "codex/order-stock-shipment-deduction-20260503"},
		{table: "req_dev", code: "DEV-150-01", title: "新增订单库存出库扣减记录，回填快递单号时扣减 FP 批次或 LEGACY-FP 对应 finished_inventory 并写库存流水，保证幂等", status: "done", assignee: "Codex", evidence: "order_stock_deductions; deductOrderAllocatedStockTx; sales_order_shipment ledger"},
		{table: "req_dev", code: "DEV-150-02", title: "生产计划排除已发货订单，并把库存待发货未发货预留量从可用成品库存中扣除，避免重复占用", status: "done", assignee: "Codex", evidence: "fetchUnproducedNeeds ship_status filter; reserved allocations excluding deductions"},
		{table: "req_unit", code: "UT-150-01", title: "单元/集成测试覆盖已发货空生产状态订单不进入生产计划、库存发货扣减幂等和需求种子", status: "done", assignee: "Codex", evidence: "TestProducePlanExcludesShippedOrdersWithBlankProcessStatus; TestOrdersShippingTrackingAPIDeductsReservedLegacyFinishedInventoryOnce; TestOrdersSingleShippingTrackingAPIDeductsReservedFinishedBatch"},
		{table: "req_api", code: "API-150-01", title: "API 测试覆盖回填快递单号触发 FP/LEGACY-FP 库存扣减和库存流水；生产计划只显示拒绝用库存订单自身缺口", status: "done", assignee: "Codex", evidence: "POST /api/orders/shipping-tracking; POST /api/orders/:id/shipping-tracking; GET /api/produce/unproduced"},
		{table: "req_review", code: "REV-150-01", prCode: "PR-150", title: "验收：使用库存的订单发货后库存减少且流水可查；SO-20260503-0001 取消使用库存后生产计划只按自身 2 件处理，不再被已发货旧单放大到 82 件", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-154", title: "开发环境登录修复：旧 token 或 API 401 不能反复弹出浏览器原生 order 密码框", status: "review", assignee: "VA", evidence: "codex/development-auth-challenge-hotfix-20260503"},
		{table: "req_dev", code: "DEV-154-01", title: "BasicAuth 中间件区分页面访问与 API/Bearer 失败：页面可触发外层 BasicAuth，API 或 Bearer 失效只返回 JSON 401", status: "done", assignee: "Codex", evidence: "shouldAdvertiseBasicAuthChallenge; auth_middleware.go"},
		{table: "req_unit", code: "UT-154-01", title: "单测覆盖 API 请求、/app/api 请求和失效 Bearer token 不返回 BasicAuth challenge，页面请求仍允许外层提示", status: "done", assignee: "Codex", evidence: "TestBasicAuthChallengeIsNotAdvertisedForAPIOrBearerFailures"},
		{table: "req_api", code: "API-154-01", title: "公网开发环境验证失效 Bearer 的 /app/api/auth/me 返回 401 JSON 且无 WWW-Authenticate；系统账号登录后 /api/auth/me 返回 200", status: "done", assignee: "Codex", evidence: "postdeploy curl /app/api/auth/me; browser smoke"},
		{table: "req_review", code: "REV-154-01", prCode: "PR-154", title: "验收：浏览器有旧 token 或系统账号过期时不再反复弹 order 密码框，重新打开 /app/login 后可进入工作台", status: "todo", assignee: "VA", evidence: "待 Van 验收"},
		{table: "req_product", code: "PR-FIN-001", title: "财务月结一期：面向咖啡烘焙厂、咖啡贸易商、咖啡壳豆加工厂，提供月度收入、成本、费用、毛利、净利、税费估算、强锁账、结账调整和 PDF/Excel 报告", status: "review", assignee: "VA", evidence: "预计开发 5-7 个工作日；codex/finance-monthly-closing-20260502"},
		{table: "req_dev", code: "DEV-FIN-001", title: "新增 finance 领域模型，支持小规模/一般纳税人、咖啡企业类型、毛利净利、增值税、附加税、企业所得税和小微优惠估算", status: "done", assignee: "Codex", evidence: "internal/domain/finance; go test ./internal/domain/finance"},
		{table: "req_dev", code: "DEV-FIN-002", title: "新增 finance 应用服务，支持设置、费用、月度报表、默认强锁账、隐藏白名单锁账模式切换和结账后金额调整", status: "done", assignee: "Codex", evidence: "internal/application/finance; go test ./internal/application/finance"},
		{table: "req_dev", code: "DEV-FIN-003", title: "新增 finance Postgres schema/repository，保存财务设置、费用、月结快照、调整记录，并从订单、生产成本和费用表聚合月度来源", status: "done", assignee: "Codex", evidence: "internal/infrastructure/postgres/finance; go test ./internal/infrastructure/postgres/finance"},
		{table: "req_dev", code: "DEV-FIN-004", title: "新增财务 HTTP API 和 PDF/Excel 导出接口，覆盖设置、首页、费用、报表、结账、调整和下载", status: "done", assignee: "Codex", evidence: "internal/interfaces/http/finance; internal/infrastructure/pdf; internal/infrastructure/excel"},
		{table: "req_dev", code: "DEV-FIN-005", title: "财务模块接入 appmain、schema 初始化和权限体系，新增 finance.read/write/close/close_mode.manage 与财务菜单权限", status: "done", assignee: "Codex", evidence: "app_routes.go; schema_setup.go; authz schema; AuthorizationMiddleware"},
		{table: "req_dev", code: "DEV-FIN-006", title: "Vue/Vite 新增财务首页、费用管理、月度结账、经营报告和财务设置页面，锁账模式切换仅对白名单用户显示", status: "done", assignee: "Codex", evidence: "frontend-vue-shell/src/views/Finance*.vue; npm run build"},
		{table: "req_dev", code: "DEV-FIN-007", title: "补充财务月结产品需求、验收清单、操作手册和需求表种子", status: "done", assignee: "Codex", evidence: "docs/REQUIREMENTS.md; docs/ACCEPTANCE_TESTS.md; finance-monthly-closing-user-manual.md"},
		{table: "req_unit", code: "UT-FIN-001", title: "单测覆盖财务领域税费估算、毛利净利、调整和强锁账可编辑规则", status: "done", assignee: "Codex", evidence: "go test ./internal/domain/finance -count=1"},
		{table: "req_unit", code: "UT-FIN-002", title: "单测覆盖财务应用服务设置、费用校验、强锁账拦截、月结状态、调整和白名单切换", status: "done", assignee: "Codex", evidence: "go test ./internal/application/finance -count=1"},
		{table: "req_unit", code: "UT-FIN-003", title: "单测/源码守卫覆盖财务仓储 schema、月度聚合、appmain 接线、权限、文档和需求种子", status: "done", assignee: "Codex", evidence: "go test ./internal/infrastructure/postgres/finance ./internal/interfaces/http/support -count=1"},
		{table: "req_unit", code: "UT-FIN-004", title: "前端单测覆盖财务格式化、API 封装、菜单入口和 Vue 页面接线", status: "done", assignee: "Codex", evidence: "node --test src/lib/*.test.js src/api/*.test.js"},
		{table: "req_api", code: "API-FIN-001", title: "API 测试覆盖财务设置、首页、费用、月度报表、结账、调整、PDF 和 Excel 导出", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/finance -count=1"},
		{table: "req_api", code: "API-FIN-002", title: "API 级权限验证覆盖财务接口权限映射、结账权限和锁账模式管理权限", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/support -count=1"},
		{table: "req_review", code: "REV-FIN-001", prCode: "PR-FIN-001", title: "验收：财务首页能看当月收入/成本/费用/毛利/净利/税费，费用可补录，默认强锁账，结账后只能走金额调整，报告可导出 PDF 和 Excel", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收；自动化验收随分支测试记录"},
		{table: "req_product", code: "PR-FIN-002", title: "费用管理保存费用后必须显示在正确月份；费用日期和月份筛选不一致时页面应自动切到费用发生月份", status: "review", assignee: "VA", evidence: "codex/finance-expense-month-sync-20260502"},
		{table: "req_dev", code: "DEV-FIN-008", title: "费用管理页新增费用日期到月份筛选的同步逻辑，保存后按后端返回的费用月份刷新列表", status: "done", assignee: "Codex", evidence: "FinanceExpensesView syncMonthFromDate; created.month"},
		{table: "req_unit", code: "UT-FIN-005", title: "前端单测覆盖 monthFromDate 和费用页保存后按费用月份刷新列表的源码守卫", status: "done", assignee: "Codex", evidence: "finance.test.js; finance-ui.test.js"},
		{table: "req_api", code: "API-FIN-003", title: "线上 API 验证 2026-04 费用可通过 GET /api/finance/expenses?month=2026-04 查到", status: "done", assignee: "Codex", evidence: "curl /api/finance/expenses?month=2026-04 返回 id=1 人工费用"},
		{table: "req_review", code: "REV-FIN-002", prCode: "PR-FIN-002", title: "验收：在费用管理选择 2026-04-15 费用日期保存后，页面自动切到 2026-04 并展示该费用", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-FIN-003", title: "费用管理必须支持费用关联员工；点击费用列表中的员工名后，应过滤出该员工相关费用", status: "review", assignee: "VA", evidence: "codex/finance-expense-employee-filter-20260502"},
		{table: "req_dev", code: "DEV-FIN-009", title: "费用表新增 employee_id 关联公司员工，费用 API 支持员工候选、employee_id 保存和筛选，费用管理页新增员工选择、员工列和点击员工过滤", status: "done", assignee: "Codex", evidence: "finance_expenses.employee_id; /api/finance/employees; ExpenseFilter; FinanceExpensesView employeeFilter"},
		{table: "req_unit", code: "UT-FIN-006", title: "单测覆盖费用员工关联字段、员工过滤参数、仓储员工 join、前端 API 参数和 Vue 源码守卫", status: "done", assignee: "Codex", evidence: "go test ./internal/application/finance ./internal/infrastructure/postgres/finance; node --test finance.test.js finance-ui.test.js"},
		{table: "req_api", code: "API-FIN-004", title: "API 测试覆盖 POST /api/finance/expenses 保存 employee_id，以及 GET /api/finance/expenses?employee_id=... 过滤员工费用", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/finance -count=1"},
		{table: "req_review", code: "REV-FIN-003", prCode: "PR-FIN-003", title: "验收：新增费用选择员工后列表显示员工名；点击员工名后列表只显示该员工相关费用，可清除员工筛选", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-FIN-004", title: "费用管理的类别和付款方式必须使用可搜索候选列表，内置覆盖常见咖啡烘焙、贸易和加工费用场景，并允许自定义输入", status: "review", assignee: "VA", evidence: "codex/finance-expense-selectable-fields-20260502"},
		{table: "req_dev", code: "DEV-FIN-010", title: "Vue 费用管理新增类别和付款方式候选数据、输入框下拉候选、模糊筛选和自定义输入保留", status: "done", assignee: "Codex", evidence: "finance-expense-options.js; FinanceExpensesView filteredExpenseCategoryOptions/filteredExpensePaymentOptions"},
		{table: "req_unit", code: "UT-FIN-007", title: "前端单测覆盖类别/付款方式候选数量、常用项、模糊匹配 helper 和费用页 datalist 接线", status: "done", assignee: "Codex", evidence: "node --test src/lib/finance-ui.test.js src/lib/finance-expense-options.test.js"},
		{table: "req_api", code: "API-FIN-005", title: "API 测试覆盖费用保存时类别、付款方式和员工字段进入 POST /api/finance/expenses 并在响应中返回", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/finance -run TestFinanceExpenseAndClosingAPI -count=1"},
		{table: "req_review", code: "REV-FIN-004", prCode: "PR-FIN-004", title: "验收：费用管理新增费用时，类别和付款方式可从下拉候选选择；输入关键字可模糊筛选；仍可保存自定义值", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-FIN-005", title: "财务二期：参照国内 ERP 月结体验，补齐月结前检查、报表来源钻取、票税台账、成本配比异常、费用多维归集和会计交接导出", status: "review", assignee: "VA", evidence: "codex/finance-improvements-yonyou-20260503"},
		{table: "req_dev", code: "DEV-FIN-011", title: "新增月结前检查服务和 API，结账前展示来源异常、来源明细、票税台账、成本配比和会计交接检查项", status: "done", assignee: "Codex", evidence: "ClosingReview; GET /api/finance/reports/:month/closing-review"},
		{table: "req_dev", code: "DEV-FIN-012", title: "新增经营报告来源钻取，按收入、主营成本、期间费用和票税来源列出来源单据与金额", status: "done", assignee: "Codex", evidence: "FinanceSourceDetails; GET /api/finance/reports/:month/drilldown"},
		{table: "req_dev", code: "DEV-FIN-013", title: "新增票税台账 schema、仓储、服务、API 和 Vue 页面，记录销售发票、采购发票、税款缴纳和其他票税事项", status: "done", assignee: "Codex", evidence: "finance_tax_ledger; FinanceTaxLedgerView.vue"},
		{table: "req_dev", code: "DEV-FIN-014", title: "新增成本配比异常检查，收入存在但主营成本缺失时在月结前检查中提示处理", status: "done", assignee: "Codex", evidence: "cost_matching check"},
		{table: "req_dev", code: "DEV-FIN-015", title: "费用管理新增订单、客户、商品、批次和维度说明字段，支持费用按业务维度归集和后续分析", status: "done", assignee: "Codex", evidence: "finance_expenses order_id/customer_id/product_id/batch_no/dimension_note"},
		{table: "req_dev", code: "DEV-FIN-016", title: "新增会计交接 Excel 导出，包含经营汇总、月结检查、来源明细、票税台账和凭证草稿", status: "done", assignee: "Codex", evidence: "GET /api/finance/reports/:month/accountant-handoff.xlsx"},
		{table: "req_unit", code: "UT-FIN-008", title: "单测覆盖财务二期服务、仓储 schema/source guard、Vue 接线和需求表种子", status: "done", assignee: "Codex", evidence: "go test ./internal/application/finance ./internal/infrastructure/postgres/finance ./internal/interfaces/http/support; node --test finance-ui.test.js"},
		{table: "req_api", code: "API-FIN-006", title: "API 测试覆盖月结前检查、报表钻取、票税台账增查和会计交接 Excel 下载", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/finance -run TestFinanceImprovementAPIs -count=1"},
		{table: "req_review", code: "REV-FIN-005", prCode: "PR-FIN-005", title: "验收：月结页显示月结前检查；经营报告可看来源明细并导出会计交接；票税台账可新增和查看；费用可记录业务维度", status: "todo", assignee: "VA", evidence: "待 Van 服务器验收"},
		{table: "req_product", code: "PR-CUSTOMER-PORTAL-P0", title: "客户服务平台 P0：小程序客户登录、客户绑定、服务能力配置和客户首页底座", status: "review", assignee: "VA", evidence: "docs/superpowers/specs/2026-05-03-customer-service-platform-design.md; docs/superpowers/plans/2026-05-03-customer-portal-p0.md"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-01", title: "新增 customerportal application service，统一 mini user、客户绑定、当前客户和服务能力聚合规则", status: "done", assignee: "Codex", evidence: "go test ./internal/application/customerportal -count=1; go test ./... -count=1"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-02", title: "新增 customerportal Postgres schema/repository，包含 mini_users、mini_sessions、customer_portal_profiles、customer_portal_user_bindings、customer_service_capabilities", status: "done", assignee: "Codex", evidence: "ORDERAPP_TEST_DATABASE_URL=<temp-postgres> go test ./internal/infrastructure/postgres/customerportal -count=1 -v; go test ./internal/appmain -count=1; go test ./... -count=1"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-03", title: "新增 /api/mini/login、/api/mini/me、/api/mini/current-customer，并让 /api/mini/* 使用小程序 token 自行鉴权", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal ./internal/interfaces/http/support ./internal/appmain -count=1; go test ./... -count=1"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-04", title: "新增 uni-app 微信小程序骨架，包含登录页、客户首页、token 存储和按服务能力显示入口", status: "done", assignee: "Codex", evidence: "npm run typecheck --prefix miniapp; npm test --prefix miniapp; npm run build:mp-weixin --prefix miniapp"},
		{table: "req_unit", code: "UT-CUSTOMER-PORTAL-P0-01", title: "单元测试覆盖客户门户服务能力聚合、schema 表定义、miniapp 能力入口显示逻辑和需求种子", status: "done", assignee: "Codex", evidence: "go test ./internal/application/customerportal ./internal/interfaces/http/support -count=1; ORDERAPP_TEST_DATABASE_URL=<temp-postgres> go test ./internal/infrastructure/postgres/customerportal -count=1 -v; npm test --prefix miniapp"},
		{table: "req_api", code: "API-CUSTOMER-PORTAL-P0-01", title: "API 测试覆盖 /api/mini/login、/api/mini/me、/api/mini/current-customer 和未绑定客户的 401/403 行为", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1; go test ./internal/interfaces/http/support -run TestMiniAPI -count=1"},
		{table: "req_review", code: "REV-CUSTOMER-PORTAL-P0-01", prCode: "PR-CUSTOMER-PORTAL-P0", title: "验收：小程序用户登录后只能看到已绑定客户和该客户开通的服务能力入口", status: "todo", assignee: "VA", evidence: "待 Van 小程序开发工具/接口验收"},
		{table: "req_product", code: "PR-CUSTOMER-PORTAL-WECHAT-CONNECT", title: "客户服务平台小程序联调：测试小程序可通过微信登录或稳定模拟登录打通客户门户底座", status: "review", assignee: "VA", evidence: "codex/customer-portal-wechat-connect-20260503"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-WECHAT-CONNECT-01", title: "后端支持 WECHAT_MINI_APP_ID/WECHAT_MINI_APP_SECRET，生产模式通过微信 code2Session 换取 openid 后创建客户门户会话", status: "done", assignee: "Codex", evidence: "WechatIdentityProvider; customerPortalIdentityProvider"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-WECHAT-CONNECT-02", title: "测试模式支持 CUSTOMER_PORTAL_DEV_OPENID 稳定模拟 openid，小程序 API 基地址支持 VITE_KFERP_API_BASE 配置", status: "done", assignee: "Codex", evidence: "StaticIdentityProvider OpenID; miniapp buildAPIURL"},
		{table: "req_unit", code: "UT-CUSTOMER-PORTAL-WECHAT-CONNECT-01", title: "单元测试覆盖微信配置读取、身份提供器选择、code2Session 请求、稳定模拟 openid 和小程序 API 地址拼接", status: "done", assignee: "Codex", evidence: "go test ./internal/config ./internal/interfaces/http/customerportal ./internal/appmain; npm test --prefix miniapp"},
		{table: "req_api", code: "API-CUSTOMER-PORTAL-WECHAT-CONNECT-01", title: "API 验证覆盖 /app/api/mini/login 在未配置微信时禁用、配置测试登录时可生成 token、/app/api/mini/me 可按 token 返回客户上下文", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1; deploy smoke 待执行"},
		{table: "req_review", code: "REV-CUSTOMER-PORTAL-WECHAT-CONNECT-01", prCode: "PR-CUSTOMER-PORTAL-WECHAT-CONNECT", title: "验收：微信开发者工具导入测试小程序后，点击登录可进入客户中心并看到绑定客户的能力入口", status: "todo", assignee: "VA", evidence: "待 Van 使用测试小程序验收"},
		{table: "req_product", code: "PR-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS", title: "客户中心服务入口必须可点击，点击后进入对应服务占位页并明确当前接入状态", status: "review", assignee: "VA", evidence: "codex/miniapp-entry-actions-20260503"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01", title: "小程序客户首页入口绑定 tap 导航，新增统一服务入口页，保留后续业务页面接入空间", status: "done", assignee: "Codex", evidence: "home.vue openEntry; pages/service/service.vue"},
		{table: "req_unit", code: "UT-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01", title: "单元测试覆盖每个可见入口带服务页 URL，源码守卫覆盖首页 tap、服务页注册和占位说明", status: "done", assignee: "Codex", evidence: "npm test --prefix miniapp -- capabilities.test.ts; go test ./internal/interfaces/http/support -run TestCustomerPortalMiniappEntriesAreTappable"},
		{table: "req_api", code: "API-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01", title: "API 层不变，复用 /app/api/mini/me 的客户能力返回作为入口展示来源，微信构建验证页面路由可编译", status: "done", assignee: "Codex", evidence: "VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp"},
		{table: "req_review", code: "REV-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01", prCode: "PR-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS", title: "验收：客户中心每个服务卡片点击后进入对应服务入口页，页面显示入口已打通和后续接入状态", status: "todo", assignee: "VA", evidence: "待 Van 微信开发者工具验收"},
		{table: "req_product", code: "PR-CUSTOMER-PORTAL-BUSINESS-TASKS", title: "客户服务平台一期业务能力：客户豆单、现货/订单、代发、代加工、库存和结算在小程序可查询或提交", status: "review", assignee: "VA", evidence: "codex/customer-portal-business-tasks-20260503"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-01", title: "新增 /api/mini/services/:key，按当前绑定客户和服务能力返回客户豆单、商品、订单、物流、库存、费用和结算只读数据", status: "done", assignee: "Codex", evidence: "GetServicePage; LoadServicePage; GET /api/mini/services/:key"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-02", title: "新增一件代发批次表和小程序提交 API，批次只归属上游客户，下游收件人不写入 customers", status: "done", assignee: "Codex", evidence: "direct_ship_import_batches; POST /api/mini/direct-ship/batches"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-03", title: "新增代加工申请表和小程序提交 API，第一版只提交申请与状态，不自动扣料或完工", status: "done", assignee: "Codex", evidence: "processing_job_requests; POST /api/mini/processing-requests"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-04", title: "新增客户库存托管快照表，支撑小程序按客户查看生豆、成品或包材库存", status: "done", assignee: "Codex", evidence: "customer_inventory_items; /api/mini/services/inventory"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-05", title: "新增统一客户费用明细和结算单表，覆盖商品费、代加工费、运费、代发服务费、包材费等费用类型", status: "done", assignee: "Codex", evidence: "customer_fee_items; customer_settlement_batches; /api/mini/services/settlement"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-BUSINESS-TASK-06", title: "小程序服务页接入真实业务 API，支持查看业务摘要、列表，并提交一件代发批次和代加工申请", status: "done", assignee: "Codex", evidence: "miniapp/src/pages/service/service.vue; miniapp/src/api/customerPortal.ts"},
		{table: "req_unit", code: "UT-CUSTOMER-PORTAL-BUSINESS-TASKS-01", title: "单元测试覆盖客户能力授权、服务页数据契约、业务表 schema、需求种子和小程序服务页 helper", status: "done", assignee: "Codex", evidence: "go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support; npm test --prefix miniapp"},
		{table: "req_api", code: "API-CUSTOMER-PORTAL-BUSINESS-TASKS-01", title: "API 测试覆盖服务页查询、一件代发提交、代加工提交、未授权能力 403 和无 token 401", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1"},
		{table: "req_review", code: "REV-CUSTOMER-PORTAL-BUSINESS-TASKS-01", prCode: "PR-CUSTOMER-PORTAL-BUSINESS-TASKS", title: "验收：测试客户进入小程序后可点击各服务入口看到真实数据空态/列表，并能提交代发批次和加工申请", status: "todo", assignee: "VA", evidence: "待 Van 微信开发者工具验收"},
		{table: "req_product", code: "PR-CUSTOMER-PORTAL-VISIBILITY-ORDERS", title: "客户门户配置和小程序订单可见性：后台可配置客户能力，小程序可直观看历史订单、状态、价格，豆单默认连接系统最新豆单", status: "review", assignee: "VA", evidence: "codex/customer-portal-visibility-orders-beanlist-20260503"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01", title: "新增 ERP 客户门户配置页和后台 API，按客户查看绑定小程序用户并保存服务能力开关", status: "done", assignee: "Codex", evidence: "CustomerPortalSettingsView; /api/customer-portal/admin/customers"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-02", title: "小程序订单服务返回订单行明细、金额、生产/收款/发货/物流状态，并在服务页展示", status: "done", assignee: "Codex", evidence: "CustomerOrderItemSummary; listCustomerOrderItems; service.vue"},
		{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-03", title: "我的豆单优先读取客户专属已发布豆单，没有专属豆单时 fallback 到系统最新官方已发布豆单", status: "done", assignee: "Codex", evidence: "listBeanLists; listLatestOfficialBeanLists"},
		{table: "req_unit", code: "UT-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01", title: "单元测试覆盖能力目录归一化、仓储源码守卫、Vue 菜单/页面接线、小程序订单明细展示和需求种子", status: "done", assignee: "Codex", evidence: "go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support ./internal/infrastructure/postgres/authz; node --test menu-ia.test.js"},
		{table: "req_api", code: "API-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01", title: "API 测试覆盖客户门户配置列表/详情/保存，以及 mini 服务页订单明细 JSON 契约", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1"},
		{table: "req_review", code: "REV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01", prCode: "PR-CUSTOMER-PORTAL-VISIBILITY-ORDERS", title: "验收：13800138075 可在 ERP 客户门户配置页调整能力；小程序可看历史订单状态、价格和明细；我的豆单可显示系统最新豆单", status: "todo", assignee: "VA", evidence: "待 Van ERP 后台和微信开发者工具验收"},
	} {
		if err := seedReqRow(ctx, pool, schema, row); err != nil {
			return err
		}
	}
	return nil
}
