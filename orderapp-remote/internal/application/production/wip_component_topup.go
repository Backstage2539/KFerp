package production

import (
	"context"
	"fmt"
)

type WIPTopUpCandidate struct {
	BatchCode       string  `json:"batch_code"`
	Warehouse       string  `json:"warehouse"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	AvailableQty    float64 `json:"available_qty"`
	QualityStatus   string  `json:"quality_status"`
	UnitCost        float64 `json:"unit_cost"`
	CostUnavailable bool    `json:"cost_unavailable"`
}
type WIPTopUpItem struct {
	Key         string              `json:"key"`
	Name        string              `json:"name"`
	Unit        string              `json:"unit"`
	RequiredQty float64             `json:"required_qty"`
	IssuedQty   float64             `json:"issued_qty"`
	ShortageQty float64             `json:"shortage_qty"`
	Candidates  []WIPTopUpCandidate `json:"candidates"`
}
type WIPTopUpPreview struct {
	Version       string         `json:"version"`
	Supported     bool           `json:"supported"`
	BlockedReason string         `json:"blocked_reason,omitempty"`
	Items         []WIPTopUpItem `json:"items"`
}
type WIPTopUpSelection struct {
	Key             string  `json:"key"`
	BatchCode       string  `json:"batch_code"`
	Warehouse       string  `json:"warehouse"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	Qty             float64 `json:"qty"`
}
type WIPTopUpCommand struct {
	ID              int64               `json:"-"`
	Operator        string              `json:"-"`
	EmployeeID      int64               `json:"-"`
	ExpectedVersion string              `json:"expected_version"`
	RequestID       string              `json:"request_id"`
	Materials       []WIPTopUpSelection `json:"materials"`
}

func (s *Service) PreviewWIPTopUp(ctx context.Context, id int64) (WIPTopUpPreview, error) {
	if r, ok := s.repo.(interface {
		PreviewWIPTopUp(context.Context, int64) (WIPTopUpPreview, error)
	}); ok {
		return r.PreviewWIPTopUp(ctx, id)
	}
	return WIPTopUpPreview{}, fmt.Errorf("工位补料暂不可用")
}
func (s *Service) ConfirmWIPTopUp(ctx context.Context, c WIPTopUpCommand) (JobCardActionResult, error) {
	if r, ok := s.repo.(interface {
		ConfirmWIPTopUp(context.Context, WIPTopUpCommand) (JobCardActionResult, error)
	}); ok {
		return r.ConfirmWIPTopUp(ctx, c)
	}
	return JobCardActionResult{}, fmt.Errorf("工位补料暂不可用")
}
