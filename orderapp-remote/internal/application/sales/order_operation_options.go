package sales

import (
	"context"
	"fmt"
)

type OrderOperationOption struct {
	OutputType    string `json:"output_type"`
	OutputID      int64  `json:"output_id"`
	BomSpecID     int64  `json:"bom_spec_id"`
	BomVariantID  int64  `json:"bom_variant_id"`
	OutputName    string `json:"output_name"`
	RouteID       int64  `json:"route_id"`
	StepID        int64  `json:"step_id"`
	OperationID   int64  `json:"operation_id"`
	OperationName string `json:"operation_name"`
}

type OrderOperationOptionsRepository interface {
	OrderOperationOptions(context.Context, int64, int64, int64) ([]OrderOperationOption, error)
}

func (s *Service) OrderOperationOptions(ctx context.Context, productID, bomSpecID, bomVariantID int64) ([]OrderOperationOption, error) {
	repo, ok := s.repo.(OrderOperationOptionsRepository)
	if !ok {
		return nil, fmt.Errorf("工序备注候选暂不可用")
	}
	return repo.OrderOperationOptions(ctx, productID, bomSpecID, bomVariantID)
}
