package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
)

func (r Repository) resolveSampleSettlementTx(ctx context.Context, tx pgx.Tx, cmd *salesapp.SaveOrderCommand, sample bool, goods, shipping, other, total float64, payID int64, prepayment float64) (string, int64, error) {
	previous := ""
	var receivedGoods, receivedShipping, storedPrepayment float64
	var voucher int64
	if cmd.EditID > 0 {
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(to_jsonb(o)->>'settlement_reason',''),COALESCE(payment_goods_amount,0),COALESCE(payment_shipping_amount,0),COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0),COALESCE(payment_voucher_asset_id,0) FROM %s.orders o WHERE id=$1`, r.schema), cmd.EditID).Scan(&previous, &receivedGoods, &receivedShipping, &storedPrepayment, &voucher)
		if err != nil {
			return "", payID, err
		}
	}
	free := salesdomain.IsFreeSampleSettlement(sample, goods, shipping, other, total)
	if free {
		if receivedGoods > 0 || receivedShipping > 0 || storedPrepayment > 0 || voucher > 0 || prepayment > 0 || cmd.PaymentGoodsAmount != 0 || cmd.PaymentShippingAmount != 0 || cmd.PaymentVoucherAssetID > 0 {
			return "", payID, fmt.Errorf("样品无需收款，但已有收款金额或凭证，请先按正常收款流程核对处理")
		}
		payID = lookupDefaultStatusID(ctx, tx, r.schema, "pay_statuses", "已收款", "已付款")
		if payID <= 0 {
			return "", payID, fmt.Errorf("请先配置已收款状态")
		}
		return "free_sample", payID, nil
	}
	if previous == "free_sample" {
		if receivedGoods > 0 || receivedShipping > 0 || storedPrepayment > 0 || voucher > 0 {
			return "", payID, fmt.Errorf("样品已有真实收款记录，请先核对处理，不能自动撤销或清除")
		}
		if prepayment > 0 || cmd.PaymentGoodsAmount != 0 || cmd.PaymentShippingAmount != 0 || cmd.PaymentVoucherAssetID > 0 {
			return "", payID, fmt.Errorf("免费样品改为收费，请先保存收费金额，再登记实际收款")
		}
		payID = lookupDefaultStatusID(ctx, tx, r.schema, "pay_statuses", "未收款", "未付款")
		if payID <= 0 {
			return "", payID, fmt.Errorf("请先配置未收款状态")
		}
	}
	return "", payID, nil
}
