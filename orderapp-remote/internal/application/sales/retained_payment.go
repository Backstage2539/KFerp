package sales

import (
	"fmt"
	"strings"
)

// ValidateNewRetainedPayment rejects receipt information on new retained purchases.
func ValidateNewRetainedPayment(c SaveOrderCommand) error {
	if c.EditID != 0 || strings.TrimSpace(c.DeliveryMode) != "customer_stock" {
		return nil
	}
	if (c.PrepaymentAmount != nil && *c.PrepaymentAmount != 0) || c.PaymentGoodsAmount != 0 || c.PaymentShippingAmount != 0 || c.PaymentVoucherAssetID != 0 || strings.TrimSpace(c.PaymentMethod) != "" {
		return fmt.Errorf("新建购货留厂订单须为未付款，请通过费用流程登记收款")
	}
	return nil
}
