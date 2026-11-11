package customerfulfillment

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"strings"
)

type BillingPaymentCommand struct {
	CustomerID              int64                          `json:"-"`
	BatchID                 int64                          `json:"-"`
	Action                  string                         `json:"action"`
	ExpectedDueAmountCents  *int64                         `json:"expected_due_amount_cents"`
	IdempotencyKey          string                         `json:"idempotency_key"`
	ExpectedPaymentRevision *int64                         `json:"expected_payment_revision"`
	PaymentMethod           string                         `json:"payment_method"`
	ReceiptDate             string                         `json:"receipt_date,omitempty"`
	ActualCents             *int64                         `json:"actual_cents,omitempty"`
	Allocations             []BillingPaymentLineAllocation `json:"allocations,omitempty"`
	Actor                   string                         `json:"-"`
}
type BillingPaymentLineAllocation struct {
	LineID      int64 `json:"line_id"`
	AmountCents int64 `json:"amount_cents"`
}

type billingPaymentRepository interface {
	ChangeBillingPayment(context.Context, BillingPaymentCommand) (BillingBill, error)
}

func (s *Service) ChangeBillingPayment(ctx context.Context, cmd BillingPaymentCommand) (BillingBill, error) {
	if cmd.ReceiptDate != "" && (cmd.Action != "pay" || salesapp.ValidateReceiptDate(cmd.ReceiptDate) != nil) {
		return BillingBill{}, fmt.Errorf("实际到账日期无效或晚于今天")
	}
	if cmd.ActualCents != nil && (cmd.Action != "pay" || *cmd.ActualCents <= 0 || cmd.ExpectedDueAmountCents == nil || *cmd.ActualCents > *cmd.ExpectedDueAmountCents) {
		return BillingBill{}, fmt.Errorf("实际收款金额无效")
	}
	if cmd.CustomerID <= 0 || cmd.BatchID <= 0 || cmd.ExpectedDueAmountCents == nil || *cmd.ExpectedDueAmountCents < 0 || cmd.ExpectedPaymentRevision == nil || *cmd.ExpectedPaymentRevision < 0 || (cmd.Action != "pay" && cmd.Action != "reverse") || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 || (cmd.Action == "pay" && strings.TrimSpace(cmd.PaymentMethod) == "") || len(cmd.PaymentMethod) > 128 {
		return BillingBill{}, fmt.Errorf("账单或付款操作无效")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		return BillingBill{}, fmt.Errorf("操作人不能为空")
	}
	repo, ok := s.repo.(billingPaymentRepository)
	if !ok {
		return BillingBill{}, fmt.Errorf("账单付款功能不可用")
	}
	return repo.ChangeBillingPayment(ctx, cmd)
}
