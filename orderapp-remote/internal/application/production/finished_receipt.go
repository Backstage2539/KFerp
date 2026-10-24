package production

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// ReceiptQuantityContext is shared by the receipt list and transactional posting.
type ReceiptQuantityContext struct {
	PackageSpecLabel         string                  `json:"package_spec_label"`
	TargetWarehouseName      string                  `json:"target_warehouse_name"`
	InventoryOwnerCustomerID int64                   `json:"inventory_owner_customer_id"`
	PlannedQty               float64                 `json:"planned_qty"`
	RemainingPlanQty         float64                 `json:"remaining_plan_qty"`
	BlockedReason            string                  `json:"blocked_reason"`
	ReceiptVersion           string                  `json:"receipt_version"`
	CorrectionBlockedReason  string                  `json:"correction_blocked_reason"`
	CorrectionCards          []ReceiptCorrectionCard `json:"correction_cards"`
}

type ReceiptCorrectionCard struct {
	ID              int64   `json:"id"`
	Operation       string  `json:"operation"`
	ActualOutputQty float64 `json:"actual_output_qty"`
}

type CorrectJobCardOutputCommand struct {
	ID              int64   `json:"job_card_id"`
	ActualOutputQty float64 `json:"actual_output_qty"`
	Reason          string  `json:"reason"`
	ExpectedVersion string  `json:"expected_version"`
	RequestID       string  `json:"request_id"`
	Operator        string  `json:"-"`
}

func (s *Service) CorrectJobCardOutput(ctx context.Context, cmd CorrectJobCardOutputCommand) (FinishedReceiptTask, error) {
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.ID <= 0 || math.IsNaN(cmd.ActualOutputQty) || math.IsInf(cmd.ActualOutputQty, 0) || cmd.ActualOutputQty < 0 {
		return FinishedReceiptTask{}, fmt.Errorf("请填写正确的累计报工数量")
	}
	if cmd.Reason == "" || cmd.ExpectedVersion == "" || cmd.RequestID == "" || strings.TrimSpace(cmd.Operator) == "" {
		return FinishedReceiptTask{}, fmt.Errorf("更正需要原因、当前版本、请求标识及操作人")
	}
	repo, ok := s.repo.(interface {
		CorrectJobCardOutput(context.Context, CorrectJobCardOutputCommand) (FinishedReceiptTask, error)
	})
	if !ok {
		return FinishedReceiptTask{}, fmt.Errorf("报工更正暂不可用")
	}
	return repo.CorrectJobCardOutput(ctx, cmd)
}
