package sales

import (
	"context"
	"fmt"
	"strings"
)

// ValidateCustomerOrder is shared by the ERP customer form and legacy portal adapter.
// Customer scope and capability are resolved from the session before this policy.
func ValidateCustomerOrder(c SaveOrderCommand, backfill bool) error {
	if c.PortalServiceCode != "direct_ship" && c.PortalServiceCode != "product_order" {
		return fmt.Errorf("无效录单能力")
	}
	if (c.EditID != 0 && !c.RequireConfirmation) || c.PayStatusID != 0 || c.ShipStatusID != 0 || c.PaymentMethod != "" || c.PaymentGoodsAmount != 0 || c.PaymentShippingAmount != 0 || c.PaymentVoucherAssetID != 0 || c.ShipTrackingNo != "" || c.ShippingAmount != 0 || c.DiscountAmount != 0 || c.RoundToInt || c.OutsourceMaterialFee != 0 || c.OutsourceRoastFee != 0 || c.OutsourcePackagingFee != 0 || c.OutsourceManualFee != 0 || c.OutsourceTaxFee != 0 || c.OutsourceOtherFee != 0 || c.PrepaymentAmount != nil && *c.PrepaymentAmount != 0 {
		return fmt.Errorf("客户只能提交商品、数量、日期和收件信息，不能修改价格或履约状态")
	}
	for _, item := range c.Items {
		if item.ManualPrice != nil || item.DiscountValue != 0 {
			return fmt.Errorf("客户订单价格必须来自所选价格表")
		}
	}
	if !backfill && !CompleteOrderRecipient(c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress) {
		return fmt.Errorf("请填写收件人、电话和地址；历史订单可使用补录模式")
	}
	return nil
}

func CompleteOrderRecipient(name, phone, address string) bool {
	return strings.TrimSpace(name) != "" && strings.TrimSpace(phone) != "" && strings.TrimSpace(address) != ""
}

type CustomerRecipientCommand struct {
	CustomerID      int64
	OrderID         int64
	Actor           string
	ReceiverName    string `json:"receiver_name"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
}
type customerRecipientRepository interface {
	UpdateCustomerRecipient(context.Context, CustomerRecipientCommand) error
}

func (s *Service) UpdateCustomerRecipient(ctx context.Context, c CustomerRecipientCommand) error {
	if c.CustomerID <= 0 || c.OrderID <= 0 || !CompleteOrderRecipient(c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress) {
		return fmt.Errorf("请填写完整收件信息")
	}
	repo, ok := s.repo.(customerRecipientRepository)
	if !ok {
		return fmt.Errorf("收件信息服务不可用")
	}
	return repo.UpdateCustomerRecipient(ctx, c)
}

func (s *Service) validateCustomerCatalog(ctx context.Context, cmd *SaveOrderCommand) error {
	data, err := s.OrderForm(ctx, 0)
	if err != nil {
		return err
	}
	tables, err := ResolveOrderPriceTableSelection(data.BeanListVersionOptions, cmd.CustomerID, cmd.SelectedPriceTableIDs, true)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("客户尚无可用价格表")
	}
	if len(cmd.SelectedPriceTableIDs) == 0 {
		for _, table := range tables {
			cmd.SelectedPriceTableIDs = append(cmd.SelectedPriceTableIDs, table.ID)
		}
	}
	products := FilterOrderProductsForCustomer(data.Products, cmd.CustomerID, data.BeanListVersionOptions, data.CustomerPublicUsages)
	allowed := map[int64]bool{}
	for _, p := range products {
		allowed[p.ID] = true
		if p.ParentProductID > 0 {
			allowed[p.ParentProductID] = true
		}
	}
	for _, item := range cmd.Items {
		if item.ProductID == nil || !allowed[*item.ProductID] {
			return fmt.Errorf("商品不在当前客户所选价格表中")
		}
	}
	return nil
}

// Request lookup runs before catalog refresh so a retried successful request still
// returns its original order after a newer price table has been published.
type customerOrderRequestRepository interface {
	FindCustomerOrderRequest(context.Context, int64, string, string) (SaveOrderResult, bool, error)
}
