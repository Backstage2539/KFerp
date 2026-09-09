package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"
)

func ensureOrderConfirmationSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_required BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_status TEXT NOT NULL DEFAULT 'accepted' CHECK(confirmation_status IN ('pending','accepted','rejected'));
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_revision BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_accepted_revision BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_pending JSONB;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_reason TEXT NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.order_content_versions (
   order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id), revision BIGINT NOT NULL,
   snapshot JSONB NOT NULL, status TEXT NOT NULL, actor TEXT NOT NULL,
   created_at TIMESTAMPTZ NOT NULL DEFAULT now(), reviewed_at TIMESTAMPTZ,
   reviewed_by TEXT NOT NULL DEFAULT '', reason TEXT NOT NULL DEFAULT '',
 request_id TEXT NOT NULL DEFAULT '', request_hash TEXT NOT NULL DEFAULT '', customer_id BIGINT NOT NULL DEFAULT 0,
   PRIMARY KEY(order_id,revision));
 CREATE UNIQUE INDEX IF NOT EXISTS order_content_request_unique ON %[1]s.order_content_versions(customer_id,request_id) WHERE request_id<>'';
 CREATE INDEX IF NOT EXISTS order_confirmation_pending_idx ON %[1]s.orders(confirmation_status,id) WHERE confirmation_required;
 `, schema))
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
 CREATE OR REPLACE FUNCTION %[1]s.require_order_accepted_execution() RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE linked_order RECORD; row_data JSONB; ids BIGINT; nos TEXT;
 BEGIN
   row_data:=to_jsonb(NEW); ids:=NULLIF(row_data->>'order_id','')::bigint;
   nos:=COALESCE(row_data->>'order_nos',row_data->>'order_no','');
   FOR linked_order IN SELECT id,confirmation_required,confirmation_status FROM %[1]s.orders
     WHERE id=ids OR order_no=ANY(string_to_array(replace(nos,' ',''),',')) ORDER BY id FOR UPDATE
   LOOP
     IF linked_order.confirmation_required AND linked_order.confirmation_status<>'accepted' THEN
       RAISE EXCEPTION '订单待确认或已拒绝，不能安排生产、占用库存或发货' USING ERRCODE='23514';
     END IF;
   END LOOP;
   RETURN NEW;
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.require_order_accepted_status() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
   IF OLD.confirmation_required AND OLD.confirmation_status<>'accepted'
     AND ((to_jsonb(NEW)->'process_status_id') IS DISTINCT FROM (to_jsonb(OLD)->'process_status_id')
       OR (to_jsonb(NEW)->'ship_status_id') IS DISTINCT FROM (to_jsonb(OLD)->'ship_status_id'))
   THEN RAISE EXCEPTION '请先确认订单再更新生产或发货状态' USING ERRCODE='23514'; END IF;
   RETURN NEW;
 END $fn$;
 DROP TRIGGER IF EXISTS require_order_accepted_status ON %[1]s.orders;
 CREATE TRIGGER require_order_accepted_status BEFORE UPDATE ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.require_order_accepted_status();
 DO $block$ DECLARE target TEXT; BEGIN
   FOREACH target IN ARRAY ARRAY['production_plan_items','work_orders','produce_running_items','produce_batch_order_items','customer_order_production_demands','order_stock_batch_allocations','order_stock_deductions','order_shipment_orders'] LOOP
     IF to_regclass('%[1]s.'||target) IS NOT NULL THEN
       EXECUTE format('DROP TRIGGER IF EXISTS require_order_accepted_execution ON %%I.%%I','%[1]s',target);
       EXECUTE format('CREATE TRIGGER require_order_accepted_execution BEFORE INSERT OR UPDATE ON %%I.%%I FOR EACH ROW EXECUTE FUNCTION %[1]s.require_order_accepted_execution()','%[1]s',target);
     END IF;
   END LOOP;
 END $block$;
 `, schema))
	if err != nil {
		return err
	}
	return ensureOrderShipmentProgressSchema(ctx, pool, schema)
}

