package production

import (
	"context"
	"fmt"
)

type LegacyReviewMaterial struct {
	OperationMaterialUse
	ConsumedQty float64 `json:"consumed_qty"`
}
type LegacyReview struct {
	FinishedReceiptTask
	Materials           []LegacyReviewMaterial `json:"materials"`
	Version             string                 `json:"version"`
	ReviewBlockedReason string                 `json:"review_blocked_reason"`
}
type LegacyReviewItem struct {
	WorkOrderID        int64                  `json:"work_order_id"`
	ActualOutputQty    *float64               `json:"actual_output_qty"`
	ActualInputG       float64                `json:"actual_input_g"`
	CardQuantities     map[int64]float64      `json:"card_quantities"`
	Materials          []OperationMaterialUse `json:"materials"`
	Reason             string                 `json:"reason"`
	ExpectedVersion    string                 `json:"expected_version"`
	RequestID          string                 `json:"request_id"`
	Operator           string                 `json:"-"`
	EmployeeID         int64                  `json:"-"`
	RecordedOutputOnly bool                   `json:"-"`
}

type RecordedOutputReceiptCommand struct {
	WorkOrderID     int64  `json:"-"`
	ExpectedVersion string `json:"expected_version"`
	RequestID       string `json:"request_id"`
	Operator        string `json:"-"`
	EmployeeID      int64  `json:"-"`
}
type LegacyReviewResult struct {
	WorkOrderID int64                       `json:"work_order_id"`
	Succeeded   bool                        `json:"succeeded"`
	Error       string                      `json:"error,omitempty"`
	Completion  *OperationCompletionReceipt `json:"completion,omitempty"`
}
type legacyReviewRepository interface {
	PreviewLegacyReview(context.Context, []int64) ([]LegacyReview, error)
	SubmitLegacyReview(context.Context, LegacyReviewItem) (LegacyReviewResult, error)
}

func (s *Service) PreviewLegacyReview(ctx context.Context, ids []int64) ([]LegacyReview, error) {
	if r, ok := s.repo.(legacyReviewRepository); ok {
		return r.PreviewLegacyReview(ctx, ids)
	}
	return nil, fmt.Errorf("旧数据核对不可用")
}
func (s *Service) SubmitLegacyReview(ctx context.Context, items []LegacyReviewItem) ([]LegacyReviewResult, error) {
	r, ok := s.repo.(legacyReviewRepository)
	if !ok {
		return nil, fmt.Errorf("旧数据核对不可用")
	}
	if len(items) == 0 || len(items) > 100 {
		return nil, fmt.Errorf("每次请选择 1 至 100 张工单")
	}
	out := []LegacyReviewResult{}
	for _, item := range items {
		result, err := r.SubmitLegacyReview(ctx, item)
		if err != nil {
			result = LegacyReviewResult{WorkOrderID: item.WorkOrderID, Error: err.Error()}
		}
		out = append(out, result)
	}
	return out, nil
}

func (s *Service) ReceiveCompletedOutput(ctx context.Context, c RecordedOutputReceiptCommand) (LegacyReviewResult, error) {
	if c.WorkOrderID <= 0 || c.ExpectedVersion == "" || c.RequestID == "" {
		return LegacyReviewResult{}, fmt.Errorf("请刷新工单后再入库")
	}
	if c.EmployeeID <= 0 {
		return LegacyReviewResult{}, fmt.Errorf("请先绑定员工")
	}
	repo, ok := s.repo.(legacyReviewRepository)
	if !ok {
		return LegacyReviewResult{}, fmt.Errorf("完工入库暂不可用")
	}
	item := LegacyReviewItem{WorkOrderID: c.WorkOrderID, ExpectedVersion: c.ExpectedVersion, RequestID: c.RequestID, Operator: c.Operator, EmployeeID: c.EmployeeID, RecordedOutputOnly: true}
	result, err := repo.SubmitLegacyReview(ctx, item)
	if err != nil {
		return LegacyReviewResult{}, err
	}
	if !result.Succeeded && result.Error != "" {
		return result, fmt.Errorf("%s", result.Error)
	}
	return result, nil
}
