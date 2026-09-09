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

func EnsureCustomerOrderSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS customer_request_id TEXT NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS customer_request_hash TEXT NOT NULL DEFAULT '';
 CREATE UNIQUE INDEX IF NOT EXISTS orders_customer_request_unique ON %[1]s.orders(customer_id,customer_request_id) WHERE customer_request_id<>'';
 CREATE OR REPLACE FUNCTION %[1]s.require_fulfillment_recipient() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF NEW.portal_service_code IN ('direct_ship','product_order','processing_ship')
 AND (btrim(coalesce(NEW.receiver_name,''))='' OR btrim(coalesce(NEW.receiver_phone,''))='' OR btrim(coalesce(NEW.receiver_address,''))='')
 AND EXISTS(SELECT 1 FROM %[1]s.ship_statuses WHERE id=NEW.ship_status_id AND name IN ('部分发货','已发货','已出库','已签收','已收货','已完成'))
 THEN RAISE EXCEPTION '待补收件信息：填写收件人、电话和地址后才能发货' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $fn$;
 DROP TRIGGER IF EXISTS require_fulfillment_recipient ON %[1]s.orders;
 CREATE TRIGGER require_fulfillment_recipient BEFORE INSERT OR UPDATE OF ship_status_id,receiver_name,receiver_phone,receiver_address ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.require_fulfillment_recipient();
 `, schema))
	if err != nil {
		return err
	}
	return ensureOrderConfirmationSchema(ctx, pool, schema)
}

func (r Repository) UpdateCustomerRecipient(ctx context.Context, c salesapp.CustomerRecipientCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var name, phone, address, status string
	var void bool
	var shipStatusID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(o.receiver_name,''),coalesce(o.receiver_phone,''),coalesce(o.receiver_address,''),coalesce(o.ship_status_id,0),coalesce(o.is_void,false) FROM %[1]s.orders o WHERE o.id=$1 AND o.customer_id=$2 FOR UPDATE OF o`, r.schema), c.OrderID, c.CustomerID).Scan(&name, &phone, &address, &shipStatusID, &void)
	if err != nil {
		return fmt.Errorf("订单不存在或不属于当前客户")
	}
	// Read the status after acquiring the order lock. A joined status row from
	// the pre-lock snapshot can be stale when a concurrent shipment commits.
	if shipStatusID > 0 {
		if err = tx.QueryRow(ctx, fmt.Sprintf("SELECT name FROM %s.ship_statuses WHERE id=$1", r.schema), shipStatusID).Scan(&status); err != nil {
			return err
		}
	}
	if void || customerRecipientLockedStatus(status) {
		return fmt.Errorf("已发货或作废的订单不能修改收件信息")
	}
	state, err := loadOrderEditState(ctx, tx, r.schema, c.OrderID, false)
	if err != nil {
		return err
	}
	if edit := salesapp.EvaluateOrderEditability(state); !edit.CanEdit {
		return salesapp.NewOrderEditConflictError(edit.BlockReason)
	}
	before, err := orderContentSnapshotTx(ctx, tx, r.schema, c.OrderID)
	if err != nil {
		return err
	}
	var pending []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf("SELECT confirmation_pending FROM %s.orders WHERE id=$1", r.schema), c.OrderID).Scan(&pending); err != nil {
		return err
	}
	if len(pending) > 0 {
		var current orderContentSnapshot
		if err = json.Unmarshal(pending, &current); err != nil {
			return err
		}
		if err = applyOrderContentSnapshotTx(ctx, tx, r.schema, c.OrderID, current); err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET receiver_name=$2,receiver_phone=$3,receiver_address=$4 WHERE id=$1", r.schema), c.OrderID, c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress)
	if err != nil {
		return err
	}
	err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "order", &c.OrderID, "update", postgresinfra.StrPtr("recipient"), postgresinfra.StrPtr(name+" "+phone+" "+address), postgresinfra.StrPtr(c.ReceiverName+" "+c.ReceiverPhone+" "+c.ReceiverAddress), postgresinfra.AuditMeta{"customer_id": c.CustomerID, "reason": "补全收件信息"})
	if err != nil {
		return err
	}
	if err = r.stageOrderConfirmationTx(ctx, tx, c.OrderID, &before, c.Actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) FindCustomerOrderRequest(ctx context.Context, customerID int64, requestID, expectedHash string) (salesapp.SaveOrderResult, bool, error) {
	var result salesapp.SaveOrderResult
	var hash string
	err := r.pool.QueryRow(ctx, orderConfirmationRequestQuery(r.schema), customerID, requestID).Scan(&result.OrderID, &result.OrderNo, &hash, &result.ConfirmationStatus)
	if err == nil {
		if hash != expectedHash {
			return result, false, fmt.Errorf("该请求已用于其他订单内容，请刷新后重试")
		}
		result.Replayed = true
		return result, true, nil
	}
	if err != pgx.ErrNoRows {
		return result, false, err
	}
	err = r.pool.QueryRow(ctx, fmt.Sprintf("SELECT id,order_no,customer_request_hash FROM %s.orders WHERE customer_id=$1 AND customer_request_id=$2", r.schema), customerID, requestID).Scan(&result.OrderID, &result.OrderNo, &hash)
	if err == pgx.ErrNoRows {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if hash != expectedHash {
		return result, false, fmt.Errorf("该请求已用于另一张订单，请刷新后重试")
	}
	result.Replayed = true
	return result, true, nil
}

func customerRecipientLockedStatus(status string) bool {
	for _, value := range []string{"已发货", "部分发货", "已出库", "已签收", "已收货", "已完成"} {
		if strings.Contains(strings.TrimSpace(status), value) {
			return true
		}
	}
	return false
}
