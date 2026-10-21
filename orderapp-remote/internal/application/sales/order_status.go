package sales

import (
	"context"
	"fmt"
	"strings"
)

// UpdateOrderStatusCommand changes settlement and shipping metadata without
// rebuilding order items or reapplying the customer's current price table.
type UpdateOrderStatusCommand struct {
	OrderID               int64
	ExpectedEditRevision  string
	PayStatusID           int64
	PaymentMethod         string
	ShipStatusID          int64
	PrepaymentAmount      *float64
	PaymentGoodsAmount    *float64
	PaymentShippingAmount *float64
	PaymentVoucherAssetID *int64
	Actor                 string
}

type OrderStatusRepository interface {
	UpdateOrderStatus(context.Context, UpdateOrderStatusCommand) error
}

func (s *Service) UpdateOrderStatus(ctx context.Context, cmd UpdateOrderStatusCommand) error {
	if cmd.OrderID <= 0 {
		return fmt.Errorf("invalid order id")
	}
	if strings.TrimSpace(cmd.ExpectedEditRevision) == "" {
		return NewOrderEditConflictError("订单数据版本缺失，请重新打开订单后再保存状态")
	}
	repo, ok := s.repo.(OrderStatusRepository)
	if !ok {
		return fmt.Errorf("order status update unavailable")
	}
	return repo.UpdateOrderStatus(ctx, cmd)
}