type orderContentSnapshot struct {
	Order json.RawMessage `json:"order"`
	Items json.RawMessage `json:"items"`
}

func orderConfirmationRequestQuery(schema string) string {
	return fmt.Sprintf(`SELECT o.id,o.order_no,v.request_hash,o.confirmation_status FROM %[1]s.order_content_versions v JOIN %[1]s.orders o ON o.id=v.order_id WHERE v.customer_id=$1 AND v.request_id=$2`, schema)
}

func preserveCustomerOrderFinance(cmd *salesapp.SaveOrderCommand, raw json.RawMessage) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	fields := map[string]any{"pay_status_id": &cmd.PayStatusID, "ship_status_id": &cmd.ShipStatusID, "payment_method": &cmd.PaymentMethod, "shipping_amount": &cmd.ShippingAmount, "discount_amount": &cmd.DiscountAmount, "round_to_int": &cmd.RoundToInt, "express_fee": &cmd.ExpressFee, "payment_goods_amount": &cmd.PaymentGoodsAmount, "payment_shipping_amount": &cmd.PaymentShippingAmount, "payment_voucher_asset_id": &cmd.PaymentVoucherAssetID, "outsource_material_fee": &cmd.OutsourceMaterialFee, "outsource_roast_fee": &cmd.OutsourceRoastFee, "outsource_packaging_fee": &cmd.OutsourcePackagingFee, "outsource_manual_fee": &cmd.OutsourceManualFee, "outsource_tax_fee": &cmd.OutsourceTaxFee, "outsource_other_fee": &cmd.OutsourceOtherFee}
	for key, dest := range fields {
		if value := values[key]; len(value) > 0 {
			if err := json.Unmarshal(value, dest); err != nil {
				return err
			}
		}
	}
	cmd.PrepaymentAmount = nil
	return nil
}

func orderContentSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (orderContentSnapshot, error) {
	var s orderContentSnapshot
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT jsonb_object_agg(key,value) FROM jsonb_each(to_jsonb(o)) WHERE key NOT LIKE 'confirmation_%%'), COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM %[1]s.order_items i WHERE i.order_id=o.id),'[]') FROM %[1]s.orders o WHERE o.id=$1`, schema), id).Scan(&s.Order, &s.Items)
	return s, err
}

// Apply only business fields from a server-created snapshot. Identity, lifecycle
// and confirmation metadata stay on the same locked order.
func applyOrderContentSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, id int64, s orderContentSnapshot) error {
	var fields []string
	rows, err := tx.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema=$1 AND table_name='orders' AND column_name NOT LIKE 'confirmation_%' AND column_name NOT IN ('id','order_no','created_at','updated_at','is_void','voided_at','void_reason','customer_request_id','customer_request_hash') ORDER BY ordinal_position`, schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		fields = append(fields, pgx.Identifier{name}.Sanitize())
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	columns := strings.Join(fields, ",")
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders SET (%[2]s)=(SELECT %[2]s FROM jsonb_populate_record(NULL::%[1]s.orders,$2::jsonb)) WHERE id=$1`, schema, columns), id, s.Order)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_items WHERE order_id=$1`, schema), id); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_items SELECT * FROM jsonb_populate_recordset(NULL::%[1]s.order_items,$1::jsonb)`, schema), s.Items)
	return err
}

func (r Repository) stageOrderConfirmationTx(ctx context.Context, tx pgx.Tx, id int64, before *orderContentSnapshot, actor string) error {
	pending, err := orderContentSnapshotTx(ctx, tx, r.schema, id)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	var required bool
	var revision, accepted int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT confirmation_required,confirmation_revision,confirmation_accepted_revision FROM %s.orders WHERE id=$1`, r.schema), id).Scan(&required, &revision, &accepted); err != nil {
		return err
	}
	if before != nil {
		if !required {
			accepted = revision + 1
			revision = accepted
			original, _ := json.Marshal(before)
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_content_versions(order_id,revision,snapshot,status,actor) VALUES($1,$2,$3,'accepted',$4)`, r.schema), id, accepted, original, actor); err != nil {
				return err
			}
		}
		if err = applyOrderContentSnapshotTx(ctx, tx, r.schema, id, *before); err != nil {
			return err
		}
	} else {
		// A new pending order has an identity but no accepted receivable or items.
		var base map[string]any
		if err = json.Unmarshal(pending.Order, &base); err != nil {
			return err
		}
		for key := range base {
			if strings.HasSuffix(key, "_amount") || strings.HasSuffix(key, "_fee") || key == "grand_total" {
				if _, ok := base[key].(float64); ok {
					base[key] = float64(0)
				}
			}
		}
		base["grand_total"] = float64(0)
		base["total_amount"] = float64(0)
		rawBase, _ := json.Marshal(base)
		if err = applyOrderContentSnapshotTx(ctx, tx, r.schema, id, orderContentSnapshot{Order: rawBase, Items: json.RawMessage(`[]`)}); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.order_content_versions SET status='superseded' WHERE order_id=$1 AND status='pending'", r.schema), id); err != nil {
		return err
	}
	revision++
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending',confirmation_pending=$2,confirmation_revision=$3,confirmation_accepted_revision=$4,confirmation_reason='' WHERE id=$1`, r.schema), id, raw, revision, accepted); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_content_versions(order_id,revision,snapshot,status,actor) VALUES($1,$2,$3,'pending',$4)`, r.schema), id, revision, raw, actor); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &id, "save_pending", postgresinfra.StrPtr("confirmation_status"), nil, postgresinfra.StrPtr("pending"), postgresinfra.AuditMeta{"revision": revision})
}

func (r Repository) OrderConfirmation(ctx context.Context, id int64) (salesapp.OrderConfirmation, error) {
	return r.orderConfirmationRead(ctx, r.pool, id)
}
func (r Repository) orderConfirmationRead(ctx context.Context, queryer orderEditabilityQueryer, id int64) (salesapp.OrderConfirmation, error) {
	var result salesapp.OrderConfirmation
	err := queryer.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,o.order_no,COALESCE(o.customer_id,0),o.confirmation_required,o.confirmation_status,o.confirmation_revision,o.confirmation_accepted_revision,COALESCE(c.responsible_employee_id,0),o.confirmation_reason,COALESCE((SELECT jsonb_agg(jsonb_build_object('revision',v.revision,'status',v.status,'actor',v.actor,'created_at',v.created_at,'reviewed_at',v.reviewed_at,'reviewed_by',v.reviewed_by,'reason',v.reason,'order_date',v.snapshot->'order'->>'order_date','grand_total',v.snapshot->'order'->>'grand_total','items',(SELECT jsonb_agg(jsonb_build_object('item_name',i->>'item_name','spec',i->>'spec','qty',i->>'qty','unit',i->>'unit','unit_price',i->>'unit_price')) FROM jsonb_array_elements(v.snapshot->'items') i)) ORDER BY v.revision DESC) FROM %[1]s.order_content_versions v WHERE v.order_id=o.id),'[]') FROM %[1]s.orders o LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=$1`, r.schema), id).Scan(&result.OrderID, &result.OrderNo, &result.CustomerID, &result.Required, &result.Status, &result.Revision, &result.AcceptedRevision, &result.ResponsibleEmployeeID, &result.Reason, &result.History)
	if err != nil {
		return result, err
	}
	state, err := loadOrderEditState(ctx, queryer, r.schema, id, false)
	edit := salesapp.EvaluateOrderEditability(state)
	result.ProcessStatus = state.ProcessStatus
	result.ShipStatus = state.ShipStatus
	if err != nil {
		return result, err
	}
	result.CanEdit = edit.CanEdit
	result.EditBlockReason = edit.BlockReason
	return result, nil
}

