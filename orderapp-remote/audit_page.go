package main

import (
	"context"
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
				ELSE NULL
			END AS entity_label,
			CASE
				WHEN a.entity_type='order' THEN '/orders/' || a.entity_id::text
				WHEN a.entity_type='product' THEN '/products/' || a.entity_id::text
				ELSE NULL
			END AS entity_url,
			a.action, a.field, a.old_value, a.new_value,
			CASE WHEN a.meta IS NULL THEN NULL ELSE a.meta::text END AS meta
		FROM %s.audit_logs a
		LEFT JOIN %s.orders o ON a.entity_type='order' AND a.entity_id=o.id
		LEFT JOIN %s.products p ON a.entity_type='product' AND a.entity_id=p.id
		%s
		ORDER BY a.id DESC
		LIMIT $%d
	`, schema, schema, schema, where, limitArg)

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

		// translate for UI
		r.EntityType = labelEntityType(r.EntityType)
		r.Action = labelAction(r.Action)
		if r.Field != nil {
			lf := labelField(*r.Field)
			r.Field = &lf
		}
		// translate values for known fields
		if r.Field != nil {
			switch *r.Field {
			case "收款状态":
				r.OldValue = idTextToLabel(r.OldValue, payMap)
				r.NewValue = idTextToLabel(r.NewValue, payMap)
			case "发货状态":
				r.OldValue = idTextToLabel(r.OldValue, shipMap)
				r.NewValue = idTextToLabel(r.NewValue, shipMap)
			}
		}

		out = append(out, r)
	}
	return out, rows.Err()
}

func labelEntityType(t string) string {
	switch strings.TrimSpace(t) {
	case "order":
		return "订单"
	case "product":
		return "商品"
	case "customer":
		return "客户"
	case "customer_asset":
		return "客户附件"
	case "auth":
		return "登录"
	case "produce_batch":
		return "生产批次"
	case "produce_running":
		return "生产完成"
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
	default:
		return f
	}
}

// NOTE: fetchIDNameMap + idTextToLabel are defined in audit_fetch.go and shared.
