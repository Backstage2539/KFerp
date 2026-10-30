package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type CombinedPaymentValidationError struct{ Message string }

func (e *CombinedPaymentValidationError) Error() string { return e.Message }
func NewCombinedPaymentValidationError(message string) error {
	return &CombinedPaymentValidationError{Message: message}
}
func CombinedPaymentValidationMessage(err error) (string, bool) {
	var target *CombinedPaymentValidationError
	if errors.As(err, &target) {
		return target.Message, true
	}
	return "", false
}

// CombinedOrderPaymentCommand describes one physical receipt allocated over
// several orders. Cents are integers so the receipt is never rounded per row.
type CombinedOrderPaymentCommand struct {
	Orders                []CombinedPaymentOrder `json:"orders"`
	ActualCents           int64                  `json:"actual_cents"`
	ExpectedDueCents      int64                  `json:"expected_due_cents"`
	PaymentMethod         string                 `json:"payment_method"`
	PaymentVoucherAssetID int64                  `json:"payment_voucher_asset_id"`
	ShipStatusID          *int64                 `json:"ship_status_id"`
	Actor                 string                 `json:"-"`
	IdempotencyKey        string                 `json:"idempotency_key"`
}

type CombinedPaymentOrder struct {
	OrderID      int64  `json:"order_id"`
	EditRevision string `json:"edit_revision"`
}

type CombinedPaymentAllocation struct {
	OrderID       int64  `json:"order_id"`
	OrderNo       string `json:"order_no"`
	GoodsCents    int64  `json:"goods_cents"`
	ShippingCents int64  `json:"shipping_cents"`
	AmountCents   int64  `json:"amount_cents"`
}

type CombinedOrderPaymentResult struct {
	BatchID               int64                       `json:"batch_id"`
	CustomerID            int64                       `json:"customer_id"`
	ActualCents           int64                       `json:"actual_cents"`
	DueCents              int64                       `json:"due_cents"`
	UnallocatedCents      int64                       `json:"unallocated_cents"`
	PaymentMethod         string                      `json:"payment_method"`
	PaymentVoucherAssetID int64                       `json:"payment_voucher_asset_id"`
	Actor                 string                      `json:"actor"`
	CreatedAt             string                      `json:"created_at"`
	Allocations           []CombinedPaymentAllocation `json:"allocations"`
	Reused                bool                        `json:"reused"`
}

type combinedOrderPaymentRepository interface {
	CombinedOrderPayment(context.Context, CombinedOrderPaymentCommand) (CombinedOrderPaymentResult, error)
	OrderCombinedPayments(context.Context, int64) ([]CombinedOrderPaymentResult, error)
}

func (s *Service) CombinedOrderPayment(ctx context.Context, cmd CombinedOrderPaymentCommand) (CombinedOrderPaymentResult, error) {
	if len(cmd.Orders) < 1 || len(cmd.Orders) > 200 || cmd.ActualCents <= 0 || cmd.ExpectedDueCents <= 0 || cmd.ActualCents < cmd.ExpectedDueCents || strings.TrimSpace(cmd.PaymentMethod) == "" || len(cmd.PaymentMethod) > 128 || cmd.PaymentVoucherAssetID < 0 || strings.TrimSpace(cmd.Actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 || (cmd.ShipStatusID != nil && *cmd.ShipStatusID <= 0) {
		return CombinedOrderPaymentResult{}, NewCombinedPaymentValidationError("合并收款资料无效，实际收款不得少于待收金额")
	}
	seen := map[int64]bool{}
	for _, order := range cmd.Orders {
		if order.OrderID <= 0 || strings.TrimSpace(order.EditRevision) == "" || seen[order.OrderID] {
			return CombinedOrderPaymentResult{}, NewCombinedPaymentValidationError("订单重复或版本缺失，请刷新后重试")
		}
		seen[order.OrderID] = true
	}
	repo, ok := s.repo.(combinedOrderPaymentRepository)
	if !ok {
		return CombinedOrderPaymentResult{}, fmt.Errorf("合并收款功能不可用")
	}
	return repo.CombinedOrderPayment(ctx, cmd)
}

func (s *Service) OrderCombinedPayments(ctx context.Context, orderID int64) ([]CombinedOrderPaymentResult, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	repo, ok := s.repo.(combinedOrderPaymentRepository)
	if !ok {
		return nil, fmt.Errorf("合并收款功能不可用")
	}
	return repo.OrderCombinedPayments(ctx, orderID)
}