func (r Repository) ReviewOrder(ctx context.Context, cmd salesapp.ReviewOrderCommand) (salesapp.OrderConfirmation, error) {
	if cmd.Decision != "accepted" && cmd.Decision != "rejected" {
		return salesapp.OrderConfirmation{}, fmt.Errorf("请选择确认或拒绝")
	}
	if cmd.Decision == "rejected" && strings.TrimSpace(cmd.Reason) == "" {
		return salesapp.OrderConfirmation{}, fmt.Errorf("请填写拒绝原因")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	defer tx.Rollback(ctx)
	var pending []byte
	var status string
	var revision, accepted, owner int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.confirmation_pending,o.confirmation_status,o.confirmation_revision,o.confirmation_accepted_revision,COALESCE(c.responsible_employee_id,0) FROM %[1]s.orders o JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=$1 AND o.confirmation_required FOR UPDATE OF o FOR SHARE OF c`, r.schema), cmd.OrderID).Scan(&pending, &status, &revision, &accepted, &owner)
	if err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if !cmd.Admin && (cmd.EmployeeID <= 0 || cmd.EmployeeID != owner) {
		return salesapp.OrderConfirmation{}, fmt.Errorf("仅客户负责人或管理员可以确认或拒绝订单")
	}
	if revision != cmd.Revision {
		return salesapp.OrderConfirmation{}, salesapp.NewOrderEditConflictError("订单已更新，请重新打开后确认")
	}
	if status != "pending" {
		return salesapp.OrderConfirmation{}, salesapp.NewOrderEditConflictError("该版本已处理，请刷新订单")
	}
	state, err := loadOrderEditState(ctx, tx, r.schema, cmd.OrderID, false)
	if err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if edit := salesapp.EvaluateOrderEditability(state); !edit.CanEdit {
		return salesapp.OrderConfirmation{}, salesapp.NewOrderEditConflictError(edit.BlockReason)
	}
	next := cmd.Decision
	if cmd.Decision == "accepted" {
		var content orderContentSnapshot
		if err = json.Unmarshal(pending, &content); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		if err = r.validateConfirmationContentTx(ctx, tx, content); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		if err = applyOrderContentSnapshotTx(ctx, tx, r.schema, cmd.OrderID, content); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		var header struct {
			Tracking string `json:"ship_tracking_no"`
		}
		if err = json.Unmarshal(content.Order, &header); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		if _, err = replaceOrderTrackingNumbersTx(ctx, tx, r.schema, cmd.OrderID, header.Tracking, "order_confirmation", cmd.Actor); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		if err = invalidateConfirmedOrderDocumentsTx(ctx, tx, r.schema, cmd.OrderID); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
		accepted = revision
	} else if accepted > 0 {
		next = "accepted"
	}
	clear := cmd.Decision == "accepted" || accepted > 0
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_status=$2,confirmation_accepted_revision=$3,confirmation_reason=$4,confirmation_pending=CASE WHEN $5 THEN NULL ELSE confirmation_pending END WHERE id=$1`, r.schema), cmd.OrderID, next, accepted, strings.TrimSpace(cmd.Reason), clear); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_content_versions SET status=$3,reviewed_by=$4,reviewed_at=now(),reason=$5 WHERE order_id=$1 AND revision=$2`, r.schema), cmd.OrderID, revision, cmd.Decision, cmd.Actor, cmd.Reason); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order", &cmd.OrderID, "confirm_"+cmd.Decision, postgresinfra.StrPtr("confirmation_status"), postgresinfra.StrPtr("pending"), postgresinfra.StrPtr(next), postgresinfra.AuditMeta{"revision": revision, "reason": cmd.Reason}); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,$2,'confirmation_status','pending',$3)`, r.schema), cmd.OrderID, cmd.Actor, next+" "+cmd.Reason); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if cmd.Decision == "rejected" && accepted > 0 {
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order", &cmd.OrderID, "restore_accepted_content", postgresinfra.StrPtr("content_revision"), postgresinfra.StrPtr(fmt.Sprint(revision)), postgresinfra.StrPtr(fmt.Sprint(accepted)), postgresinfra.AuditMeta{"reason": cmd.Reason}); err != nil {
			return salesapp.OrderConfirmation{}, err
		}
	}
	result, err := r.orderConfirmationRead(ctx, tx, cmd.OrderID)
	if err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	return result, nil
}

