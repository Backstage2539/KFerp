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

func fetchAuditPage(ctx context.Context, pool *pgxpool.Pool, schema string, from, to, q, entityType string, limit int) ([]AuditLogRow, error) {
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
	args = append(args, limit)
	limitArg := arg

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
		LIMIT $%d
	`, schema, schema, schema, schema, schema, where, limitArg)

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AuditLogRow, 0)
	for rows.Next() {
		var r AuditLogRow
		if err := rows.Scan(&r.Ts, &r.Actor, &r.EntityType, &r.EntityID, &r.EntityLabel, &r.EntityURL, &r.Action, &r.Field, &r.OldValue, &r.NewValue, &r.Meta); err != nil {
			return nil, err
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
	return out, rows.Err()
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
	r.Summary = auditSummary(r, rawEntityType, rawAction, rawField)
}

func auditMenuFeature(entityType, action, field string, meta *string) (string, string) {
	if entityType == "operation" {
		return operationMenuFeature(meta, field)
	}
	switch entityType {
	case "order":
		switch action {
		case "void":
			return "订单 / 订单列表", "作废订单"
		case "unvoid":
			return "订单 / 订单列表", "恢复订单"
		case "create":
			return "订单 / 录单", "新建订单"
		default:
			return "订单 / 订单列表", "编辑订单"
		}
	case "product":
		return "档案 / 商品档案", "编辑商品档案"
	case "material":
		return "物料管理 / 物料档案/库存", "编辑物料档案"
	case "customer", "customer_asset":
		if entityType == "customer_asset" {
			return "档案 / 客户档案", "维护客户附件"
		}
		return "档案 / 客户档案", "编辑客户档案"
	case "produce_batch":
		return "生产流程 / 生产计划/开始生产", "创建生产批次"
	case "produce_running":
		if action == "cancel" {
			return "生产流程 / 生产中", "取消生产"
		}
		return "生产流程 / 生产中", "完成生产"
	case "auth":
		return "移动端 / 登录", "员工登录"
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
	target := route
	if target == "" {
		target = path
	}
	switch {
	case strings.HasPrefix(target, "/api/materials/") || strings.HasPrefix(target, "/api/materials/:id"):
		if method == "POST" {
			return "物料管理 / 物料档案/库存", "保存物料行内编辑"
		}
		return "物料管理 / 物料档案/库存", "读取物料档案"
	case strings.HasPrefix(target, "/api/materials"):
		return "物料管理 / 物料档案/库存", "加载物料列表"
	case strings.Contains(target, "/materials"):
		return "物料管理 / 物料档案/库存", "打开物料档案/库存"
	case strings.Contains(target, "/audit"):
		return "日志 / 操作日志", "查看操作日志"
	case strings.Contains(target, "/orders"):
		return "订单 / 订单列表", "查看订单"
	case strings.Contains(target, "/order"):
		return "订单 / 录单", "录入订单"
	case strings.Contains(target, "/products"):
		return "档案 / 商品档案", "维护商品档案"
	case strings.Contains(target, "/customers"):
		return "档案 / 客户档案", "维护客户档案"
	case strings.Contains(target, "/produce/running"):
		return "生产流程 / 生产中", "处理生产中任务"
	case strings.Contains(target, "/produce"):
		return "生产流程 / 生产计划/开始生产", "处理生产计划"
	case strings.Contains(target, "/bom"):
		return "物料管理 / BOM配方维护", "维护BOM配方"
	case strings.Contains(target, "/req/"):
		return "需求管理 / 需求表", "维护需求记录"
	default:
		return "其他 / 未分类", "访问系统页面"
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
	case "finish":
		return fmt.Sprintf("%s 在%s完成了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "cancel":
		return fmt.Sprintf("%s 在%s取消了%s", actor, menuName, auditTargetName(r, rawEntityType))
	case "login":
		return fmt.Sprintf("%s 完成员工登录", actor)
	default:
		return fmt.Sprintf("%s 在%s执行了%s", actor, menuName, r.Feature)
	}
}

func auditTargetName(r *AuditLogRow, rawEntityType string) string {
	name := labelEntityType(rawEntityType)
	if r.EntityLabel != nil && strings.TrimSpace(*r.EntityLabel) != "" {
		return name + " " + strings.TrimSpace(*r.EntityLabel)
	}
	if r.EntityID != nil {
		return fmt.Sprintf("%s %d", name, *r.EntityID)
	}
	return name
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
		return "商品"
	case "material":
		return "物料"
	case "customer":
		return "客户"
	case "customer_asset":
		return "客户附件"
	case "auth":
		return "登录"
	case "produce_batch":
		return "生产批次"
	case "produce_running":
		return "生产任务"
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
	case "cancel":
		return "取消"
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
	default:
		return f
	}
}

// NOTE: fetchIDNameMap + idTextToLabel are defined in audit_fetch.go and shared.
