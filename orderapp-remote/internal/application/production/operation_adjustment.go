package production

import (
	"context"
	"fmt"
)

type StartedComponentBalance struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	BatchCode       string  `json:"batch_code"`
	Warehouse       string  `json:"warehouse"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	Unit            string  `json:"unit"`
	RemainingQty    float64 `json:"remaining_qty"`
	RemainingCost   float64 `json:"remaining_cost"`
}
type StartedInputDetail struct {
	ID           int64                     `json:"id"`
	RemainingQty float64                   `json:"remaining_qty"`
	Unit         string                    `json:"unit"`
	Components   []StartedComponentBalance `json:"components"`
}
type ComponentReturn struct {
	ID  int64   `json:"id"`
	Qty float64 `json:"qty"`
}
type OperationAdjustmentCommand struct {
	ID                   int64                  `json:"-"`
	EmployeeID           int64                  `json:"-"`
	Operator             string                 `json:"-"`
	Kind                 string                 `json:"kind"`
	RequestID            string                 `json:"request_id"`
	ExpectedVersion      string                 `json:"expected_version"`
	Reason               string                 `json:"reason"`
	ActualInputQty       float64                `json:"actual_input_qty"`
	InputUnit            string                 `json:"input_unit"`
	SourceInputID        int64                  `json:"source_input_id"`
	UnprocessedConfirmed bool                   `json:"unprocessed_confirmed"`
	Materials            []OperationMaterialUse `json:"materials"`
	Returns              []ComponentReturn      `json:"returns"`
}

func (s *Service) AdjustOperationMaterials(ctx context.Context, c OperationAdjustmentCommand) (JobCardActionResult, error) {
	if r, ok := s.repo.(interface {
		AdjustOperationMaterials(context.Context, OperationAdjustmentCommand) (JobCardActionResult, error)
	}); ok {
		return r.AdjustOperationMaterials(ctx, c)
	}
	return JobCardActionResult{}, fmt.Errorf("用料调整暂不可用")
}
