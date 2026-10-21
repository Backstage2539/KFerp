package sales

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) UpdateOrderStatus(ctx context.Context, cmd salesapp.UpdateOrderStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var curPay, curShip, companyID, productID, voucherID int64
	var curMethod, shipMethod, reason, confirmation string
	var prepayment, total, goods, receivedGoods, receivedShipping float64
	var isVoid, managed bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(pay_status_id,0),COALESCE(ship_status_id,0),COALESCE(payment_method,''),
		       COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0)::float8,
		       COALESCE(grand_total,0)::float8,COALESCE(total_amount,0)::float8,
		       COALESCE(payment_goods_amount,0)::float8,COALESCE(payment_shipping_amount,0)::float8,
		       COALESCE(payment_voucher_asset_id,0),COALESCE(logistics_company_id,0),
		       COALESCE(logistics_product_id,0),COALESCE(ship_method,''),COALESCE(is_void,false),
		       COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),
		       COALESCE(to_jsonb(o)->>'confirmation_status','accepted'),
		       COALESCE(to_jsonb(o)->>'settlement_reason','')
		FROM %s.orders o WHERE id=$1 FOR UPDATE`, r.schema), cmd.OrderID).Scan(
		&curPay, &curShip, &curMethod, &prepayment, &total, &goods, &receivedGoods,
		&receivedShipping, &voucherID, &companyID, &productID, &shipMethod, &isVoid,
		&managed, &confirmation, &reason,
	)
	if err != nil {
		return err
	}
	sample, err := sampleOrderLinked(ctx, tx, r.schema, cmd.OrderID)
	if err != nil {
		return err
	}
	if isVoid {
		return salesapp.NewOrderEditConflictError("订单已作废，不能修改状态")
	}
	if managed && confirmation != "accepted" {
		return salesapp.NewOrderEditConflictError("订单尚未确认，请先确认订单后再修改付款或发货状态")
	}
	if managed && cmd.ShipStatusID != curShip {
		return salesapp.NewOrderEditConflictError("履约订单的发货状态由实际履约进度更新，请通过发货操作处理")
	}
	expected := strings.TrimSpace(cmd.ExpectedEditRevision)
	if expected == "" {
		return salesapp.NewOrderEditConflictError("订单数据版本缺失，请重新打开订单后再保存状态")
	}
	current, err := currentOrderEditRevisionTx(ctx, tx, r.schema, cmd.OrderID)
	if err != nil {
		return err
	}
	if current != expected {
		return salesapp.NewOrderEditConflictError("订单已被其他操作修改，请刷新订单后再保存状态")
	}
	if cmd.PayStatusID <= 0 && cmd.PayStatusID != curPay {
		return fmt.Errorf("invalid pay_status_id")
	}
	if cmd.ShipStatusID <= 0 && cmd.ShipStatusID != curShip {
		return fmt.Errorf("invalid ship_status_id")
	}
	statusName, err := lookupStatusName(ctx, tx, r.schema, "pay_statuses", cmd.PayStatusID)
	if err != nil {
		return fmt.Errorf("invalid pay_status_id")
	}
	nextPrepayment := prepayment
	if cmd.PrepaymentAmount != nil {
		nextPrepayment = *cmd.PrepaymentAmount
	}
	if err = salesdomain.ValidatePrepayment(statusName, nextPrepayment, total); err != nil {
		return err
	}
	nextGoods, nextShipping, nextVoucher := receivedGoods, receivedShipping, voucherID
	if cmd.PaymentGoodsAmount != nil {
		nextGoods = *cmd.PaymentGoodsAmount
	}
	if cmd.PaymentShippingAmount != nil {
		nextShipping = *cmd.PaymentShippingAmount
	}
	if cmd.PaymentVoucherAssetID != nil {
		nextVoucher = *cmd.PaymentVoucherAssetID
	}
	for _, amount := range []float64{nextGoods, nextShipping} {
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || math.Abs(amount*100-math.Round(amount*100)) > 0.00001 {
			return fmt.Errorf("实收金额必须为非负金额，最多两位小数")
		}
	}
	if nextVoucher < 0 {
		return fmt.Errorf("invalid payment_voucher_asset_id")
	}
	if nextVoucher > 0 && nextVoucher != voucherID {
		var owned bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher' AND created_by=$2)`, r.schema), nextVoucher, strings.TrimSpace(cmd.Actor)).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return fmt.Errorf("invalid payment_voucher_asset_id")
		}
	}
	method := ""
	if reason == "free_sample" {
		if !salesdomain.IsFullyPaid(statusName) || nextPrepayment != 0 || nextGoods != 0 || nextShipping != 0 || nextVoucher != 0 {
			return salesapp.NewOrderEditConflictError("免费样品无需收款，不能修改为未付款或登记实收，请先核对样品费用")
		}
	} else {
		method, err = normalizeOrderPaymentMethodForStatusTx(ctx, tx, r.schema, cmd.PayStatusID, cmd.PaymentMethod)
		if err != nil {
			return err
		}
	}
	// A payment update must not require repairing historical shipping metadata.
	// Recheck logistics only when the user actually changes shipping status.
	shipToValidate := cmd.ShipStatusID
	if shipToValidate == curShip {
		shipToValidate = 0
	}
	if err = validateOrderFulfillmentWithPaymentPolicyTx(ctx, tx, r.schema, cmd.PayStatusID, shipToValidate, companyID, productID, nextGoods, nextShipping, nextVoucher, shipMethod, reason == "free_sample", sample && goods == 0); err != nil {
		return err
	}
	type statusDiff struct{ field, old, new string }
	diffs := []statusDiff{}
	add := func(field, old, new string) {
		if old != new {
			diffs = append(diffs, statusDiff{field, old, new})
		}
	}
	add("pay_status_id", strconv.FormatInt(curPay, 10), strconv.FormatInt(cmd.PayStatusID, 10))
	add("ship_status_id", strconv.FormatInt(curShip, 10), strconv.FormatInt(cmd.ShipStatusID, 10))
	add("payment_method", curMethod, method)
	add("prepayment_amount", fmt.Sprintf("%.2f", prepayment), fmt.Sprintf("%.2f", nextPrepayment))
	add("payment_goods_amount", fmt.Sprintf("%.2f", receivedGoods), fmt.Sprintf("%.2f", nextGoods))
	add("payment_shipping_amount", fmt.Sprintf("%.2f", receivedShipping), fmt.Sprintf("%.2f", nextShipping))
	add("payment_voucher_asset_id", strconv.FormatInt(voucherID, 10), strconv.FormatInt(nextVoucher, 10))
	if len(diffs) == 0 {
		return nil
	}
	// PostgreSQL UPDATE OF triggers run even when a value is unchanged. Omit
	// shipping entirely for payment-only writes so recipient/progress triggers
	// cannot block collection or change shipment progress as a side effect.
	assignments := "pay_status_id=$2,payment_method=$3,prepayment_amount=$4,payment_goods_amount=$5,payment_shipping_amount=$6,payment_voucher_asset_id=$7"
	args := []any{cmd.OrderID, nullInt(cmd.PayStatusID), method, nextPrepayment, nextGoods, nextShipping, nextVoucher}
	if cmd.ShipStatusID != curShip {
		assignments += ",ship_status_id=$8"
		args = append(args, nullInt(cmd.ShipStatusID))
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET %s WHERE id=$1`, r.schema, assignments), args...)
	if err != nil {
		return err
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "unknown"
	}
	for _, diff := range diffs {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,$2,$3,$4,$5)`, r.schema), cmd.OrderID, actor, diff.field, diff.old, diff.new)
		if err != nil {
			return err
		}
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &cmd.OrderID, "update", postgresinfra.StrPtr(diff.field), postgresinfra.StrPtr(diff.old), postgresinfra.StrPtr(diff.new), postgresinfra.AuditMeta{"order_id": cmd.OrderID, "source": "mini-order-status"}); err != nil {
			return err
		}
	}
	if err = invalidateConfirmedOrderDocumentsTx(ctx, tx, r.schema, cmd.OrderID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
