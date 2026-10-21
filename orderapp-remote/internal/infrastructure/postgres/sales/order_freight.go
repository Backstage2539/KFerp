package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
)

func protectFreightReceiptsTx(ctx context.Context, tx pgx.Tx, schema string, id int64, c *salesapp.SaveOrderCommand, shipping, total float64) error {
	var oldShipping, paid, goods, freight, prepay, oldTotal float64
	var voucher, status int64
	var statusName string
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(o.shipping_amount,0),COALESCE(o.payment_goods_amount,0),COALESCE(o.payment_shipping_amount,0),COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0),COALESCE(o.payment_voucher_asset_id,0),COALESCE(o.pay_status_id,0),COALESCE(p.name,''),COALESCE(o.grand_total,0) FROM %s.orders o LEFT JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, schema, schema), id).Scan(&oldShipping, &goods, &freight, &prepay, &voucher, &status, &statusName, &oldTotal)
	if e != nil {
		return e
	}
	if math.Abs(shipping-oldShipping) < 0.00001 {
		return nil
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
		if c.PaymentVoucherAssetID != voucher || math.Abs(c.PaymentGoodsAmount-goods) > 0.00001 || math.Abs(c.PaymentShippingAmount-freight) > 0.00001 {
			return fmt.Errorf("已有真实收款及凭证不能随运费调整改写，请先核对原收款")
		}
		if total > paid+0.00001 && salesdomain.IsFullyPaid(statusName) {
			var partial int64
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name ILIKE '%%预付款%%' ORDER BY id LIMIT 1`, schema)).Scan(&partial)
			if e != nil {
				return fmt.Errorf("新增运费尚未收取，请先维护部分收款状态")
			}
			c.PayStatusID = partial
			c.PrepaymentAmount = &paid
		} else if prepay > 0 {
			c.PrepaymentAmount = &prepay
		}
	}
	return nil
}
