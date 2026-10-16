package samples

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	customers "orderapp/internal/application/customer"
	sales "orderapp/internal/application/sales"
	app "orderapp/internal/application/samples"
	customerpg "orderapp/internal/infrastructure/postgres/customer"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"strconv"
	"strings"
	"time"
)

func (r Repository) customerAllowed(ctx context.Context, q querier, p app.Principal, v app.Request, id int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customers c WHERE c.id=$1 AND c.active AND ($2 OR c.responsible_employee_id=$3 OR c.phone=$4 OR EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.mini_user_id=$5 AND b.status='approved')))`, r.Schema), id, p.Admin, p.EmployeeID, v.VerifiedPhone, v.MiniUserID).Scan(&ok)
	return ok, err
}
func (r Repository) Options(ctx context.Context, p app.Principal, id int64, search string) (app.Options, error) {
	out := app.Options{Customers: []app.Option{}, Employees: []app.Option{}, Specs: []app.SpecOption{}, Sources: []app.Option{}, OrderTypes: []app.Option{}, CustomerTypes: []string{"retail", "ecommerce", "wholesale", "channel"}}
	if !p.Staff() {
		return out, app.ErrForbidden
	}
	options := func(query string, args ...any) ([]app.Option, error) {
		rows, err := r.Pool.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := []app.Option{}
		for rows.Next() {
			var o app.Option
			if err = rows.Scan(&o.ID, &o.Name); err != nil {
				return nil, err
			}
			result = append(result, o)
		}
		return result, rows.Err()
	}
	var err error
	if p.Admin {
		out.Employees, err = options(fmt.Sprintf(`SELECT e.id,e.name FROM %[1]s.company_employees e WHERE e.active AND COALESCE(NULLIF(e.account_type,''),'internal_employee')='internal_employee' AND EXISTS(SELECT 1 FROM %[1]s.employee_roles er WHERE er.employee_id=e.id AND er.role_code IN('admin','sales')) AND NOT EXISTS(SELECT 1 FROM %[1]s.employee_login_passwords ep WHERE ep.employee_id=e.id AND ep.login_disabled) ORDER BY e.name`, r.Schema))
		if err != nil {
			return out, err
		}
	}
	if id == 0 {
		return out, nil
	}
	v, err := r.Get(ctx, p, id)
	if err != nil {
		return out, err
	}
	// Only a unique existing approved binding may be preselected automatically.
	err = r.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT CASE WHEN count(*)=1 THEN min(c.id) ELSE 0 END FROM %[1]s.customer_portal_user_bindings b JOIN %[1]s.customers c ON c.id=b.customer_id AND c.active WHERE b.mini_user_id=$1 AND b.status='approved'`, r.Schema), v.MiniUserID).Scan(&out.SuggestedCustomerID)
	if err != nil {
		return out, err
	}
	out.Customers, err = options(fmt.Sprintf(`SELECT c.id,c.name FROM %[1]s.customers c WHERE c.active AND ($1 OR c.responsible_employee_id=$2 OR c.phone=$3 OR EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.mini_user_id=$4 AND b.status='approved')) AND ($5='' OR c.name ILIKE '%%'||$5||'%%' OR c.phone ILIKE '%%'||$5||'%%') ORDER BY EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.mini_user_id=$4 AND b.status='approved') DESC,(c.phone=$3) DESC,c.id DESC LIMIT 100`, r.Schema), p.Admin, p.EmployeeID, v.VerifiedPhone, v.MiniUserID, strings.TrimSpace(search))
	if err != nil {
		return out, err
	}
	out.Sources, err = options("SELECT id,name FROM " + r.Schema + ".sources ORDER BY id")
	if err != nil {
		return out, err
	}
	out.OrderTypes, err = options("SELECT id,name FROM " + r.Schema + ".order_types ORDER BY id")
	if err != nil {
		return out, err
	}
	rows, err := r.Pool.Query(ctx, fmt.Sprintf(`SELECT spec.id,b.output_id,COALESCE(NULLIF(v.spec_name_snapshot,''),spec.name),COALESCE(NULLIF(v.inventory_unit,''),spec.inventory_unit) FROM %[1]s.production_bom_output_bindings b JOIN %[1]s.products p ON p.id=b.output_id AND p.active JOIN %[1]s.production_bom_versions version ON version.id=b.bom_version_id AND version.bom_id=b.bom_id AND version.status='published' JOIN %[1]s.production_bom_specs spec ON spec.bom_id=b.bom_id JOIN %[1]s.production_bom_version_variants v ON v.version_id=version.id AND v.bom_spec_id=spec.id WHERE b.output_type='product' AND b.is_default AND EXISTS(SELECT 1 FROM %[1]s.sample_request_items i WHERE i.request_id=$1 AND i.product_id=b.output_id) ORDER BY b.output_id,v.is_default DESC,v.sort_order,spec.id`, r.Schema), id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var o app.SpecOption
		if err = rows.Scan(&o.ID, &o.ProductID, &o.Name, &o.Unit); err != nil {
			return out, err
		}
		out.Specs = append(out.Specs, o)
	}
	return out, rows.Err()
}
func (r Repository) Confirm(ctx context.Context, p app.Principal, id int64, c app.ConfirmCommand) (app.Request, error) {
	if !p.Staff() {
		return app.Request{}, app.ErrForbidden
	}
	if err := app.ValidateConfirm(c); err != nil {
		return app.Request{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Request{}, err
	}
	defer tx.Rollback(ctx)
	v, err := r.lock(ctx, tx, p, id, -1)
	if err != nil {
		return v, err
	}
	if v.OrderID > 0 {
		return v, nil
	}
	if v.Revision != c.Revision {
		return v, app.ErrConflict
	}
	if v.WorkOrderID == 0 || v.AssigneeID <= 0 {
		return v, app.Invalid("请先分配工单")
	}
	if app.Terminal(v.FollowupStatus) {
		return v, app.Invalid("工单已结束，不能确认寄样")
	}
	allowed := map[int64]bool{}
	for _, i := range v.Items {
		allowed[i.ProductID] = true
	}
	items := []sales.OrderItemCommand{}
	for index, i := range c.Items {
		if !allowed[i.ProductID] {
			return v, app.Invalid("确认商品必须来自客户申请")
		}
		// Freeze a verified spec label and unit; the caller cannot supply these snapshots.
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(v.spec_name_snapshot,''),spec.name),COALESCE(NULLIF(v.inventory_unit,''),spec.inventory_unit) FROM %[1]s.production_bom_output_bindings b JOIN %[1]s.products product ON product.id=b.output_id AND product.active JOIN %[1]s.production_bom_versions version ON version.id=b.bom_version_id AND version.bom_id=b.bom_id AND version.status='published' JOIN %[1]s.production_bom_specs spec ON spec.bom_id=b.bom_id JOIN %[1]s.production_bom_version_variants v ON v.version_id=version.id AND v.bom_spec_id=spec.id WHERE b.output_type='product' AND b.is_default AND b.output_id=$1 AND spec.id=$2 FOR SHARE OF b,product,version,spec,v`, r.Schema), i.ProductID, i.BomSpecID).Scan(&c.Items[index].SpecName, &c.Items[index].Unit)
		if errors.Is(err, pgx.ErrNoRows) {
			return v, app.Invalid("商品没有所选的有效已发布BOM规格，请先配置样品规格")
		}
		if err != nil {
			return v, err
		}
		pid, price := i.ProductID, i.UnitPrice
		items = append(items, sales.OrderItemCommand{ProductID: &pid, ParentProductID: pid, BomSpecID: i.BomSpecID, Units: i.Qty, ManualPrice: &price, PriceSelectionMode: "manual", Note: "样品申请 " + v.Number})
	}
	customerID := c.CustomerID
	if customerID > 0 {
		ok, e := r.customerAllowed(ctx, tx, p, v, customerID)
		if e != nil {
			return v, e
		}
		if !ok {
			return v, app.Invalid("客户不可用或不在当前处理范围")
		}
	} else {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "sample-customer:"+v.VerifiedPhone); err != nil {
			return v, err
		}
		var existing bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customers WHERE active AND phone=$1) OR EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings b JOIN %[1]s.customers c ON c.id=b.customer_id AND c.active WHERE b.mini_user_id=$2 AND b.status='approved')`, r.Schema), v.VerifiedPhone, v.MiniUserID).Scan(&existing)
		if err != nil {
			return v, err
		}
		if existing {
			return v, app.Invalid("已有匹配客户，请刷新候选并选择，避免重复建档")
		}
		n := c.NewCustomer
		disabled := false
		svc := customers.NewService(customerpg.NewRepository(r.Pool, r.Schema, "").WithUpsertTransaction(tx))
		customerID, err = svc.Upsert(ctx, p.Actor, nil, customers.UpsertCommand{Name: n.Name, CustomerType: n.CustomerType, Contact: c.Receiver.Name, Phone: v.VerifiedPhone, Address: c.Receiver.Address, DefaultSourceID: strconv.FormatInt(n.SourceID, 10), DefaultOrderTypeID: strconv.FormatInt(n.OrderTypeID, 10), ResponsibleEmployeeID: strconv.FormatInt(v.AssigneeID, 10), Active: "true", PortalEnabled: &disabled})
		if err != nil {
			return v, err
		}
	}
	var unpaid int64
	err = tx.QueryRow(ctx, "SELECT id FROM "+r.Schema+".pay_statuses WHERE name IN('未付款','未收款') ORDER BY id LIMIT 1").Scan(&unpaid)
	if err != nil {
		return v, app.Invalid("请先配置未付款状态")
	}
	now := time.Now()
	orderService := sales.NewService(salespg.NewRepository(r.Pool, r.Schema).WithSampleOrderTransaction(tx))
	result, err := orderService.SaveOrder(ctx, sales.SaveOrderCommand{Actor: p.Actor, CustomerID: customerID, OrderDate: now, DocumentDate: now, Items: items, ShippingAmount: c.ShippingAmount, PayStatusID: unpaid, ReceiverName: c.Receiver.Name, ReceiverPhone: c.Receiver.Phone, ReceiverAddress: c.Receiver.Address, Notes: "样品申请 " + v.Number + "；" + strings.TrimSpace(c.Note), AllowResponsibleEmployeeFallback: true, FallbackResponsibleEmployeeID: v.AssigneeID, FallbackResponsibleEmployeeName: v.AssigneeName})
	if err != nil {
		return v, err
	}
	confirmed, _ := json.Marshal(c)
	_, err = tx.Exec(ctx, "UPDATE "+r.Schema+".sample_work_orders SET customer_id=$2,order_id=$3,status='ordered',followup_status='following',next_reminder_at=$4,revision=revision+1,confirmed_json=$5,updated_at=now() WHERE request_id=$1", id, customerID, result.OrderID, now.Add(7*24*time.Hour), confirmed)
	if err != nil {
		return v, err
	}
	if err = r.activity(ctx, tx, id, "confirm", "following", "已联系客户并确认寄样；订单 "+result.OrderNo, p.Actor); err != nil {
		return v, err
	}
	if err = r.notify(ctx, tx, id, fmt.Sprintf("sample:%d:ordered", id), "sample.ordered", "样品订单已生成 "+result.OrderNo, v.AssigneeID); err != nil {
		return v, err
	}
	if err = tx.Commit(ctx); err != nil {
		return v, err
	}
	return r.Get(ctx, p, id)
}

// ErrorMessage exposes only known business validation failures to the HTTP adapter.
func ErrorMessage(err error) (string, bool) {
	var v *app.ValidationError
	if errors.As(err, &v) {
		return v.Error(), true
	}
	if message, ok := customers.MaintenanceValidationMessage(err); ok {
		return message, true
	}
	for _, key := range []string{"BOM spec is not published", "product_bom_spec_not_configured", "客户资料缺少", "库存", "规格", "客户", "金额", "价格", "来源", "订单类型", "负责人", "manual price", "product required"} {
		if strings.Contains(err.Error(), key) {
			return err.Error(), true
		}
	}
	return "", false
}
