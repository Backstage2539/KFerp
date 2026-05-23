package sales

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (r Repository) ListOrders(ctx context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	rows, hasNext, err := fetchOrders(ctx, r.pool, r.schema, query)
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	summary, err := fetchOrdersSummary(ctx, r.pool, r.schema, query)
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	orderTypes, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".order_types ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	payStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".pay_statuses ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	shipStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".ship_statuses ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	processStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".order_process_statuses WHERE active=true ORDER BY sort,id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	return salesapp.OrderListResult{
		Rows:            rows,
		Summary:         summary,
		OrderTypes:      orderTypes,
		PayStatuses:     payStatuses,
		ShipStatuses:    shipStatuses,
		ProcessStatuses: processStatuses,
		HasNext:         hasNext,
	}, nil
}

func (r Repository) ListOrderAuditLogs(ctx context.Context, orderID int64, limit int) ([]salesapp.AuditRow, error) {
	if limit <= 0 {
		limit = 50
	}

	payMap, _ := fetchOrderIDNameMap(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses", r.schema))
	shipMap, _ := fetchOrderIDNameMap(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses", r.schema))

	sql := fmt.Sprintf(`
		SELECT
			to_char(changed_at,'YYYY-MM-DD HH24:MI:SS') AS changed_at,
			actor, field, old_value, new_value
		FROM %s.order_audit_logs
		WHERE order_id=$1
		ORDER BY id DESC
		LIMIT $2
	`, r.schema)
	rows, err := r.pool.Query(ctx, sql, orderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.AuditRow, 0)
	for rows.Next() {
		var row salesapp.AuditRow
		if err := rows.Scan(&row.ChangedAt, &row.Actor, &row.Field, &row.OldValue, &row.NewValue); err != nil {
			return nil, err
		}
		switch row.Field {
		case "pay_status_id":
			row.OldValue = auditIDTextToLabel(row.OldValue, payMap)
			row.NewValue = auditIDTextToLabel(row.NewValue, payMap)
		case "ship_status_id":
			row.OldValue = auditIDTextToLabel(row.OldValue, shipMap)
			row.NewValue = auditIDTextToLabel(row.NewValue, shipMap)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func fetchOrders(ctx context.Context, pool *pgxpool.Pool, schema string, query salesapp.OrderListQuery) ([]salesapp.OrderRow, bool, error) {
	where, args, nextArg := orderListWhere(schema, query)

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	args = append(args, query.Limit+1, query.Offset)
	limitArg := nextArg
	offsetArg := nextArg + 1

	sql := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(to_char(o.document_date, 'YYYY-MM-DD'), to_char(o.order_date, 'YYYY-MM-DD'), '') AS document_date,
			COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), '') AS order_date,
			COALESCE(o.customer_id,0) AS customer_id,
			COALESCE(c.name, '') AS customer,
			COALESCE(o.responsible_party_type, '') AS responsible_party_type,
			COALESCE(o.responsible_party_id, 0) AS responsible_party_id,
			COALESCE(o.responsible_party_name, '') AS responsible_party_name,
			COALESCE(to_char(o.total_amount, 'FM999999999.00'), '') AS total_amount,
			COALESCE(to_char(o.shipping_amount, 'FM999999999.00'), '') AS shipping_amount,
			COALESCE(to_char(o.discount_amount, 'FM999999999.00'), '') AS discount_amount,
			COALESCE(to_char(o.grand_total, 'FM999999999.00'), '') AS grand_total,
			COALESCE(o.express_fee, '') AS express_fee,
			COALESCE(to_char(o.outsource_material_fee, 'FM999999999.00'), '') AS outsource_material_fee,
			COALESCE(to_char(o.outsource_roast_fee, 'FM999999999.00'), '') AS outsource_roast_fee,
			COALESCE(to_char(o.outsource_packaging_fee, 'FM999999999.00'), '') AS outsource_packaging_fee,
			COALESCE(to_char(o.outsource_manual_fee, 'FM999999999.00'), '') AS outsource_manual_fee,
			COALESCE(to_char(o.outsource_tax_fee, 'FM999999999.00'), '') AS outsource_tax_fee,
			COALESCE(to_char(o.outsource_other_fee, 'FM999999999.00'), '') AS outsource_other_fee,
			COALESCE(to_char(o.outsource_total_fee, 'FM999999999.00'), '') AS outsource_total_fee,
			COALESCE(ot.name, '') AS order_type,
			COALESCE(ps.name, '') AS pay_status,
			COALESCE(o.payment_method, '') AS payment_method,
			COALESCE(ss.name, '') AS ship_status,
			%s AS ship_tracking_no,
			COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, '') AS receiver_name,
			COALESCE(NULLIF(o.receiver_phone,''), c.phone, '') AS receiver_phone,
			COALESCE(NULLIF(o.receiver_address,''), c.address, '') AS receiver_address,
			COALESCE(o.receiver_company, '') AS receiver_company,
			COALESCE(o.portal_service_code,'') AS portal_service_code,
			COALESCE(o.source_warehouse,'') AS source_warehouse,
			COALESCE(NULLIF(ship_sender.sender_id,0), NULLIF(o.sender_id,0), 0) AS sender_id,
			COALESCE(sender.sender_label, '') AS sender_label,
			COALESCE(sender.sender_name, '') AS sender_name,
			COALESCE(ops.name, '') AS process_status,
			COALESCE((
				SELECT string_agg(DISTINCT COALESCE(NULLIF(oi_kind.product_kind,''), NULLIF(p_kind.product_kind,''), 'roasted'), ',' ORDER BY COALESCE(NULLIF(oi_kind.product_kind,''), NULLIF(p_kind.product_kind,''), 'roasted'))
				FROM %s.order_items oi_kind
				LEFT JOIN %s.products p_kind ON p_kind.id=oi_kind.product_id
				WHERE oi_kind.order_id=o.id
			), '') AS product_kind_summary,
			COALESCE((SELECT al.actor FROM %s.order_audit_logs al WHERE al.order_id=o.id ORDER BY al.id ASC LIMIT 1), '未知') AS created_by_employee,
			COALESCE(o.order_type_id,0) AS order_type_id,
			COALESCE(o.pay_status_id,0) AS pay_status_id,
			COALESCE(o.ship_status_id,0) AS ship_status_id,
			COALESCE(o.process_status_id,0) AS process_status_id,
			COALESCE(o.notes,'') AS notes,
			o.is_void,
			COALESCE(oi.status,'') AS invoice_status,
			COALESCE(ia.filename,'') AS invoice_filename,
			COALESCE(ia.object_key,'') AS invoice_object_key
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		LEFT JOIN %s.order_types ot ON ot.id = o.order_type_id
		LEFT JOIN %s.pay_statuses ps ON ps.id = o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id = o.ship_status_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id = o.process_status_id
		LEFT JOIN %s.order_invoices oi ON oi.order_id = o.id
		LEFT JOIN %s.sales_order_assets ia ON ia.id = oi.invoice_asset_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(oso.sender_id, os.sender_id, 0) AS sender_id
			FROM %s.order_shipment_orders oso
			JOIN %s.order_shipments os ON os.id=oso.shipment_id
			WHERE oso.order_id=o.id
			ORDER BY os.created_at DESC, oso.id DESC
			LIMIT 1
		) ship_sender ON true
		LEFT JOIN %s.sender_settings sender ON sender.id=ship_sender.sender_id
		%s
		ORDER BY o.order_date DESC, o.id DESC
		LIMIT $%d OFFSET $%d
	`, orderTrackingSummaryExpr(schema, "o"), schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, wsql, limitArg, offsetArg)

	dbRows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer dbRows.Close()

	out := make([]salesapp.OrderRow, 0)
	for dbRows.Next() {
		var r salesapp.OrderRow
		var invoiceObjectKey string
		if err := dbRows.Scan(&r.ID, &r.OrderNo, &r.DocumentDate, &r.OrderDate, &r.CustomerID, &r.Customer, &r.ResponsibleType, &r.ResponsibleID, &r.ResponsibleName, &r.TotalAmount, &r.ShippingAmount, &r.DiscountAmount, &r.GrandTotal, &r.ExpressFee, &r.OutsourceMaterialFee, &r.OutsourceRoastFee, &r.OutsourcePackagingFee, &r.OutsourceManualFee, &r.OutsourceTaxFee, &r.OutsourceOtherFee, &r.OutsourceTotalFee, &r.OrderType, &r.PayStatus, &r.PaymentMethod, &r.ShipStatus, &r.ShipTrackingNo, &r.ReceiverName, &r.ReceiverPhone, &r.ReceiverAddress, &r.ReceiverCompany, &r.PortalServiceCode, &r.SourceWarehouse, &r.SenderID, &r.SenderLabel, &r.SenderName, &r.ProcessStatus, &r.ProductKindSummary, &r.CreatedByEmployee, &r.OrderTypeID, &r.PayStatusID, &r.ShipStatusID, &r.ProcessStatusID, &r.Notes, &r.IsVoid, &r.InvoiceStatus, &r.InvoiceFilename, &invoiceObjectKey); err != nil {
			return nil, false, err
		}
		r.InvoiceFileURL = salesOrderAssetURL(invoiceObjectKey)
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, false, err
	}

	hasNext := false
	if len(out) > query.Limit {
		hasNext = true
		out = out[:query.Limit]
	}
	return out, hasNext, nil
}

func fetchOrderIDNameMap(ctx context.Context, pool *pgxpool.Pool, sqlstr string) (map[int64]string, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		m[id] = name
	}
	return m, rows.Err()
}

func auditIDTextToLabel(v *string, labels map[int64]string) *string {
	if v == nil {
		return nil
	}
	id, err := strconv.ParseInt(strings.TrimSpace(*v), 10, 64)
	if err != nil || id <= 0 {
		return v
	}
	label, ok := labels[id]
	if !ok {
		return v
	}
	return &label
}

func fetchOrdersSummary(ctx context.Context, pool *pgxpool.Pool, schema string, query salesapp.OrderListQuery) (salesapp.OrdersSummary, error) {
	where, args, _ := orderListWhere(schema, query)
	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	sql := fmt.Sprintf(`
		SELECT count(*)::int AS orders,
		       count(distinct o.customer_id)::int AS customers
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		%s
	`, schema, schema, wsql)

	var s salesapp.OrdersSummary
	if err := pool.QueryRow(ctx, sql, args...).Scan(&s.Orders, &s.Customers); err != nil {
		return salesapp.OrdersSummary{}, err
	}
	return s, nil
}

func orderListWhere(schema string, query salesapp.OrderListQuery) ([]string, []any, int) {
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if query.OrderID > 0 {
		where = append(where, fmt.Sprintf("o.id = $%d", argn))
		args = append(args, query.OrderID)
		argn++
	}
	if q := strings.TrimSpace(query.Q); q != "" {
		where = append(where, fmt.Sprintf(`(o.order_no ILIKE $%d OR c.name ILIKE $%d OR o.responsible_party_name ILIKE $%d OR o.ship_tracking_no ILIKE $%d OR EXISTS (
			SELECT 1 FROM %s.order_shipping_trackings ost WHERE ost.order_id=o.id AND ost.tracking_no ILIKE $%d
		))`, argn, argn, argn, argn, schema, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	if query.CustomerID > 0 {
		where = append(where, fmt.Sprintf("o.customer_id = $%d", argn))
		args = append(args, query.CustomerID)
		argn++
	}
	switch strings.TrimSpace(query.Scope) {
	case "mine":
		if query.EmployeeID > 0 {
			where = append(where, fmt.Sprintf("o.responsible_party_type='employee' AND o.responsible_party_id=$%d", argn))
			args = append(args, query.EmployeeID)
			argn++
		} else {
			where = append(where, "1=0")
		}
	case "fulfillment":
		employeeBindingClause := ""
		if query.FulfillmentEmployeeID > 0 {
			employeeBindingClause = fmt.Sprintf("AND b.employee_id=$%d", argn)
			args = append(args, query.FulfillmentEmployeeID)
			argn++
		}
		where = append(where, fmt.Sprintf(`COALESCE(NULLIF(c.customer_type,''),'retail')='wholesale'
				AND o.portal_service_code IN ('direct_ship','processing_ship','product_order')
			AND EXISTS (
				SELECT 1 FROM %[1]s.customer_erp_user_bindings b
				JOIN %[1]s.company_employees e ON e.id=b.employee_id
				LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id
				LEFT JOIN %[1]s.customer_portal_profiles p ON p.customer_id=b.customer_id
				WHERE b.customer_id=o.customer_id AND b.status='active'
				  AND e.active=true
				  AND e.account_type='channel_customer'
				  AND COALESCE(lp.login_disabled,false)=false
				  %[2]s
				  AND (
				      (
				          COALESCE(NULLIF(p.capability_template_key,''),'processing_fulfillment') IN ('processing_fulfillment','public_sku_direct_ship')
				          AND NOT EXISTS (
				              SELECT 1 FROM %[1]s.customer_capability_templates inactive_template
				              WHERE inactive_template.template_key=COALESCE(NULLIF(p.capability_template_key,''),'processing_fulfillment')
				                AND inactive_template.active=false
				          )
				      )
				      OR EXISTS (
				          SELECT 1 FROM %[1]s.customer_capability_templates active_template
				          WHERE active_template.template_key=p.capability_template_key
				            AND active_template.active=true
				            AND (jsonb_array_length(active_template.erp_permissions)>0 OR jsonb_array_length(active_template.erp_view_keys)>0)
				      )
				  )
			)`, schema, employeeBindingClause))
	}
	if query.PayStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.pay_status_id,0) = $%d", argn))
		args = append(args, query.PayStatusID)
		argn++
	}
	if query.ShipStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.ship_status_id,0) = $%d", argn))
		args = append(args, query.ShipStatusID)
		argn++
	}
	if query.ProcessStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.process_status_id,0) = $%d", argn))
		args = append(args, query.ProcessStatusID)
		argn++
	}
	if query.UnproducedOnly {
		where = append(where, "COALESCE(o.process_status_id,0) IN (0,1,2)")
	}
	if query.CompletedOnly {
		where = append(where, "COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4)")
	}
	if query.ShipReadyOnly {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM %s.order_process_statuses ops WHERE ops.id=o.process_status_id AND ops.name IN ('生产完成','已生产完成','无需生产','库存待发货'))", schema))
	}
	if from := strings.TrimSpace(query.From); from != "" {
		where = append(where, fmt.Sprintf("o.order_date >= $%d", argn))
		args = append(args, from)
		argn++
	}
	if to := strings.TrimSpace(query.To); to != "" {
		where = append(where, fmt.Sprintf("o.order_date <= $%d", argn))
		args = append(args, to)
		argn++
	}

	switch strings.TrimSpace(query.Void) {
	case "void":
		where = append(where, "o.is_void = true")
	case "all":
	default:
		where = append(where, "o.is_void = false")
	}
	return where, args, argn
}

func fetchOrderOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]salesapp.Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.Option, 0)
	for rows.Next() {
		var o salesapp.Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
