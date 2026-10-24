package production

import (
	"context"
	"fmt"
)

type OperationConversionCard struct {
	ID             int64              `json:"id"`
	Sequence       int                `json:"sequence"`
	Operation      string             `json:"operation"`
	Status         string             `json:"status"`
	OriginalInput  float64            `json:"original_input"`
	OriginalOutput float64            `json:"original_output"`
	InputQty       *float64           `json:"input_qty"`
	OutputQty      *float64           `json:"output_qty"`
	InputUnit      string             `json:"input_unit"`
	Execution      OperationExecution `json:"execution"`
}
type OperationConversionPreview struct {
	LegacyReview
	Cards []OperationConversionCard `json:"cards"`
}
type OperationConversionCommand struct {
	WorkOrderID     int64                     `json:"work_order_id"`
	Cards           []OperationConversionCard `json:"cards"`
	Materials       []OperationMaterialUse    `json:"materials"`
	Reason          string                    `json:"reason"`
	ExpectedVersion string                    `json:"expected_version"`
	RequestID       string                    `json:"request_id"`
	Operator        string                    `json:"-"`
	EmployeeID      int64                     `json:"-"`
}
type operationConversionRepository interface {
	PreviewOperationConversion(context.Context, int64) (OperationConversionPreview, error)
	SubmitOperationConversion(context.Context, OperationConversionCommand) ([]OperationCompletionReceipt, error)
}

func (s *Service) PreviewOperationConversion(ctx context.Context, id int64) (OperationConversionPreview, error) {
	if r, ok := s.repo.(operationConversionRepository); ok {
		return r.PreviewOperationConversion(ctx, id)
	}
	return OperationConversionPreview{}, fmt.Errorf("工单核对暂不可用")
}
func (s *Service) SubmitOperationConversion(ctx context.Context, c OperationConversionCommand) ([]OperationCompletionReceipt, error) {
	if r, ok := s.repo.(operationConversionRepository); ok {
		return r.SubmitOperationConversion(ctx, c)
	}
	return nil, fmt.Errorf("工单核对暂不可用")
}
