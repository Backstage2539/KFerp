package sales

import (
	"context"
	"strings"
	"time"
)

type OrderReceiptCommand struct {
	OrderID               int64  `json:"order_id"`
	EditRevision          string `json:"edit_revision"`
	ActualCents           int64  `json:"actual_cents"`
	ExpectedDueCents      int64  `json:"expected_due_cents"`
	ReceiptDate           string `json:"receipt_date"`
	PaymentMethod         string `json:"payment_method"`
	PaymentVoucherAssetID int64  `json:"payment_voucher_asset_id"`
	IdempotencyKey        string `json:"idempotency_key"`
	Actor                 string `json:"-"`
}
type OrderReceiptResult struct {
	ReceiptID int64  `json:"receipt_id"`
	ReceiptNo string `json:"receipt_no"`
	Reused    bool   `json:"reused"`
}

type OrderReceiptHistory struct {
	ReceiptID       int64  `json:"receipt_id"`
	ReceiptNo       string `json:"receipt_no"`
	ReceiptDate     string `json:"receipt_date"`
	ActualCents     int64  `json:"actual_cents"`
	ReceivableCents int64  `json:"receivable_cents"`
	PaymentMethod   string `json:"payment_method"`
	VoucherAssetID  int64  `json:"payment_voucher_asset_id"`
	Actor           string `json:"actor"`
}
type orderReceiptRepository interface {
	RegisterOrderReceipt(context.Context, OrderReceiptCommand) (OrderReceiptResult, error)
	OrderReceipts(context.Context, int64) ([]OrderReceiptHistory, error)
}

func (s *Service) OrderReceipts(ctx context.Context, orderID int64) ([]OrderReceiptHistory, error) {
	if orderID <= 0 {
		return nil, NewCombinedPaymentValidationError("订单编号无效")
	}
	repo, ok := s.repo.(orderReceiptRepository)
	if !ok {
		return nil, NewCombinedPaymentValidationError("收款记录不可用")
	}
	return repo.OrderReceipts(ctx, orderID)
}

func ValidateOrderReceipt(cmd OrderReceiptCommand) error {
	if validateReceiptDate(cmd.ReceiptDate, time.Now()) != nil || cmd.OrderID <= 0 || cmd.ActualCents <= 0 || cmd.ExpectedDueCents <= 0 || cmd.ActualCents > cmd.ExpectedDueCents || strings.TrimSpace(cmd.EditRevision) == "" || strings.TrimSpace(cmd.PaymentMethod) == "" || strings.TrimSpace(cmd.Actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 || cmd.PaymentVoucherAssetID < 0 {
		return NewCombinedPaymentValidationError("收款资料无效，请核对金额、日期和订单版本")
	}
	return nil
}

func validateReceiptDate(date string, now time.Time) error {
	d, err := time.Parse("2006-01-02", date)
	if err != nil || d.Format("2006-01-02") != date || date > now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02") {
		return NewCombinedPaymentValidationError("实际到账日期无效或晚于今天")
	}
	return nil
}

func ValidateReceiptDate(date string) error { return validateReceiptDate(date, time.Now()) }
func (s *Service) RegisterOrderReceipt(ctx context.Context, cmd OrderReceiptCommand) (OrderReceiptResult, error) {
	if err := ValidateOrderReceipt(cmd); err != nil {
		return OrderReceiptResult{}, err
	}
	repo, ok := s.repo.(orderReceiptRepository)
	if !ok {
		return OrderReceiptResult{}, NewCombinedPaymentValidationError("收款功能不可用")
	}
	return repo.RegisterOrderReceipt(ctx, cmd)
}
