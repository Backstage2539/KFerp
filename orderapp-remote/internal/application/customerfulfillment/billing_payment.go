package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
)

type BillingPaymentCommand struct {
	CustomerID              int64  `json:"-"`
	BatchID                 int64  `json:"-"`
	Action                  string `json:"action"`
	ExpectedDueAmountCents  *int64 `json:"expected_due_amount_cents"`
	IdempotencyKey          string `json:"idempotency_key"`
	ExpectedPaymentRevision *int64 `json:"expected_payment_revision"`
	PaymentMethod           string `json:"payment_method"`
	Actor                   string `json:"-"`
}

type billingPaymentRepository interface {
	ChangeBillingPayment(context.Context, BillingPaymentCommand) (BillingBill, error)
}

func (s *Service) ChangeBillingPayment(ctx context.Context, cmd BillingPaymentCommand) (BillingBill, error) {
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