func (r Repository) validateConfirmationContentTx(ctx context.Context, tx pgx.Tx, s orderContentSnapshot) error {
	var order struct {
		CustomerID int64 `json:"customer_id"`
	}
	var items []struct {
		ProductID     int64   `json:"product_id"`
		BomSpecID     int64   `json:"bom_spec_id"`
		BomVariantID  int64   `json:"bom_variant_id"`
		PublicationID int64   `json:"bean_list_publication_id"`
		ProductKind   string  `json:"product_kind"`
		UnitPrice     float64 `json:"unit_price"`
	}
	if err := json.Unmarshal(s.Order, &order); err != nil {
		return err
	}
	if err := json.Unmarshal(s.Items, &items); err != nil {
		return err
	}
	for _, item := range items {
		var active bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.products WHERE id=$1 FOR SHARE`, r.schema), item.ProductID).Scan(&active); err != nil {
			return err
		}
		if !active || item.UnitPrice <= 0 {
			return fmt.Errorf("商品或价格已不可用，请修改订单后重新保存")
		}
		if item.BomSpecID > 0 {
			if _, err := resolveOrderBOMSpecIdentityTx(ctx, tx, r.schema, item.ProductID, item.BomSpecID, item.BomVariantID); err != nil {
				return err
			}
		}
		if item.PublicationID > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.bean_list_publications IN SHARE MODE", r.schema)); err != nil {
				return err
			}
			kind := "commercial"
			if item.ProductKind == "green_bean" {
				kind = "green"
			}
			if item.ProductKind == "drip_bag" {
				kind = "drip"
			}
			ok, err := isCurrentDefaultOrderPublicationTx(ctx, tx, r.schema, order.CustomerID, item.PublicationID, kind)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("价格表已更新或失效，请修改订单后重新保存，系统不会自动换价")
			}
		}
	}
	return nil
}

func invalidateConfirmedOrderDocumentsTx(ctx context.Context, tx pgx.Tx, schema string, id int64) error {
	for _, table := range []string{"sales_order_documents", "sales_order_images", "delivery_note_documents", "combined_sales_order_documents", "combined_sales_order_images", "combined_delivery_note_documents"} {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		condition := "order_id=$1"
		if strings.HasPrefix(table, "combined_") {
			condition = "order_ids @> jsonb_build_array($1::bigint)"
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET is_latest=false WHERE %s", schema, table, condition), id); err != nil {
			return err
		}
	}
	return nil
}

// Legacy header/inline endpoints cannot silently bypass the full review flow.
func requireOrdinaryOrderEditTx(ctx context.Context, tx pgx.Tx, schema string, id int64) error {
	var managed bool
	if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false) FROM %s.orders o WHERE id=$1 FOR UPDATE", schema), id).Scan(&managed); err != nil {
		return err
	}
	if managed {
		return fmt.Errorf("履约订单请从订单详情修改并保存为待确认，状态由实际履约进度更新")
	}
	return nil
}
func requireAcceptedOrderExecutionTx(ctx context.Context, tx pgx.Tx, schema string, id int64) error {
	var accepted bool
	if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted' FROM %s.orders o WHERE id=$1 FOR UPDATE", schema), id).Scan(&accepted); err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("履约订单尚未确认，不能进行发货操作")
	}
	return nil
}
