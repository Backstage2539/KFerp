package support

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditPageData struct {
	From       string
	To         string
	Q          string
	EntityType string
	Rows       []AuditLogRow
	Error      string
}

type AuditPageResult struct {
	Rows  []AuditLogRow
	Total int
}

func fetchAuditPage(ctx context.Context, pool *pgxpool.Pool, schema string, from, to, q, entityType string, limit, offset int) (AuditPageResult, error) {
	w := make([]string, 0)
	args := make([]any, 0)
	arg := 1

	payMap, _ := fetchIDNameMap(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses", schema))
	shipMap, _ := fetchIDNameMap(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses", schema))

	if strings.TrimSpace(from) != "" {
		w = append(w, fmt.Sprintf("ts >= $%d", arg))
		args = append(args, from+" 00:00:00")
		arg++
	}
	if strings.TrimSpace(to) != "" {
		w = append(w, fmt.Sprintf("ts <= $%d", arg))
		args = append(args, to+" 23:59:59")
		arg++
	}
	if strings.TrimSpace(entityType) != "" {
		w = append(w, fmt.Sprintf("entity_type = $%d", arg))
		args = append(args, entityType)
		arg++
	}
	if strings.TrimSpace(q) != "" {
		// search in actor, action, field, old/new, and meta text
		w = append(w, fmt.Sprintf("(actor ILIKE $%d OR action ILIKE $%d OR COALESCE(field,'') ILIKE $%d OR COALESCE(old_value,'') ILIKE $%d OR COALESCE(new_value,'') ILIKE $%d OR COALESCE(meta::text,'') ILIKE $%d)", arg, arg, arg, arg, arg, arg))
		args = append(args, "%"+q+"%")
		arg++
	}

	where := ""
	if len(w) > 0 {
		where = "WHERE " + strings.Join(w, " AND ")
	}
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	countSQL := fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs a %s`, schema, where)
	var total int
	if err := pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return AuditPageResult{}, err
	}
	args = append(args, limit)
	limitArg := arg
	arg++
	args = append(args, offset)
	offsetArg := arg

	sql := fmt.Sprintf(`
		SELECT
			to_char(a.ts,'YYYY-MM-DD HH24:MI:SS') AS ts,
			a.actor, a.entity_type, a.entity_id,
			CASE
				WHEN a.entity_type='order' THEN o.order_no
				WHEN a.entity_type='product' THEN p.name
				WHEN a.entity_type='material' THEN m.name
				WHEN a.entity_type='customer' THEN c.name
				ELSE NULL
			END AS entity_label,
			CASE
				WHEN a.entity_type='order' THEN '/orders/' || a.entity_id::text
				WHEN a.entity_type='product' THEN '/products/' || a.entity_id::text
				WHEN a.entity_type='material' THEN '/materials?q=' || m.code
				WHEN a.entity_type='customer' THEN '/customers'
				ELSE NULL
			END AS entity_url,
			a.action, a.field, a.old_value, a.new_value,
			CASE WHEN a.meta IS NULL THEN NULL ELSE a.meta::text END AS meta
		FROM %s.audit_logs a
		LEFT JOIN %s.orders o ON a.entity_type='order' AND a.entity_id=o.id
		LEFT JOIN %s.products p ON a.entity_type='product' AND a.entity_id=p.id
		LEFT JOIN %s.materials m ON a.entity_type='material' AND a.entity_id=m.id
		LEFT JOIN %s.customers c ON a.entity_type='customer' AND a.entity_id=c.id
		%s
		ORDER BY a.id DESC
		LIMIT $%d OFFSET $%d
	`, schema, schema, schema, schema, schema, where, limitArg, offsetArg)

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return AuditPageResult{}, err
	}
	defer rows.Close()

	out := make([]AuditLogRow, 0)
	for rows.Next() {
		var r AuditLogRow
		if err := rows.Scan(&r.Ts, &r.Actor, &r.EntityType, &r.EntityID, &r.EntityLabel, &r.EntityURL, &r.Action, &r.Field, &r.OldValue, &r.NewValue, &r.Meta); err != nil {
			return AuditPageResult{}, err
		}

		if r.Field != nil {
			switch strings.TrimSpace(*r.Field) {
			case "pay_status_id":
				r.OldValue = idTextToLabel(r.OldValue, payMap)
				r.NewValue = idTextToLabel(r.NewValue, payMap)
			case "ship_status_id":
				r.OldValue = idTextToLabel(r.OldValue, shipMap)
				r.NewValue = idTextToLabel(r.NewValue, shipMap)
			}
		}
		decorateAuditLogRow(&r, payMap, shipMap)

		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return AuditPageResult{}, err
	}
	return AuditPageResult{Rows: out, Total: total}, nil
}

func decorateAuditLogRow(r *AuditLogRow, payMap, shipMap map[int64]string) {
	rawEntityType := strings.TrimSpace(r.EntityType)
	rawAction := strings.TrimSpace(r.Action)
	rawField := ""
	if r.Field != nil {
		rawField = strings.TrimSpace(*r.Field)
	}
	menu, feature := auditMenuFeature(rawEntityType, rawAction, rawField, r.Meta)
	r.Menu = menu
	r.Feature = feature
	r.EntityType = labelEntityType(rawEntityType)
	r.Action = labelAction(rawAction)
	if r.Field != nil {
		lf := labelField(rawField)
		r.Field = &lf
	}
	if r.EntityLabel == nil || strings.TrimSpace(*r.EntityLabel) == "" {
		if target := auditTargetName(r, rawEntityType); target != "" && target != r.EntityType {
			r.EntityLabel = &target
		}
	}
	r.Summary = auditSummary(r, rawEntityType, rawAction, rawField)
}

func auditMenuFeature(entityType, action, field string, meta *string) (string, string) {
	if entityType == "operation" {
		return operationMenuFeature(meta, field)
	}
	action = strings.TrimSpace(action)
	field = strings.TrimSpace(field)
	switch entityType {
	case "order":
		switch action {
		case "void":
			return "订单销售 / 订单列表", "作废订单"
		case "unvoid":
			return "订单销售 / 订单列表", "恢复订单"
		case "create":
			return "订单销售 / 录单", "新建订单"
		default:
			return "订单销售 / 订单列表", "编辑订单"
		}
	case "product":
		if action == "move" || field == "product_category" {
			return "商品与配方 / 产品设置", "调整产品分类"
		}
		if field == "price_tiers" {
			return "商品与配方 / 产品设置", "维护价格阶梯"
		}
		return "商品与配方 / 产品设置", "编辑产品设置"
	case "product_category":
		if action == "delete" {
			return "商品与配方 / 产品设置", "删除产品分类"
		}
		return "商品与配方 / 产品设置", "调整产品分类"
	case "material":
		return "库存管理 / 物料档案", "编辑物料档案"
	case "customer", "customer_asset":
		if entityType == "customer_asset" {
			return "订单销售 / 客户档案", "维护客户附件"
		}
		return "订单销售 / 客户档案", "编辑客户档案"
	case "company_profile":
		return "设置 / 公司设置", "保存公司设置"
	case "sales_order_settings":
		if field == "seal_asset_id" {
			return "设置 / 销售单设置", "上传销售单公章"
		}
		return "设置 / 销售单设置", "保存销售单设置"
	case "sales_order_asset":
		return "设置 / 销售单设置", "上传销售单素材"
	case "sales_order_payment_code":
		if action == "delete" {
			return "设置 / 销售单设置", "停用收款二维码"
		}
		return "设置 / 销售单设置", "维护收款二维码"
	case "sales_order_document":
		return "订单销售 / 销售单", "生成销售单PDF"
	case "sales_order_image":
		return "订单销售 / 销售单", "生成销售单图片"
	case "material_receipt":
		return "库存管理 / 采购入库", "提交原料入库"
	case "material_transfer":
		return "库存管理 / 库存作业", "提交原料转仓"
	case "finished_product_transfer":
		return "库存管理 / 库存作业", "提交成品转仓"
	case "finished_inventory":
		return "库存管理 / 仓库库存", "调整成品库存"
	case "stock_adjustment":
		return "库存管理 / 库存作业", "提交库存调整"
	case "cost_parameter":
		return "设置 / 成本参数设置", "保存成本参数"
	case "bean_list_publication":
		switch action {
		case "publish":
			return "商品与配方 / 产品设置", "发布豆单"
		case "save_draft":
			return "商品与配方 / 产品设置", "保存豆单草稿"
		case "withdraw":
			return "商品与配方 / 产品设置", "撤回豆单发布"
		default:
			return "商品与配方 / 产品设置", "维护豆单发布"
		}
	case "costing_run":
		if action == "publish" {
			return "生产管理 / 生产成本", "发布成本试算"
		}
		return "生产管理 / 生产成本", "保存成本试算"
	case "produce_batch":
		if action == "update" {
			return "生产管理 / 生产计划/开始生产", "扣减生产批次物料"
		}
		return "生产管理 / 生产计划/开始生产", "创建生产批次"
	case "produce_running":
		if action == "cancel" {
			return "生产管理 / 生产中", "取消生产"
		}
		if action == "partial_finish" {
			return "生产管理 / 生产中", "部分完成生产"
		}
		return "生产管理 / 生产中", "完成生产"
	case "wip_reservation":
		if action == "release" {
			return "生产管理 / 生产中", "释放WIP占用"
		}
		return "生产管理 / 生产中", "调整WIP占用"
	case "auth":
		return "移动端 / 登录", "员工登录"
	case "auth_account":
		if action == "reset_password" {
			return "系统 / 员工维护", "重置员工密码"
		}
		return "系统 / 员工维护", "修改员工账号状态"
	case "import":
		return "系统 / 导入", "导入数据"
	case "system":
		return "系统 / 系统任务", "系统操作"
	default:
		return "其他 / 未分类", "其他操作"
	}
}

func operationMenuFeature(meta *string, field string) (string, string) {
	method, route, path := operationMeta(meta)
	if method == "" || path == "" {
		method, path = splitOperationField(field)
	}
	target := operationTarget(route, path)
	switch {
	case strings.HasPrefix(target, "/api/costing/settings"):
		if method == "POST" {
			return "设置 / 成本参数设置", "保存成本参数"
		}
		return "设置 / 成本参数设置", "查看成本参数"
	case strings.HasPrefix(target, "/api/costing/bean-list/publications") && strings.Contains(target, "/withdraw"):
		return "商品与配方 / 产品设置", "撤回豆单发布"
	case strings.HasPrefix(target, "/api/costing/bean-list/publications"):
		if method == "POST" {
			return "商品与配方 / 产品设置", "发布豆单"
		}
		return "商品与配方 / 产品设置", "查看豆单发布"
	case strings.HasPrefix(target, "/api/costing/bean-list/drafts"):
		return "商品与配方 / 产品设置", "保存豆单草稿"
	case strings.HasPrefix(target, "/api/costing/runs") && strings.Contains(target, "/publish"):
		return "生产管理 / 生产成本", "发布成本试算"
	case strings.HasPrefix(target, "/api/costing/runs"):
		return "生产管理 / 生产成本", "保存成本试算"
	case strings.HasPrefix(target, "/api/costing/calculate"):
		return "生产管理 / 生产成本", "计算生产成本"
	case strings.HasPrefix(target, "/api/costing/bean-list"):
		return "商品与配方 / 产品设置", "生成豆单预览"
	case strings.HasPrefix(target, "/public/bean-list"):
		return "商品与配方 / 产品设置", "查看公开豆单"
	case strings.HasPrefix(target, "/api/stock/material-receipts"):
		if method == "POST" {
			return "库存管理 / 采购入库", "提交原料入库"
		}
		return "库存管理 / 采购入库", "查看原料入库"
	case strings.HasPrefix(target, "/api/stock/material-transfers"):
		if method == "POST" {
			return "库存管理 / 库存作业", "提交原料转仓"
		}
		return "库存管理 / 库存作业", "查看原料转仓"
	case strings.HasPrefix(target, "/api/stock/finished-transfers"):
		if method == "POST" {
			return "库存管理 / 库存作业", "提交成品转仓"
		}
		return "库存管理 / 库存作业", "查看成品转仓"
	case strings.HasPrefix(target, "/api/stock/adjustments"):
		return "库存管理 / 库存作业", "提交库存调整"
	case strings.HasPrefix(target, "/api/stock/ledger"):
		return "库存管理 / 库存流水", "查看库存流水"
	case strings.HasPrefix(target, "/api/stock/batches") || strings.HasPrefix(target, "/api/stock/material-batches") || strings.HasPrefix(target, "/api/stock/trace"):
		return "库存管理 / 批次追溯", "查看库存批次"
	case strings.HasPrefix(target, "/api/stock/warehouses") || strings.HasPrefix(target, "/api/stock/warehouse-inventory") || strings.HasPrefix(target, "/api/stock/material-batch-locations"):
		return "库存管理 / 仓库库存", "查看仓库库存"
	case strings.HasPrefix(target, "/api/purchase/receipts"):
		return "库存管理 / 采购入库", "维护采购入库"
	case strings.HasPrefix(target, "/api/purchase/orders"):
		return "库存管理 / 采购入库", "维护采购订单"
	case strings.HasPrefix(target, "/api/purchase/suppliers"):
		return "库存管理 / 采购入库", "维护供应商"
	case strings.HasPrefix(target, "/api/products/inventory"):
		if method == "POST" {
			return "库存管理 / 仓库库存", "调整成品库存"
		}
		return "库存管理 / 仓库库存", "查看成品库存"
	case strings.HasPrefix(target, "/api/materials/") || strings.HasPrefix(target, "/api/materials/:id"):
		if method == "POST" {
			return "库存管理 / 物料档案", "保存物料行内编辑"
		}
		return "库存管理 / 物料档案", "读取物料档案"
	case strings.HasPrefix(target, "/api/materials"):
		return "库存管理 / 物料档案", "加载物料列表"
	case strings.Contains(target, "/materials"):
		return "库存管理 / 物料档案", "打开物料档案"
	case strings.HasPrefix(target, "/api/product-settings/products") && strings.Contains(target, "/category"):
		return "商品与配方 / 产品设置", "调整产品分类"
	case strings.HasPrefix(target, "/api/product-settings/categories"):
		return "商品与配方 / 产品设置", "维护产品分类"
	case strings.HasPrefix(target, "/api/product-settings"):
		return "商品与配方 / 产品设置", "查看产品设置"
	case strings.HasPrefix(target, "/api/products") || strings.Contains(target, "/products"):
		return "商品与配方 / 产品设置", "维护产品设置"
	case strings.Contains(target, "/bom"):
		return "商品与配方 / BOM配方维护", "维护BOM配方"
	case strings.Contains(target, "/audit"):
		return "系统 / 操作日志", "查看操作日志"
	case strings.HasPrefix(target, "/api/settings/sales-order/seal-position"):
		return "设置 / 销售单设置", "保存公章位置"
	case strings.HasPrefix(target, "/api/settings/sales-order/seal") || strings.HasPrefix(target, "/api/settings/sales-order/payment-codes"):
		return "设置 / 销售单设置", "维护销售单素材"
	case strings.HasPrefix(target, "/api/settings/sales-order"):
		if method == "POST" {
			return "设置 / 销售单设置", "保存销售单设置"
		}
		return "设置 / 销售单设置", "查看销售单设置"
	case strings.HasPrefix(target, "/api/settings/sender") || strings.HasPrefix(target, "/settings/sender"):
		return "设置 / 发货人设置", "维护发货人"
	case strings.HasPrefix(target, "/api/outsource/templates") || strings.HasPrefix(target, "/settings/outsource"):
		return "设置 / 代加工模板设置", "维护代加工模板"
	case strings.Contains(target, "/sales-order-images"):
		return "订单销售 / 销售单", "生成销售单图片"
	case strings.Contains(target, "/sales-order-preview") || strings.Contains(target, "/sales-orders"):
		return "订单销售 / 销售单", "生成销售单PDF"
	case strings.Contains(target, "/orders"):
		return "订单销售 / 订单列表", "查看订单"
	case strings.Contains(target, "/order"):
		return "订单销售 / 录单", "录入订单"
	case strings.Contains(target, "/customers"):
		return "订单销售 / 客户档案", "维护客户档案"
	case strings.HasPrefix(target, "/api/company/profile"):
		if method == "POST" {
			return "设置 / 公司设置", "保存公司设置"
		}
		return "设置 / 公司设置", "查看公司设置"
	case strings.HasPrefix(target, "/api/company/departments") || strings.Contains(target, "/company/departments"):
		return "系统 / 部门维护", "维护部门"
	case strings.HasPrefix(target, "/api/company/employees") || strings.Contains(target, "/company/employees"):
		return "系统 / 员工维护", "维护员工"
	case strings.HasPrefix(target, "/api/auth/password/reset") || strings.Contains(target, "/password/reset"):
		return "系统 / 员工维护", "重置员工密码"
	case strings.Contains(target, "/api/auth/login/accounts") || strings.Contains(target, "/account-state"):
		return "系统 / 员工维护", "修改员工账号状态"
	case strings.Contains(target, "/api/auth/me/roles") || strings.Contains(target, "/api/auth"):
		return "系统 / 员工维护", "检查用户权限"
	case strings.Contains(target, "/produce/quality-inspections"):
		if method == "POST" {
			return "生产管理 / 生产质检", "保存质检记录"
		}
		return "生产管理 / 生产质检", "查看质检记录"
	case strings.Contains(target, "/produce/work-orders"):
		return "生产管理 / 生产工单", "维护生产工单"
	case strings.Contains(target, "/produce/job-cards"):
		return "生产管理 / 工序卡", "维护工序卡"
	case strings.Contains(target, "/produce/costs"):
		return "生产管理 / 生产成本", "查看生产成本"
	case strings.Contains(target, "/produce/running"):
		return "生产管理 / 生产中", "处理生产中任务"
	case strings.Contains(target, "/produce/wip-reservations"):
		return "生产管理 / 生产中", "处理WIP占用"
	case strings.Contains(target, "/produce/logs"):
		return "生产管理 / 生产日志", "查看生产日志"
	case strings.Contains(target, "/produce/machines"):
		return "设置 / 设备产能配置", "维护设备产能"
	case strings.Contains(target, "/produce/allocations"):
		return "生产管理 / 分配批次查看", "查看生产分配"
	case strings.Contains(target, "/produce"):
		return "生产管理 / 生产计划/开始生产", "处理生产计划"
	case strings.Contains(target, "/req/"):
		return operationRequirementMenu(target), "维护需求记录"
	default:
		return "其他 / 未分类", "访问系统页面"
	}
}

func operationTarget(route, path string) string {
	target := cleanOperationPath(route)
	if target == "" {
		target = cleanOperationPath(path)
	}
	return target
}

func cleanOperationPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "/api/"); idx >= 0 {
		return path[idx:]
	}
	if idx := strings.Index(path, "/public/"); idx >= 0 {
		return path[idx:]
	}
	if idx := strings.Index(path, "/vue-shell"); idx >= 0 {
		return path[idx:]
	}
	return path
}

func operationRequirementMenu(target string) string {
	switch {
	case strings.Contains(target, "req/product"):
		return "需求管理 / 产品需求表"
	case strings.Contains(target, "req/dev"):
		return "需求管理 / 开发需求表"
	case strings.Contains(target, "req/unit"):
		return "需求管理 / 单元测试表"
	case strings.Contains(target, "req/api"):
		return "需求管理 / API 测试表"
	case strings.Contains(target, "req/review"):
		return "需求管理 / 需求审核表"
	default:
		return "需求管理 / 产品需求表"
	}
}

func operationMeta(meta *string) (method, route, path string) {
	if meta == nil || strings.TrimSpace(*meta) == "" {
		return "", "", ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*meta), &m); err != nil {
		return "", "", ""
	}
	if v, ok := m["method"].(string); ok {
		method = strings.ToUpper(strings.TrimSpace(v))
	}
	if v, ok := m["route"].(string); ok {
		route = strings.TrimSpace(v)
	}
	if v, ok := m["path"].(string); ok {
		path = strings.TrimSpace(v)
	}
	return method, route, path
}

func splitOperationField(field string) (method, path string) {
	parts := strings.Fields(strings.TrimSpace(field))
	if len(parts) >= 2 {
		return strings.ToUpper(parts[0]), parts[1]
	}
	return "", strings.TrimSpace(field)
}

func auditSummary(r *AuditLogRow, rawEntityType, rawAction, rawField string) string {
	actor := strings.TrimSpace(r.Actor)
	if actor == "" {
		actor = "unknown"
	}
	menuName := leafMenuName(r.Menu)
	switch rawAction {
	case "request":
		status := ""
		if r.NewValue != nil && strings.TrimSpace(*r.NewValue) != "" {
			status = strings.TrimSpace(*r.NewValue)
		}
		result := "请求"
		if strings.HasPrefix(status, "2") || strings.HasPrefix(status, "3") {
			result = "请求成功"
		} else if status != "" {
			result = "请求返回 " + status
		}
		return fmt.Sprintf("%s 在%s%s，%s", actor, menuName, r.Feature, result)
	case "update":
		target := auditTargetName(r, rawEntityType)
		field := labelField(rawField)
		oldValue := ptrText(r.OldValue)
		newValue := ptrText(r.NewValue)
		return fmt.Sprintf("%s 在%s修改了%s 的%s：%s -> %s", actor, menuName, target, field, oldValue, newValue)
	case "create":
		return fmt.Sprintf("%s 在%s新增了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "delete":
		return fmt.Sprintf("%s 在%s删除了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "submit":
		return fmt.Sprintf("%s 在%s提交了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "adjust":
		target := auditTargetName(r, rawEntityType)
		field := labelField(rawField)
		oldValue := ptrText(r.OldValue)
		newValue := ptrText(r.NewValue)
		if rawField != "" {
			return fmt.Sprintf("%s 在%s调整了%s 的%s：%s -> %s", actor, menuName, target, field, oldValue, newValue)
		}
		return fmt.Sprintf("%s 在%s调整了%s", actor, menuName, target)
	case "release":
		return fmt.Sprintf("%s 在%s释放了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "publish":
		return fmt.Sprintf("%s 在%s发布了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "save_draft":
		return fmt.Sprintf("%s 在%s保存了%s草稿", actor, menuName, auditTargetName(r, rawEntityType))
	case "withdraw":
		return fmt.Sprintf("%s 在%s撤回了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "move":
		return fmt.Sprintf("%s 在%s移动了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "finish":
		return fmt.Sprintf("%s 在%s完成了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "partial_finish":
		return fmt.Sprintf("%s 在%s部分完成了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "cancel":
		return fmt.Sprintf("%s 在%s取消了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "login":
		return fmt.Sprintf("%s 完成员工登录", actor)
	case "upload":
		return fmt.Sprintf("%s 在%s上传了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "set_login_enabled":
		return fmt.Sprintf("%s 在%s修改了%s启用状态", actor, menuName, auditTargetName(r, rawEntityType))
	case "reset_password":
		return fmt.Sprintf("%s 在%s重置了%s密码", actor, menuName, auditTargetName(r, rawEntityType))
	default:
		return fmt.Sprintf("%s 在%s执行了%s", actor, menuName, r.Feature)
	}
}

func auditTargetName(r *AuditLogRow, rawEntityType string) string {
	name := labelEntityType(rawEntityType)
	if r.EntityLabel != nil && strings.TrimSpace(*r.EntityLabel) != "" {
		label := strings.TrimSpace(*r.EntityLabel)
		if label == name || strings.HasPrefix(label, name+" ") {
			return label
		}
		return name + " " + label
	}
	if hint := auditTargetHint(r, rawEntityType); hint != "" {
		return name + " " + hint
	}
	if r.EntityID != nil {
		return fmt.Sprintf("%s %d", name, *r.EntityID)
	}
	return name
}

func auditTargetHint(r *AuditLogRow, rawEntityType string) string {
	meta := auditMetaMap(r.Meta)
	switch rawEntityType {
	case "order":
		return firstMetaText(meta, "order_no", "order_id")
	case "company_profile":
		return firstNonEmpty(firstMetaText(meta, "company_name", "name"), valueForField(r, "company_name"))
	case "customer", "product", "material":
		return firstMetaText(meta, "name", "code")
	case "customer_asset", "sales_order_asset":
		return firstNonEmpty(firstMetaText(meta, "kind", "asset_id"), valueForField(r, "kind"))
	case "sales_order_payment_code":
		return firstNonEmpty(firstMetaText(meta, "label", "asset_id"), valueForField(r, "label"))
	case "sales_order_document", "sales_order_image":
		orderNo := firstMetaText(meta, "order_no", "order_id")
		version := firstNonEmpty(firstMetaText(meta, "version_no", "version"), valueForField(r, "version_no"))
		if orderNo != "" && version != "" {
			return orderNo + " v" + version
		}
		return firstNonEmpty(orderNo, version)
	case "sales_order_settings":
		return firstMetaText(meta, "company_name", "seal_asset_id", "asset_id")
	case "material_receipt":
		return firstMetaText(meta, "batch_code", "material_id")
	case "material_transfer", "finished_product_transfer":
		return firstMetaText(meta, "transfer_no", "material_id", "product_id")
	case "finished_inventory":
		productID := firstMetaText(meta, "product_id")
		specG := firstMetaText(meta, "spec_g")
		if productID != "" && specG != "" {
			return "产品 " + productID + " " + specG + "g"
		}
		return firstNonEmpty(productID, specG)
	case "stock_adjustment":
		return firstMetaText(meta, "batch_code", "material_batch_id", "item_name", "item_id")
	case "cost_parameter":
		return firstNonEmpty(firstMetaText(meta, "label", "key"), labelField(fieldText(r.Field)))
	case "bean_list_publication":
		listType := labelBeanListType(firstMetaText(meta, "list_type"))
		version := firstMetaText(meta, "version_no", "version", "source_version_no")
		if listType != "" && version != "" {
			return listType + " v" + version
		}
		return firstNonEmpty(listType, version)
	case "costing_run":
		return firstMetaText(meta, "run_id")
	case "produce_batch":
		return firstNonEmpty(firstMetaText(meta, "batch_id", "batch_code"), valueForField(r, "batch_id"))
	case "produce_running":
		return firstMetaText(meta, "work_order_no", "running_item_id", "batch_id", "product_id")
	case "wip_reservation":
		return firstMetaText(meta, "work_order_no", "note", "running_item_id", "material_id")
	case "product_category":
		return firstNonEmpty(firstMetaText(meta, "name", "category"), valueForField(r, "category"))
	case "auth_account":
		return firstMetaText(meta, "employee_id")
	}
	return ""
}

func valueForField(r *AuditLogRow, field string) string {
	if r.Field == nil {
		return ""
	}
	currentField := strings.TrimSpace(*r.Field)
	if currentField != field && currentField != labelField(field) {
		return ""
	}
	if r.NewValue != nil && strings.TrimSpace(*r.NewValue) != "" {
		return strings.TrimSpace(*r.NewValue)
	}
	if r.OldValue != nil && strings.TrimSpace(*r.OldValue) != "" {
		return strings.TrimSpace(*r.OldValue)
	}
	return ""
}

func fieldText(field *string) string {
	if field == nil {
		return ""
	}
	return strings.TrimSpace(*field)
}

func auditMetaMap(meta *string) map[string]any {
	if meta == nil || strings.TrimSpace(*meta) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*meta), &m); err != nil {
		return nil
	}
	return m
}

func firstMetaText(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := meta[key]; ok {
			if text := metaValueText(v); text != "" {
				return text
			}
		}
	}
	return ""
}

func metaValueText(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", x), "0"), ".")
	case bool:
		if x {
			return "是"
		}
		return "否"
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func labelBeanListType(t string) string {
	switch strings.TrimSpace(t) {
	case "commercial":
		return "商用豆单"
	case "retail":
		return "零售豆单"
	case "drip":
		return "挂耳豆单"
	default:
		return strings.TrimSpace(t)
	}
}

func leafMenuName(menu string) string {
	parts := strings.Split(menu, " / ")
	if len(parts) == 0 {
		return strings.TrimSpace(menu)
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func ptrText(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "空"
	}
	return strings.TrimSpace(*v)
}

func labelEntityType(t string) string {
	switch strings.TrimSpace(t) {
	case "order":
		return "订单"
	case "product":
		return "产品"
	case "product_category":
		return "产品分类"
	case "material":
		return "物料"
	case "customer":
		return "客户"
	case "customer_asset":
		return "客户附件"
	case "company_profile":
		return "公司信息"
	case "sales_order_settings":
		return "销售单设置"
	case "sales_order_asset":
		return "销售单素材"
	case "sales_order_payment_code":
		return "收款二维码"
	case "sales_order_document":
		return "销售单文件"
	case "sales_order_image":
		return "销售单图片"
	case "material_receipt":
		return "原料入库单"
	case "material_transfer":
		return "原料转仓单"
	case "finished_product_transfer":
		return "成品转仓单"
	case "finished_inventory":
		return "成品库存"
	case "stock_adjustment":
		return "库存调整单"
	case "cost_parameter":
		return "成本参数"
	case "bean_list_publication":
		return "豆单发布"
	case "costing_run":
		return "成本试算"
	case "auth":
		return "登录"
	case "auth_account":
		return "员工账号"
	case "produce_batch":
		return "生产批次"
	case "produce_running":
		return "生产任务"
	case "wip_reservation":
		return "WIP占用"
	case "operation":
		return "操作"
	case "import":
		return "导入"
	case "system":
		return "系统"
	default:
		return t
	}
}

func labelAction(a string) string {
	switch strings.TrimSpace(a) {
	case "update":
		return "修改"
	case "create":
		return "新增"
	case "delete":
		return "删除"
	case "submit":
		return "提交"
	case "adjust":
		return "调整"
	case "release":
		return "释放"
	case "publish":
		return "发布"
	case "save_draft":
		return "保存草稿"
	case "withdraw":
		return "撤回"
	case "move":
		return "移动"
	case "void":
		return "作废"
	case "unvoid":
		return "恢复"
	case "import":
		return "导入"
	case "request":
		return "访问/操作"
	case "login":
		return "登录"
	case "upload":
		return "上传"
	case "finish":
		return "完成"
	case "partial_finish":
		return "部分完成"
	case "cancel":
		return "取消"
	case "set_login_enabled":
		return "启用/停用账号"
	case "reset_password":
		return "重置密码"
	default:
		return a
	}
}

func labelField(f string) string {
	switch strings.TrimSpace(f) {
	case "pay_status_id":
		return "收款状态"
	case "ship_status_id":
		return "发货状态"
	case "notes":
		return "备注"
	case "created":
		return "创建"
	case "header":
		return "订单头"
	case "order":
		return "订单"
	case "price_tiers":
		return "价格阶梯"
	case "batch_id":
		return "批次号"
	case "deduct_status":
		return "扣减状态"
	case "material_consumption":
		return "物料消耗"
	case "finished_allocation":
		return "成品预扣"
	case "code":
		return "编码"
	case "name":
		return "名称"
	case "company_name":
		return "公司名称"
	case "kind":
		return "类型"
	case "unit":
		return "单位"
	case "purchase_price":
		return "进货价"
	case "sale_price":
		return "销售价"
	case "onhand_g":
		return "库存(g)"
	case "onhand_units":
		return "库存(个)"
	case "min_level_g":
		return "警戒线(g)"
	case "min_level_units":
		return "警戒线(个)"
	case "order_type_id":
		return "订单类型"
	case "process_status_id":
		return "处理状态"
	case "default_source_id":
		return "默认来源"
	case "default_order_type_id":
		return "默认订单类型"
	case "active":
		return "启用状态"
	case "settings":
		return "设置"
	case "asset_id":
		return "素材ID"
	case "seal_asset_id":
		return "公章素材"
	case "label":
		return "标签"
	case "status":
		return "状态"
	case "version_no":
		return "版本号"
	case "quantity":
		return "数量"
	case "qty_g":
		return "数量(g)"
	case "unit_cost":
		return "批次成本/kg"
	case "reserved_g":
		return "占用量(g)"
	case "work_order_no":
		return "生产工单号"
	case "product_count":
		return "产品数量"
	case "product_basics":
		return "产品基础信息"
	case "product_category":
		return "产品分类"
	case "category":
		return "分类"
	case "parent_position":
		return "分类位置"
	case "seal_x_mm":
		return "公章横向位置"
	case "seal_y_mm":
		return "公章纵向位置"
	case "seal_width_mm":
		return "公章宽度"
	case "roast_yield_rate":
		return "烘焙得率"
	case "kg_to_lb_factor":
		return "公斤转磅系数"
	case "small_batch_production_cost_per_kg":
		return "小批量生产成本/kg"
	case "large_batch_production_cost_per_kg":
		return "大批量生产成本/kg"
	case "wholesale_package_cost_per_kg":
		return "批发包装成本/kg"
	case "product_loss_per_kg":
		return "产品损耗/kg"
	case "retail_bean_margin_rate":
		return "零售熟豆利润系数"
	case "retail_tax_rate":
		return "零售税费率"
	case "retail_logistics_per_kg":
		return "零售物流/kg"
	case "retail_drip_logistics_per_10_bags":
		return "零售挂耳物流/10袋"
	case "drip_green_ratio_kg_per_bag":
		return "挂耳熟豆用量/袋"
	case "drip_process_cost_per_bag":
		return "挂耳加工成本/袋"
	case "drip_extra_cost_per_bag":
		return "挂耳额外成本/袋"
	case "drip_packing_material_per_bag":
		return "挂耳包装材料/袋"
	case "retail_drip_multiplier":
		return "零售挂耳利润系数"
	case "wholesale_kg_margin_rate_1":
		return "商用熟豆利润系数1"
	case "wholesale_kg_margin_rate_2":
		return "商用熟豆利润系数2"
	case "wholesale_kg_margin_rate_3":
		return "商用熟豆利润系数3"
	case "wholesale_kg_margin_rate_4":
		return "商用熟豆利润系数4"
	case "wholesale_kg_margin_rate_5":
		return "商用熟豆利润系数5"
	case "wholesale_kg_margin_rate_6":
		return "商用熟豆利润系数6"
	case "wholesale_drip_multiplier_1":
		return "商用挂耳利润系数1"
	case "wholesale_drip_multiplier_2":
		return "商用挂耳利润系数2"
	case "wholesale_drip_multiplier_3":
		return "商用挂耳利润系数3"
	case "wholesale_drip_multiplier_4":
		return "商用挂耳利润系数4"
	default:
		return f
	}
}

// NOTE: fetchIDNameMap + idTextToLabel are defined in audit_fetch.go and shared.
