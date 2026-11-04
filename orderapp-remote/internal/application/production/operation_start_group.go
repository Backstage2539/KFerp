package production

import (
	"context"
	"fmt"
)

type OperationStartGroupPreview struct {
	Tasks   []OperationCompletionPreview `json:"tasks"`
	Version string                       `json:"version"`
}
type OperationStartGroupCommand struct {
	Items           []OperationStartItem `json:"items"`
	RequestID       string               `json:"request_id"`
	ExpectedVersion string               `json:"expected_version"`
	Operator        string               `json:"-"`
	EmployeeID      int64                `json:"-"`
}
type OperationStartGroupResult struct {
	Items []JobCardActionResult `json:"items"`
}

func (s *Service) PreviewOperationStartGroup(ctx context.Context, ids []int64) (OperationStartGroupPreview, error) {
	if r, ok := s.repo.(interface {
		PreviewOperationStartGroup(context.Context, []int64) (OperationStartGroupPreview, error)
	}); ok {
		return r.PreviewOperationStartGroup(ctx, ids)
	}
	return OperationStartGroupPreview{}, fmt.Errorf("合并开工暂不可用")
}
func (s *Service) StartOperationGroup(ctx context.Context, c OperationStartGroupCommand) (OperationStartGroupResult, error) {
	if r, ok := s.repo.(interface {
		StartOperationGroup(context.Context, OperationStartGroupCommand) (OperationStartGroupResult, error)
	}); ok {
		return r.StartOperationGroup(ctx, c)
	}
	return OperationStartGroupResult{}, fmt.Errorf("合并开工暂不可用")
}

type OperationStartItem struct {
	ID               int64                  `json:"id"`
	ActualInputQty   float64                `json:"actual_input_qty"`
	InputUnit        string                 `json:"input_unit"`
	ExpectedVersion  string                 `json:"expected_version"`
	DifferenceReason string                 `json:"difference_reason"`
	Materials        []OperationMaterialUse `json:"materials"`
}

func (c OperationStartItem) Command() JobCardActionCommand {
	return JobCardActionCommand{ID: c.ID, ActualInputQty: c.ActualInputQty, InputUnit: c.InputUnit, ExpectedVersion: c.ExpectedVersion, DifferenceReason: c.DifferenceReason, Materials: c.Materials}
}
