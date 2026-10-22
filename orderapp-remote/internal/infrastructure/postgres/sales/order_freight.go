package sales

import (
	"context"
	"fmt"
	"math"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5"
)

func protectFreightReceiptsTx(ctx context.Context, tx pgx.Tx, schema string, id int64, c *salesapp.SaveOrderCommand, shipping, total float64) error {
	var oldShipping, paid, goods, freight, prepay, oldTotal float64
	var voucher, status int64
	var statusName string
	// Use finance's historical total fallback, also retaining rounded zeros.
	// A discount, shipping or rounding distinguishes a calculated zero total.
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(o.shipping_amount,0),COALESCE(o.payment_goods_amount,0),COALESCE(o.payment_shipping_amount,0),COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0),COALESCE(o.payment_voucher_asset_id,0),COALESCE(o.pay_status_id,0),COALESCE(p.name,''),
		CASE WHEN COALESCE(o.grand_total,0) <> 0 OR COALESCE(o.discount_amount,0) <> 0 OR COALESCE(o.shipping_amount,0) <> 0 OR COALESCE(o.rounding_amount,0) <> 0
		THEN COALESCE(o.grand_total,0) ELSE COALESCE(o.total_amount,0) END
		FROM %s.orders o LEFT JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, schema, schema), id).Scan(&oldShipping, &goods, &freight, &prepay, &voucher, &status, &statusName, &oldTotal)
	if e != nil {
		return e
	}
	samePrepayment := c.PrepaymentAmount == nil || math.Abs(*c.PrepaymentAmount-prepay) < 0.00001
	sameReceipt := c.PaymentVoucherAssetID == voucher && math.Abs(c.PaymentGoodsAmount-goods) < 0.00001 && math.Abs(c.PaymentShippingAmount-freight) < 0.00001
	paymentChanged := c.PayStatusID != status || !samePrepayment || !sameReceipt
	if math.Abs(shipping-oldShipping) < 0.00001 && math.Abs(total-oldTotal) < 0.00001 {
		// A payment-only save is an explicit collection command. In the legacy
		// workflow, choosing 已付款 records receipt of the balance while keeping
		// the original deposit; do not replace it or synthesize receipt fields.
		if (voucher > 0 || orderPaidStatusRequiresReceipt(statusName)) && c.PaymentVoucherAssetID <= 0 && paymentChanged {
			return salesapp.NewOrderEditConflictError("已有收款凭证不能清空，请先通过财务流程核对处理")
		}
		if paymentChanged && c.PaymentVoucherAssetID > 0 {
			if c.PaymentVoucherAssetID != voucher {
				var validVoucher bool
				if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher')`, schema), c.PaymentVoucherAssetID).Scan(&validVoucher); err != nil {
					return err
				}
				if !validVoucher {
					return fmt.Errorf("invalid payment_voucher_asset_id")
				}
			}
			nextStatus, err := lookupStatusName(ctx, tx, schema, "pay_statuses", c.PayStatusID)
			if err != nil {
				return err
			}
			collected := prepay
			if c.PrepaymentAmount != nil {
				collected = *c.PrepaymentAmount
			}
			if salesdomain.IsFullyPaid(nextStatus) {
				collected = total
			}
			if math.Abs(c.PaymentGoodsAmount+c.PaymentShippingAmount-collected) > 0.00001 {
				return salesapp.NewOrderEditConflictError("累计实收金额与付款状态或预付款不一致，请按实际累计到账金额核对货款、运费及收款凭证")
			}
		}
		return nil
	}
	// Do not silently discard an explicit collection while adjusting the
	// receivable. Separate saves make the amount being acknowledged unambiguous.
	if paymentChanged {
		return salesapp.NewOrderEditConflictError("运费或订单应收与收款变更请分两次保存：先保留原付款信息，只保存费用调整；再重新打开订单，核对新应收后单独登记实际累计收款")
	}
	paid = prepay
	if salesdomain.IsFullyPaid(statusName) && oldTotal > paid {
		paid = oldTotal
	}
	if voucher > 0 || orderPaidStatusRequiresReceipt(statusName) {
		if goods+freight > paid {
			paid = goods + freight
		}
	}
	if paid > 0 {
		if total+0.00001 < paid || voucher > 0 && shipping+0.00001 < freight {
			return fmt.Errorf("调整运费将使已有真实收款超出应收，请先通过财务流程核对处理")
		}
		// Freight edits preserve the recorded collection; new money is recorded separately.
		if total > paid+0.00001 {
			var partial int64
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name ILIKE '%%预付款%%' ORDER BY id LIMIT 1`, schema)).Scan(&partial)
			if e != nil {
				return fmt.Errorf("新增运费尚未收取，请先维护部分收款状态")
			}
			c.PayStatusID = partial
			c.PrepaymentAmount = &paid
		} else if salesdomain.IsFullyPaid(statusName) {
			c.PayStatusID = status
		} else {
			full := lookupDefaultStatusID(ctx, tx, schema, "pay_statuses", "已付款", "已收款", "已支付")
			if full <= 0 {
				return fmt.Errorf("原已收款已覆盖新应付，请先维护已付款状态")
			}
			c.PayStatusID = full
		}
	} else if salesdomain.IsFullyPaid(statusName) && total > 0.00001 {
		// A zero-value/full-discount order had no collection to carry forward.
		// New freight is wholly unpaid, not a zero-amount prepayment.
		unpaid := lookupDefaultStatusID(ctx, tx, schema, "pay_statuses", "未付款", "未收款", "未支付")
		if unpaid <= 0 {
			return fmt.Errorf("新增应收尚未收取，请先维护未付款状态")
		}
		c.PayStatusID = unpaid
	}
	return nil
}
